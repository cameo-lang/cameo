package date

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cameo-lang/cameo/src/stdlib"
)

// Reading a date from a string goes through Go's time package, as Time's
// reading does (src/stdlib/golayout.go): Date.parse tries the forms of
// stdlib.ParseCandidates, strptime translates its format into a Go
// layout, and the fixed formats are Go's layouts of them. What the Go
// layouts cannot say is not read: a week date, an ordinal suffix, an era
// word, a leftover after the format.

// frags are the fragments of a date a parser found, in the order Ruby's
// hash has them.
type frags struct {
	keys   []string
	values map[string]any // int64 for a number, float64 for a fraction, string for a zone
	comp   bool
	bc     bool
}

func newFrags() *frags { return &frags{values: map[string]any{}, comp: true} }

func (f *frags) set(k string, v any) {
	if _, ok := f.values[k]; !ok {
		f.keys = append(f.keys, k)
	}
	f.values[k] = v
}

func (f *frags) has(k string) bool { _, ok := f.values[k]; return ok }

func (f *frags) int(k string) (int64, bool) {
	v, ok := f.values[k].(int64)
	return v, ok
}

func (f *frags) del(k string) {
	if !f.has(k) {
		return
	}
	delete(f.values, k)
	for i, x := range f.keys {
		if x == k {
			f.keys = append(f.keys[:i], f.keys[i+1:]...)
			break
		}
	}
}

var errNoMatch = errors.New("no match")

// fragsFrom takes the fragments keys name from a parsed time, then the
// fraction and the zone when the layout read them.
func fragsFrom(t time.Time, keys []string, fields stdlib.LayoutFields, s string) *frags {
	f := newFrags()
	for _, k := range keys {
		switch k {
		case "year":
			f.set(k, int64(t.Year()))
		case "mon":
			f.set(k, int64(t.Month()))
		case "mday":
			f.set(k, int64(t.Day()))
		case "yday":
			f.set(k, int64(t.YearDay()))
		case "hour":
			f.set(k, int64(t.Hour()))
		case "min":
			f.set(k, int64(t.Minute()))
		case "sec":
			f.set(k, int64(t.Second()))
		case "wday":
			// Go reads a weekday's name without keeping it; the text has it.
			if w, ok := stdlib.WeekdayNamed(s); ok {
				f.set(k, int64(w))
			}
		}
	}
	if fields.Frac && t.Nanosecond() != 0 {
		f.set("sec_fraction", float64(t.Nanosecond())/1e9)
	}
	if fields.Zone {
		zone := stdlib.ZoneText(s)
		if zone != "" {
			f.set("zone", zone)
		}
		_, off := t.Zone()
		f.set("offset", int64(off))
	}
	return f
}

// parse is Date._parse: the first form of stdlib.ParseCandidates that
// reads str. comp, the completion of a two-digit year, is Go's: 69 to 99
// are the 1900s, the rest the 2000s.
func parse(str string, comp bool) *frags {
	t, c, ok := stdlib.ParseFreeForm(str, time.UTC)
	if !ok {
		if f := parseJISX0301(str); len(f.keys) > 0 {
			return f
		}
		return newFrags()
	}
	// The fragments come from the format, not from the candidate's Keys,
	// which name the date's alone.
	return fragsFrom(t, fragmentKeys(c.Format), c.Fields, str)
}

// fragmentKeys are the fragments a strftime format sets, in its order.
func fragmentKeys(format string) []string {
	format = stdlib.ExpandComposites(format)
	var keys []string
	add := func(k string) {
		for _, x := range keys {
			if x == k {
				return
			}
		}
		keys = append(keys, k)
	}
	for i := 0; i < len(format)-1; i++ {
		if format[i] != '%' {
			continue
		}
		i++
		for i < len(format) && strings.IndexByte("-_0^#:0123456789", format[i]) >= 0 {
			i++
		}
		if i >= len(format) {
			break
		}
		switch format[i] {
		case 'Y', 'y':
			add("year")
		case 'm', 'B', 'b', 'h':
			add("mon")
		case 'd', 'e':
			add("mday")
		case 'j':
			add("yday")
		case 'H', 'k', 'I', 'l', 'p', 'P':
			add("hour")
		case 'M':
			add("min")
		case 'S':
			add("sec")
		case 'A', 'a':
			add("wday")
		}
	}
	return keys
}

// strptime is Date._strptime: str read by format through its Go layout,
// errNoMatch when it does not read, and another error for a directive Go
// has no layout for.
func strptime(str, format string) (*frags, error) {
	layouts, fields, err := stdlib.StrptimeLayouts(format)
	if err != nil {
		return nil, err
	}
	if fields.Epoch {
		t, ok := stdlib.ParseEpoch(str, layouts, fields, time.UTC)
		if !ok {
			return nil, errNoMatch
		}
		// Ruby keeps the seconds as written and the fraction apart, both
		// positive for "-1.5": -1 and 1/2.
		secs, nsec := t.Unix(), t.Nanosecond()
		if secs < 0 && nsec != 0 {
			secs, nsec = secs+1, int(1e9)-nsec
		}
		f := newFrags()
		f.set("seconds", secs)
		if fields.Frac && nsec != 0 {
			f.set("sec_fraction", float64(nsec)/1e9)
		}
		if fields.Zone {
			f.set("zone", stdlib.ZoneText(str))
			_, off := t.Zone()
			f.set("offset", int64(off))
		}
		return f, nil
	}
	t, ok := stdlib.TryLayouts(str, layouts, time.UTC)
	if !ok {
		return nil, errNoMatch
	}
	return fragsFrom(t, fragmentKeys(format), fields, str), nil
}

// fixedForm is a fixed format with its layouts and fragments, made once.
type fixedForm struct {
	keys    []string
	layouts []string
	fields  stdlib.LayoutFields
}

// forms translates the formats of a fixed reader once, at start.
func forms(formats ...string) []fixedForm {
	out := make([]fixedForm, 0, len(formats))
	for _, format := range formats {
		layouts, fields, err := stdlib.StrptimeLayouts(format)
		if err != nil {
			panic("date: fixed format " + format + ": " + err.Error())
		}
		out = append(out, fixedForm{keys: fragmentKeys(format), layouts: layouts, fields: fields})
	}
	return out
}

// fixed reads str by the first form that fits, with the fragments in
// Ruby's order for it; nothing read is an empty set of fragments.
func fixed(str string, forms []fixedForm) *frags {
	str = strings.TrimSpace(str)
	for _, f := range forms {
		if t, ok := stdlib.TryLayouts(str, f.layouts, time.UTC); ok {
			return fragsFrom(t, f.keys, f.fields, str)
		}
	}
	return newFrags()
}

// parseISO8601 is Date._iso8601: the calendar and ordinal dates, with a
// time and a zone or without, extended and basic.
var iSO8601Forms = forms(
	"%Y-%m-%dT%H:%M:%S.%N%z", "%Y-%m-%dT%H:%M:%S%z", "%Y-%m-%dT%H:%M:%S.%N", "%Y-%m-%dT%H:%M:%S", "%Y-%m-%dT%H:%M%z", "%Y-%m-%dT%H:%M",
	"%Y-%m-%d", "%Y%m%dT%H%M%S%z", "%Y%m%dT%H%M%S", "%Y%m%d", "%Y-%jT%H:%M:%S%z", "%Y-%j", "%Y%j",
)

func parseISO8601(str string) *frags {
	return fixed(str, iSO8601Forms)
}

// parseRFC3339 is Date._rfc3339: a date, a time and a zone.
var rFC3339Forms = forms(
	"%Y-%m-%dT%H:%M:%S.%N%z", "%Y-%m-%dT%H:%M:%S%z", "%Y-%m-%d %H:%M:%S.%N%z", "%Y-%m-%d %H:%M:%S%z",
)

func parseRFC3339(str string) *frags {
	return fixed(str, rFC3339Forms)
}

// parseXMLSchema is Date._xmlschema: a date with or without a time, and
// a zone or not.
var xMLSchemaForms = forms(
	"%Y-%m-%dT%H:%M:%S.%N%z", "%Y-%m-%dT%H:%M:%S%z", "%Y-%m-%dT%H:%M:%S.%N", "%Y-%m-%dT%H:%M:%S", "%Y-%m-%d%z", "%Y-%m-%d",
)

func parseXMLSchema(str string) *frags {
	return fixed(str, xMLSchemaForms)
}

// parseRFC2822 is Date._rfc2822: the mail date.
var rFC2822Forms = forms(
	"%a, %d %b %Y %H:%M:%S %z", "%a, %-d %b %Y %H:%M:%S %z", "%d %b %Y %H:%M:%S %z", "%-d %b %Y %H:%M:%S %z",
	"%a, %d %b %Y %H:%M %z", "%a, %-d %b %Y %H:%M %z", "%a, %d %b %Y %H:%M:%S %Z", "%a, %-d %b %Y %H:%M:%S %Z",
)

func parseRFC2822(str string) *frags {
	return fixed(str, rFC2822Forms)
}

// parseHTTPDate is Date._httpdate: the three HTTP date forms.
var hTTPDateForms = forms(
	"%a, %d %b %Y %H:%M:%S %Z", "%A, %d-%b-%y %H:%M:%S %Z", "%a %b %e %H:%M:%S %Y",
)

func parseHTTPDate(str string) *frags {
	return fixed(str, hTTPDateForms)
}

var jisx0301Re = regexp.MustCompile(`(?i)\A\s*([mtshr])?(\d{1,2})\.(\d{1,2})\.(\d{1,2})(?:t(?:(\d{2}):(\d{2})(?::(\d{2})(?:[,.](\d*))?)?(z|[-+]\d{2}(?::?\d{2})?)?)?)?\s*\z`)

// parseJISX0301 is Date._jisx0301: an era letter, Meiji to Reiwa, Heisei
// when none, and a year in it, then the month and the day.
func parseJISX0301(str string) *frags {
	m := jisx0301Re.FindStringSubmatch(str)
	if m == nil {
		// Without an era, the ISO 8601 forms, as Ruby's falls back to.
		return parseISO8601(str)
	}
	base := map[string]int64{"m": 1867, "t": 1911, "s": 1925, "h": 1988, "r": 2018}
	era := strings.ToLower(m[1])
	if era == "" {
		era = "h"
	}
	f := newFrags()
	num := func(s string) int64 {
		n, _ := strconv.ParseInt(s, 10, 64)
		return n
	}
	f.set("year", base[era]+num(m[2]))
	f.set("mon", num(m[3]))
	f.set("mday", num(m[4]))
	if m[5] != "" {
		f.set("hour", num(m[5]))
		f.set("min", num(m[6]))
		if m[7] != "" {
			f.set("sec", num(m[7]))
		}
		if m[8] != "" {
			frac, _ := strconv.ParseFloat("0."+m[8], 64)
			f.set("sec_fraction", frac)
		}
		if m[9] != "" {
			f.set("zone", m[9])
			if off, ok := zoneToDiff(m[9]); ok {
				f.set("offset", off)
			}
		}
	}
	return f
}

// zoneToDiff is a zone as seconds east of UTC, read by Go's zone layouts.
func zoneToDiff(zone string) (int64, bool) {
	loc, ok := stdlib.ZoneNamed(strings.TrimSpace(zone))
	if !ok {
		return 0, false
	}
	_, off := time.Now().In(loc).Zone()
	return int64(off), true
}
