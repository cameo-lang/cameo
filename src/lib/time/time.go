// Package time is Ruby's time library, built into cameo: what require
// "time" adds to Time. Time.parse, iso8601 (xmlschema) and strptime read a
// time, and Time#iso8601 prints one. Time.new's own string form is the
// core's, in src/stdlib.
package time

import (
	"errors"
	"strings"
	gotime "time"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

// parseTime is Time.parse: the first form of stdlib.ParseCandidates that
// reads s, its fields set in the Clock, the rest today's.
func parseTime(s string) (stdlib.Clock, bool) {
	t, c, ok := stdlib.ParseFreeForm(s, gotime.Local)
	if !ok {
		return stdlib.Clock{}, false
	}
	return stdlib.ClockFrom(t, c.Fields), true
}

var xmlSchemaLayouts = []string{
	"2006-01-02T15:04:05.999999999Z07:00", "2006-01-02T15:04:05.999999999-0700", "2006-01-02T15:04:05.999999999-07", "2006-01-02T15:04:05.999999999",
}

// parseXMLSchema is Time.iso8601: a date and time with an optional
// fraction and zone, and nothing else, read with Go's RFC 3339 layouts.
func parseXMLSchema(s string) (stdlib.Clock, bool) {
	t, ok := stdlib.TryLayouts(strings.TrimSpace(s), xmlSchemaLayouts, gotime.Local)
	if !ok {
		return stdlib.Clock{}, false
	}
	f := stdlib.LayoutFields{Year: true, Mon: true, Day: true, Hour: true, Min: true, Sec: true, Frac: true}
	f.Zone = t.Location() != gotime.Local || strings.ContainsAny(s[10:], "Z+-z")
	return stdlib.ClockFrom(t, f), true
}

// strptime reads s by a strftime format through the Go layout the format
// translates to; a format Go has no layout for is an error.
func strptime(s, format string) (stdlib.Clock, error) {
	layouts, fields, err := stdlib.StrptimeLayouts(format)
	if err != nil {
		return stdlib.Clock{}, err
	}
	if fields.Epoch {
		t, ok := stdlib.ParseEpoch(s, format)
		if !ok {
			return stdlib.Clock{}, errNoMatch
		}
		return stdlib.Clock{Year: t.Year(), Mon: int(t.Month()), Day: t.Day(), Hour: t.Hour(), Min: t.Minute(), Sec: t.Second(), Nsec: 0, Loc: t.Location()}, nil
	}
	t, ok := stdlib.TryLayouts(s, layouts, gotime.Local)
	if !ok {
		return stdlib.Clock{}, errNoMatch
	}
	return stdlib.ClockFrom(t, fields), nil
}

var errNoMatch = errors.New("no match")

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
		t, err := strptime(s, f)
		if err == errNoMatch {
			return object.Value{}, ctx.Raise(object.ArgumentError, "invalid date or strptime format - '%s' '%s'", s, f)
		} else if err != nil {
			return object.Value{}, ctx.Raise(object.ArgumentError, "strptime: %s", err.Error())
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
