# date: testcase to demonstrate that date and time convert
#
# sec_fraction is (1/4) and offset (0/1) in Ruby.
#
# The scripts are plain Ruby, and the expected outputs are Ruby 3.4's, with
# date 3.4.1, except where a comment says that Ruby returns a Rational: cameo
# returns an Integer when it is whole and a Float otherwise.
#
# (Introduced in PR #264. expected output follows the __END__ separator.)
require "date"
t = Time.utc(2024, 2, 29, 10, 11, 12.25)
p [t.to_date, t.to_datetime, t.to_datetime.sec_fraction, t.to_datetime.offset, Time.utc(2024, 2, 29, 23).to_date, t.to_time == t]
dt = DateTime.new(2001, 2, 3, 4, 5, 6.5, "+07:00")
p [dt.to_time, dt.to_time.utc, dt.to_time.utc_offset, dt.to_date, Date.new(2024, 2, 29).to_datetime, Date.new(2024, 2, 29).to_time.hour]
p [Date.today.class, DateTime.now.class, DateTime.now.offset.class, {Date.new(2024, 2, 29) => 1}[Date.new(2024, 2, 29)]]

__END__
[#<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>, #<DateTime: 2024-02-29T10:11:12+00:00 ((2460370j,36672s,250000000n),+0s,2299161j)>, 0.25, 0, #<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>, true]
[2001-02-03 04:05:06.5 +0700, 2001-02-02 21:05:06.5 UTC, 25200, #<Date: 2001-02-03 ((2451944j,0s,0n),+0s,2299161j)>, #<DateTime: 2024-02-29T00:00:00+00:00 ((2460370j,0s,0n),+0s,2299161j)>, 0]
[Date, DateTime, Integer, 1]
