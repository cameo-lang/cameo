# date: testcase to demonstrate that date parse reads Ruby's forms
#
# sec_fraction is (1/2) in Ruby.
#
# The scripts are plain Ruby, and the expected outputs are Ruby 3.4's, with
# date 3.4.1, except where a comment says that Ruby returns a Rational: cameo
# returns an Integer when it is whole and a Float otherwise.
#
# (Introduced in PR #264. expected output follows the __END__ separator.)
require "date"
["2024-2-9", "24-02-29", "2024 Feb", "Feb 2024", "29.02.2024", "2024-02-29 10:11:12 UTC", "Sat Aug 28 02:55:50 +09:00 1999",
 "20240229T1011", "990828", "S63.1.1", "M45.7.30", "R1.5.1", "19990828 02:55:50",
 "1999-08-28T02:55:50.123456789+09:00", "2024-02-29Z"].each { |s| puts Date.parse(s) }
["2024-02", "02/29/24", "2024 60", "1999", "99", "2:55:50 PM", "02:55pm", "1-1-99"].each do |s|
  Date.parse(s)
rescue Date::Error => e
  puts "#{s}: #{e.message}"
end
p Date.parse("12-02-29"), Date.parse("12-02-29", false), Date.parse("2024-02-29", true, Date::JULIAN).jd
p Date._parse("2024-02-29T10:11:12.5+09:00"), Date._parse("x"), Date._parse("10:00"), Date._parse("1-1-99", true), Date._parse("20240229T101112Z"), Date._parse("Thursday")

__END__
2024-02-09
2024-02-29
2024-02-01
2024-02-01
2024-02-29
2024-02-29
1999-08-28
2024-02-29
1999-08-28
1988-01-01
1912-07-30
2019-05-01
1999-08-28
1999-08-28
2024-02-29
2024-02: invalid date
02/29/24: invalid date
2024 60: invalid date
1999: invalid date
99: invalid date
2:55:50 PM: invalid date
02:55pm: invalid date
1-1-99: invalid date
#<Date: 2012-02-29 ((2455987j,0s,0n),+0s,2299161j)>
#<Date: 2012-02-29 ((2455987j,0s,0n),+0s,2299161j)>
2460383
{year: 2024, mon: 2, mday: 29, hour: 10, min: 11, sec: 12, sec_fraction: 0.5, zone: "+09:00", offset: 32400}
{}
{hour: 10, min: 0}
{}
{year: 2024, mon: 2, mday: 29, hour: 10, min: 11, sec: 12, zone: "Z", offset: 0}
{wday: 4}
