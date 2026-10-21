package stdlib

import (
	"math"
	"math/big"
	"math/rand/v2"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
)

// installKernel defines puts, print, p, warn, raise and format on every
// value, as Ruby's Kernel does.
func installKernel() {
	o := object.ObjectClass
	def(o, "rand", -1, kernelRand)
	// srand reseeds the generator, with a random seed when given none, and
	// returns the previous seed.
	def(o, "srand", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 1 {
			return object.Value{}, argError(ctx, len(args), "0..1")
		}
		seed := rand.Uint64() >> 1
		if len(args) == 1 {
			s, err := wantInt(ctx, args[0], "")
			if err != nil {
				return object.Value{}, err
			}
			seed = uint64(s)
		}
		return object.Int(int64(ctx.Srand(seed))), nil
	})
	def(o, "puts", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Nil, puts(ctx, false, args)
	})
	def(o, "print", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for _, a := range args {
			s, err := ctx.ToS(a)
			if err != nil {
				return object.Value{}, err
			}
			if err := ctx.Write(false, s); err != nil {
				return object.Value{}, err
			}
		}
		return object.Nil, nil
	})
	def(o, "p", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for _, a := range args {
			s, err := ctx.Inspect(a)
			if err != nil {
				return object.Value{}, err
			}
			if err := ctx.Write(false, s+"\n"); err != nil {
				return object.Value{}, err
			}
		}
		switch len(args) {
		case 0:
			return object.Nil, nil
		case 1:
			return args[0], nil
		}
		return object.NewList(args...), nil
	})
	def(o, "pp", -1, o.Methods["p"].Native)
	def(o, "warn", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Nil, puts(ctx, true, args)
	})
	def(o, "raise", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Value{}, raise(ctx, args)
	})
	def(o, "fail", -1, o.Methods["raise"].Native)
	def(o, "format", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 {
			return object.Value{}, argError(ctx, 0, "1+")
		}
		f, err := wantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		s, err := format(ctx, f, args[1:])
		return object.String(s), err
	})
	def(o, "sprintf", -1, o.Methods["format"].Native)
	def(o, "loop", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "loop"); err != nil {
			return object.Value{}, err
		}
		for {
			if _, err := ctx.Yield(block); err != nil {
				if exc, ok := err.(*object.Exception); ok && exc.Cls.IsA(object.StopIteration) {
					return object.Nil, nil
				}
				return object.Value{}, err
			}
		}
	})
	def(o, "block_given?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		// The evaluator handles this for user methods; a native call site has
		// no enclosing method.
		return object.False, nil
	})
	def(o, "Integer", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		switch args[0].Kind() {
		case object.KindInt, object.KindBig:
			return args[0], nil
		case object.KindFloat:
			return floatToInteger(ctx, args[0].Float())
		case object.KindString, object.KindBytes, object.KindASCII:
			v := args[0].Str()
			i, ok := parseInteger(strings.TrimSpace(v))
			if !ok {
				return object.Value{}, ctx.Raise(object.ArgumentError, "invalid value for Integer(): %s", object.QuoteString(v))
			}
			return i, nil
		}
		return object.Value{}, ctx.Raise(object.TypeError, "can't convert %s into Integer", kindName(args[0]))
	})
	def(o, "Float", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		switch args[0].Kind() {
		case object.KindInt, object.KindBig:
			f, _ := args[0].AsNumber()
			return object.Float(f), nil
		case object.KindFloat:
			v := args[0].Float()
			return object.Float(v), nil
		case object.KindString, object.KindBytes, object.KindASCII:
			v := args[0].Str()
			f, ok := parseFloat(strings.TrimSpace(string(v)))
			if !ok {
				return object.Value{}, ctx.Raise(object.ArgumentError, "invalid value for Float(): %s", object.QuoteString(string(v)))
			}
			return object.Float(f), nil
		}
		return object.Value{}, ctx.Raise(object.TypeError, "can't convert %s into Float", kindName(args[0]))
	})
	def(o, "String", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := ctx.ToS(args[0])
		return object.String(s), err
	})
	def(o, "Array", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if args[0].IsNil() {
			return object.NewList(), nil
		}
		elems, err := toList(ctx, args[0])
		if err != nil {
			return object.NewList(args[0]), nil
		}
		return object.NewList(append([]object.Value{}, elems...)...), nil
	})
}

// puts writes each argument on its own line, a list element by element,
// and a bare newline for no arguments.
func puts(ctx object.Context, stderr bool, args []object.Value) error {
	if len(args) == 0 {
		return ctx.Write(stderr, "\n")
	}
	for _, a := range args {
		if l, ok := a.AsList(); ok {
			if len(l.Elems) == 0 {
				if err := ctx.Write(stderr, "\n"); err != nil {
					return err
				}
			}
			if err := puts(ctx, stderr, l.Elems); err != nil {
				return err
			}
			continue
		}
		s, err := ctx.ToS(a)
		if err != nil {
			return err
		}
		if !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		if err := ctx.Write(stderr, s); err != nil {
			return err
		}
	}
	return nil
}

// raise builds the exception for raise's arguments: a message makes a
// RuntimeError, a class is instantiated with an optional message, an
// exception is raised as it is.
func raise(ctx object.Context, args []object.Value) error {
	if len(args) == 0 {
		return &object.Exception{Cls: object.RuntimeError, Message: "unhandled exception"}
	}
	switch args[0].Kind() {
	case object.KindString, object.KindBytes, object.KindASCII:
		v := args[0].Str()
		if len(args) > 1 {
			return argError(ctx, len(args), "1")
		}
		return &object.Exception{Cls: object.RuntimeError, Message: string(v)}
	case object.KindException:
		v := args[0].Exception()
		if len(args) > 1 {
			if msg, err := wantString(ctx, args[1]); err == nil {
				return &object.Exception{Cls: v.Cls, Message: msg, Attrs: v.Attrs}
			}
		}
		return v
	case object.KindClass:
		v := args[0].Cls()
		if !v.IsA(object.ExceptionClass) {
			return ctx.Raise(object.TypeError, "exception class/object expected")
		}
		msg := v.QualifiedName()
		if len(args) > 1 {
			s, err := ctx.ToS(args[1])
			if err != nil {
				return err
			}
			msg = s
		}
		return &object.Exception{Cls: v, Message: msg}
	}
	return ctx.Raise(object.TypeError, "exception class/object expected")
}

// kernelRand is Kernel#rand: a float in [0, 1) without an argument (or
// with nil or 0), an integer below max.to_i.abs for a number, and a number
// within a range, or nil for an empty one.
func kernelRand(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if len(args) > 1 {
		return object.Value{}, argError(ctx, len(args), "0..1")
	}
	r := ctx.Rand()
	if len(args) == 0 || args[0].IsNil() {
		return object.Float(r.Float64()), nil
	}
	switch a := args[0]; a.Kind() {
	case object.KindBig:
		return object.BigInt(randBelow(r, new(big.Int).Abs(a.Big()))), nil
	case object.KindInt, object.KindFloat:
		f, _ := a.AsNumber()
		n := int64(math.Abs(f))
		if n == 0 {
			return object.Float(r.Float64()), nil
		}
		return object.Int(r.Int64N(n)), nil
	case object.KindRange:
		rg := a.Range()
		lo, hi := object.OrNil(rg.Lo), object.OrNil(rg.Hi)
		if li, ok := lo.AsInt(); ok {
			if hiI, ok := hi.AsInt(); ok {
				if rg.Exclusive {
					hiI--
				}
				if hiI < li {
					return object.Nil, nil
				}
				return object.Int(li + r.Int64N(hiI-li+1)), nil
			}
		}
		lf, ok1 := lo.AsNumber()
		hf, ok2 := hi.AsNumber()
		if ok1 && ok2 {
			if hf < lf || (rg.Exclusive && hf == lf) {
				return object.Nil, nil
			}
			return object.Float(lf + r.Float64()*(hf-lf)), nil
		}
	}
	return object.Value{}, ctx.Raise(object.ArgumentError, "invalid argument - %s", kindName(args[0]))
}

// randBelow is a uniform integer in [0, n), for an n beyond 64 bits: random
// words with the bits of n, drawn again while they reach n.
func randBelow(r *rand.Rand, n *big.Int) *big.Int {
	words := (n.BitLen() + 63) / 64
	extra := uint(words*64 - n.BitLen())
	for {
		v := new(big.Int)
		for i := 0; i < words; i++ {
			v.Lsh(v, 64).Or(v, new(big.Int).SetUint64(r.Uint64()))
		}
		if v.Rsh(v, extra).Cmp(n) < 0 {
			return v
		}
	}
}
