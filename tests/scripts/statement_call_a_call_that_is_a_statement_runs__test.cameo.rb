# statement call: testcase to demonstrate that a call that is a statement runs
#
# A call that is a statement, whose value nothing reads, runs as any call.
#
# (Introduced in PR #394. expected output follows the __END__ separator.)
def twice(v) = v * 2
def noisy(v)
  puts "noisy #{v}"
  v
end
twice 2
noisy 3
noisy(4)

__END__
noisy 3
noisy 4
