// Package time is Ruby's time library, built into cameo: what require
// "time" adds to Time. Time.parse, iso8601 (xmlschema) and strptime read a
// time, and Time#iso8601 prints one. Time.new's own string form is the
// core's, in src/stdlib.
package time

import (
	"regexp"
	"strconv"
	"strings"
	gotime "time"
	"unicode"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

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
)

// parseTime is Time.parse: the first date it finds in s, with a time and a
// zone where they follow, after the forms Ruby's Date._parse reads most.
func parseTime(s string) (stdlib.Clock, bool) {
	c := stdlib.NewClock()
	num := func(v string) int {
		n, _ := strconv.Atoi(v)
		return n
	}
	zone := func(v string) {
		if loc, ok := stdlib.ZoneNamed(v); ok {
			c.Loc = loc
		}
	}
	setTime := func(h, m, sec, frac string) {
		c.Hour, c.Min = num(h), num(m)
		if sec != "" {
			c.Sec = num(sec)
		}
		if frac != "" {
			c.Nsec = stdlib.Fraction(frac)
		}
	}
	if m := isoTime.FindStringSubmatch(s); m != nil {
		c.Year, c.Mon, c.Day = num(m[1]), num(m[2]), num(m[3])
		if m[4] != "" {
			setTime(m[4], m[5], m[6], m[7])
		}
		zone(m[8])
		return c, true
	}
	if m := namedTime.FindStringSubmatch(s); m != nil {
		if m[1] != "" {
			c.Day, c.Mon, c.Year = num(m[1]), stdlib.MonthNamed(m[2]), num(m[3])
		} else {
			c.Mon, c.Day, c.Year = stdlib.MonthNamed(m[4]), num(m[5]), num(m[6])
		}
		if c.Mon > 0 {
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
func parseXMLSchema(s string) (stdlib.Clock, bool) {
	m := xmlSchema.FindStringSubmatch(s)
	if m == nil {
		return stdlib.Clock{}, false
	}
	c := stdlib.NewClock()
	num := func(v string) int {
		n, _ := strconv.Atoi(v)
		return n
	}
	c.Year, c.Mon, c.Day = num(m[1]), num(m[2]), num(m[3])
	c.Hour, c.Min, c.Sec = num(m[4]), num(m[5]), num(m[6])
	c.Nsec = stdlib.Fraction(m[7])
	if m[8] != "" {
		c.Loc, _ = stdlib.ZoneNamed(m[8])
	}
	return c, true
}

func strptime(s, format string) (stdlib.Clock, bool) {
	c := stdlib.NewClock()
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
			c.Year = int(n)
		case 'C':
			n, ok = digits(2, true)
			c.Year = int(n) * 100
		case 'y':
			n, ok = digits(2, false)
			if n < 69 {
				n += 2000
			} else {
				n += 1900
			}
			c.Year = int(n)
		case 'm':
			n, ok = digits(2, false)
			c.Mon = int(n)
		case 'd', 'e':
			skipSpace()
			n, ok = digits(2, false)
			c.Day = int(n)
		case 'j':
			n, ok = digits(3, false)
			c.Yday = int(n)
		case 'H', 'k':
			skipSpace()
			n, ok = digits(2, false)
			c.Hour = int(n)
		case 'I', 'l':
			skipSpace()
			n, ok = digits(2, false)
			c.Hour = int(n)
		case 'M':
			n, ok = digits(2, false)
			c.Min = int(n)
		case 'S':
			n, ok = digits(2, false)
			c.Sec = int(n)
		case 'L', 'N':
			start := at
			for at < len(s) && s[at] >= '0' && s[at] <= '9' {
				at++
			}
			ok = at > start
			c.Nsec = stdlib.Fraction(s[start:at])
		case 's':
			n, ok = digits(64, true)
			epoch = &n
		case 'b', 'B', 'h':
			var m int
			m, ok = word(func(i int) string { return gotime.Month(i + 1).String() }, 12)
			c.Mon = m + 1
		case 'a', 'A':
			_, ok = word(func(i int) string { return gotime.Weekday(i).String() }, 7)
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
			c.Loc, ok = stdlib.ZoneNamed(s[at:end])
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
	if pm >= 0 && c.Hour >= 0 {
		c.Hour = c.Hour%12 + 12*pm
	}
	if epoch != nil {
		loc := c.Loc
		if loc == nil {
			loc = gotime.Local
		}
		t := gotime.Unix(*epoch, int64(c.Nsec)).In(loc)
		c = stdlib.Clock{Year: t.Year(), Mon: int(t.Month()), Day: t.Day(), Hour: t.Hour(), Min: t.Minute(), Sec: t.Second(), Nsec: t.Nanosecond(), Loc: loc}
	}
	return c, true
}

// Install defines what require "time" adds to Time. lib.Install calls it
// once.
func Install() {
	c := object.TimeClass
	str := func(ctx object.Context, args []object.Value, n string) (string, error) {
		if len(args) < 1 {
			return "", stdlib.ArgError(ctx, len(args), n)
		}
		return stdlib.WantString(ctx, args[0])
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
		return object.TimeValue(t.At()), nil
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
		return object.TimeValue(t.At()), nil
	}
	c.DefineSingleton("iso8601", 1, iso)
	c.DefineSingleton("xmlschema", 1, iso)
	c.DefineSingleton("strptime", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "2..3")
		}
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		f, err := stdlib.WantString(ctx, args[1])
		if err != nil {
			return object.Value{}, err
		}
		t, ok := strptime(s, f)
		if !ok {
			return object.Value{}, ctx.Raise(object.ArgumentError, "invalid date or strptime format - '%s' '%s'", s, f)
		}
		return object.TimeValue(t.At()), nil
	})
	// iso8601 prints the time in ISO 8601, with n digits of the second's
	// fraction, and Z for UTC.
	xml := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n := int64(0)
		if len(args) == 1 {
			var err error
			if n, err = stdlib.WantInt(ctx, args[0], ""); err != nil {
				return object.Value{}, err
			}
		}
		t := self.Time()
		out := stdlib.Strftime(t, "%Y-%m-%dT%H:%M:%S")
		if n > 0 {
			out += "." + stdlib.PadDigits(int64(t.Nanosecond()), 9)[:min(n, 9)] + strings.Repeat("0", int(max(n-9, 0)))
		}
		if t.Location() == gotime.UTC {
			return object.String(out + "Z"), nil
		}
		return object.String(out + stdlib.Strftime(t, "%:z")), nil
	}
	c.Define("iso8601", -1, xml)
	c.Define("xmlschema", -1, xml)
}
