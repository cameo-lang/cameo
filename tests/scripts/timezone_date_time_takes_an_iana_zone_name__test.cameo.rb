# timezone: testcase to demonstrate that date time takes an IANA zone name
#
# An IANA zone name is accepted wherever Ruby takes a UTC offset; Ruby takes a
# zone object there, so these are cameo's own and have no Ruby output to
# match. The instants are fixed, so the output does not depend on the
# machine's zone or clock.
#
# (Introduced in PR #265. expected output follows the __END__ separator.)
require "date"
summer = DateTime.new(2024, 7, 1, 12, 0, 0, "Europe/Berlin")
winter = DateTime.new(2024, 1, 15, 12, 0, 0, "Europe/Berlin")
p summer, winter, summer.zone, winter.zone
p DateTime.new(2024, 7, 1, 12, 0, 0, "+07:00").new_offset("America/New_York")
p DateTime.jd(2460493, 12, 0, 0, "Asia/Tokyo"), DateTime.ordinal(2024, 183, 12, 0, 0, "Australia/Sydney")
begin; DateTime.new(2024, 7, 1, 12, 0, 0, "Europe/Nowhere"); rescue ArgumentError => e; puts e.message; end

__END__
#<DateTime: 2024-07-01T12:00:00+02:00 ((2460493j,36000s,0n),+7200s,2299161j)>
#<DateTime: 2024-01-15T12:00:00+01:00 ((2460325j,39600s,0n),+3600s,2299161j)>
"+02:00"
"+01:00"
#<DateTime: 2024-07-01T01:00:00-04:00 ((2460493j,18000s,0n),-14400s,2299161j)>
#<DateTime: 2024-07-01T12:00:00+09:00 ((2460493j,10800s,0n),+32400s,2299161j)>
#<DateTime: 2024-07-01T12:00:00+10:00 ((2460493j,7200s,0n),+36000s,2299161j)>
unknown time zone Europe/Nowhere: an IANA name such as Europe/Berlin, or an offset such as +01:00, expected
