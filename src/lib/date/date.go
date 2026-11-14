// Package date is Ruby's date library, built into cameo: Date and
// DateTime, with Ruby's calendar arithmetic, parsing and formatting.
//
// It follows date 3.4.1, the version Ruby 3.4 ships. A Date is an instance
// with the fields Ruby keeps: the Julian Day Number and the seconds and
// nanoseconds of the day in UTC, the offset, and the start of the
// Gregorian calendar; a DateTime is the same with a time of day. Where
// Ruby returns a Rational, cameo returns an Integer when the value is
// whole and a Float otherwise: Date - Date, ajd, day_fraction,
// sec_fraction and offset.
package date

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

// The fields of a Date: Ruby's, in the order its inspect shows them.
const (
	fJD = iota // the Julian Day Number of the UTC day
	fDF        // seconds into the UTC day
	fSF        // nanoseconds into the second
	fOF        // the offset from UTC, in seconds
	fSG        // the start of the Gregorian calendar, a Julian Day Number or ±Infinity
	fieldCount
)

var fieldNames = [fieldCount]string{"jd", "df", "sf", "of", "sg"}

var (
	dateClass     = object.NewClass("Date", object.ObjectClass)
	dateTimeClass = object.NewClass("DateTime", dateClass)
	errorClass    = object.NewClass("Error", object.ArgumentError)
)

// dt is a Date as Go reads its fields.
type dt struct {
	jd, df, sf, of int64
	sg             float64
}

func read(v object.Value) dt {
	f := v.Instance().Fields
	num := func(i int) int64 {
		n, _ := f[i].AsInt()
		return n
	}
	sg, _ := f[fSG].AsFloat()
	return dt{jd: num(fJD), df: num(fDF), sf: num(fSF), of: num(fOF), sg: sg}
}

func build(cls *object.Class, d dt) object.Value {
	fields := make([]object.Value, len(cls.Fields))
	fields[fJD], fields[fDF], fields[fSF], fields[fOF] = object.Int(d.jd), object.Int(d.df), object.Int(d.sf), object.Int(d.of)
	fields[fSG] = object.Float(d.sg)
	return object.InstanceValue(&object.Instance{Cls: cls, Fields: fields})
}

func isDate(v object.Value) bool {
	inst, ok := v.AsInstance()
	return ok && inst.Cls.IsA(dateClass)
}

func isDateTime(v object.Value) bool {
	inst, ok := v.AsInstance()
	return ok && inst.Cls.IsA(dateTimeClass)
}

// local is the Julian Day Number and the seconds of the day where the
// date's offset applies.
func (d dt) local() (jd, df int64) {
	s := d.df + d.of
	jd = d.jd
	if s < 0 {
		jd--
		s += daySecs
	} else if s >= daySecs {
		jd++
		s -= daySecs
	}
	return jd, s
}

// fromLocal makes a dt of a local day and time of day under an offset.
func fromLocal(jd, df, sf, of int64, sg float64) dt {
	s := df - of
	if s < 0 {
		jd--
		s += daySecs
	} else if s >= daySecs {
		jd++
		s -= daySecs
	}
	return dt{jd: jd, df: s, sf: sf, of: of, sg: sg}
}

func (d dt) civil() (y, m, day int64) {
	jd, _ := d.local()
	return jdToCivil(jd, d.sg)
}

func (d dt) clock() (h, min, s int64) {
	_, df := d.local()
	return df / 3600, df / 60 % 60, df % 60
}

func (d dt) gregorian() bool { return float64(d.jd) >= d.sg }

// Install defines Date and DateTime, and Time's conversions to them.
// lib.Install calls it once.
func Install() {
	for _, c := range []*object.Class{dateClass, dateTimeClass} {
		object.DefineNative(c.Name, object.ClassValue(c))
	}
	errorClass.Lexical = dateClass
	dateClass.Consts["Error"] = object.ClassValue(errorClass)
	dateClass.Include(object.Comparable)
	declareFields(dateClass)
	for _, f := range dateClass.Fields {
		dateTimeClass.FieldIndex[f.Name] = len(dateTimeClass.Fields)
		dateTimeClass.Fields = append(dateTimeClass.Fields, f)
	}
	dateClass.Consts["ITALY"] = object.Int(italy)
	dateClass.Consts["ENGLAND"] = object.Int(england)
	dateClass.Consts["JULIAN"] = object.Float(julianStart)
	dateClass.Consts["GREGORIAN"] = object.Float(gregorianStart)
	dateClass.Consts["VERSION"] = object.NewString("3.4.1", object.ASCII)
	names := func(ns []string, leadingNil bool) object.Value {
		var out []object.Value
		if leadingNil {
			out = append(out, object.Nil)
		}
		for _, n := range ns {
			out = append(out, object.NewString(n, object.UTF8))
		}
		return object.NewList(out...)
	}
	dateClass.Consts["MONTHNAMES"] = names(monthNames, true)
	dateClass.Consts["ABBR_MONTHNAMES"] = names(abbrMonthNames, true)
	dateClass.Consts["DAYNAMES"] = names(dayNames, false)
	dateClass.Consts["ABBR_DAYNAMES"] = names(abbrDayNames, false)

	installConstructors()
	installReaders()
	installArithmetic()
	installFormatting()
	installParsing()
	installTimeConversions()
}

var (
	monthNames     = []string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
	abbrMonthNames = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	dayNames       = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	abbrDayNames   = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
)

// declareFields gives Date its fields: Integers, and the Float start.
func declareFields(c *object.Class) {
	decls := make([]stdlib.FieldDecl, len(fieldNames))
	for i, name := range fieldNames {
		decls[i] = stdlib.FieldDecl{Name: name, Type: "int"}
		if i == fSG {
			decls[i].Type = "float"
		}
	}
	stdlib.DeclareFields(c, decls...)
}

type fn = object.NativeFn

func invalid(ctx object.Context) error { return ctx.Raise(errorClass, "invalid date") }

// number reads a numeric argument as an Integer and the fraction it
// carries, or fails as Ruby does for a non-numeric one.
func number(ctx object.Context, v object.Value, what string) (whole int64, frac float64, err error) {
	switch {
	case v.Kind() == object.KindInt:
		return v.Int(), 0, nil
	case v.Kind() == object.KindFloat:
		f := v.Float()
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, 0, ctx.Raise(object.FloatDomainError, "%v", f)
		}
		w := math.Floor(f)
		return int64(w), f - w, nil
	case v.Kind() == object.KindBig:
		return 0, 0, ctx.Raise(object.RangeError, "bignum too big to convert into 'long'")
	}
	return 0, 0, ctx.Raise(object.TypeError, "invalid %s (not numeric)", what)
}

// startArg reads a start argument: an Integer Julian Day Number, or
// Date::JULIAN or Date::GREGORIAN.
func startArg(ctx object.Context, v object.Value) (float64, error) {
	if v.IsNil() {
		return italy, nil
	}
	f, ok := v.AsNumber()
	if !ok {
		return 0, ctx.Raise(object.TypeError, "invalid start (not numeric)")
	}
	return f, nil
}

// offsetArg reads an offset argument: a number of days, a string such as
// "+09:00", or nil for zero. Ruby ignores an offset it cannot read, with a
// warning, and uses zero. An IANA zone name, cameo's own, is returned as
// the zone instead, since its offset depends on the moment: timeOf
// resolves it at the civil time of the date being built, new_offset at the
// instant.
func offsetArg(ctx object.Context, v object.Value) (int64, *time.Location, error) {
	if v.IsNil() {
		return 0, nil, nil
	}
	if s, ok := v.AsString(); ok {
		if strings.Contains(s, "/") {
			loc, err := stdlib.LoadZone(ctx, s)
			return 0, loc, err
		}
		if off, ok := zoneToDiff(s); ok {
			return off, nil, nil
		}
		return 0, nil, nil
	}
	f, ok := v.AsNumber()
	if !ok {
		return 0, nil, ctx.Raise(object.TypeError, "invalid offset (not numeric)")
	}
	if f < -1 || f > 1 {
		// Ruby ignores an offset it cannot read, with a warning only in
		// verbose mode.
		return 0, nil, nil
	}
	return int64(math.Round(f * daySecs)), nil, nil
}

// args reads the positional arguments of a constructor: each as an
// Integer with its fraction, the given ones replacing the defaults.
func args(ctx object.Context, in []object.Value, defaults []int64, names []string) ([]int64, []float64, error) {
	out := append([]int64(nil), defaults...)
	fracs := make([]float64, len(defaults))
	if len(in) > len(defaults) {
		return nil, nil, stdlib.ArgError(ctx, len(in), "0.."+strconv.Itoa(len(defaults)))
	}
	for i, v := range in {
		w, f, err := number(ctx, v, names[i])
		if err != nil {
			return nil, nil, err
		}
		out[i], fracs[i] = w, f
	}
	return out, fracs, nil
}

// clockOf reads an hour, minute and second, each counted from the end
// when negative, as the seconds of the day, or false when they are not a
// time of day. 24:00:00 is the next day's midnight.
func clockOf(h, m, s int64) (df int64, carry int64, ok bool) {
	if h < 0 {
		h += 24
	}
	if m < 0 {
		m += 60
	}
	if s < 0 {
		s += 60
	}
	if h == 24 && m == 0 && s == 0 {
		return 0, 1, true
	}
	if h < 0 || h > 23 || m < 0 || m > 59 || s < 0 || s > 59 {
		return 0, 0, false
	}
	return h*3600 + m*60 + s, 0, true
}

func newDate(cls *object.Class, jd int64, sg float64) object.Value {
	return build(cls, dt{jd: jd, sg: sg})
}

// newDateTime makes a DateTime of a local day, a time of day and an
// offset.
func newDateTime(cls *object.Class, jd, df, sf, of int64, sg float64) object.Value {
	return build(cls, fromLocal(jd, df, sf, of, sg))
}

func installConstructors() {
	d := dateClass
	// Date.new and Date.civil take up to four arguments, the fourth the
	// start; args reads three, so the start is read apart.
	withStart := func(f func(ctx object.Context, cls *object.Class, a []object.Value, sg float64) (object.Value, error), max int) fn {
		return func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			sg := float64(italy)
			if len(a) > max {
				return object.Value{}, stdlib.ArgError(ctx, len(a), "0.."+strconv.Itoa(max))
			}
			if len(a) == max {
				var err error
				if sg, err = startArg(ctx, a[max-1]); err != nil {
					return object.Value{}, err
				}
				a = a[:max-1]
			}
			return f(ctx, self.Cls(), a, sg)
		}
	}
	d.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if self.Cls().IsA(dateTimeClass) {
			return withStart(newCivilDateTime, 8)(ctx, self, a, kw, block)
		}
		return withStart(func(ctx object.Context, cls *object.Class, a []object.Value, sg float64) (object.Value, error) {
			vals, _, err := args(ctx, a, []int64{-4712, 1, 1}, []string{"year", "month", "day"})
			if err != nil {
				return object.Value{}, err
			}
			return civilDate(ctx, cls, vals[0], vals[1], vals[2], sg)
		}, 4)(ctx, self, a, kw, block)
	})
	d.DefineSingleton("civil", -1, d.SingletonMethods["new"].Native)
	d.DefineSingleton("jd", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		cls := self.Cls()
		if cls.IsA(dateTimeClass) {
			return withStart(func(ctx object.Context, cls *object.Class, a []object.Value, sg float64) (object.Value, error) {
				a, of := popOffset(a, 5)
				vals, fracs, err := args(ctx, a, []int64{0, 0, 0, 0}, []string{"jd", "hour", "minute", "second"})
				if err != nil {
					return object.Value{}, err
				}
				return timeOf(ctx, cls, vals[0], vals[1], vals[2], vals[3], fracs[3], of, sg)
			}, 6)(ctx, self, a, kw, block)
		}
		return withStart(func(ctx object.Context, cls *object.Class, a []object.Value, sg float64) (object.Value, error) {
			vals, fracs, err := args(ctx, a, []int64{0}, []string{"jd"})
			if err != nil {
				return object.Value{}, err
			}
			return build(cls, withFraction(dt{jd: vals[0], sg: sg}, fracs[0])), nil
		}, 2)(ctx, self, a, kw, block)
	})
	d.DefineSingleton("ordinal", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		cls := self.Cls()
		if cls.IsA(dateTimeClass) {
			return withStart(func(ctx object.Context, cls *object.Class, a []object.Value, sg float64) (object.Value, error) {
				a, of := popOffset(a, 6)
				vals, fracs, err := args(ctx, a, []int64{-4712, 1, 0, 0, 0}, []string{"year", "yday", "hour", "minute", "second"})
				if err != nil {
					return object.Value{}, err
				}
				jd, ok := validOrdinal(vals[0], vals[1], sg)
				if !ok {
					return object.Value{}, invalid(ctx)
				}
				return timeOf(ctx, cls, jd, vals[2], vals[3], vals[4], fracs[4], of, sg)
			}, 7)(ctx, self, a, kw, block)
		}
		return withStart(func(ctx object.Context, cls *object.Class, a []object.Value, sg float64) (object.Value, error) {
			vals, fracs, err := args(ctx, a, []int64{-4712, 1}, []string{"year", "yday"})
			if err != nil {
				return object.Value{}, err
			}
			jd, ok := validOrdinal(vals[0], vals[1], sg)
			if !ok {
				return object.Value{}, invalid(ctx)
			}
			return build(cls, withFraction(dt{jd: jd, sg: sg}, fracs[1])), nil
		}, 3)(ctx, self, a, kw, block)
	})
	d.DefineSingleton("commercial", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		cls := self.Cls()
		if cls.IsA(dateTimeClass) {
			return withStart(func(ctx object.Context, cls *object.Class, a []object.Value, sg float64) (object.Value, error) {
				a, of := popOffset(a, 7)
				vals, fracs, err := args(ctx, a, []int64{-4712, 1, 1, 0, 0, 0}, []string{"cwyear", "cweek", "cwday", "hour", "minute", "second"})
				if err != nil {
					return object.Value{}, err
				}
				jd, ok := validCommercial(vals[0], vals[1], vals[2], sg)
				if !ok {
					return object.Value{}, invalid(ctx)
				}
				return timeOf(ctx, cls, jd, vals[3], vals[4], vals[5], fracs[5], of, sg)
			}, 8)(ctx, self, a, kw, block)
		}
		return withStart(func(ctx object.Context, cls *object.Class, a []object.Value, sg float64) (object.Value, error) {
			vals, fracs, err := args(ctx, a, []int64{-4712, 1, 1}, []string{"cwyear", "cweek", "cwday"})
			if err != nil {
				return object.Value{}, err
			}
			jd, ok := validCommercial(vals[0], vals[1], vals[2], sg)
			if !ok {
				return object.Value{}, invalid(ctx)
			}
			return build(cls, withFraction(dt{jd: jd, sg: sg}, fracs[2])), nil
		}, 4)(ctx, self, a, kw, block)
	})
	d.DefineSingleton("today", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		sg := float64(italy)
		if len(a) > 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(a), "0..1")
		}
		if len(a) == 1 {
			var err error
			if sg, err = startArg(ctx, a[0]); err != nil {
				return object.Value{}, err
			}
		}
		now := time.Now()
		jd := civilToJD(int64(now.Year()), int64(now.Month()), int64(now.Day()), gregorianStart)
		return newDate(self.Cls(), jd, sg), nil
	})
	dateTimeClass.DefineSingleton("now", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		sg := float64(italy)
		if len(a) > 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(a), "0..1")
		}
		if len(a) == 1 {
			var err error
			if sg, err = startArg(ctx, a[0]); err != nil {
				return object.Value{}, err
			}
		}
		return fromTime(self.Cls(), time.Now(), sg), nil
	})

	valid := func(name string, n int, check func(vals []int64, sg float64) bool) {
		d.DefineSingleton(name, -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(a) < n || len(a) > n+1 {
				return object.Value{}, stdlib.ArgError(ctx, len(a), fmt.Sprintf("%d..%d", n, n+1))
			}
			sg := float64(italy)
			if len(a) == n+1 {
				var err error
				if sg, err = startArg(ctx, a[n]); err != nil {
					return object.Value{}, err
				}
			}
			vals := make([]int64, n)
			for i := 0; i < n; i++ {
				w, _, err := number(ctx, a[i], "argument")
				if err != nil {
					return object.Value{}, err
				}
				vals[i] = w
			}
			return object.Bool(check(vals, sg)), nil
		})
	}
	valid("valid_date?", 3, func(v []int64, sg float64) bool { _, ok := validCivil(v[0], v[1], v[2], sg); return ok })
	valid("valid_civil?", 3, func(v []int64, sg float64) bool { _, ok := validCivil(v[0], v[1], v[2], sg); return ok })
	valid("valid_jd?", 1, func(v []int64, sg float64) bool { return true })
	valid("valid_ordinal?", 2, func(v []int64, sg float64) bool { _, ok := validOrdinal(v[0], v[1], sg); return ok })
	valid("valid_commercial?", 3, func(v []int64, sg float64) bool { _, ok := validCommercial(v[0], v[1], v[2], sg); return ok })
	leap := func(name string, f func(int64) bool) {
		d.DefineSingleton(name, 1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			y, _, err := number(ctx, a[0], "year")
			if err != nil {
				return object.Value{}, err
			}
			return object.Bool(f(y)), nil
		})
	}
	leap("leap?", gregorianLeap)
	leap("gregorian_leap?", gregorianLeap)
	leap("julian_leap?", julianLeap)
}

// popOffset takes the offset off a DateTime constructor's arguments when
// it is the nth, and returns the rest with the offset argument, which
// timeOf resolves once it knows the civil time.
func popOffset(a []object.Value, n int) ([]object.Value, object.Value) {
	if len(a) < n {
		return a, object.Nil
	}
	return a[:n-1], a[n-1]
}

// civilDate is Date.civil's result: the day. A fraction of the day
// argument is dropped, as Ruby's civil drops it, where jd, ordinal and
// commercial keep theirs.
func civilDate(ctx object.Context, cls *object.Class, y, m, d int64, sg float64) (object.Value, error) {
	jd, ok := validCivil(y, m, d, sg)
	if !ok {
		return object.Value{}, invalid(ctx)
	}
	return build(cls, dt{jd: jd, sg: sg}), nil
}

// withFraction adds a fraction of a day to a date.
func withFraction(d dt, frac float64) dt {
	if frac == 0 {
		return d
	}
	return addSeconds(d, frac*daySecs)
}

// newCivilDateTime is DateTime.civil(year, month, day, hour, minute,
// second, offset, start).
func newCivilDateTime(ctx object.Context, cls *object.Class, a []object.Value, sg float64) (object.Value, error) {
	if len(a) > 7 {
		return object.Value{}, stdlib.ArgError(ctx, len(a), "0..8")
	}
	offset := object.Nil
	if len(a) == 7 {
		offset, a = a[6], a[:6]
	}
	vals, fracs, err := args(ctx, a, []int64{-4712, 1, 1, 0, 0, 0}, []string{"year", "month", "day", "hour", "minute", "second"})
	if err != nil {
		return object.Value{}, err
	}
	jd, ok := validCivil(vals[0], vals[1], vals[2], sg)
	if !ok {
		return object.Value{}, invalid(ctx)
	}
	return timeOf(ctx, cls, jd, vals[3], vals[4], vals[5], fracs[5], offset, sg)
}

// timeOf is a DateTime of a local day and a time of day, under the offset
// argument: an offset as it is, or a zone's offset at that civil time.
func timeOf(ctx object.Context, cls *object.Class, jd, h, m, s int64, frac float64, offset object.Value, sg float64) (object.Value, error) {
	df, carry, ok := clockOf(h, m, s)
	if !ok {
		return object.Value{}, invalid(ctx)
	}
	of, loc, err := offsetArg(ctx, offset)
	if err != nil {
		return object.Value{}, err
	}
	sf := int64(math.Round(frac * nanos))
	if loc != nil {
		// Go's time places a Gregorian civil time in the zone, whatever
		// calendar the date was given in; a time the spring change skips
		// moves forward by the hour, as Time.new's does.
		y, mo, d := jdToCivil(jd+carry, gregorianStart)
		t := time.Date(int(y), time.Month(mo), int(d), int(df/3600), int(df/60%60), int(df%60), int(sf), loc)
		return fromTime(cls, t, sg), nil
	}
	return newDateTime(cls, jd+carry, df, sf, of, sg), nil
}

// fromTime is a DateTime of a Time, with the Time's offset.
func fromTime(cls *object.Class, t time.Time, sg float64) object.Value {
	_, off := t.Zone()
	jd := civilToJD(int64(t.Year()), int64(t.Month()), int64(t.Day()), gregorianStart)
	df := int64(t.Hour()*3600 + t.Minute()*60 + t.Second())
	return newDateTime(cls, jd, df, int64(t.Nanosecond()), int64(off), sg)
}

// addSeconds adds seconds, fractional ones included, to a date.
func addSeconds(d dt, secs float64) dt {
	whole := math.Floor(secs)
	ns := d.sf + int64(math.Round((secs-whole)*nanos))
	total := d.df + int64(whole) + floorDiv(ns, nanos)
	d.sf = floorMod(ns, nanos)
	d.jd += floorDiv(total, daySecs)
	d.df = floorMod(total, daySecs)
	return d
}

// addDays adds a number of days, a fraction included, to a date.
func addDays(ctx object.Context, d dt, v object.Value) (dt, error) {
	switch v.Kind() {
	case object.KindInt:
		d.jd += v.Int()
		return d, nil
	case object.KindFloat:
		f := v.Float()
		whole := math.Floor(f)
		d.jd += int64(whole)
		return addSeconds(d, (f-whole)*daySecs), nil
	case object.KindBig:
		return d, ctx.Raise(object.RangeError, "bignum too big to convert into 'long'")
	}
	return d, ctx.Raise(object.TypeError, "expected numeric")
}

// days is the difference of two dates in days: an Integer when whole, a
// Float otherwise.
func days(a, b dt) object.Value {
	jd := a.jd - b.jd
	secs := a.df - b.df
	ns := a.sf - b.sf
	if secs == 0 && ns == 0 {
		return object.Int(jd)
	}
	return object.Float(float64(jd) + (float64(secs)+float64(ns)/nanos)/daySecs)
}

// ratio is a Rational of Ruby's as cameo gives it: an Integer when whole,
// a Float otherwise.
func ratio(num, den float64) object.Value {
	if den != 0 && math.Mod(num, den) == 0 {
		return object.Int(int64(num / den))
	}
	return object.Float(num / den)
}

func installReaders() {
	c := dateClass
	reader := func(name string, f func(d dt) object.Value) {
		c.Define(name, 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return f(read(self)), nil
		})
	}
	for _, name := range []string{"year", "month", "mon", "day", "mday", "yday", "wday", "cwday", "cweek", "cwyear"} {
		name := name
		reader(name, func(d dt) object.Value {
			jd, _ := d.local()
			switch name {
			case "year":
				y, _, _ := d.civil()
				return object.Int(y)
			case "month", "mon":
				_, m, _ := d.civil()
				return object.Int(m)
			case "day", "mday":
				_, _, day := d.civil()
				return object.Int(day)
			case "yday":
				_, yd := jdToOrdinal(jd, d.sg)
				return object.Int(yd)
			case "wday":
				return object.Int(wday(jd))
			case "cwday":
				_, _, cd := jdToCommercial(jd, d.sg)
				return object.Int(cd)
			case "cweek":
				_, w, _ := jdToCommercial(jd, d.sg)
				return object.Int(w)
			}
			y, _, _ := jdToCommercial(jd, d.sg)
			return object.Int(y)
		})
	}
	reader("jd", func(d dt) object.Value { jd, _ := d.local(); return object.Int(jd) })
	reader("mjd", func(d dt) object.Value { jd, _ := d.local(); return object.Int(jd - 2400001) })
	reader("ld", func(d dt) object.Value { jd, _ := d.local(); return object.Int(jd - 2299160) })
	reader("ajd", func(d dt) object.Value {
		return ratio(float64(d.jd)*daySecs*2-daySecs+float64(d.df)*2+float64(d.sf)*2/nanos, daySecs*2)
	})
	reader("amjd", func(d dt) object.Value {
		return ratio((float64(d.jd)-2400001)*daySecs+float64(d.df)+float64(d.sf)/nanos, daySecs)
	})
	reader("day_fraction", func(d dt) object.Value {
		_, df := d.local()
		return ratio(float64(df)+float64(d.sf)/nanos, daySecs)
	})
	reader("start", func(d dt) object.Value { return object.Float(d.sg) })
	reader("julian?", func(d dt) object.Value { return object.Bool(!d.gregorian()) })
	reader("gregorian?", func(d dt) object.Value { return object.Bool(d.gregorian()) })
	reader("leap?", func(d dt) object.Value {
		y, _, _ := d.civil()
		if d.gregorian() {
			return object.Bool(gregorianLeap(y))
		}
		return object.Bool(julianLeap(y))
	})
	reader("infinite?", func(d dt) object.Value { return object.False })
	for i, name := range dayNames {
		i := i
		reader(strings.ToLower(name)+"?", func(d dt) object.Value { jd, _ := d.local(); return object.Bool(wday(jd) == int64(i)) })
	}
	c.Define("new_start", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		sg := float64(italy)
		if len(a) > 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(a), "0..1")
		}
		if len(a) == 1 {
			var err error
			if sg, err = startArg(ctx, a[0]); err != nil {
				return object.Value{}, err
			}
		}
		d := read(self)
		d.sg = sg
		return build(self.Instance().Cls, d), nil
	})
	for name, sg := range map[string]float64{"italy": italy, "england": england, "julian": julianStart, "gregorian": gregorianStart} {
		sg := sg
		reader(name, func(d dt) object.Value { d.sg = sg; return build(dateClass, d) })
	}
	// The readers above build a Date; a DateTime keeps its class.
	for _, name := range []string{"italy", "england", "julian", "gregorian"} {
		name := name
		m := c.Methods[name].Native
		c.Define(name, 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			v, err := m(ctx, self, a, kw, block)
			if err != nil {
				return v, err
			}
			return build(self.Instance().Cls, read(v)), nil
		})
	}

	// DateTime's time of day.
	t := dateTimeClass
	treader := func(name string, f func(d dt) object.Value) {
		t.Define(name, 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return f(read(self)), nil
		})
	}
	treader("hour", func(d dt) object.Value { h, _, _ := d.clock(); return object.Int(h) })
	treader("min", func(d dt) object.Value { _, m, _ := d.clock(); return object.Int(m) })
	treader("minute", func(d dt) object.Value { _, m, _ := d.clock(); return object.Int(m) })
	treader("sec", func(d dt) object.Value { _, _, s := d.clock(); return object.Int(s) })
	treader("second", func(d dt) object.Value { _, _, s := d.clock(); return object.Int(s) })
	treader("sec_fraction", func(d dt) object.Value { return ratio(float64(d.sf), nanos) })
	treader("second_fraction", func(d dt) object.Value { return ratio(float64(d.sf), nanos) })
	treader("offset", func(d dt) object.Value { return ratio(float64(d.of), daySecs) })
	treader("zone", func(d dt) object.Value { return object.NewString(offsetString(d.of, true), object.UTF8) })
	t.Define("new_offset", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(a) > 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(a), "0..1")
		}
		d := read(self)
		of := int64(0)
		if len(a) == 1 {
			var loc *time.Location
			var err error
			if of, loc, err = offsetArg(ctx, a[0]); err != nil {
				return object.Value{}, err
			}
			if loc != nil {
				// A zone name's offset is the one in force at this instant.
				_, off := time.Unix((d.jd-unixEpoch)*daySecs+d.df, 0).In(loc).Zone()
				of = int64(off)
			}
		}
		d.of = of
		return build(self.Instance().Cls, d), nil
	})
}

// offsetString is an offset as "+HH:MM", or "+HHMM" without the colon.
func offsetString(of int64, colon bool) string {
	sign := "+"
	if of < 0 {
		sign, of = "-", -of
	}
	h, m := of/3600, of/60%60
	if colon {
		return fmt.Sprintf("%s%02d:%02d", sign, h, m)
	}
	return fmt.Sprintf("%s%02d%02d", sign, h, m)
}

func installArithmetic() {
	c := dateClass
	c.Define("+", 1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		d, err := addDays(ctx, read(self), a[0])
		if err != nil {
			return object.Value{}, err
		}
		return build(self.Instance().Cls, d), nil
	})
	c.Define("-", 1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if isDate(a[0]) {
			return days(read(self), read(a[0])), nil
		}
		neg := a[0]
		switch neg.Kind() {
		case object.KindInt:
			neg = object.Int(-neg.Int())
		case object.KindFloat:
			neg = object.Float(-neg.Float())
		}
		d, err := addDays(ctx, read(self), neg)
		if err != nil {
			return object.Value{}, err
		}
		return build(self.Instance().Cls, d), nil
	})
	shift := func(ctx object.Context, self object.Value, n int64) (object.Value, error) {
		d := read(self)
		y, m, day := d.civil()
		t := y*12 + (m - 1) + n
		y2, m2 := floorDiv(t, 12), floorMod(t, 12)+1
		var jd int64
		for day2 := day; day2 >= 1; day2-- {
			if j, ok := validCivil(y2, m2, day2, d.sg); ok {
				jd = j
				break
			}
			if day2 == 1 {
				return object.Value{}, invalid(ctx)
			}
		}
		local, _ := d.local()
		d.jd += jd - local
		return build(self.Instance().Cls, d), nil
	}
	months := func(name string, sign int64, unit int64) {
		c.Define(name, -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			n := int64(1)
			if len(a) > 1 {
				return object.Value{}, stdlib.ArgError(ctx, len(a), "0..1")
			}
			if len(a) == 1 {
				var err error
				if n, _, err = number(ctx, a[0], "argument"); err != nil {
					return object.Value{}, err
				}
			}
			return shift(ctx, self, sign*n*unit)
		})
	}
	c.Define(">>", 1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, _, err := number(ctx, a[0], "argument")
		if err != nil {
			return object.Value{}, err
		}
		return shift(ctx, self, n)
	})
	c.Define("<<", 1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, _, err := number(ctx, a[0], "argument")
		if err != nil {
			return object.Value{}, err
		}
		return shift(ctx, self, -n)
	})
	months("next_month", 1, 1)
	months("prev_month", -1, 1)
	months("next_year", 1, 12)
	months("prev_year", -1, 12)
	daysFn := func(name string, sign int64) {
		c.Define(name, -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			n := object.Int(1)
			if len(a) > 1 {
				return object.Value{}, stdlib.ArgError(ctx, len(a), "0..1")
			}
			if len(a) == 1 {
				n = a[0]
			}
			if sign < 0 {
				switch n.Kind() {
				case object.KindInt:
					n = object.Int(-n.Int())
				case object.KindFloat:
					n = object.Float(-n.Float())
				}
			}
			d, err := addDays(ctx, read(self), n)
			if err != nil {
				return object.Value{}, err
			}
			return build(self.Instance().Cls, d), nil
		})
	}
	daysFn("next_day", 1)
	daysFn("prev_day", -1)
	succ := func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		d := read(self)
		d.jd++
		return build(self.Instance().Cls, d), nil
	}
	c.Define("next", 0, succ)
	c.Define("succ", 0, succ)

	c.Define("<=>", 1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if !isDate(a[0]) {
			if f, ok := a[0].AsNumber(); ok {
				d := read(self)
				ajd := float64(d.jd) - 0.5 + (float64(d.df)+float64(d.sf)/nanos)/daySecs
				return object.Int(int64(compareFloat(ajd, f))), nil
			}
			return object.Nil, nil
		}
		return object.Int(int64(compare(read(self), read(a[0])))), nil
	})
	c.Define("==", 1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if !isDate(a[0]) {
			return object.False, nil
		}
		return object.Bool(compare(read(self), read(a[0])) == 0), nil
	})
	c.Define("===", 1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if !isDate(a[0]) {
			return object.False, nil
		}
		x, _ := read(self).local()
		y, _ := read(a[0]).local()
		return object.Bool(x == y), nil
	})

	// step, upto and downto yield each date from self to the limit; without
	// a block they return an Enumerator over them.
	c.Define("step", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(a) < 1 || len(a) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(a), "1..2")
		}
		by := object.Int(1)
		if len(a) == 2 {
			by = a[1]
		}
		return step(ctx, self, a[0], by, block)
	})
	c.Define("upto", 1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return step(ctx, self, a[0], object.Int(1), block)
	})
	c.Define("downto", 1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return step(ctx, self, a[0], object.Int(-1), block)
	})
}

func compare(a, b dt) int {
	switch {
	case a.jd != b.jd:
		return sign(a.jd - b.jd)
	case a.df != b.df:
		return sign(a.df - b.df)
	}
	return sign(a.sf - b.sf)
}

func compareFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func sign(n int64) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// step is Date#step(limit, by): the dates from self by by while they do
// not pass limit.
func step(ctx object.Context, self, limit, by object.Value, block object.Value) (object.Value, error) {
	if !isDate(limit) {
		return object.Value{}, ctx.Raise(object.TypeError, "expected date")
	}
	dir := 0
	switch by.Kind() {
	case object.KindInt:
		dir = sign(by.Int())
	case object.KindFloat:
		dir = compareFloat(by.Float(), 0)
	default:
		return object.Value{}, ctx.Raise(object.TypeError, "expected numeric")
	}
	if dir == 0 {
		return object.Value{}, ctx.Raise(object.ArgumentError, "step can't be 0")
	}
	each := func(ctx object.Context, yield func(object.Value) error) error {
		cls := self.Instance().Cls
		lim := read(limit)
		for cur := read(self); ; {
			c := compare(cur, lim)
			if dir > 0 && c > 0 || dir < 0 && c < 0 {
				return nil
			}
			if err := yield(build(cls, cur)); err != nil {
				return err
			}
			var err error
			if cur, err = addDays(ctx, cur, by); err != nil {
				return err
			}
		}
	}
	if block.IsZero() {
		return object.EnumeratorValue(&object.Enumerator{Each: each, Recv: self, Meth: "step", Args: []object.Value{limit, by}}), nil
	}
	err := each(ctx, func(v object.Value) error {
		_, err := ctx.Yield(block, v)
		return err
	})
	return self, err
}

// installTimeConversions defines to_date, to_datetime and to_time on
// Date, DateTime and Time.
func installTimeConversions() {
	d, t := dateClass, object.TimeClass
	d.Define("to_date", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		x := read(self)
		jd, _ := x.local()
		return newDate(dateClass, jd, x.sg), nil
	})
	d.Define("to_datetime", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return build(dateTimeClass, read(self)), nil
	})
	d.Define("to_time", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		x := read(self)
		y, m, day := x.civil()
		if isDateTime(self) {
			h, min, s := x.clock()
			return object.TimeValue(time.Date(int(y), time.Month(m), int(day), int(h), int(min), int(s), int(x.sf), time.FixedZone("", int(x.of)))), nil
		}
		return object.TimeValue(time.Date(int(y), time.Month(m), int(day), 0, 0, 0, 0, time.Local)), nil
	})
	t.Define("to_time", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	t.Define("to_date", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		tm := self.Time()
		return newDate(dateClass, civilToJD(int64(tm.Year()), int64(tm.Month()), int64(tm.Day()), gregorianStart), italy), nil
	})
	t.Define("to_datetime", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return fromTime(dateTimeClass, self.Time(), italy), nil
	})
}
