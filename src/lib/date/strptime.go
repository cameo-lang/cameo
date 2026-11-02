package date

import (
	"regexp"
	"strconv"
	"strings"
)

// Date._strptime, after date_strptime.c: the format's directives read the
// text piece by piece into fragments; what the format does not consume is
// the leftover.

var (
	strpZoneRe = regexp.MustCompile(`(?i)\A(?:gmt|utc?|z|[-+]\d{1,2}(?::?\d{2}(?::?\d{2})?)?|(?:gmt|utc?)[-+]\d{1,2}(?::?\d{2})?|[a-z.\s]+(?:standard|daylight)\s+time\b|[a-z]+(?:\s+dst)?\b)`)
	strpNumRe  = regexp.MustCompile(`\A[-+]?\d+`)
)

// strptime reads str by format, returning the fragments and false when
// the text does not match.
func strptime(str, format string) (*frags, bool) {
	f := newFrags()
	pos := 0
	var cent *int64
	var merid *int64
	var ok bool
	if pos, ok = strptimeAt(str, 0, format, f, &cent, &merid); !ok {
		return nil, false
	}
	if cent != nil {
		for _, k := range []string{"year", "cwyear"} {
			if y, has := f.int(k); has {
				f.values[k] = *cent*100 + y
			}
		}
	}
	if merid != nil {
		if h, has := f.int("hour"); has {
			f.values["hour"] = h%12 + *merid
		}
	}
	if pos < len(str) {
		f.set("leftover", str[pos:])
	}
	return f, true
}

// digits reads up to width digits, a sign first when signed, at str[pos:].
func digits(str string, pos, width int, signed bool) (int64, int, bool) {
	start := pos
	if signed && pos < len(str) && (str[pos] == '-' || str[pos] == '+') {
		pos++
	}
	n := 0
	for pos < len(str) && str[pos] >= '0' && str[pos] <= '9' && (width <= 0 || n < width) {
		pos++
		n++
	}
	if n == 0 {
		return 0, start, false
	}
	v, err := strconv.ParseInt(str[start:pos], 10, 64)
	if err != nil {
		return 0, start, false
	}
	return v, pos, true
}

// name matches one of names at str[pos:], the longest first, ignoring
// case, and returns its index.
func name(str string, pos int, names ...[]string) (int, int, bool) {
	best, bestLen := -1, 0
	for _, list := range names {
		for i, n := range list {
			if len(str)-pos >= len(n) && strings.EqualFold(str[pos:pos+len(n)], n) && len(n) > bestLen {
				best, bestLen = i, len(n)
			}
		}
	}
	if best < 0 {
		return 0, pos, false
	}
	return best, pos + bestLen, true
}

func strptimeAt(str string, pos int, format string, f *frags, cent, merid **int64) (int, bool) {
	for i := 0; i < len(format); i++ {
		c := format[i]
		switch {
		case c == '%' && i+1 < len(format):
			i++
			// Flags and a width, which the digit count takes as its limit.
			for i < len(format) && strings.IndexByte("-_0^#", format[i]) >= 0 {
				i++
			}
			width := 0
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				width = width*10 + int(format[i]-'0')
				i++
			}
			for i < len(format) && format[i] == ':' {
				i++
			}
			if i >= len(format) {
				return pos, false
			}
			d := format[i]
			// A numeric directive followed by another numeric one takes
			// its fixed width, as %Y%m%d reads 20240229.
			nextNumeric := i+1 < len(format) && format[i+1] == '%' && i+2 < len(format) && strings.IndexByte("CDdeFGgHIjkLlMmNQRrSsTUuVvWwXxYy", format[i+2]) >= 0
			var ok bool
			switch d {
			case 'A', 'a':
				var w int
				if w, pos, ok = name(str, pos, dayNames, abbrDayNames); !ok {
					return pos, false
				}
				f.set("wday", int64(w))
			case 'B', 'b', 'h':
				var m int
				if m, pos, ok = name(str, pos, monthNames, abbrMonthNames); !ok {
					return pos, false
				}
				f.set("mon", int64(m+1))
			case 'C':
				var n int64
				w := width
				if w == 0 && nextNumeric {
					w = 2
				}
				if n, pos, ok = digits(str, pos, w, true); !ok {
					return pos, false
				}
				*cent = &n
			case 'c':
				if pos, ok = strptimeAt(str, pos, "%a %b %e %H:%M:%S %Y", f, cent, merid); !ok {
					return pos, false
				}
			case 'D', 'x':
				if pos, ok = strptimeAt(str, pos, "%m/%d/%y", f, cent, merid); !ok {
					return pos, false
				}
			case 'd', 'e':
				if pos < len(str) && str[pos] == ' ' {
					pos++
				}
				var n int64
				if n, pos, ok = digits(str, pos, 2, false); !ok || n < 1 || n > 31 {
					return pos, false
				}
				f.set("mday", n)
			case 'F':
				if pos, ok = strptimeAt(str, pos, "%Y-%m-%d", f, cent, merid); !ok {
					return pos, false
				}
			case 'G':
				var n int64
				w := width
				if w == 0 && nextNumeric {
					w = 4
				}
				if n, pos, ok = digits(str, pos, w, true); !ok {
					return pos, false
				}
				f.set("cwyear", n)
			case 'g':
				var n int64
				if n, pos, ok = digits(str, pos, 2, false); !ok || n > 99 {
					return pos, false
				}
				if n >= 69 {
					n += 1900
				} else {
					n += 2000
				}
				f.set("cwyear", n)
			case 'H', 'k':
				if pos < len(str) && str[pos] == ' ' {
					pos++
				}
				var n int64
				if n, pos, ok = digits(str, pos, 2, false); !ok || n > 24 {
					return pos, false
				}
				f.set("hour", n)
			case 'I', 'l':
				if pos < len(str) && str[pos] == ' ' {
					pos++
				}
				var n int64
				if n, pos, ok = digits(str, pos, 2, false); !ok || n < 1 || n > 12 {
					return pos, false
				}
				f.set("hour", n)
			case 'j':
				var n int64
				if n, pos, ok = digits(str, pos, 3, false); !ok || n < 1 || n > 366 {
					return pos, false
				}
				f.set("yday", n)
			case 'L', 'N':
				start := pos
				w := width
				if w == 0 && nextNumeric {
					w = 3
					if d == 'N' {
						w = 9
					}
				}
				for pos < len(str) && str[pos] >= '0' && str[pos] <= '9' && (w == 0 || pos-start < w) {
					pos++
				}
				if pos == start {
					return pos, false
				}
				f.set("sec_fraction", fractionOf(str[start:pos]))
			case 'M':
				var n int64
				if n, pos, ok = digits(str, pos, 2, false); !ok || n > 59 {
					return pos, false
				}
				f.set("min", n)
			case 'm':
				var n int64
				if n, pos, ok = digits(str, pos, 2, false); !ok || n < 1 || n > 12 {
					return pos, false
				}
				f.set("mon", n)
			case 'n', 't':
				for pos < len(str) && isSpace(str[pos]) {
					pos++
				}
			case 'P', 'p':
				rest := strings.ToLower(str[pos:])
				switch {
				case strings.HasPrefix(rest, "a.m."):
					pos += 4
					*merid = new(int64)
				case strings.HasPrefix(rest, "p.m."):
					pos += 4
					m := int64(12)
					*merid = &m
				case strings.HasPrefix(rest, "am"):
					pos += 2
					*merid = new(int64)
				case strings.HasPrefix(rest, "pm"):
					pos += 2
					m := int64(12)
					*merid = &m
				default:
					return pos, false
				}
			case 'Q':
				var n int64
				if n, pos, ok = digits(str, pos, 0, true); !ok {
					return pos, false
				}
				f.set("seconds", float64(n)/1000)
			case 'R':
				if pos, ok = strptimeAt(str, pos, "%H:%M", f, cent, merid); !ok {
					return pos, false
				}
			case 'r':
				if pos, ok = strptimeAt(str, pos, "%I:%M:%S %p", f, cent, merid); !ok {
					return pos, false
				}
			case 'S':
				var n int64
				if n, pos, ok = digits(str, pos, 2, false); !ok || n > 60 {
					return pos, false
				}
				f.set("sec", n)
			case 's':
				var n int64
				if n, pos, ok = digits(str, pos, 0, true); !ok {
					return pos, false
				}
				f.set("seconds", n)
			case 'T', 'X':
				if pos, ok = strptimeAt(str, pos, "%H:%M:%S", f, cent, merid); !ok {
					return pos, false
				}
			case 'U', 'W':
				var n int64
				if n, pos, ok = digits(str, pos, 2, false); !ok || n > 53 {
					return pos, false
				}
				if d == 'U' {
					f.set("wnum0", n)
				} else {
					f.set("wnum1", n)
				}
			case 'u':
				var n int64
				if n, pos, ok = digits(str, pos, 1, false); !ok || n < 1 || n > 7 {
					return pos, false
				}
				f.set("cwday", n)
			case 'V':
				var n int64
				if n, pos, ok = digits(str, pos, 2, false); !ok || n < 1 || n > 53 {
					return pos, false
				}
				f.set("cweek", n)
			case 'v':
				if pos, ok = strptimeAt(str, pos, "%e-%b-%Y", f, cent, merid); !ok {
					return pos, false
				}
			case 'w':
				var n int64
				if n, pos, ok = digits(str, pos, 1, false); !ok || n > 6 {
					return pos, false
				}
				f.set("wday", n)
			case 'Y':
				var n int64
				w := width
				if w == 0 && nextNumeric {
					w = 4
				}
				if n, pos, ok = digits(str, pos, w, true); !ok {
					return pos, false
				}
				f.set("year", n)
			case 'y':
				var n int64
				if n, pos, ok = digits(str, pos, 2, false); !ok || n > 99 {
					return pos, false
				}
				if *cent == nil {
					if n >= 69 {
						n += 1900
					} else {
						n += 2000
					}
				}
				f.set("year", n)
			case 'Z', 'z':
				m := strpZoneRe.FindString(str[pos:])
				if m == "" {
					return pos, false
				}
				pos += len(m)
				f.setZone(m)
			case '+':
				if pos, ok = strptimeAt(str, pos, "%a %b %e %H:%M:%S %Z %Y", f, cent, merid); !ok {
					return pos, false
				}
			case '%':
				if pos >= len(str) || str[pos] != '%' {
					return pos, false
				}
				pos++
			default:
				// An unknown directive matches itself.
				if pos+1 >= len(str) || str[pos] != '%' || str[pos+1] != d {
					return pos, false
				}
				pos += 2
			}
		case isSpace(c):
			for pos < len(str) && isSpace(str[pos]) {
				pos++
			}
		default:
			if pos >= len(str) || str[pos] != c {
				return pos, false
			}
			pos++
		}
	}
	return pos, true
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r'
}
