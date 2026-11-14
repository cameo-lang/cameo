package date

import (
	"fmt"
	"github.com/cameo-lang/cameo/src/str"
	"math"
	"strings"
	"time"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

// moment gives Strftime a Date's fields under its own calendar; it is a
// stdlib.Clockwork.
type moment struct{ d dt }

func (m moment) Year() int {
	y, _, _ := m.d.civil()
	return int(y)
}

func (m moment) Month() time.Month {
	_, mon, _ := m.d.civil()
	return time.Month(mon)
}

func (m moment) Day() int {
	_, _, d := m.d.civil()
	return int(d)
}

func (m moment) YearDay() int {
	jd, _ := m.d.local()
	_, yd := jdToOrdinal(jd, m.d.sg)
	return int(yd)
}

func (m moment) Hour() int   { h, _, _ := m.d.clock(); return int(h) }
func (m moment) Minute() int { _, min, _ := m.d.clock(); return int(min) }
func (m moment) Second() int { _, _, s := m.d.clock(); return int(s) }
func (m moment) Nanosecond() int {
	return int(m.d.sf)
}

func (m moment) Weekday() time.Weekday {
	jd, _ := m.d.local()
	return time.Weekday(wday(jd))
}

func (m moment) ISOWeek() (int, int) {
	jd, _ := m.d.local()
	y, w, _ := jdToCommercial(jd, m.d.sg)
	return int(y), int(w)
}

func (m moment) Zone() (string, int) { return offsetString(m.d.of, true), int(m.d.of) }

func (m moment) Unix() int64 { return (m.d.jd-unixEpoch)*daySecs + m.d.df }

// strftime is Date#strftime: Ruby's directives on the date's fields. The
// core's Strftime takes the directives Time has; %Q, %+ and the week
// numbers of a Julian date are the date's own, so they are expanded first.
func strftime(d dt, f string) string {
	m := moment{d}
	var b strings.Builder
	for i := 0; i < len(f); i++ {
		if f[i] != '%' || i+1 >= len(f) {
			b.WriteByte(f[i])
			continue
		}
		switch f[i+1] {
		case 'Q':
			b.WriteString(fmt.Sprint(m.Unix()*1000 + d.sf/1_000_000))
			i++
		case '+':
			b.WriteString(stdlib.Strftime(m, "%a %b %e %H:%M:%S %Z %Y"))
			i++
		case '%':
			b.WriteString("%%")
			i++
		default:
			b.WriteByte('%')
		}
	}
	return stdlib.Strftime(m, b.String())
}

// toS is Date#to_s, "2024-02-29", and DateTime#to_s,
// "2001-02-03T04:05:06+07:00".
func toS(v object.Value) string {
	d := read(v)
	if isDateTime(v) {
		return strftime(d, "%Y-%m-%dT%H:%M:%S%:z")
	}
	return strftime(d, "%Y-%m-%d")
}

// inspect is Date#inspect: the string, and the fields as Ruby shows them,
// "#<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>".
func inspect(v object.Value) string {
	d := read(v)
	sg := "Infj"
	switch {
	case math.IsInf(d.sg, -1):
		sg = "-Infj"
	case !math.IsInf(d.sg, 0):
		sg = fmt.Sprintf("%dj", int64(d.sg))
	}
	return fmt.Sprintf("#<%s: %s ((%dj,%ds,%dn),%+ds,%s)>", v.Instance().Cls.QualifiedName(), toS(v), d.jd, d.df, d.sf, d.of, sg)
}

// fraction is the second's fraction with n digits, as iso8601(n) prints
// it: ".500" for n == 3, nothing for n == 0.
func fraction(sf int64, n int64) string {
	if n <= 0 {
		return ""
	}
	digits := fmt.Sprintf("%09d", sf)
	if n <= 9 {
		return "." + digits[:n]
	}
	return "." + digits + strings.Repeat("0", int(n-9))
}

// jisx0301 is Date#jisx0301: the Japanese era and the year in it, or the
// ISO date before the Meiji era.
func jisx0301(d dt) string {
	jd, _ := d.local()
	y, m, day := d.civil()
	var era byte
	var base int64
	switch {
	case jd < 2405160:
		return strftime(d, "%Y-%m-%d")
	case jd < 2419614:
		era, base = 'M', 1867
	case jd < 2424875:
		era, base = 'T', 1911
	case jd < 2447535:
		era, base = 'S', 1925
	case jd < 2458605:
		era, base = 'H', 1988
	default:
		era, base = 'R', 2018
	}
	return fmt.Sprintf("%c%02d.%02d.%02d", era, y-base, m, day)
}

func installFormatting() {
	c := dateClass
	strOf := func(name string, f func(d dt, v object.Value) string) {
		c.Define(name, 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.NewString(f(read(self), self), str.UTF8), nil
		})
	}
	strOf("to_s", func(d dt, v object.Value) string { return toS(v) })
	strOf("inspect", func(d dt, v object.Value) string { return inspect(v) })
	strOf("asctime", func(d dt, v object.Value) string { return strftime(d, "%a %b %e %H:%M:%S %Y") })
	strOf("ctime", func(d dt, v object.Value) string { return strftime(d, "%a %b %e %H:%M:%S %Y") })
	strOf("rfc2822", func(d dt, v object.Value) string { return strftime(d, "%a, %-d %b %Y %T %z") })
	strOf("rfc822", func(d dt, v object.Value) string { return strftime(d, "%a, %-d %b %Y %T %z") })
	strOf("httpdate", func(d dt, v object.Value) string {
		d.of = 0
		return strftime(d, "%a, %d %b %Y %T GMT")
	})
	// iso8601(n), xmlschema(n), rfc3339(n) and jisx0301(n) take the digits
	// of the second's fraction on a DateTime; a Date prints the day alone.
	timed := func(name string, date func(d dt) string, datetime func(d dt, n int64) string) {
		c.Define(name, -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			d := read(self)
			if !isDateTime(self) {
				if len(a) > 0 {
					return object.Value{}, stdlib.ArgError(ctx, len(a), "0")
				}
				return object.NewString(date(d), str.UTF8), nil
			}
			n := int64(0)
			if len(a) > 1 {
				return object.Value{}, stdlib.ArgError(ctx, len(a), "0..1")
			}
			if len(a) == 1 {
				var err error
				if n, err = stdlib.WantInt(ctx, a[0], ""); err != nil {
					return object.Value{}, err
				}
			}
			return object.NewString(datetime(d, n), str.UTF8), nil
		})
	}
	iso := func(d dt, n int64) string {
		return strftime(d, "%Y-%m-%dT%H:%M:%S") + fraction(d.sf, n) + strftime(d, "%:z")
	}
	timed("iso8601", func(d dt) string { return strftime(d, "%Y-%m-%d") }, iso)
	timed("xmlschema", func(d dt) string { return strftime(d, "%Y-%m-%d") }, iso)
	timed("rfc3339", func(d dt) string { return strftime(d, "%Y-%m-%dT%H:%M:%S%:z") }, iso)
	timed("jisx0301", jisx0301, func(d dt, n int64) string {
		return jisx0301(d) + strftime(d, "T%H:%M:%S") + fraction(d.sf, n) + strftime(d, "%:z")
	})
	c.Define("strftime", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		f := "%F"
		if isDateTime(self) {
			f = "%FT%T%:z"
		}
		if len(a) > 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(a), "0..1")
		}
		if len(a) == 1 {
			var err error
			if f, err = stdlib.WantString(ctx, a[0]); err != nil {
				return object.Value{}, err
			}
		}
		return object.NewString(strftime(read(self), f), str.UTF8), nil
	})
	c.Define("deconstruct_keys", 1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		d := read(self)
		y, m, day := d.civil()
		jd, _ := d.local()
		_, yd := jdToOrdinal(jd, d.sg)
		all := map[string]object.Value{
			"year": object.Int(y), "month": object.Int(m), "day": object.Int(day),
			"yday": object.Int(yd), "wday": object.Int(wday(jd)),
		}
		order := []string{"year", "month", "day", "yday", "wday"}
		if isDateTime(self) {
			h, min, s := d.clock()
			all["hour"], all["min"], all["sec"] = object.Int(h), object.Int(min), object.Int(s)
			all["sec_fraction"] = ratio(float64(d.sf), nanos)
			all["zone"] = object.NewString(offsetString(d.of, true), str.UTF8)
			order = append(order, "hour", "min", "sec", "sec_fraction", "zone")
		}
		b := object.NewMapBuilder()
		if a[0].IsNil() {
			for _, k := range order {
				b.Set(object.Symbol(k), all[k])
			}
			return object.MapValue(b.Map()), nil
		}
		keys, err := stdlib.ToList(ctx, a[0])
		if err != nil {
			return object.Value{}, err
		}
		for _, k := range keys {
			name, ok := k.AsSymbol()
			if !ok {
				return object.Value{}, ctx.Raise(object.TypeError, "wrong argument type %s (expected Symbol)", stdlib.KindName(k))
			}
			if v, ok := all[name]; ok {
				b.Set(k, v)
			}
		}
		return object.MapValue(b.Map()), nil
	})
}
