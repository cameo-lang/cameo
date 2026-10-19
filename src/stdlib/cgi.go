package stdlib

import (
	"strconv"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
)

// installCGI defines the functions of Ruby's cgi/escape and cgi/util that
// need no I/O: HTML escapes, URL encoding for forms and for URI components,
// escaping of chosen elements, the RFC 1123 date and CGI.pretty. Ruby
// builds escapeElement and pretty on regular expressions with lookaround,
// which cameo's regexps lack, so they are scanned by hand here; the
// comment on each names the expression it follows.
func installCGI() {
	m := object.NewModule("CGI")
	object.ObjectClass.Consts["CGI"] = object.ClassValue(m)
	str := func(f func(string) string) object.NativeFn {
		return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			s, err := wantString(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			return object.String(f(s)), nil
		}
	}
	for _, name := range []string{"escapeHTML", "escape_html", "h"} {
		m.DefineSingleton(name, 1, str(escapeHTML))
	}
	for _, name := range []string{"unescapeHTML", "unescape_html"} {
		m.DefineSingleton(name, 1, str(unescapeHTML))
	}
	m.DefineSingleton("escape", 1, str(func(s string) string { return urlEscape(s, "+") }))
	m.DefineSingleton("unescape", 1, str(func(s string) string { return urlUnescape(s, true) }))
	for _, name := range []string{"escapeURIComponent", "escape_uri_component"} {
		m.DefineSingleton(name, 1, str(func(s string) string { return urlEscape(s, "%20") }))
	}
	for _, name := range []string{"unescapeURIComponent", "unescape_uri_component"} {
		m.DefineSingleton(name, 1, str(func(s string) string { return urlUnescape(s, false) }))
	}
	elementFn := func(f func(s string, elements []string) string) object.NativeFn {
		return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) == 0 {
				return object.Value{}, argError(ctx, 0, "1+")
			}
			s, err := wantString(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			rest := args[1:]
			if len(rest) > 0 && rest[0].Kind() == object.KindList {
				rest = rest[0].List().Elems
			}
			elements := make([]string, len(rest))
			for i, el := range rest {
				if elements[i], err = wantString(ctx, el); err != nil {
					return object.Value{}, err
				}
			}
			return object.String(f(s, elements)), nil
		}
	}
	for _, name := range []string{"escapeElement", "escape_element"} {
		m.DefineSingleton(name, -1, elementFn(escapeElement))
	}
	for _, name := range []string{"unescapeElement", "unescape_element"} {
		m.DefineSingleton(name, -1, elementFn(unescapeElement))
	}
	m.DefineSingleton("rfc1123_date", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		t, ok := args[0].AsTime()
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into Time", kindName(args[0]))
		}
		return object.String(t.UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")), nil
	})
	m.DefineSingleton("pretty", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, argError(ctx, len(args), "1..2")
		}
		s, err := wantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		shift := "  "
		if len(args) == 2 {
			if shift, err = wantString(ctx, args[1]); err != nil {
				return object.Value{}, err
			}
		}
		return object.String(prettyHTML(s, shift)), nil
	})
}

var htmlEscaper = strings.NewReplacer("'", "&#39;", "&", "&amp;", `"`, "&quot;", "<", "&lt;", ">", "&gt;")

func escapeHTML(s string) string { return htmlEscaper.Replace(s) }

// urlEscape percent-encodes every byte but letters, digits and _.-~, and a
// space as space.
func urlEscape(s, space string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', strings.IndexByte("_.-~", c) >= 0:
			b.WriteByte(c)
		case c == ' ':
			b.WriteString(space)
		default:
			b.WriteString("%" + strings.ToUpper(strconv.FormatUint(uint64(c)|0x100, 16)[1:]))
		}
	}
	return b.String()
}

// urlUnescape decodes %XX, and + as a space when plus says so.
func urlUnescape(s string, plus bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '+' && plus:
			b.WriteByte(' ')
		case c == '%' && isHex(s, i+1):
			n, _ := strconv.ParseUint(s[i+1:i+3], 16, 8)
			b.WriteByte(byte(n))
			i += 2
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func isHex(s string, i int) bool {
	if i+2 > len(s) {
		return false
	}
	_, err := strconv.ParseUint(s[i:i+2], 16, 8)
	return err == nil
}

// unescapeHTML decodes what Ruby's does: &amp; &quot; &gt; &lt; &apos;, and
// decimal and hexadecimal references below U+10FFFF. Any other entity
// stays as it is.
func unescapeHTML(s string) string {
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '&')
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		s = s[i:]
		if text, n := htmlEntity(s); n > 0 {
			b.WriteString(text)
			s = s[n:]
		} else {
			b.WriteByte('&')
			s = s[1:]
		}
	}
}

// htmlEntity decodes the entity s starts with, returning its text and its
// length, or a length of 0 when s starts with none Ruby decodes.
func htmlEntity(s string) (string, int) {
	for name, text := range map[string]string{"&amp;": "&", "&quot;": `"`, "&gt;": ">", "&lt;": "<", "&apos;": "'"} {
		if strings.HasPrefix(s, name) {
			return text, len(name)
		}
	}
	if len(s) < 2 || s[1] != '#' {
		return "", 0
	}
	digits, base := s[2:], 10
	if digits != "" && (digits[0] == 'x' || digits[0] == 'X') {
		digits, base = digits[1:], 16
	}
	end := 0
	for end < len(digits) && isDigit(digits[end], base) {
		end++
	}
	if end == 0 || end == len(digits) || digits[end] != ';' {
		return "", 0
	}
	n := len(s) - len(digits) + end + 1
	code, err := strconv.ParseUint(digits[:end], base, 32)
	if err != nil || code >= 0x10ffff {
		return s[:n], n
	}
	return encodeCodepoint(rune(code)), n
}

func isDigit(c byte, base int) bool {
	return c >= '0' && c <= '9' || base == 16 && (c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F')
}

// encodeCodepoint is UTF-8's encoding of r, a surrogate included, as
// Ruby's unescapeHTML writes it.
func encodeCodepoint(r rune) string {
	switch {
	case r < 0x80:
		return string([]byte{byte(r)})
	case r < 0x800:
		return string([]byte{0xc0 | byte(r>>6), 0x80 | byte(r)&0x3f})
	case r < 0x10000:
		return string([]byte{0xe0 | byte(r>>12), 0x80 | byte(r>>6)&0x3f, 0x80 | byte(r)&0x3f})
	}
	return string([]byte{0xf0 | byte(r>>18), 0x80 | byte(r>>12)&0x3f, 0x80 | byte(r>>6)&0x3f, 0x80 | byte(r)&0x3f})
}

// escapeElement escapes the HTML of each tag of the named elements, what
// Ruby matches with /<\/?(?:elements)(?!\w)(?:.|\n)*?>/i.
func escapeElement(s string, elements []string) string {
	return replaceTags(s, elements, "<", ">", escapeHTML)
}

// unescapeElement undoes escapeElement, matching
// /&lt;\/?(?:elements)(?!\w)(?:.|\n)*?&gt;/i as Ruby does.
func unescapeElement(s string, elements []string) string {
	return replaceTags(s, elements, "&lt;", "&gt;", unescapeHTML)
}

// replaceTags replaces each tag of the named elements, from open to the
// first close after the name, by f of it. Names match without regard to
// case and must not be followed by a word character.
func replaceTags(s string, elements []string, open, close string, f func(string) string) string {
	if len(elements) == 0 {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if end := tagAt(s, i, elements, open, close); end > 0 {
			b.WriteString(f(s[i:end]))
			i = end
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// tagAt returns the end of the tag of one of elements that starts at s[i],
// and 0 when none does.
func tagAt(s string, i int, elements []string, open, close string) int {
	if !hasPrefixFold(s[i:], open) {
		return 0
	}
	j := i + len(open)
	if j < len(s) && s[j] == '/' {
		j++
	}
	for _, el := range elements {
		k := j + len(el)
		if k > len(s) || !strings.EqualFold(s[j:k], el) || startsWithWord(s[k:]) {
			continue
		}
		if c := indexFold(s[k:], close); c >= 0 {
			return k + c + len(close)
		}
	}
	return 0
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// indexFold is the index of the first sub in s without regard to ASCII
// case, or -1.
func indexFold(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if strings.EqualFold(s[i:i+len(sub)], sub) {
			return i
		}
	}
	return -1
}

// startsWithWord reports s starting with what Ruby's \w matches, an ASCII
// letter, digit or _.
func startsWithWord(s string) bool { return s != "" && isWordByte(s[0]) }

func isWordByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// prettyHTML indents HTML by shift per nesting level, a port of Ruby's
// CGI.pretty step by step.
func prettyHTML(s, shift string) string {
	// gsub(/(?!\A)<.*?>/m, "\n\\0"): a newline before every tag but one at
	// the start.
	var b strings.Builder
	for i := 0; i < len(s); {
		if i > 0 && s[i] == '<' {
			if end := strings.IndexByte(s[i:], '>'); end >= 0 {
				b.WriteString("\n" + s[i:i+end+1])
				i += end + 1
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	s = b.String()

	// gsub(/<.*?>(?!\n)/m, "\\0\n"): a newline after a tag, which reaches to
	// the first > that no newline follows.
	b.Reset()
	for i := 0; i < len(s); {
		if s[i] == '<' {
			if end := closeNotBeforeNewline(s, i); end > 0 {
				b.WriteString(s[i:end] + "\n")
				i = end
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	lines := b.String()

	// Each closing tag at the start of a line, with the last line at or
	// before it that opens the element, marks a block to indent; "__"
	// fences the block so neither tag matches again.
	for pos := 0; ; {
		end, element := closingTagFrom(lines, pos)
		if end < 0 {
			break
		}
		pos = end
		start := openingTagBefore(lines, end, element)
		block := lines[start:end]
		body := strings.ReplaceAll(strings.TrimSuffix(block, "\n"), "\n", "\n"+shift)
		if strings.HasSuffix(block, "\n") {
			body += "\n"
		}
		lines = lines[:start] + "__" + body + "__" + lines[end:]
	}

	// gsub(/^((?:shift)*)__(?=<\/?\w)/, '\1'): drop the fences.
	b.Reset()
	for i := 0; i < len(lines); {
		if i == 0 || lines[i-1] == '\n' {
			if fence := fenceAt(lines, i, shift); fence >= 0 {
				b.WriteString(lines[i:fence])
				i = fence + 2
				continue
			}
		}
		b.WriteByte(lines[i])
		i++
	}
	return b.String()
}

// closeNotBeforeNewline returns the end of the tag that opens at s[i] and
// closes at the first > no newline follows, and 0 when there is none.
func closeNotBeforeNewline(s string, i int) int {
	for j := i + 1; j < len(s); j++ {
		if s[j] == '>' && (j+1 == len(s) || s[j+1] != '\n') {
			return j + 1
		}
	}
	return 0
}

// closingTagFrom returns the index and element of the first closing tag at
// the start of a line at or after pos, /^<\/(\w+)/, and -1 when there is
// none.
func closingTagFrom(s string, pos int) (int, string) {
	for i := pos; i+2 < len(s); i++ {
		if (i == 0 || s[i-1] == '\n') && s[i] == '<' && s[i+1] == '/' {
			j := i + 2
			for j < len(s) && isWordByte(s[j]) {
				j++
			}
			if j > i+2 {
				return i, s[i+2 : j]
			}
		}
	}
	return -1, ""
}

// openingTagBefore returns the last index at or before end where
// /^\s*<element/i matches, and 0 when there is none, as Ruby's
// lines[nil...end] starts at 0.
func openingTagBefore(s string, end int, element string) int {
	for i := end; i >= 0; i-- {
		if i > 0 && s[i-1] != '\n' {
			continue
		}
		j := i
		for j < len(s) && strings.IndexByte(" \t\n\v\f\r", s[j]) >= 0 {
			j++
		}
		if j < len(s) && s[j] == '<' && hasPrefixFold(s[j+1:], element) {
			return i
		}
	}
	return 0
}

// fenceAt returns the index of the "__" fence after repeats of shift at
// s[i], when a tag follows it, and -1 otherwise. Fewer repeats are tried
// after more, as a regexp backtracks.
func fenceAt(s string, i int, shift string) int {
	reps := 0
	if shift != "" {
		for j := i; strings.HasPrefix(s[j:], shift); j += len(shift) {
			reps++
		}
	}
	for ; reps >= 0; reps-- {
		j := i + reps*len(shift)
		if !strings.HasPrefix(s[j:], "__<") {
			continue
		}
		k := j + 3
		if k < len(s) && s[k] == '/' {
			k++
		}
		if startsWithWord(s[k:]) {
			return j
		}
	}
	return -1
}
