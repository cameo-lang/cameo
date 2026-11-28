# closure: testcase to demonstrate that a block captures values when it is
# made
#
# A block captures the values of its variables when it is made, not the
# variables: these outputs are the specification. A rebinding after the block
# was made is invisible to it, a rebinding inside it is refused by the checker
# (src/syntax/syntax_test.go), its own variables are its own, and a lambda may
# call itself by the name it is assigned to.
#
# (Introduced in PR #247. expected output follows the __END__ separator.)
a = 3
l = proc do a end
kept = { fn: l }
a = 4
puts l.call, kept[:fn].call
greeting = "hi"
routes = [-> { greeting }]
greeting = "bye"
puts routes[0].call, greeting
def make
  b = 5
  proc { b }
end
puts make.call
counter = 3.times.reduce(0) { |c, _| c + 1 }
puts counter
[1].each { |i| own = i + 10; puts own }
puts defined?(own).inspect
x = 2
puts [1].map { |i| y = x + i; y }.inspect, x

__END__
3
3
hi
bye
5
3
11
nil
[3]
2
