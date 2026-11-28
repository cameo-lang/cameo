# date: testcase to demonstrate that date reads what Ruby reads
#
# This test holds the forms the review of the library found read otherwise
# than Ruby 3.4.1 does: the readers with no string, an endless range of dates,
# jisx0301 on an ISO date, a year with a weekday alone, a fraction of the day
# on civil against jd and ordinal, and an offset out of range. A day's
# fraction is a Float where Ruby's is a Rational, and _jisx0301 gives the
# fragments of an ISO date in the date's order where Ruby's gives the day
# first.
#
# The scripts are plain Ruby, and the expected outputs are Ruby 3.4's, with
# date 3.4.1, except where a comment says that Ruby returns a Rational: cameo
# returns an Integer when it is whole and a Float otherwise.
#
# (Introduced in PR #264. expected output follows the __END__ separator.)
require "date"
p Date.parse, DateTime.strptime, Date.iso8601, Date.rfc2822, Date.httpdate, Date.jisx0301, DateTime.rfc2822
p (Date.new(2024,1,1)..).first(2), (Date.new(2024,1,1)..).each { |d| break d }
p Date.jisx0301("2024-02-29"), Date.strptime("2024 Mon", "%Y %a"), Date.strptime("2024 Tue", "%Y %a")
p Date.new(2024,1,1.5), Date.new(2024,1,1.5) == Date.new(2024,1,1), Date.new(2024,1,1.5).day_fraction, Date.commercial(2024,1,1.5).day_fraction, Date.jd(2460311.5).day_fraction, Date.ordinal(2024, 1.5).day_fraction
p DateTime.new(2024,1,1,0,0,0,2), DateTime.new(2024,1,1,0,0,0,"+02:00")
p Date._jisx0301("H31.04.30"), Date._jisx0301("2024-02-29")

__END__
#<Date: -4712-01-01 ((0j,0s,0n),+0s,2299161j)>
#<DateTime: -4712-01-01T00:00:00+00:00 ((0j,0s,0n),+0s,2299161j)>
#<Date: -4712-01-01 ((0j,0s,0n),+0s,2299161j)>
#<Date: -4712-01-01 ((0j,0s,0n),+0s,2299161j)>
#<Date: -4712-01-01 ((0j,0s,0n),+0s,2299161j)>
#<Date: -4712-01-01 ((0j,0s,0n),+0s,2299161j)>
#<DateTime: -4712-01-01T00:00:00+00:00 ((0j,0s,0n),+0s,2299161j)>
[#<Date: 2024-01-01 ((2460311j,0s,0n),+0s,2299161j)>, #<Date: 2024-01-02 ((2460312j,0s,0n),+0s,2299161j)>]
#<Date: 2024-01-01 ((2460311j,0s,0n),+0s,2299161j)>
#<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>
#<Date: 2024-01-01 ((2460311j,0s,0n),+0s,2299161j)>
#<Date: 2024-01-02 ((2460312j,0s,0n),+0s,2299161j)>
#<Date: 2024-01-01 ((2460311j,0s,0n),+0s,2299161j)>
true
0
0.5
0.5
0.5
#<DateTime: 2024-01-01T00:00:00+00:00 ((2460311j,0s,0n),+0s,2299161j)>
#<DateTime: 2024-01-01T00:00:00+02:00 ((2460310j,79200s,0n),+7200s,2299161j)>
{year: 2019, mon: 4, mday: 30}
{year: 2024, mon: 2, mday: 29}
