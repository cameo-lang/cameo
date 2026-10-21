#!/usr/bin/env ruby
# frozen_string_literal: true

# Times the scripts under tests/performance on cameo's interpreter, compiled
# with cameo build, and on CRuby with and without YJIT, and prints one
# Markdown table: the interpreter against CRuby's, and the compiled build
# against YJIT, each with its difference from the Ruby it is compared with.
#
#   ruby tests/performance/run.rb [--ruby PATH] [--runs N] [DIR_OR_FILE...]
#
# Each script prints "result seconds" as its last line, the seconds read
# from the clock inside the script, so no start is in a time. A cell is the
# best of --runs runs (default 3). Without paths, every directory under
# tests/performance runs. The machine, the versions and cameo's commit go to
# stderr, so that the table can be pasted on its own. Build output goes to
# bin/_performance, which the Go tool ignores.
#
# A difference is green when cameo takes less than 90% of Ruby's time, red
# when more than 110%, and yellow in between.

require "etc"
require "optparse"

HERE = __dir__
ROOT = File.expand_path("../..", HERE)
BIN = File.join(ROOT, "bin")
OUT = File.join(BIN, "_performance")
GREEN, YELLOW, RED = "\u{1F7E2}", "\u{1F7E1}", "\u{1F534}"

options = { ruby: "ruby", runs: 3 }
OptionParser.new do |o|
  o.banner = "usage: ruby run.rb [options] [DIR_OR_FILE...]"
  o.on("--ruby PATH", "the CRuby to compare with (default: ruby on PATH)") { options[:ruby] = _1 }
  o.on("--runs N", Integer, "runs per cell, the best of which counts (default 3)") { options[:runs] = _1 }
end.parse!

# sh runs a command in the repository and returns its output, or fails.
def sh(*cmd, chdir: ROOT)
  out = IO.popen(cmd, chdir:, err: [:child, :out], &:read)
  raise "#{cmd.join(' ')} failed:\n#{out}" unless $?.success?
  out
end

# best runs a command runs times and returns the best seconds and the
# result, from the "result seconds" line each run prints last.
def best(cmd, runs)
  secs = nil
  result = nil
  runs.times do
    result, t = sh(*cmd).lines.last.to_s.split
    t = Float(t)
    secs = t if secs.nil? || t < secs
  end
  [secs, result]
end

# delta is a time as a difference from the Ruby it is compared with, with
# its colour, or "-" when either is missing.
def delta(cameo, ruby)
  return "-" unless cameo && ruby
  ratio = cameo / ruby
  colour = ratio < 0.9 ? GREEN : ratio > 1.1 ? RED : YELLOW
  format("%s %+d%%", colour, ((ratio - 1) * 100).round)
end

def cell(secs) = secs ? format("%.3f", secs) : "-"

ruby = options[:ruby]
version = sh(ruby, "-e", "print RUBY_VERSION")
yjit = system(ruby, "--yjit", "-e", "exit(defined?(RubyVM::YJIT) && RubyVM::YJIT.enabled? ? 0 : 1)", out: File::NULL, err: File::NULL)

sh("go", "build", "-o", File.join(BIN, "cameo"), "./src/cmd/cameo")
cameo = File.join(BIN, "cameo")
model = `sysctl -n machdep.cpu.brand_string 2>/dev/null`.strip
model = File.read("/proc/cpuinfo")[/^model name\s*:\s*(.*)/, 1].to_s.strip if model.empty? && File.exist?("/proc/cpuinfo")
warn "machine: #{`uname -sm`.strip}, #{model}, #{Etc.nprocessors} cores"
warn "go: #{sh('go', 'env', 'GOVERSION').strip}, cameo: #{sh('git', 'rev-parse', '--short', 'HEAD').strip}"
warn "ruby: #{sh(ruby, '-v').strip}#{yjit ? '' : ', without YJIT'}"

roots = ARGV.empty? ? Dir.glob(File.join(HERE, "*/")).sort : ARGV.map { File.expand_path(_1) }
scripts = roots.flat_map { File.file?(_1) ? [_1] : Dir.glob(File.join(_1, "*.rb")).sort }
abort "no scripts under #{roots.join(', ')}" if scripts.empty?

puts "| script | cameo interp | ruby #{version} (100%) | Δ | cameo compiled | ruby #{version} yjit (100%) | Δ |"
puts "|---|---|---|---|---|---|---|"
differs = []
scripts.each do |script|
  name = File.basename(script, ".rb")
  label = File.join(File.basename(File.dirname(script)), name)
  dir = File.join(OUT, label)
  built = sh(cameo, "build", script, dir)
  built.lines.grep(/^interpreted (?!now:)/).each { warn "#{label}: #{_1}" }
  binary = File.join(dir, name)
  sh("go", "build", "-o", binary, "./" + dir.delete_prefix("#{ROOT}/"))

  times = {}
  results = {}
  [["interp", [cameo, script]], ["compiled", [binary]],
   ["ruby", [ruby, "--disable-yjit", script]], ["yjit", yjit ? [ruby, "--yjit", script] : nil]].each do |who, cmd|
    next unless cmd
    times[who], results[who] = best(cmd, options[:runs])
  end
  if results.values.uniq.size > 1
    differs << "#{label}: #{results.map { |who, r| "#{who} printed #{r}" }.join(', ')}"
  end
  puts "| #{label} | #{cell(times['interp'])} | #{cell(times['ruby'])} | #{delta(times['interp'], times['ruby'])} " \
       "| #{cell(times['compiled'])} | #{cell(times['yjit'])} | #{delta(times['compiled'], times['yjit'])} |"
end
unless differs.empty?
  puts
  puts "The results differ:", *differs.map { "- #{_1}" }
  exit 1
end
