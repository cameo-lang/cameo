# date: testcase to demonstrate that date arithmetic is Ruby's
#
# Ruby: (59/1), Rational; (-20987/172800), Rational; (1/2); (7/24).
#
# The scripts are plain Ruby, and the expected outputs are Ruby 3.4's, with
# date 3.4.1, except where a comment says that Ruby returns a Rational: cameo
# returns an Integer when it is whole and a Float otherwise.
#
# (Introduced in PR #264. expected output follows the __END__ separator.)
require "date"
d = Date.new(2024, 2, 29)
p [d - Date.new(2024, 1, 1), (d - Date.new(2024, 1, 1)).class, d + 1.5, d - 0.5, d.next_day(1.5)]
p [d >> 1, d << 1, d >> 12, d << 12, d.next_month(13), d.prev_month, Date.new(2024, 1, 31) >> 1, Date.new(2023, 1, 31) >> 1]
dt = DateTime.new(2001, 2, 3, 4, 5, 6.5, "+07:00")
p [dt + 0.5, dt - 1.25, dt - DateTime.new(2001, 2, 3), (dt - DateTime.new(2001, 2, 3)).class, dt.sec_fraction, dt.offset]
p [d <=> 2460369.5, d == 1, Date.new(2024, 3, 1) > d, d.between?(Date.new(2024, 1, 1), Date.new(2024, 12, 31)), d.clamp(Date.new(2024, 3, 1), Date.new(2024, 3, 5))]

__END__
[59, Integer, #<Date: 2024-03-01 ((2460371j,43200s,0n),+0s,2299161j)>, #<Date: 2024-02-28 ((2460369j,43200s,0n),+0s,2299161j)>, #<Date: 2024-03-01 ((2460371j,43200s,0n),+0s,2299161j)>]
[#<Date: 2024-03-29 ((2460399j,0s,0n),+0s,2299161j)>, #<Date: 2024-01-29 ((2460339j,0s,0n),+0s,2299161j)>, #<Date: 2025-02-28 ((2460735j,0s,0n),+0s,2299161j)>, #<Date: 2023-02-28 ((2460004j,0s,0n),+0s,2299161j)>, #<Date: 2025-03-29 ((2460764j,0s,0n),+0s,2299161j)>, #<Date: 2024-01-29 ((2460339j,0s,0n),+0s,2299161j)>, #<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>, #<Date: 2023-02-28 ((2460004j,0s,0n),+0s,2299161j)>]
[#<DateTime: 2001-02-03T16:05:06+07:00 ((2451944j,32706s,500000000n),+25200s,2299161j)>, #<DateTime: 2001-02-01T22:05:06+07:00 ((2451942j,54306s,500000000n),+25200s,2299161j)>, -0.12145254629629632, Float, 0.5, 0.2916666666666667]
[0, false, true, true, #<Date: 2024-03-01 ((2460371j,0s,0n),+0s,2299161j)>]
