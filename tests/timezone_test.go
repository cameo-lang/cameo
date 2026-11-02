package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// An IANA zone name is accepted wherever Ruby takes a UTC offset; Ruby
// takes a zone object there, so these are cameo's own and have no Ruby
// output to match. The instants are fixed, so the output does not depend
// on the machine's zone or clock.

func TestTimeTakesAnIANAZoneName(t *testing.T) {
	out := runScript(t, `
t = Time.at(1719835200, in: "Europe/Berlin")
p t, t.zone, t.utc_offset, t.dst?, t.strftime("%Z %z %:z"), t == Time.at(1719835200)
w = Time.new(2024, 1, 15, 12, 0, 0, in: "America/New_York")
p w, w.zone, w.utc_offset, w.dst?, w.utc, w.to_a
p Time.utc(2024, 7, 1, 12).localtime("Asia/Tokyo"), Time.utc(2024, 7, 1, 12).getlocal("Asia/Kolkata").zone
p Time.new(2024, 3, 31, 1, 30, 0, "Europe/Berlin") + 3600, Time.new(2024, 10, 27, 4, 0, 0, in: "Europe/Berlin").utc_offset
p Time.now(in: "Etc/UTC").zone, Time.at(0, in: "+09:00").zone
`)
	assert.Equals(t, out, `2024-07-01 14:00:00 +0200
"CEST"
7200
true
"CEST +0200 +02:00"
true
2024-01-15 12:00:00 -0500
"EST"
-18000
false
2024-01-15 17:00:00 UTC
[0, 0, 12, 15, 1, 2024, 1, 15, false, "EST"]
2024-07-01 21:00:00 +0900
"IST"
2024-03-31 03:30:00 +0200
3600
"UTC"
nil
`)
}

func TestTimeRefusesAZoneNameItDoesNotKnow(t *testing.T) {
	out := runScript(t, `
begin; Time.at(0, in: "Europe/Nowhere"); rescue ArgumentError => e; puts e.message; end
begin; Time.at(0, in: "CET"); rescue ArgumentError => e; puts e.message; end
begin; Time.utc(2024, 1, 1).localtime("Mars/Olympus"); rescue ArgumentError => e; puts e.message; end
`)
	assert.Equals(t, out, `unknown time zone Europe/Nowhere: an IANA name such as Europe/Berlin, or an offset such as +01:00, expected for utc_offset
"+HH:MM", "-HH:MM", "UTC" or "A".."I","K".."Z" expected for utc_offset: CET
unknown time zone Mars/Olympus: an IANA name such as Europe/Berlin, or an offset such as +01:00, expected for utc_offset
`)
}

func TestDateTimeTakesAnIANAZoneName(t *testing.T) {
	out := runScript(t, `require "date"
summer = DateTime.new(2024, 7, 1, 12, 0, 0, "Europe/Berlin")
winter = DateTime.new(2024, 1, 15, 12, 0, 0, "Europe/Berlin")
p summer, winter, summer.zone, winter.zone
p DateTime.new(2024, 7, 1, 12, 0, 0, "+07:00").new_offset("America/New_York")
p DateTime.jd(2460493, 12, 0, 0, "Asia/Tokyo"), DateTime.ordinal(2024, 183, 12, 0, 0, "Australia/Sydney")
begin; DateTime.new(2024, 7, 1, 12, 0, 0, "Europe/Nowhere"); rescue ArgumentError => e; puts e.message; end
`)
	assert.Equals(t, out, `#<DateTime: 2024-07-01T12:00:00+02:00 ((2460493j,36000s,0n),+7200s,2299161j)>
#<DateTime: 2024-01-15T12:00:00+01:00 ((2460325j,39600s,0n),+3600s,2299161j)>
"+02:00"
"+01:00"
#<DateTime: 2024-07-01T01:00:00-04:00 ((2460493j,18000s,0n),-14400s,2299161j)>
#<DateTime: 2024-07-01T12:00:00+09:00 ((2460493j,10800s,0n),+32400s,2299161j)>
#<DateTime: 2024-07-01T12:00:00+10:00 ((2460493j,7200s,0n),+36000s,2299161j)>
unknown time zone Europe/Nowhere: an IANA name such as Europe/Berlin, or an offset such as +01:00, expected
`)
}
