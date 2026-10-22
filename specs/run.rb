#!/usr/bin/env ruby
# frozen_string_literal: true

# Runs ruby/spec's specs under cameo, with the part of mspec that cameo can
# run (runner/prelude.cameo) in place of mspec.
#
# mspec defines should on Object, and cameo cannot reopen a builtin class,
# so each spec is rewritten with Prism before it runs: mspec's expectations
# become calls into the prelude, and each require_relative of a fixture is
# replaced by the fixture's text, once per spec. The spec_helper is dropped.
# One program, the prelude and the rewritten spec, goes through cameo check
# and then cameo; each example prints one line, which this script counts.
#
#   ruby run.rb [--spec DIR] [--cameo BIN] [--jobs N] [--timeout SECONDS]
#               [--ruby RUBY] [--out DIR] [DIR_OR_FILE ...]
#
# The default --spec is the copy checkout.rb returns, and the default
# targets are its language/ and core/. --ruby runs each program that cameo
# runs under that Ruby as well: the prelude is Ruby, so the examples that
# pass there are the ones this harness can run, and the summary counts how
# many of them pass under cameo. --out keeps each rewritten program and its
# output, and writes results.tsv, one row per spec file, and failures.tsv,
# one row per example that failed or raised under cameo.

require "etc"
require "fileutils"
require "open3"
require "optparse"
require "prism"
require "tmpdir"

HERE = __dir__
PRELUDE = File.read(File.join(HERE, "runner", "prelude.cameo"))

options = { jobs: Etc.nprocessors, timeout: 20 }
OptionParser.new do |o|
  o.banner = "usage: ruby run.rb [options] [DIR_OR_FILE ...]"
  o.on("--spec DIR", "a copy of github.com/ruby/spec (default: what checkout.rb prints)") { options[:spec] = File.expand_path(_1) }
  o.on("--cameo BIN", "the cameo binary (default: build it)") { options[:cameo] = File.expand_path(_1) }
  o.on("--jobs N", Integer, "spec files run at once (default: the number of CPUs)") { options[:jobs] = _1 }
  o.on("--timeout SECONDS", Integer, "the time one spec file may take (default: 20)") { options[:timeout] = _1 }
  o.on("--ruby RUBY", "also run each program under this Ruby, as a baseline") { options[:ruby] = _1 }
  o.on("--out DIR", "keep the rewritten programs, their output and results.tsv here") { options[:out] = File.expand_path(_1) }
end.parse!

spec = options[:spec] || begin
  out, status = Open3.capture2("ruby", File.join(HERE, "checkout.rb"))
  abort "run.rb: checkout.rb failed" unless status.success?
  out.strip
end

cameo = options[:cameo] || begin
  bin = File.join(Dir.tmpdir, "cameo-spec-run")
  system("go", "build", "-o", bin, "./src/cmd/cameo", chdir: File.expand_path("..", HERE)) or abort "run.rb: building cameo failed"
  bin
end

targets = ARGV.empty? ? %w[language core].map { File.join(spec, _1) } : ARGV.map { File.expand_path(_1) }
files = targets.flat_map { |t| File.directory?(t) ? Dir.glob(File.join(t, "**", "*_spec.rb")) : [t] }.sort
abort "run.rb: no spec files under #{targets.join(", ")}" if files.empty?

OPERATORS = %i[== != < <= > >= =~ !~ ===].freeze

# Rewriter turns one spec file and the fixtures it loads into one program.
class Rewriter
  def initialize
    @loaded = {}
  end

  def file(path)
    return "" if @loaded[path]
    @loaded[path] = true
    src = File.binread(path).force_encoding(Encoding::UTF_8)
    result = Prism.parse(src, filepath: path)
    raise "#{path}: #{result.errors.first.message}" unless result.success?
    emit(src, path, result.value)
  end

  private

  def should?(node)
    node.is_a?(Prism::CallNode) && %i[should should_not].include?(node.name) && node.receiver
  end

  # target? is true for a node this rewriter replaces.
  def target?(node)
    return false unless node.is_a?(Prism::CallNode)
    return true if node.name == :require_relative && node.receiver.nil?
    return true if should?(node)
    return false unless should?(node.receiver) && node.receiver.arguments.nil? && node.receiver.block.nil?
    true
  end

  # targets returns the outermost nodes under node, not node itself, that
  # this rewriter replaces.
  def targets(node, acc = [])
    node.compact_child_nodes.each { |child| target?(child) ? acc << child : targets(child, acc) }
    acc
  end

  def slice(src, from, to) = src.byteslice(from...to)

  # emit returns the source of node, rewritten.
  def emit(src, path, node)
    return "" if node.nil?
    target?(node) ? replace(src, path, node) : copy(src, path, node)
  end

  # copy returns the source of node with the nodes under it rewritten.
  def copy(src, path, node)
    loc = node.location
    out = +""
    pos = loc.start_offset
    targets(node).sort_by { _1.location.start_offset }.each do |t|
      out << slice(src, pos, t.location.start_offset) << replace(src, path, t)
      pos = t.location.end_offset
    end
    out << slice(src, pos, loc.end_offset)
  end

  # call returns name(negated, (receiver of should)prefix, arguments) and
  # the block of rest_call, rewritten.
  def call(name, src, path, should, rest_call, prefix)
    out = +"#{name}(#{should.name == :should_not}, (#{emit(src, path, should.receiver)})#{prefix}"
    out << ", " << emit(src, path, rest_call.arguments) if rest_call.arguments
    block = rest_call.block
    out << ", " << emit(src, path, block) if block.is_a?(Prism::BlockArgumentNode)
    out << ")"
    out << " " << emit(src, path, block) if block.is_a?(Prism::BlockNode)
    out
  end

  def replace(src, path, node)
    if node.name == :require_relative
      arg = node.arguments&.arguments&.first
      return copy(src, path, node) unless arg.is_a?(Prism::StringNode)
      return "nil" if File.basename(arg.unescaped, ".rb") == "spec_helper"
      target = File.expand_path(arg.unescaped, File.dirname(path))
      target += ".rb" unless target.end_with?(".rb")
      return "\n#{file(target)}\nnil\n"
    end
    if should?(node)
      return call("__should", src, path, node, node, "") if node.arguments
      return copy(src, path, node)
    end
    should = node.receiver
    case node.name
    when *OPERATORS
      call("__should_op", src, path, should, node, ", :#{node.name}")
    when :raise
      call("__should_raise", src, path, should, node, "")
    else
      call("__should_pred", src, path, should, node, ", #{node.name.inspect}")
    end
  end
end

# examples returns the [kind, text] of each line an example printed.
def examples(stdout)
  stdout.each_line.filter_map do |line|
    line.chomp.delete_prefix("\u0001S ").split(" ", 2) if line.start_with?("\u0001S ")
  end
end

# passed returns the names of the examples that passed, each as often as it
# passed.
def passed(lines) = lines.filter_map { |kind, text| text if kind == "P" }.tally

# run_file returns the result of one spec file as a Hash.
def run_file(cameo, ruby, spec, path, timeout, out)
  rel = path.delete_prefix("#{spec}/")
  row = { file: rel, status: "ran", examples: 0, passed: 0, failed: 0, errors: 0, unsupported: 0,
          ruby_passed: 0, both_passed: 0, message: "" }
  program = begin
    PRELUDE + "\n" + Rewriter.new.file(path)
  rescue StandardError => e
    return row.merge(status: "rewrite", message: e.message)
  end
  dir = out ? File.join(out, "programs", File.dirname(rel)) : Dir.mktmpdir("cameo-spec")
  FileUtils.mkdir_p(dir)
  prog = File.join(dir, File.basename(rel))
  File.write(prog, program)

  check_out, check_status = Open3.capture2e(cameo, "check", prog)
  unless check_status.success?
    first = check_out.lines.first.to_s.strip.sub(/\A[^:]*:\d+: /, "")
    return row.merge(status: "check", message: first)
  end

  stdout, stderr, status, timed_out = run_with_timeout([cameo, prog], timeout)
  File.write("#{prog}.out", stdout + stderr) if out
  failures = []
  lines = examples(stdout)
  if ruby
    ruby_out, = run_with_timeout([ruby, prog], timeout)
    on_ruby = passed(examples(ruby_out))
    row[:ruby_passed] = on_ruby.values.sum
    row[:both_passed] = passed(lines).sum { |name, n| [n, on_ruby.fetch(name, 0)].min }
  end
  lines.each do |kind, text|
    case kind
    when "P" then row[:passed] += 1
    when "F" then row[:failed] += 1; failures << ["F", text]
    when "E" then row[:errors] += 1; failures << ["E", text]
    when "U" then row[:unsupported] += 1
    end
  end
  row[:examples] = row[:passed] + row[:failed] + row[:errors]
  row[:failures] = failures
  if timed_out
    row.merge(status: "timeout", message: "after #{timeout}s")
  elsif !status.success?
    row.merge(status: "crash", message: stderr.lines.first.to_s.strip.sub(/\A[^:]*:\d+: /, ""))
  else
    row
  end
ensure
  FileUtils.rm_rf(dir) if dir && !out
end

def run_with_timeout(command, timeout)
  Open3.popen3(*command) do |stdin, stdout, stderr, wait|
    stdin.close
    out = Thread.new { stdout.read }
    err = Thread.new { stderr.read }
    timed_out = wait.join(timeout).nil?
    if timed_out
      Process.kill(:KILL, wait.pid)
      wait.join
    end
    [out.value.force_encoding(Encoding::UTF_8).scrub, err.value.force_encoding(Encoding::UTF_8).scrub, wait.value, timed_out]
  end
end

queue = Queue.new
files.each { queue << _1 }
results = []
lock = Mutex.new
workers = Array.new([options[:jobs], files.size].min) do
  Thread.new do
    while (path = (queue.pop(true) rescue nil))
      row = run_file(cameo, options[:ruby], spec, path, options[:timeout], options[:out])
      lock.synchronize do
        results << row
        $stderr.print "\r#{results.size}/#{files.size}" if $stderr.tty?
      end
    end
  end
end
workers.each(&:join)
$stderr.puts if $stderr.tty?
results.sort_by! { _1[:file] }

if options[:out]
  File.open(File.join(options[:out], "results.tsv"), "w") do |f|
    columns = %i[file status examples passed failed errors unsupported ruby_passed both_passed message]
    f.puts columns.join("\t")
    results.each { |r| f.puts r.values_at(*columns).join("\t") }
  end
  File.open(File.join(options[:out], "failures.tsv"), "w") do |f|
    results.each { |r| (r[:failures] || []).each { |kind, text| f.puts [r[:file], kind, text].join("\t") } }
  end
end

# The summary: per top-level directory, files by status and examples.
results.group_by { _1[:file].split("/").first }.each do |area, rows|
  puts "## #{area}: #{rows.size} spec files"
  rows.group_by { _1[:status] }.sort_by { -_2.size }.each { |status, rs| puts "  #{status.ljust(8)} #{rs.size}" }
  sum = ->(k) { rows.sum { _1[k] } }
  puts "  examples #{sum.(:examples)}: #{sum.(:passed)} passed, #{sum.(:failed)} failed, #{sum.(:errors)} errors; #{sum.(:unsupported)} before/after/shared blocks not run"
  puts "  #{sum.(:both_passed)} of the #{sum.(:ruby_passed)} examples #{options[:ruby]} passes pass under cameo" if options[:ruby]
end
