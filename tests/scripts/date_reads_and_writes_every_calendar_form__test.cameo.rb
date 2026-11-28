# date: testcase to demonstrate that date reads and writes every calendar form
#
# ajd is (4920739/2), amjd (60369/1) and day_fraction 0 in Ruby.
#
# The scripts are plain Ruby, and the expected outputs are Ruby 3.4's, with
# date 3.4.1, except where a comment says that Ruby returns a Rational: cameo
# returns an Integer when it is whole and a Float otherwise.
#
# (Introduced in PR #264. expected output follows the __END__ separator.)
require "date"
d = Date.new(2024, 2, 29)
p [d.jd, d.ajd, d.mjd, d.ld, d.amjd, d.day_fraction, d.start]
p [Date.new(1582, 10, 4) + 1, Date.new(1582, 10, 15) - 1, Date.new(2024, 2, 29, Date::JULIAN), Date.new(1000, 1, 1).julian?]
p [d.new_start(Date::JULIAN), d.england, Date.new(1752, 9, 2, Date::ENGLAND) + 1, Date.new(2024, 2, 29).gregorian.start]
p [Date.new(2024, 12, 30).cweek, Date.new(2024, 12, 30).cwyear, Date.new(2021, 1, 3).cweek, Date.new(2021, 1, 3).cwyear, Date.new(2021, 1, 4).cweek]
p [Date.jd(0), Date.ordinal(2024, -1), Date.commercial(2020, 53, 7), Date.commercial(2024, -1, 1), Date.valid_ordinal?(2023, 366), Date.valid_jd?(5)]
p d.strftime("%G-W%V-%u %-y %_y %^B %#A %-j %5j %-e %10s"), Date.new(-4712, 1, 1).to_s, Date.new(12, 2, 29).to_s

__END__
[2460370, 2460369.5, 60369, 161210, 60369, 0, 2299161.0]
[#<Date: 1582-10-15 ((2299161j,0s,0n),+0s,2299161j)>, #<Date: 1582-10-04 ((2299160j,0s,0n),+0s,2299161j)>, #<Date: 2024-02-29 ((2460383j,0s,0n),+0s,Infj)>, true]
[#<Date: 2024-02-16 ((2460370j,0s,0n),+0s,Infj)>, #<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2361222j)>, #<Date: 1752-09-14 ((2361222j,0s,0n),+0s,2361222j)>, -Infinity]
[1, 2025, 53, 2020, 1]
[#<Date: -4712-01-01 ((0j,0s,0n),+0s,2299161j)>, #<Date: 2024-12-31 ((2460676j,0s,0n),+0s,2299161j)>, #<Date: 2021-01-03 ((2459218j,0s,0n),+0s,2299161j)>, #<Date: 2024-12-23 ((2460668j,0s,0n),+0s,2299161j)>, false, true]
"2024-W09-4 24 24 FEBRUARY THURSDAY 60 00060 29 1709164800"
"-4712-01-01"
"0012-02-29"
