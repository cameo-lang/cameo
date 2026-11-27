# benchmark: testcase to demonstrate that benchmark tms shows its times
#
# Ruby shows an address before the instance variables; a cameo value has none.
#
# The scripts are plain Ruby, and the expected outputs are Ruby 3.4's, with
# benchmark 0.4.0.
#
# times are a time Benchmark measured, which differs from run to run.
#
# (Introduced in PR #114. expected output follows the __END__ separator.)
p Benchmark::Tms.new(1.5, 0.25, 0.0, 0.0, 2.0, "x")
class Timer
  include Benchmark
  def run = measure("in a class") { 1 }.label
end
p Timer.new.run

__END__
#<Benchmark::Tms @utime=1.5, @stime=0.25, @cutime=0.0, @cstime=0.0, @real=2.0, @label="x", @total=1.75>
"in a class"
