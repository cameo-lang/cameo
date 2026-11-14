// Package securerandom is Ruby's securerandom library, built into cameo:
// SecureRandom, drawing from the operating system's randomness through
// Go's crypto/rand, and Random::Formatter, the same methods on a Random
// and on any class that includes the module and defines bytes.
//
// It follows securerandom 0.4.1, the version Ruby 3.4 ships: hex, uuid,
// uuid_v4, uuid_v7, random_bytes, bytes, random_number, rand,
// alphanumeric with chars:, base64, urlsafe_base64 and gen_random. A
// Random draws them from its own generator, so a seeded Random gives the
// bytes CRuby's gives; SecureRandom and a class of the script's make
// numbers of their bytes as CRuby's obj_random_ulong_limited and
// int_pair_to_real do. The numbers are stdlib's draws, Random#rand's, over
// the source.
package securerandom

import (
	crand "crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"github.com/cameo-lang/cameo/src/str"
	"math/big"
	"strings"
	"time"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	module    = object.NewModule("SecureRandom")
	formatter = object.NewModule("Formatter")
)

// source is where a formatter's bytes and numbers come from: a Random's
// generator, or a bytes method.
type source interface {
	stdlib.Source
	bytes(n int) []byte
}

// generator is a Random as a source.
type generator struct{ *object.Random }

func (g generator) bytes(n int) []byte { return g.Bytes(n) }

// fromBytes makes numbers of bytes, as CRuby does for an object that is
// not a Random: SecureRandom's from the system's randomness, a script's
// class's from its bytes method. An error of that method is kept for the
// formatter method to return.
type fromBytes struct {
	read func(n int) ([]byte, error)
	err  error
}

func (f *fromBytes) bytes(n int) []byte {
	b, err := f.read(n)
	if err != nil && f.err == nil {
		f.err = err
	}
	if len(b) < n {
		b = append(b, make([]byte, n-len(b))...)
	}
	return b[:n]
}

func (f *fromBytes) Float(inclusive bool) float64 {
	b := f.bytes(8)
	return object.PairToReal(binary.BigEndian.Uint32(b), binary.BigEndian.Uint32(b[4:]), inclusive)
}

// Below is CRuby's obj_random_ulong_limited and random_ulong_limited_big:
// as many bytes as limit has, masked to its bits, and drawn again while
// the result passes limit.
func (f *fromBytes) Below(limit *big.Int) *big.Int {
	if limit.Sign() <= 0 {
		return new(big.Int)
	}
	bits := limit.BitLen()
	mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(bits)), big.NewInt(1))
	for {
		v := new(big.Int).SetBytes(f.bytes((bits + 7) / 8))
		if v.And(v, mask); v.Cmp(limit) <= 0 || f.err != nil {
			return v
		}
	}
}

func (f *fromBytes) BelowInt(limit uint64) uint64 {
	return f.Below(new(big.Int).SetUint64(limit)).Uint64()
}

func secure() *fromBytes {
	return &fromBytes{read: func(n int) ([]byte, error) {
		b := make([]byte, n)
		_, err := crand.Read(b)
		return b, err
	}}
}

// ofBytes is the source of a class of the script's that includes
// Random::Formatter: its bytes method.
func ofBytes(ctx object.Context, self object.Value) *fromBytes {
	return &fromBytes{read: func(n int) ([]byte, error) {
		v, err := ctx.Call(self, "bytes", object.Int(int64(n)))
		if err != nil {
			return nil, err
		}
		s, err := stdlib.WantString(ctx, v)
		return []byte(s), err
	}}
}

// Install defines SecureRandom and Random::Formatter. lib.Install calls it
// once.
func Install() {
	m := module
	object.DefineNative("SecureRandom", object.ClassValue(m))
	m.Consts["VERSION"] = object.NewString("0.4.1", str.ASCII)
	define(func(name string, arity int, f method) {
		m.DefineSingleton(name, arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			src := secure()
			v, err := f(ctx, src, args, kw)
			return finish(src, v, err)
		})
	})
	formatter.Lexical = object.RandomClass
	object.RandomClass.Consts["Formatter"] = object.ClassValue(formatter)
	define(func(name string, arity int, f method) {
		// A Random draws from its generator, as Random::Formatter's methods
		// are its own; a class of the script's that includes the module
		// draws from its bytes, as Ruby's module does.
		formatter.Define(name, arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if self.Kind() == object.KindRandom {
				return f(ctx, generator{self.Random()}, args, kw)
			}
			src := ofBytes(ctx, self)
			v, err := f(ctx, src, args, kw)
			return finish(src, v, err)
		})
		// Random.hex draws from Kernel#rand's generator; the class's own
		// methods, rand, random_number and bytes, stay the core's.
		if object.RandomClass.SingletonMethods[name] == nil {
			object.RandomClass.DefineSingleton(name, arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
				return f(ctx, generator{ctx.Rand()}, args, kw)
			})
		}
	})
	object.RandomClass.Include(formatter)
}

// finish is a formatter method's result, or the error its source's bytes
// method raised.
func finish(src *fromBytes, v object.Value, err error) (object.Value, error) {
	if err == nil && src.err != nil {
		return object.Value{}, src.err
	}
	return v, err
}

type method func(ctx object.Context, src source, args []object.Value, kw *object.Map) (object.Value, error)

// define gives a receiver every formatter method through def.
func define(def func(name string, arity int, f method)) {
	// count is the number of bytes a method takes, sixteen by default.
	count := func(ctx object.Context, args []object.Value) (int, error) {
		if len(args) > 1 {
			return 0, stdlib.ArgError(ctx, len(args), "0..1")
		}
		if len(args) == 0 || args[0].IsNil() {
			return 16, nil
		}
		return stdlib.ByteCount(ctx, args[0])
	}
	bytesFn := func(ctx object.Context, src source, args []object.Value, kw *object.Map) (object.Value, error) {
		n, err := count(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		return object.Bytes(src.bytes(n)), nil
	}
	def("random_bytes", -1, bytesFn)
	def("bytes", -1, bytesFn)
	def("gen_random", -1, bytesFn)
	def("hex", -1, func(ctx object.Context, src source, args []object.Value, kw *object.Map) (object.Value, error) {
		n, err := count(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		return object.NewString(hex.EncodeToString(src.bytes(n)), str.ASCII), nil
	})
	def("base64", -1, func(ctx object.Context, src source, args []object.Value, kw *object.Map) (object.Value, error) {
		n, err := count(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		return object.NewString(base64.StdEncoding.EncodeToString(src.bytes(n)), str.ASCII), nil
	})
	def("urlsafe_base64", -1, func(ctx object.Context, src source, args []object.Value, kw *object.Map) (object.Value, error) {
		if len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "0..2")
		}
		n, err := count(ctx, args[:min(len(args), 1)])
		if err != nil {
			return object.Value{}, err
		}
		padding := len(args) == 2 && args[1].Truthy()
		s := base64.URLEncoding.EncodeToString(src.bytes(n))
		if !padding {
			s = strings.TrimRight(s, "=")
		}
		return object.NewString(s, str.UTF8), nil
	})
	uuid4 := func(ctx object.Context, src source, args []object.Value, kw *object.Map) (object.Value, error) {
		b := src.bytes(16)
		b[6] = b[6]&0x0f | 0x40
		b[8] = b[8]&0x3f | 0x80
		return object.NewString(formatUUID(b), str.ASCII), nil
	}
	def("uuid", 0, uuid4)
	def("uuid_v4", 0, uuid4)
	def("uuid_v7", -1, func(ctx object.Context, src source, args []object.Value, kw *object.Map) (object.Value, error) {
		b := src.bytes(16)
		ms := uint64(time.Now().UnixMilli())
		for i := 0; i < 6; i++ {
			b[i] = byte(ms >> (8 * (5 - i)))
		}
		b[6] = b[6]&0x0f | 0x70
		b[8] = b[8]&0x3f | 0x80
		return object.NewString(formatUUID(b), str.ASCII), nil
	})
	number := func(ctx object.Context, src source, args []object.Value, kw *object.Map) (object.Value, error) {
		return stdlib.RandomNumber(ctx, src, args)
	}
	def("random_number", -1, number)
	def("rand", -1, number)
	def("alphanumeric", -1, func(ctx object.Context, src source, args []object.Value, kw *object.Map) (object.Value, error) {
		n, err := count(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		chars := alphanumerics
		if kw != nil {
			if v, ok := kw.Get(object.Symbol("chars")); ok {
				list, err := stdlib.ToList(ctx, v)
				if err != nil {
					return object.Value{}, err
				}
				chars = make([]string, len(list))
				for i, c := range list {
					if chars[i], err = ctx.ToS(c); err != nil {
						return object.Value{}, err
					}
				}
			}
		}
		return object.NewString(choose(src, chars, n), str.UTF8), nil
	})
}

var alphanumerics = strings.Split("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789", "")

func formatUUID(b []byte) string {
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// choose is Random::Formatter#choose: n characters drawn from source, by
// Ruby's algorithm, so a seeded Random draws the string CRuby's does.
func choose(src source, chars []string, n int) string {
	size := uint64(len(chars))
	if size == 0 || n <= 0 {
		return ""
	}
	if size == 1 {
		return strings.Repeat(chars[0], n)
	}
	m := 1
	limit := size
	for limit*size <= 0x100000000 {
		limit *= size
		m++
	}
	var b strings.Builder
	digits := func(rs uint64, count int) []uint64 {
		var ds []uint64
		for rs > 0 {
			ds = append(ds, rs%size)
			rs /= size
		}
		for len(ds) < count {
			ds = append(ds, 0)
		}
		return ds[:count]
	}
	for m <= n {
		for _, d := range digits(src.BelowInt(limit-1), m) {
			b.WriteString(chars[d])
		}
		n -= m
	}
	if n > 0 {
		for _, d := range digits(src.BelowInt(limit-1), n) {
			b.WriteString(chars[d])
		}
	}
	return b.String()
}
