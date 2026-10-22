package object

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/rx"
)

// Regexp option bits, as Ruby's Regexp constants number them.
const (
	RegexpIgnoreCase = 1
	RegexpExtended   = 2
	RegexpMultiline  = 4
	// RegexpFixedEncoding marks a pattern that matches UTF-8 text only:
	// one with the u flag, a character past ASCII or a \u escape of one.
	RegexpFixedEncoding = 16
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
	// Names are the names of the named groups, one per group in the order
	// they appear; groups may share a name.
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
	if options&RegexpNoEncoding == 0 && fixedEncoding(source) {
		options |= RegexpFixedEncoding
	}
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

// fixedEncoding reports a pattern Ruby gives the UTF-8 encoding for
// good: one with a character past ASCII, or a \u escape of one.
func fixedEncoding(source string) bool {
	for i := 0; i < len(source); i++ {
		switch {
		case source[i] >= 0x80:
			return true
		case source[i] != '\\' || i+1 == len(source):
			continue
		}
		i++
		if source[i] != 'u' {
			continue
		}
		rest := source[i+1:]
		if strings.HasPrefix(rest, "{") {
			end := strings.IndexByte(rest, '}')
			if end < 0 {
				continue
			}
			for _, h := range strings.Fields(rest[1:end]) {
				if n, err := strconv.ParseUint(h, 16, 32); err == nil && n >= 0x80 {
					return true
				}
			}
			continue
		}
		if len(rest) >= 4 {
			if n, err := strconv.ParseUint(rest[:4], 16, 32); err == nil && n >= 0x80 {
				return true
			}
		}
	}
	return false
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

// UniqueNames are the group names, each once, in the order they first
// appear, as Regexp#names lists them.
func (r *Regexp) UniqueNames() []string {
	var out []string
	seen := map[string]bool{}
	for _, n := range r.Names {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// GroupIndexes are the numbers of the groups named name, in order, and
// none for a name the pattern does not have. In a pattern with named
// groups only those capture, numbered in the order they appear, as in
// Ruby.
func (r *Regexp) GroupIndexes(name string) []int {
	var out []int
	for i, n := range r.Names {
		if n == name {
			out = append(out, i+1)
		}
	}
	return out
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

// NameIndex is the number of the group named name that a reference to
// the name reads: of the groups sharing it, the last that took part, or
// the last of them when none did, as in Ruby; -1 for a name the pattern
// does not have.
func (m *Match) NameIndex(name string) int {
	idx := m.Re.GroupIndexes(name)
	if len(idx) == 0 {
		return -1
	}
	for k := len(idx) - 1; k >= 0; k-- {
		if m.Caps[2*idx[k]] >= 0 {
			return idx[k]
		}
	}
	return idx[len(idx)-1]
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
