# closure: testcase to demonstrate that a for loop and a rescue bind in the
# scope around them
#
# A block captures the values of its variables when it is made, not the
# variables: these outputs are the specification. A rebinding after the block
# was made is invisible to it, a rebinding inside it is refused by the checker
# (src/syntax/syntax_test.go), its own variables are its own, and a lambda may
# call itself by the name it is assigned to.
#
# (Introduced in PR #247. expected output follows the __END__ separator.)
total = 0
for x in [1, 2, 3]
  total += x
end
p total, x
e = :none
[1].each do
  raise "inner"
rescue => e
  puts e.message
end
p e

__END__
6
3
inner
:none
