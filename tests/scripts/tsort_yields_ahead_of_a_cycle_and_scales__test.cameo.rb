# tsort: testcase to demonstrate that t sort yields ahead of a cycle and
# scales
#
# tsort_each yields every node ahead of a cycle before raising, a search from
# a node returns its minimum index, carried id_map and stack are refused, a
# class that includes TSort without the two methods gets NotImplementedError,
# and a chain of sixteen thousand nodes sorts in one pass. The answers are
# Ruby 3.4.1's.
#
# The expected values are Ruby's, with tsort 0.2.0.
#
# (Introduced in PR #278. expected output follows the __END__ separator.)
require "tsort"
g = {1 => [2], 2 => [3, 4], 3 => [2], 4 => []}
gn = ->(&b) { g.each_key(&b) }
gc = ->(n, &b) { g[n].each(&b) }
begin; TSort.tsort_each(gn, gc) { |n| print n, " " }; rescue TSort::Cyclic => e; puts "| #{e.message}"; end
p TSort.each_strongly_connected_component_from(1, gc) { |c| }, TSort.each_strongly_connected_component_from(4, gc) { |c| }
begin; TSort.each_strongly_connected_component_from(4, gc, {}, []) { |c| p c }; rescue ArgumentError => e; puts e.message; end
class Bare
  include TSort
end
begin; Bare.new.tsort; rescue NotImplementedError => e; puts e.message; end
n = 16000
chain = ->(i, &b) { (i + 1 < n ? [i + 1] : []).each(&b) }
p TSort.tsort(->(&b) { (0...n).each(&b) }, chain).first(3)

__END__
4 | topological sort failed: [2, 3]
0
0
id_map and stack cannot be carried between calls: each call searches afresh
NotImplementedError
[15999, 15998, 15997]
