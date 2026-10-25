package stdlib

import (
	"iter"
	"math"
	"math/big"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
)

// An Enumerator produces its elements on demand (object.Enumerator). An
// eager one, from a blockless cycle or then, answers Enumerable's methods
// by listing its elements, except those that can stop early: first, take,
// take_while, find and include?, which also work on an endless one. A lazy
// one, from lazy, answers map, select, take and the rest with another lazy
// enumerator, and does no work until first, to_a, force or each asks.

// each is how an enumerator's elements are produced.
type each = func(ctx object.Context, yield func(object.Value) error) error

// stop is what an early consumer returns from yield when it has had enough.
// Each consumer makes its own and recognises only that one, so a stop never
// ends an enumeration nested inside the one it was meant for.
type stop struct{}

func (*stop) Error() string { return "stop" }

// until runs e, handing each element to f until f says it is done, and
// reports whether it was.
func until(ctx object.Context, e each, f func(object.Value) (done bool, err error)) (bool, error) {
	mine := &stop{}
	err := e(ctx, func(v object.Value) error {
		done, err := f(v)
		if err == nil && done {
			return mine
		}
		return err
	})
	if err == mine {
		return true, nil
	}
	return false, err
}

// eachOf returns how to produce v's elements without listing them first:
// an enumerator's own, a range's counted from its start, endless ones too,
// and for anything else the elements toList gives.
func eachOf(ctx object.Context, v object.Value) (each, error) {
	if e, ok := v.AsEnumerator(); ok {
		return e.Each, nil
	}
	if r, ok := v.AsRange(); ok {
		if seq, sym, _, ok := rangeStrings(r); ok {
			return func(ctx object.Context, yield func(object.Value) error) error {
				for s := range seq {
					if err := yield(strValue(s, sym)); err != nil {
						return err
					}
				}
				return nil
			}, nil
		}
		// An integer range to a float end counts up to it; to Infinity it
		// is endless.
		if f, ok := r.Hi.AsFloat(); ok && r.Lo.IsInteger() && !math.IsInf(f, 1) && !math.IsNaN(f) {
			end := math.Floor(f)
			if r.Exclusive && end == f {
				end--
			}
			if math.IsInf(f, -1) {
				return func(object.Context, func(object.Value) error) error { return nil }, nil
			}
			lo, hi := r.Lo, object.FloatToInteger(end)
			return func(ctx object.Context, yield func(object.Value) error) error {
				return eachInteger(lo, hi, 1, yield)
			}, nil
		}
		if f, _ := r.Hi.AsFloat(); r.Lo.IsInteger() && (r.Hi.IsZero() || math.IsInf(f, 1)) {
			lo := r.Lo
			return func(ctx object.Context, yield func(object.Value) error) error {
				for i := lo; ; {
					if err := yield(i); err != nil {
						return err
					}
					if x, ok := i.AsInt(); ok && x < math.MaxInt64 {
						i = object.Int(x + 1)
					} else {
						i = object.BigInt(new(big.Int).Add(bigOf(i), big.NewInt(1)))
					}
				}
			}, nil
		}
		if lo, hi, ok := intEnds(r); ok {
			return func(ctx object.Context, yield func(object.Value) error) error {
				return eachInteger(lo, hi, 1, yield)
			}, nil
		}
	}
	elems, err := toList(ctx, v)
	if err != nil {
		return nil, err
	}
	return func(ctx object.Context, yield func(object.Value) error) error {
		for _, el := range elems {
			if err := yield(el); err != nil {
				return err
			}
		}
		return nil
	}, nil
}

// enumerator makes an enumerator value.
func enumerator(lazy bool, recv object.Value, meth string, args []object.Value, e each) object.Value {
	return object.EnumeratorValue(&object.Enumerator{Lazy: lazy, Each: e, Recv: recv, Meth: meth, Args: args})
}

// enumElems lists an enumerator's elements; an endless one never returns.
func enumElems(ctx object.Context, e *object.Enumerator) ([]object.Value, error) {
	out := []object.Value{}
	err := e.Each(ctx, func(v object.Value) error {
		out = append(out, v)
		return nil
	})
	return out, err
}

// firstOf lists up to n of e's first elements, producing no more.
func firstOf(ctx object.Context, e each, n int64) ([]object.Value, error) {
	out := []object.Value{}
	if n <= 0 {
		return out, nil
	}
	_, err := until(ctx, e, func(v object.Value) (bool, error) {
		out = append(out, v)
		return int64(len(out)) >= n, nil
	})
	return out, err
}

// EnumeratorInspect is Enumerator#inspect: #<Enumerator: recv:meth(args)>,
// and for a lazy one #<Enumerator::Lazy: ...>, the receiver being the
// enumerator a lazy method was called on.
func EnumeratorInspect(ctx object.Context, e *object.Enumerator) (string, error) {
	var b strings.Builder
	b.WriteString("#<")
	b.WriteString(object.EnumeratorValue(e).Class().QualifiedName())
	b.WriteString(": ")
	recv, err := ctx.Inspect(e.Recv)
	if err != nil {
		return "", err
	}
	b.WriteString(recv)
	if e.Meth != "" {
		b.WriteString(":")
		b.WriteString(e.Meth)
		if len(e.Args) > 0 {
			b.WriteString("(")
			for i, a := range e.Args {
				if i > 0 {
					b.WriteString(", ")
				}
				s, err := ctx.Inspect(a)
				if err != nil {
					return "", err
				}
				b.WriteString(s)
			}
			b.WriteString(")")
		}
	}
	b.WriteString(">")
	return b.String(), nil
}

func installEnumerator() {
	c := object.EnumeratorClass
	en := func(self object.Value) *object.Enumerator { return self.Enumerator() }
	def(c, "each", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return self, nil
		}
		if call := en(self).Call; call != nil {
			return call(ctx, block)
		}
		err := en(self).Each(ctx, func(v object.Value) error {
			_, err := ctx.Yield(block, v)
			return err
		})
		if err != nil {
			return object.Value{}, err
		}
		return self, nil
	})
	toA := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		elems, err := enumElems(ctx, en(self))
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(elems...), nil
	}
	def(c, "to_a", 0, toA)
	def(c, "entries", 0, toA)
	def(c, "force", 0, toA)
	def(c, "first", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 {
			elems, err := firstOf(ctx, en(self).Each, 1)
			if err != nil || len(elems) == 0 {
				return object.Nil, err
			}
			return elems[0], nil
		}
		n, err := wantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if n < 0 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "attempt to take negative size")
		}
		elems, err := firstOf(ctx, en(self).Each, n)
		return object.NewList(elems...), err
	})
	def(c, "take", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, err := wantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if n < 0 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "attempt to take negative size")
		}
		elems, err := firstOf(ctx, en(self).Each, n)
		return object.NewList(elems...), err
	})
	def(c, "take_while", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "take_while"); err != nil {
			return object.Value{}, err
		}
		out := []object.Value{}
		_, err := until(ctx, en(self).Each, func(v object.Value) (bool, error) {
			r, err := ctx.Yield(block, v)
			if err != nil || !r.Truthy() {
				return true, err
			}
			out = append(out, v)
			return false, nil
		})
		return object.NewList(out...), err
	})
	find := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "find"); err != nil {
			return object.Value{}, err
		}
		found := object.Nil
		_, err := until(ctx, en(self).Each, func(v object.Value) (bool, error) {
			r, err := ctx.Yield(block, v)
			if err == nil && r.Truthy() {
				found = v
				return true, nil
			}
			return false, err
		})
		return found, err
	}
	def(c, "find", 0, find)
	def(c, "detect", 0, find)
	include := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		found, err := until(ctx, en(self).Each, func(v object.Value) (bool, error) {
			return ctx.Equal(v, args[0])
		})
		return object.Bool(found), err
	}
	def(c, "include?", 1, include)
	def(c, "member?", 1, include)
	def(c, "lazy", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if en(self).Lazy {
			return self, nil
		}
		return enumerator(true, self, "", nil, en(self).Each), nil
	})
	def(c, "size", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Nil, nil
	})
	inspect := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := EnumeratorInspect(ctx, en(self))
		return object.String(s), err
	}
	def(c, "inspect", 0, inspect)
	def(c, "to_s", 0, inspect)
	installLazy()
}

// installLazy defines Enumerator::Lazy's methods, each a new lazy
// enumerator over the one it is called on.
func installLazy() {
	c := object.LazyClass
	// stage defines a lazy method from what it does with each element,
	// given the next stage's yield; start runs for every enumeration, so
	// a stage that counts starts from zero each time.
	stage := func(name string, block bool, start func(ctx object.Context, args []object.Value, block object.Value, yield func(object.Value) error) func(object.Value) error) {
		def(c, name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, blk object.Value) (object.Value, error) {
			if block {
				if err := needBlock(ctx, blk, name); err != nil {
					return object.Value{}, err
				}
			}
			parent := self.Enumerator().Each
			return enumerator(true, self, name, args, func(ctx object.Context, yield func(object.Value) error) error {
				return parent(ctx, start(ctx, args, blk, yield))
			}), nil
		})
	}
	mapping := func(ctx object.Context, args []object.Value, block object.Value, yield func(object.Value) error) func(object.Value) error {
		return func(v object.Value) error {
			r, err := ctx.Yield(block, v)
			if err != nil {
				return err
			}
			return yield(r)
		}
	}
	stage("map", true, mapping)
	stage("collect", true, mapping)
	filter := func(keep bool) func(ctx object.Context, args []object.Value, block object.Value, yield func(object.Value) error) func(object.Value) error {
		return func(ctx object.Context, args []object.Value, block object.Value, yield func(object.Value) error) func(object.Value) error {
			return func(v object.Value) error {
				r, err := ctx.Yield(block, v)
				if err != nil || r.Truthy() != keep {
					return err
				}
				return yield(v)
			}
		}
	}
	stage("select", true, filter(true))
	stage("filter", true, filter(true))
	stage("reject", true, filter(false))
	stage("filter_map", true, func(ctx object.Context, args []object.Value, block object.Value, yield func(object.Value) error) func(object.Value) error {
		return func(v object.Value) error {
			r, err := ctx.Yield(block, v)
			if err != nil || !r.Truthy() {
				return err
			}
			return yield(r)
		}
	})
	// flat_map yields the elements of an array or an enumerator the block
	// returns, and anything else as it is.
	flatMap := func(ctx object.Context, args []object.Value, block object.Value, yield func(object.Value) error) func(object.Value) error {
		return func(v object.Value) error {
			r, err := ctx.Yield(block, v)
			if err != nil {
				return err
			}
			if l, ok := r.AsList(); ok {
				for _, el := range l.Elems {
					if err := yield(el); err != nil {
						return err
					}
				}
				return nil
			}
			if e, ok := r.AsEnumerator(); ok {
				return e.Each(ctx, yield)
			}
			return yield(r)
		}
	}
	stage("flat_map", true, flatMap)
	stage("collect_concat", true, flatMap)
	stage("compact", false, func(ctx object.Context, args []object.Value, block object.Value, yield func(object.Value) error) func(object.Value) error {
		return func(v object.Value) error {
			if v.IsNil() {
				return nil
			}
			return yield(v)
		}
	})
	stage("uniq", false, func(ctx object.Context, args []object.Value, block object.Value, yield func(object.Value) error) func(object.Value) error {
		seen := map[string]bool{}
		return func(v object.Value) error {
			k := v
			if !block.IsZero() {
				var err error
				if k, err = ctx.Yield(block, v); err != nil {
					return err
				}
			}
			if key := object.Key(k); !seen[key] {
				seen[key] = true
				return yield(v)
			}
			return nil
		}
	})
	stage("drop_while", true, func(ctx object.Context, args []object.Value, block object.Value, yield func(object.Value) error) func(object.Value) error {
		dropping := true
		return func(v object.Value) error {
			if dropping {
				r, err := ctx.Yield(block, v)
				if err != nil || r.Truthy() {
					return err
				}
				dropping = false
			}
			return yield(v)
		}
	})
	// with_index pairs each element with its index from an offset, or,
	// given a block, calls it with both and passes the element on.
	withIndex := func(ctx object.Context, args []object.Value, block object.Value, yield func(object.Value) error) func(object.Value) error {
		i := object.Int(0)
		if len(args) > 0 && args[0].IsInteger() {
			i = args[0]
		}
		return func(v object.Value) error {
			at := i
			i = addIntegers(i, object.Int(1))
			if block.IsZero() {
				return yield(object.NewList(v, at))
			}
			if _, err := ctx.Yield(block, v, at); err != nil {
				return err
			}
			return yield(v)
		}
	}
	stage("with_index", false, withIndex)
	stage("each_with_index", false, withIndex)
	grep := func(keep bool) func(ctx object.Context, args []object.Value, block object.Value, yield func(object.Value) error) func(object.Value) error {
		return func(ctx object.Context, args []object.Value, block object.Value, yield func(object.Value) error) func(object.Value) error {
			return func(v object.Value) error {
				r, err := ctx.Call(args[0], "===", v)
				if err != nil || r.Truthy() != keep {
					return err
				}
				if !block.IsZero() {
					if v, err = ctx.Yield(block, v); err != nil {
						return err
					}
				}
				return yield(v)
			}
		}
	}
	stage("grep", false, grep(true))
	stage("grep_v", false, grep(false))

	// count reads take and drop's argument, a size that is not negative.
	count := func(ctx object.Context, args []object.Value, name string) (int64, error) {
		if len(args) != 1 {
			return 0, argError(ctx, len(args), "1")
		}
		n, err := wantInt(ctx, args[0], "")
		if err == nil && n < 0 {
			err = ctx.Raise(object.ArgumentError, "attempt to %s negative size", name)
		}
		return n, err
	}
	// take stops its source after the n-th element, so it ends an endless
	// one; take(0) never starts it.
	def(c, "take", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, err := count(ctx, args, "take")
		if err != nil {
			return object.Value{}, err
		}
		parent := self.Enumerator().Each
		return enumerator(true, self, "take", args, func(ctx object.Context, yield func(object.Value) error) error {
			if n == 0 {
				return nil
			}
			taken := int64(0)
			_, err := until(ctx, parent, func(v object.Value) (bool, error) {
				taken++
				return taken >= n, yield(v)
			})
			return err
		}), nil
	})
	def(c, "take_while", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "take_while"); err != nil {
			return object.Value{}, err
		}
		parent := self.Enumerator().Each
		return enumerator(true, self, "take_while", nil, func(ctx object.Context, yield func(object.Value) error) error {
			_, err := until(ctx, parent, func(v object.Value) (bool, error) {
				r, err := ctx.Yield(block, v)
				if err != nil || !r.Truthy() {
					return true, err
				}
				return false, yield(v)
			})
			return err
		}), nil
	})
	def(c, "drop", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, err := count(ctx, args, "drop")
		if err != nil {
			return object.Value{}, err
		}
		parent := self.Enumerator().Each
		return enumerator(true, self, "drop", args, func(ctx object.Context, yield func(object.Value) error) error {
			seen := int64(0)
			return parent(ctx, func(v object.Value) error {
				if seen < n {
					seen++
					return nil
				}
				return yield(v)
			})
		}), nil
	})
	// zip pairs each element with the elements at the same place in its
	// arguments, nil past their end; an argument that is not an array is
	// enumerated alongside, so it may be endless.
	def(c, "zip", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		sources := make([]each, len(args))
		for i, a := range args {
			if _, ok := a.AsList(); ok {
				continue
			}
			e, err := eachOf(ctx, a)
			if err != nil {
				return object.Value{}, err
			}
			sources[i] = e
		}
		parent := self.Enumerator().Each
		zipped := enumerator(true, self, "zip", args, func(ctx object.Context, yield func(object.Value) error) error {
			next := make([]func() (object.Value, bool), len(args))
			for i, a := range args {
				if l, ok := a.AsList(); ok {
					at := 0
					next[i] = func() (object.Value, bool) {
						if at >= l.Len() {
							return object.Value{}, false
						}
						at++
						return l.Elems[at-1], true
					}
					continue
				}
				pull, end := iter.Pull(func(y func(object.Value) bool) {
					mine := &stop{}
					_ = sources[i](ctx, func(v object.Value) error {
						if !y(v) {
							return mine
						}
						return nil
					})
				})
				defer end()
				next[i] = pull
			}
			return parent(ctx, func(v object.Value) error {
				row := []object.Value{v}
				for _, n := range next {
					el, ok := n()
					if !ok {
						el = object.Nil
					}
					row = append(row, el)
				}
				return yield(object.NewList(row...))
			})
		})
		if block.IsZero() {
			return zipped, nil
		}
		// With a block zip is eager, as Ruby's is: it calls the block with
		// each row and returns nil.
		err := zipped.Enumerator().Each(ctx, func(v object.Value) error {
			_, err := ctx.Yield(block, v)
			return err
		})
		return object.Nil, err
	})
	def(c, "lazy", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def(c, "eager", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return enumerator(false, self, "each", nil, self.Enumerator().Each), nil
	})
}
