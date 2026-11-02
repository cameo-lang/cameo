package date

import "math"

// The calendar arithmetic of Ruby's date_core.c: a day is its Julian Day
// Number, and sg, the start, is the first Julian Day Number of the
// Gregorian calendar. Before it a civil date is Julian, from it Gregorian;
// ITALY is 1582-10-15, JULIAN (+Inf) keeps every date Julian and GREGORIAN
// (-Inf) every date Gregorian.

const (
	italy     = 2299161
	england   = 2361222
	unixEpoch = 2440588 // the Julian Day Number of 1970-01-01
	daySecs   = 86400
	nanos     = 1_000_000_000
)

var (
	julianStart    = math.Inf(1)
	gregorianStart = math.Inf(-1)
)

func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func floorMod(a, b int64) int64 {
	return a - floorDiv(a, b)*b
}

// civilToJD is c_civil_to_jd: the Julian Day Number of a civil date under
// the start sg.
func civilToJD(y, m, d int64, sg float64) int64 {
	if m <= 2 {
		y--
		m += 12
	}
	a := math.Floor(float64(y) / 100)
	b := 2 - a + math.Floor(a/4)
	jd := math.Floor(365.25*float64(y+4716)) + math.Floor(30.6001*float64(m+1)) + float64(d) + b - 1524
	if jd < sg {
		jd -= b
	}
	return int64(jd)
}

// jdToCivil is c_jd_to_civil.
func jdToCivil(jd int64, sg float64) (y, m, d int64) {
	var a float64
	if float64(jd) < sg {
		a = float64(jd)
	} else {
		x := math.Floor((float64(jd) - 1867216.25) / 36524.25)
		a = float64(jd) + 1 + x - math.Floor(x/4)
	}
	b := a + 1524
	c := math.Floor((b - 122.1) / 365.25)
	dd := math.Floor(365.25 * c)
	e := math.Floor((b - dd) / 30.6001)
	dom := b - dd - math.Floor(30.6001*e)
	if e <= 13 {
		m = int64(e) - 1
		y = int64(c) - 4716
	} else {
		m = int64(e) - 13
		y = int64(c) - 4715
	}
	return y, m, int64(dom)
}

// validCivil is c_valid_civil_p: the Julian Day Number of y-m-d, with a
// negative month or day counted from the end, and false for a date the
// calendar does not have.
func validCivil(y, m, d int64, sg float64) (int64, bool) {
	if m < 0 {
		m += 13
	}
	if m < 1 || m > 12 {
		return 0, false
	}
	if d < 0 {
		last, ok := lastDayOfMonth(y, m, sg)
		if !ok {
			return 0, false
		}
		d = last + d + 1
	}
	if d < 1 {
		return 0, false
	}
	jd := civilToJD(y, m, d, sg)
	y2, m2, d2 := jdToCivil(jd, sg)
	if y2 != y || m2 != m || d2 != d {
		return 0, false
	}
	return jd, true
}

// lastDayOfMonth is c_find_ldom: the last day y-m has.
func lastDayOfMonth(y, m int64, sg float64) (int64, bool) {
	for d := int64(31); d >= 1; d-- {
		if _, ok := validCivil(y, m, d, sg); ok {
			return d, true
		}
	}
	return 0, false
}

// firstDayOfYear is c_find_fdoy: the Julian Day Number of the first day of
// the year.
func firstDayOfYear(y int64, sg float64) (int64, bool) {
	for d := int64(1); d <= 30; d++ {
		if jd, ok := validCivil(y, 1, d, sg); ok {
			return jd, true
		}
	}
	return 0, false
}

// lastDayOfYear is c_find_ldoy.
func lastDayOfYear(y int64, sg float64) (int64, bool) {
	for d := int64(31); d >= 1; d-- {
		if jd, ok := validCivil(y, 12, d, sg); ok {
			return jd, true
		}
	}
	return 0, false
}

// jdToOrdinal is c_jd_to_ordinal: the year and the day of the year.
func jdToOrdinal(jd int64, sg float64) (y, yd int64) {
	y, _, _ = jdToCivil(jd, sg)
	fdoy, _ := firstDayOfYear(y, sg)
	return y, jd - fdoy + 1
}

// validOrdinal is c_valid_ordinal_p.
func validOrdinal(y, yd int64, sg float64) (int64, bool) {
	if yd < 0 {
		ldoy, ok := lastDayOfYear(y, sg)
		if !ok {
			return 0, false
		}
		y2, yd2 := jdToOrdinal(ldoy+yd+1, sg)
		if y2 != y {
			return 0, false
		}
		yd = yd2
	}
	fdoy, ok := firstDayOfYear(y, sg)
	if !ok {
		return 0, false
	}
	jd := fdoy + yd - 1
	y2, yd2 := jdToOrdinal(jd, sg)
	if y2 != y || yd2 != yd {
		return 0, false
	}
	return jd, true
}

// commercialToJD is c_commercial_to_jd: the Julian Day Number of the day
// d of ISO week w of the week-based year y.
func commercialToJD(y, w, d int64, sg float64) int64 {
	fdoy, _ := firstDayOfYear(y, sg)
	rjd2 := fdoy + 3
	return (rjd2 - floorMod(rjd2-1+1, 7)) + 7*(w-1) + (d - 1)
}

// jdToCommercial is c_jd_to_commercial.
func jdToCommercial(jd int64, sg float64) (y, w, d int64) {
	a, _, _ := jdToCivil(jd-3, sg)
	rjd2 := commercialToJD(a+1, 1, 1, sg)
	if jd >= rjd2 {
		y = a + 1
	} else {
		rjd2 = commercialToJD(a, 1, 1, sg)
		y = a
	}
	w = 1 + floorDiv(jd-rjd2, 7)
	d = floorMod(jd+1, 7)
	if d == 0 {
		d = 7
	}
	return y, w, d
}

// validCommercial is c_valid_commercial_p.
func validCommercial(y, w, d int64, sg float64) (int64, bool) {
	if d < 0 {
		d += 8
	}
	if w < 0 {
		rjd2 := commercialToJD(y+1, 1, 1, sg)
		y2, w2, _ := jdToCommercial(rjd2+w*7, sg)
		if y2 != y {
			return 0, false
		}
		w = w2
	}
	jd := commercialToJD(y, w, d, sg)
	y2, w2, d2 := jdToCommercial(jd, sg)
	if y2 != y || w2 != w || d2 != d {
		return 0, false
	}
	return jd, true
}

// weekNumber is the week of the year as %U (f == 0, weeks start on
// Sunday) and %W (f == 1, Monday) count it.
func weekNumber(jd int64, f int64, sg float64) (y, w int64) {
	y, _, _ = jdToCivil(jd, sg)
	fdoy, _ := firstDayOfYear(y, sg)
	rjd2 := fdoy + 6
	w = floorDiv(jd-(rjd2-floorMod(rjd2-f+1, 7))+7, 7)
	return y, w
}

// weeknumToJD is the inverse of weekNumber: the day d (0 = Sunday) of
// week w of year y.
func weeknumToJD(y, w, d, f int64, sg float64) int64 {
	fdoy, _ := firstDayOfYear(y, sg)
	rjd2 := fdoy + 6
	return (rjd2 - floorMod(rjd2-f+1, 7)) + 7*(w-1) + (d - f)
}

func gregorianLeap(y int64) bool {
	return floorMod(y, 4) == 0 && y%100 != 0 || floorMod(y, 400) == 0
}

func julianLeap(y int64) bool { return floorMod(y, 4) == 0 }

func wday(jd int64) int64 { return floorMod(jd+1, 7) }
