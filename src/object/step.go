package object

// A builtin that calls its block is Go code in the middle of its work
// while the block runs: how far Integer#times has come is a Go variable.
// A stepper is such a builtin written so that what it keeps between two
// calls of the block is values. The evaluator calls it for one step at a
// time and runs the block itself, so a run pauses inside the block, and
// its dump holds how far the builtin came. doc/design/ir.md, "Builtins,
// and where a run can pause", has the design.

// StateHook puts a handle through a dump. A handle is a library's value
// that changes, a StringIO, whose state is the library's own Go data: no
// envelope carries it. Save returns what a handle holds as a value the
// envelope does carry, and Restore makes a handle of that value again.
// The dump keeps a handle once, however many variables hold it, so they
// hold one handle after the resume too.
type StateHook struct {
	Save    func(ctx Context, handle Value) (Value, error)
	Restore func(ctx Context, state Value) (Value, error)
}

// Call is the call a stepper was made with.
type Call struct {
	Self  Value
	Args  []Value
	Kw    *Map
	Block Value
}

// Got is what a stepper is called with: First on the first step, and
// after that what the call it asked for came to. Value is what the block
// returned; Raised is the exception it raised instead, for a stepper
// that asked for it with Step.Rescue.
type Got struct {
	First  bool
	Value  Value
	Raised *Exception
}

// Step is what a stepper wants next: with Done set the builtin ends with
// Value; otherwise the block is called with Yield, and the stepper is
// called again with what the block returned and with State.
type Step struct {
	Done  bool
	Value Value
	Yield []Value
	// State is what the stepper keeps for its next step: values alone,
	// which a dump holds.
	State []Value
	// Rescue has an exception the block raises come back to the stepper
	// in Got.Raised, where without it the exception ends the builtin.
	Rescue bool
	// Walk and Count say what the steps after this one do, for a
	// stepper whose steps are all alike: whoever runs the stepper may
	// do them itself, without a call per step. One who does not calls
	// the stepper on, which does the same.
	Walk  *Walk
	Count *Count
}

// The places of the state of a walk.
const (
	// WalkElems is the elements the walk is over, a List.
	WalkElems = iota
	// WalkIndex is the index of the element the block was called with
	// last, an Integer.
	WalkIndex
	// WalkKept is where what the builtin gathers starts.
	WalkKept
)

// Walk is the steps of a stepper that calls its block with the elements
// of a list, one after the other: each, map, select. Its state is the
// elements, the index of the last one the block was called with, and
// what the builtin gathers. One who walks it keeps the index as a
// number of its own, and writes it into the state where the state is
// read: before a Fold that is Indexed, before Done, and for a dump.
type Walk struct {
	// Yield returns what the block is called with for the element el at
	// index i, appended to into, which is empty and may have room: one
	// who walks hands it the same room for every call, and reads the
	// arguments before the next. nil calls the block with the element.
	Yield func(state []Value, el Value, i int, into []Value) []Value
	// Fold takes the block's value v for the element el into the state
	// and returns the state; nil keeps nothing. Returning stop ends the
	// builtin with that value.
	Fold func(ctx Context, state []Value, el, v Value) (next []Value, stop *Value, err error)
	// Indexed is set when Fold reads the index in the state.
	Indexed bool
	// Done returns the builtin's value once the walk is through; nil
	// returns the receiver.
	Done func(ctx Context, c *Call, state []Value) (Value, error)
}

// Count is the steps of a stepper that calls its block with the
// Integers from the one after its state, which is the Integer the
// block was called with last, to Last, By apart: times, upto, a range
// of Integers. The builtin's value is then the receiver, unless More
// is set: the stepper is then called on from Last, as for a range
// without an end. One who counts keeps the Integer as a number of its
// own, and writes it into the state for a dump.
type Count struct {
	Last int64
	By   int64
	More bool
}

// Stepper runs a builtin up to its next call of its block. state is the
// State of the step before, nil on the first.
type Stepper func(ctx Context, c *Call, state []Value, got Got) (Step, error)

// Done is the step that ends a builtin with the value v.
func Done(v Value) Step { return Step{Done: true, Value: v} }

// Yield is the step that calls the block with args and keeps state.
func Yield(state []Value, args ...Value) Step { return Step{Yield: args, State: state} }

// YieldKept is the step of a stepper whose state is the one value it
// calls the block with: a count, the next Integer of a range. It keeps v
// in state when state is of the step before, and so makes a slice once
// for all the builtin's steps. The block is called with the state
// itself, which the next step writes: whoever runs the step reads the
// arguments before then, or copies them.
func YieldKept(state []Value, v Value) Step {
	if len(state) != 1 {
		state = make([]Value, 1)
	}
	state[0] = v
	return Step{Yield: state[:1:1], State: state}
}

// native is the stepper as a function Go calls: it runs every step and
// calls the block in between, to the builtin's end.
func (step Stepper) native() NativeFn {
	return func(ctx Context, self Value, args []Value, kw *Map, block Value) (Value, error) {
		c := &Call{Self: self, Args: args, Kw: kw, Block: block}
		var state []Value
		got := Got{First: true}
		for {
			s, err := step(ctx, c, state, got)
			if err != nil {
				return Value{}, err
			}
			if s.Done {
				return s.Value, nil
			}
			state, got = s.State, Got{}
			// A copy: the block may keep its arguments, and the next
			// step may write them (YieldKept).
			if got.Value, err = ctx.Yield(block, append([]Value(nil), s.Yield...)...); err != nil {
				exc, ok := err.(*Exception)
				if !ok || !s.Rescue {
					return Value{}, err
				}
				got.Raised = exc
			}
		}
	}
}

// DefineStep adds a native method that calls its block, as a stepper.
func (c *Class) DefineStep(name string, arity int, step Stepper) {
	c.SetMethod(&Method{Name: name, Owner: c, Native: step.native(), Step: step, Arity: arity})
}

// DefineStepFunction is DefineFunction for a function of Kernel that
// calls its block.
func DefineStepFunction(name string, arity int, step Stepper) {
	fn := step.native()
	Kernel.SetMethod(&Method{Name: name, Owner: Kernel, Native: fn, Step: step, Arity: arity, Private: true})
	Kernel.SetSingletonMethod(&Method{Name: name, Owner: Kernel, Native: fn, Step: step, Arity: arity})
}
