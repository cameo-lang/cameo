package eval

import "github.com/cameo-lang/cameo/src/object"

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

// lay gives env, the scope of the call m is about to run, the layout l.
// A scope that is the machine's own takes the slots the machine keeps
// when it needs more than it holds itself.
func (m *machine) lay(env *Env, l *layout) {
	if n := len(l.names); env == &m.own.env && n > envInline && n <= cap(m.spare) {
		env.layout, env.slots = l, m.spare[:n]
		return
	}
	env.lay(l)
}

// release keeps m, whose activation ended, for the next call. The values
// and the cursors it held are cleared, so that it keeps nothing alive.
func (in *Interp) release(m *machine) {
	if m.held {
		// A stepper runs its block's next call on it (stepping.act).
		return
	}
	if m.kind == actStep || len(in.free) >= maxFree || cap(m.stack) > maxKeptStack || cap(m.cursors) > maxKeptCursors {
		return
	}
	// The slots of a scope the machine held itself are the next one's,
	// when they were more than the scope holds inline.
	if s := m.own.env.slots; cap(s) > cap(m.spare) && cap(s) <= maxKeptStack {
		m.spare = s
	}
	// The frame and the scope are cleared when a call used them: they
	// are most of a machine, and most calls of a block use neither or
	// the scope alone.
	if m.own.frame.Method != nil || m.own.frame.Lambda {
		m.own.frame = Frame{}
	}
	if m.own.env.layout != nil || m.own.env.parent != nil {
		m.own.env = Env{}
	}
	clear(m.stack[:cap(m.stack)])
	clear(m.cursors[:cap(m.cursors)])
	clear(m.tries[:cap(m.tries)])
	clear(m.spare[:cap(m.spare)])
	// Each field is set by itself: the machine as a whole is several
	// hundred bytes, which a call need not write. A test holds a
	// released machine against a new one, field by field.
	m.stack, m.cursors, m.bases, m.tries, m.spare = m.stack[:0], m.cursors[:0], m.bases[:0], m.tries[:0], m.spare[:0]
	m.unit, m.syms, m.env, m.fr = nil, nil, nil, nil
	m.flat, m.pc, m.flatBody, m.again = nil, 0, false, false
	m.kind, m.proc, m.site, m.debugged, m.stepper = actList, nil, nil, false, nil
	m.result, m.wait, m.inGo, m.passing, m.row, m.sofar = object.Value{}, nil, 0, false, 0, nil
	in.free = append(in.free, m)
}
