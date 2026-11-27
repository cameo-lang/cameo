# benchmark: testcase to demonstrate that benchmark tms computes and formats
# as Ruby's
#
# The scripts are plain Ruby, and the expected outputs are Ruby 3.4's, with
# benchmark 0.4.0.
#
# times are a time Benchmark measured, which differs from run to run.
#
# (Introduced in PR #114. expected output follows the __END__ separator.)
require "benchmark"
t = Benchmark::Tms.new(1.5, 0.25, 0.0, 0.0, 2.0, "x")
u = Benchmark::Tms.new(0.5, 0.75, 0.0, 0.0, 1.0)
p t.utime, t.stime, t.cutime, t.cstime, t.real, t.total, t.label
p t.to_a, t.to_h
print t.to_s, t
s = t + u
p s.to_a, (t - u).to_a, (t * 2).to_a, (t / 4).to_a
p Benchmark::Tms.new.to_a, Benchmark::Tms.new(1, 2).total
puts t.format("%n: %u user, %y sys, %t total, %r real")
puts t.format("%-5n|%.2u|%3.1r| %s", "extra")
puts Benchmark::CAPTION.inspect, Benchmark::FORMAT.inspect, Benchmark::VERSION
r = Benchmark.realtime { 1 + 1 }
p r.class, r >= 0
m = Benchmark.measure("lbl") { 1 }
p m.class, m.label, m.real >= 0
p t.add { 1 }.class

__END__
1.5
0.25
0.0
0.0
2.0
1.75
"x"
["x", 1.5, 0.25, 0.0, 0.0, 2.0]
{label: "x", utime: 1.5, stime: 0.25, cutime: 0.0, cstime: 0.0, real: 2.0}
  1.500000   0.250000   1.750000 (  2.000000)
  1.500000   0.250000   1.750000 (  2.000000)
["", 2.0, 1.0, 0.0, 0.0, 3.0]
["", 1.0, -0.5, 0.0, 0.0, 1.0]
["", 3.0, 0.5, 0.0, 0.0, 4.0]
["", 0.375, 0.0625, 0.0, 0.0, 0.5]
["", 0.0, 0.0, 0.0, 0.0, 0.0]
3.0
x: 1.500000 user, 0.250000 sys, 1.750000 total, (2.000000) real
x    |1.50|(2.0)| extra
"      user     system      total        real\n"
"%10.6u %10.6y %10.6t %10.6r\n"
0.4.0
Float
true
Benchmark::Tms
"lbl"
true
Benchmark::Tms
