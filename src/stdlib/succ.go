package stdlib

import (
	"iter"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// String#succ and the iteration over a range of strings follow CRuby's
// str_succ and rb_str_upto_each in string.c, character by character where
// CRuby works on the bytes of a character: the byte order of UTF-8 is its
// codepoint order, so stepping a character's bytes is stepping its
// codepoint among the codepoints of the same encoded length.

// succ is String#succ. The rightmost alphanumeric character steps to the
// next character of its kind, a digit or a letter, skipping at most one
// character of another kind ("ö" to "ø" past "÷"). At the end of its run
// ("z", "Z", "9") it wraps to the run's first character and carries into
// the alphanumeric to its left, past any other characters between them,
// unless that one is a letter after a digit or a digit after a letter. A
// carry out of the leftmost wrapped character inserts a new one there: "a"
// for a letter run, "1" for a digit run. A string with no alphanumeric
// steps its rightmost character by codepoint, carrying as it wraps.
func succ(s string) string {
	if s == "" {
		return ""
	}
	rs := []rune(s)
	carry, carryPos := rune(1), 0
	foundAlnum := false
	lastAlnum := -1
	n := neighborFound
	for i := len(rs) - 1; i >= 0; i-- {
		if n == neighborNotChar && lastAlnum >= 0 {
			// CRuby looks at the first byte only, so this stops at ASCII
			// kinds alone.
			l := rs[lastAlnum]
			if isASCIIAlpha(l) && isASCIIDigit(rs[i]) || isASCIIDigit(l) && isASCIIAlpha(rs[i]) {
				break
			}
		}
		var next, c rune
		n, next, c = succAlnum(rs[i])
		switch n {
		case neighborNotChar:
			continue
		case neighborFound:
			rs[i] = next
			return string(rs)
		}
		rs[i] = next
		lastAlnum = i
		foundAlnum = true
		carry, carryPos = c, i
	}
	if !foundAlnum {
		for i := len(rs) - 1; i >= 0; i-- {
			next, ok := nextRune(rs[i])
			rs[i] = next
			if ok {
				return string(rs)
			}
			carryPos = i
		}
	}
	out := make([]rune, 0, len(rs)+1)
	out = append(out, rs[:carryPos]...)
	out = append(out, carry)
	out = append(out, rs[carryPos:]...)
	return string(out)
}

type neighbor int

const (
	neighborNotChar neighbor = iota
	neighborFound
	neighborWrapped
)

// succAlnum steps an alphanumeric character: to the next character of its
// kind, found within two steps, or, at the end of its run, to the run's
// first character with a carry. Anything else, and a run of one, is not an
// alphanumeric to step.
func succAlnum(r rune) (n neighbor, next, carry rune) {
	var is func(rune) bool
	switch {
	case unicode.IsDigit(r):
		is = unicode.IsDigit
	case isAlpha(r):
		is = isAlpha
	default:
		return neighborNotChar, r, 0
	}
	cur := r
	for try := 0; try < 2; try++ {
		c, ok := nextRune(cur)
		if !ok {
			break
		}
		if is(c) {
			return neighborFound, c, 0
		}
		cur = c
	}
	first, run := r, 1
	for {
		p, ok := prevRune(first)
		if !ok || !is(p) {
			break
		}
		first, run = p, run+1
	}
	if run == 1 {
		return neighborNotChar, r, 0
	}
	carry = first
	if unicode.IsDigit(r) {
		carry, _ = nextRune(first)
	}
	return neighborWrapped, first, carry
}

// isAlpha is Unicode's Alphabetic property, which Onigmo's alpha class
// matches: the letters, the letter numbers such as "Ⅸ", and the other
// alphabetic marks.
func isAlpha(r rune) bool {
	return unicode.IsLetter(r) || unicode.Is(unicode.Nl, r) || unicode.Is(unicode.Other_Alphabetic, r)
}

func isASCIIAlpha(r rune) bool { return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' }
func isASCIIDigit(r rune) bool { return '0' <= r && r <= '9' }

// runeClass is the range of codepoints encoded in as many bytes as r.
func runeClass(r rune) (lo, hi rune) {
	switch {
	case r < 0x80:
		return 0, 0x7f
	case r < 0x800:
		return 0x80, 0x7ff
	case r < 0x10000:
		return 0x800, 0xffff
	}
	return 0x10000, utf8.MaxRune
}

// nextRune is the codepoint after r of the same encoded length, or, past
// the last one, the first one and false.
func nextRune(r rune) (rune, bool) {
	lo, hi := runeClass(r)
	n := r + 1
	if n == 0xd800 {
		n = 0xe000
	}
	if n > hi {
		return lo, false
	}
	return n, true
}

// prevRune is the codepoint before r of the same encoded length.
func prevRune(r rune) (rune, bool) {
	lo, _ := runeClass(r)
	n := r - 1
	if n == 0xdfff {
		n = 0xd7ff
	}
	if n < lo {
		return 0, false
	}
	return n, true
}

// stringsUpto is String#upto and the elements of a range of strings: lo
// and its successors up to hi, or forever when endless. Two ASCII
// characters step by code; two strings of digits count, keeping lo's
// width ("08" to "11"); otherwise the successors run until hi, or until
// they grow longer than hi, and lo after hi gives nothing.
func stringsUpto(lo, hi string, exclusive, endless bool) iter.Seq[string] {
	return func(yield func(string) bool) {
		ascii := isASCII(lo) && isASCII(hi)
		if !endless && ascii && len(lo) == 1 && len(hi) == 1 {
			c, e := lo[0], hi[0]
			if c > e || exclusive && c == e {
				return
			}
			for {
				if !yield(string(c)) || !exclusive && c == e {
					return
				}
				c++
				if exclusive && c == e {
					return
				}
			}
		}
		if (endless || ascii) && allDigits(lo) && (endless || allDigits(hi)) {
			b, err1 := strconv.ParseInt(lo, 10, 64)
			e, err2 := strconv.ParseInt(hi, 10, 64)
			if err1 == nil && (endless || err2 == nil) {
				for ; endless || b < e || !exclusive && b == e; b++ {
					if !yield(padDigits(b, len(lo))) {
						return
					}
				}
				return
			}
		}
		if endless {
			for cur := lo; cur != ""; cur = succ(cur) {
				if !yield(cur) {
					return
				}
			}
			return
		}
		if c := strings.Compare(lo, hi); c > 0 || exclusive && c == 0 {
			return
		}
		afterEnd := succ(hi)
		for cur := lo; cur != afterEnd; {
			last := !exclusive && cur == hi
			if !yield(cur) || last {
				return
			}
			cur = succ(cur)
			if exclusive && cur == hi || len(cur) > len(hi) || cur == "" {
				return
			}
		}
	}
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isASCIIDigit(rune(s[i])) {
			return false
		}
	}
	return true
}

// padDigits formats n with at least width digits, as C's "%.*d" does.
func padDigits(n int64, width int) string {
	s := strconv.FormatInt(n, 10)
	if len(s) < width {
		s = strings.Repeat("0", width-len(s)) + s
	}
	return s
}
