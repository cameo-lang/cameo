# date: testcase to demonstrate that date fixed formats and strptime
#
# The scripts are plain Ruby, and the expected outputs are Ruby 3.4's, with
# date 3.4.1, except where a comment says that Ruby returns a Rational: cameo
# returns an Integer when it is whole and a Float otherwise.
#
# (Introduced in PR #264. expected output follows the __END__ separator.)
require "date"
p [Date.iso8601("20240229"), Date.iso8601("2024-060"), Date.rfc3339("2024-02-29T10:11:12+09:00"), Date.xmlschema("2024-02-29"), Date.jisx0301("H36.02.29")]
["2024/02/29", "x"].each do |s|
  Date.iso8601(s)
rescue Date::Error => e
  puts e.message
end
p Date._iso8601("2024-02-29T10:11:12+09:00"), Date._rfc2822("Thu, 29 Feb 2024 10:00:00 +0900"), Date._httpdate("Thu, 29 Feb 2024 10:00:00 GMT"), Date._jisx0301("H36.02.29")
p [Date.strptime("2024-02-29", "%Y-%m-%d"), Date.strptime("Thu Feb 29 2024", "%a %b %d %Y"), Date.strptime("2024-02-29T10:11:12", "%FT%T")]
["2024-02-30", "x"].each do |s|
  Date.strptime(s, "%Y-%m-%d")
rescue Date::Error => e
  puts e.message
end
p Date._strptime("2024-02-29", "%Y-%m-%d"), DateTime._strptime("04:05 PM", "%I:%M %p")
p [DateTime.rfc2822("Sat, 3 Feb 2001 04:05:06 +0700"), DateTime.httpdate("Sat, 03 Feb 2001 04:05:06 GMT"), DateTime.jisx0301("H13.02.03T04:05:06+07:00"), DateTime.strptime("1709164800", "%s"), DateTime.strptime("04:05 PM", "%I:%M %p").hour]

__END__
[#<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>, #<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>, #<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>, #<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>, #<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>]
invalid date
invalid date
{year: 2024, mon: 2, mday: 29, hour: 10, min: 11, sec: 12, zone: "+09:00", offset: 32400}
{wday: 4, mday: 29, mon: 2, year: 2024, hour: 10, min: 0, sec: 0, zone: "+0900", offset: 32400}
{wday: 4, mday: 29, mon: 2, year: 2024, hour: 10, min: 0, sec: 0, zone: "GMT", offset: 0}
{year: 2024, mon: 2, mday: 29}
[#<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>, #<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>, #<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>]
invalid date
invalid date
{year: 2024, mon: 2, mday: 29}
{hour: 16, min: 5}
[#<DateTime: 2001-02-03T04:05:06+07:00 ((2451943j,75906s,0n),+25200s,2299161j)>, #<DateTime: 2001-02-03T04:05:06+00:00 ((2451944j,14706s,0n),+0s,2299161j)>, #<DateTime: 2001-02-03T04:05:06+07:00 ((2451943j,75906s,0n),+25200s,2299161j)>, #<DateTime: 2024-02-29T00:00:00+00:00 ((2460370j,0s,0n),+0s,2299161j)>, 16]
