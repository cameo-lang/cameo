package object

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/radiospiel/cameo/src/rx"
)

// Regexp option bits, as Ruby's Regexp constants number them.
const (
	RegexpIgnoreCase = 1
	RegexpExtended   = 2
	RegexpMultiline  = 4
	// RegexpNoEncoding is the n flag: the pattern matches bytes.
	RegexpNoEncoding = 32
)

// Regexp is a compiled regular expression: its source and options, which
// are what it is and what travels, and the Go program matching them. Go's
// regexp has no backtracking, so a pattern that needs it is refused (see
// package rx).
type Regexp struct {
	Source  string
	Options int
	// Names are the named groups, in the order they appear.
	Names    []string
	re       *regexp.Regexp
	anchored *regexp.Regexp
}

// NewRegexp compiles source with the options. A pattern Go's regexp
// cannot match is an *rx.Unsupported, and a malformed one another error.
func NewRegexp(source string, options int) (*Regexp, error) {
	pattern, names, err := rx.Translate(source, options)
	if err != nil {
		return nil, err
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	anchored := regexp.MustCompile(`\A(?:` + pattern + `)`)
	return &Regexp{Source: source, Options: options, Names: names, re: re, anchored: anchored}, nil
}

// view is the text the program reads for s: s itself, or for a pattern
// over bytes each byte as the character of its number; at maps an offset
// of s into the view, back an offset of the view into s.
func (r *Regexp) view(s string) (v string, at, back func(int) int) {
	same := func(i int) int { return i }
	if r.Options&RegexpNoEncoding == 0 || allASCII(s) {
		return s, same, same
	}
	var b strings.Builder
	offsets := make([]int, 0, len(s)+1)
	for i := 0; i < len(s); i++ {
		offsets = append(offsets, b.Len())
		b.WriteRune(rune(s[i]))
	}
	offsets = append(offsets, b.Len())
	v = b.String()
	at = func(i int) int { return offsets[i] }
	back = func(j int) int {
		if j < 0 {
			return j
		}
		lo, hi := 0, len(offsets)-1
		for lo < hi {
			mid := (lo + hi) / 2
			if offsets[mid] < j {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		return lo
	}
	return v, at, back
}

func allASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func (r *Regexp) match(s string, idx []int, shift int, back func(int) int) *Match {
	if idx == nil {
		return nil
	}
	caps := make([]int, len(idx))
	for i, c := range idx {
		if c < 0 {
			caps[i] = -1
			continue
		}
		caps[i] = back(c + shift)
	}
	return &Match{Input: s, Caps: caps, Re: r}
}

// MatchAt finds the first match starting at or after byte offset pos and
// returns it or nil. From a position past 0 the program reads the text
// from there, so ^, \A and \b see that position as the start.
func (r *Regexp) MatchAt(s string, pos int) *Match {
	v, at, back := r.view(s)
	p := at(pos)
	return r.match(s, r.re.FindStringSubmatchIndex(v[p:]), p, back)
}

// MatchHere is a match that starts at byte offset pos, or nil.
func (r *Regexp) MatchHere(s string, pos int) *Match {
	v, at, back := r.view(s)
	p := at(pos)
	return r.match(s, r.anchored.FindStringSubmatchIndex(v[p:]), p, back)
}

// FindAll is every match left to right, at most limit of them for a
// limit of 0 or more, as Ruby finds them: where a match ends the next may
// be empty, which Go's regexp passes over, so such a one is put back.
func (r *Regexp) FindAll(s string, limit int) []*Match {
	v, _, back := r.view(s)
	found := r.re.FindAllStringSubmatchIndex(v, -1)
	var out []*Match
	for i, idx := range found {
		if limit >= 0 && len(out) >= limit {
			break
		}
		out = append(out, r.match(s, idx, 0, back))
		end := idx[1]
		if idx[0] == end || i+1 < len(found) && found[i+1][0] == end {
			continue
		}
		if empty := r.anchored.FindStringSubmatchIndex(v[end:]); empty != nil && empty[1] == 0 && (limit < 0 || len(out) < limit) {
			out = append(out, r.match(s, empty, end, back))
		}
	}
	return out
}

// GroupIndex is the number of a named group, or -1. In a pattern with
// named groups only those capture, numbered in the order they appear, as
// in Ruby.
func (r *Regexp) GroupIndex(name string) int {
	for i, n := range r.Names {
		if n == name {
			return i + 1
		}
	}
	return -1
}

// Inspect is /source/flags, as Regexp#inspect prints it.
func (r *Regexp) Inspect() string {
	var b strings.Builder
	b.WriteByte('/')
	for i := 0; i < len(r.Source); i++ {
		switch c := r.Source[i]; c {
		case '\\':
			b.WriteByte(c)
			if i+1 < len(r.Source) {
				i++
				b.WriteByte(r.Source[i])
			}
		case '/':
			b.WriteString(`\/`)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('/')
	if r.Options&RegexpMultiline != 0 {
		b.WriteByte('m')
	}
	if r.Options&RegexpIgnoreCase != 0 {
		b.WriteByte('i')
	}
	if r.Options&RegexpExtended != 0 {
		b.WriteByte('x')
	}
	if r.Options&RegexpNoEncoding != 0 {
		b.WriteByte('n')
	}
	return b.String()
}

// String is Regexp#to_s: (?on-off:source), which embeds in another pattern
// with the same meaning.
func (r *Regexp) String() string {
	on, off := "", ""
	for _, f := range []struct {
		bit int
		c   string
	}{{RegexpMultiline, "m"}, {RegexpIgnoreCase, "i"}, {RegexpExtended, "x"}} {
		if r.Options&f.bit != 0 {
			on += f.c
		} else {
			off += f.c
		}
	}
	if off != "" {
		off = "-" + off
	}
	return "(?" + on + off + ":" + r.Source + ")"
}

// Match is a successful match: its input, the byte spans of the whole
// match and of each group (-1 for a group that took no part), and the
// regexp.
type Match struct {
	Input string
	Caps  []int
	Re    *Regexp
}

// Groups is the number of groups, not counting the whole match.
func (m *Match) Groups() int { return len(m.Caps)/2 - 1 }

// Group is the text of group i, and false for one that took no part.
func (m *Match) Group(i int) (string, bool) {
	if i < 0 || 2*i+1 >= len(m.Caps) || m.Caps[2*i] < 0 {
		return "", false
	}
	return m.Input[m.Caps[2*i]:m.Caps[2*i+1]], true
}

// CharOffset is a byte offset of the input as a character offset, as
// Ruby's MatchData#begin counts.
func (m *Match) CharOffset(b int) int { return utf8.RuneCountInString(m.Input[:b]) }

func (m *Match) key() string {
	var b strings.Builder
	b.WriteString(m.Re.Source)
	b.WriteByte(0)
	b.WriteString(m.Input)
	for _, c := range m.Caps {
		b.WriteByte(',')
		b.WriteString(strconv.Itoa(c))
	}
	return b.String()
}
