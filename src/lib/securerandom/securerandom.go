// Package securerandom is Ruby's securerandom library, built into cameo:
// SecureRandom, drawing from the operating system's randomness through
// Go's crypto/rand, and Random::Formatter, the same methods on a Random.
//
// It follows securerandom 0.4.1, the version Ruby 3.4 ships: hex, uuid,
// uuid_v4, uuid_v7, random_bytes, bytes, random_number, rand,
// alphanumeric with chars:, base64, urlsafe_base64 and gen_random. A
// Random draws them from its own generator, so a seeded Random gives the
// bytes CRuby's gives.
package securerandom

import (
	crand "crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"math"
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

// source is where a formatter's bytes and numbers come from: the system's
// randomness for SecureRandom, a Random's generator for Random::Formatter.
type source interface {
	bytes(n int) []byte
	// below is a uniform integer in [0, limit).
	below(limit uint64) uint64
	float() float64
}

type secure struct{}

func (secure) bytes(n int) []byte {
	b := make([]byte, n)
	_, _ = crand.Read(b)
	return b
}

func (secure) below(limit uint64) uint64 {
	n, _ := crand.Int(crand.Reader, new(big.Int).SetUint64(limit))
	return n.Uint64()
}

func (s secure) float() float64 {
	// 53 random bits, as a Float in [0, 1).
	return float64(binary.BigEndian.Uint64(s.bytes(8))>>11) / (1 << 53)
}

type generator struct{ r *object.Random }

func (g generator) bytes(n int) []byte { return g.r.Bytes(n) }
func (g generator) float() float64     { return g.r.Float(false) }

// below draws as Random#rand(limit) does: BelowInt's bound is inclusive,
// CRuby's limited_rand, so the draw matches Ruby's for the same seed.
func (g generator) below(limit uint64) uint64 { return g.r.BelowInt(limit - 1) }

// Install defines SecureRandom and Random::Formatter. lib.Install calls it
// once.
func Install() {
	m := module
	object.DefineNative("SecureRandom", object.ClassValue(m))
	m.Consts["VERSION"] = object.String("0.4.1")
	define(func(name string, arity int, f method) {
		m.DefineSingleton(name, arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return f(ctx, secure{}, args, kw)
		})
	})
	formatter.Lexical = object.RandomClass
	object.RandomClass.Consts["Formatter"] = object.ClassValue(formatter)
	define(func(name string, arity int, f method) {
		// Random.hex draws from Kernel#rand's generator, r.hex from r's.
		formatter.Define(name, arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return f(ctx, generator{self.Random()}, args, kw)
		})
		object.RandomClass.DefineSingleton(name, arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return f(ctx, generator{ctx.Rand()}, args, kw)
		})
	})
	object.RandomClass.Include(formatter)
}

type method func(ctx object.Context, src source, args []object.Value, kw *object.Map) (object.Value, error)

// define gives a receiver every formatter method through def.
func define(def func(name string, arity int, f method)) {
	count := func(ctx object.Context, args []object.Value, def int) (int, error) {
		if len(args) > 1 {
			return 0, stdlib.ArgError(ctx, len(args), "0..1")
		}
		if len(args) == 0 || args[0].IsNil() {
			return def, nil
		}
		n, err := stdlib.WantInt(ctx, args[0], "")
		if err != nil {
			return 0, err
		}
		if n < 0 {
			return 0, ctx.Raise(object.ArgumentError, "negative string size (or size too big)")
		}
		return int(n), nil
	}
	bytesFn := func(ctx object.Context, src source, args []object.Value, kw *object.Map) (object.Value, error) {
		n, err := count(ctx, args, 16)
		if err != nil {
			return object.Value{}, err
		}
		return object.Bytes(src.bytes(n)), nil
	}
	def("random_bytes", -1, bytesFn)
	def("bytes", -1, bytesFn)
	def("gen_random", -1, bytesFn)
	def("hex", -1, func(ctx object.Context, src source, args []object.Value, kw *object.Map) (object.Value, error) {
		n, err := count(ctx, args, 16)
		if err != nil {
			return object.Value{}, err
		}
		return object.String(hex.EncodeToString(src.bytes(n))), nil
	})
	def("base64", -1, func(ctx object.Context, src source, args []object.Value, kw *object.Map) (object.Value, error) {
		n, err := count(ctx, args, 16)
		if err != nil {
			return object.Value{}, err
		}
		return object.String(base64.StdEncoding.EncodeToString(src.bytes(n))), nil
	})
	def("urlsafe_base64", -1, func(ctx object.Context, src source, args []object.Value, kw *object.Map) (object.Value, error) {
		if len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "0..2")
		}
		n, err := count(ctx, args[:min(len(args), 1)], 16)
		if err != nil {
			return object.Value{}, err
		}
		padding := len(args) == 2 && args[1].Truthy()
		s := base64.URLEncoding.EncodeToString(src.bytes(n))
		if !padding {
			s = strings.TrimRight(s, "=")
		}
		return object.String(s), nil
	})
	uuid4 := func(ctx object.Context, src source, args []object.Value, kw *object.Map) (object.Value, error) {
		b := src.bytes(16)
		b[6] = b[6]&0x0f | 0x40
		b[8] = b[8]&0x3f | 0x80
		return object.String(formatUUID(b)), nil
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
		return object.String(formatUUID(b)), nil
	})
	number := func(ctx object.Context, src source, args []object.Value, kw *object.Map) (object.Value, error) {
		if len(args) > 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "0..1")
		}
		if len(args) == 0 || args[0].IsNil() {
			return object.Float(src.float()), nil
		}
		return randomNumber(ctx, src, args[0])
	}
	def("random_number", -1, number)
	def("rand", -1, number)
	def("alphanumeric", -1, func(ctx object.Context, src source, args []object.Value, kw *object.Map) (object.Value, error) {
		n, err := count(ctx, args, 16)
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
		return object.String(choose(src, chars, n)), nil
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
		for _, d := range digits(src.below(limit), m) {
			b.WriteString(chars[d])
		}
		n -= m
	}
	if n > 0 {
		ds := digits(src.below(limit), n)
		for _, d := range ds {
			b.WriteString(chars[d])
		}
	}
	return b.String()
}

// randomNumber is random_number(limit): below an Integer, within a Range,
// or a Float below a Float; zero or negative falls back to a Float in
// [0, 1), as Ruby's does.
func randomNumber(ctx object.Context, src source, limit object.Value) (object.Value, error) {
	if r, ok := limit.AsRange(); ok {
		lo, okLo := r.Lo.AsInt()
		hi, okHi := r.Hi.AsInt()
		if okLo && okHi {
			if r.Exclusive {
				hi--
			}
			if hi < lo {
				return object.Nil, nil
			}
			return object.Int(lo + int64(src.below(uint64(hi-lo+1)))), nil
		}
		flo, okLo := r.Lo.AsNumber()
		fhi, okHi := r.Hi.AsNumber()
		if !okLo || !okHi {
			return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion into Integer")
		}
		return object.Float(flo + src.float()*(fhi-flo)), nil
	}
	if n, ok := limit.AsInt(); ok {
		if n <= 0 {
			return object.Float(src.float()), nil
		}
		return object.Int(int64(src.below(uint64(n)))), nil
	}
	if f, ok := limit.AsFloat(); ok {
		if f <= 0 || math.IsNaN(f) || math.IsInf(f, 0) {
			return object.Float(src.float()), nil
		}
		return object.Float(src.float() * f), nil
	}
	return object.Value{}, ctx.Raise(object.ArgumentError, "invalid argument - %s", stdlib.KindName(limit))
}
