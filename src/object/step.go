package object

// A builtin that calls its block is Go code in the middle of its work
// while the block runs: how far Integer#times has come is a Go variable.
// A stepper is such a builtin written so that what it keeps between two
// calls of the block is values. The evaluator calls it for one step at a
// time and runs the block itself, so a run pauses inside the block, and
// its dump holds how far the builtin came. doc/design/ir.md, "Builtins,
// and where a run can pause", has the design.

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
}

// Stepper runs a builtin up to its next call of its block. state is the
// State of the step before, nil on the first.
type Stepper func(ctx Context, c *Call, state []Value, got Got) (Step, error)

// Done is the step that ends a builtin with the value v.
func Done(v Value) Step { return Step{Done: true, Value: v} }

// Yield is the step that calls the block with args and keeps state.
func Yield(state []Value, args ...Value) Step { return Step{Yield: args, State: state} }

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
			if got.Value, err = ctx.Yield(block, s.Yield...); err != nil {
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
