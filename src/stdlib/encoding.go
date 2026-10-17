package stdlib

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/radiospiel/amber/src/object"
)

// A String carries one of three encodings, as its kind: UTF-8 (KindString),
// ASCII-8BIT, Ruby's binary (KindBytes), and US-ASCII (KindASCII).
// force_encoding relabels the same bytes, encode converts them, and a
// string may hold bytes its encoding does not allow, which valid_encoding?
// reports and scrub replaces.

// encodings are the encodings amber knows, by the names Ruby accepts for
// them.
var encodings = map[string]object.Kind{
	"UTF-8": object.KindString, "ASCII-8BIT": object.KindBytes, "BINARY": object.KindBytes,
	"US-ASCII": object.KindASCII, "ASCII": object.KindASCII,
}

// encodingName is the name of the encoding a string kind is.
func encodingName(k object.Kind) string {
	switch k {
	case object.KindBytes:
		return "ASCII-8BIT"
	case object.KindASCII:
		return "US-ASCII"
	}
	return "UTF-8"
}

// encodingLabel is how Ruby 3.4 names an encoding in a message: BINARY
// with its other name.
func encodingLabel(k object.Kind) string {
	if k == object.KindBytes {
		return "BINARY (ASCII-8BIT)"
	}
	return encodingName(k)
}

var encodingValues = map[object.Kind]object.Value{}

func encodingValue(k object.Kind) object.Value { return encodingValues[k] }

// encodingArg reads an encoding, an Encoding or its name, as the string
// kind it is.
func encodingArg(ctx object.Context, v object.Value) (object.Kind, error) {
	if inst, ok := v.AsInstance(); ok && inst.Cls == object.EncodingClass {
		return encodings[inst.Fields[0].Str()], nil
	}
	s, err := wantString(ctx, v)
	if err != nil {
		return 0, err
	}
	if k, ok := encodings[strings.ToUpper(s)]; ok {
		return k, nil
	}
	return 0, ctx.Raise(object.ArgumentError, "unknown encoding name - %s", s)
}

// validIn reports whether s is a valid string of encoding k.
func validIn(k object.Kind, s string) bool {
	switch k {
	case object.KindBytes:
		return true
	case object.KindASCII:
		return object.ASCIIOnly(s)
	}
	return utf8.ValidString(s)
}

// compatible is the encoding of a string made from a and b, as Ruby's
// rb_enc_compatible decides it: either's when they share it, the other's
// when one is ASCII alone, and none when both hold other bytes in
// different encodings.
func compatible(a, b object.Value) (object.Kind, bool) {
	ka, kb := a.Kind(), b.Kind()
	switch {
	case ka == kb:
		return ka, true
	case b.Str() == "":
		return ka, true
	case a.Str() == "":
		return kb, true
	case object.ASCIIOnly(b.Str()):
		return ka, true
	case object.ASCIIOnly(a.Str()):
		return kb, true
	}
	return 0, false
}

// CombineEncodings is compatible for a string being built: the encoding
// so far and whether it is empty or ASCII so far, then the next part's.
func CombineEncodings(acc object.Kind, accEmpty, accASCII bool, part object.Kind, partEmpty, partASCII bool) (object.Kind, bool) {
	switch {
	case acc == part, partEmpty:
		return acc, true
	case accEmpty:
		return part, true
	case partASCII:
		return acc, true
	case accASCII:
		return part, true
	}
	return 0, false
}

// IncompatibleEncodings is the message for combining two strings of
// encodings a and b that have nothing in common.
func IncompatibleEncodings(a, b object.Kind) string {
	return "incompatible character encodings: " + encodingLabel(a) + " and " + encodingLabel(b)
}

// incompatible is the error for combining two strings that have no
// encoding in common.
func incompatible(ctx object.Context, a, b object.Value) error {
	return ctx.Raise(object.CompatibilityError, "incompatible character encodings: %s and %s", encodingLabel(a.Kind()), encodingLabel(b.Kind()))
}

// joinEncoded concatenates strings into one of the encoding they are
// compatible in, starting from kind, as interpolation and join do.
func joinEncoded(ctx object.Context, kind object.Kind, parts []object.Value) (object.Value, error) {
	var b strings.Builder
	acc := object.StringOf(kind, "")
	for _, p := range parts {
		k, ok := compatible(acc, p)
		if !ok {
			return object.Value{}, incompatible(ctx, acc, p)
		}
		b.WriteString(p.Str())
		acc = object.StringOf(k, b.String())
	}
	return acc, nil
}

// invalidRun is how many bytes from i on make one invalid sequence, the
// longest start of a UTF-8 sequence there that goes no further, which
// Ruby's scrub replaces as one.
func invalidRun(s string, i int) int {
	b := s[i]
	lo, hi, need := byte(0x80), byte(0xBF), 0
	switch {
	case b >= 0xC2 && b <= 0xDF:
		need = 1
	case b == 0xE0:
		lo, need = 0xA0, 2
	case b >= 0xE1 && b <= 0xEC, b == 0xEE, b == 0xEF:
		need = 2
	case b == 0xED:
		hi, need = 0x9F, 2
	case b == 0xF0:
		lo, need = 0x90, 3
	case b >= 0xF1 && b <= 0xF3:
		need = 3
	case b == 0xF4:
		hi, need = 0x8F, 3
	default:
		return 1
	}
	n := 1
	for ; n <= need && i+n < len(s); n++ {
		if c := s[i+n]; c < lo || c > hi {
			break
		}
		lo, hi = 0x80, 0xBF
	}
	return n
}

// scrub replaces each invalid sequence of s, in encoding k, with what with
// returns for it: in UTF-8 the longest start of a sequence that goes no
// further, in US-ASCII each byte past 7 bits.
func scrub(k object.Kind, s string, with func(bad string) (string, error)) (string, error) {
	if validIn(k, s) {
		return s, nil
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if k == object.KindASCII {
			if s[i] < 0x80 {
				b.WriteByte(s[i])
			} else {
				repl, err := with(s[i : i+1])
				if err != nil {
					return "", err
				}
				b.WriteString(repl)
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r != utf8.RuneError || size > 1 {
			b.WriteString(s[i : i+size])
			i += size
			continue
		}
		n := invalidRun(s, i)
		repl, err := with(s[i : i+n])
		if err != nil {
			return "", err
		}
		b.WriteString(repl)
		i += n
	}
	return b.String(), nil
}

// hexBytes prints bytes as Ruby's conversion errors do: "\xE3\x81".
func hexBytes(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		fmt.Fprintf(&b, `\x%02X`, s[i])
	}
	b.WriteByte('"')
	return b.String()
}

// encode is String#encode: s, in encoding from, converted to encoding to.
// A byte sequence from does not allow is an InvalidByteSequenceError, and
// a character to cannot hold an UndefinedConversionError, unless invalid:
// :replace or undef: :replace says to put replace in its place.
func encode(ctx object.Context, s string, from, to object.Kind, invalid, undef bool, replace string, hasReplace bool) (string, error) {
	if !hasReplace {
		replace = "?"
		if to == object.KindString {
			replace = "�"
		}
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		// The next character in the source encoding, and whether it is one.
		var ch string
		valid := true
		switch from {
		case object.KindString:
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size <= 1 {
				valid = false
				size = invalidRun(s, i)
			}
			ch = s[i : i+size]
		case object.KindASCII:
			ch = s[i : i+1]
			valid = s[i] < 0x80
		default:
			ch = s[i : i+1]
		}
		i += len(ch)
		if !valid {
			if !invalid {
				return "", ctx.Raise(object.InvalidByteSequenceError, "%s on %s", hexBytes(ch), encodingName(from))
			}
			b.WriteString(replace)
			continue
		}
		// Whether the target holds it: everything holds ASCII, UTF-8 holds
		// any character, binary and US-ASCII no character past 7 bits, and
		// a binary byte past 7 bits is no character UTF-8 knows.
		if ch[0] < 0x80 || from == to {
			b.WriteString(ch)
			continue
		}
		if !undef {
			if from == object.KindString {
				r, _ := utf8.DecodeRuneInString(ch)
				return "", ctx.Raise(object.UndefinedConversionError, "U+%04X from UTF-8 to %s", r, encodingName(to))
			}
			return "", ctx.Raise(object.UndefinedConversionError, "%s from %s to %s", hexBytes(ch), encodingName(from), encodingName(to))
		}
		b.WriteString(replace)
	}
	return b.String(), nil
}

// asciiCase maps the ASCII letters of s alone, as a binary or US-ASCII
// string's case methods do.
func asciiCase(s string, f func(c byte) byte) string {
	out := []byte(s)
	for i, c := range out {
		out[i] = f(c)
	}
	return string(out)
}

func asciiUpper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 32
	}
	return c
}

func asciiLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

func installEncodings() {
	e := object.EncodingClass
	for _, k := range []object.Kind{object.KindString, object.KindBytes, object.KindASCII} {
		encodingValues[k] = object.InstanceValue(&object.Instance{Cls: e, Fields: []object.Value{object.String(encodingName(k))}})
	}
	e.Consts["UTF_8"] = encodingValue(object.KindString)
	e.Consts["ASCII_8BIT"] = encodingValue(object.KindBytes)
	e.Consts["BINARY"] = encodingValue(object.KindBytes)
	e.Consts["US_ASCII"] = encodingValue(object.KindASCII)
	name := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self.Instance().Fields[0], nil
	}
	def(e, "name", 0, name)
	def(e, "to_s", 0, name)

	s := object.StringClass
	def(s, "encoding", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return encodingValue(self.Kind()), nil
	})
	def(s, "force_encoding", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		k, err := encodingArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.StringOf(k, self.Str()), nil
	})
	def(s, "b", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.StringOf(object.KindBytes, self.Str()), nil
	})
	def(s, "valid_encoding?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(validIn(self.Kind(), self.Str())), nil
	})
	// scrub replaces each invalid sequence with U+FFFD (? in US-ASCII), a
	// string, or what the block returns for the sequence's bytes.
	def(s, "scrub", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		k := self.Kind()
		repl := "�"
		if k == object.KindASCII {
			repl = "?"
		}
		if len(args) > 0 && !args[0].IsNil() {
			var err error
			if repl, err = wantString(ctx, args[0]); err != nil {
				return object.Value{}, err
			}
		}
		out, err := scrub(k, self.Str(), func(bad string) (string, error) {
			if block.IsZero() {
				return repl, nil
			}
			v, err := ctx.Yield(block, object.Bytes([]byte(bad)))
			if err != nil {
				return "", err
			}
			return wantString(ctx, v)
		})
		return object.StringOf(k, out), err
	})
	// encode converts to an encoding, from another when a second one says
	// what the bytes are, with invalid:, undef: and replace:.
	def(s, "encode", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		from, to := self.Kind(), object.KindString
		var err error
		if len(args) > 0 {
			if to, err = encodingArg(ctx, args[0]); err != nil {
				return object.Value{}, err
			}
		}
		if len(args) > 1 {
			if from, err = encodingArg(ctx, args[1]); err != nil {
				return object.Value{}, err
			}
		}
		opt := func(name string) bool {
			if kw == nil {
				return false
			}
			v, ok := kw.Get(object.Symbol(name))
			sym, _ := v.AsSymbol()
			return ok && sym == "replace"
		}
		replace, hasReplace := "", false
		if kw != nil {
			if v, ok := kw.Get(object.Symbol("replace")); ok {
				if replace, err = wantString(ctx, v); err != nil {
					return object.Value{}, err
				}
				hasReplace = true
			}
		}
		out, err := encode(ctx, self.Str(), from, to, opt("invalid"), opt("undef"), replace, hasReplace)
		if err != nil {
			return object.Value{}, err
		}
		return object.StringOf(to, out), nil
	})
	def(s, "unicode_normalize", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if self.Kind() != object.KindString {
			return object.Value{}, ctx.Raise(object.CompatibilityError, "Unicode Normalization not appropriate for %s", encodingName(self.Kind()))
		}
		return self, nil
	})
}
