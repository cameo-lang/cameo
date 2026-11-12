package stdlib

import "github.com/cameo-lang/cameo/src/object"

// foldsOf gives the methods names of c the steps Enumerable's have: for a
// class that defines a method of its own around Enumerable's, to answer
// some calls faster. A call with a block is Enumerable's either way.
func foldsOf(c *object.Class, names ...string) {
	for _, name := range names {
		c.Methods[name].Step = object.Enumerable.Methods[name].Step
	}
}

// installFolds gives the methods of Enumerable that fold the block's
// values into one a walk (walk.go) for the calls whose block is all they
// are made with, so that a run pauses inside the block. Any other call,
// one with a pattern or with a symbol for the operation, runs the method
// as installEnumerable defined it, to its end in one step.
func installFolds() {
	e := object.Enumerable
	stepped := func(name string, plain func(call *object.Call) bool, w walk) {
		m := e.Methods[name]
		native, step := m.Native, w.stepper(name, false)
		m.Step = func(ctx object.Context, call *object.Call, st []object.Value, got object.Got) (object.Step, error) {
			if got.First && !plain(call) {
				v, err := native(ctx, call.Self, call.Args, call.Kw, call.Block)
				return object.Done(v), err
			}
			return step(ctx, call, st, got)
		}
	}
	noArgs := func(call *object.Call) bool { return len(call.Args) == 0 }
	oneArg := func(call *object.Call) bool { return len(call.Args) <= 1 }
	always := func(call *object.Call) bool { return true }
	// inject keeps the value so far, which is the first element when the
	// call names no start.
	reducing := walk{
		elems: func(ctx object.Context, call *object.Call) ([]object.Value, []object.Value, error) {
			elems, err := ToList(ctx, call.Self)
			switch {
			case err != nil:
				return nil, nil, err
			case len(call.Args) == 1:
				return elems, []object.Value{call.Args[0]}, nil
			case len(elems) == 0:
				return nil, []object.Value{{}}, nil
			}
			return elems[1:], []object.Value{elems[0]}, nil
		},
		yield: func(st []object.Value, el object.Value, i int, into []object.Value) []object.Value {
			return append(into, st[walkKept], el)
		},
		fold: func(ctx object.Context, st []object.Value, el, v object.Value) ([]object.Value, *object.Value, error) {
			st[walkKept] = v
			return st, nil, nil
		},
		done: func(ctx object.Context, call *object.Call, st []object.Value) (object.Value, error) {
			return object.OrNil(st[walkKept]), nil
		},
	}
	stepped("inject", oneArg, reducing)
	stepped("reduce", oneArg, reducing)
	// sum keeps the sum so far.
	stepped("sum", oneArg, walk{
		elems: func(ctx object.Context, call *object.Call) ([]object.Value, []object.Value, error) {
			elems, err := ToList(ctx, call.Self)
			var start object.Value = object.Int(0)
			if len(call.Args) == 1 {
				start = call.Args[0]
			}
			return elems, []object.Value{start}, err
		},
		fold: func(ctx object.Context, st []object.Value, el, v object.Value) ([]object.Value, *object.Value, error) {
			sum, err := ctx.Call(st[walkKept], "+", v)
			st[walkKept] = sum
			return st, nil, err
		},
		done: func(ctx object.Context, call *object.Call, st []object.Value) (object.Value, error) {
			return st[walkKept], nil
		},
	})
	// count and the quantifiers keep how many elements the block was
	// true for; each goes through every element, as it did.
	counting := func(want func(hits, total int) object.Value) walk {
		return walk{
			elems: func(ctx object.Context, call *object.Call) ([]object.Value, []object.Value, error) {
				elems, err := ToList(ctx, call.Self)
				return elems, []object.Value{object.Int(0)}, err
			},
			fold: func(ctx object.Context, st []object.Value, el, v object.Value) ([]object.Value, *object.Value, error) {
				if v.Truthy() {
					st[walkKept] = object.Int(st[walkKept].Int() + 1)
				}
				return st, nil, nil
			},
			done: func(ctx object.Context, call *object.Call, st []object.Value) (object.Value, error) {
				return want(int(st[walkKept].Int()), len(st[walkElems].List().Elems)), nil
			},
		}
	}
	stepped("count", noArgs, counting(func(hits, total int) object.Value { return object.Int(hits) }))
	stepped("any?", noArgs, counting(func(hits, total int) object.Value { return object.Bool(hits > 0) }))
	stepped("all?", noArgs, counting(func(hits, total int) object.Value { return object.Bool(hits == total) }))
	stepped("none?", noArgs, counting(func(hits, total int) object.Value { return object.Bool(hits == 0) }))
	stepped("one?", noArgs, counting(func(hits, total int) object.Value { return object.Bool(hits == 1) }))
	// take_while gathers elements until the block is false for one;
	// drop_while ends there with the elements from that one on.
	stepped("take_while", always, walk{
		fold: func(ctx object.Context, st []object.Value, el, v object.Value) ([]object.Value, *object.Value, error) {
			if !v.Truthy() {
				taken, _ := gathered(ctx, nil, st)
				return st, &taken, nil
			}
			return append(st, el), nil, nil
		},
		done: gathered,
	})
	stepped("drop_while", always, walk{
		indexed: true,
		fold: func(ctx object.Context, st []object.Value, el, v object.Value) ([]object.Value, *object.Value, error) {
			if !v.Truthy() {
				rest := object.NewList(append([]object.Value{}, st[walkElems].List().Elems[st[walkIndex].Int():]...)...)
				return st, &rest, nil
			}
			return st, nil, nil
		},
		done: func(ctx object.Context, call *object.Call, st []object.Value) (object.Value, error) {
			return object.NewList(), nil
		},
	})
	// group_by gathers the block's value for each element, and groups
	// the elements by it when the walk is through.
	stepped("group_by", always, walk{
		fold: func(ctx object.Context, st []object.Value, el, v object.Value) ([]object.Value, *object.Value, error) {
			return append(st, v), nil, nil
		},
		done: func(ctx object.Context, call *object.Call, st []object.Value) (object.Value, error) {
			groups := object.NewMapBuilder()
			var order []object.Value
			members := map[string][]object.Value{}
			for i, el := range st[walkElems].List().Elems {
				k := kept(st)[i]
				key := object.Key(k)
				if _, seen := members[key]; !seen {
					order = append(order, k)
				}
				members[key] = append(members[key], el)
			}
			for _, k := range order {
				groups.Set(k, object.NewList(members[object.Key(k)]...))
			}
			return object.MapValue(groups.Map()), nil
		},
	})
	// each_slice and each_cons walk the slices and the windows.
	parts := func(cut func(elems []object.Value, n int) []object.Value) walk {
		return walk{
			elems: func(ctx object.Context, call *object.Call) ([]object.Value, []object.Value, error) {
				n, err := sizeArg(ctx, call.Args)
				if err != nil {
					return nil, nil, err
				}
				elems, err := ToList(ctx, call.Self)
				return cut(elems, n), nil, err
			},
			done: func(ctx object.Context, call *object.Call, st []object.Value) (object.Value, error) {
				return object.Nil, nil
			},
		}
	}
	stepped("each_slice", always, parts(func(elems []object.Value, n int) []object.Value {
		var out []object.Value
		for i := 0; i < len(elems); i += n {
			out = append(out, object.NewList(append([]object.Value{}, elems[i:min(i+n, len(elems))]...)...))
		}
		return out
	}))
	stepped("each_cons", always, parts(func(elems []object.Value, n int) []object.Value {
		var out []object.Value
		for i := 0; i+n <= len(elems); i++ {
			out = append(out, object.NewList(append([]object.Value{}, elems[i:i+n]...)...))
		}
		return out
	}))
	// reverse_each walks the elements from the last one.
	stepped("reverse_each", always, walk{
		elems: func(ctx object.Context, call *object.Call) ([]object.Value, []object.Value, error) {
			elems, err := ToList(ctx, call.Self)
			back := append([]object.Value{}, elems...)
			reverse(back)
			return back, nil, err
		},
	})
}
