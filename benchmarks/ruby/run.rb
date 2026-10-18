#!/usr/bin/env ruby
# frozen_string_literal: true

# Runs the benchmarks under cameo and reports, per file, whether it ran
# cleanly. Exits non-zero when a file under works/ fails, so the grouping
# cannot go stale silently, and lists the files elsewhere that run cleanly
# now: candidates to move to works/, once their result has been checked
# against Ruby's.
#
#   ruby run.rb [--cameo PATH] [--jobs N] [--timeout SECONDS] [DIR_OR_FILE...]
#
# Without paths it runs every directory. Without --cameo it builds the
# binary from this component first.

require "optparse"
require "tmpdir"

HERE = __dir__
COMPONENT = File.expand_path("../..", HERE)

options = { jobs: 4, timeout: 300 }
OptionParser.new do |o|
  o.banner = "usage: ruby run.rb [options] [DIR_OR_FILE...]"
  o.on("--cameo PATH", "the cameo binary to run") { options[:cameo] = _1 }
  o.on("--jobs N", Integer, "files run at once (default 4)") { options[:jobs] = _1 }
  o.on("--timeout SECONDS", Integer, "per file (default 300)") { options[:timeout] = _1 }
end.parse!

Dir.mktmpdir("cameo-benchmarks") do |tmp|
  cameo = options[:cameo]
  unless cameo
    cameo = File.join(tmp, "cameo")
    system("go", "build", "-o", cameo, "./src/cmd/cameo", chdir: COMPONENT, exception: true)
  end
  cameo = File.expand_path(cameo)

  roots = ARGV.empty? ? [HERE] : ARGV.map { File.expand_path(_1) }
  files = roots.flat_map { File.file?(_1) ? [_1] : Dir.glob(File.join(_1, "**", "*.rb")) }
  files -= [File.expand_path(__FILE__)]
  files.sort!

  results = {}
  lock = Mutex.new
  queue = Queue.new
  files.each { queue << _1 }
  queue.close
  workers = Array.new(options[:jobs]) do |w|
    Thread.new do
      while (file = queue.pop)
        err = File.join(tmp, "err-#{w}")
        started = Process.clock_gettime(Process::CLOCK_MONOTONIC)
        pid = Process.spawn(cameo, file, out: File::NULL, err: err, in: File::NULL, chdir: tmp)
        status = nil
        until (status = Process.wait2(pid, Process::WNOHANG)&.last)
          if Process.clock_gettime(Process::CLOCK_MONOTONIC) - started > options[:timeout]
            Process.kill("KILL", pid)
            Process.wait(pid)
            break
          end
          sleep 0.02
        end
        seconds = Process.clock_gettime(Process::CLOCK_MONOTONIC) - started
        outcome = if status.nil? then :timeout elsif status.success? then :ok else :fail end
        message = File.read(err, encoding: "UTF-8").scrub.lines.first.to_s.strip.sub(/\A\S+?:\d+:\s*/, "")
        lock.synchronize { results[file] = [outcome, seconds, message] }
      end
    end
  end
  workers.each(&:join)

  relative = ->(f) { f.delete_prefix("#{HERE}/") }
  broken = []
  promoted = []
  results.sort.each do |file, (outcome, seconds, message)|
    rel = relative.(file)
    puts format("%-7s %7.2fs  %s%s", outcome, seconds, rel, outcome == :ok ? "" : "  #{message}")
    in_works = rel.start_with?("works/")
    broken << rel if in_works && outcome != :ok
    promoted << rel if !in_works && outcome == :ok
  end
  puts
  puts "#{results.size} files, #{results.count { _2[0] == :ok }} ran cleanly"
  unless promoted.empty?
    puts "Run cleanly outside works/ (check the result against Ruby's before moving one):"
    promoted.each { puts "  #{_1}" }
  end
  unless broken.empty?
    puts "Fail under works/:"
    broken.each { puts "  #{_1}" }
    exit 1
  end
end
