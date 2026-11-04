// Package prime is Ruby's prime library, built into cameo: Prime.each,
// prime?, prime_division and int_from_prime_division, the Enumerable
// methods on Prime, and Integer#prime?, prime_division,
// Integer.from_prime_division and each_prime.
//
// It follows prime 0.1.3, the version Ruby 3.4 ships. prime? is a
// Miller-Rabin test, so a large number answers at once where Ruby's trial
// division does not finish; prime_division divides out the small factors
// and then runs Pollard's rho. Prime.each returns an Enumerator, not
// Ruby's Prime::EratosthenesGenerator.
package prime

import (
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
// against.
type bound struct {
	set bool
	f   float64
	i   *big.Int
}

func (b bound) admits(p *big.Int) bool {
	if !b.set {
		return true
	}
	if b.i != nil {
		return p.Cmp(b.i) <= 0
	}
	pf, _ := new(big.Float).SetInt(p).Float64()
	return pf <= b.f
}

// boundOf reads the ubound argument.
func boundOf(ctx object.Context, args []object.Value) (bound, error) {
	if len(args) == 0 || args[0].IsNil() {
		return bound{}, nil
	}
	if n, ok := args[0].AsBigInt(); ok {
		return bound{set: true, i: n}, nil
	}
	if n, ok := args[0].AsInt(); ok {
		return bound{set: true, i: big.NewInt(n)}, nil
	}
	if f, ok := args[0].AsFloat(); ok {
		return bound{set: true, f: f}, nil
	}
	return bound{}, ctx.Raise(object.ArgumentError, "comparison of Integer with %s failed", stdlib.KindName(args[0]))
}

// each yields the primes within the bound, in order, until yield fails.
func each(b bound, yield func(*big.Int) error) error {
	// A sieve by segments: the small primes first, then a window at a time.
	var primes []uint64
	const limit = 1 << 20
	sieve := make([]bool, limit)
	for i := 2; i < limit; i++ {
		if sieve[i] {
			continue
		}
		p := big.NewInt(int64(i))
		if !b.admits(p) {
			return nil
		}
		if err := yield(p); err != nil {
			return err
		}
		primes = append(primes, uint64(i))
		for j := i * i; j < limit; j += i {
			sieve[j] = true
		}
	}
	for lo := uint64(limit); ; lo += limit {
		window := make([]bool, limit)
		for _, p := range primes {
			if p*p >= lo+limit {
				break
			}
			start := (lo + p - 1) / p * p
			for j := start; j < lo+limit; j += p {
				window[j-lo] = true
			}
		}
		for i := uint64(0); i < limit; i++ {
			if window[i] {
				continue
			}
			p := new(big.Int).SetUint64(lo + i)
			if !b.admits(p) {
				return nil
			}
			if err := yield(p); err != nil {
				return err
			}
			primes = append(primes, lo+i)
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

// rho is one run of Pollard's rho with x^2 + c, Brent's cycle finding; a
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

// wantInteger reads an Integer argument, with prime's message for another
// value.
func wantInteger(ctx object.Context, v object.Value) (*big.Int, error) {
	if n, ok := v.AsBigInt(); ok {
		return n, nil
	}
	if n, ok := v.AsInt(); ok {
		return big.NewInt(n), nil
	}
	inspect, err := ctx.Inspect(v)
	if err != nil {
		return nil, err
	}
	return nil, ctx.Raise(object.ArgumentError, "Expected an integer, got %s", inspect)
}

// enumerate runs each with a block, or returns the Enumerator.
func enumerate(ctx object.Context, b bound, block object.Value) (object.Value, error) {
	if block.IsZero() {
		return object.EnumeratorValue(&object.Enumerator{Recv: instance, Meth: "each", Each: func(ctx object.Context, yield func(object.Value) error) error {
			return each(b, func(p *big.Int) error { return yield(intValue(p)) })
		}, Size: func(ctx object.Context) (object.Value, error) { return object.Float(math.Inf(1)), nil }}), nil
	}
	last := object.Nil
	err := each(b, func(p *big.Int) error {
		v, err := ctx.Yield(block, intValue(p))
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
		n.Mul(n, new(big.Int).Exp(p, e, nil))
	}
	return intValue(n), nil
}

// Install defines Prime and the Integer methods. lib.Install calls it
// once.
func Install() {
	c := class
	object.DefineNative("Prime", object.ClassValue(c))
	c.Consts["VERSION"] = object.String("0.1.3")
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
	c.DefineSingleton("include?", 1, primeP)
	c.Define("include?", 1, primeP)
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
