# proc field: testcase to demonstrate that a proc field is a closure in the
# message
#
# The class is a message whose field is a cameo.Closure, and an instance reads
# back from its message with a proc that runs the same code over the values it
# captured.
#
# proc fields are cameo's own; these outputs are their specification.
#
# (Introduced in PR #355. expected output follows the __END__ separator.)
class Route
  string :path
  proc :handler
end
class App
  list :routes, of: Route
end
puts Route.proto, App.proto
k = 10
r = Route.new(path: "/a", handler: proc { |x| x * k })
back = Route.from_proto(r.to_proto)
p back.path, back.handler.call(4), back.handler.lambda?
p Route.from_proto(Route.new(path: "/n").to_proto)

__END__
message Route {
  optional string path = 36596;
  optional cameo.Closure handler = 55264;
}
message App {
  repeated Route routes = 23606;
}
"/a"
40
false
Route(path: "/n", handler: nil)
