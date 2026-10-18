package stdlib

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/object"
)

// Array#pack and String#unpack, for the directives that carry values:
// integers of 8, 16, 32 and 64 bits (C c S s L l Q q J j, n N v V, with <
// and > for the byte order and _ and ! accepted), UTF-8 characters (U),
// floats (e E f F d D g G), strings (a A Z), hex (H h), base64 (m) and a
// NUL byte (x). pack makes bytes, or a string when the format starts with
// U, as Ruby's makes a UTF-8 string then.

// directive is one item of a pack format: its letter, its count (-1 for
// *, 1 when none is given), whether a count was given, and its byte order.
type directive struct {
	letter   byte
	count    int
	explicit bool
	big      bool
}

func parseFormat(ctx object.Context, format string) ([]directive, error) {
	var out []directive
	for i := 0; i < len(format); {
		c := format[i]
		i++
		if c == ' ' || c == '\t' || c == '\n' {
			continue
		}
		d := directive{letter: c, count: 1, big: c == 'n' || c == 'N' || c == 'E' || c == 'G' || c == 'g'}
		for i < len(format) && strings.IndexByte("_!<>", format[i]) >= 0 {
			switch format[i] {
			case '>':
				d.big = true
			case '<':
				d.big = false
			}
			i++
		}
		switch {
		case i < len(format) && format[i] == '*':
			d.count, d.explicit = -1, true
			i++
		case i < len(format) && format[i] >= '0' && format[i] <= '9':
			j := i
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				i++
			}
			d.count, _ = strconv.Atoi(format[j:i])
			d.explicit = true
		}
		if strings.IndexByte("CcSsLlQqJjnNvVUeEfFdDgGaAZHhmx", c) < 0 {
			return nil, ctx.Raise(object.ArgumentError, "unknown pack directive '%c' in '%s'", c, format)
		}
		out = append(out, d)
	}
	return out, nil
}

// intSize is the width in bytes of an integer directive, 0 for others.
func intSize(c byte) int {
	switch c {
	case 'C', 'c':
		return 1
	case 'S', 's', 'n', 'v':
		return 2
	case 'L', 'l', 'N', 'V':
		return 4
	case 'Q', 'q', 'J', 'j':
		return 8
	}
	return 0
}

func floatSize(c byte) int {
	switch c {
	case 'e', 'f', 'F', 'g':
		return 4
	case 'E', 'd', 'D', 'G':
		return 8
	}
	return 0
}

// byteOrder reads and appends in one byte order.
type byteOrder interface {
	binary.ByteOrder
	binary.AppendByteOrder
}

func order(big bool) byteOrder {
	if big {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

// textOf reads a string or bytes argument as its bytes.
func textOf(ctx object.Context, v object.Value) (string, error) {
	if b, ok := v.AsBytes(); ok {
		return string(b), nil
	}
	return wantString(ctx, v)
}

func pack(ctx object.Context, elems []object.Value, format string) (object.Value, error) {
	ds, err := parseFormat(ctx, format)
	if err != nil {
		return object.Value{}, err
	}
	var out []byte
	next := 0
	take := func() (object.Value, error) {
		if next >= len(elems) {
			return object.Value{}, ctx.Raise(object.ArgumentError, "too few arguments")
		}
		next++
		return elems[next-1], nil
	}
	for _, d := range ds {
		// times is how many items a numeric directive takes: its count,
		// or every element left for *.
		times := d.count
		if times < 0 {
			times = len(elems) - next
		}
		switch {
		case intSize(d.letter) > 0:
			size := intSize(d.letter)
			for k := 0; k < times; k++ {
				v, err := take()
				if err != nil {
					return object.Value{}, err
				}
				n, err := packInt(ctx, v)
				if err != nil {
					return object.Value{}, err
				}
				buf := make([]byte, 8)
				order(d.big).PutUint64(buf, n)
				if d.big {
					out = append(out, buf[8-size:]...)
				} else {
					out = append(out, buf[:size]...)
				}
			}
		case floatSize(d.letter) > 0:
			for k := 0; k < times; k++ {
				v, err := take()
				if err != nil {
					return object.Value{}, err
				}
				f, ok := v.AsNumber()
				if !ok {
					return object.Value{}, ctx.Raise(object.TypeError, "can't convert %s into Float", kindName(v))
				}
				if floatSize(d.letter) == 4 {
					out = order(d.big).AppendUint32(out, math.Float32bits(float32(f)))
				} else {
					out = order(d.big).AppendUint64(out, math.Float64bits(f))
				}
			}
		case d.letter == 'U':
			for k := 0; k < times; k++ {
				v, err := take()
				if err != nil {
					return object.Value{}, err
				}
				n, err := wantInt(ctx, v, "")
				if err != nil {
					return object.Value{}, err
				}
				if n < 0 || n > 0x7fffffff {
					return object.Value{}, ctx.Raise(object.RangeError, "pack(U): value out of range")
				}
				out = utf8.AppendRune(out, rune(n))
			}
		case d.letter == 'x':
			out = append(out, make([]byte, max(d.count, 0))...)
		default:
			v, err := take()
			if err != nil {
				return object.Value{}, err
			}
			s, err := textOf(ctx, v)
			if err != nil {
				return object.Value{}, err
			}
			out = append(out, packText(d, s)...)
		}
	}
	if len(ds) > 0 && ds[0].letter == 'U' {
		return object.String(string(out)), nil
	}
	return object.Bytes(out), nil
}

// packInt reads an integer as the bits pack stores: its low 64 bits in
// two's complement, so -1 packs as all ones whatever the width.
func packInt(ctx object.Context, v object.Value) (uint64, error) {
	if f, ok := v.AsFloat(); ok {
		return uint64(int64(f)), nil
	}
	if !v.IsInteger() {
		return 0, ctx.Raise(object.TypeError, "no implicit conversion of %s into Integer", kindName(v))
	}
	if n, ok := v.AsInt(); ok {
		return uint64(n), nil
	}
	b := new(big.Int).And(v.Big(), new(big.Int).SetUint64(math.MaxUint64))
	return b.Uint64(), nil
}

// packText is a string directive: a, A and Z pad or cut to the count (Z
// ending in a NUL for *), H and h read hex digits, m encodes base64 in
// lines of count/3*3 bytes, or in one line for m0.
func packText(d directive, s string) []byte {
	switch d.letter {
	case 'a', 'A', 'Z':
		pad := byte(0)
		if d.letter == 'A' {
			pad = ' '
		}
		if d.count < 0 {
			if d.letter == 'Z' {
				return append([]byte(s), 0)
			}
			return []byte(s)
		}
		out := []byte(s)
		if len(out) > d.count {
			out = out[:d.count]
		}
		for len(out) < d.count {
			out = append(out, pad)
		}
		return out
	case 'H', 'h':
		digits := s
		if d.count >= 0 && d.count < len(digits) {
			digits = digits[:d.count]
		}
		if len(digits)%2 == 1 {
			digits += "0"
		}
		if d.letter == 'h' {
			swapped := []byte(digits)
			for i := 0; i+1 < len(swapped); i += 2 {
				swapped[i], swapped[i+1] = swapped[i+1], swapped[i]
			}
			digits = string(swapped)
		}
		out, _ := hex.DecodeString(digits)
		return out
	case 'm':
		enc := base64.StdEncoding.EncodeToString([]byte(s))
		if d.explicit && d.count == 0 {
			return []byte(enc)
		}
		width := 60
		if d.explicit && d.count >= 3 {
			width = d.count / 3 * 4
		}
		var b strings.Builder
		for len(enc) > width {
			b.WriteString(enc[:width] + "\n")
			enc = enc[width:]
		}
		if enc != "" {
			b.WriteString(enc + "\n")
		}
		return []byte(b.String())
	}
	return nil
}

func unpack(ctx object.Context, data []byte, format string) ([]object.Value, error) {
	ds, err := parseFormat(ctx, format)
	if err != nil {
		return nil, err
	}
	out := []object.Value{}
	at := 0
	for _, d := range ds {
		switch {
		case intSize(d.letter) > 0 || floatSize(d.letter) > 0:
			size := intSize(d.letter) + floatSize(d.letter)
			times := d.count
			if times < 0 {
				times = (len(data) - at) / size
			}
			for k := 0; k < times; k++ {
				if at+size > len(data) {
					out = append(out, object.Nil)
					continue
				}
				chunk := data[at : at+size]
				at += size
				out = append(out, unpackNumber(d, chunk))
			}
		case d.letter == 'U':
			for k := 0; (d.count < 0 || k < d.count) && at < len(data); k++ {
				r, size := utf8.DecodeRune(data[at:])
				if r == utf8.RuneError && size <= 1 {
					return nil, ctx.Raise(object.ArgumentError, "malformed UTF-8 character")
				}
				out = append(out, object.Int(r))
				at += size
			}
		case d.letter == 'x':
			at += max(d.count, 0)
		case d.letter == 'm':
			text := strings.Map(func(r rune) rune {
				if r == '\n' || r == '\r' {
					return -1
				}
				return r
			}, string(data[at:]))
			dec, err := base64.StdEncoding.DecodeString(text)
			if err != nil {
				dec, _ = base64.RawStdEncoding.DecodeString(strings.TrimRight(text, "="))
			}
			out = append(out, object.Bytes(dec))
			at = len(data)
		case d.letter == 'H' || d.letter == 'h':
			rest := data[at:]
			s := hex.EncodeToString(rest)
			if d.letter == 'h' {
				b := []byte(s)
				for i := 0; i+1 < len(b); i += 2 {
					b[i], b[i+1] = b[i+1], b[i]
				}
				s = string(b)
			}
			if d.count >= 0 && d.count < len(s) {
				s = s[:d.count]
			}
			at += (len(s) + 1) / 2
			out = append(out, object.String(s))
		default:
			n := len(data) - at
			if d.count >= 0 && d.count < n {
				n = d.count
			}
			field := data[at : at+n]
			at += n
			switch d.letter {
			case 'A':
				field = []byte(strings.TrimRight(string(field), " \x00"))
			case 'Z':
				if i := strings.IndexByte(string(field), 0); i >= 0 {
					if d.count < 0 {
						at -= n - i - 1
					}
					field = field[:i]
				}
			}
			out = append(out, object.Bytes(append([]byte{}, field...)))
		}
	}
	return out, nil
}

func unpackNumber(d directive, chunk []byte) object.Value {
	o := order(d.big)
	switch floatSize(d.letter) {
	case 4:
		return object.Float(float64(math.Float32frombits(o.Uint32(chunk))))
	case 8:
		return object.Float(math.Float64frombits(o.Uint64(chunk)))
	}
	var u uint64
	switch len(chunk) {
	case 1:
		u = uint64(chunk[0])
	case 2:
		u = uint64(o.Uint16(chunk))
	case 4:
		u = uint64(o.Uint32(chunk))
	case 8:
		u = o.Uint64(chunk)
	}
	signed := d.letter == 'c' || d.letter == 's' || d.letter == 'l' || d.letter == 'q' || d.letter == 'j'
	if signed {
		shift := 64 - 8*uint(len(chunk))
		return object.Int(int64(u<<shift) >> shift)
	}
	if u > math.MaxInt64 {
		return object.BigInt(new(big.Int).SetUint64(u))
	}
	return object.Int(int64(u))
}

func installPack() {
	def(object.ListClass, "pack", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		f, err := wantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return pack(ctx, self.List().Elems, f)
	})
	for _, c := range []*object.Class{object.StringClass} {
		def(c, "unpack", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			f, err := wantString(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			data, err := textOf(ctx, self)
			if err != nil {
				return object.Value{}, err
			}
			vals, err := unpack(ctx, []byte(data), f)
			if err != nil {
				return object.Value{}, err
			}
			return object.NewList(vals...), nil
		})
		def(c, "unpack1", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			f, err := wantString(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			data, err := textOf(ctx, self)
			if err != nil {
				return object.Value{}, err
			}
			vals, err := unpack(ctx, []byte(data), f)
			if err != nil || len(vals) == 0 {
				return object.Nil, err
			}
			return vals[0], nil
		})
	}
}
