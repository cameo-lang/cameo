package stdlib

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/cameo-lang/cameo/src/object"
)

// What require 'time' adds to Time: parse, iso8601 (xmlschema) and
// strptime to read a time, and iso8601 to print one. Time.new takes a
// string too, as Ruby's does since 3.2.

// clock is a time read field by field; a field a string does not name is
// unset (-1), and loc is nil for a time without a zone, which is local.
type clock struct {
	year, mon, day, yday, hour, min, sec, nsec int
	loc                                        *time.Location
}

func newClock() clock {
	return clock{year: -1, mon: -1, day: -1, yday: -1, hour: -1, min: -1, sec: -1}
}

// at is the clock as a time: the date fields it lacks are today's, or the
// first of the year it names, and the time of day it lacks is midnight.
func (c clock) at() time.Time {
	loc := c.loc
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
	year, mon, day := or(c.year, now.Year()), or(c.mon, int(now.Month())), or(c.day, now.Day())
	if c.year >= 0 && c.mon < 0 {
		mon, day = 1, or(c.day, 1)
	}
	t := time.Date(year, time.Month(mon), day, or(c.hour, 0), or(c.min, 0), or(c.sec, 0), c.nsec, loc)
	if c.yday > 0 {
		t = time.Date(year, 1, c.yday, or(c.hour, 0), or(c.min, 0), or(c.sec, 0), c.nsec, loc)
	}
	return t
}

// fraction is a decimal fraction of a second, in nanoseconds; digits past
// the ninth are dropped, since a time holds nanoseconds.
func fraction(digits string) int {
	if len(digits) > 9 {
		digits = digits[:9]
	}
	n, _ := strconv.Atoi(digits + strings.Repeat("0", 9-len(digits)))
	return n
}

// zoneNamed reads a zone as a parsed time names one: an offset with or
// without a colon, Z, UTC or GMT.
func zoneNamed(s string) (*time.Location, bool) {
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
	// A date and maybe a time, in ISO 8601's order, with - or /.
	isoTime = regexp.MustCompile(`(\d{4})[-/](\d{1,2})[-/](\d{1,2})(?:(?:T|\s+)(\d{1,2}):(\d{2})(?::(\d{2})(?:[.,](\d+))?)?)?\s*(Z|UTC|GMT|[+-]\d{2}:?\d{2})?`)
	// A date with a month name, day first or month first, as RFC 2822,
	// HTTP dates and inspect of other languages print one.
	namedTime = regexp.MustCompile(`(?i)(?:(\d{1,2})\s+([a-z]{3})[a-z]*\.?,?\s+(\d{4})|([a-z]{3})[a-z]*\.?\s+(\d{1,2}),?\s+(\d{4}))(?:\s+(\d{1,2}):(\d{2})(?::(\d{2}))?)?\s*(Z|UTC|GMT|[+-]\d{2}:?\d{2})?`)
	// A time of day alone.
	dayTime = regexp.MustCompile(`(\d{1,2}):(\d{2})(?::(\d{2})(?:[.,](\d+))?)?\s*(Z|UTC|GMT|[+-]\d{2}:?\d{2})?`)
	// ISO 8601's own form, all of it: what Time.iso8601 takes.
	xmlSchema = regexp.MustCompile(`(?i)^\s*(-?\d{4,})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?(Z|[+-]\d{2}(?::?\d{2})?)?\s*$`)
	// A date alone, which Time.new refuses for want of a time.
	dateOnly = regexp.MustCompile(`^\s*-?\d{4,}-\d{2}-\d{2}\s*$`)
)

// parseTime is Time.parse: the first date it finds in s, with a time and a
// zone where they follow, after the forms Ruby's Date._parse reads most.
func parseTime(s string) (clock, bool) {
	c := newClock()
	num := func(v string) int {
		n, _ := strconv.Atoi(v)
		return n
	}
	zone := func(v string) {
		if loc, ok := zoneNamed(v); ok {
			c.loc = loc
		}
	}
	setTime := func(h, m, sec, frac string) {
		c.hour, c.min = num(h), num(m)
		if sec != "" {
			c.sec = num(sec)
		}
		if frac != "" {
			c.nsec = fraction(frac)
		}
	}
	if m := isoTime.FindStringSubmatch(s); m != nil {
		c.year, c.mon, c.day = num(m[1]), num(m[2]), num(m[3])
		if m[4] != "" {
			setTime(m[4], m[5], m[6], m[7])
		}
		zone(m[8])
		return c, true
	}
	if m := namedTime.FindStringSubmatch(s); m != nil {
		if m[1] != "" {
			c.day, c.mon, c.year = num(m[1]), monthNamed(m[2]), num(m[3])
		} else {
			c.mon, c.day, c.year = monthNamed(m[4]), num(m[5]), num(m[6])
		}
		if c.mon > 0 {
			if m[7] != "" {
				setTime(m[7], m[8], m[9], "")
			}
			zone(m[10])
			return c, true
		}
	}
	if m := dayTime.FindStringSubmatch(s); m != nil {
		setTime(m[1], m[2], m[3], m[4])
		zone(m[5])
		return c, true
	}
	return c, false
}

// parseXMLSchema is Time.iso8601: a date and time with an optional
// fraction and zone, and nothing else.
func parseXMLSchema(s string) (clock, bool) {
	m := xmlSchema.FindStringSubmatch(s)
	if m == nil {
		return clock{}, false
	}
	c := newClock()
	num := func(v string) int {
		n, _ := strconv.Atoi(v)
		return n
	}
	c.year, c.mon, c.day = num(m[1]), num(m[2]), num(m[3])
	c.hour, c.min, c.sec = num(m[4]), num(m[5]), num(m[6])
	c.nsec = fraction(m[7])
	if m[8] != "" {
		c.loc, _ = zoneNamed(m[8])
	}
	return c, true
}

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
	return time.Date(num(m[1]), time.Month(num(m[2])), num(m[3]), num(m[4]), num(m[5]), num(m[6]), fraction(m[7]), loc), nil
}

// strptime reads s by a strftime format, after Ruby's Date._strptime: a
// numeric field takes as many digits as it may, or its width when the
// next directive is numeric too, whitespace matches any run of it, and
// what s has left over is an error.
func strptime(s, format string) (clock, bool) {
	c := newClock()
	var epoch *int64
	pm := -1
	at := 0
	skipSpace := func() {
		for at < len(s) && unicode.IsSpace(rune(s[at])) {
			at++
		}
	}
	// digits reads a number of at most max digits, with a sign when signed.
	digits := func(max int, signed bool) (int64, bool) {
		start := at
		if signed && at < len(s) && (s[at] == '+' || s[at] == '-') {
			at++
		}
		d := at
		for at < len(s) && at-d < max && s[at] >= '0' && s[at] <= '9' {
			at++
		}
		if at == d {
			at = start
			return 0, false
		}
		n, err := strconv.ParseInt(s[start:at], 10, 64)
		return n, err == nil
	}
	word := func(names func(int) string, count int) (int, bool) {
		for i := 0; i < count; i++ {
			for _, name := range []string{names(i), names(i)[:3]} {
				if len(s)-at >= len(name) && strings.EqualFold(s[at:at+len(name)], name) {
					at += len(name)
					return i, true
				}
			}
		}
		return 0, false
	}
	expand := strings.NewReplacer("%T", "%H:%M:%S", "%D", "%m/%d/%y", "%F", "%Y-%m-%d", "%R", "%H:%M", "%r", "%I:%M:%S %p")
	format = expand.Replace(format)
	numeric := func(i int) bool {
		return i+1 < len(format) && format[i] == '%' && strings.IndexByte("YmdejHkIlMSyLNsCU", format[i+1]) >= 0
	}
	for i := 0; i < len(format); i++ {
		f := format[i]
		if unicode.IsSpace(rune(f)) {
			skipSpace()
			continue
		}
		if f != '%' || i+1 == len(format) {
			if at >= len(s) || s[at] != f {
				return c, false
			}
			at++
			continue
		}
		i++
		width := func(w int) int {
			if numeric(i + 1) {
				return w
			}
			return 64
		}
		var n int64
		ok := true
		switch format[i] {
		case 'Y':
			n, ok = digits(width(4), true)
			c.year = int(n)
		case 'C':
			n, ok = digits(2, true)
			c.year = int(n) * 100
		case 'y':
			n, ok = digits(2, false)
			if n < 69 {
				n += 2000
			} else {
				n += 1900
			}
			c.year = int(n)
		case 'm':
			n, ok = digits(2, false)
			c.mon = int(n)
		case 'd', 'e':
			skipSpace()
			n, ok = digits(2, false)
			c.day = int(n)
		case 'j':
			n, ok = digits(3, false)
			c.yday = int(n)
		case 'H', 'k':
			skipSpace()
			n, ok = digits(2, false)
			c.hour = int(n)
		case 'I', 'l':
			skipSpace()
			n, ok = digits(2, false)
			c.hour = int(n)
		case 'M':
			n, ok = digits(2, false)
			c.min = int(n)
		case 'S':
			n, ok = digits(2, false)
			c.sec = int(n)
		case 'L', 'N':
			start := at
			for at < len(s) && s[at] >= '0' && s[at] <= '9' {
				at++
			}
			ok = at > start
			c.nsec = fraction(s[start:at])
		case 's':
			n, ok = digits(64, true)
			epoch = &n
		case 'b', 'B', 'h':
			var m int
			m, ok = word(func(i int) string { return time.Month(i + 1).String() }, 12)
			c.mon = m + 1
		case 'a', 'A':
			_, ok = word(func(i int) string { return time.Weekday(i).String() }, 7)
		case 'p', 'P':
			ok = false
			if len(s)-at >= 2 {
				switch strings.ToUpper(s[at : at+2]) {
				case "AM":
					pm, ok = 0, true
				case "PM":
					pm, ok = 1, true
				}
				at += 2
			}
		case 'z', 'Z':
			end := at
			if end < len(s) && (s[end] == '+' || s[end] == '-') {
				end++
				for end < len(s) && (s[end] >= '0' && s[end] <= '9' || s[end] == ':') {
					end++
				}
			} else {
				for end < len(s) && unicode.IsLetter(rune(s[end])) {
					end++
				}
			}
			c.loc, ok = zoneNamed(s[at:end])
			at = end
		case '%':
			ok = at < len(s) && s[at] == '%'
			at++
		default:
			return c, false
		}
		if !ok {
			return c, false
		}
	}
	if at != len(s) {
		return c, false
	}
	if pm >= 0 && c.hour >= 0 {
		c.hour = c.hour%12 + 12*pm
	}
	if epoch != nil {
		loc := c.loc
		if loc == nil {
			loc = time.Local
		}
		t := time.Unix(*epoch, int64(c.nsec)).In(loc)
		c = clock{year: t.Year(), mon: int(t.Month()), day: t.Day(), hour: t.Hour(), min: t.Minute(), sec: t.Second(), nsec: t.Nanosecond(), loc: loc}
	}
	return c, true
}

func installTimeParsing() {
	c := object.TimeClass
	str := func(ctx object.Context, args []object.Value, n string) (string, error) {
		if len(args) < 1 {
			return "", argError(ctx, len(args), n)
		}
		return wantString(ctx, args[0])
	}
	c.DefineSingleton("parse", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := str(ctx, args, "1..2")
		if err != nil {
			return object.Value{}, err
		}
		t, ok := parseTime(s)
		if !ok {
			return object.Value{}, ctx.Raise(object.ArgumentError, "no time information in %s", object.QuoteString(s))
		}
		return object.TimeValue(t.at()), nil
	})
	iso := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := str(ctx, args, "1")
		if err != nil {
			return object.Value{}, err
		}
		t, ok := parseXMLSchema(s)
		if !ok {
			return object.Value{}, ctx.Raise(object.ArgumentError, "invalid xmlschema format: %s", object.QuoteString(s))
		}
		return object.TimeValue(t.at()), nil
	}
	c.DefineSingleton("iso8601", 1, iso)
	c.DefineSingleton("xmlschema", 1, iso)
	c.DefineSingleton("strptime", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 2 {
			return object.Value{}, argError(ctx, len(args), "2..3")
		}
		s, err := wantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		f, err := wantString(ctx, args[1])
		if err != nil {
			return object.Value{}, err
		}
		t, ok := strptime(s, f)
		if !ok {
			return object.Value{}, ctx.Raise(object.ArgumentError, "invalid date or strptime format - '%s' '%s'", s, f)
		}
		return object.TimeValue(t.at()), nil
	})
	// iso8601 prints the time in ISO 8601, with n digits of the second's
	// fraction, and Z for UTC.
	xml := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n := int64(0)
		if len(args) == 1 {
			var err error
			if n, err = wantInt(ctx, args[0], ""); err != nil {
				return object.Value{}, err
			}
		}
		t := self.Time()
		out := Strftime(t, "%Y-%m-%dT%H:%M:%S")
		if n > 0 {
			out += "." + padDigits(int64(t.Nanosecond()), 9)[:min(n, 9)] + strings.Repeat("0", int(max(n-9, 0)))
		}
		if t.Location() == time.UTC {
			return object.String(out + "Z"), nil
		}
		return object.String(out + Strftime(t, "%:z")), nil
	}
	def(c, "iso8601", -1, xml)
	def(c, "xmlschema", -1, xml)
}
