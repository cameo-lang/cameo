#!/usr/bin/env ruby
# frozen_string_literal: true

# Runs amber's supported subset of CRuby's benchmarks, benchmarks/ruby/works,
# on four Rubies and shows how long each took and whether its output was
# CRuby's:
#
#   mri       the ruby on PATH, which every time is a percentage of
#   amber     this component's interpreter
#   warmruby  CRuby 3.4 compiled to WASI, on wazero, compiled ahead (../warmruby)
#   rbgo      go-embedded-ruby, a Ruby interpreter in Go (../rbgo)
#
# A time is what a run of the script costs without the Ruby's start. Before
# the cases, each Ruby runs an empty script for a few seconds, and the median
# is its start. Each case then runs again and again until the runs add up to
# at least a second (--min-time), and its time is the mean run less the
# start. rbgo and warmruby report the script's own time, without the binary's
# start or the VM's, and that is used for them instead. A run that takes less
# than a millisecond counts as one.
#
# Each case is a row of the table as it finishes: a time and its
# percentage of MRI's for each Ruby, the fastest in green and the slowest in
# red, and ≠ where the output is not CRuby's. A summary follows: per Ruby,
# the cases it ran, the failures, the outputs that differ, the total time,
# the geometric mean of its percentages, and how often it was fastest and
# slowest. The rows also go to a tab-separated file.
#
# CRuby 3.4, as warmruby runs it, is the reference output: amber follows 3.4
# where 3.3 differs.
#
#   ruby compare.rb [--jobs N] [--min-time SECONDS] [--timeout SECONDS] [--out FILE] [FILE...]
#   ruby compare.rb --report FILE [--whole]     # show a results file again
#
# --whole shows whole processes, start included, instead.

require "fileutils"
require "json"
require "open3"
require "optparse"
require "tmpdir"

HERE = __dir__
AMBER = File.expand_path("../..", HERE)

options = { jobs: 2, timeout: 300, min_time: 1.0, out: File.join(HERE, "results.tsv"), color: $stdout.tty? }
OptionParser.new do |o|
  o.banner = "usage: ruby compare.rb [options] [FILE...]"
  o.on("--jobs N", Integer, "cases run at once (default 2)") { options[:jobs] = _1 }
  o.on("--timeout SECONDS", Integer, "per run (default 300)") { options[:timeout] = _1 }
  o.on("--min-time SECONDS", Float, "repeat a case until its runs take this long (default 1)") { options[:min_time] = _1 }
  o.on("--out FILE", "results table (default compare/results.tsv)") { options[:out] = _1 }
  o.on("--whole", "show whole processes, start included") { options[:whole] = true }
  o.on("--report FILE", "show a results file instead of running") { options[:report] = _1 }
  o.on("--[no-]color", "colour the table (default: when standard output is a terminal)") { options[:color] = _1 }
end.parse!

# The Rubies in the order of the table's columns; MRI is the baseline.
NAMES = %w[mri amber warmruby rbgo].freeze
# The Ruby whose output is the reference: CRuby 3.4.
REFERENCE = "warmruby"
GREEN, RED, BOLD = 32, 31, 1
WIDTH = 17

def paint(text, code, color) = color && code ? "\e[#{code}m#{text}\e[0m" : text

# time is what a row compares for a Ruby: a run without its start, or with
# whole the whole process; nil when it did not run cleanly. A results file
# from before runs were repeated has no net time, and gives its whole one.
def time(row, name, whole)
  return nil unless row["#{name}_result"].to_s == "ok"
  net = row["#{name}_net_secs"]
  (whole || net.nil? || net.to_s == "" ? row["#{name}_secs"] : net).to_f
end

def header(color)
  line = format("%-46s", "case") + NAMES.map { format(" %-#{WIDTH}s", _1) }.join
  puts paint(line, BOLD, color), "-" * line.size
end

def table_row(row, whole, color)
  times = NAMES.to_h { [_1, time(row, _1, whole)] }
  ran = times.values.compact
  best, worst = ran.min, ran.max
  base = times["mri"]
  cells = NAMES.map do |name|
    t = times[name]
    next " " + paint(format("%-#{WIDTH}s", row["#{name}_result"]), RED, color) unless t
    pct = base && base.positive? ? format("%5.0f%%", t / base * 100) : "     -"
    mark = row["#{name}_output"].to_s == "differs" ? "≠" : " "
    code = (GREEN if ran.size > 1 && t == best) || (RED if ran.size > 1 && t == worst)
    " " + paint(format("%7.3fs %s %s", t, pct, mark), code, color)
  end
  puts format("%-46s", row["file"].to_s[0, 46]) + cells.join
end

def summary(rows, whole, color)
  stats = NAMES.to_h do |name|
    ran = rows.count { time(_1, name, whole) }
    ratios = rows.filter_map do |r|
      t, base = time(r, name, whole), time(r, "mri", whole)
      t / base if t && base&.positive? && t.positive?
    end
    geo = ratios.empty? ? nil : Math.exp(ratios.sum { Math.log(_1) } / ratios.size) * 100
    fastest = rows.count { |r| ts = NAMES.filter_map { time(r, _1, whole) }; ts.size > 1 && time(r, name, whole) == ts.min }
    slowest = rows.count { |r| ts = NAMES.filter_map { time(r, _1, whole) }; ts.size > 1 && time(r, name, whole) == ts.max }
    [name, { ran:, failed: rows.size - ran, differs: rows.count { _1["#{name}_output"].to_s == "differs" },
             total: rows.sum { time(_1, name, whole) || 0 }, geo:, fastest:, slowest: }]
  end
  geos = stats.values.filter_map { _1[:geo] }
  puts
  puts paint(format("%-24s", whole ? "summary (whole process)" : "summary (start removed)") + NAMES.map { format(" %12s", _1) }.join, BOLD, color)
  [["ran cleanly", :ran], ["failed or timed out", :failed], ["output differs", :differs],
   ["total seconds", :total], ["geometric mean vs mri", :geo], ["fastest", :fastest], ["slowest", :slowest]].each do |label, key|
    cells = NAMES.map do |name|
      v = stats[name][key]
      text = case key
             when :total then format("%11.1fs", v)
             when :geo then v ? format("%11.0f%%", v) : format("%12s", "-")
             else format("%12d", v)
             end
      code = (GREEN if key == :geo && v && v == geos.min) || (RED if key == :geo && v && v == geos.max)
      " " + paint(text, code, color)
    end
    puts format("%-24s", label) + cells.join
  end
end

if options[:report]
  lines = File.readlines(options[:report], chomp: true)
  keys = lines.shift.split("\t")
  rows = lines.map { keys.zip(_1.split("\t", -1)).to_h }
  header(options[:color])
  rows.each { table_row(_1, options[:whole], options[:color]) }
  summary(rows, options[:whole], options[:color])
  exit
end

bin = File.join(Dir.tmpdir, "amber-compare")
FileUtils.mkdir_p(bin)
def build(dir, cmd)
  system(cmd, chdir: dir) or abort("compare.rb: building in #{dir} failed")
end
build(AMBER, "go build -o #{bin}/amber ./src/cmd/amber")
build(File.join(AMBER, "benchmarks/rbgo"), "make build >/dev/null")
build(File.join(AMBER, "benchmarks/warmruby"), "make build >/dev/null")

ENGINES = {
  "mri" => ->(f, _t) { ["ruby", f] },
  "amber" => ->(f, _t) { ["#{bin}/amber", f] },
  "warmruby" => ->(f, t) { ["#{AMBER}/benchmarks/warmruby/bin/warmruby", "-time", t, f] },
  "rbgo" => ->(f, t) { ["#{AMBER}/benchmarks/rbgo/bin/rbgo", "-time", t, f] },
}.freeze

# A first run downloads warmruby's module and compiles it into its cache.
system("#{AMBER}/benchmarks/warmruby/bin/warmruby", File::NULL, out: File::NULL)

files = ARGV.empty? ? Dir[File.join(AMBER, "benchmarks/ruby/works/*.rb")].sort : ARGV.map { File.expand_path(_1) }
TIMING_FILE = ->(name) { File.join(Dir.tmpdir, "amber-compare-#{Thread.current.object_id}-#{name}") }

def run_one(argv, timeout)
  started = Process.clock_gettime(Process::CLOCK_MONOTONIC)
  out, status = nil, nil
  Open3.popen2e(*argv, chdir: File.dirname(argv.last), pgroup: true) do |stdin, io, wait|
    stdin.close
    reader = Thread.new { io.read }
    unless wait.join(timeout)
      Process.kill("KILL", -wait.pid) rescue nil
      wait.join
      reader.join
      return [:timeout, Process.clock_gettime(Process::CLOCK_MONOTONIC) - started, ""]
    end
    out = reader.value
    status = wait.value
  end
  [status.success? ? :ok : :fail, Process.clock_gettime(Process::CLOCK_MONOTONIC) - started, out]
end

# repeat runs a script until the runs add up to min_time, and returns the
# result, the runs, their mean time, the mean script time the Ruby reported
# (nil when it reports none) and the first run's output. A run that fails
# ends it.
def repeat(name, argv, file, min_time, timeout)
  runs, total, script, output = 0, 0.0, 0.0, nil
  timing = TIMING_FILE.call(name)
  loop do
    File.delete(timing) if File.exist?(timing)
    result, secs, out = run_one(argv.call(file, timing), timeout)
    return [result, 1, secs, nil, out] unless result == :ok
    runs += 1
    total += secs
    output ||= out
    script = File.exist?(timing) && script ? script + File.read(timing).to_f : nil
    break if total >= min_time
  end
  [:ok, runs, total / runs, script && script / runs, output]
end

# The start of each Ruby: the median time of an empty script.
empty = File.join(Dir.tmpdir, "amber-compare-empty.rb")
File.write(empty, "nil\n")
STARTS = ENGINES.to_h do |name, argv|
  times = []
  started = Process.clock_gettime(Process::CLOCK_MONOTONIC)
  while times.size < 5 || (Process.clock_gettime(Process::CLOCK_MONOTONIC) - started < 2 && times.size < 50)
    result, secs, = run_one(argv.call(empty, TIMING_FILE.call(name)), 60)
    abort "compare.rb: #{name} cannot run an empty script" unless result == :ok
    times << secs
  end
  [name, times.sort[times.size / 2]]
end
puts "start: " + STARTS.map { |n, s| format("%s %.3fs", n, s) }.join("  "), ""

header(options[:color])
queue = Queue.new
files.each { queue << _1 }
rows = []
lock = Mutex.new
options[:jobs].times.map do
  Thread.new do
    while (file = queue.pop(true) rescue nil)
      row = { "file" => File.basename(file) }
      outputs = {}
      ENGINES.each do |name, argv|
        result, runs, secs, script, out = repeat(name, argv, file, options[:min_time], options[:timeout])
        row["#{name}_result"] = result
        row["#{name}_runs"] = runs
        row["#{name}_secs"] = secs.round(4)
        row["#{name}_script_secs"] = script&.round(4)
        row["#{name}_net_secs"] = [script || secs - STARTS[name], 0.001].max.round(4) if result == :ok
        outputs[name] = out
      end
      ENGINES.each_key do |name|
        row["#{name}_output"] = if row["#{name}_result"] != :ok then "-"
                                elsif row["#{REFERENCE}_result"] != :ok then "?"
                                elsif outputs[name] == outputs[REFERENCE] then "same"
                                else "differs"
                                end
      end
      lock.synchronize do
        rows << row
        table_row(row, options[:whole], options[:color])
      end
    end
  end
end.each(&:join)

rows.sort_by! { _1["file"] }
columns = ["file"] + ENGINES.keys.flat_map { ["#{_1}_result", "#{_1}_runs", "#{_1}_secs", "#{_1}_script_secs", "#{_1}_net_secs", "#{_1}_output"] }
File.write(options[:out], ([columns.join("\t")] + rows.map { |r| columns.map { r[_1] }.join("\t") }).join("\n") + "\n")
summary(rows, options[:whole], options[:color])
puts "", "wrote #{rows.size} rows to #{options[:out]}"
