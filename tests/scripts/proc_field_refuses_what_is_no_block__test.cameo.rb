# proc field: testcase to demonstrate that a proc field refuses what is no
# block and is nil when not given
#
# proc fields are cameo's own; these outputs are their specification. What the
# checker refuses, a default and a proc field outside a class body, is checked
# in tests/proc_field_test.go.
#
# (Introduced in PR #248. expected output follows the __END__ separator.)
class Route
  proc :handler
end
begin
  Route.new(handler: "x")
rescue TypeError => e
  puts e.message
end
p Route.new.handler

__END__
field handler of Route takes a Proc, not an instance of String
nil
