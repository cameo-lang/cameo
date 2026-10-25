package stdlib

import (
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cameo-lang/cameo/src/object"
)

// installTime defines Time, an immutable instant with the offset it shows
// in: UTC, the local zone, or a fixed offset such as "+09:00". Ruby's utc
// and localtime change their receiver; cameo's return a new time, as
// getutc and getlocal do. subsec is a Float, since cameo has no Rational.
func installTime() {
	c := object.TimeClass
	tm := func(self object.Value) time.Time { return self.Time() }

	c.DefineSingleton("now", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		loc, err := inKeyword(ctx, kw, time.Local)
		if err != nil {
			return object.Value{}, err
		}
		return object.TimeValue(time.Now().Round(0).In(loc)), nil
	})
	c.DefineSingleton("at", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 3 {
			return object.Value{}, argError(ctx, len(args), "1..3")
		}
		var t time.Time
		loc := time.Local
		if at, ok := args[0].AsTime(); ok {
			t, loc = at, at.Location()
		} else {
			sec, nsec, err := exactSeconds(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			t = time.Unix(sec, nsec)
		}
		if len(args) > 1 {
			unit := int64(1000)
			if len(args) == 3 {
				u, _ := args[2].AsSymbol()
				switch u {
				case "microsecond", "usec":
				case "millisecond":
					unit = 1000000
				case "nanosecond", "nsec":
					unit = 1
				default:
					s, err := ctx.Inspect(args[2])
					if err != nil {
						return object.Value{}, err
					}
					return object.Value{}, ctx.Raise(object.ArgumentError, "unexpected unit: %s", strings.TrimPrefix(s, ":"))
				}
			}
			if n, ok := args[1].AsInt(); ok {
				t = t.Add(time.Duration(n * unit))
			} else if x, ok := args[1].AsFloat(); ok {
				t = t.Add(time.Duration(floorScaled(x, unit)))
			} else {
				return object.Value{}, ctx.Raise(object.TypeError, "can't convert %s into an exact number", kindName(args[1]))
			}
		}
		loc, err := inKeyword(ctx, kw, loc)
		if err != nil {
			return object.Value{}, err
		}
		return object.TimeValue(t.In(loc)), nil
	})
	// newTime is Time.new, or with civil Time.utc and Time.local, whose
	// seventh argument is microseconds, not a zone, and which also take
	// the ten values of Time#to_a.
	newTime := func(defaultLoc *time.Location, civil bool) fn {
		return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			loc, err := inKeyword(ctx, kw, defaultLoc)
			if err != nil {
				return object.Value{}, err
			}
			usec := object.Nil
			switch {
			case civil && len(args) == 10:
				args = []object.Value{args[5], args[4], args[3], args[2], args[1], args[0]}
			case civil && (len(args) == 7 || len(args) == 8):
				usec, args = args[6], args[:6]
			case civil && len(args) > 8:
				return object.Value{}, argError(ctx, len(args), "1..8")
			}
			if !usec.IsNil() {
				t, err := civilTime(ctx, args, loc)
				if err != nil {
					return object.Value{}, err
				}
				ns, err := microseconds(ctx, usec)
				if err != nil {
					return object.Value{}, err
				}
				return object.TimeValue(t.Truncate(time.Second).Add(time.Duration(ns))), nil
			}
			if len(args) == 7 {
				if loc, err = zoneArg(ctx, args[6]); err != nil {
					return object.Value{}, err
				}
				args = args[:6]
			}
			if len(args) > 6 {
				return object.Value{}, argError(ctx, len(args), "0..7")
			}
			if len(args) == 0 {
				return object.TimeValue(time.Now().Round(0).In(loc)), nil
			}
			if s, ok := args[0].AsString(); ok && len(args) == 1 {
				t, err := parseCivil(ctx, s, loc)
				if err != nil {
					return object.Value{}, err
				}
				return object.TimeValue(t), nil
			}
			t, err := civilTime(ctx, args, loc)
			if err != nil {
				return object.Value{}, err
			}
			return object.TimeValue(t), nil
		}
	}
	c.DefineSingleton("new", -1, newTime(time.Local, false))
	c.DefineSingleton("local", -1, newTime(time.Local, true))
	c.DefineSingleton("mktime", -1, newTime(time.Local, true))
	c.DefineSingleton("utc", -1, newTime(time.UTC, true))
	c.DefineSingleton("gm", -1, newTime(time.UTC, true))

	ints := map[string]func(t time.Time) int64{
		"year":  func(t time.Time) int64 { return int64(t.Year()) },
		"month": func(t time.Time) int64 { return int64(t.Month()) },
		"day":   func(t time.Time) int64 { return int64(t.Day()) },
		"hour":  func(t time.Time) int64 { return int64(t.Hour()) },
		"min":   func(t time.Time) int64 { return int64(t.Minute()) },
		"sec":   func(t time.Time) int64 { return int64(t.Second()) },
		"usec":  func(t time.Time) int64 { return int64(t.Nanosecond() / 1000) },
		"nsec":  func(t time.Time) int64 { return int64(t.Nanosecond()) },
		"wday":  func(t time.Time) int64 { return int64(t.Weekday()) },
		"yday":  func(t time.Time) int64 { return int64(t.YearDay()) },
		"to_i":  func(t time.Time) int64 { return t.Unix() },
		"utc_offset": func(t time.Time) int64 {
			_, off := t.Zone()
			return int64(off)
		},
	}
	aliases := map[string][]string{
		"month": {"mon"}, "day": {"mday"}, "usec": {"tv_usec"}, "nsec": {"tv_nsec"},
		"to_i": {"tv_sec"}, "utc_offset": {"gmt_offset", "gmtoff"},
	}
	for name, f := range ints {
		native := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Int(f(tm(self))), nil
		}
		def(c, name, 0, native)
		for _, a := range aliases[name] {
			def(c, a, 0, native)
		}
	}
	for i, day := range []string{"sunday?", "monday?", "tuesday?", "wednesday?", "thursday?", "friday?", "saturday?"} {
		def(c, day, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Bool(int(tm(self).Weekday()) == i), nil
		})
	}
	def(c, "subsec", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		ns := tm(self).Nanosecond()
		if ns == 0 {
			return object.Int(0), nil
		}
		return object.Float(float64(ns) / 1e9), nil
	})
	def(c, "to_f", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		t := tm(self)
		return object.Float(float64(t.Unix()) + float64(t.Nanosecond())/1e9), nil
	})
	def(c, "zone", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if name := zoneName(tm(self)); name != "" {
			return object.String(name), nil
		}
		return object.Nil, nil
	})
	utcP := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(tm(self).Location() == time.UTC), nil
	}
	def(c, "utc?", 0, utcP)
	def(c, "gmt?", 0, utcP)
	def(c, "dst?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(tm(self).IsDST()), nil
	})
	def(c, "isdst", 0, c.Methods["dst?"].Native)
	toUTC := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.TimeValue(tm(self).UTC()), nil
	}
	for _, name := range []string{"utc", "gmtime", "getutc", "getgm"} {
		def(c, name, 0, toUTC)
	}
	toLocal := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 1 {
			return object.Value{}, argError(ctx, len(args), "0..1")
		}
		loc := time.Local
		if len(args) == 1 && !args[0].IsNil() {
			var err error
			if loc, err = zoneArg(ctx, args[0]); err != nil {
				return object.Value{}, err
			}
		}
		return object.TimeValue(tm(self).In(loc)), nil
	}
	def(c, "localtime", -1, toLocal)
	def(c, "getlocal", -1, toLocal)
	timeKeys := []string{"year", "month", "day", "yday", "wday", "hour", "min", "sec", "subsec", "dst", "zone"}
	def(c, "deconstruct_keys", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		keys := timeKeys
		if !args[0].IsNil() {
			l, ok := args[0].AsList()
			if !ok {
				return object.Value{}, ctx.Raise(object.TypeError, "wrong argument type %s (expected Array or nil)", args[0].Class().Name)
			}
			keys = nil
			for _, k := range l.Elems {
				if name, ok := k.AsSymbol(); ok && slices.Contains(timeKeys, name) {
					keys = append(keys, name)
				}
			}
		}
		out := object.NewMap()
		for _, k := range keys {
			m := "dst?"
			if k != "dst" {
				m = k
			}
			v, err := c.Methods[m].Native(ctx, self, nil, nil, object.Value{})
			if err != nil {
				return object.Value{}, err
			}
			out = out.Put(object.Symbol(k), v)
		}
		return object.MapValue(out), nil
	})
	asctime := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(Strftime(tm(self), "%a %b %e %H:%M:%S %Y")), nil
	}
	def(c, "asctime", 0, asctime)
	def(c, "ctime", 0, asctime)
	def(c, "to_a", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		t := tm(self)
		zone := object.Nil
		if name := zoneName(t); name != "" {
			zone = object.String(name)
		}
		return object.NewList(object.Int(t.Second()), object.Int(t.Minute()), object.Int(t.Hour()), object.Int(t.Day()),
			object.Int(int(t.Month())), object.Int(t.Year()), object.Int(int(t.Weekday())), object.Int(t.YearDay()),
			object.Bool(t.IsDST()), zone), nil
	})

	def(c, "+", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if args[0].Kind() == object.KindTime {
			return object.Value{}, ctx.Raise(object.TypeError, "time + time?")
		}
		d, err := duration(ctx, args[0], 1)
		if err != nil {
			return object.Value{}, err
		}
		return object.TimeValue(tm(self).Add(d)), nil
	})
	def(c, "-", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		t := tm(self)
		if o, ok := args[0].AsTime(); ok {
			return object.Float(float64(t.Unix()-o.Unix()) + float64(t.Nanosecond()-o.Nanosecond())/1e9), nil
		}
		d, err := duration(ctx, args[0], -1)
		if err != nil {
			return object.Value{}, err
		}
		return object.TimeValue(t.Add(d)), nil
	})
	def(c, "<=>", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, ok := args[0].AsTime()
		if !ok {
			return object.Nil, nil
		}
		return object.Int(tm(self).Compare(o)), nil
	})
	def(c, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(object.Equal(self, args[0])), nil
	})
	def(c, "eql?", 1, c.Methods["=="].Native)
	def(c, "hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(hashString(object.Key(self))), nil
	})
	rounder := func(mode string) fn {
		return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) > 1 {
				return object.Value{}, argError(ctx, len(args), "0..1")
			}
			digits := int64(0)
			if len(args) == 1 {
				var err error
				if digits, err = wantInt(ctx, args[0], ""); err != nil {
					return object.Value{}, err
				}
				if digits < 0 {
					return object.Value{}, ctx.Raise(object.ArgumentError, "negative ndigits given")
				}
			}
			if digits >= 9 {
				return self, nil
			}
			t := tm(self)
			unit := time.Duration(math.Pow10(9 - int(digits)))
			down := t.Truncate(unit)
			switch {
			case mode == "round":
				t = t.Round(unit)
			case mode == "ceil" && !down.Equal(t):
				t = down.Add(unit)
			default:
				t = down
			}
			return object.TimeValue(t), nil
		}
	}
	def(c, "round", -1, rounder("round"))
	def(c, "floor", -1, rounder("floor"))
	def(c, "ceil", -1, rounder("ceil"))
	def(c, "strftime", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		f, err := wantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.String(Strftime(tm(self), f)), nil
	})
	def(c, "to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(TimeToS(tm(self))), nil
	})
	def(c, "inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(TimeInspect(tm(self))), nil
	})
}

// TimeToS is Time#to_s: "2021-08-22 00:00:00 +0900", or "... UTC".
func TimeToS(t time.Time) string {
	if t.Location() == time.UTC {
		return Strftime(t, "%Y-%m-%d %H:%M:%S UTC")
	}
	return Strftime(t, "%Y-%m-%d %H:%M:%S %z")
}

// TimeInspect is Time#inspect: to_s with the fraction of a second, when
// there is one, in as few digits as it takes.
func TimeInspect(t time.Time) string {
	s := Strftime(t, "%Y-%m-%d %H:%M:%S")
	if ns := t.Nanosecond(); ns != 0 {
		s += "." + strings.TrimRight(padDigits(int64(ns), 9), "0")
	}
	if t.Location() == time.UTC {
		return s + " UTC"
	}
	return s + Strftime(t, " %z")
}

// zoneName is Time#zone: "UTC" for a UTC time, the local zone's name, and
// none for a fixed offset.
func zoneName(t time.Time) string {
	name, _ := t.Zone()
	return name
}

// inKeyword reads the in: keyword of Time.now, Time.at and Time.new.
func inKeyword(ctx object.Context, kw *object.Map, loc *time.Location) (*time.Location, error) {
	if kw == nil {
		return loc, nil
	}
	for _, k := range kw.Keys() {
		if name, _ := k.AsSymbol(); name != "in" {
			s, err := ctx.Inspect(k)
			if err != nil {
				return nil, err
			}
			return nil, ctx.Raise(object.ArgumentError, "unknown keyword: %s", s)
		}
	}
	v, _ := kw.Get(object.Symbol("in"))
	if v.IsNil() {
		return loc, nil
	}
	return zoneArg(ctx, v)
}

// zoneArg reads a UTC offset as Ruby takes one: seconds, "+HH:MM" and its
// variants ("+HH", "+HHMM", "+HH:MM:SS"), "UTC", "Z", or a military zone
// letter. "-00:00" is UTC.
func zoneArg(ctx object.Context, v object.Value) (*time.Location, error) {
	if n, ok := v.AsInt(); ok {
		if n <= -86400 || n >= 86400 {
			return nil, ctx.Raise(object.ArgumentError, "utc_offset out of range")
		}
		return time.FixedZone("", int(n)), nil
	}
	s, ok := v.AsString()
	if !ok {
		return nil, ctx.Raise(object.TypeError, "can't convert %s into an exact number", kindName(v))
	}
	bad := func() error {
		return ctx.Raise(object.ArgumentError, `"+HH:MM", "-HH:MM", "UTC" or "A".."I","K".."Z" expected for utc_offset: %s`, s)
	}
	switch {
	case s == "UTC" || s == "Z" || s == "-00:00":
		return time.UTC, nil
	case len(s) == 1 && 'A' <= s[0] && s[0] <= 'Y' && s[0] != 'J':
		var h int
		switch c := int(s[0]); {
		case c <= 'I':
			h = c - 'A' + 1
		case c <= 'M':
			h = c - 'K' + 10
		default:
			h = -(c - 'N' + 1)
		}
		return time.FixedZone("", h*3600), nil
	case len(s) < 3 || s[0] != '+' && s[0] != '-':
		return nil, bad()
	}
	digits := strings.ReplaceAll(s[1:], ":", "")
	colons := strings.Count(s, ":")
	if colons > 0 && len(s[1:]) != len(digits)+colons || len(digits)%2 != 0 || len(digits) > 6 || !allDigits(digits) {
		return nil, bad()
	}
	if colons > 0 && (s[3] != ':' || len(digits) == 6 && s[6] != ':') {
		return nil, bad()
	}
	parts := []int{0, 0, 0}
	for i := 0; i < len(digits); i += 2 {
		parts[i/2], _ = strconv.Atoi(digits[i : i+2])
	}
	if parts[1] >= 60 || parts[2] >= 60 {
		return nil, bad()
	}
	if parts[0] >= 24 {
		return nil, ctx.Raise(object.ArgumentError, "utc_offset out of range")
	}
	off := parts[0]*3600 + parts[1]*60 + parts[2]
	if s[0] == '-' {
		off = -off
	}
	return time.FixedZone("", off), nil
}

// civilTime is the time Time.new names: year, month (a number or a name
// such as "aug"), day, hour, minute and second, which may have a fraction.
func civilTime(ctx object.Context, args []object.Value, loc *time.Location) (time.Time, error) {
	f := []int64{0, 1, 1, 0, 0, 0}
	var frac float64
	for i, a := range args {
		if a.IsNil() {
			continue
		}
		if i == 1 {
			if s, ok := a.AsString(); ok {
				if m := monthNamed(s); m > 0 {
					f[1] = int64(m)
					continue
				}
				n, ok := parseInt(strings.TrimSpace(s))
				if !ok {
					return time.Time{}, ctx.Raise(object.ArgumentError, "invalid value for Integer(): %s", object.QuoteString(s))
				}
				f[1] = n
				continue
			}
		}
		if i == 5 {
			if x, ok := a.AsFloat(); ok {
				f[5] = int64(math.Floor(x))
				frac = x - math.Floor(x)
				continue
			}
		}
		n, err := wantInt(ctx, a, "")
		if err != nil {
			return time.Time{}, err
		}
		f[i] = n
	}
	switch {
	case f[1] < 1 || f[1] > 12:
		return time.Time{}, ctx.Raise(object.ArgumentError, "mon out of range")
	case f[2] < 1:
		return time.Time{}, ctx.Raise(object.ArgumentError, "mday out of range")
	case f[2] > 31:
		return time.Time{}, ctx.Raise(object.ArgumentError, "argument out of range")
	case f[3] < 0 || f[3] > 24:
		return time.Time{}, ctx.Raise(object.ArgumentError, "hour out of range")
	case f[4] < 0 || f[4] > 59:
		return time.Time{}, ctx.Raise(object.ArgumentError, "min out of range")
	case f[5] < 0 || f[5] > 60:
		return time.Time{}, ctx.Raise(object.ArgumentError, "sec out of range")
	}
	return time.Date(int(f[0]), time.Month(f[1]), int(f[2]), int(f[3]), int(f[4]), int(f[5]), int(floorScaled(frac, 1e9)), loc), nil
}

// microseconds reads the microseconds argument of Time.utc and
// Time.local, a number or a string of digits, as nanoseconds.
func microseconds(ctx object.Context, v object.Value) (int64, error) {
	if s, ok := v.AsString(); ok {
		n, ok := parseInt(strings.TrimSpace(s))
		if !ok {
			return 0, ctx.Raise(object.ArgumentError, "invalid value for Integer(): %s", object.QuoteString(s))
		}
		v = object.Int(n)
	}
	var ns int64
	if x, ok := v.AsFloat(); ok {
		ns = floorScaled(x, 1000)
	} else {
		n, err := wantInt(ctx, v, "")
		if err != nil {
			return 0, err
		}
		if n < 0 || n >= 1e6 {
			return 0, ctx.Raise(object.ArgumentError, "subsecx out of range")
		}
		ns = n * 1000
	}
	if ns < 0 || ns >= 1e9 {
		return 0, ctx.Raise(object.ArgumentError, "subsecx out of range")
	}
	return ns, nil
}

func monthNamed(s string) int {
	for m := time.January; m <= time.December; m++ {
		if strings.EqualFold(s, m.String()[:3]) {
			return int(m)
		}
	}
	return 0
}

// exactSeconds splits a number of seconds into whole seconds and
// nanoseconds.
func exactSeconds(ctx object.Context, v object.Value) (int64, int64, error) {
	if n, ok := v.AsInt(); ok {
		return n, 0, nil
	}
	if x, ok := v.AsFloat(); ok {
		r := new(big.Rat).SetFloat64(x)
		r.Mul(r, big.NewRat(1e9, 1))
		ns := new(big.Int).Div(r.Num(), r.Denom())
		sec, frac := new(big.Int).DivMod(ns, big.NewInt(1e9), new(big.Int))
		return sec.Int64(), frac.Int64(), nil
	}
	return 0, 0, ctx.Raise(object.TypeError, "can't convert %s into an exact number", kindName(v))
}

// duration is sign times a number of seconds as a duration, rounded down
// to the nanosecond as Ruby's exact arithmetic is when it shows a time.
func duration(ctx object.Context, v object.Value, sign int64) (time.Duration, error) {
	if n, ok := v.AsInt(); ok {
		return time.Duration(sign*n) * time.Second, nil
	}
	if x, ok := v.AsFloat(); ok {
		return time.Duration(floorScaled(float64(sign)*x, 1e9)), nil
	}
	return 0, ctx.Raise(object.TypeError, "can't convert %s into an exact number", kindName(v))
}

// floorScaled is x times scale, rounded down, computed exactly: Ruby takes
// a float for the exact binary fraction it is, so 0.1 seconds before a
// whole second is a nanosecond further back than 0.1 * 1e9 says.
func floorScaled(x float64, scale int64) int64 {
	r := new(big.Rat).SetFloat64(x)
	r.Mul(r, new(big.Rat).SetInt64(scale))
	// Div is Euclidean, which for a positive denominator rounds down.
	return new(big.Int).Div(r.Num(), r.Denom()).Int64()
}

// Strftime formats t as Ruby's Time#strftime does, with its flags ("-" no
// padding, "_" spaces, "0" zeros, "^" upper case, "#" swapped case), a
// width, and colons for %z. A directive it does not know is copied as it
// is.
func Strftime(t time.Time, f string) string {
	var b strings.Builder
	for i := 0; i < len(f); i++ {
		if f[i] != '%' {
			b.WriteByte(f[i])
			continue
		}
		start := i
		i++
		var pad byte
		upper, swap := false, false
	flags:
		for ; i < len(f); i++ {
			switch f[i] {
			case '-', '_', '0':
				pad = f[i]
			case '^':
				upper = true
			case '#':
				swap = true
			default:
				break flags
			}
		}
		width := -1
		for ; i < len(f) && isASCIIDigit(rune(f[i])); i++ {
			if width < 0 {
				width = 0
			}
			width = width*10 + int(f[i]-'0')
		}
		colons := 0
		for ; i < len(f) && f[i] == ':'; i++ {
			colons++
		}
		if i >= len(f) {
			b.WriteString(f[start:])
			break
		}
		num := func(v int64, w int, defPad byte) {
			if width >= 0 {
				w = width
			}
			p := defPad
			switch pad {
			case '-':
				w = 0
			case '_':
				p = ' '
			case '0':
				p = '0'
			}
			s := strconv.FormatInt(v, 10)
			neg := v < 0
			if neg {
				s = s[1:]
			}
			if p == '0' {
				if neg {
					w--
				}
				if len(s) < w {
					s = strings.Repeat("0", w-len(s)) + s
				}
				if neg {
					s = "-" + s
				}
			} else {
				if neg {
					s = "-" + s
				}
				if len(s) < w {
					s = strings.Repeat(" ", w-len(s)) + s
				}
			}
			b.WriteString(s)
		}
		text := func(s string, lowerOnSwap bool) {
			switch {
			case upper:
				s = strings.ToUpper(s)
			case swap && lowerOnSwap:
				s = strings.ToLower(s)
			case swap:
				s = strings.ToUpper(s)
			}
			if width > len(s) && pad != '-' {
				p := " "
				if pad == '0' {
					p = "0"
				}
				s = strings.Repeat(p, width-len(s)) + s
			}
			b.WriteString(s)
		}
		hour12 := int64(t.Hour() % 12)
		if hour12 == 0 {
			hour12 = 12
		}
		year := int64(t.Year())
		switch c := f[i]; c {
		case 'Y':
			w := 4
			if year < 0 {
				w = 5
			}
			num(year, w, '0')
		case 'C':
			num(int64(math.Floor(float64(year)/100)), 2, '0')
		case 'y':
			num((year%100+100)%100, 2, '0')
		case 'm':
			num(int64(t.Month()), 2, '0')
		case 'd':
			num(int64(t.Day()), 2, '0')
		case 'e':
			num(int64(t.Day()), 2, ' ')
		case 'j':
			num(int64(t.YearDay()), 3, '0')
		case 'H':
			num(int64(t.Hour()), 2, '0')
		case 'k':
			num(int64(t.Hour()), 2, ' ')
		case 'I':
			num(hour12, 2, '0')
		case 'l':
			num(hour12, 2, ' ')
		case 'M':
			num(int64(t.Minute()), 2, '0')
		case 'S':
			num(int64(t.Second()), 2, '0')
		case 'L', 'N':
			w := 9
			if c == 'L' {
				w = 3
			}
			if width >= 0 {
				w = width
			}
			s := padDigits(int64(t.Nanosecond()), 9)
			if w <= 9 {
				s = s[:w]
			} else {
				s += strings.Repeat("0", w-9)
			}
			b.WriteString(s)
		case 'z':
			_, off := t.Zone()
			sign := byte('+')
			if off < 0 {
				sign, off = '-', -off
			}
			h, m, s := off/3600, off/60%60, off%60
			b.WriteByte(sign)
			b.WriteString(padDigits(int64(h), 2))
			switch {
			case colons == 0:
				b.WriteString(padDigits(int64(m), 2))
			case colons == 1 || colons == 3 && s == 0 && m != 0:
				b.WriteString(":" + padDigits(int64(m), 2))
			case colons == 2 || colons == 3 && s != 0:
				b.WriteString(":" + padDigits(int64(m), 2) + ":" + padDigits(int64(s), 2))
			}
		case 'Z':
			text(zoneName(t), true)
		case 'A':
			text(t.Weekday().String(), false)
		case 'a':
			text(t.Weekday().String()[:3], false)
		case 'B':
			text(t.Month().String(), false)
		case 'b', 'h':
			text(t.Month().String()[:3], false)
		case 'p', 'P':
			s := "AM"
			if t.Hour() >= 12 {
				s = "PM"
			}
			if c == 'P' {
				s = strings.ToLower(s)
			}
			text(s, true)
		case 'u':
			num(int64((int(t.Weekday())+6)%7+1), 1, '0')
		case 'w':
			num(int64(t.Weekday()), 1, '0')
		case 'U':
			num(int64((t.YearDay()-1+7-int(t.Weekday()))/7), 2, '0')
		case 'W':
			num(int64((t.YearDay()-1+7-(int(t.Weekday())+6)%7)/7), 2, '0')
		case 'G':
			y, _ := t.ISOWeek()
			num(int64(y), 4, '0')
		case 'g':
			y, _ := t.ISOWeek()
			num(int64((y%100+100)%100), 2, '0')
		case 'V':
			_, w := t.ISOWeek()
			num(int64(w), 2, '0')
		case 's':
			num(t.Unix(), 1, '0')
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case '%':
			b.WriteByte('%')
		case 'c':
			text(Strftime(t, "%a %b %e %H:%M:%S %Y"), false)
		case 'D', 'x':
			text(Strftime(t, "%m/%d/%y"), false)
		case 'F':
			text(Strftime(t, "%Y-%m-%d"), false)
		case 'T', 'X':
			text(Strftime(t, "%H:%M:%S"), false)
		case 'R':
			text(Strftime(t, "%H:%M"), false)
		case 'r':
			text(Strftime(t, "%I:%M:%S %p"), false)
		case 'v':
			text(Strftime(t, "%e-%^b-%4Y"), false)
		default:
			b.WriteString(f[start : i+1])
		}
	}
	return b.String()
}
