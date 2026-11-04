// Package shellwords is Ruby's shellwords library, built into cameo:
// Shellwords.split, escape and join, String#shellsplit and shellescape,
// Array#shelljoin, with the Bourne shell's rules as Ruby applies them.
//
// It follows shellwords 0.2.2, the version Ruby 3.4 ships.
package shellwords

import (
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var module = object.NewModule("Shellwords")

// Install defines Shellwords and the String and Array methods. lib.Install
// calls it once.
func Install() {
	m := module
	object.DefineNative("Shellwords", object.ClassValue(m))
	m.Consts["VERSION"] = object.String("0.2.2")
	split := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
		}
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		words, bad := Split(s)
		if bad != nil {
			return object.Value{}, ctx.Raise(object.ArgumentError, "Unmatched quote at %d: %s", bad.at, bad.text)
		}
		out := make([]object.Value, len(words))
		for i, w := range words {
			out[i] = object.String(w)
		}
		return object.NewList(out...), nil
	}
	escape := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
		}
		s, err := ctx.ToS(args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.String(Escape(s)), nil
	}
	join := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
		}
		list, err := stdlib.ToList(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		parts := make([]string, len(list))
		for i, v := range list {
			s, err := ctx.ToS(v)
			if err != nil {
				return object.Value{}, err
			}
			parts[i] = Escape(s)
		}
		return object.String(strings.Join(parts, " ")), nil
	}
	for _, name := range []string{"shellsplit", "shellwords", "split"} {
		m.DefineSingleton(name, 1, split)
		m.Define(name, 1, split)
	}
	for _, name := range []string{"shellescape", "escape"} {
		m.DefineSingleton(name, 1, escape)
		m.Define(name, 1, escape)
	}
	for _, name := range []string{"shelljoin", "join"} {
		m.DefineSingleton(name, 1, join)
		m.Define(name, 1, join)
	}
	onSelf := func(f object.NativeFn) object.NativeFn {
		return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) != 0 {
				return object.Value{}, stdlib.ArgError(ctx, len(args), "0")
			}
			return f(ctx, self, []object.Value{self}, kw, block)
		}
	}
	object.StringClass.Define("shellsplit", 0, onSelf(split))
	object.StringClass.Define("shellescape", 0, onSelf(escape))
	object.ListClass.Define("shelljoin", 0, onSelf(join))
}

// Unmatched is an unmatched quote: where Ruby's pattern matched it, and
// the text of that match as Ruby's message shows it.
type Unmatched struct {
	at   int
	text string
}

// Split is Shellwords.shellsplit: the words of a line, with single quotes
// taken as they are, double quotes with their escapes, and a backslash
// escaping the next character. An unmatched quote is returned as such.
func Split(line string) ([]string, *Unmatched) {
	var words []string
	var field strings.Builder
	i := 0
	for {
		start := i
		for i < len(line) && isSpace(line[i]) {
			i++
		}
		if i >= len(line) {
			return words, nil
		}
		// unmatched describes the quote at i as Ruby's pattern matched
		// it: the spaces before it, the quote, one space after.
		unmatched := func() *Unmatched {
			end := i + 1
			if end < len(line) && isSpace(line[end]) {
				end++
			}
			text := line[start:end]
			if start > 0 {
				text = "..." + text
			}
			return &Unmatched{at: start, text: text}
		}
		// One word: tokens up to the next unquoted space.
		for {
			if i >= len(line) {
				break
			}
			c := line[i]
			switch {
			case !isSpace(c) && c != '\\' && c != '\'' && c != '"':
				j := i
				for j < len(line) && !isSpace(line[j]) && line[j] != '\\' && line[j] != '\'' && line[j] != '"' {
					j++
				}
				field.WriteString(line[i:j])
				i = j
			case c == '\'':
				end := strings.IndexByte(line[i+1:], '\'')
				if end < 0 {
					return nil, unmatched()
				}
				field.WriteString(line[i+1 : i+1+end])
				i += end + 2
			case c == '"':
				j := i + 1
				var inner strings.Builder
				closed := false
				for j < len(line) {
					if line[j] == '\\' && j+1 < len(line) {
						// \$ \` \" \\ and a backslash before a newline drop
						// the backslash; any other keeps it.
						if strings.IndexByte("$`\"\\\n", line[j+1]) >= 0 {
							inner.WriteByte(line[j+1])
						} else {
							inner.WriteByte('\\')
							inner.WriteByte(line[j+1])
						}
						j += 2
						continue
					}
					if line[j] == '"' {
						closed = true
						break
					}
					inner.WriteByte(line[j])
					j++
				}
				if !closed {
					return nil, unmatched()
				}
				field.WriteString(inner.String())
				i = j + 1
			case c == '\\':
				if i+1 < len(line) && line[i+1] != '\n' {
					field.WriteByte(line[i+1])
					i += 2
				} else if i+1 < len(line) {
					// Ruby's unescape takes no newline: both stay.
					field.WriteString("\\\n")
					i += 2
				} else {
					field.WriteByte('\\')
					i++
				}
			default:
				// A quote-free non-space character is a word; a lone
				// space never reaches here.
				field.WriteByte(c)
				i++
			}
			if i >= len(line) || isSpace(line[i]) {
				break
			}
		}
		words = append(words, field.String())
		field.Reset()
		if i < len(line) {
			i++ // the one separator the pattern takes
		}
	}
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// Escape is Shellwords.shellescape: every character but the safe ones is
// backslashed, a newline is quoted, and the empty string is ”.
func Escape(s string) string {
	if s == "" {
		return "''"
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\n':
			b.WriteString("'\n'")
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			c == '_', c == '-', c == '.', c == ',', c == ':', c == '+', c == '/', c == '@':
			b.WriteByte(c)
		case c >= 0x80:
			// A multibyte character is not in the safe set: Ruby escapes
			// it as one character.
			b.WriteByte('\\')
			b.WriteByte(c)
			for i+1 < len(s) && s[i+1]&0xC0 == 0x80 {
				i++
				b.WriteByte(s[i])
			}
		default:
			b.WriteByte('\\')
			b.WriteByte(c)
		}
	}
	return b.String()
}
