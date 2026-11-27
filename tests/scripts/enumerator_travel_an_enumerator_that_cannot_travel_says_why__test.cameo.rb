# enumerator travel: testcase to demonstrate that an enumerator that cannot
# travel says why
#
# An Enumerator travels as the call that made it: these outputs are the
# specification of what reads back.
#
# (Introduced in PR #356. expected output follows the __END__ separator.)
require "stringio"
dice = Random.new(1)
[
  -> { StringIO.new("a\nb").each_line },
  -> { [dice].each },
  -> { [1, 2].each_with_object(dice) },
  -> { [1, 2].lazy.map { dice.rand } },
  -> { [1].each + [dice] },
].each do |f|
  f.call.to_value_proto
rescue TypeError => e
  puts e.message
end

__END__
an Enumerator cannot be serialised
an Enumerator of a Random cannot be serialised
an Enumerator with the argument a Random cannot be serialised
an Enumerator with a block, a Proc that captured dice, a Random cannot be serialised
an Enumerator that combines a Random cannot be serialised
