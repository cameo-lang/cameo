# closure: testcase to demonstrate that a block's own rebinding lasts for its
# call only
#
# An assignment inside a block to a name it does not capture is the call's
# own, so each call starts from the captured values.
#
# A block captures the values of its variables when it is made, not the
# variables: these outputs are the specification. A rebinding after the block
# was made is invisible to it, a rebinding inside it is refused by the checker
# (src/syntax/syntax_test.go), its own variables are its own, and a lambda may
# call itself by the name it is assigned to.
#
# (Introduced in PR #247. expected output follows the __END__ separator.)
n = 1
l = -> { m = n + 1; m += 1; m }
puts l.call, l.call, n
def f
  s = 0
  3.times { |i| t = s + i; puts t }
  s
end
puts f

__END__
3
3
1
0
1
2
0
