package stdlib

import (
	"strconv"
	"strings"

	"github.com/radiospiel/cameo/src/object"
)

// installCGI defines CGI's escaping functions, what require 'cgi/escape'
// gives: HTML escapes, and URL encoding for forms and for URI components.
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
	html := strings.NewReplacer("'", "&#39;", "&", "&amp;", `"`, "&quot;", "<", "&lt;", ">", "&gt;")
	for _, name := range []string{"escapeHTML", "escape_html", "h"} {
		m.DefineSingleton(name, 1, str(html.Replace))
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
}

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

// unescapeHTML undoes escapeHTML, and decodes &apos; and numeric
// references; any other entity stays as it is, as in Ruby.
func unescapeHTML(s string) string {
	named := map[string]string{"amp": "&", "quot": `"`, "gt": ">", "lt": "<", "apos": "'"}
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '&')
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		s = s[i:]
		end := strings.IndexByte(s, ';')
		if end < 0 {
			b.WriteString(s)
			return b.String()
		}
		ref := s[1:end]
		switch {
		case named[ref] != "":
			b.WriteString(named[ref])
		case strings.HasPrefix(ref, "#x") || strings.HasPrefix(ref, "#X"):
			if n, err := strconv.ParseUint(ref[2:], 16, 32); err == nil && n <= 0x10ffff {
				b.WriteRune(rune(n))
			} else {
				b.WriteString(s[:end+1])
			}
		case strings.HasPrefix(ref, "#"):
			if n, err := strconv.ParseUint(ref[1:], 10, 32); err == nil && n <= 0x10ffff {
				b.WriteRune(rune(n))
			} else {
				b.WriteString(s[:end+1])
			}
		default:
			b.WriteString(s[:end+1])
		}
		s = s[end+1:]
	}
}
