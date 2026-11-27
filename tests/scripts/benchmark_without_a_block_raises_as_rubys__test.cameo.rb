# benchmark: testcase to demonstrate that Benchmark without a block raises as
# Ruby's does
#
# The scripts are plain Ruby, and the expected outputs are Ruby 3.4's, with
# benchmark 0.4.0.
#
# (Introduced in PR #114. expected output follows the __END__ separator.)
require "benchmark"
[-> { Benchmark.measure }, -> { Benchmark.realtime }, -> { Benchmark.bm }, -> { Benchmark.bmbm { |x| x.report("a") } }, -> { Benchmark::Tms.new.add }].each do |f|
  f.call
rescue => e
  puts "#{e.class}: #{e.message}"
end
p require("benchmark")

__END__
LocalJumpError: no block given (yield)
LocalJumpError: no block given (yield)
LocalJumpError: no block given (yield)
ArgumentError: no block
LocalJumpError: no block given (yield)
false
