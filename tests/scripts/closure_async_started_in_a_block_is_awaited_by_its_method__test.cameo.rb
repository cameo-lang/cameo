# closure: testcase to demonstrate that async started in a block is awaited by
# its method
#
# A block captures the values of its variables when it is made, not the
# variables: these outputs are the specification. A rebinding after the block
# was made is invisible to it, a rebinding inside it is refused by the checker
# (src/syntax/syntax_test.go), its own variables are its own, and a lambda may
# call itself by the name it is assigned to.
#
# (Introduced in PR #247. expected output follows the __END__ separator.)
include Cameo
class W
  def sq(i) = i * i
  def fan(list)
    list.each { |i| async(self, :sq, i) }
    await :all
  end
end
p W.new.fan([1, 2, 3])

__END__
[1, 4, 9]
