// Package prime is Ruby's prime library, built into cameo: Prime.each,
// prime?, prime_division and int_from_prime_division, the Enumerable
// methods on Prime, and Integer#prime?, prime_division,
// Integer.from_prime_division and each_prime.
//
// It follows prime 0.1.3, the version Ruby 3.4 ships. prime? is a
// Miller-Rabin test, and prime_division divides out the small factors and
// then runs Pollard's rho. This deviates from Ruby's behaviour: Ruby
// divides by trial, which does not finish for a large number, cameo
// answers at once. Prime.each returns an Enumerator, not
// Ruby's Prime::EratosthenesGenerator.
package prime

import (
	"github.com/cameo-lang/cameo/src/str"
	"math"
	"math/big"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	class    = object.NewClass("Prime", object.ObjectClass)
	instance = object.InstanceValue(&object.Instance{Cls: class})
)

// bound is the upper bound of an each: none, or a number to compare
// against. Every prime each yields is below 2^64, so an Integer bound
// is kept as one, or as huge when it is 2^64 or more.
type bound struct {
	set  bool
	f    float64
	i    bool
	u    uint64
	huge bool
}

func (b bound) admits(p uint64) bool {
	if !b.set {
		return true
	}
	if b.i {
		return b.huge || p <= b.u
	}
	return float64(p) <= b.f
}

// boundOf reads the ubound argument.
func boundOf(ctx object.Context, args []object.Value) (bound, error) {
	if len(args) == 0 || args[0].IsNil() {
		return bound{}, nil
	}
	if n, ok := args[0].AsBigInt(); ok {
		b := bound{set: true, i: true}
		switch {
		case n.Sign() < 0:
			b.u = 0
		case n.IsUint64():
			b.u = n.Uint64()
		default:
			b.huge = true
		}
		return b, nil
	}
	if n, ok := args[0].AsInt(); ok {
		return bound{set: true, i: true, u: uint64(max(n, 0))}, nil
	}
	if f, ok := args[0].AsFloat(); ok {
		return bound{set: true, f: f}, nil
	}
	return bound{}, ctx.Raise(object.ArgumentError, "comparison of Integer with %s failed", stdlib.KindName(args[0]))
}

// each yields the primes within the bound, in order, until yield fails: a
// sieve by windows, marked by the base primes up to the square root of
// the window's end, which are sieved again, over a larger range, when a
// window needs more. What is kept is the base, not every prime yielded.
func each(b bound, yield func(uint64) error) error {
	const limit = 1 << 20
	var base []uint64
	baseTo := uint64(0)
	extend := func(need uint64) {
		if need <= baseTo {
			return
		}
		n := max(need, 2*baseTo, 1<<16)
		sieve := make([]bool, n)
		base = base[:0]
		for i := uint64(2); i < n; i++ {
			if sieve[i] {
				continue
			}
			base = append(base, i)
			for j := i * i; j < n; j += i {
				sieve[j] = true
			}
		}
		baseTo = n
	}
	for lo := uint64(0); ; lo += limit {
		hi := lo + limit
		extend(uint64(math.Sqrt(float64(hi))) + 2)
		window := make([]bool, limit)
		if lo == 0 {
			window[0], window[1] = true, true
		}
		for _, p := range base {
			if p*p >= hi {
				break
			}
			start := max(p*p, (lo+p-1)/p*p)
			for j := start; j < hi; j += p {
				window[j-lo] = true
			}
		}
		for i := uint64(0); i < limit; i++ {
			if window[i] {
				continue
			}
			if !b.admits(lo + i) {
				return nil
			}
			if err := yield(lo + i); err != nil {
				return err
			}
		}
	}
}

// isPrime is a deterministic test up to 2^64 and a Baillie-PSW and
// Miller-Rabin test above.
func isPrime(n *big.Int) bool {
	if n.Sign() <= 0 || n.Cmp(big.NewInt(2)) < 0 {
		return false
	}
	return n.ProbablyPrime(20)
}

// division is Prime.prime_division: [[prime, exponent], ...], with
// [-1, 1] first for a negative number.
func division(n *big.Int) [][2]*big.Int {
	var out [][2]*big.Int
	n = new(big.Int).Set(n)
	if n.Sign() < 0 {
		out = append(out, [2]*big.Int{big.NewInt(-1), big.NewInt(1)})
		n.Neg(n)
	}
	if n.Cmp(big.NewInt(1)) == 0 {
		return out
	}
	factors := map[string]*big.Int{}
	var order []*big.Int
	add := func(p *big.Int) {
		k := p.String()
		if _, ok := factors[k]; !ok {
			factors[k] = big.NewInt(0)
			order = append(order, new(big.Int).Set(p))
		}
		factors[k].Add(factors[k], big.NewInt(1))
	}
	// The small factors by trial division.
	for _, p := range []int64{2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37} {
		bp := big.NewInt(p)
		for new(big.Int).Mod(n, bp).Sign() == 0 {
			n.Quo(n, bp)
			add(bp)
		}
	}
	factor(n, add)
	// Sorted by the prime.
	for i := 1; i < len(order); i++ {
		for j := i; j > 0 && order[j].Cmp(order[j-1]) < 0; j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}
	for _, p := range order {
		out = append(out, [2]*big.Int{p, factors[p.String()]})
	}
	return out
}

// factor adds the prime factors of n > 1 through Pollard's rho.
func factor(n *big.Int, add func(*big.Int)) {
	one := big.NewInt(1)
	if n.Cmp(one) <= 0 {
		return
	}
	if isPrime(n) {
		add(n)
		return
	}
	// A perfect square has its root twice.
	if r := new(big.Int).Sqrt(n); new(big.Int).Mul(r, r).Cmp(n) == 0 {
		factor(r, add)
		factor(r, add)
		return
	}
	for c := int64(1); ; c++ {
		d := rho(n, c)
		if d != nil && d.Cmp(one) > 0 && d.Cmp(n) < 0 {
			factor(d, add)
			factor(new(big.Int).Quo(n, d), add)
			return
		}
	}
}

// rho is one run of Pollard's rho with x^2 + c, Floyd's cycle finding; a
// nontrivial factor, or nil when the run fails.
func rho(n *big.Int, c int64) *big.Int {
	x, y := big.NewInt(2), big.NewInt(2)
	d := big.NewInt(1)
	cc := big.NewInt(c)
	f := func(v *big.Int) *big.Int {
		r := new(big.Int).Mul(v, v)
		r.Add(r, cc)
		return r.Mod(r, n)
	}
	one := big.NewInt(1)
	for steps := 0; d.Cmp(one) == 0; steps++ {
		if steps > 1<<22 {
			return nil
		}
		x = f(x)
		y = f(f(y))
		d.GCD(nil, nil, new(big.Int).Abs(new(big.Int).Sub(x, y)), n)
	}
	if d.Cmp(n) == 0 {
		return nil
	}
	return d
}

func intValue(n *big.Int) object.Value {
	if n.IsInt64() {
		return object.Int(n.Int64())
	}
	return object.BigInt(n)
}

// primeValue is a prime each yields, as an Integer.
func primeValue(p uint64) object.Value {
	if p <= math.MaxInt64 {
		return object.Int(int64(p))
	}
	return object.BigInt(new(big.Int).SetUint64(p))
}

// wantInteger reads an Integer argument, with prime's message for another
// value.
func wantInteger(ctx object.Context, v object.Value) (*big.Int, error) {
	if n, ok := v.AsBigInt(); ok {
		return n, nil
	}
	if n, ok := v.AsInt(); ok {
		return big.NewInt(n), nil
	}
	// prime's message interpolates the value: its to_s.
	shown, err := ctx.ToS(v)
	if err != nil {
		return nil, err
	}
	return nil, ctx.Raise(object.ArgumentError, "Expected an integer, got %s", shown)
}

// enumerate runs each with a block, or returns the Enumerator.
func enumerate(ctx object.Context, b bound, block object.Value) (object.Value, error) {
	if block.IsZero() {
		return object.EnumeratorValue(&object.Enumerator{Recv: instance, Meth: "each", Each: func(ctx object.Context, yield func(object.Value) error) error {
			return each(b, func(p uint64) error { return yield(primeValue(p)) })
		}, Call: func(ctx object.Context, block object.Value) (object.Value, error) {
			return enumerate(ctx, b, block)
		}, Size: func(ctx object.Context) (object.Value, error) { return object.Float(math.Inf(1)), nil }}), nil
	}
	last := object.Nil
	err := each(b, func(p uint64) error {
		v, err := ctx.Yield(block, primeValue(p))
		last = v
		return err
	})
	return last, err
}

func divisionValue(n *big.Int) object.Value {
	pairs := division(n)
	out := make([]object.Value, len(pairs))
	for i, pe := range pairs {
		out[i] = object.NewList(intValue(pe[0]), intValue(pe[1]))
	}
	return object.NewList(out...)
}

// fromDivision is Prime.int_from_prime_division: the product.
func fromDivision(ctx object.Context, v object.Value) (object.Value, error) {
	pairs, err := stdlib.ToList(ctx, v)
	if err != nil {
		return object.Value{}, err
	}
	n := big.NewInt(1)
	for _, pair := range pairs {
		pe, err := stdlib.ToList(ctx, pair)
		if err != nil {
			return object.Value{}, err
		}
		if len(pe) != 2 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "a prime division is pairs of [prime, exponent]")
		}
		p, err := wantInteger(ctx, pe[0])
		if err != nil {
			return object.Value{}, err
		}
		e, err := wantInteger(ctx, pe[1])
		if err != nil {
			return object.Value{}, err
		}
		if e.Sign() < 0 {
			// Ruby's product is a Rational then, which cameo has not.
			return object.Value{}, ctx.Raise(object.ArgumentError, "a negative exponent, %s for %s, makes a Rational, which cameo has not", e, p)
		}
		n.Mul(n, new(big.Int).Exp(p, e, nil))
	}
	return intValue(n), nil
}

// Install defines Prime and the Integer methods. lib.Install calls it
// once.
func Install() {
	c := class
	object.DefineNative("Prime", object.ClassValue(c))
	c.Consts["VERSION"] = object.NewString("0.1.3", str.ASCII)
	c.Include(object.Enumerable)
	eachFn := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "0..2")
		}
		b, err := boundOf(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		return enumerate(ctx, b, block)
	}
	c.Define("each", -1, eachFn)
	c.DefineSingleton("each", -1, eachFn)
	c.DefineSingleton("instance", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return instance, nil
	})
	primeP := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
		}
		n, err := wantInteger(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.Bool(isPrime(n)), nil
	}
	c.DefineSingleton("prime?", 1, primeP)
	c.Define("prime?", 1, primeP)
	// include? is false for what is not an Integer, where prime? refuses it.
	includeP := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if !args[0].IsInteger() {
			return object.False, nil
		}
		return primeP(ctx, self, args, kw, block)
	}
	c.DefineSingleton("include?", 1, includeP)
	c.Define("include?", 1, includeP)
	divisionFn := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		n, err := wantInteger(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if n.Sign() == 0 {
			return object.Value{}, ctx.Raise(object.ZeroDivisionError, "ZeroDivisionError")
		}
		return divisionValue(n), nil
	}
	c.DefineSingleton("prime_division", -1, divisionFn)
	c.Define("prime_division", -1, divisionFn)
	fromFn := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
		}
		return fromDivision(ctx, args[0])
	}
	c.DefineSingleton("int_from_prime_division", 1, fromFn)
	c.Define("int_from_prime_division", 1, fromFn)
	// Prime forwards the Enumerable methods to its instance, as Ruby's
	// Prime does through its instance.
	for name, m := range object.Enumerable.Methods {
		if _, taken := c.SingletonMethods[name]; taken || m.Native == nil {
			continue
		}
		name := name
		c.DefineSingleton(name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return ctx.CallWith(instance, name, args, kw, block)
		})
	}

	object.IntegerClass.Define("prime?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, _ := wantInteger(ctx, self)
		return object.Bool(isPrime(n)), nil
	})
	object.IntegerClass.Define("prime_division", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return divisionFn(ctx, self, []object.Value{self}, kw, block)
	})
	object.IntegerClass.DefineSingleton("from_prime_division", 1, fromFn)
	object.IntegerClass.DefineSingleton("each_prime", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
		}
		b, err := boundOf(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		return enumerate(ctx, b, block)
	})
}
