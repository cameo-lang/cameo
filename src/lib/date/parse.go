package date

import (
	"regexp"
	"strconv"
	"strings"
)

// Date._parse, after date_parse.c: a text is scanned for a weekday, a
// time, then a date in each of the forms Ruby knows, in Ruby's order, and
// each match is blanked out of the text before the next form is tried.
// What is found goes into a hash of fragments, which the fixed-format
// parsers (formats.go) and strptime fill the same way, and which
// completeFrags turns into a date.

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

const (
	abbrDays   = `sun|mon|tue|wed|thu|fri|sat`
	abbrMonths = `jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec`
	// bcAD are the era words a European or American date may carry.
	bcAD = `(c(?:e|\.e\.)|b(?:ce|\.c\.e\.)|a(?:d|\.d\.)|b(?:c|\.c\.))`
)

var (
	dayRe  = regexp.MustCompile(`(?i)\b(` + abbrDays + `)[^-/\d\s]*`)
	timeRe = regexp.MustCompile(`(?i)(` +
		`(?:\d+\s*:\s*\d+(?:\s*:\s*\d+(?:[,.]\d*)?)?|\d+\s*h(?:\s*\d+m?(?:\s*\d+s?)?)?)(?:\s*[ap](?:m\b|\.m\.))?` +
		`|\d+\s*[ap](?:m\b|\.m\.)` +
		`)(?:\s*(` +
		`(?:gmt|utc?)?[-+]\d+(?:[,.:]\d+(?::\d+)?)?` +
		`|[a-zA-Z.\s]+(?:standard|daylight)\stime\b` +
		`|[a-zA-Z]+(?:\sdst)?\b` +
		`))?`)
	timePartRe = regexp.MustCompile(`(?i)\A(\d+)h?(?:\s*:?\s*(\d+)m?(?:\s*:?\s*(\d+)(?:[,.](\d+))?s?)?)?(?:\s*([ap])(?:m\b|\.m\.))?`)
	euRe       = regexp.MustCompile(`(?i)('?\d+)[^-\d\s]*\s*(` + abbrMonths + `)[^-\d\s']*(?:\s*(?:` + bcAD + `?\s*('?-?\d+(?:(?:st|nd|rd|th)\b)?)))?`)
	usRe       = regexp.MustCompile(`(?i)\b(` + abbrMonths + `)[^-\d\s']*\s*('?\d+)[^-\d\s']*(?:\s*,?\s*` + bcAD + `?\s*('?-?\d+))?`)
	isoRe      = regexp.MustCompile(`('?[-+]?\d+)-(\d+)-('?-?\d+)`)
	jisRe      = regexp.MustCompile(`(?i)\b([mtshr])(\d+)\.(\d+)\.(\d+)`)
	vms11Re    = regexp.MustCompile(`(?i)('?-?\d+)-(` + abbrMonths + `)[^-/.]*-('?-?\d+)`)
	vms12Re    = regexp.MustCompile(`(?i)\b(` + abbrMonths + `)[^-/.]*-('?-?\d+)(?:-('?-?\d+))?`)
	slaRe      = regexp.MustCompile(`('?-?\d+)/\s*('?\d+)(?:\D\s*('?-?\d+))?`)
	dotRe      = regexp.MustCompile(`('?-?\d+)\.\s*('?\d+)\.\s*('?-?\d+)`)
	iso21Re    = regexp.MustCompile(`(?i)\b(\d{2}|\d{4})?-?w(\d{2})(?:-?(\d))?\b`)
	iso22Re    = regexp.MustCompile(`(?i)-w-(\d)\b`)
	iso23Re    = regexp.MustCompile(`--(\d{2})?-(\d{2})\b`)
	iso24Re    = regexp.MustCompile(`--(\d{2})(\d{2})?\b`)
	iso25GdRe  = regexp.MustCompile(`[,.](\d{2}|\d{4})-\d{3}\b`)
	iso25Re    = regexp.MustCompile(`\b(\d{2}|\d{4})-(\d{3})\b`)
	iso26GdRe  = regexp.MustCompile(`\d-\d{3}\b`)
	iso26Re    = regexp.MustCompile(`\b-(\d{3})\b`)
	yearRe     = regexp.MustCompile(`'(\d+)\b`)
	monRe      = regexp.MustCompile(`(?i)\b(` + abbrMonths + `)\S*`)
	mdayRe     = regexp.MustCompile(`(?i)(\d+)(st|nd|rd|th)\b`)
	dddRe      = regexp.MustCompile(`(?i)([-+]?)(\d{2,14})(?:\s*t?\s*(\d{2,6})?(?:[,.](\d*))?)?(?:\s*(z\b|[-+]\d{1,4}\b|\[[-+]?\d[^\]]*\]))?`)
	bcRe       = regexp.MustCompile(`(?i)\b(bc\b|bce\b|b\.c\.|b\.c\.e\.)`)
	fragRe     = regexp.MustCompile(`\A\s*(\d{1,2})\s*\z`)
	nonTokenRe = regexp.MustCompile(`[^-+',./:@[\]\w]+`)
)

// subx replaces the first match of re in *s with a space and returns the
// submatches, or nil.
func subx(s *string, re *regexp.Regexp) []string {
	m := re.FindStringSubmatchIndex(*s)
	if m == nil {
		return nil
	}
	groups := make([]string, len(m)/2)
	for i := range groups {
		if m[2*i] >= 0 {
			groups[i] = (*s)[m[2*i]:m[2*i+1]]
		} else {
			groups[i] = "\x00" // absent
		}
	}
	*s = (*s)[:m[0]] + " " + (*s)[m[1]:]
	return groups
}

func absent(s string) bool { return s == "\x00" }

func atoi(s string) int64 {
	s = strings.TrimLeft(s, "'")
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// digitsOnly is the digits of a token with its sign, without the
// apostrophe and the ordinal suffix.
func digitsOnly(s string) string {
	s = strings.TrimLeft(s, "'")
	end := 0
	if end < len(s) && (s[end] == '-' || s[end] == '+') {
		end++
	}
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	return s[:end]
}

func monthIndex(name string) int64 {
	name = strings.ToLower(name[:3])
	for i, m := range abbrMonthNames {
		if strings.ToLower(m) == name {
			return int64(i + 1)
		}
	}
	return 0
}

func dayIndex(name string) int64 {
	name = strings.ToLower(name[:3])
	for i, d := range abbrDayNames {
		if strings.ToLower(d) == name {
			return int64(i)
		}
	}
	return 0
}

// parseDay reads a weekday name.
func parseDay(s *string, f *frags) bool {
	m := subx(s, dayRe)
	if m == nil {
		return false
	}
	f.set("wday", dayIndex(m[1]))
	return true
}

// parseTime reads a time of day with an optional meridian and zone.
func parseTime(s *string, f *frags) bool {
	m := subx(s, timeRe)
	if m == nil {
		return false
	}
	if !absent(m[2]) {
		f.set("zone", m[2])
	}
	t := timePartRe.FindStringSubmatch(m[1])
	if t == nil {
		return true
	}
	h := atoi(t[1])
	if t[2] != "" {
		f.set("min", atoi(t[2]))
	}
	if t[3] != "" {
		f.set("sec", atoi(t[3]))
	}
	if t[4] != "" {
		f.set("sec_fraction", fractionOf(t[4]))
	}
	if t[5] != "" {
		h %= 12
		if strings.EqualFold(t[5], "p") {
			h += 12
		}
	}
	// Ruby sets the hour before the minute; the order shows in the hash.
	f.setBefore("hour", h, "min")
	return true
}

// setBefore sets k, placing it before key other when other is present
// already, as Ruby's hash would have had k first.
func (f *frags) setBefore(k string, v any, other string) {
	if f.has(k) || !f.has(other) {
		f.set(k, v)
		return
	}
	f.values[k] = v
	for i, x := range f.keys {
		if x == other {
			f.keys = append(f.keys[:i], append([]string{k}, f.keys[i:]...)...)
			return
		}
	}
	f.keys = append(f.keys, k)
}

// fractionOf is the fraction a run of digits after a point names.
func fractionOf(digits string) float64 {
	n, _ := strconv.ParseFloat("0."+digits, 64)
	return n
}

// s3e is Ruby's s3e: three tokens that may be a year, a month and a day
// in some order, sorted out by their shape. An apostrophe or more than two
// digits marks the year; a token with a suffix such as 29th is a day.
func s3e(f *frags, y, m, d string, bc bool) {
	nil3 := func(s string) bool { return absent(s) || s == "" }
	if !nil3(y) && !nil3(m) && nil3(d) {
		// Two tokens: a month and a day.
		y, d = "\x00", y
	}
	if nil3(y) {
		if !nil3(d) && (len(digitsOnly(strings.TrimLeft(d, "'"))) > 2 || strings.HasPrefix(d, "'")) {
			y, d = d, "\x00"
		}
	}
	if !nil3(y) {
		// A year token with an ordinal suffix is a day.
		if rest := strings.TrimPrefix(strings.TrimLeft(y, "'"), digitsOnly(strings.TrimLeft(y, "'"))); rest != "" {
			y, d = d, y
		}
	}
	if !nil3(m) && (strings.HasPrefix(m, "'") || len(m) > 2) {
		// A month of more than two digits is the year: us -> be.
		y, m, d = m, d, y
	}
	if !nil3(d) && (strings.HasPrefix(d, "'") || len(digitsOnly(strings.TrimLeft(d, "'"))) > 2) {
		y, d = d, y
	}
	if !nil3(y) {
		digits := digitsOnly(strings.TrimLeft(y, "'"))
		if strings.HasPrefix(y, "'") || len(strings.TrimLeft(digits, "-+")) > 2 {
			f.comp = false
		}
		n := atoi(digits)
		if bc {
			n = -n + 1
		}
		f.set("year", n)
	}
	if !nil3(m) {
		f.set("mon", atoi(digitsOnly(strings.TrimLeft(m, "'"))))
	}
	if !nil3(d) {
		f.set("mday", atoi(digitsOnly(strings.TrimLeft(d, "'"))))
	}
}

func isBC(s string) bool {
	s = strings.ToLower(s)
	return strings.HasPrefix(s, "b")
}

func parseEU(s *string, f *frags) bool {
	m := subx(s, euRe)
	if m == nil {
		return false
	}
	mon := strconv.FormatInt(monthIndex(m[2]), 10)
	s3e(f, m[4], mon, m[1], !absent(m[3]) && isBC(m[3]))
	return true
}

func parseUS(s *string, f *frags) bool {
	m := subx(s, usRe)
	if m == nil {
		return false
	}
	mon := strconv.FormatInt(monthIndex(m[1]), 10)
	s3e(f, m[4], mon, m[2], !absent(m[3]) && isBC(m[3]))
	return true
}

func parseISO(s *string, f *frags) bool {
	m := subx(s, isoRe)
	if m == nil {
		return false
	}
	s3e(f, m[1], m[2], m[3], false)
	return true
}

func parseJIS(s *string, f *frags) bool {
	m := subx(s, jisRe)
	if m == nil {
		return false
	}
	era := map[byte]int64{'m': 1867, 't': 1911, 's': 1925, 'h': 1988, 'r': 2018}[strings.ToLower(m[1])[0]]
	f.set("year", atoi(m[2])+era)
	f.set("mon", atoi(m[3]))
	f.set("mday", atoi(m[4]))
	return true
}

func parseVMS(s *string, f *frags) bool {
	if m := subx(s, vms11Re); m != nil {
		s3e(f, m[3], strconv.FormatInt(monthIndex(m[2]), 10), m[1], false)
		return true
	}
	if m := subx(s, vms12Re); m != nil {
		s3e(f, m[3], strconv.FormatInt(monthIndex(m[1]), 10), m[2], false)
		return true
	}
	return false
}

func parseSla(s *string, f *frags) bool {
	m := subx(s, slaRe)
	if m == nil {
		return false
	}
	s3e(f, m[1], m[2], m[3], false)
	return true
}

func parseDot(s *string, f *frags) bool {
	m := subx(s, dotRe)
	if m == nil {
		return false
	}
	s3e(f, m[1], m[2], m[3], false)
	return true
}

func parseISO2(s *string, f *frags) bool {
	if m := subx(s, iso21Re); m != nil {
		if !absent(m[1]) {
			f.set("cwyear", atoi(m[1]))
		}
		f.set("cweek", atoi(m[2]))
		if !absent(m[3]) {
			f.set("cwday", atoi(m[3]))
		}
		return true
	}
	if m := subx(s, iso22Re); m != nil {
		f.set("cwday", atoi(m[1]))
		return true
	}
	if m := subx(s, iso23Re); m != nil {
		if !absent(m[1]) {
			f.set("mon", atoi(m[1]))
		}
		f.set("mday", atoi(m[2]))
		return true
	}
	if m := subx(s, iso24Re); m != nil {
		f.set("mon", atoi(m[1]))
		if !absent(m[2]) {
			f.set("mday", atoi(m[2]))
		}
		return true
	}
	if !iso25GdRe.MatchString(*s) {
		if m := subx(s, iso25Re); m != nil {
			f.set("year", atoi(m[1]))
			f.set("yday", atoi(m[2]))
			return true
		}
	}
	if !iso26GdRe.MatchString(*s) {
		if m := subx(s, iso26Re); m != nil {
			f.set("yday", atoi(m[1]))
			return true
		}
	}
	return false
}

func parseYear(s *string, f *frags) bool {
	m := subx(s, yearRe)
	if m == nil {
		return false
	}
	f.set("year", atoi(m[1]))
	return true
}

func parseMon(s *string, f *frags) bool {
	m := subx(s, monRe)
	if m == nil {
		return false
	}
	f.set("mon", monthIndex(m[1]))
	return true
}

func parseMday(s *string, f *frags) bool {
	m := subx(s, mdayRe)
	if m == nil {
		return false
	}
	f.set("mday", atoi(m[1]))
	return true
}

// parseDDD reads a run of digits as a date and time by its length:
// yyyymmdd, yymmdd, mmdd, dd, and yyyyddd, yyddd, ddd; then hhmmss.
func parseDDD(s *string, f *frags) bool {
	m := subx(s, dddRe)
	if m == nil {
		return false
	}
	sign, digits := m[1], m[2]
	num := func(a, b int) int64 { return atoi(digits[a:b]) }
	switch len(digits) {
	case 2:
		if absent(m[3]) && !absent(m[4]) {
			f.set("sec", num(0, 2))
		} else {
			f.set("mday", num(0, 2))
		}
	case 4:
		if absent(m[3]) && !absent(m[4]) {
			f.set("sec", num(2, 4))
			f.set("min", num(0, 2))
		} else {
			f.set("mon", num(0, 2))
			f.set("mday", num(2, 4))
		}
	case 6:
		if absent(m[3]) && !absent(m[4]) {
			f.set("sec", num(4, 6))
			f.set("min", num(2, 4))
			f.set("hour", num(0, 2))
		} else {
			f.set("year", atoi(sign+digits[0:2]))
			f.set("mon", num(2, 4))
			f.set("mday", num(4, 6))
		}
	case 8, 10, 12, 14:
		if absent(m[3]) && !absent(m[4]) {
			f.set("sec", num(len(digits)-2, len(digits)))
			f.set("min", num(len(digits)-4, len(digits)-2))
			f.set("hour", num(len(digits)-6, len(digits)-4))
			f.set("mday", num(len(digits)-8, len(digits)-6))
			if len(digits) >= 10 {
				f.set("mon", num(len(digits)-10, len(digits)-8))
			}
			if len(digits) == 12 {
				f.set("year", atoi(sign+digits[0:2]))
			}
			if len(digits) == 14 {
				f.set("year", atoi(sign+digits[0:4]))
				f.comp = false
			}
		} else {
			f.set("year", atoi(sign+digits[0:4]))
			f.set("mon", num(4, 6))
			f.set("mday", num(6, 8))
			if len(digits) >= 10 {
				f.set("hour", num(8, 10))
			}
			if len(digits) >= 12 {
				f.set("min", num(10, 12))
			}
			if len(digits) >= 14 {
				f.set("sec", num(12, 14))
			}
			f.comp = false
		}
	case 3:
		if absent(m[3]) && !absent(m[4]) {
			f.set("sec", num(1, 3))
			f.set("min", num(0, 1))
		} else {
			f.set("yday", num(0, 3))
		}
	case 5:
		if absent(m[3]) && !absent(m[4]) {
			f.set("sec", num(3, 5))
			f.set("min", num(1, 3))
			f.set("hour", num(0, 1))
		} else {
			f.set("year", atoi(sign+digits[0:2]))
			f.set("yday", num(2, 5))
		}
	case 7:
		if absent(m[3]) && !absent(m[4]) {
			f.set("sec", num(5, 7))
			f.set("min", num(3, 5))
			f.set("hour", num(1, 3))
			f.set("mday", num(0, 1))
		} else {
			f.set("year", atoi(sign+digits[0:4]))
			f.set("yday", num(4, 7))
			f.comp = false
		}
	}
	if !absent(m[3]) {
		t := m[3]
		switch len(t) {
		case 2, 4, 6:
			f.set("hour", atoi(t[0:2]))
			if len(t) >= 4 {
				f.set("min", atoi(t[2:4]))
			}
			if len(t) >= 6 {
				f.set("sec", atoi(t[4:6]))
			}
		}
	}
	if !absent(m[4]) && m[4] != "" {
		f.set("sec_fraction", fractionOf(m[4]))
	}
	if !absent(m[5]) {
		z := m[5]
		if strings.HasPrefix(z, "[") {
			z = strings.Trim(z, "[]")
			zone, name, found := strings.Cut(z, ":")
			if found {
				z = name
				if off, ok := zoneToDiff(zone); ok {
					f.set("zone", name)
					f.set("offset", off)
					return true
				}
			}
			if z != "" && (z[0] == '-' || z[0] == '+' || z[0] >= '0' && z[0] <= '9') && !strings.HasPrefix(z, "-") && !strings.HasPrefix(z, "+") {
				z = "+" + z
			}
		}
		f.set("zone", z)
	}
	return true
}

func parseBC(s *string, f *frags) bool {
	m := subx(s, bcRe)
	if m == nil {
		return false
	}
	f.bc = true
	return true
}

// parseFrag reads a lone one- or two-digit number left over: a day when a
// time was found, an hour when a day was.
func parseFrag(s *string, f *frags) bool {
	m := fragRe.FindStringSubmatch(*s)
	if m == nil {
		return false
	}
	n := atoi(m[1])
	if f.has("hour") && !f.has("mday") && n >= 1 && n <= 31 {
		f.set("mday", n)
		return true
	}
	if f.has("mday") && !f.has("hour") && n >= 0 && n <= 24 {
		f.set("hour", n)
		return true
	}
	return false
}

// parse is Date._parse(str, comp): the fragments of the date in str.
func parse(str string, comp bool) *frags {
	f := newFrags()
	f.comp = comp
	s := nonTokenRe.ReplaceAllString(str, " ")
	hasAlpha := strings.ContainsFunc(s, func(r rune) bool { return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' })
	hasDigit := strings.ContainsFunc(s, func(r rune) bool { return r >= '0' && r <= '9' })
	if hasAlpha {
		parseDay(&s, f)
	}
	if hasDigit {
		parseTime(&s, f)
	}
	found := false
	if hasAlpha && hasDigit {
		found = parseEU(&s, f) || parseUS(&s, f)
	}
	if !found && hasDigit && strings.Contains(s, "-") {
		found = parseISO(&s, f)
	}
	if !found && hasDigit && strings.Contains(s, ".") {
		found = parseJIS(&s, f)
	}
	if !found && hasAlpha && hasDigit && strings.Contains(s, "-") {
		found = parseVMS(&s, f)
	}
	if !found && hasDigit && strings.Contains(s, "/") {
		found = parseSla(&s, f)
	}
	if !found && hasDigit && strings.Contains(s, ".") {
		found = parseDot(&s, f)
	}
	if !found && hasDigit {
		found = parseISO2(&s, f)
	}
	if !found && hasDigit {
		found = parseYear(&s, f)
	}
	if !found && hasAlpha {
		found = parseMon(&s, f)
	}
	if !found && hasDigit {
		found = parseMday(&s, f)
	}
	if !found && hasDigit {
		parseDDD(&s, f)
	}
	if hasAlpha {
		parseBC(&s, f)
	}
	if hasDigit {
		parseFrag(&s, f)
	}
	if f.bc {
		for _, k := range []string{"year", "cwyear"} {
			if y, ok := f.int(k); ok {
				f.values[k] = -y + 1
			}
		}
	}
	if f.comp {
		for _, k := range []string{"year", "cwyear"} {
			if y, ok := f.int(k); ok && y >= 0 && y <= 99 {
				if y >= 69 {
					f.values[k] = y + 1900
				} else {
					f.values[k] = y + 2000
				}
			}
		}
	}
	if z, ok := f.values["zone"].(string); ok && !f.has("offset") {
		if off, ok := zoneToDiff(z); ok {
			f.set("offset", off)
		}
	}
	return f
}
