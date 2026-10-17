#!/usr/bin/env ruby
# frozen_string_literal: true

# run.rb times the history benchmarks: each script on each command given,
# best of three wall-clock runs, start included. A script a command cannot
# run shows as -.
#
#   ruby doc/history/benchmarks/run.rb bin/amber [ruby] ...
require "open3"

scripts = Dir[File.join(__dir__, "*.arb")].sort
commands = ARGV.empty? ? ["bin/amber"] : ARGV
puts (["script"] + commands).join("\t")
scripts.each do |script|
  times = commands.map do |cmd|
    best = nil
    3.times do
      started = Process.clock_gettime(Process::CLOCK_MONOTONIC)
      _, status = Open3.capture2e(*cmd.split, script)
      elapsed = Process.clock_gettime(Process::CLOCK_MONOTONIC) - started
      break best = nil unless status.success?
      best = [best, elapsed].compact.min
    end
    best ? format("%.3f", best) : "-"
  end
  puts ([File.basename(script, ".arb")] + times).join("\t")
end
