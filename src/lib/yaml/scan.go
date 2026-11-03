package yaml

import (
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// This file is Psych::ScalarScanner: what a plain scalar means. Ruby's
// regular expressions are kept as they are, with Ruby's line anchors (^ and
// $ match at every line) where they matter, and its Unicode classes
// spelled out, since Go's [[:alpha:]] is ASCII alone.

// kind is what a plain scalar resolves to.
type kind int

const (
	kindString kind = iota
	kindNil
	kindBool
	kindInt
	kindFloat
	kindSymbol
	kindDate // a Date; valid says whether Ruby's Date accepts it
	kindTime // a Time; valid says whether Ruby's Time accepts it
)

// scalar is a resolved plain scalar.
type scalar struct {
	kind  kind
	str   string // the symbol's name, or the string itself
	b     bool
	i     *big.Int
	f     float64
	y     int64 // the date's or time's parts
	m, d  int64
	h     int64
	min   int64
	sec   int64
	nsec  int64
	zone  zone
	valid bool
}

// zone is a timestamp's zone: none (a local time of the instant read as
// UTC, as Psych's), Z, or an offset in seconds.
type zone struct {
	kind   int // 0 none, 1 Z, 2 offset
	offset int
}

const (
	alpha = `\pL\pM\p{Nl}`    // Onigmo's [[:alpha:]]
	word  = `\pL\pM\pN\p{Pc}` // Onigmo's [[:word:]]
	time1 = `(?:[Tt]|\s+)\d{1,2}:\d\d:\d\d(?:\.\d*)?(?:\s*(?:Z|[-+]\d{1,2}:?(?:\d\d)?))?`
)

var (
	// stringish is the check for a String, "being careful not to get
	// caught by hash keys, hex values, and special floats".
	stringish  = regexp.MustCompile(`(?m)^[^\d.:-]?[` + alpha + `_\s!@#$%\^&*(){}<>|/\\~;=]+`)
	notYtonf   = regexp.MustCompile(`(?mi)^[^ytonf~]`)
	nullWord   = regexp.MustCompile(`(?mi)^null$`)
	yesWord    = regexp.MustCompile(`(?mi)^(yes|true|on)$`)
	noWord     = regexp.MustCompile(`(?mi)^(no|false|off)$`)
	timeRe     = regexp.MustCompile(`^-?\d{4}-\d{1,2}-\d{1,2}` + time1 + `$`)
	dateRe     = regexp.MustCompile(`^\d{4}-(?:1[012]|0\d|\d)-(?:[12]\d|3[01]|0\d|\d)$`)
	posInf     = regexp.MustCompile(`(?i)^\+?\.inf$`)
	negInf     = regexp.MustCompile(`(?i)^-\.inf$`)
	nanRe      = regexp.MustCompile(`(?i)^\.nan$`)
	sexaInt    = regexp.MustCompile(`^[-+]?[0-9][0-9_]*(:[0-5]?[0-9]){1,2}$`)
	sexaFloat  = regexp.MustCompile(`^[-+]?[0-9][0-9_]*(:[0-5]?[0-9]){1,2}\.[0-9_]*$`)
	floatRe    = regexp.MustCompile(`^(?:[-+]?([0-9][0-9_,]*)?\.[0-9]*([eE][-+][0-9]+)?)$`)
	dotOnly    = regexp.MustCompile(`\A[-+]?\.\z`)
	intStrict  = regexp.MustCompile(`^(?:[-+]?0b[0-1_]+|[-+]?0[0-7_]+|[-+]?(0|[1-9][0-9_]*)|[-+]?0x[0-9a-fA-F_]+)$`)
	intLegacy  = regexp.MustCompile(`^(?:[-+]?0b[0-1_,]+|[-+]?0[0-7_,]+|[-+]?(?:0|[1-9](?:[0-9]|,[0-9]|_[0-9])*)|[-+]?0x[0-9a-fA-F_,]+)$`)
	dateParts  = regexp.MustCompile(`^(-?\d{4})-(\d{1,2})-(\d{1,2})`)
	timeParts  = regexp.MustCompile(`(\d+:\d+:\d+)(?:\.(\d*))?\s*(Z|[-+]\d+(:\d\d)?)?`)
	zoneParts  = regexp.MustCompile(`^([+\-]?\d{1,2}):?(\d{1,2})?$`)
	dateTimeSp = regexp.MustCompile(`[ tT]`)
)

// tokenize resolves a plain scalar as Psych's ScalarScanner#tokenize does.
// strict refuses the commas Psych's legacy integers allow.
func tokenize(s string, strict bool) scalar {
	if s == "" {
		return scalar{kind: kindNil}
	}
	if stringish.MatchString(s) || strings.Contains(s, "\n") {
		if utf8.RuneCountInString(s) > 5 {
			return scalar{kind: kindString, str: s}
		}
		switch {
		case notYtonf.MatchString(s):
			return scalar{kind: kindString, str: s}
		case s == "~" || nullWord.MatchString(s):
			return scalar{kind: kindNil}
		case yesWord.MatchString(s):
			return scalar{kind: kindBool, b: true}
		case noWord.MatchString(s):
			return scalar{kind: kindBool, b: false}
		}
		return scalar{kind: kindString, str: s}
	}
	if timeRe.MatchString(s) {
		return parseTime(s)
	}
	if dateRe.MatchString(s) {
		m := dateParts.FindStringSubmatch(s)
		y, _ := strconv.ParseInt(m[1], 10, 64)
		mo, _ := strconv.ParseInt(m[2], 10, 64)
		d, _ := strconv.ParseInt(m[3], 10, 64)
		return scalar{kind: kindDate, str: s, y: y, m: mo, d: d, valid: validDate(y, mo, d)}
	}
	if posInf.MatchString(s) {
		return scalar{kind: kindFloat, f: math.Inf(1)}
	}
	if negInf.MatchString(s) {
		return scalar{kind: kindFloat, f: math.Inf(-1)}
	}
	if nanRe.MatchString(s) {
		return scalar{kind: kindFloat, f: math.NaN()}
	}
	if strings.HasPrefix(s, ":") && len(s) > 1 {
		// /^:(["'])(.*)\1/: a quoted name ends at the last quote of its kind.
		if q := s[1]; q == '"' || q == '\'' {
			if end := strings.LastIndexByte(s[2:], q); end >= 0 {
				return scalar{kind: kindSymbol, str: strings.TrimPrefix(s[2:2+end], ":")}
			}
		}
		return scalar{kind: kindSymbol, str: s[1:]}
	}
	if sexaInt.MatchString(s) {
		var i int64
		for e, n := range strings.Split(s, ":") {
			i += rubyToI(n) * pow60(e)
		}
		return scalar{kind: kindInt, i: big.NewInt(i)}
	}
	if sexaFloat.MatchString(s) {
		var f float64
		for e, n := range strings.Split(s, ":") {
			f += rubyToF(n) * float64(pow60(e))
		}
		return scalar{kind: kindFloat, f: f}
	}
	if floatRe.MatchString(s) {
		if dotOnly.MatchString(s) {
			return scalar{kind: kindString, str: s}
		}
		t := strings.NewReplacer(",", "", "_", "").Replace(s)
		// Ruby's Float() takes no "1." or "1.e3": the dot goes.
		if i := strings.Index(t, "."); i >= 0 && (i+1 == len(t) || t[i+1] == 'e' || t[i+1] == 'E') {
			t = t[:i] + t[i+1:]
		}
		f, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return scalar{kind: kindString, str: s}
		}
		return scalar{kind: kindFloat, f: f}
	}
	intRe := intLegacy
	if strict {
		intRe = intStrict
	}
	if intRe.MatchString(s) {
		if i, ok := parseInt(strings.NewReplacer(",", "", "_", "").Replace(s)); ok {
			return scalar{kind: kindInt, i: i}
		}
	}
	return scalar{kind: kindString, str: s}
}

// pow60 is 60 ** |e - 2|: Psych weighs the parts of a sexagesimal number
// from the left, so "1:30" is 1 hour and 30 minutes whatever its parts.
func pow60(e int) int64 {
	n := e - 2
	if n < 0 {
		n = -n
	}
	r := int64(1)
	for ; n > 0; n-- {
		r *= 60
	}
	return r
}

// rubyToI is String#to_i: the leading integer, underscores between digits
// allowed, and 0 when there is none.
func rubyToI(s string) int64 {
	end := 0
	if end < len(s) && (s[end] == '-' || s[end] == '+') {
		end++
	}
	for end < len(s) && (s[end] >= '0' && s[end] <= '9' || s[end] == '_' && end+1 < len(s) && s[end+1] >= '0' && s[end+1] <= '9') {
		end++
	}
	i, _ := strconv.ParseInt(strings.ReplaceAll(s[:end], "_", ""), 10, 64)
	return i
}

// rubyToF is String#to_f on what the sexagesimal pattern admits.
func rubyToF(s string) float64 {
	f, _ := strconv.ParseFloat(strings.ReplaceAll(s, "_", ""), 64)
	return f
}

// parseInt is Kernel#Integer on a string without separators: a 0b, 0o, 0x
// or 0 prefix sets the base.
func parseInt(s string) (*big.Int, bool) {
	neg := false
	if s != "" && (s[0] == '-' || s[0] == '+') {
		neg = s[0] == '-'
		s = s[1:]
	}
	base := 10
	switch {
	case len(s) > 2 && (s[:2] == "0b" || s[:2] == "0B"):
		base, s = 2, s[2:]
	case len(s) > 2 && (s[:2] == "0x" || s[:2] == "0X"):
		base, s = 16, s[2:]
	case len(s) > 2 && (s[:2] == "0o" || s[:2] == "0O"):
		base, s = 8, s[2:]
	case len(s) > 1 && s[0] == '0':
		base, s = 8, s[1:]
	}
	i, ok := new(big.Int).SetString(s, base)
	if !ok {
		return nil, false
	}
	if neg {
		i.Neg(i)
	}
	return i, true
}

// parseTime reads a timestamp the TIME pattern matched, as Psych's
// parse_time does: the date, the time, an optional fraction, and Z, an
// offset, or nothing.
func parseTime(s string) scalar {
	parts := dateTimeSp.Split(s, 2)
	dm := dateParts.FindStringSubmatch(parts[0])
	tm := timeParts.FindStringSubmatch(parts[1])
	sc := scalar{kind: kindTime, str: s}
	sc.y, _ = strconv.ParseInt(dm[1], 10, 64)
	sc.m, _ = strconv.ParseInt(dm[2], 10, 64)
	sc.d, _ = strconv.ParseInt(dm[3], 10, 64)
	hms := strings.Split(tm[1], ":")
	sc.h, _ = strconv.ParseInt(hms[0], 10, 64)
	sc.min, _ = strconv.ParseInt(hms[1], 10, 64)
	sc.sec, _ = strconv.ParseInt(hms[2], 10, 64)
	if frac := tm[2]; frac != "" {
		frac = (frac + "000000000")[:9]
		sc.nsec, _ = strconv.ParseInt(frac, 10, 64)
	}
	sc.valid = sc.m >= 1 && sc.m <= 12 && sc.d >= 1 && sc.d <= 31 && sc.h <= 24 && sc.min <= 59 && sc.sec <= 60
	switch z := tm[3]; {
	case z == "Z":
		sc.zone = zone{kind: 1}
	case z != "":
		zm := zoneParts.FindStringSubmatch(z)
		if zm == nil {
			sc.valid = false
			return sc
		}
		hh, _ := strconv.Atoi(zm[1])
		mm, _ := strconv.Atoi(zm[2])
		off := hh * 3600
		if off < 0 {
			off -= mm * 60
		} else {
			off += mm * 60
		}
		sc.zone = zone{kind: 2, offset: off}
	}
	return sc
}

// at is the Time a timestamp names: UTC for Z, a fixed offset, or, with no
// zone, the local time of the instant the digits name in UTC.
func (sc scalar) at() time.Time {
	t := time.Date(int(sc.y), time.Month(sc.m), int(sc.d), int(sc.h), int(sc.min), int(sc.sec), int(sc.nsec), time.UTC)
	switch sc.zone.kind {
	case 1:
		return t
	case 2:
		loc := time.FixedZone("", sc.zone.offset)
		return time.Date(int(sc.y), time.Month(sc.m), int(sc.d), int(sc.h), int(sc.min), int(sc.sec), int(sc.nsec), loc)
	}
	return t.In(time.Local)
}

// validDate says whether y-m-d is a day of the proleptic Gregorian
// calendar, which Psych reads dates in.
func validDate(y, m, d int64) bool {
	if m < 1 || m > 12 || d < 1 {
		return false
	}
	days := [...]int64{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	max := days[m-1]
	if m == 2 && (y%4 == 0 && y%100 != 0 || y%400 == 0) {
		max = 29
	}
	return d <= max
}
