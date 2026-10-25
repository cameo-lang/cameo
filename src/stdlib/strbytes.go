package stdlib

import (
	"math/big"
	"strings"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/object"
)

// The byte methods of String: getbyte reads one byte, and setbyte,
// bytesplice and append_as_bytes return the string with its bytes changed.
// Called on a local they rebind it (see eval's mutations); setbyte then
// returns the byte, as in Ruby, so its method here returns that and SetByte
// computes the changed string.

func installStringBytes() {
	c := object.StringClass
	def(c, "getbyte", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		i, err := wantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		s := self.Str()
		if i < 0 {
			i += int64(len(s))
		}
		if i < 0 || i >= int64(len(s)) {
			return object.Nil, nil
		}
		return object.Int(int64(s[i])), nil
	})
	def(c, "setbyte", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if _, err := SetByte(ctx, self, args); err != nil {
			return object.Value{}, err
		}
		return args[1], nil
	})
	def(c, "bytesplice", -1, bytesplice)
	def(c, "append_as_bytes", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		var b strings.Builder
		b.WriteString(self.Str())
		for _, a := range args {
			switch a.Kind() {
			case object.KindString, object.KindBytes, object.KindASCII:
				b.WriteString(a.Str())
			case object.KindInt:
				b.WriteByte(byte(a.Int()))
			case object.KindBig:
				low := new(big.Int).And(bigOf(a), big.NewInt(0xff))
				b.WriteByte(byte(low.Int64()))
			default:
				return object.Value{}, ctx.Raise(object.TypeError, "wrong argument type %s (expected String or Integer)", kindName(a))
			}
		}
		return object.StringOf(self.Kind(), b.String()), nil
	})
}

// SetByte returns the string self with the byte at args[0] set to the low
// eight bits of args[1], as String#setbyte leaves its receiver.
func SetByte(ctx object.Context, self object.Value, args []object.Value) (object.Value, error) {
	if len(args) != 2 {
		return object.Value{}, argError(ctx, len(args), "2")
	}
	i, err := wantInt(ctx, args[0], "")
	if err != nil {
		return object.Value{}, err
	}
	s := self.Str()
	if i < -int64(len(s)) || i >= int64(len(s)) {
		return object.Value{}, ctx.Raise(object.IndexError, "index %d out of string", i)
	}
	if i < 0 {
		i += int64(len(s))
	}
	var v byte
	switch args[1].Kind() {
	case object.KindInt:
		v = byte(args[1].Int())
	case object.KindBig:
		v = byte(new(big.Int).And(bigOf(args[1]), big.NewInt(0xff)).Int64())
	default:
		if _, err := wantInt(ctx, args[1], ""); err != nil {
			return object.Value{}, err
		}
		v = byte(int64(args[1].Float()))
	}
	b := []byte(s)
	b[i] = v
	return object.StringOf(self.Kind(), string(b)), nil
}

// bytesplice is String#bytesplice: the bytes of a start and length, or of
// a range, replaced by a string or by the bytes of its own start and length
// or range.
func bytesplice(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if len(args) < 2 || len(args) > 5 {
		return object.Value{}, argError(ctx, len(args), "2..5")
	}
	if len(args) == 4 {
		return object.Value{}, argError(ctx, len(args), "2, 3, or 5")
	}
	s := self.Str()
	var beg, n, vbeg, vlen int64
	var val object.Value
	if len(args) == 2 || len(args) == 3 && !args[0].IsInteger() {
		var err error
		if beg, n, err = byteRange(ctx, args[0], int64(len(s))); err != nil {
			return object.Value{}, err
		}
		if val, err = spliceString(ctx, args[1]); err != nil {
			return object.Value{}, err
		}
		vlen = int64(len(val.Str()))
		if len(args) == 3 {
			if vbeg, vlen, err = byteRange(ctx, args[2], vlen); err != nil {
				return object.Value{}, err
			}
		}
	} else {
		var err error
		if beg, err = wantInt(ctx, args[0], ""); err != nil {
			return object.Value{}, err
		}
		if n, err = wantInt(ctx, args[1], ""); err != nil {
			return object.Value{}, err
		}
		if val, err = spliceString(ctx, args[2]); err != nil {
			return object.Value{}, err
		}
		vlen = int64(len(val.Str()))
		if len(args) == 5 {
			if vbeg, err = wantInt(ctx, args[3], ""); err != nil {
				return object.Value{}, err
			}
			if vlen, err = wantInt(ctx, args[4], ""); err != nil {
				return object.Value{}, err
			}
		}
	}
	beg, n, err := checkBytes(ctx, self, beg, n)
	if err != nil {
		return object.Value{}, err
	}
	vbeg, vlen, err = checkBytes(ctx, val, vbeg, vlen)
	if err != nil {
		return object.Value{}, err
	}
	k := self.Kind()
	if k != val.Kind() {
		var ok bool
		if k, ok = compatible(self, val); !ok {
			return object.Value{}, incompatible(ctx, self, val)
		}
	}
	v := val.Str()
	return object.StringOf(k, s[:beg]+v[vbeg:vbeg+vlen]+s[beg+n:]), nil
}

func spliceString(ctx object.Context, v object.Value) (object.Value, error) {
	if !v.IsString() {
		return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into String", kindName(v))
	}
	return v, nil
}

// byteRange is the start and length a range covers in size bytes, as
// Ruby's rb_range_beg_len reads it for bytesplice: the end clamped to the
// size, a negative length as none, and a start outside a RangeError.
func byteRange(ctx object.Context, v object.Value, size int64) (int64, int64, error) {
	r, ok := v.AsRange()
	if !ok {
		return 0, 0, ctx.Raise(object.TypeError, "wrong argument type %s (expected Range)", kindName(v))
	}
	var beg, end int64
	var err error
	if !r.Lo.IsZero() && !r.Lo.IsNil() {
		if beg, err = wantInt(ctx, r.Lo, ""); err != nil {
			return 0, 0, err
		}
	}
	excl := r.Exclusive
	if r.Hi.IsZero() || r.Hi.IsNil() {
		end, excl = -1, false
	} else if end, err = wantInt(ctx, r.Hi, ""); err != nil {
		return 0, 0, err
	}
	if beg < 0 {
		beg += size
	}
	if end < 0 {
		end += size
	}
	if !excl {
		end++
	}
	if beg < 0 || beg > size {
		s, err := ctx.Inspect(v)
		if err != nil {
			return 0, 0, err
		}
		return 0, 0, ctx.Raise(object.RangeError, "%s out of range", s)
	}
	end = min(end, size)
	return beg, max(end-beg, 0), nil
}

// checkBytes resolves a negative start and clamps the length to the
// string, failing as Ruby's str_check_beg_len does, also when either end
// falls inside a character of a UTF-8 string.
func checkBytes(ctx object.Context, str object.Value, beg, n int64) (int64, int64, error) {
	s := str.Str()
	size := int64(len(s))
	if n < 0 {
		return 0, 0, ctx.Raise(object.IndexError, "negative length %d", n)
	}
	if size < beg || beg < 0 && beg+size < 0 {
		return 0, 0, ctx.Raise(object.IndexError, "index %d out of string", beg)
	}
	if beg < 0 {
		beg += size
	}
	n = min(n, size-beg)
	if str.Kind() == object.KindString && !object.ASCIIOnly(s) {
		for _, at := range []int64{beg, beg + n} {
			if at < size && !utf8.RuneStart(s[at]) {
				return 0, 0, ctx.Raise(object.IndexError, "offset %d does not land on character boundary", at)
			}
		}
	}
	return beg, n, nil
}
