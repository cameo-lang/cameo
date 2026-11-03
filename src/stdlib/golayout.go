package stdlib

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Reading a time from a string goes through Go's time package: a strftime
// format becomes a Go layout, a zone is read with Go's zone layouts, and
// a free-form string is tried against a list of layouts. What Go's layouts
// cannot say, such as a week number or an ordinal suffix, is not read.

// LayoutFields says which parts of a time a layout sets.
type LayoutFields struct {
	Year, Mon, Day, Hour, Min, Sec, Frac, Zone, Wday, Yday, Epoch bool
}

// layoutDirective is one strftime layoutDirective as a Go layout token: alt is a
// second spelling Go needs for the same text, and set what it sets.
type layoutDirective struct {
	token, alt string
	set        func(*LayoutFields)
}

var directives = map[byte]layoutDirective{
	'Y': {"2006", "", func(f *LayoutFields) { f.Year = true }},
	'y': {"06", "", func(f *LayoutFields) { f.Year = true }},
	'm': {"01", "", func(f *LayoutFields) { f.Mon = true }},
	'B': {"January", "", func(f *LayoutFields) { f.Mon = true }},
	'b': {"Jan", "", func(f *LayoutFields) { f.Mon = true }},
	'h': {"Jan", "", func(f *LayoutFields) { f.Mon = true }},
	'd': {"02", "", func(f *LayoutFields) { f.Day = true }},
	'e': {"_2", "", func(f *LayoutFields) { f.Day = true }},
	'j': {"002", "", func(f *LayoutFields) { f.Yday = true }},
	'H': {"15", "", func(f *LayoutFields) { f.Hour = true }},
	'k': {"15", "", func(f *LayoutFields) { f.Hour = true }},
	'I': {"03", "", func(f *LayoutFields) { f.Hour = true }},
	'l': {"3", "", func(f *LayoutFields) { f.Hour = true }},
	'M': {"04", "", func(f *LayoutFields) { f.Min = true }},
	'S': {"05", "", func(f *LayoutFields) { f.Sec = true }},
	'L': {"999", "", func(f *LayoutFields) { f.Frac = true }},
	'N': {"999999999", "", func(f *LayoutFields) { f.Frac = true }},
	'p': {"PM", "", func(f *LayoutFields) { f.Hour = true }},
	'P': {"pm", "", func(f *LayoutFields) { f.Hour = true }},
	'z': {"-0700", "Z07:00", func(f *LayoutFields) { f.Zone = true }},
	'Z': {"MST", "", func(f *LayoutFields) { f.Zone = true }},
	'A': {"Monday", "", func(f *LayoutFields) { f.Wday = true }},
	'a': {"Mon", "", func(f *LayoutFields) { f.Wday = true }},
	'n': {"\n", "", nil},
	't': {"\t", "", nil},
	'%': {"%", "", nil},
}

// unpadded are the tokens %-d and kin take: Go's one-digit forms.
var unpadded = map[byte]string{'d': "2", 'm': "1", 'e': "2", 'H': "15", 'I': "3", 'M': "4", 'S': "5", 'j': "__2", 'y': "06", 'Y': "2006"}

// compositeDirectives expand to other directives before translation.
var compositeDirectives = strings.NewReplacer("%T", "%H:%M:%S", "%D", "%m/%d/%y", "%F", "%Y-%m-%d", "%R", "%H:%M", "%r", "%I:%M:%S %p",
	"%c", "%a %b %e %H:%M:%S %Y", "%x", "%m/%d/%y", "%X", "%H:%M:%S", "%v", "%e-%b-%Y")

// ExpandComposites spells out the composite directives, %F, %T and kin.
func ExpandComposites(format string) string { return compositeDirectives.Replace(format) }

// StrptimeLayouts translates a strftime format into the Go layouts that
// read it: one, or two when the format has %z, which Go spells one way
// for +0900 and another for +09:00 and Z. A layoutDirective Go has no layout
// for is an error naming it; %s alone is the Epoch field.
func StrptimeLayouts(format string) ([]string, LayoutFields, error) {
	var fields LayoutFields
	if epochFormat.MatchString(format) {
		fields.Epoch = true
		return nil, fields, nil
	}
	format = compositeDirectives.Replace(format)
	layouts := []string{""}
	add := func(token string) {
		for i := range layouts {
			layouts[i] += token
		}
	}
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' {
			if strings.ContainsAny(string(c), "0123456789") || c == 'J' || c == 'M' || c == 'P' || c == 'Z' {
				// A digit or a letter Go's layout would read as a token is
				// escaped by doubling nothing: Go has no escape, so refuse.
				return nil, fields, fmt.Errorf("the format's %q is read as a layout token by Go's time", c)
			}
			add(string(c))
			continue
		}
		i++
		flag := byte(0)
		for i < len(format) && strings.IndexByte("-_0^#:", format[i]) >= 0 {
			if format[i] == '-' {
				flag = '-'
			}
			i++
		}
		for i < len(format) && format[i] >= '0' && format[i] <= '9' {
			i++
		}
		if i >= len(format) {
			return nil, fields, fmt.Errorf("the format ends in %%")
		}
		d, ok := directives[format[i]]
		if !ok {
			return nil, fields, fmt.Errorf("Go's time has no layout for %%%c", format[i])
		}
		token := d.token
		if flag == '-' {
			if u, ok := unpadded[format[i]]; ok {
				token = u
			}
		}
		if format[i] == 'L' || format[i] == 'N' {
			// A fraction follows the seconds' separator in Go's layout.
			for k := range layouts {
				if !strings.HasSuffix(layouts[k], ".") && !strings.HasSuffix(layouts[k], ",") {
					return nil, fields, fmt.Errorf("Go's time reads a fraction, %%%c, only after a . or ,", format[i])
				}
			}
		}
		if d.set != nil {
			d.set(&fields)
		}
		if d.alt != "" {
			more := make([]string, len(layouts))
			for k := range layouts {
				more[k] = layouts[k] + d.alt
			}
			add(token)
			layouts = append(layouts, more...)
		} else {
			add(token)
		}
	}
	return layouts, fields, nil
}

// epochFormat is the one form a %s takes: alone, or with a zone after it,
// since Go's layouts have no token for seconds since the epoch.
var epochFormat = regexp.MustCompile(`^%s(\s+%[zZ])?$`)

// ParseEpoch reads s by an epoch format, %s or %s %z: the seconds since
// the epoch, in the zone named or the local one.
func ParseEpoch(s, format string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	num, zone := s, ""
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		num, zone = s[:i], strings.TrimSpace(s[i:])
	}
	n, err := strconv.ParseInt(num, 10, 64)
	if err != nil || (zone != "") != (format != "%s") {
		return time.Time{}, false
	}
	loc := time.Local
	if zone != "" {
		var ok bool
		if loc, ok = ZoneNamed(zone); !ok {
			return time.Time{}, false
		}
	}
	return time.Unix(n, 0).In(loc), true
}

// TryLayouts reads s by the first layout that fits, in loc for a layout
// without a zone.
func TryLayouts(s string, layouts []string, loc *time.Location) (time.Time, bool) {
	if loc == nil {
		loc = time.Local
	}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, loc); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

var zoneLayouts = []string{"Z07:00", "-07:00", "-0700", "-07", "Z0700", "MST"}

// ZoneNamed reads a zone as a parsed time names one, with Go's zone
// layouts: an offset with or without a colon, Z, or a name such as UTC or
// GMT, which Go reads as an offset of zero under that name.
func ZoneNamed(s string) (*time.Location, bool) {
	if s == "" {
		return nil, false
	}
	if strings.EqualFold(s, "UTC") || s == "Z" {
		return time.UTC, true
	}
	for _, l := range zoneLayouts {
		t, err := time.Parse(l, s)
		if err != nil {
			continue
		}
		name, off := t.Zone()
		if l == "MST" && off == 0 && !strings.EqualFold(name, "GMT") && !strings.EqualFold(name, "UT") {
			// A name Go does not know reads as an offset of zero; Ruby
			// reads only the names it knows.
			continue
		}
		return time.FixedZone(name, off), true
	}
	return nil, false
}

var zoneTextRe = regexp.MustCompile(`(?i)(z|utc?|gmt|[+-]\d{2}(?::?\d{2})?|[a-z]{3,4})\s*$`)

// ZoneText is the zone as a string wrote it, at its end.
func ZoneText(s string) string {
	m := zoneTextRe.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return m[1]
}

// Candidate is a form a free-form string may take: its strftime format
// and the layouts and fields of it.
type Candidate struct {
	Format  string
	Layouts []string
	Fields  LayoutFields
	// Keys are the fragments the form sets, in the order Ruby lists them.
	Keys []string
}

// ParseCandidates are the forms Time.parse and Date.parse read, most
// specific first: ISO 8601 with and without a time and a zone, the RFC
// 2822 and HTTP dates, the dates with month names, slashed and dashed
// dates, times of day, and a weekday alone.
var ParseCandidates = func() []Candidate {
	iso := []string{"year", "mon", "mday"}
	dmy := []string{"mday", "mon", "year"}
	mdy := []string{"mon", "mday", "year"}
	specs := []struct {
		format string
		keys   []string
	}{
		{"%Y-%m-%dT%H:%M:%S.%N%z", iso}, {"%Y-%m-%dT%H:%M:%S%z", iso}, {"%Y-%m-%dT%H:%M:%S.%N", iso}, {"%Y-%m-%dT%H:%M:%S", iso},
		{"%Y-%m-%dT%H:%M%z", iso}, {"%Y-%m-%dT%H:%M", iso},
		{"%Y-%m-%d %H:%M:%S.%N %z", iso}, {"%Y-%m-%d %H:%M:%S %z", iso}, {"%Y-%m-%d %H:%M:%S.%N %Z", iso}, {"%Y-%m-%d %H:%M:%S %Z", iso},
		{"%Y-%m-%d %H:%M:%S.%N", iso}, {"%Y-%m-%d %H:%M:%S", iso}, {"%Y-%m-%d %H:%M %z", iso}, {"%Y-%m-%d %H:%M", iso}, {"%Y-%m-%d", iso},
		{"%Y%m%dT%H%M%S%z", iso}, {"%Y%m%dT%H%M%S", iso}, {"%Y%m%dT%H%M", iso}, {"%Y%m%d %H:%M:%S", iso}, {"%Y%m%d", iso}, {"%y%m%d", iso}, {"%Y-%j", []string{"year", "yday"}},
		{"%Y-%m-%d%z", iso}, {"%Y-%-m-%-d", iso},
		{"%Y/%m/%d %H:%M:%S", iso}, {"%Y/%-m/%-d %H:%M:%S", iso}, {"%Y/%m/%d %H:%M", iso}, {"%Y/%-m/%-d %H:%M", iso}, {"%Y/%m/%d", iso}, {"%Y/%-m/%-d", iso},
		{"%y-%m-%d", iso}, {"%y-%-m-%-d", iso},
		{"%a, %d %b %Y %H:%M:%S %Z", []string{"wday", "mday", "mon", "year", "hour", "min", "sec"}},
		{"%a, %d %b %Y %H:%M:%S %z", []string{"wday", "mday", "mon", "year", "hour", "min", "sec"}},
		{"%a, %-d %b %Y %H:%M:%S %z", []string{"wday", "mday", "mon", "year", "hour", "min", "sec"}},
		{"%a, %-d %b %Y %H:%M:%S %Z", []string{"wday", "mday", "mon", "year", "hour", "min", "sec"}},
		{"%a, %d %b %Y %H:%M:%S", []string{"wday", "mday", "mon", "year", "hour", "min", "sec"}},
		{"%a, %d %b %Y %H:%M %z", []string{"wday", "mday", "mon", "year", "hour", "min"}},
		{"%a, %d %b %Y", []string{"wday", "mday", "mon", "year"}},
		{"%A, %d-%b-%y %H:%M:%S %Z", []string{"wday", "mday", "mon", "year", "hour", "min", "sec"}},
		{"%a %b %e %H:%M:%S %Z %Y", []string{"wday", "mon", "mday", "hour", "min", "sec", "year"}},
		{"%a %b %e %H:%M:%S %z %Y", []string{"wday", "mon", "mday", "hour", "min", "sec", "year"}},
		{"%a %b %e %H:%M:%S %Y", []string{"wday", "mon", "mday", "hour", "min", "sec", "year"}},
		{"%a %b %e %Y", []string{"wday", "mon", "mday", "year"}},
		{"%d %b %Y %H:%M:%S %z", dmy}, {"%-d %b %Y %H:%M:%S %z", dmy}, {"%d %b %Y %H:%M:%S", dmy}, {"%-d %b %Y %H:%M:%S", dmy},
		{"%d %b %Y", dmy}, {"%-d %b %Y", dmy}, {"%d %B %Y", dmy}, {"%-d %B %Y", dmy},
		{"%b %d %Y %H:%M:%S %z", mdy}, {"%b %-d %Y %H:%M:%S %z", mdy}, {"%b %d %Y %H:%M:%S", mdy}, {"%b %-d %Y %H:%M:%S", mdy},
		{"%b %d %Y %H:%M %z", mdy}, {"%b %-d %Y %H:%M %z", mdy}, {"%b %d %Y %H:%M", mdy}, {"%b %-d %Y %H:%M", mdy},
		{"%d %b %Y %H:%M %z", dmy}, {"%-d %b %Y %H:%M %z", dmy}, {"%d %b %Y %H:%M", dmy}, {"%-d %b %Y %H:%M", dmy},
		{"%b %d %Y", mdy}, {"%b %-d %Y", mdy}, {"%b %d, %Y", mdy}, {"%b %-d, %Y", mdy},
		{"%B %d %Y", mdy}, {"%B %-d %Y", mdy}, {"%B %d, %Y", mdy}, {"%B %-d, %Y", mdy},
		{"%A, %B %d, %Y", []string{"wday", "mon", "mday", "year"}}, {"%A, %B %-d, %Y", []string{"wday", "mon", "mday", "year"}},
		{"%A, %d %B %Y", []string{"wday", "mday", "mon", "year"}}, {"%A, %-d %B %Y", []string{"wday", "mday", "mon", "year"}},
		{"%d/%m/%Y", dmy}, {"%-d/%-m/%Y", dmy}, {"%d.%m.%Y", dmy}, {"%-d.%-m.%Y", dmy},
		{"%B %Y", []string{"mon", "year"}}, {"%b %Y", []string{"mon", "year"}}, {"%Y %B", []string{"year", "mon"}}, {"%Y %b", []string{"year", "mon"}}, {"%b %d", []string{"mon", "mday"}}, {"%b %-d", []string{"mon", "mday"}},
		{"%H:%M:%S.%N %z", []string{"hour", "min", "sec"}}, {"%H:%M:%S %z", []string{"hour", "min", "sec"}}, {"%H:%M:%S %Z", []string{"hour", "min", "sec"}},
		{"%H:%M:%S.%N", []string{"hour", "min", "sec"}}, {"%H:%M:%S", []string{"hour", "min", "sec"}}, {"%H:%M %z", []string{"hour", "min"}}, {"%H:%M", []string{"hour", "min"}},
		{"%I:%M:%S %p", []string{"hour", "min", "sec"}}, {"%I:%M %p", []string{"hour", "min"}},
		{"%A", []string{"wday"}}, {"%a", []string{"wday"}},
	}
	out := make([]Candidate, 0, len(specs))
	for _, sp := range specs {
		layouts, fields, err := StrptimeLayouts(sp.format)
		if err != nil {
			panic("stdlib: parse candidate " + sp.format + ": " + err.Error())
		}
		out = append(out, Candidate{Format: sp.format, Layouts: layouts, Fields: fields, Keys: sp.keys})
	}
	return out
}()

// ParseFreeForm reads s by the first candidate that fits, the string
// trimmed of its surrounding whitespace.
func ParseFreeForm(s string, loc *time.Location) (time.Time, *Candidate, bool) {
	s = strings.TrimSpace(s)
	for i := range ParseCandidates {
		c := &ParseCandidates[i]
		if t, ok := TryLayouts(s, c.Layouts, loc); ok {
			return t, c, true
		}
	}
	return time.Time{}, nil, false
}

// WeekdayNamed is the weekday a string names, by its full or its
// three-letter name, in any case.
func WeekdayNamed(s string) (time.Weekday, bool) {
	lower := strings.ToLower(s)
	for d := time.Sunday; d <= time.Saturday; d++ {
		name := strings.ToLower(d.String())
		if strings.Contains(lower, name) || strings.Contains(lower, name[:3]) {
			return d, true
		}
	}
	return 0, false
}

// ClockFrom fills a Clock with the fields a layout read from t.
func ClockFrom(t time.Time, f LayoutFields) Clock {
	c := NewClock()
	if f.Year {
		c.Year = t.Year()
	}
	if f.Mon || f.Yday {
		c.Mon = int(t.Month())
	}
	if f.Day || f.Yday {
		c.Day = t.Day()
	}
	if f.Hour {
		c.Hour = t.Hour()
	}
	if f.Min {
		c.Min = t.Minute()
	}
	if f.Sec {
		c.Sec = t.Second()
	}
	if f.Frac {
		c.Nsec = t.Nanosecond()
	}
	if f.Zone {
		c.Loc = t.Location()
	}
	return c
}
