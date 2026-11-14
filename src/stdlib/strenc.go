package stdlib

import (
	"github.com/cameo-lang/cameo/src/str"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/object"
)

// The String methods are written for UTF-8. installStringEncodings wraps
// each so that a binary or US-ASCII string behaves as Ruby's does: one
// character per byte, case changed in ASCII alone, and a result in the
// receiver's encoding. A UTF-8 string with UTF-8 arguments goes straight
// to the method.
//
// A UTF-8 or US-ASCII string takes no binary string with bytes above 127
// as an argument: the method would put those bytes, which are no
// characters, into a string of characters. It raises
// Encoding::CompatibilityError, also where Ruby makes a binary string of
// an ASCII receiver. +, << and concat say themselves what they join.
//
// Most methods see a binary string through latin1: each byte as the
// character of the same number, so that UTF-8 code counting characters
// counts bytes; the result comes back the same way. Methods that take a
// pattern instead see the bytes as they are, so that a pattern over bytes
// matches them.

// ownEncoding are the methods that read the receiver's encoding
// themselves, or do not depend on it.
var ownEncoding = map[string]bool{
	"encoding": true, "force_encoding": true, "b": true, "valid_encoding?": true, "scrub": true,
	"encode": true, "unicode_normalize": true, "unicode_normalized?": true, "bytes": true, "bytesize": true, "byteindex": true,
	"unpack": true, "unpack1": true, "hash": true, "to_sym": true, "intern": true, "to_i": true,
	"to_f": true, "hex": true, "oct": true, "==": true, "eql?": true, "===": true, "<=>": true,
	"empty?": true, "ascii_only?": true, "frozen?": true, "-@": true, "+@": true, "dedup": true,
	"to_s": true, "to_str": true, "include?": true, "start_with?": true, "end_with?": true,
	"getbyte": true, "setbyte": true, "bytesplice": true, "append_as_bytes": true, "ord": true,
	"casecmp": true, "casecmp?": true,
}

// byteMethods are the methods a binary string answers: those that read
// or make bytes, compare, and say what the string is. A binary string
// has bytes and no characters, so a method that needs to know what
// character a byte is (its case, whether it is a space or a digit, where
// a line ends) raises for one, where Ruby reads the bytes as ASCII: the
// script says what the bytes are first, with force_encoding.
var byteMethods = map[string]bool{
	"encoding": true, "force_encoding": true, "b": true, "valid_encoding?": true, "scrub": true,
	"encode": true, "ascii_only?": true, "bytes": true, "bytesize": true, "byteindex": true,
	"bytesplice": true, "getbyte": true, "setbyte": true, "append_as_bytes": true,
	"unpack": true, "unpack1": true, "hash": true, "==": true, "eql?": true, "===": true,
	"<=>": true, "empty?": true, "frozen?": true, "freeze": true, "dup": true, "clone": true,
	"-@": true, "+@": true, "dedup": true, "to_s": true, "to_str": true, "inspect": true,
	"dump": true, "+": true, "*": true, "<<": true, "concat": true, "length": true,
	"size": true, "[]": true, "slice": true, "include?": true, "start_with?": true,
	"end_with?": true, "index": true, "rindex": true, "delete_prefix": true,
	"delete_suffix": true, "to_proto": true, "to_value_proto": true, "to_json": true,
	// These say themselves what they do with a binary string.
	"unicode_normalize": true, "unicode_normalized?": true,
}

// patternMethods take a string or a regexp to match the bytes with.
var patternMethods = map[string]bool{
	"sub": true, "gsub": true, "scan": true, "match": true, "match?": true, "=~": true,
	"partition": true, "rpartition": true,
}

// joins are the methods that join the receiver and an argument, and say
// themselves which encodings go together.
var joins = map[string]bool{"+": true, "<<": true, "concat": true}

func toLatin1(s string) string {
	if str.ASCIIOnly(s) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		b.WriteRune(rune(s[i]))
	}
	return b.String()
}

func fromLatin1(s string) string {
	if str.ASCIIOnly(s) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if r < 0x100 {
			b.WriteByte(byte(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// relabel gives the strings a method returned the receiver's encoding,
// through f, in lists too.
func relabel(v object.Value, k object.Kind, f func(string) string) object.Value {
	switch {
	case v.IsString():
		return object.NewString(f(v.Str()), object.EncodingOf(k))
	case v.Kind() == object.KindList:
		elems := v.List().Elems
		out := make([]object.Value, len(elems))
		for i, e := range elems {
			out[i] = relabel(e, k, f)
		}
		return object.NewList(out...)
	}
	return v
}

// asciiWhere gives the UTF-8 strings of v that hold ASCII alone the
// US-ASCII encoding, in lists too: what a method of a US-ASCII string
// made.
func asciiWhere(v object.Value) object.Value {
	switch v.Kind() {
	case object.KindString:
		if str.ASCIIOnly(v.Str()) {
			return object.NewString(v.Str(), str.USASCII)
		}
	case object.KindList:
		elems := v.List().Elems
		out := make([]object.Value, len(elems))
		for i, e := range elems {
			out[i] = asciiWhere(e)
		}
		return object.NewList(out...)
	}
	return v
}

func viewArgs(args []object.Value) []object.Value {
	out := make([]object.Value, len(args))
	for i, a := range args {
		switch {
		case a.IsString():
			out[i] = object.NewString(toLatin1(a.Str()), str.UTF8)
		case a.Kind() == object.KindList:
			out[i] = object.NewList(viewArgs(a.List().Elems)...)
		default:
			out[i] = a
		}
	}
	return out
}

// binaryAmong returns the first binary string with a byte above 127
// among args and the elements of the lists among them.
func binaryAmong(args []object.Value) (object.Value, bool) {
	for _, a := range args {
		switch a.Kind() {
		case object.KindBytes:
			if !str.ASCIIOnly(a.Str()) {
				return a, true
			}
		case object.KindList:
			if b, ok := binaryAmong(a.List().Elems); ok {
				return b, true
			}
		}
	}
	return object.Value{}, false
}

func hasRegexp(args []object.Value) bool {
	for _, a := range args {
		if _, ok := a.AsRegexp(); ok {
			return true
		}
	}
	return false
}

// noCharacters is the error of the method name called on a binary
// string, which it would have to read as characters.
func noCharacters(ctx object.Context, name string) error {
	return ctx.Raise(object.CompatibilityError, "%s reads characters, and a binary string has bytes alone: say what they are with force_encoding first", name)
}

// InstallStringEncodings gives each String method one wrapper at most,
// which reads the receiver's encoding once: a UTF-8 receiver goes to the
// method, a binary one raises unless the method is one of byteMethods
// or patternMethods, and any other is run as its encoding has it.
func InstallStringEncodings() {
	c := object.StringClass
	var names []string
	for name, m := range c.Methods {
		if m.Native != nil && !ownEncoding[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		m := c.Methods[name]
		fn, name := m.Native, name
		// A pattern matches bytes as it matches characters.
		bytes, joins := byteMethods[name] || patternMethods[name], joins[name]
		index := name == "[]" || name == "slice"
		// format says itself that it takes no binary string.
		formats := name == "%" || name == "format"
		c.Define(name, m.Arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			k := self.Kind()
			if k == object.KindBytes {
				if !bytes {
					return object.Value{}, noCharacters(ctx, name)
				}
			} else if !joins && !formats {
				if b, ok := binaryAmong(args); ok {
					return object.Value{}, incompatible(ctx, self, b)
				}
			}
			if k == object.KindString {
				return fn(ctx, self, args, kw, block)
			}
			s := self.Str()
			// An index into a binary or a US-ASCII string is an index
			// into its bytes: the byte is read where it is, without a
			// view of the whole string.
			if index && len(args) == 1 && args[0].Kind() == object.KindInt {
				i := args[0].Int()
				if i < 0 {
					i += int64(len(s))
				}
				if i < 0 || i >= int64(len(s)) {
					return object.Nil, nil
				}
				return object.NewString(s[i:i+1], object.EncodingOf(k)), nil
			}
			// A US-ASCII string holds 7-bit bytes alone, which are
			// UTF-8 as they are: the method runs on them, and what
			// it makes is US-ASCII while it holds nothing else, and
			// UTF-8 once an argument brought other characters in,
			// as in Ruby.
			if k == object.KindASCII && name != "inspect" && name != "dump" {
				v, err := fn(ctx, object.NewString(s, str.ASCII), args, kw, block)
				if err != nil {
					return object.Value{}, err
				}
				return asciiWhere(v), nil
			}
			switch {
			case patternMethods[name] || hasRegexp(args):
				v, err := fn(ctx, self, args, kw, block)
				return relabel(v, k, func(s string) string { return s }), err
			case name == "inspect" || name == "dump":
				return object.NewString(object.QuoteBytes([]byte(s)), str.UTF8), nil
			}
			view := object.NewString(toLatin1(s), str.UTF8)
			v, err := fn(ctx, view, viewArgs(args), kw, block)
			if err != nil {
				return object.Value{}, err
			}
			return relabel(v, k, fromLatin1), nil
		})
		// The wrapper hands the arguments on or copies them, so it keeps
		// their slice no more than the method does.
		c.Methods[name].Borrows = m.Borrows
	}
	// Case changes in ASCII alone in a US-ASCII string. A binary string
	// has no case.
	asciiOnly := func(name string, f func(string) string) {
		utf := c.Methods[name].Native
		def(c, name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			switch self.Kind() {
			case object.KindString:
				return utf(ctx, self, args, kw, block)
			case object.KindBytes:
				return object.Value{}, noCharacters(ctx, name)
			}
			return object.NewString(f(self.Str()), object.EncodingOf(self.Kind())), nil
		})
	}
	asciiOnly("upcase", func(s string) string { return asciiCase(s, asciiUpper) })
	asciiOnly("downcase", func(s string) string { return asciiCase(s, asciiLower) })
	asciiOnly("swapcase", func(s string) string {
		return asciiCase(s, func(c byte) byte {
			if u := asciiUpper(c); u != c {
				return u
			}
			return asciiLower(c)
		})
	})
	asciiOnly("capitalize", func(s string) string {
		low := asciiCase(s, asciiLower)
		if low == "" {
			return low
		}
		return string(asciiUpper(low[0])) + low[1:]
	})
	// each_char and each_line yield in the receiver's encoding.
	// Without a block each_char is an enumerator of the characters.
	def(c, "each_char", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		k, s := self.Kind(), self.Str()
		if k == object.KindBytes {
			return object.Value{}, noCharacters(ctx, "each_char")
		}
		chars := func(ctx object.Context, yield func(object.Value) error) error {
			for i := 0; i < len(s); {
				size := 1
				if k == object.KindString {
					_, size = utf8.DecodeRuneInString(s[i:])
				}
				if err := yield(object.NewString(s[i:i+size], object.EncodingOf(k))); err != nil {
					return err
				}
				i += size
			}
			return nil
		}
		if block.IsZero() {
			return enumFor(self, "each_char", nil, func(ctx object.Context) (object.Value, error) {
				return ctx.Call(self, "size")
			}), nil
		}
		err := chars(ctx, func(v object.Value) error {
			_, err := ctx.Yield(block, v)
			return err
		})
		if err != nil {
			return object.Value{}, err
		}
		return self, nil
	})
	eachLine := c.Methods["each_line"].Native
	def(c, "each_line", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		k := self.Kind()
		if k == object.KindBytes {
			return object.Value{}, noCharacters(ctx, "each_line")
		}
		if k == object.KindString || block.IsZero() {
			return eachLine(ctx, self, args, kw, block)
		}
		for _, line := range strings.SplitAfter(self.Str(), "\n") {
			if line == "" {
				continue
			}
			if _, err := ctx.Yield(block, object.NewString(line, object.EncodingOf(k))); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	// A binary string answers its byte methods alone. The methods above
	// say so themselves; these are the ones that read the receiver's
	// encoding themselves and have no wrapper yet.
	for name, m := range c.Methods {
		if m.Native == nil || !ownEncoding[name] || byteMethods[name] || patternMethods[name] {
			continue
		}
		fn, name, borrows := m.Native, name, m.Borrows
		c.Define(name, m.Arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if self.Kind() == object.KindBytes {
				return object.Value{}, noCharacters(ctx, name)
			}
			return fn(ctx, self, args, kw, block)
		})
		c.Methods[name].Borrows = borrows
	}
	// + makes a string of the encoding both are compatible in.
	plus := c.Methods["+"].Native
	def(c, "+", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if !args[0].IsString() || self.Kind() == object.KindString && args[0].Kind() == object.KindString {
			return plus(ctx, self, args, kw, block)
		}
		k, ok := compatible(self, args[0])
		if !ok {
			return object.Value{}, incompatible(ctx, self, args[0])
		}
		return object.NewString(self.Str()+args[0].Str(), object.EncodingOf(k)), nil
	})
}
