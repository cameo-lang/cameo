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

// layoutDirective is one strftime layoutDirective as a Go layout token: alts are
// the other spellings Go needs for the same directive, and set what it sets.
type layoutDirective struct {
	token string
	alts  []string
	set   func(*LayoutFields)
}

var directives = map[byte]layoutDirective{
	'Y': {"2006", nil, func(f *LayoutFields) { f.Year = true }},
	'y': {"06", nil, func(f *LayoutFields) { f.Year = true }},
	'm': {"01", nil, func(f *LayoutFields) { f.Mon = true }},
	'B': {"January", nil, func(f *LayoutFields) { f.Mon = true }},
	'b': {"Jan", nil, func(f *LayoutFields) { f.Mon = true }},
	'h': {"Jan", nil, func(f *LayoutFields) { f.Mon = true }},
	'd': {"02", nil, func(f *LayoutFields) { f.Day = true }},
	'e': {"_2", nil, func(f *LayoutFields) { f.Day = true }},
	'j': {"002", nil, func(f *LayoutFields) { f.Yday = true }},
	'H': {"15", nil, func(f *LayoutFields) { f.Hour = true }},
	'k': {"15", nil, func(f *LayoutFields) { f.Hour = true }},
	'I': {"03", nil, func(f *LayoutFields) { f.Hour = true }},
	'l': {"3", nil, func(f *LayoutFields) { f.Hour = true }},
	'M': {"04", nil, func(f *LayoutFields) { f.Min = true }},
	'S': {"05", nil, func(f *LayoutFields) { f.Sec = true }},
	'L': {"999", nil, func(f *LayoutFields) { f.Frac = true }},
	'N': {"999999999", nil, func(f *LayoutFields) { f.Frac = true }},
	'p': {"PM", nil, func(f *LayoutFields) { f.Hour = true }},
	'P': {"pm", nil, func(f *LayoutFields) { f.Hour = true }},
	'z': {"-0700", []string{"Z07:00", "MST"}, func(f *LayoutFields) { f.Zone = true }},
	'Z': {"MST", nil, func(f *LayoutFields) { f.Zone = true }},
	'A': {"Monday", nil, func(f *LayoutFields) { f.Wday = true }},
	'a': {"Mon", nil, func(f *LayoutFields) { f.Wday = true }},
	'n': {"\n", nil, nil},
	't': {"\t", nil, nil},
	'%': {"%", nil, nil},
}

// unpadded are the tokens %-d and kin take: Go's one-digit forms.
var unpadded = map[byte]string{'d': "2", 'm': "1", 'e': "2", 'H': "15", 'I': "3", 'M': "4", 'S': "5", 'j': "__2", 'y': "06", 'Y': "2006"}

// compositeDirectives expand to other directives before translation.
var compositeDirectives = strings.NewReplacer("%T", "%H:%M:%S", "%D", "%m/%d/%y", "%F", "%Y-%m-%d", "%R", "%H:%M", "%r", "%I:%M:%S %p",
	"%c", "%a %b %e %H:%M:%S %Y", "%x", "%m/%d/%y", "%X", "%H:%M:%S", "%v", "%e-%b-%Y")

// ExpandComposites spells out the composite directives, %F, %T and kin.
func ExpandComposites(format string) string { return compositeDirectives.Replace(format) }

// StrptimeLayouts translates a strftime format into the Go layouts that
// read it: one, or three for each %z, which Go spells one way for +0900,
// another for +09:00 and Z, and a third for a zone name such as UTC. A
// directive Go has no layout for is an error naming it.
//
// Go's layouts have no token for seconds since the epoch, so a format that
// starts with %s sets the Epoch field, and the layouts read what follows
// the seconds (ParseEpoch). That rest may set a fraction and a zone only.
func StrptimeLayouts(format string) ([]string, LayoutFields, error) {
	if rest, ok := strings.CutPrefix(format, "%s"); ok {
		layouts, fields, err := strptimeLayouts(rest)
		if err != nil {
			return nil, fields, err
		}
		if fields != (LayoutFields{Frac: fields.Frac, Zone: fields.Zone}) {
			return nil, fields, fmt.Errorf("Go's time reads %%s only with a fraction and a zone after it")
		}
		fields.Epoch = true
		return layouts, fields, nil
	}
	return strptimeLayouts(format)
}

func strptimeLayouts(format string) ([]string, LayoutFields, error) {
	var fields LayoutFields
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
		base := layouts
		layouts = nil
		for _, tok := range append([]string{token}, d.alts...) {
			for _, l := range base {
				layouts = append(layouts, l+tok)
			}
		}
	}
	return layouts, fields, nil
}

// epochSeconds is what %s reads: an integer with an optional sign.
var epochSeconds = regexp.MustCompile(`^\s*[-+]?\d+`)

// ParseEpoch reads s by the layouts and fields StrptimeLayouts returns for
// a format that starts with %s: the seconds since the epoch, then the rest
// of s by the layouts. A fraction is added to the seconds away from zero,
// as Ruby adds it, and the time is in the zone read, or in loc without one.
func ParseEpoch(s string, layouts []string, fields LayoutFields, loc *time.Location) (time.Time, bool) {
	num := epochSeconds.FindString(s)
	if num == "" {
		return time.Time{}, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(num), 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	if loc == nil {
		loc = time.Local
	}
	rest, ok := TryLayouts(s[len(num):], layouts, loc)
	if !ok {
		return time.Time{}, false
	}
	nsec := int64(0)
	if fields.Frac {
		nsec = int64(rest.Nanosecond())
		if n < 0 {
			nsec = -nsec
		}
	}
	if fields.Zone {
		loc = rest.Location()
	}
	return time.Unix(n, nsec).In(loc), true
}

// TryLayouts reads s by the first layout that fits, in loc for a layout
// without a zone.
func TryLayouts(s string, layouts []string, loc *time.Location) (time.Time, bool) {
	if loc == nil {
		loc = time.Local
	}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, loc); err == nil {
			return fixZone(t, loc), true
		}
	}
	return time.Time{}, false
}

var zoneLayouts = []string{"Z07:00", "-07:00", "-0700", "-07", "Z0700"}

// zoneAbbrevs are the zone names Ruby's Time knows (Time.zone_offset):
// the names of UTC, the North American zones, and the military letters.
// Any other name is the local zone, as Ruby reads it.
var zoneAbbrevs = func() map[string]int {
	m := map[string]int{"UTC": 0, "Z": 0, "UT": 0, "GMT": 0,
		"EST": -5 * 3600, "EDT": -4 * 3600, "CST": -6 * 3600, "CDT": -5 * 3600,
		"MST": -7 * 3600, "MDT": -6 * 3600, "PST": -8 * 3600, "PDT": -7 * 3600}
	for i, c := 1, byte('A'); c <= 'M'; c++ {
		if c == 'J' {
			continue
		}
		m[string(c)] = i * 3600
		i++
	}
	for i, c := 1, byte('N'); c <= 'Y'; c++ {
		m[string(c)] = -i * 3600
		i++
	}
	return m
}()

// ZoneOffset is the offset of a zone name Ruby's Time knows, in seconds.
func ZoneOffset(name string) (int, bool) {
	off, ok := zoneAbbrevs[strings.ToUpper(name)]
	return off, ok
}

// zoneAt is the location of a named zone's offset, as Ruby's Time.new
// takes an offset: a fixed offset, named UTC when it is zero, so that
// Time#zone says "UTC" for a parsed GMT and nothing for a parsed EST, and
// inspect prints the offset for both.
func zoneAt(off int) *time.Location {
	if off == 0 {
		return time.FixedZone("UTC", 0)
	}
	return time.FixedZone("", off)
}

// ZoneNamed reads a zone as a parsed time names one: an offset with or
// without a colon, Z, or a name Ruby's Time knows (UTC, GMT, EST, PST and
// kin). A name it does not know is no zone.
func ZoneNamed(s string) (*time.Location, bool) {
	if s == "" {
		return nil, false
	}
	if strings.EqualFold(s, "UTC") || s == "Z" {
		return time.UTC, true
	}
	if off, ok := ZoneOffset(s); ok {
		return zoneAt(off), true
	}
	for _, l := range zoneLayouts {
		t, err := time.Parse(l, s)
		if err != nil {
			continue
		}
		_, off := t.Zone()
		return zoneAt(off), true
	}
	return nil, false
}

// fixZone puts a time Go read under a zone name into the zone that name
// means: Go takes any name it does not know as an offset of zero under
// that name, where Ruby takes the names it knows at their offsets and any
// other as the local zone, loc.
func fixZone(t time.Time, loc *time.Location) time.Time {
	name, _ := t.Zone()
	if name == "" || t.Location() == loc || t.Location() == time.UTC {
		return t
	}
	in := loc
	if off, ok := ZoneOffset(name); ok {
		in = zoneAt(off)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), in)
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
		{"%I:%M:%S %P", []string{"hour", "min", "sec"}}, {"%I:%M %P", []string{"hour", "min"}},
		{"%A", []string{"wday"}}, {"%a", []string{"wday"}},
	}
	// A date with a 12-hour clock after it, "Jan 5 2024 3:04 PM", in the
	// date forms above with hours and either case of the meridian.
	var clocked []struct {
		format string
		keys   []string
	}
	for _, sp := range specs {
		if strings.Contains(sp.format, "%H") || strings.Contains(sp.format, "%I") || strings.Contains(sp.format, "%z") || strings.Contains(sp.format, "%Z") || strings.Contains(sp.format, "%a") || strings.Contains(sp.format, "%A") || !strings.Contains(sp.format, "%Y") || !strings.ContainsAny(sp.format, "bBm") {
			continue
		}
		keys := func(more ...string) []string { return append(append([]string{}, sp.keys...), more...) }
		for _, clock := range []struct {
			f    string
			keys []string
		}{{" %I:%M:%S %p", keys("hour", "min", "sec")}, {" %-I:%M:%S %p", keys("hour", "min", "sec")}, {" %I:%M %p", keys("hour", "min")}, {" %-I:%M %p", keys("hour", "min")},
			{" %I:%M:%S %P", keys("hour", "min", "sec")}, {" %-I:%M:%S %P", keys("hour", "min", "sec")}, {" %I:%M %P", keys("hour", "min")}, {" %-I:%M %P", keys("hour", "min")}} {
			clocked = append(clocked, struct {
				format string
				keys   []string
			}{sp.format + clock.f, clock.keys})
		}
	}
	// The clocked forms come before the bare dates, which are prefixes of them.
	specs = append(clocked, specs...)
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

// shortZoneRe is a zone name at the end of a string that Go's layouts
// cannot read, since they take three letters or more: UT, and the military
// letters.
var shortZoneRe = regexp.MustCompile(`\s(UT|[A-IK-Z])$`)

// spellShortZone writes such a name as the offset it means, which the
// layouts with %z read.
func spellShortZone(s string) string {
	m := shortZoneRe.FindStringSubmatchIndex(s)
	if m == nil {
		return s
	}
	off, _ := ZoneOffset(s[m[2]:m[3]])
	sign := "+"
	if off < 0 {
		sign, off = "-", -off
	}
	return s[:m[2]] + fmt.Sprintf("%s%02d%02d", sign, off/3600, off%3600/60)
}

// ParseFreeForm reads s by the first candidate that fits, the string
// trimmed of its surrounding whitespace.
func ParseFreeForm(s string, loc *time.Location) (time.Time, *Candidate, bool) {
	s = spellShortZone(strings.TrimSpace(s))
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
