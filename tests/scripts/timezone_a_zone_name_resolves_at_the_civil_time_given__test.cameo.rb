# timezone: testcase to demonstrate that a zone name resolves at the civil
# time given
#
# 02:30 on 2024-03-31 does not exist in Berlin: the clocks go from 02:00 to
# 03:00. DateTime moves forward by the hour, as Time does. A Julian date is
# placed by its Gregorian day: Julian 2024-03-20 is Gregorian 2024-04-02,
# after the change. A zone that is UTC under another name is UTC.
#
# An IANA zone name is accepted wherever Ruby takes a UTC offset; Ruby takes a
# zone object there, so these are cameo's own and have no Ruby output to
# match. The instants are fixed, so the output does not depend on the
# machine's zone or clock.
#
# (Introduced in PR #265. expected output follows the __END__ separator.)
require "date"
p DateTime.new(2024, 3, 31, 2, 30, 0, "Europe/Berlin"), Time.new(2024, 3, 31, 2, 30, 0, "Europe/Berlin")
p DateTime.new(2024, 3, 20, 12, 0, 0, "Europe/Berlin", Date::JULIAN)
t = Time.at(0, in: "Etc/UTC")
p t, t.utc?, DateTime.new(2024, 7, 1, 12, 0, 0, "Etc/UTC").zone

__END__
#<DateTime: 2024-03-31T03:30:00+02:00 ((2460401j,5400s,0n),+7200s,2299161j)>
2024-03-31 03:30:00 +0200
#<DateTime: 2024-03-20T12:00:00+02:00 ((2460403j,36000s,0n),+7200s,Infj)>
1970-01-01 00:00:00 UTC
true
"+00:00"
