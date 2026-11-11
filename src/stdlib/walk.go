package stdlib

import "github.com/cameo-lang/cameo/src/object"

// walk is a builtin that calls its block with the elements of a list, one
// after the other, and makes its value of what the block returns: each,
// map, select, find. It is written as a stepper (object.Stepper), so a
// run pauses inside the block and the dump holds how far the walk came.
//
// What a walk keeps is its state: the elements as a List, the index of
// the element it yielded last, and then whatever the builtin gathers,
// which fold adds to.
type walk struct {
	// elems returns the elements to walk and what the builtin starts
	// with; nil walks the receiver's elements (ToList) and starts with
	// nothing.
	elems func(ctx object.Context, call *object.Call) (elems, start []object.Value, err error)
	// yield returns what the block is called with for the element el at
	// index i; nil calls it with the element.
	yield func(st []object.Value, el object.Value, i int) []object.Value
	// fold takes the block's value v for the element el into the state
	// and returns the state; nil keeps nothing. Returning stop ends the
	// walk with that value.
	fold func(ctx object.Context, st []object.Value, el, v object.Value) (next []object.Value, stop *object.Value, err error)
	// done returns the builtin's value once the walk is through; nil
	// returns the receiver.
	done func(ctx object.Context, call *object.Call, st []object.Value) (object.Value, error)
}

// The places of a walk's state.
const (
	walkElems = iota
	walkIndex
	// walkKept is where what the builtin gathers starts.
	walkKept
)

// kept is what a walk has gathered: the state after its elements and its
// index.
func kept(st []object.Value) []object.Value { return st[walkKept:] }

// stepper is the walk as a stepper. Without a block the builtin returns
// what blockless returns, an enumerator, which sized gives the
// receiver's size.
func (w walk) stepper(name string, sized bool) object.Stepper {
	return func(ctx object.Context, call *object.Call, st []object.Value, got object.Got) (object.Step, error) {
		i := 0
		if got.First {
			if call.Block.IsZero() {
				return object.Done(blockless(call.Self, name, call.Args, sized)), nil
			}
			var elems, start []object.Value
			var err error
			if w.elems != nil {
				elems, start, err = w.elems(ctx, call)
			} else {
				elems, err = ToList(ctx, call.Self)
			}
			if err != nil {
				return object.Step{}, err
			}
			st = append([]object.Value{object.NewList(elems...), object.Int(0)}, start...)
		} else {
			i = int(st[walkIndex].Int())
			if w.fold != nil {
				next, stop, err := w.fold(ctx, st, st[walkElems].List().Elems[i], got.Value)
				if err != nil {
					return object.Step{}, err
				}
				if stop != nil {
					return object.Done(*stop), nil
				}
				st = next
			}
			i++
		}
		elems := st[walkElems].List().Elems
		if i >= len(elems) {
			if w.done == nil {
				return object.Done(call.Self), nil
			}
			v, err := w.done(ctx, call, st)
			return object.Done(v), err
		}
		st[walkIndex] = object.Int(i)
		if w.yield != nil {
			return object.Yield(st, w.yield(st, elems[i], i)...), nil
		}
		return object.Yield(st, elems[i]), nil
	}
}

// gathered is the done of a walk whose value is the list of what it
// gathered.
func gathered(ctx object.Context, call *object.Call, st []object.Value) (object.Value, error) {
	return object.NewList(append([]object.Value{}, kept(st)...)...), nil
}
