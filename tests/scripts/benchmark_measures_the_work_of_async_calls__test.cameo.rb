# benchmark: testcase to demonstrate that benchmark measures the work of async
# calls
#
# The CPU times are the process's, so they include what workers ran while the
# block waited for them.
#
# The scripts are plain Ruby, and the expected outputs are Ruby 3.4's, with
# benchmark 0.4.0.
#
# times are a time Benchmark measured, which differs from run to run.
#
# (Introduced in PR #114. expected output follows the __END__ separator.)
include Cameo
class Spin
  def go(n) = (1..n).sum { |i| i * i }
end
t = Benchmark.measure { await((1..4).map { async(Spin.new, :go, 300_000) }) }
p t.real > 0, t.utime + t.stime > 0

__END__
true
true
