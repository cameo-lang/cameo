package stdlib

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cameo-lang/cameo/src/object"
)

// Reading a time from a string: Time.new's one-string form, which is
// core, and the Clock the time library (src/lib/time) fills field by
// field for Time.parse, iso8601 and strptime.

// Clock is a time read field by field; a field a string does not name is
// unset (-1), and Loc is nil for a time without a zone, which is local.
type Clock struct {
	Year, Mon, Day, Yday, Hour, Min, Sec, Nsec int
	Loc                                        *time.Location
}

// NewClock is a Clock with every field unset.
func NewClock() Clock {
	return Clock{Year: -1, Mon: -1, Day: -1, Yday: -1, Hour: -1, Min: -1, Sec: -1}
}

// At is the Clock as a time: the date fields it lacks are today's, or the
// first of the year it names, and the time of day it lacks is midnight.
func (c Clock) At() time.Time {
	loc := c.Loc
	if loc == nil {
		loc = time.Local
	}
	now := time.Now().In(loc)
	or := func(v, def int) int {
		if v < 0 {
			return def
		}
		return v
	}
	year, mon, day := or(c.Year, now.Year()), or(c.Mon, int(now.Month())), or(c.Day, now.Day())
	if c.Year >= 0 && c.Mon < 0 {
		mon, day = 1, or(c.Day, 1)
	}
	t := time.Date(year, time.Month(mon), day, or(c.Hour, 0), or(c.Min, 0), or(c.Sec, 0), c.Nsec, loc)
	if c.Yday > 0 {
		t = time.Date(year, 1, c.Yday, or(c.Hour, 0), or(c.Min, 0), or(c.Sec, 0), c.Nsec, loc)
	}
	return t
}

// Fraction is a decimal fraction of a second, in nanoseconds; digits past
// the ninth are dropped, since a time holds nanoseconds.
func Fraction(digits string) int {
	if len(digits) > 9 {
		digits = digits[:9]
	}
	n, _ := strconv.Atoi(digits + strings.Repeat("0", 9-len(digits)))
	return n
}

// ZoneNamed reads a zone as a parsed time names one: an offset with or
// without a colon, Z, UTC or GMT.
func ZoneNamed(s string) (*time.Location, bool) {
	switch strings.ToUpper(s) {
	case "Z", "UTC":
		return time.UTC, true
	case "GMT", "UT":
		// Ruby reads these as an offset of zero, not as UTC.
		return time.FixedZone("", 0), true
	}
	if len(s) < 2 || s[0] != '+' && s[0] != '-' {
		return nil, false
	}
	digits := strings.ReplaceAll(s[1:], ":", "")
	if !allDigits(digits) || len(digits) == 0 || len(digits) > 4 {
		return nil, false
	}
	if len(digits) <= 2 {
		digits += "00"
	} else if len(digits) == 3 {
		digits = "0" + digits
	}
	h, _ := strconv.Atoi(digits[:2])
	m, _ := strconv.Atoi(digits[2:])
	off := h*3600 + m*60
	if s[0] == '-' {
		off = -off
	}
	return time.FixedZone("", off), true
}

var (
	// A date alone, which Time.new refuses for want of a time.
	dateOnly = regexp.MustCompile(`^\s*-?\d{4,}-\d{2}-\d{2}\s*$`)
)

// civilString is Time.new with one string, Ruby 3.2's form: a date, a
// time, an optional fraction and an optional zone; a date alone is an
// error, as it is in Ruby.
var civilString = regexp.MustCompile(`^\s*(-?\d{4,})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?\s*(\S+)?\s*$`)

func parseCivil(ctx object.Context, s string, loc *time.Location) (time.Time, error) {
	m := civilString.FindStringSubmatch(s)
	if m == nil {
		if dateOnly.MatchString(s) {
			return time.Time{}, ctx.Raise(object.ArgumentError, "no time information")
		}
		return time.Time{}, ctx.Raise(object.ArgumentError, "can't parse: %s", object.QuoteString(s))
	}
	num := func(v string) int {
		n, _ := strconv.Atoi(v)
		return n
	}
	if m[8] != "" {
		z, err := zoneArg(ctx, object.String(m[8]))
		if err != nil {
			return time.Time{}, err
		}
		loc = z
	}
	return time.Date(num(m[1]), time.Month(num(m[2])), num(m[3]), num(m[4]), num(m[5]), num(m[6]), Fraction(m[7]), loc), nil
}

// strptime reads s by a strftime format, after Ruby's Date._strptime: a
// numeric field takes as many digits as it may, or its width when the
// next directive is numeric too, whitespace matches any run of it, and
