package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected values are Ruby's, with tsort 0.2.0.

func TestTSortSortsAndFindsComponentsAsRubyDoes(t *testing.T) {
	out := runScript(t, `require "tsort"
h = {1 => [2, 3], 2 => [3], 3 => [], 4 => [1]}
each_node = ->(&b) { h.each_key(&b) }
each_child = ->(n, &b) { h[n].each(&b) }
p TSort.tsort(each_node, each_child), TSort.strongly_connected_components(each_node, each_child), TSort.tsort_each(each_node, each_child).to_a, TSort.each_strongly_connected_component(each_node, each_child).to_a, TSort.each_strongly_connected_component_from(1, each_child).to_a
g = {1 => [2], 2 => [3, 4], 3 => [2], 4 => [], 5 => [5], 6 => []}
gn = ->(&b) { g.each_key(&b) }
gc = ->(n, &b) { g[n].each(&b) }
p TSort.strongly_connected_components(gn, gc), TSort.each_strongly_connected_component_from(2, gc).to_a, TSort.tsort(->(&b) {}, gc), TSort.tsort(->(&b) { [1, 2].each(&b) }, ->(n, &b) { [].each(&b) }), TSort.tsort(->(&b) { [1].each(&b) }, ->(n, &b) { [2].each(&b) }), TSort.tsort(->(&b) { {"a" => 1}.each_key(&b) }, ->(n, &b) { [].each(&b) })
begin; TSort.tsort(gn, gc); rescue TSort::Cyclic => e; p e.class, e.message; end
TSort.tsort_each(each_node, each_child) { |n| print n, " " }
TSort.each_strongly_connected_component(gn, gc) { |c| print c.inspect, " " }
puts
H = h
G = g
class Deps
  include TSort
  string :which
  def initialize(which)
    self.which = which
  end
  def graph
    which == "h" ? H : G
  end
  def tsort_each_node(&b)
    graph.each_key(&b)
  end
  def tsort_each_child(n, &b)
    graph[n].each(&b)
  end
end
d = Deps.new("h")
p d.tsort, d.strongly_connected_components, d.tsort_each.to_a, d.each_strongly_connected_component.to_a, d.each_strongly_connected_component_from(1).to_a, Deps.new("g").strongly_connected_components
begin; Deps.new("g").tsort; rescue TSort::Cyclic => e; puts e.message; end
p TSort::VERSION, TSort::Cyclic.superclass, Deps.include?(TSort), d.is_a?(TSort)
`)
	assert.Equals(t, out, `[3, 2, 1, 4]
[[3], [2], [1], [4]]
[3, 2, 1, 4]
[[3], [2], [1], [4]]
[[3], [2], [1]]
[[4], [2, 3], [1], [5], [6]]
[[4], [2, 3]]
[]
[1, 2]
[2, 1]
["a"]
TSort::Cyclic
"topological sort failed: [2, 3]"
3 2 1 4 [4] [2, 3] [1] [5] [6] 
[3, 2, 1, 4]
[[3], [2], [1], [4]]
[3, 2, 1, 4]
[[3], [2], [1], [4]]
[[3], [2], [1]]
[[4], [2, 3], [1], [5], [6]]
topological sort failed: [2, 3]
"0.2.0"
StandardError
true
true
`)
}
