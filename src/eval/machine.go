package eval

// The machines of activations that ended are kept and run the next
// calls, with the value stack and the cursors they grew: a call then
// allocates neither. Nothing holds a machine once its activation ended.
// The frame and the scope of a call are not reused, since a block or a
// lambda may keep them.

const (
	// maxFree is how many machines an interpreter keeps for reuse.
	maxFree = 256
	// maxKeptStack and maxKeptCursors are the capacities of a value stack
	// and of the cursors above which a machine is not kept.
	maxKeptStack   = 64
	maxKeptCursors = 16
)

// machine returns a machine for a new activation, with nothing set but
// its interpreter.
func (in *Interp) machine() *machine {
	if n := len(in.free); n > 0 {
		m := in.free[n-1]
		in.free = in.free[:n-1]
		return m
	}
	return &machine{in: in}
}

// release keeps m, whose activation ended, for the next call. The values
// and the cursors it held are cleared, so that it keeps nothing alive.
func (in *Interp) release(m *machine) {
	if m.kind == actStep || len(in.free) >= maxFree || cap(m.stack) > maxKeptStack || cap(m.cursors) > maxKeptCursors {
		return
	}
	stack, cursors := m.stack[:cap(m.stack)], m.cursors[:cap(m.cursors)]
	clear(stack)
	clear(cursors)
	*m = machine{in: in, stack: stack[:0], cursors: cursors[:0]}
	in.free = append(in.free, m)
}
