package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The scripts are plain Ruby, and the expected outputs are Ruby 3.4's,
// with date 3.4.1, except where a comment says that Ruby returns a
// Rational: cameo returns an Integer when it is whole and a Float
// otherwise.

func TestDateReadsAndWritesEveryCalendarForm(t *testing.T) {
	out := runScript(t, `require "date"
d = Date.new(2024, 2, 29)
p [d.jd, d.ajd, d.mjd, d.ld, d.amjd, d.day_fraction, d.start]
p [Date.new(1582, 10, 4) + 1, Date.new(1582, 10, 15) - 1, Date.new(2024, 2, 29, Date::JULIAN), Date.new(1000, 1, 1).julian?]
p [d.new_start(Date::JULIAN), d.england, Date.new(1752, 9, 2, Date::ENGLAND) + 1, Date.new(2024, 2, 29).gregorian.start]
p [Date.new(2024, 12, 30).cweek, Date.new(2024, 12, 30).cwyear, Date.new(2021, 1, 3).cweek, Date.new(2021, 1, 3).cwyear, Date.new(2021, 1, 4).cweek]
p [Date.jd(0), Date.ordinal(2024, -1), Date.commercial(2020, 53, 7), Date.commercial(2024, -1, 1), Date.valid_ordinal?(2023, 366), Date.valid_jd?(5)]
p d.strftime("%G-W%V-%u %-y %_y %^B %#A %-j %5j %-e %10s"), Date.new(-4712, 1, 1).to_s, Date.new(12, 2, 29).to_s
`)
	// ajd is (4920739/2), amjd (60369/1) and day_fraction 0 in Ruby.
	assert.Equals(t, out, `[2460370, 2460369.5, 60369, 161210, 60369, 0, 2299161.0]
[#<Date: 1582-10-15 ((2299161j,0s,0n),+0s,2299161j)>, #<Date: 1582-10-04 ((2299160j,0s,0n),+0s,2299161j)>, #<Date: 2024-02-29 ((2460383j,0s,0n),+0s,Infj)>, true]
[#<Date: 2024-02-16 ((2460370j,0s,0n),+0s,Infj)>, #<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2361222j)>, #<Date: 1752-09-14 ((2361222j,0s,0n),+0s,2361222j)>, -Infinity]
[1, 2025, 53, 2020, 1]
[#<Date: -4712-01-01 ((0j,0s,0n),+0s,2299161j)>, #<Date: 2024-12-31 ((2460676j,0s,0n),+0s,2299161j)>, #<Date: 2021-01-03 ((2459218j,0s,0n),+0s,2299161j)>, #<Date: 2024-12-23 ((2460668j,0s,0n),+0s,2299161j)>, false, true]
"2024-W09-4 24 24 FEBRUARY THURSDAY 60 00060 29 1709164800"
"-4712-01-01"
"0012-02-29"
`)
}

func TestDateArithmeticIsRubys(t *testing.T) {
	out := runScript(t, `require "date"
d = Date.new(2024, 2, 29)
p [d - Date.new(2024, 1, 1), (d - Date.new(2024, 1, 1)).class, d + 1.5, d - 0.5, d.next_day(1.5)]
p [d >> 1, d << 1, d >> 12, d << 12, d.next_month(13), d.prev_month, Date.new(2024, 1, 31) >> 1, Date.new(2023, 1, 31) >> 1]
dt = DateTime.new(2001, 2, 3, 4, 5, 6.5, "+07:00")
p [dt + 0.5, dt - 1.25, dt - DateTime.new(2001, 2, 3), (dt - DateTime.new(2001, 2, 3)).class, dt.sec_fraction, dt.offset]
p [d <=> 2460369.5, d == 1, Date.new(2024, 3, 1) > d, d.between?(Date.new(2024, 1, 1), Date.new(2024, 12, 31)), d.clamp(Date.new(2024, 3, 1), Date.new(2024, 3, 5))]
`)
	// Ruby: (59/1), Rational; (-20987/172800), Rational; (1/2); (7/24).
	assert.Equals(t, out, `[59, Integer, #<Date: 2024-03-01 ((2460371j,43200s,0n),+0s,2299161j)>, #<Date: 2024-02-28 ((2460369j,43200s,0n),+0s,2299161j)>, #<Date: 2024-03-01 ((2460371j,43200s,0n),+0s,2299161j)>]
[#<Date: 2024-03-29 ((2460399j,0s,0n),+0s,2299161j)>, #<Date: 2024-01-29 ((2460339j,0s,0n),+0s,2299161j)>, #<Date: 2025-02-28 ((2460735j,0s,0n),+0s,2299161j)>, #<Date: 2023-02-28 ((2460004j,0s,0n),+0s,2299161j)>, #<Date: 2025-03-29 ((2460764j,0s,0n),+0s,2299161j)>, #<Date: 2024-01-29 ((2460339j,0s,0n),+0s,2299161j)>, #<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>, #<Date: 2023-02-28 ((2460004j,0s,0n),+0s,2299161j)>]
[#<DateTime: 2001-02-03T16:05:06+07:00 ((2451944j,32706s,500000000n),+25200s,2299161j)>, #<DateTime: 2001-02-01T22:05:06+07:00 ((2451942j,54306s,500000000n),+25200s,2299161j)>, -0.12145254629629632, Float, 0.5, 0.2916666666666667]
[0, false, true, true, #<Date: 2024-03-01 ((2460371j,0s,0n),+0s,2299161j)>]
`)
}

func TestDateParseReadsRubysForms(t *testing.T) {
	out := runScript(t, `require "date"
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
`)
	// sec_fraction is (1/2) in Ruby.
	assert.Equals(t, out, `2024-02-09
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
`)
}

func TestDateFixedFormatsAndStrptime(t *testing.T) {
	out := runScript(t, `require "date"
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
`)
	assert.Equals(t, out, `[#<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>, #<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>, #<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>, #<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>, #<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>]
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
`)
}

func TestDateAndTimeConvert(t *testing.T) {
	out := runScript(t, `require "date"
t = Time.utc(2024, 2, 29, 10, 11, 12.25)
p [t.to_date, t.to_datetime, t.to_datetime.sec_fraction, t.to_datetime.offset, Time.utc(2024, 2, 29, 23).to_date, t.to_time == t]
dt = DateTime.new(2001, 2, 3, 4, 5, 6.5, "+07:00")
p [dt.to_time, dt.to_time.utc, dt.to_time.utc_offset, dt.to_date, Date.new(2024, 2, 29).to_datetime, Date.new(2024, 2, 29).to_time.hour]
p [Date.today.class, DateTime.now.class, DateTime.now.offset.class, {Date.new(2024, 2, 29) => 1}[Date.new(2024, 2, 29)]]
`)
	// sec_fraction is (1/4) and offset (0/1) in Ruby.
	assert.Equals(t, out, `[#<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>, #<DateTime: 2024-02-29T10:11:12+00:00 ((2460370j,36672s,250000000n),+0s,2299161j)>, 0.25, 0, #<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>, true]
[2001-02-03 04:05:06.5 +0700, 2001-02-02 21:05:06.5 UTC, 25200, #<Date: 2001-02-03 ((2451944j,0s,0n),+0s,2299161j)>, #<DateTime: 2024-02-29T00:00:00+00:00 ((2460370j,0s,0n),+0s,2299161j)>, 0]
[Date, DateTime, Integer, 1]
`)
}

// TestDateReadsWhatRubyReads holds the forms the review of the library
// found read otherwise than Ruby 3.4.1 does: the readers with no string,
// an endless range of dates, jisx0301 on an ISO date, a year with a
// weekday alone, a fraction of the day on civil against jd and ordinal,
// and an offset out of range. A day's fraction is a Float where Ruby's
// is a Rational, and _jisx0301 gives the fragments of an ISO date in the
// date's order where Ruby's gives the day first.
func TestDateReadsWhatRubyReads(t *testing.T) {
	out := runScript(t, `require "date"
p Date.parse, DateTime.strptime, Date.iso8601, Date.rfc2822, Date.httpdate, Date.jisx0301, DateTime.rfc2822
p (Date.new(2024,1,1)..).first(2), (Date.new(2024,1,1)..).each { |d| break d }
p Date.jisx0301("2024-02-29"), Date.strptime("2024 Mon", "%Y %a"), Date.strptime("2024 Tue", "%Y %a")
p Date.new(2024,1,1.5), Date.new(2024,1,1.5) == Date.new(2024,1,1), Date.new(2024,1,1.5).day_fraction, Date.commercial(2024,1,1.5).day_fraction, Date.jd(2460311.5).day_fraction, Date.ordinal(2024, 1.5).day_fraction
p DateTime.new(2024,1,1,0,0,0,2), DateTime.new(2024,1,1,0,0,0,"+02:00")
p Date._jisx0301("H31.04.30"), Date._jisx0301("2024-02-29")
`)
	assert.Equals(t, out, `#<Date: -4712-01-01 ((0j,0s,0n),+0s,2299161j)>
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
`)
}
