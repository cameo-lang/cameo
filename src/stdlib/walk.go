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
	// index i, appended to into, which is empty; nil calls it with the
	// element.
	yield func(st []object.Value, el object.Value, i int, into []object.Value) []object.Value
	// fold takes the block's value v for the element el into the state
	// and returns the state; nil keeps nothing. Returning stop ends the
	// walk with that value.
	fold func(ctx object.Context, st []object.Value, el, v object.Value) (next []object.Value, stop *object.Value, err error)
	// done returns the builtin's value once the walk is through; nil
	// returns the receiver.
	done func(ctx object.Context, call *object.Call, st []object.Value) (object.Value, error)
	// fills is set when fold keeps one value per element: the state is
	// then made with room for them all, and gathered returns it as the
	// list without a copy.
	fills bool
	// indexed is set when fold reads the index in the state.
	indexed bool
	// packed is set when what yield returns is the elements of the one
	// value the block is called with (object.Step.Packed).
	packed bool
}

// The places of a walk's state.
const (
	walkElems = object.WalkElems
	walkIndex = object.WalkIndex
	walkKept  = object.WalkKept
)

// kept is what a walk has gathered: the state after its elements and its
// index.
func kept(st []object.Value) []object.Value { return st[walkKept:] }

// stepper is the walk as a stepper. Without a block the builtin returns
// what blockless returns, an enumerator, which sized gives the
// receiver's size.
func (w walk) stepper(name string, sized bool) object.Stepper {
	// What the steps after the first do, for an evaluator that does
	// them itself.
	steps := &object.Walk{Yield: w.yield, Fold: w.fold, Indexed: w.indexed, Packed: w.packed, Done: w.done}
	return func(ctx object.Context, call *object.Call, st []object.Value, got object.Got) (object.Step, error) {
		i := 0
		var elems []object.Value
		if got.First {
			if call.Block.IsZero() {
				return object.Done(blockless(call.Self, name, call.Args, sized)), nil
			}
			var start []object.Value
			var err error
			if w.elems != nil {
				elems, start, err = w.elems(ctx, call)
			} else {
				elems, err = ToList(ctx, call.Self)
			}
			if err != nil {
				return object.Step{}, err
			}
			room := 0
			if w.fills {
				room = len(elems)
			}
			st = make([]object.Value, walkKept, walkKept+len(start)+room)
			st[walkElems], st[walkIndex] = object.NewList(elems...), object.Int(0)
			st = append(st, start...)
		} else {
			i = int(st[walkIndex].Int())
			elems = st[walkElems].List().Elems
			if w.fold != nil {
				next, stop, err := w.fold(ctx, st, elems[i], got.Value)
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
		if i >= len(elems) {
			if w.done == nil {
				return object.Done(call.Self), nil
			}
			v, err := w.done(ctx, call, st)
			return object.Done(v), err
		}
		st[walkIndex] = object.Int(i)
		if w.yield != nil {
			return object.Step{Yield: w.yield(st, elems[i], i, nil), State: st, Packed: w.packed, Walk: steps}, nil
		}
		// The element as the list holds it is the block's arguments: the
		// evaluator reads them and keeps no slice of them.
		return object.Step{Yield: elems[i : i+1 : i+1], State: st, Walk: steps}, nil
	}
}

// gathered is the done of a walk whose value is the list of what it
// gathered.
//
// The state is the walk's own and the walk is over, so a state that is
// full is the list as it stands: no script saw it while the walk ran,
// and none can change it after. One with room left is copied, so that
// the list does not keep the room.
func gathered(ctx object.Context, call *object.Call, st []object.Value) (object.Value, error) {
	got := kept(st)
	if len(got) > 0 && cap(got) == len(got) {
		return object.NewList(got...), nil
	}
	return object.NewList(append([]object.Value{}, got...)...), nil
}
