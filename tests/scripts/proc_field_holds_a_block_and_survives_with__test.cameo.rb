# proc field: testcase to demonstrate that proc field holds a block and
# survives with
#
# proc fields are cameo's own; these outputs are their specification.
#
# (Introduced in PR #248. expected output follows the __END__ separator.)
class Route
  string :path
  proc :handler
end
r = Route.new(path: "/hi", handler: proc { |x| "hi #{x}" })
puts r.handler.call("you")
r2 = r.with(path: "/bye")
puts r2.handler.call("bye"), r2.path
puts r == r.with(path: "/hi"), r == r2
p r

__END__
hi you
hi bye
/bye
true
false
Route(path: "/hi", handler: #<Proc:proc_field_holds_a_block_and_survives_with__test.cameo.rb:11>)
