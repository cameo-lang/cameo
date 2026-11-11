package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// Ruby 3.4's Time.parse reads a zone name it knows at its offset (EST,
// PST, the military letters), any other name as the local zone, a
// 12-hour clock after a date, and refuses a weekday alone. Every line
// prints what Ruby prints under TZ=UTC.
func TestTimeParsesZoneNamesAndClocksAsRubyDoes(t *testing.T) {
	out := runScript(t, `require "time"
["2024-01-02 03:04:05 EST", "2024-01-02 03:04:05 PDT", "2024-01-02 03:04:05 UT", "2024-01-02 03:04:05 GMT",
 "2024-01-02 03:04:05 A", "2024-01-02 03:04:05 N", "Tue, 02 Jan 2024 03:04:05 PST",
 "Jan 5 2024 3:04 PM", "Jan 5 2024 3:04:05 pm", "January 5, 2024 11:30 am", "5 Jan 2024 3:04 PM",
 "Mon, 23 Aug 2021 09:57:02 GMT", "2024-01-02 03:04:05 +0530"].each do |s|
  t = Time.parse(s)
  puts "#{s} => #{t.strftime("%Y-%m-%d %H:%M:%S %z")} #{t.zone.inspect}"
end
begin; Time.parse("Sunday"); rescue ArgumentError => e; puts e.message; end
p Time.strptime("2018-091 EST", "%Y-%j %Z").utc_offset
`)
	assert.Equals(t, out, `2024-01-02 03:04:05 EST => 2024-01-02 03:04:05 -0500 nil
2024-01-02 03:04:05 PDT => 2024-01-02 03:04:05 -0700 nil
2024-01-02 03:04:05 UT => 2024-01-02 03:04:05 +0000 "UTC"
2024-01-02 03:04:05 GMT => 2024-01-02 03:04:05 +0000 "UTC"
2024-01-02 03:04:05 A => 2024-01-02 03:04:05 +0100 nil
2024-01-02 03:04:05 N => 2024-01-02 03:04:05 -0100 nil
Tue, 02 Jan 2024 03:04:05 PST => 2024-01-02 03:04:05 -0800 nil
Jan 5 2024 3:04 PM => 2024-01-05 15:04:00 +0000 "UTC"
Jan 5 2024 3:04:05 pm => 2024-01-05 15:04:05 +0000 "UTC"
January 5, 2024 11:30 am => 2024-01-05 11:30:00 +0000 "UTC"
5 Jan 2024 3:04 PM => 2024-01-05 15:04:00 +0000 "UTC"
Mon, 23 Aug 2021 09:57:02 GMT => 2021-08-23 09:57:02 +0000 "UTC"
2024-01-02 03:04:05 +0530 => 2024-01-02 03:04:05 +0530 nil
no time information in "Sunday"
-18000
`)
}

// Time.strptime reads %s, seconds since the epoch, with a fraction and a
// zone after it, and %z reads a zone name as well as an offset. Every
// line prints what CRuby 3.3 prints under TZ=UTC.
func TestTimeStrptimeReadsEpochSecondsAndZoneNames(t *testing.T) {
	out := runScript(t, `require "time"
[["1.5", "%s.%N"], ["-1.5", "%s.%N"], ["1.123", "%s.%L"], ["1.5 +0900", "%s.%N %z"], ["1709164800", "%s"],
 ["20010203 UTC", "%Y%m%d %z"], ["20010203 GMT", "%Y%m%d %z"], ["20010203 EST", "%Y%m%d %z"], ["20010203 +0900", "%Y%m%d %z"]].each do |s, f|
  t = Time.strptime(s, f)
  puts "#{s} => #{t.strftime("%Y-%m-%d %H:%M:%S.%L %z")}"
end
p [Time.strptime("20010203 UTC", "%Y%m%d %z").utc?, Time.strptime("20010203 EST", "%Y%m%d %z").utc?]
`)
	assert.Equals(t, out, `1.5 => 1970-01-01 00:00:01.500 +0000
-1.5 => 1969-12-31 23:59:58.500 +0000
1.123 => 1970-01-01 00:00:01.123 +0000
1.5 +0900 => 1970-01-01 09:00:01.500 +0900
1709164800 => 2024-02-29 00:00:00.000 +0000
20010203 UTC => 2001-02-03 00:00:00.000 +0000
20010203 GMT => 2001-02-03 00:00:00.000 +0000
20010203 EST => 2001-02-03 00:00:00.000 -0500
20010203 +0900 => 2001-02-03 00:00:00.000 +0900
[true, false]
`)
}
