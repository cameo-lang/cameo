# timezone: testcase to demonstrate that a named zone crosses to a worker by
# its name
#
# A time in a named zone comes back from async in that zone, so that its
# daylight saving holds six months on; a fixed offset stays fixed.
#
# An IANA zone name is accepted wherever Ruby takes a UTC offset; Ruby takes a
# zone object there, so these are cameo's own and have no Ruby output to
# match. The instants are fixed, so the output does not depend on the
# machine's zone or clock.
#
# (Introduced in PR #265. expected output follows the __END__ separator.)
include Cameo
class W
  def back(t) = t
end
[Time.new(2024, 1, 15, 12, 0, 0, in: "Europe/Berlin"), Time.new(2024, 1, 15, 12, 0, 0, in: "+01:00")].each do |t|
  u = await(W.new, :back, t)
  puts [u.zone.inspect, u.to_s, (u + 180 * 86400).to_s].join(" | ")
end

__END__
"CET" | 2024-01-15 12:00:00 +0100 | 2024-07-13 13:00:00 +0200
nil | 2024-01-15 12:00:00 +0100 | 2024-07-13 12:00:00 +0100
