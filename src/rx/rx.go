// Package rx translates a Ruby regular expression into one for Go's regexp
// package, RE2, which matches in linear time and so has no backtracking:
// no backreferences, lookaround, atomic groups, possessive quantifiers or
// subexpression calls. Translate names the first such feature a pattern
// uses, and cameo refuses the pattern.
package rx

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Ruby's Regexp option bits.
const (
	IgnoreCase = 1
	Extended   = 2
	Multiline  = 4
	NoEncoding = 32
)

// Unsupported is a pattern that uses what Go's regexp cannot do.
type Unsupported struct {
	Source, Feature string
}

func (e *Unsupported) Error() string {
	return fmt.Sprintf("This regexp is not supported in Cameo: /%s/ uses %s", e.Source, e.Feature)
}

// Translate is the Go pattern for a Ruby one with its options, and the
// names of its named groups in order. Ruby's ^ and $ match at every line,
// so the pattern is in Go's multiline mode; Ruby's m is Go's s. In a
// pattern with named groups a plain group does not capture, as in Ruby. A
// pattern over bytes (NoEncoding) reads a byte past 7 bits as the
// character of that number, the subject being read the same way.
func Translate(src string, options int) (string, []string, error) {
	t := &translator{src: src, bytes: options&NoEncoding != 0, named: hasNamedGroup(src)}
	var b strings.Builder
	b.WriteString("(?m")
	if options&IgnoreCase != 0 {
		b.WriteString("i")
	}
	if options&Multiline != 0 {
		b.WriteString("s")
	}
	b.WriteString(")")
	t.extended = []bool{options&Extended != 0}
	if err := t.run(&b); err != nil {
		return "", nil, err
	}
	return b.String(), t.names, nil
}

type translator struct {
	src      string
	i        int
	bytes    bool
	named    bool
	names    []string
	extended []bool // per open group: whether x is on
}

func (t *translator) unsupported(feature string) error {
	return &Unsupported{Source: t.src, Feature: feature}
}

func (t *translator) peek(prefix string) bool { return strings.HasPrefix(t.src[t.i:], prefix) }

func (t *translator) run(b *strings.Builder) error {
	for t.i < len(t.src) {
		c := t.src[t.i]
		ext := t.extended[len(t.extended)-1]
		switch {
		case ext && (c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'):
			t.i++
		case ext && c == '#':
			for t.i < len(t.src) && t.src[t.i] != '\n' {
				t.i++
			}
		case c == '\\':
			s, err := t.escape(false)
			if err != nil {
				return err
			}
			b.WriteString(s)
		case c == '[':
			s, err := t.class()
			if err != nil {
				return err
			}
			b.WriteString(s)
		case c == '(':
			if err := t.group(b); err != nil {
				return err
			}
		case c == ')':
			if len(t.extended) > 1 {
				t.extended = t.extended[:len(t.extended)-1]
			}
			b.WriteByte(')')
			t.i++
		case c == '*' || c == '+' || c == '?' || c == '{':
			if err := t.quantifier(b); err != nil {
				return err
			}
		default:
			t.literal(b)
		}
	}
	return nil
}

// literal copies one character, a byte past 7 bits in a pattern over
// bytes as the character of that number.
func (t *translator) literal(b *strings.Builder) {
	c := t.src[t.i]
	if c < 0x80 {
		b.WriteByte(c)
		t.i++
		return
	}
	if t.bytes {
		fmt.Fprintf(b, `\x{%X}`, c)
		t.i++
		return
	}
	_, size := utf8.DecodeRuneInString(t.src[t.i:])
	b.WriteString(t.src[t.i : t.i+size])
	t.i += size
}

func (t *translator) quantifier(b *strings.Builder) error {
	c := t.src[t.i]
	if c == '{' {
		end := strings.IndexByte(t.src[t.i:], '}')
		body := ""
		if end > 0 {
			body = t.src[t.i+1 : t.i+end]
		}
		if !isRepeat(body) {
			// Not a repetition: a literal brace, as in Ruby.
			b.WriteString(`\{`)
			t.i++
			return nil
		}
		if strings.HasPrefix(body, ",") {
			body = "0" + body
		}
		for _, n := range strings.Split(body, ",") {
			if v, err := strconv.Atoi(n); err == nil && v > 1000 {
				return t.unsupported("a repetition count above 1000")
			}
		}
		b.WriteString("{" + body + "}")
		t.i += end + 1
	} else {
		b.WriteByte(c)
		t.i++
	}
	switch {
	case t.peek("+"):
		return t.unsupported("a possessive quantifier")
	case t.peek("?"):
		b.WriteByte('?')
		t.i++
	}
	return nil
}

// isRepeat reports the inside of a brace that is a repetition count:
// {n}, {n,}, {n,m} or Ruby's {,m}.
func isRepeat(body string) bool {
	lo, hi, comma := body, "", false
	if i := strings.IndexByte(body, ','); i >= 0 {
		lo, hi, comma = body[:i], body[i+1:], true
	}
	digits := func(s string) bool {
		for _, c := range s {
			if c < '0' || c > '9' {
				return false
			}
		}
		return true
	}
	return digits(lo) && digits(hi) && (lo != "" || comma && hi != "")
}

func (t *translator) group(b *strings.Builder) error {
	ext := t.extended[len(t.extended)-1]
	switch {
	case t.peek("(?#"):
		end := strings.IndexByte(t.src[t.i:], ')')
		if end < 0 {
			return t.unsupported("an unterminated comment")
		}
		t.i += end + 1
		return nil
	case t.peek("(?="), t.peek("(?!"):
		return t.unsupported("lookahead")
	case t.peek("(?<="), t.peek("(?<!"):
		return t.unsupported("lookbehind")
	case t.peek("(?>"):
		return t.unsupported("an atomic group")
	case t.peek("(?~"):
		return t.unsupported("an absence operator")
	case t.peek("(?("):
		return t.unsupported("a conditional")
	case t.peek("(?<"), t.peek("(?'"):
		closer := byte('>')
		if t.src[t.i+2] == '\'' {
			closer = '\''
		}
		end := strings.IndexByte(t.src[t.i+3:], closer)
		if end <= 0 {
			return t.unsupported("a malformed group name")
		}
		name := t.src[t.i+3 : t.i+3+end]
		t.names = append(t.names, name)
		b.WriteString("(?P<" + name + ">")
		t.i += 3 + end + 1
		t.extended = append(t.extended, ext)
		return nil
	case t.peek("(?:"):
		b.WriteString("(?:")
		t.i += 3
		t.extended = append(t.extended, ext)
		return nil
	case t.peek("(?"):
		// Flags, for the rest of the group or for a group of their own:
		// (?imx-imx) or (?imx-imx:...).
		j := t.i + 2
		on := true
		var goOn, goOff strings.Builder
		x := ext
		for ; j < len(t.src) && t.src[j] != ':' && t.src[j] != ')'; j++ {
			switch t.src[j] {
			case '-':
				on = false
			case 'i', 'm':
				f := "i"
				if t.src[j] == 'm' {
					f = "s"
				}
				if on {
					goOn.WriteString(f)
				} else {
					goOff.WriteString(f)
				}
			case 'x':
				x = on
			default:
				return t.unsupported(fmt.Sprintf("the group option %q", t.src[j]))
			}
		}
		if j >= len(t.src) {
			return t.unsupported("an unterminated group")
		}
		flags := goOn.String()
		if goOff.Len() > 0 {
			flags += "-" + goOff.String()
		}
		if t.src[j] == ')' {
			// For the rest of the enclosing group.
			if flags != "" {
				b.WriteString("(?" + flags + ")")
			}
			t.extended[len(t.extended)-1] = x
			t.i = j + 1
			return nil
		}
		if flags != "" {
			b.WriteString("(?" + flags + ":")
		} else {
			b.WriteString("(?:")
		}
		t.i = j + 1
		t.extended = append(t.extended, x)
		return nil
	}
	if t.named {
		b.WriteString("(?:")
	} else {
		b.WriteByte('(')
	}
	t.i++
	t.extended = append(t.extended, ext)
	return nil
}

// properties maps Ruby's property names Go does not know to what it does.
var properties = map[string]string{
	"alpha": `\p{L}\p{M}`, "alnum": `\p{L}\p{M}\p{Nd}`, "digit": `\p{Nd}`, "upper": `\p{Lu}`,
	"lower": `\p{Ll}`, "space": `\s\x{85}\p{Z}`, "punct": `\p{P}`, "word": `\p{L}\p{M}\p{Nd}\p{Pc}`,
	"cntrl": `\p{Cc}`, "xdigit": `0-9A-Fa-f`, "print": `\P{C}`, "graph": `\p{L}\p{M}\p{N}\p{P}\p{S}`,
	"blank": `\t\p{Zs}`, "any": `\x00-\x{10FFFF}`,
}

// property is a \p{...} or \P{...} escape as the inside of a Go class
// (inner) or an atom.
func (t *translator) property(neg bool, name string, inner bool) (string, error) {
	if strings.HasPrefix(name, "^") {
		neg, name = !neg, name[1:]
	}
	var body string
	switch {
	case properties[strings.ToLower(name)] != "":
		body = properties[strings.ToLower(name)]
	case unicode.Categories[name] != nil || unicode.Scripts[name] != nil:
		body = `\p{` + name + `}`
	default:
		for script := range unicode.Scripts {
			if strings.EqualFold(script, name) {
				body = `\p{` + script + `}`
			}
		}
		if body == "" {
			return "", t.unsupported(`the property \p{` + name + `}`)
		}
	}
	if inner {
		if neg {
			return "", t.unsupported(`a negated property inside a character class`)
		}
		return body, nil
	}
	if neg {
		return "[^" + body + "]", nil
	}
	return "[" + body + "]", nil
}

// escape reads a backslash escape, as an atom or (inner) as the inside of
// a character class.
func (t *translator) escape(inner bool) (string, error) {
	if t.i+1 >= len(t.src) {
		return "", t.unsupported("a trailing backslash")
	}
	c := t.src[t.i+1]
	t.i += 2
	wrap := func(set string) string {
		if inner {
			return set
		}
		return "[" + set + "]"
	}
	switch c {
	case 'A', 'z', 'b', 'B', 'd', 'D', 'w', 'W', 'n', 't', 'r', 'f', 'v', 'a':
		if inner && (c == 'A' || c == 'z' || c == 'b' || c == 'B') {
			if c == 'b' {
				return `\x08`, nil
			}
			return "", t.unsupported(`\` + string(c) + ` in a character class`)
		}
		return `\` + string(c), nil
	case 's':
		return wrap(`\t\n\v\f\r `), nil
	case 'S':
		if inner {
			return "", t.unsupported(`\S in a character class`)
		}
		return `[^\t\n\v\f\r ]`, nil
	case 'h':
		return wrap(`0-9a-fA-F`), nil
	case 'H':
		if inner {
			return "", t.unsupported(`\H in a character class`)
		}
		return `[^0-9a-fA-F]`, nil
	case 'e':
		return `\x1b`, nil
	case 'R':
		if inner {
			return "", t.unsupported(`\R in a character class`)
		}
		return `(?:\r\n|[\n\v\f\r\x{85}\x{2028}\x{2029}])`, nil
	case 'Z':
		return "", t.unsupported(`\Z, which needs lookahead`)
	case 'G':
		return "", t.unsupported(`\G`)
	case 'K':
		return "", t.unsupported(`\K, which keeps text out of the match`)
	case 'X':
		return "", t.unsupported(`\X`)
	case 'k':
		return "", t.unsupported("a named backreference")
	case 'g':
		return "", t.unsupported("a subexpression call")
	case 'p', 'P':
		if t.i >= len(t.src) || t.src[t.i] != '{' {
			return "", t.unsupported(`a malformed \p`)
		}
		end := strings.IndexByte(t.src[t.i:], '}')
		if end < 0 {
			return "", t.unsupported(`a malformed \p`)
		}
		name := t.src[t.i+1 : t.i+end]
		t.i += end + 1
		return t.property(c == 'P', name, inner)
	case 'x':
		return t.hexBytes(inner)
	case 'u':
		var hexDigits string
		if t.i < len(t.src) && t.src[t.i] == '{' {
			end := strings.IndexByte(t.src[t.i:], '}')
			if end < 0 {
				return "", t.unsupported(`a malformed \u`)
			}
			hexDigits = strings.TrimSpace(t.src[t.i+1 : t.i+end])
			if strings.ContainsAny(hexDigits, " \t") {
				return "", t.unsupported(`\u{} with several characters`)
			}
			t.i += end + 1
		} else {
			if t.i+4 > len(t.src) {
				return "", t.unsupported(`a malformed \u`)
			}
			hexDigits = t.src[t.i : t.i+4]
			t.i += 4
		}
		if _, err := strconv.ParseUint(hexDigits, 16, 32); err != nil {
			return "", t.unsupported(`a malformed \u`)
		}
		return `\x{` + hexDigits + `}`, nil
	case '0':
		// \0 and up to two more octal digits.
		j := t.i
		for j < len(t.src) && j < t.i+2 && t.src[j] >= '0' && t.src[j] <= '7' {
			j++
		}
		n, _ := strconv.ParseUint("0"+t.src[t.i:j], 8, 8)
		t.i = j
		return fmt.Sprintf(`\x{%X}`, n), nil
	}
	if c >= '1' && c <= '9' {
		if inner {
			// An octal escape inside a class.
			j := t.i - 1
			for j < len(t.src) && j < t.i+2 && t.src[j] >= '0' && t.src[j] <= '7' {
				j++
			}
			if n, err := strconv.ParseUint(t.src[t.i-1:j], 8, 8); err == nil {
				t.i = j
				return fmt.Sprintf(`\x{%X}`, n), nil
			}
		}
		return "", t.unsupported("a backreference")
	}
	if c >= 0x80 || unicode.IsLetter(rune(c)) || unicode.IsDigit(rune(c)) {
		if c >= 0x80 {
			t.i--
			var b strings.Builder
			t.literal(&b)
			return regexpQuote(b.String()), nil
		}
		return "", t.unsupported(`the escape \` + string(c))
	}
	// Escaped punctuation, and an escaped space in an extended pattern.
	return `\` + string(c), nil
}

// hexBytes reads \xHH escapes: over bytes each is the character of its
// number; over text a run of them past 7 bits is the UTF-8 sequence of a
// character, as Ruby reads /\xE3\x81\x82/.
func (t *translator) hexBytes(inner bool) (string, error) {
	read := func() (byte, bool) {
		j := t.i
		for j < len(t.src) && j < t.i+2 && strings.IndexByte("0123456789abcdefABCDEF", t.src[j]) >= 0 {
			j++
		}
		if j == t.i {
			return 0, false
		}
		n, _ := strconv.ParseUint(t.src[t.i:j], 16, 8)
		t.i = j
		return byte(n), true
	}
	first, ok := read()
	if !ok {
		return "", t.unsupported(`a malformed \x`)
	}
	if first < 0x80 || t.bytes {
		return fmt.Sprintf(`\x{%X}`, first), nil
	}
	seq := []byte{first}
	for !utf8.FullRune(seq) && t.peek(`\x`) {
		t.i += 2
		next, ok := read()
		if !ok {
			return "", t.unsupported(`a malformed \x`)
		}
		seq = append(seq, next)
	}
	r, size := utf8.DecodeRune(seq)
	if r == utf8.RuneError || size != len(seq) {
		return "", t.unsupported(`\x bytes that are no UTF-8 character`)
	}
	return fmt.Sprintf(`\x{%X}`, r), nil
}

// class reads a character class, which Go takes as Ruby writes it but
// for nesting, intersection and the escapes escape translates.
func (t *translator) class() (string, error) {
	var b strings.Builder
	b.WriteByte('[')
	t.i++
	if t.peek("^") {
		b.WriteByte('^')
		t.i++
	}
	first := true
	for t.i < len(t.src) {
		c := t.src[t.i]
		switch {
		case c == ']' && !first:
			b.WriteByte(']')
			t.i++
			return b.String(), nil
		case c == '[' && t.peek("[:"):
			end := strings.Index(t.src[t.i:], ":]")
			if end < 0 {
				return "", t.unsupported("a malformed POSIX class")
			}
			b.WriteString(t.src[t.i : t.i+end+2])
			t.i += end + 2
		case c == '[':
			return "", t.unsupported("a nested character class")
		case c == '&' && t.peek("&&"):
			return "", t.unsupported("a character class intersection")
		case c == '\\':
			s, err := t.escape(true)
			if err != nil {
				return "", err
			}
			b.WriteString(s)
		default:
			t.literal(&b)
		}
		first = false
	}
	return "", t.unsupported("an unterminated character class")
}

// hasNamedGroup reports a (?<name>...) or (?'name'...) group, outside
// classes and escapes.
func hasNamedGroup(src string) bool {
	depth := 0
	for i := 0; i < len(src); i++ {
		switch c := src[i]; {
		case c == '\\':
			i++
		case c == '[':
			depth++
		case c == ']' && depth > 0:
			depth--
		case depth == 0 && strings.HasPrefix(src[i:], "(?<") && i+3 < len(src) && src[i+3] != '=' && src[i+3] != '!',
			depth == 0 && strings.HasPrefix(src[i:], "(?'"):
			return true
		}
	}
	return false
}

func regexpQuote(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(`\.+*?()|[]{}^$`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
