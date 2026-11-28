# closure: testcase to demonstrate that a lambda calls itself by its name
#
# A block captures the values of its variables when it is made, not the
# variables: these outputs are the specification. A rebinding after the block
# was made is invisible to it, a rebinding inside it is refused by the checker
# (src/syntax/syntax_test.go), its own variables are its own, and a lambda may
# call itself by the name it is assigned to.
#
# (Introduced in PR #247. expected output follows the __END__ separator.)
fact = ->(n) { n <= 1 ? 1 : n * fact.(n - 1) }
puts fact.(5)
fib = proc { |n| n < 2 ? n : fib.call(n - 1) + fib.call(n - 2) }
puts fib.call(10)
walk = ->(n) { n == 0 ? [] : [n] + [n - 1].flat_map { |c| walk.(c) } }
p walk.(3)
x = x
p x

__END__
120
55
[3, 2, 1]
nil
