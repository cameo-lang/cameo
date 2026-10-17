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
# Each case is timed from inside the Ruby that runs it, by a harness this
# script writes around it (see harness): the case's prelude runs once, then
# its script runs once to warm up and again until the timed runs add up to
# at least a second (--min-time). A time is the mean of the timed runs, read
# from the Ruby's own monotonic clock, so neither the Ruby's start nor the
# prelude is in it. A case from a .yml definition is marked where its
# prelude ends; a .rb benchmark has none, and runs whole each time.
#
# Each case is a row of the table as it finishes: a time and its percentage
# of MRI's for each Ruby, the fastest in green and the slowest in red, and ≠
# where the warm-up run's output is not CRuby's. A summary follows: per
# Ruby, the cases it ran, the failures, the outputs that differ, the total
# time, the geometric mean of its percentages, and how often it was fastest
# and slowest. The rows also go to a tab-separated file.
#
# CRuby 3.4, as warmruby runs it, is the reference output: amber follows 3.4
# where 3.3 differs.
#
#   ruby compare.rb [--jobs N] [--min-time SECONDS] [--timeout SECONDS] [--out FILE] [FILE...]
#   ruby compare.rb --only NAME [FILE...]       # time one Ruby again, keep the others
#   ruby compare.rb --report FILE               # show a results file again

require "fileutils"
require "open3"
require "optparse"
require "tmpdir"

# The Ruby whose output is the reference: CRuby 3.4.
REFERENCE = "warmruby"
GREEN, RED, BOLD = 32, 31, 1
WIDTH = 17

def paint(text, code, color) = color && code ? "\e[#{code}m#{text}\e[0m" : text

# time is what a row compares for a Ruby: the mean timed run, nil when the
# case did not run cleanly.
def time(row, name)
  return nil unless row["#{name}_result"].to_s == "ok"
  row["#{name}_secs"].to_f
end

# duration prints seconds in eight characters, in the unit that suits them.
def duration(t)
  if t < 1e-3 then format("%6.1fµs", t * 1e6)
  elsif t < 1 then format("%6.1fms", t * 1e3)
  else format("%7.3fs", t)
  end
end

def header(color)
  line = format("%-46s", "case") + NAMES.map { format(" %-#{WIDTH}s", _1) }.join
  puts paint(line, BOLD, color), "-" * line.size
end

def table_row(row, color)
  times = NAMES.to_h { [_1, time(row, _1)] }
  ran = times.values.compact
  best, worst = ran.min, ran.max
  base = times["mri"]
  cells = NAMES.map do |name|
    t = times[name]
    next " " + paint(format("%-#{WIDTH}s", row["#{name}_result"]), RED, color) unless t
    pct = base && base.positive? ? format("%5.0f%%", t / base * 100) : "     -"
    mark = row["#{name}_output"].to_s == "differs" ? "≠" : " "
    code = (GREEN if ran.size > 1 && t == best) || (RED if ran.size > 1 && t == worst)
    " " + paint(format("%s %s %s", duration(t), pct, mark), code, color)
  end
  puts format("%-46s", row["file"].to_s[0, 46]) + cells.join
end

def summary(rows, color)
  stats = NAMES.to_h do |name|
    ran = rows.count { time(_1, name) }
    ratios = rows.filter_map do |r|
      t, base = time(r, name), time(r, "mri")
      t / base if t && base&.positive? && t.positive?
    end
    geo = ratios.empty? ? nil : Math.exp(ratios.sum { Math.log(_1) } / ratios.size) * 100
    fastest = rows.count { |r| ts = NAMES.filter_map { time(r, _1) }; ts.size > 1 && time(r, name) == ts.min }
    slowest = rows.count { |r| ts = NAMES.filter_map { time(r, _1) }; ts.size > 1 && time(r, name) == ts.max }
    [name, { ran:, failed: rows.size - ran, differs: rows.count { _1["#{name}_output"].to_s == "differs" },
             total: rows.sum { time(_1, name) || 0 }, geo:, fastest:, slowest: }]
  end
  geos = stats.values.filter_map { _1[:geo] }
  puts
  puts paint(format("%-24s", "summary (one run each)") + NAMES.map { format(" %12s", _1) }.join, BOLD, color)
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

# The line that ends a case's prelude in benchmarks/ruby/works.
PRELUDE_END = /^#--- benchmark: the prelude above runs once, the script below in the loop ---\n/
# What the harness prints after the warm-up run's output, and the line it
# reports its timing on.
WARMED = "\u0001amber-harness: warmed up"
REPORT = "\u0001amber-harness:"
# Magic comments, which Ruby reads only before the first line of code.
MAGIC = /\A#.*\b(frozen_string_literal|encoding|coding|warn_indent|shareable_constant_value)\b/

# harness is the program that times a case from inside the Ruby that runs
# it: the prelude once, then the script in a loop whose first pass warms up
# and whose later passes are timed until they add up to min_time. Its
# report is the timed runs, their seconds and the warm-up's seconds.
def harness(source, min_time)
  prelude, script = source.split(PRELUDE_END, 2)
  prelude, script = "", prelude if script.nil?
  head = ""
  if prelude.empty?
    # A whole program keeps its magic comments at the top.
    lines = script.lines
    head = lines.take_while { _1.match?(MAGIC) }.join
    script = lines.drop(head.lines.size).join
  end
  <<~RUBY
    #{head}#{prelude.chomp}
    __harness_t0 = Process.clock_gettime(Process::CLOCK_MONOTONIC)
    __harness_start = __harness_t0
    __harness_runs = -1
    __harness_warm = 0.0
    __harness_secs = 0.0
    while true
    #{script.chomp}
      __harness_now = Process.clock_gettime(Process::CLOCK_MONOTONIC)
      __harness_runs += 1
      if __harness_runs == 0
        __harness_warm = __harness_now - __harness_t0
        puts #{WARMED.inspect}
        __harness_start = Process.clock_gettime(Process::CLOCK_MONOTONIC)
      else
        __harness_secs = __harness_now - __harness_start
        break if __harness_secs >= #{min_time}
      end
    end
    warn #{REPORT.inspect} + " \#{__harness_runs} \#{__harness_secs} \#{__harness_warm}"
  RUBY
end

options = { jobs: 2, timeout: 600, min_time: 1.0, out: File.join(__dir__, "results.tsv"), color: $stdout.tty? }
OptionParser.new do |o|
  o.banner = "usage: ruby compare.rb [options] [FILE...]"
  o.on("--jobs N", Integer, "cases run at once (default 2)") { options[:jobs] = _1 }
  o.on("--timeout SECONDS", Integer, "per case and Ruby (default 600)") { options[:timeout] = _1 }
  o.on("--min-time SECONDS", Float, "time the runs until they add up to this (default 1)") { options[:min_time] = _1 }
  o.on("--out FILE", "results table (default compare/results.tsv)") { options[:out] = _1 }
  o.on("--report FILE", "show a results file instead of running") { options[:report] = _1 }
  o.on("--only NAME", "time only this Ruby and keep the others' timings from --out") { options[:only] = _1 }
  o.on("--[no-]color", "colour the table (default: when standard output is a terminal)") { options[:color] = _1 }
end.parse!

# The Rubies in the order of the table's columns; MRI is the baseline.
NAMES = %w[mri amber warmruby rbgo].freeze

if options[:report]
  lines = File.readlines(options[:report], chomp: true)
  keys = lines.shift.split("\t")
  rows = lines.map { keys.zip(_1.split("\t", -1)).to_h }
  header(options[:color])
  rows.each { table_row(_1, options[:color]) }
  summary(rows, options[:color])
  exit
end

# With --only, the other Rubies' rows come from the existing results.
kept = {}
if options[:only]
  abort "compare.rb: --only takes one of #{NAMES.join(", ")}" unless NAMES.include?(options[:only])
  lines = File.readlines(options[:out], chomp: true)
  keys = lines.shift.split("\t")
  kept = lines.to_h { |l| r = keys.zip(l.split("\t", -1)).to_h; [r["file"], r] }
end

HERE = __dir__
AMBER = File.expand_path("../..", HERE)
bin = File.join(Dir.tmpdir, "amber-compare")
FileUtils.mkdir_p(bin)
def build(dir, cmd)
  system(cmd, chdir: dir) or abort("compare.rb: building in #{dir} failed")
end
build(AMBER, "go build -o #{bin}/amber ./src/cmd/amber")
build(File.join(AMBER, "benchmarks/rbgo"), "make build >/dev/null")
build(File.join(AMBER, "benchmarks/warmruby"), "make build >/dev/null")

ENGINES = {
  "mri" => ->(f) { ["ruby", f] },
  "amber" => ->(f) { ["#{bin}/amber", f] },
  "warmruby" => ->(f) { ["#{AMBER}/benchmarks/warmruby/bin/warmruby", f] },
  "rbgo" => ->(f) { ["#{AMBER}/benchmarks/rbgo/bin/rbgo", f] },
}.freeze

# A first run downloads warmruby's module and compiles it into its cache.
system("#{AMBER}/benchmarks/warmruby/bin/warmruby", File::NULL, out: File::NULL)

files = ARGV.empty? ? Dir[File.join(AMBER, "benchmarks/ruby/works/*.rb")].sort : ARGV.map { File.expand_path(_1) }

# run_harness runs a harness program and returns the result, the timed
# runs, the mean timed run, the warm-up run, the whole process's seconds
# and the warm-up run's output.
def run_harness(argv, timeout)
  started = Process.clock_gettime(Process::CLOCK_MONOTONIC)
  out = err = status = nil
  Open3.popen3(*argv, chdir: File.dirname(argv.last), pgroup: true) do |stdin, stdout, stderr, wait|
    stdin.close
    readers = [Thread.new { stdout.read }, Thread.new { stderr.read }]
    unless wait.join(timeout)
      Process.kill("KILL", -wait.pid) rescue nil
      wait.join
      readers.each(&:join)
      return [:timeout, 0, nil, nil, Process.clock_gettime(Process::CLOCK_MONOTONIC) - started, ""]
    end
    out, err = readers.map(&:value)
    status = wait.value
  end
  wall = Process.clock_gettime(Process::CLOCK_MONOTONIC) - started
  output, warmed, = out.partition("#{WARMED}\n")
  # The report is on standard error, or standard output where warn writes
  # there, as rbgo's does.
  report = "#{err}\n#{out}"[/^#{Regexp.escape(REPORT)} (\d+) (\S+) (\S+)$/]
  return [:fail, 0, nil, nil, wall, output + err] unless status.success? && !warmed.empty? && report
  runs, secs, warm = $1.to_i, $2.to_f, $3.to_f
  [:ok, runs, secs / runs, warm, wall, output]
end

# The start of each Ruby, the median time of an empty script, for the
# record: the harness keeps it out of every time.
empty = File.join(Dir.tmpdir, "amber-compare-empty.rb")
File.write(empty, "nil\n")
starts = ENGINES.filter_map do |name, argv|
  next if options[:only] && name != options[:only]
  times = 5.times.map do
    t = Process.clock_gettime(Process::CLOCK_MONOTONIC)
    system(*argv.call(empty), out: File::NULL, err: File::NULL) or abort "compare.rb: #{name} cannot run an empty script"
    Process.clock_gettime(Process::CLOCK_MONOTONIC) - t
  end
  [name, times.sort[2]]
end
puts "start, not in any time: " + starts.map { |n, s| format("%s %.3fs", n, s) }.join("  "), ""

header(options[:color])
queue = Queue.new
files.each { queue << _1 }
rows = []
lock = Mutex.new
options[:jobs].times.map do |job|
  Thread.new do
    dir = Dir.mktmpdir("amber-compare-#{job}-")
    while (file = queue.pop(true) rescue nil)
      row = { "file" => File.basename(file) }
      outputs = {}
      source = File.read(file, encoding: "UTF-8")
      ENGINES.each do |name, argv|
        harness_file = File.join(dir, "#{name}-#{File.basename(file)}")
        if options[:only] && name != options[:only]
          # Kept: its timings from the table, its output from one short run.
          old = kept.fetch(row["file"]) { abort "compare.rb: #{row["file"]} is not in #{options[:out]}" }
          old.each { |k, v| row[k] = v if k.start_with?("#{name}_") }
          row["#{name}_result"] = old["#{name}_result"].to_sym
          if row["#{name}_result"] == :ok
            File.write(harness_file, harness(source, 0))
            outputs[name] = run_harness(argv.call(harness_file), options[:timeout]).last
          end
          next
        end
        File.write(harness_file, harness(source, options[:min_time]))
        result, runs, secs, warm, wall, out = run_harness(argv.call(harness_file), options[:timeout])
        row["#{name}_result"] = result
        row["#{name}_runs"] = runs
        row["#{name}_secs"] = secs&.round(7)
        row["#{name}_warm_secs"] = warm&.round(4)
        row["#{name}_wall_secs"] = wall.round(3)
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
        table_row(row, options[:color])
      end
    end
    FileUtils.rm_rf(dir)
  end
end.each(&:join)

# Cases not run this time keep their rows.
kept.each_value { |r| rows << r unless rows.any? { _1["file"] == r["file"] } }
rows.sort_by! { _1["file"] }
columns = ["file"] + ENGINES.keys.flat_map { ["#{_1}_result", "#{_1}_runs", "#{_1}_secs", "#{_1}_warm_secs", "#{_1}_wall_secs", "#{_1}_output"] }
File.write(options[:out], ([columns.join("\t")] + rows.map { |r| columns.map { r[_1] }.join("\t") }).join("\n") + "\n")
summary(rows, options[:color])
puts "", "wrote #{rows.size} rows to #{options[:out]}"
