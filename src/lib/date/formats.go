package date

import (
	"regexp"
	"strconv"
	"strings"
)

// The fixed formats: Date._iso8601, _rfc3339, _xmlschema, _rfc2822,
// _httpdate and _jisx0301, each a regular expression of date_parse.c, and
// the zone table of date_zone_to_diff.

var (
	iso8601ExtDatetime = regexp.MustCompile(`(?i)\A\s*(?:([-+]?\d{2,}|-)-(\d{2})?(?:-(\d{2}))?|([-+]?\d{2,})?-(\d{3})|(\d{4}|\d{2})?-w(\d{2})-(\d)|-w-(\d))(?:t(\d{2}):(\d{2})(?::(\d{2})(?:[,.](\d+))?)?(z|[-+]\d{2}(?::?\d{2})?)?)?\s*\z`)
	iso8601BasDatetime = regexp.MustCompile(`(?i)\A\s*(?:([-+]?(?:\d{4}|\d{2})|--)(\d{2}|-)(\d{2})|([-+]?(?:\d{4}|\d{2}))(\d{3})|-(\d{3})|(\d{4}|\d{2})w(\d{2})(\d)|-w(\d{2})(\d)|-w-(\d))(?:t?(\d{2})(\d{2})(?:(\d{2})(?:[,.](\d+))?)?(z|[-+]\d{2}(?:\d{2})?)?)?\s*\z`)
	iso8601ExtTime     = regexp.MustCompile(`(?i)\A\s*(\d{2}):(\d{2})(?::(\d{2})(?:[,.](\d+))?(z|[-+]\d{2}(?::?\d{2})?)?)?\s*\z`)
	iso8601BasTime     = regexp.MustCompile(`(?i)\A\s*(\d{2})(\d{2})(?:(\d{2})(?:[,.](\d+))?(z|[-+]\d{2}(?:\d{2})?)?)?\s*\z`)
	rfc3339Re          = regexp.MustCompile(`(?i)\A\s*(-?\d{4})-(\d{2})-(\d{2})(?:t|\s)(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?(z|[-+]\d{2}:\d{2})\s*\z`)
	xmlschemaDatetime  = regexp.MustCompile(`(?i)\A\s*(-?\d{4,})(?:-(\d{2})(?:-(\d{2}))?)?(?:t(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?)?(z|[-+]\d{2}:\d{2})?\s*\z`)
	xmlschemaTime      = regexp.MustCompile(`(?i)\A\s*(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?(z|[-+]\d{2}:\d{2})?\s*\z`)
	xmlschemaTrunc     = regexp.MustCompile(`(?i)\A\s*(?:--(\d{2})(?:-(\d{2}))?|---(\d{2}))(z|[-+]\d{2}:\d{2})?\s*\z`)
	rfc2822Re          = regexp.MustCompile(`(?i)\A\s*(?:(` + abbrDays + `)\s*,\s+)?(\d{1,2})\s+(` + abbrMonths + `)\s+(-?\d{2,})\s+(\d{2}):(\d{2})(?::(\d{2}))?\s*([-+]\d{4}|ut|gmt|e[sd]t|c[sd]t|m[sd]t|p[sd]t|[a-ik-z])\s*\z`)
	httpdateType1      = regexp.MustCompile(`(?i)\A\s*(` + abbrDays + `)\s*,\s+(\d{2})\s+(` + abbrMonths + `)\s+(-?\d{4})\s+(\d{2}):(\d{2}):(\d{2})\s+(gmt)\s*\z`)
	httpdateType2      = regexp.MustCompile(`(?i)\A\s*(sunday|monday|tuesday|wednesday|thursday|friday|saturday)\s*,\s+(\d{2})\s*-\s*(` + abbrMonths + `)\s*-\s*(\d{2})\s+(\d{2}):(\d{2}):(\d{2})\s+(gmt)\s*\z`)
	httpdateType3      = regexp.MustCompile(`(?i)\A\s*(` + abbrDays + `)\s+(` + abbrMonths + `)\s+(\d{1,2})\s+(\d{2}):(\d{2}):(\d{2})\s+(\d{4})\s*\z`)
	jisx0301Re         = regexp.MustCompile(`(?i)\A\s*([mtshr])?(\d{2})\.(\d{2})\.(\d{2})(?:t(?:(\d{2}):(\d{2})(?::(\d{2})(?:[,.](\d*))?)?(z|[-+]\d{2}(?::?\d{2})?)?)?)?\s*\z`)
)

// setNum sets k from a captured number when it was captured.
func (f *frags) setNum(k, s string) {
	if s != "" {
		f.set(k, atoi(s))
	}
}

// setZone records a zone and its offset.
func (f *frags) setZone(z string) {
	if z == "" {
		return
	}
	f.set("zone", z)
	if off, ok := zoneToDiff(z); ok {
		f.set("offset", off)
	}
}

// setTime records an hour, minute, second and fraction, those captured.
func (f *frags) setTime(h, m, s, frac string) {
	f.setNum("hour", h)
	f.setNum("min", m)
	f.setNum("sec", s)
	if frac != "" {
		f.set("sec_fraction", fractionOf(frac))
	}
}

// completeYear is a two-digit year completed to 1969..2068, as the fixed
// formats do when the year has two digits.
func completeYear(s string) int64 {
	y := atoi(s)
	if len(strings.TrimLeft(s, "-+")) < 4 && y >= 0 && y <= 99 {
		if y >= 69 {
			return y + 1900
		}
		return y + 2000
	}
	return y
}

func parseISO8601(str string) *frags {
	f := newFrags()
	if m := iso8601ExtDatetime.FindStringSubmatch(str); m != nil {
		switch {
		case m[3] != "":
			f.setNum("mday", m[3])
			if m[1] != "-" {
				f.set("year", completeYear(m[1]))
			}
			f.setNum("mon", m[2])
		case m[2] != "":
			if m[1] != "-" {
				f.set("year", completeYear(m[1]))
			}
			f.setNum("mon", m[2])
		case m[1] != "":
			f.set("year", completeYear(m[1]))
		case m[5] != "":
			if m[4] != "" {
				f.set("year", completeYear(m[4]))
			}
			f.setNum("yday", m[5])
		case m[7] != "":
			if m[6] != "" {
				f.set("cwyear", completeYear(m[6]))
			}
			f.setNum("cweek", m[7])
			f.setNum("cwday", m[8])
		case m[9] != "":
			f.setNum("cwday", m[9])
		}
		f.setTime(m[10], m[11], m[12], m[13])
		f.setZone(m[14])
		return f
	}
	if m := iso8601BasDatetime.FindStringSubmatch(str); m != nil {
		switch {
		case m[3] != "":
			f.setNum("mday", m[3])
			if m[1] != "--" {
				f.set("year", completeYear(m[1]))
			}
			if m[2] != "-" {
				f.setNum("mon", m[2])
			}
		case m[5] != "":
			f.set("year", completeYear(m[4]))
			f.setNum("yday", m[5])
		case m[6] != "":
			f.setNum("yday", m[6])
		case m[8] != "":
			f.set("cwyear", completeYear(m[7]))
			f.setNum("cweek", m[8])
			f.setNum("cwday", m[9])
		case m[10] != "":
			f.setNum("cweek", m[10])
			f.setNum("cwday", m[11])
		case m[12] != "":
			f.setNum("cwday", m[12])
		}
		f.setTime(m[13], m[14], m[15], m[16])
		f.setZone(m[17])
		return f
	}
	if m := iso8601ExtTime.FindStringSubmatch(str); m != nil {
		f.setTime(m[1], m[2], m[3], m[4])
		f.setZone(m[5])
		return f
	}
	if m := iso8601BasTime.FindStringSubmatch(str); m != nil {
		f.setTime(m[1], m[2], m[3], m[4])
		f.setZone(m[5])
		return f
	}
	return f
}

func parseRFC3339(str string) *frags {
	f := newFrags()
	if m := rfc3339Re.FindStringSubmatch(str); m != nil {
		f.setNum("year", m[1])
		f.setNum("mon", m[2])
		f.setNum("mday", m[3])
		f.setTime(m[4], m[5], m[6], m[7])
		f.setZone(m[8])
	}
	return f
}

func parseXMLSchema(str string) *frags {
	f := newFrags()
	if m := xmlschemaDatetime.FindStringSubmatch(str); m != nil {
		f.setNum("year", m[1])
		f.setNum("mon", m[2])
		f.setNum("mday", m[3])
		f.setTime(m[4], m[5], m[6], m[7])
		f.setZone(m[8])
		return f
	}
	if m := xmlschemaTime.FindStringSubmatch(str); m != nil {
		f.setTime(m[1], m[2], m[3], m[4])
		f.setZone(m[5])
		return f
	}
	if m := xmlschemaTrunc.FindStringSubmatch(str); m != nil {
		f.setNum("mon", m[1])
		f.setNum("mday", m[2])
		f.setNum("mday", m[3])
		f.setZone(m[4])
	}
	return f
}

func parseRFC2822(str string) *frags {
	f := newFrags()
	if m := rfc2822Re.FindStringSubmatch(str); m != nil {
		if m[1] != "" {
			f.set("wday", dayIndex(m[1]))
		}
		f.setNum("mday", m[2])
		f.set("mon", monthIndex(m[3]))
		y := atoi(m[4])
		if len(strings.TrimLeft(m[4], "-")) < 4 {
			if y >= 50 {
				y += 1900
			} else {
				y += 2000
			}
		}
		f.set("year", y)
		f.setTime(m[5], m[6], m[7], "")
		f.setZone(m[8])
	}
	return f
}

func parseHTTPDate(str string) *frags {
	f := newFrags()
	if m := httpdateType1.FindStringSubmatch(str); m != nil {
		f.set("wday", dayIndex(m[1]))
		f.setNum("mday", m[2])
		f.set("mon", monthIndex(m[3]))
		f.setNum("year", m[4])
		f.setTime(m[5], m[6], m[7], "")
		f.setZone(m[8])
		return f
	}
	if m := httpdateType2.FindStringSubmatch(str); m != nil {
		f.set("wday", dayIndex(m[1]))
		f.setNum("mday", m[2])
		f.set("mon", monthIndex(m[3]))
		y := atoi(m[4])
		if y >= 0 && y <= 99 {
			if y >= 69 {
				y += 1900
			} else {
				y += 2000
			}
		}
		f.set("year", y)
		f.setTime(m[5], m[6], m[7], "")
		f.setZone(m[8])
		return f
	}
	if m := httpdateType3.FindStringSubmatch(str); m != nil {
		f.set("wday", dayIndex(m[1]))
		f.set("mon", monthIndex(m[2]))
		f.setNum("mday", m[3])
		f.setTime(m[4], m[5], m[6], "")
		f.setNum("year", m[7])
	}
	return f
}

func parseJISX0301(str string) *frags {
	f := newFrags()
	m := jisx0301Re.FindStringSubmatch(str)
	if m == nil {
		return parseISO8601(str)
	}
	era := int64(1988)
	if m[1] != "" {
		era = map[byte]int64{'m': 1867, 't': 1911, 's': 1925, 'h': 1988, 'r': 2018}[strings.ToLower(m[1])[0]]
	}
	f.set("year", atoi(m[2])+era)
	f.setNum("mon", m[3])
	f.setNum("mday", m[4])
	f.setTime(m[5], m[6], m[7], m[8])
	f.setZone(m[9])
	return f
}

// zoneTable is Ruby's zonetab: the offsets of the zone names a parsed
// date may carry, in seconds.
var zoneTable = map[string]int64{
	"ut": 0, "gmt": 0, "utc": 0, "z": 0, "wet": 0, "west": 3600, "bst": 3600, "cet": 3600, "cest": 7200,
	"met": 3600, "mest": 7200, "mez": 3600, "mesz": 7200, "eet": 7200, "eest": 10800, "msk": 10800, "msd": 14400,
	"ist": 19800, "jst": 32400, "kst": 32400, "hkt": 28800, "sgt": 28800, "awst": 28800, "acst": 34200, "aest": 36000,
	"aedt": 39600, "nzst": 43200, "nzdt": 46800,
	"ast": -14400, "adt": -10800, "est": -18000, "edt": -14400, "cst": -21600, "cdt": -18000, "mst": -25200, "mdt": -21600,
	"pst": -28800, "pdt": -25200, "akst": -32400, "akdt": -28800, "hst": -36000, "hast": -36000, "hadt": -32400,
	"nst": -12600, "ndt": -9000, "yst": -32400, "ydt": -28800, "wat": 3600, "cat": 7200, "eat": 10800, "sast": 7200,
	"idlw": -43200, "idle": 43200, "nt": -39600, "ahst": -36000, "cadt": 37800, "aust": 36000,
	"a": 3600, "b": 7200, "c": 10800, "d": 14400, "e": 18000, "f": 21600, "g": 25200, "h": 28800, "i": 32400,
	"k": 36000, "l": 39600, "m": 43200, "n": -3600, "o": -7200, "p": -10800, "q": -14400, "r": -18000, "s": -21600,
	"t": -25200, "u": -28800, "v": -32400, "w": -36000, "x": -39600, "y": -43200,
}

var zoneRegionNames = map[string]int64{
	"eastern": -18000, "central": -21600, "mountain": -25200, "pacific": -28800, "alaska": -32400, "hawaii": -36000,
	"atlantic": -14400, "newfoundland": -12600, "greenwich": 0, "british summer": 3600, "japan": 32400,
	"central european": 3600, "eastern european": 7200, "western european": 0, "india": 19800, "china": 28800,
}

var numericZoneRe = regexp.MustCompile(`(?i)\A(?:gmt|utc?)?([-+])(\d+)(?:[,.:](\d+)(?::(\d+))?)?\z`)

// zoneToDiff is date_zone_to_diff: a zone name or an offset as seconds
// east of UTC, and false for a zone it does not know.
func zoneToDiff(zone string) (int64, bool) {
	z := strings.ToLower(strings.TrimSpace(zone))
	dst := int64(0)
	for _, suffix := range []string{" standard time", " daylight time", " dst"} {
		if strings.HasSuffix(z, suffix) {
			if suffix != " standard time" {
				dst = 3600
			}
			z = strings.TrimSuffix(z, suffix)
			break
		}
	}
	z = strings.TrimSpace(z)
	if off, ok := zoneTable[z]; ok {
		return off + dst, true
	}
	if off, ok := zoneRegionNames[z]; ok {
		return off + dst, true
	}
	m := numericZoneRe.FindStringSubmatch(z)
	if m == nil {
		return 0, false
	}
	sign := int64(1)
	if m[1] == "-" {
		sign = -1
	}
	var secs int64
	switch {
	case m[3] == "" && len(m[2]) <= 2:
		secs = atoi(m[2]) * 3600
	case m[3] == "":
		// hhmm, hhmmss
		d := m[2]
		if len(d)%2 == 1 {
			d = "0" + d
		}
		if len(d) > 6 {
			d = d[:6]
		}
		secs = atoi(d[:2]) * 3600
		if len(d) >= 4 {
			secs += atoi(d[2:4]) * 60
		}
		if len(d) >= 6 {
			secs += atoi(d[4:6])
		}
	default:
		secs = atoi(m[2]) * 3600
		if strings.ContainsAny(zone, ",.") && !strings.Contains(zone, ":") {
			// A decimal fraction of an hour.
			frac, _ := strconv.ParseFloat("0."+m[3], 64)
			secs += int64(frac * 3600)
		} else {
			secs += atoi(m[3]) * 60
			if m[4] != "" {
				secs += atoi(m[4])
			}
		}
	}
	return sign*secs + dst, true
}
