# proc field: testcase to demonstrate that a proc field that cannot travel is
# named
#
# A proc that captured what the envelope cannot carry does not travel: the
# instance that holds it in a field cannot cross, and the error says which
# field and why.
#
# proc fields are cameo's own; these outputs are their specification.
#
# (Introduced in PR #355. expected output follows the __END__ separator.)
class Route
  proc :handler
end
include Cameo
class W
  def run(route) = 1
end
dice = Random.new(1)
r = Route.new(handler: proc { dice.rand })
[-> { r.to_proto }, -> { r.to_value_proto }, -> { async(W.new, :run, r) }].each do |f|
  f.call
rescue CameoError, TypeError => e
  puts e.message
end

__END__
Route.handler: a Proc that captured dice, a Random cannot be serialised
Route.handler: a Proc that captured dice, a Random cannot be serialised
a Proc that captured dice, a Random cannot be passed to async
