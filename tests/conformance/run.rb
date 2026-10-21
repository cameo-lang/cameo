#!/usr/bin/env ruby
# frozen_string_literal: true

# Runs every script cameo holds to an output, the examples, the samples on
# the site and CRuby's benchmarks under benchmarks/ruby/works, twice: on
# the interpreter, and with the top-level methods that cameo build compiles.
# The compiled run must print what the interpreter prints and exit as it
# exits.
#
#   ruby tests/conformance/run.rb [--jobs N] [--timeout SECONDS] [FILE_OR_DIR...]
#
# Both runs go through a generated main that runs the script and prints its
# last value after its output, "=> 5702887" for a script that ends in
# fib(34), since most of CRuby's benchmarks print nothing. The compiled runs
# share one binary: cameo build writes a package per script under
# bin/_conformance, the main imports them all, and go build links them into
# one program that runs the script it is given with that script's methods
# compiled. A script none of whose methods compile is counted and not run.
# A script whose two interpreter runs print differently, one that draws
# random numbers or reads the clock, is nondeterministic and held to
# nothing. The run fails when a script builds wrongly or prints differently
# compiled.

require "cgi"
require "etc"
require "fileutils"
require "optparse"
require "tmpdir"

HERE = __dir__
ROOT = File.expand_path("../..", HERE)
BIN = File.join(ROOT, "bin")
OUT = File.join(BIN, "_conformance")
MODULE = "github.com/cameo-lang/cameo"

options = { jobs: [Etc.nprocessors / 2, 1].max, timeout: 300 }
OptionParser.new do |o|
  o.banner = "usage: ruby run.rb [options] [FILE_OR_DIR...]"
  o.on("--jobs N", Integer, "scripts run at once (default: half the cores)") { options[:jobs] = _1 }
  o.on("--timeout SECONDS", Integer, "per run (default 300)") { options[:timeout] = _1 }
end.parse!

# sh runs a command in the repository and returns its output, or fails.
def sh(*cmd, chdir: ROOT)
  out = IO.popen(cmd, chdir:, err: [:child, :out], &:read)
  raise "#{cmd.join(' ')} failed:\n#{out}" unless $?.success?
  out
end

# A case is one script: where it came from, its path, and the directory it
# runs in.
Case = Struct.new(:group, :label, :path, :cwd, :compiled, :outcome, :detail)

# cases lists the scripts: the examples run in their directory, since one
# requires a file beside it; the site's samples are written out under
# bin/_conformance; CRuby's benchmarks run in a scratch directory.
def cases(paths, scratch)
  list = []
  examples = File.join(ROOT, "examples")
  Dir.glob(File.join(examples, "*.cameo")).sort.each do |f|
    list << Case.new("examples", File.basename(f), f, examples)
  end
  site = File.join(OUT, "site")
  FileUtils.mkdir_p(site)
  Dir.glob(File.join(ROOT, "doc/gh_pages/*.html")).sort.each do |page|
    File.read(page).scan(%r{<pre class="cameo( rejected)?"><code>(.*?)</code></pre>}m).each_with_index do |(rejected, code), i|
      next if rejected
      f = File.join(site, "#{File.basename(page, '.html')}-#{i + 1}.cameo")
      File.write(f, CGI.unescapeHTML(code))
      list << Case.new("site", File.basename(f), f, scratch)
    end
  end
  Dir.glob(File.join(ROOT, "benchmarks/ruby/works/*.rb")).sort.each do |f|
    list << Case.new("benchmarks", File.basename(f), f, scratch)
  end
  return list if paths.empty?
  wanted = paths.map { File.expand_path(_1) }
  list.select { |c| wanted.any? { |w| c.path == w || c.path.start_with?("#{w}/") } }
end

# run runs a command with a timeout and returns its outcome, :ok, :fail or
# :timeout, with what it printed.
def run(cmd, cwd, timeout, out)
  started = Process.clock_gettime(Process::CLOCK_MONOTONIC)
  pid = Process.spawn(*cmd, out:, err: File::NULL, in: File::NULL, chdir: cwd)
  status = nil
  until (status = Process.wait2(pid, Process::WNOHANG)&.last)
    if Process.clock_gettime(Process::CLOCK_MONOTONIC) - started > timeout
      Process.kill("KILL", pid)
      Process.wait(pid)
      break
    end
    sleep 0.01
  end
  outcome = status.nil? ? :timeout : status.success? ? :ok : :fail
  [outcome, File.read(out, encoding: "UTF-8")]
end

# first_difference names the first line two outputs differ on.
def first_difference(a, b)
  al, bl = a.lines, b.lines
  i = al.zip(bl).index { |x, y| x != y } || [al.size, bl.size].min
  "line #{i + 1}: interpreter #{al[i].inspect}, compiled #{bl[i].inspect}"
end

sh("go", "build", "-o", File.join(BIN, "cameo"), "./src/cmd/cameo")
cameo = File.join(BIN, "cameo")
FileUtils.rm_rf(OUT)

Dir.mktmpdir("cameo-conformance") do |scratch|
  list = cases(ARGV, scratch)
  abort "no scripts match #{ARGV.join(', ')}" if list.empty?

  # Build a package per script, and main.go importing those that compiled
  # something.
  imports = []
  list.each_with_index do |c, i|
    pkg = format("c%04d", i + 1)
    dir = File.join(OUT, "cases", pkg)
    built = IO.popen([cameo, "build", c.path, dir, pkg], err: [:child, :out], &:read)
    unless $?.success?
      c.outcome, c.detail = :build_failed, built.lines.first.to_s.strip
      next
    end
    c.compiled = built.lines.grep(/^compiled /).map { _1.split[1] }
    imports << "#{MODULE}/#{dir.delete_prefix("#{ROOT}/")}" unless c.compiled.empty?
  end
  # main runs a script and prints its last value, with the compiled
  # packages imported or not.
  main = lambda do |imported|
    <<~GO
      // Code generated by tests/conformance/run.rb; DO NOT EDIT.

      // Runs the script it is given and prints its last value after its
      // output. With the compiled packages imported, a script runs with its
      // own methods compiled, found by its digest.
      package main

      import (
      \t"fmt"
      \t"os"
      \t"runtime/debug"

      \t"#{MODULE}/src/cameo"
      #{imported.empty? ? "" : "\n" + imported.map { "\t_ \"#{_1}\"" }.join("\n")}
      )

      func main() {
      \tif os.Getenv("GOGC") == "" {
      \t\tdebug.SetGCPercent(400)
      \t}
      \tif os.Getenv("GOMEMLIMIT") == "" {
      \t\tdebug.SetMemoryLimit(1 << 30)
      \t}
      \tsrc, err := os.ReadFile(os.Args[1])
      \tfail(err)
      \tprog, err := cameo.Load(os.Args[1], string(src), os.Stdout, os.Stderr)
      \tfail(err)
      \tfail(prog.Run())
      \tlast, err := prog.LastValue()
      \tfail(err)
      \tfmt.Printf("=> %s\\n", last)
      }

      func fail(err error) {
      \tif err != nil {
      \t\tfmt.Fprintln(os.Stderr, err)
      \t\tos.Exit(1)
      \t}
      }
    GO
  end
  binaries = { "interpreted" => [], "compiled" => imports }.to_h do |name, imported|
    dir = File.join(OUT, name)
    FileUtils.mkdir_p(dir)
    File.write(File.join(dir, "main.go"), main.(imported))
    binary = File.join(dir, name)
    sh("go", "build", "-o", binary, "./" + dir.delete_prefix("#{ROOT}/"))
    [name, binary]
  end
  interpreted, compiled = binaries.values_at("interpreted", "compiled")

  queue = Queue.new
  list.each { queue << _1 if _1.outcome.nil? && !_1.compiled.empty? }
  queue.close
  Array.new(options[:jobs]) do |w|
    Thread.new do
      a, b = File.join(scratch, "out-#{w}-a"), File.join(scratch, "out-#{w}-b")
      while (c = queue.pop)
        want, want_out = run([interpreted, c.path], c.cwd, options[:timeout], a)
        got, got_out = run([compiled, c.path], c.cwd, options[:timeout], b)
        if want == got && want_out == got_out
          c.outcome = :same
        else
          again, again_out = run([interpreted, c.path], c.cwd, options[:timeout], a)
          if again != want || again_out != want_out
            c.outcome = :nondeterministic
          else
            c.outcome = :differs
            c.detail = want == got ? first_difference(want_out, got_out) : "the interpreter run #{want}, the compiled run #{got}"
          end
        end
      end
    end
  end.each(&:join)

  groups = list.group_by(&:group)
  puts format("%-12s %8s %9s %8s %6s %8s %16s %13s", "", "scripts", "compiled", "methods", "same", "differs", "nondeterministic", "build failed")
  (groups.keys + ["total"]).each do |g|
    cs = g == "total" ? list : groups[g]
    with = cs.count { _1.compiled && !_1.compiled.empty? }
    puts format("%-12s %8d %9d %8d %6d %8d %16d %13d", g, cs.size, with, cs.sum { _1.compiled.to_a.size },
                cs.count { _1.outcome == :same }, cs.count { _1.outcome == :differs },
                cs.count { _1.outcome == :nondeterministic }, cs.count { _1.outcome == :build_failed })
  end
  bad = list.select { %i[differs build_failed].include?(_1.outcome) }
  unless bad.empty?
    puts
    bad.each { puts "#{_1.outcome.to_s.tr('_', ' ')}: #{_1.group}/#{_1.label}: #{_1.detail}" }
    exit 1
  end
end
