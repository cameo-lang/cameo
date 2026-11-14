package date

import (
	"math"
	"time"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

// Turning fragments into a date: rt_complete_frags fills what a text left
// out from today, rt_rewrite_frags turns seconds since the epoch into a
// day and a time, and validDateFrags tries each way of naming a day.

// hashOf is the fragments as the Hash Date._parse returns.
func hashOf(f *frags) object.Value {
	b := object.NewMapBuilder()
	for _, k := range f.keys {
		var v object.Value
		switch x := f.values[k].(type) {
		case int64:
			v = object.Int(x)
		case float64:
			v = ratio(x, 1)
			if k == "sec_fraction" || k == "seconds" {
				v = object.Float(x)
				if x == math.Trunc(x) {
					v = object.Int(int64(x))
				}
			}
		case string:
			v = object.NewString(x, object.UTF8)
		}
		b.Set(object.Symbol(k), v)
	}
	return object.MapValue(b.Map())
}

// fragTables are the sets of fragments that name a day, from
// rt_complete_frags; the one with the most members present wins.
var fragTables = []struct {
	kind string
	keys []string
}{
	{"time", []string{"hour", "min", "sec"}},
	{"", []string{"jd"}},
	{"ordinal", []string{"year", "yday", "hour", "min", "sec"}},
	{"civil", []string{"year", "mon", "mday", "hour", "min", "sec"}},
	{"commercial", []string{"cwyear", "cweek", "cwday", "hour", "min", "sec"}},
	{"wday", []string{"wday", "hour", "min", "sec"}},
	{"wnum0", []string{"year", "wnum0", "wday", "hour", "min", "sec"}},
	{"wnum1", []string{"year", "wnum1", "wday", "hour", "min", "sec"}},
	{"", []string{"cwyear", "cweek", "wday", "hour", "min", "sec"}},
	{"", []string{"year", "wnum0", "cwday", "hour", "min", "sec"}},
	{"", []string{"year", "wnum1", "cwday", "hour", "min", "sec"}},
}

// rewriteFrags turns seconds since the epoch into a day and a time of
// day, under the offset given. The fraction is the seconds' own, as Ruby's
// rt_rewrite_frags sets it: a sec_fraction %N read beside %s is dropped.
func rewriteFrags(f *frags) {
	secs, ok := f.values["seconds"]
	if !ok {
		return
	}
	var s float64
	switch x := secs.(type) {
	case int64:
		s = float64(x)
	case float64:
		s = x
	}
	off := int64(0)
	if o, ok := f.int("offset"); ok {
		off = o
	}
	s += float64(off)
	whole := math.Floor(s)
	day := floorDiv(int64(whole), daySecs)
	df := floorMod(int64(whole), daySecs)
	f.del("seconds")
	f.del("sec_fraction")
	f.set("jd", unixEpoch+day)
	f.set("hour", df/3600)
	f.set("min", df/60%60)
	f.set("sec", df%60)
	if frac := s - whole; frac != 0 {
		f.set("sec_fraction", frac)
	}
}

// completeFrags fills in what the text left out, from today, as Ruby's
// rt_complete_frags does for the class.
func completeFrags(f *frags, datetime bool, sg float64) {
	best, bestN := -1, 0
	for i, t := range fragTables {
		n := 0
		for _, k := range t.keys {
			if f.has(k) {
				n++
			}
		}
		if n > bestN {
			best, bestN = i, n
		}
	}
	now := time.Now()
	todayJD := civilToJD(int64(now.Year()), int64(now.Month()), int64(now.Day()), gregorianStart)
	if best >= 0 {
		t := fragTables[best]
		switch t.kind {
		case "ordinal":
			if !f.has("year") {
				y, _ := jdToOrdinal(todayJD, sg)
				f.set("year", y)
			}
			if !f.has("yday") {
				f.set("yday", int64(1))
			}
		case "civil":
			y, m, d := jdToCivil(todayJD, sg)
			for _, k := range []string{"year", "mon", "mday"} {
				if f.has(k) {
					break
				}
				switch k {
				case "year":
					f.set(k, y)
				case "mon":
					f.set(k, m)
				case "mday":
					f.set(k, d)
				}
			}
			if !f.has("mon") {
				f.set("mon", int64(1))
			}
			if !f.has("mday") {
				f.set("mday", int64(1))
			}
		case "commercial":
			y, w, d := jdToCommercial(todayJD, sg)
			for _, k := range []string{"cwyear", "cweek", "cwday"} {
				if f.has(k) {
					break
				}
				switch k {
				case "cwyear":
					f.set(k, y)
				case "cweek":
					f.set(k, w)
				case "cwday":
					f.set(k, d)
				}
			}
			if !f.has("cweek") {
				f.set("cweek", int64(1))
			}
			if !f.has("cwday") {
				f.set("cwday", int64(1))
			}
		case "wday":
			wd, _ := f.int("wday")
			f.set("jd", todayJD-wday(todayJD)+wd)
		case "wnum0", "wnum1":
			fnum := int64(0)
			if t.kind == "wnum1" {
				fnum = 1
			}
			y, w := weekNumber(todayJD, fnum, sg)
			for _, k := range []string{"year", t.kind, "wday"} {
				if f.has(k) {
					break
				}
				switch k {
				case "year":
					f.set(k, y)
				case t.kind:
					f.set(k, w)
				case "wday":
					f.set(k, fnum)
				}
			}
			if !f.has(t.kind) {
				// A year with a weekday and no week is week 0, as Ruby's
				// completion takes it.
				f.set(t.kind, int64(0))
			}
			if !f.has("wday") {
				f.set("wday", fnum)
			}
		case "time":
			// A DateTime of a time alone is today's; a Date has no day and
			// is invalid.
			if datetime && !f.has("jd") {
				f.set("jd", todayJD)
			}
		}
	}
	if datetime {
		for _, k := range []string{"hour", "min", "sec"} {
			if !f.has(k) {
				f.set(k, int64(0))
			}
		}
		if s, ok := f.int("sec"); ok && s > 59 {
			f.values["sec"] = int64(59)
		}
	}
}

// validDateFrags is rt__valid_date_frags_p: the Julian Day Number the
// fragments name, by the first way that works.
func validDateFrags(f *frags, sg float64) (int64, bool) {
	get := func(k string) (int64, bool) { return f.int(k) }
	if jd, ok := get("jd"); ok {
		return jd, true
	}
	if y, ok := get("year"); ok {
		if yd, ok := get("yday"); ok {
			if jd, ok := validOrdinal(y, yd, sg); ok {
				return jd, true
			}
		}
	}
	if y, ok := get("year"); ok {
		if m, ok := get("mon"); ok {
			if d, ok := get("mday"); ok {
				if jd, ok := validCivil(y, m, d, sg); ok {
					return jd, true
				}
			}
		}
	}
	if y, ok := get("cwyear"); ok {
		if w, ok := get("cweek"); ok {
			d, has := get("cwday")
			if !has {
				if wd, ok := get("wday"); ok {
					d = wd
					if d == 0 {
						d = 7
					}
					has = true
				}
			}
			if has {
				if jd, ok := validCommercial(y, w, d, sg); ok {
					return jd, true
				}
			}
		}
	}
	if y, ok := get("year"); ok {
		if w, ok := get("wnum0"); ok {
			d, has := get("wday")
			if !has {
				if cd, ok := get("cwday"); ok {
					d, has = cd%7, true
				}
			}
			if has {
				jd := weeknumToJD(y, w, d, 0, sg)
				return jd, true
			}
		}
	}
	if y, ok := get("year"); ok {
		if w, ok := get("wnum1"); ok {
			d, has := get("wday")
			if !has {
				if cd, ok := get("cwday"); ok {
					d, has = cd%7, true
				}
			}
			if has {
				d = (d - 1 + 7) % 7
				jd := weeknumToJD(y, w, d, 1, sg)
				return jd, true
			}
		}
	}
	return 0, false
}

// dateFromFrags is a Date or DateTime of the fragments, or Date::Error.
func dateFromFrags(ctx object.Context, cls *object.Class, f *frags, sg float64) (object.Value, error) {
	datetime := cls.IsA(dateTimeClass)
	rewriteFrags(f)
	completeFrags(f, datetime, sg)
	jd, ok := validDateFrags(f, sg)
	if !ok {
		return object.Value{}, invalid(ctx)
	}
	if !datetime {
		return newDate(cls, jd, sg), nil
	}
	h, _ := f.int("hour")
	m, _ := f.int("min")
	s, _ := f.int("sec")
	df, carry, ok := clockOf(h, m, s)
	if !ok {
		return object.Value{}, invalid(ctx)
	}
	sf := int64(0)
	if frac, ok := f.values["sec_fraction"].(float64); ok {
		sf = int64(math.Round(frac * nanos))
	}
	of, _ := f.int("offset")
	return newDateTime(cls, jd+carry, df, sf, of, sg), nil
}

// installParsing defines Date.parse, strptime and the fixed formats, each
// with its underscored form that returns the fragments.
func installParsing() {
	c := dateClass
	type reader func(s string) *frags
	fixed := map[string]reader{
		"iso8601": parseISO8601, "rfc3339": parseRFC3339, "xmlschema": parseXMLSchema,
		"rfc2822": parseRFC2822, "rfc822": parseRFC2822, "httpdate": parseHTTPDate, "jisx0301": parseJISX0301,
	}
	// defaultFrags are the fragments of the day each reader takes with
	// no string, -4712-01-01, the epoch of the Julian day, at midnight
	// UTC for a DateTime: Ruby parses its default string, which a layout
	// cannot read for the sign of the year.
	defaultFrags := func(cls *object.Class) *frags {
		f := newFrags()
		f.set("year", int64(-4712))
		f.set("mon", int64(1))
		f.set("mday", int64(1))
		if cls.IsA(dateTimeClass) {
			f.set("hour", int64(0))
			f.set("min", int64(0))
			f.set("sec", int64(0))
			f.set("zone", "+00:00")
			f.set("offset", int64(0))
		}
		return f
	}
	for name, read := range fixed {
		name, read := name, read
		c.DefineSingleton("_"+name, -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(a) < 1 || len(a) > 2 {
				return object.Value{}, stdlib.ArgError(ctx, len(a), "1..2")
			}
			s, err := stdlib.WantString(ctx, a[0])
			if err != nil {
				return object.Value{}, err
			}
			return hashOf(read(s)), nil
		})
		c.DefineSingleton(name, -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(a) > 3 {
				return object.Value{}, stdlib.ArgError(ctx, len(a), "0..3")
			}
			f := defaultFrags(self.Cls())
			if len(a) >= 1 {
				s, err := stdlib.WantString(ctx, a[0])
				if err != nil {
					return object.Value{}, err
				}
				f = read(s)
			}
			sg := float64(italy)
			if len(a) == 2 {
				var err error
				if sg, err = startArg(ctx, a[1]); err != nil {
					return object.Value{}, err
				}
			}
			return dateFromFrags(ctx, self.Cls(), f, sg)
		})
	}
	c.DefineSingleton("_parse", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(a) < 1 || len(a) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(a), "1..2")
		}
		s, err := stdlib.WantString(ctx, a[0])
		if err != nil {
			return object.Value{}, err
		}
		comp := true
		if len(a) == 2 {
			comp = a[1].Truthy()
		}
		return hashOf(parse(s, comp)), nil
	})
	c.DefineSingleton("parse", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(a) > 3 {
			return object.Value{}, stdlib.ArgError(ctx, len(a), "0..3")
		}
		f := defaultFrags(self.Cls())
		comp := true
		if len(a) >= 2 {
			comp = a[1].Truthy()
		}
		if len(a) >= 1 {
			s, err := stdlib.WantString(ctx, a[0])
			if err != nil {
				return object.Value{}, err
			}
			f = parse(s, comp)
		}
		sg := float64(italy)
		if len(a) == 3 {
			var err error
			if sg, err = startArg(ctx, a[2]); err != nil {
				return object.Value{}, err
			}
		}
		return dateFromFrags(ctx, self.Cls(), f, sg)
	})
	c.DefineSingleton("_strptime", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(a) < 1 || len(a) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(a), "1..2")
		}
		s, err := stdlib.WantString(ctx, a[0])
		if err != nil {
			return object.Value{}, err
		}
		format := "%F"
		if self.Cls().IsA(dateTimeClass) {
			format = "%FT%T%z"
		}
		if len(a) == 2 {
			if format, err = stdlib.WantString(ctx, a[1]); err != nil {
				return object.Value{}, err
			}
		}
		f, err := strptime(s, format)
		if err == errNoMatch {
			return object.Nil, nil
		} else if err != nil {
			return object.Value{}, ctx.Raise(object.ArgumentError, "strptime: %s", err.Error())
		}
		return hashOf(f), nil
	})
	c.DefineSingleton("strptime", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(a) > 3 {
			return object.Value{}, stdlib.ArgError(ctx, len(a), "0..3")
		}
		datetime := self.Cls().IsA(dateTimeClass)
		s, format := "-4712-01-01", "%F"
		if datetime {
			s, format = "-4712-01-01T00:00:00+00:00", "%FT%T%z"
		}
		var err error
		if len(a) >= 1 {
			if s, err = stdlib.WantString(ctx, a[0]); err != nil {
				return object.Value{}, err
			}
		}
		if len(a) >= 2 {
			if format, err = stdlib.WantString(ctx, a[1]); err != nil {
				return object.Value{}, err
			}
		}
		sg := float64(italy)
		if len(a) == 3 {
			if sg, err = startArg(ctx, a[2]); err != nil {
				return object.Value{}, err
			}
		}
		f := defaultFrags(self.Cls())
		if len(a) >= 1 {
			if f, err = strptime(s, format); err == errNoMatch {
				return object.Value{}, invalid(ctx)
			} else if err != nil {
				return object.Value{}, ctx.Raise(object.ArgumentError, "strptime: %s", err.Error())
			}
		}
		return dateFromFrags(ctx, self.Cls(), f, sg)
	})
}
