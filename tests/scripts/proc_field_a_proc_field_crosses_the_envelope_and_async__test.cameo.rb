# proc field: testcase to demonstrate that a proc field crosses the envelope
# and async
#
# Through a list, a map value, a set and an inherited field; a worker returns
# an instance with a proc and takes one.
#
# proc fields are cameo's own; these outputs are their specification.
#
# (Introduced in PR #355. expected output follows the __END__ separator.)
class Route
  string :path
  proc :handler
end
class Sub < Route
end
class App
  list :routes, of: Route
  map :named, key: :string, value: Route
  set :pool, of: Route
end
include Cameo
class W
  def make(n) = Route.new(path: "/w", handler: proc { |x| x + n })
  def run(route, x) = route.handler.call(x)
end
r = Route.new(path: "/a", handler: proc { |x| x * 10 })
s = Sub.new(path: "/s", handler: ->(x) { x - 1 })
app = App.new(routes: [r, s], named: {"a" => r}, pool: Set.new([s]))
again = from_value_proto(app.to_value_proto)
p again.routes.map { |x| x.handler.call(5) }, again.named["a"].handler.call(2)
p again.pool.to_a[0].handler.call(1), again.routes[1].handler.lambda?
sub = from_value_proto(s.to_value_proto)
p sub.class, sub.handler.call(3)
p (await async(W.new, :make, 3)).handler.call(4)
p await async(W.new, :run, r, 6)

__END__
[50, 4]
20
0
true
Sub
2
7
60
