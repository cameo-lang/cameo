package eval

import (
	"sort"
	"unsafe"

	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/object"
)

// Env is a lexical scope of local variables. A block's scope has the scope
// it was written in as parent and sees its variables; a method body starts
// a scope with no parent.
//
// A block captures values, not variables: when a block is written, the
// variables it sees are copied into a captured scope (snapshot), and that
// is the scope its calls run in. A captured scope is never rebound in,
// so a block written inside a call of another block shares that block's
// captured scope as the parent of its own. A rebinding in the enclosing scope after
// that is invisible to the block, and a rebinding inside the block lands
// in the call's own scope, so it is gone when the call returns. Nothing
// observes a variable from outside its scope, which is what an immutable
// value needs.
//
// A scope holds its variables in slots, which its layout names: the
// scope of a call has the layout of the proc that is called, made once
// per program, so flat code reads and writes a variable by its index
// (flat.go). Any other scope has a layout of its own, which grows as
// names are bound. A name the layout of a proc does not hold is kept in a
// map. The methods that take a name find its slot in the layout; the
// ones that take it as a string intern it first, for the code that runs
// seldom: a dump, the debugger, a pattern's bindings.
//
// The first few slots live in the scope itself, so a scope of a method
// with a handful of locals costs no allocation beyond its own.
type Env struct {
	// layout names the slots, nil while nothing is bound in a scope
	// without a proc's layout; own is set when the layout is the
	// scope's own, which a new name is added to.
	layout *layout
	own    bool
	// slots holds the variables of the layout, the zero value for one
	// that is unbound.
	slots  []object.Value
	inline [envInline]object.Value
	// more holds the variables the layout of a proc does not name.
	more   map[object.Sym]object.Value
	parent *Env
	// captured marks a scope a block captured: Set never rebinds in it.
	captured bool
	// scope is, for a captured scope, the method scope it was copied
	// from, where the promises of a block's async calls belong.
	scope *Env
	// promises are the ones async started in this scope, which `await
	// :all` collects.
	promises []*Promise
}

// envInline is how many variables a scope holds without an allocation.
const envInline = 4

// layout is the names of the slots of a scope, index for index.
type layout struct {
	names []object.Sym
	// index finds the slot of a name of a layout too long to look
	// through.
	index map[object.Sym]int32
}

// layoutScan is the number of names up to which a layout is looked
// through for a name.
const layoutScan = 8

// newLayout makes the layout of names, each of them once.
func newLayout(names []object.Sym) *layout {
	l := &layout{}
	for _, name := range names {
		if l.slot(name) < 0 {
			l.add(name)
		}
	}
	return l
}

// slot is the index of name in the layout, -1 when it has none.
func (l *layout) slot(name object.Sym) int {
	if l.index != nil {
		if i, ok := l.index[name]; ok {
			return int(i)
		}
		return -1
	}
	for i, n := range l.names {
		if n == name {
			return i
		}
	}
	return -1
}

// add gives name the next slot.
func (l *layout) add(name object.Sym) {
	l.names = append(l.names, name)
	switch {
	case l.index != nil:
		l.index[name] = int32(len(l.names) - 1)
	case len(l.names) > layoutScan:
		l.index = make(map[object.Sym]int32, 2*len(l.names))
		for i, n := range l.names {
			l.index[n] = int32(i)
		}
	}
}

// NewEnv makes a scope inside parent (nil for a method body).
func NewEnv(parent *Env) *Env {
	return &Env{parent: parent}
}

// newEnvOf makes a scope inside parent with the layout l of a proc.
func newEnvOf(parent *Env, l *layout) *Env {
	e := &Env{parent: parent}
	e.lay(l)
	return e
}

// lay gives a scope nothing is bound in the layout l of a proc, nil for
// none.
func (e *Env) lay(l *layout) {
	if l == nil {
		return
	}
	e.layout = l
	if n := len(l.names); n <= envInline {
		e.slots = e.inline[:n]
	} else {
		e.slots = make([]object.Value, n)
	}
}

// lookup finds name in this scope alone.
func (e *Env) lookup(name string) (object.Value, bool) {
	// A name nothing interned is the name of no variable.
	if sym, ok := object.LookupSym(name); ok {
		return e.lookupSym(sym)
	}
	return object.Value{}, false
}

// lookupSym is lookup for a name that is interned.
func (e *Env) lookupSym(name object.Sym) (object.Value, bool) {
	if l := e.layout; l != nil {
		if i := l.slot(name); i >= 0 {
			v := e.slots[i]
			return v, !v.IsZero()
		}
	}
	if e.more != nil {
		v, ok := e.more[name]
		return v, ok
	}
	return object.Value{}, false
}

// bind binds name in this scope alone, rebinding it when it is bound.
func (e *Env) bind(name string, v object.Value) { e.bindSym(object.Intern(name), v) }

// bindSym is bind for a name that is interned.
func (e *Env) bindSym(name object.Sym, v object.Value) {
	if i := e.reserve(name); i >= 0 {
		e.slots[i] = v
		return
	}
	if e.more == nil {
		e.more = map[object.Sym]object.Value{}
	}
	e.more[name] = v
}

// reserve returns the slot of name in this scope, giving it one when
// the layout is the scope's own; -1 when the layout of a proc has none
// for it. A slot it gives holds no value yet: the name is unbound.
func (e *Env) reserve(name object.Sym) int {
	l := e.layout
	if l == nil {
		l = &layout{}
		e.layout, e.own, e.slots = l, true, e.inline[:0]
	}
	if i := l.slot(name); i >= 0 || !e.own {
		return i
	}
	l.add(name)
	e.slots = append(e.slots, object.Value{})
	return len(e.slots) - 1
}

// each calls f with every variable of this scope alone, in the order of
// its slots and then by name.
func (e *Env) each(f func(name string, v object.Value)) {
	for i, v := range e.slots {
		if !v.IsZero() {
			f(e.layout.names[i].String(), v)
		}
	}
	rest := make(map[string]object.Value, len(e.more))
	names := make([]string, 0, len(e.more))
	for sym, v := range e.more {
		name := sym.String()
		rest[name] = v
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		f(name, rest[name])
	}
}

// Get reads a variable, searching outwards, and false when it is unbound.
func (e *Env) Get(name string) (object.Value, bool) {
	if sym, ok := object.LookupSym(name); ok {
		return e.GetSym(sym)
	}
	return object.Value{}, false
}

// GetSym is Get for a name that is interned.
func (e *Env) GetSym(name object.Sym) (object.Value, bool) {
	for s := e; s != nil; s = s.parent {
		if v, ok := s.lookupSym(name); ok {
			return v, true
		}
	}
	return object.Value{}, false
}

// Set rebinds a variable where it is bound, or binds it in this scope when
// it is new or captured: a block's new variable is the block's own, and so
// is its rebinding of a captured one.
func (e *Env) Set(name string, v object.Value) { e.SetSym(object.Intern(name), v) }

// SetSym is Set for a name that is interned.
func (e *Env) SetSym(name object.Sym, v object.Value) {
	for s := e; s != nil && !s.captured; s = s.parent {
		if _, ok := s.lookupSym(name); ok {
			s.bindSym(name, v)
			return
		}
	}
	e.bindSym(name, v)
}

// snapshot copies the variables the scope sees into a captured scope, the
// values they have now, an inner binding shadowing an outer one. A block
// closes over the snapshot, not the scope. The variables of a captured
// scope the scope sits in are not copied: nothing rebinds them, so the
// snapshot has that scope as its parent and reads them there.
func (e *Env) snapshot() *Env {
	s := &Env{captured: true}
	if s.parent = e.copyLive(s); s.parent != nil && s.parent.scope != nil {
		s.scope = s.parent.scope
	} else {
		s.scope = methodScope(e)
	}
	return s
}

// capture is snapshot for a block whose proc has the layout l and reads
// the variables in the slots reads of it, itself or in the blocks
// written in it: the captured scope has that layout and holds those
// variables alone. A call of the block reads what it captured by slot.
func (e *Env) capture(l *layout, reads []int32) *Env {
	s := &Env{captured: true}
	s.lay(l)
	captured := e
	for captured != nil && !captured.captured {
		captured = captured.parent
	}
	for _, i := range reads {
		name := l.names[i]
		for live := e; live != captured; live = live.parent {
			if v, ok := live.lookupSym(name); ok {
				s.slots[i] = v
				break
			}
		}
	}
	if s.parent = captured; captured != nil && captured.scope != nil {
		s.scope = captured.scope
	} else {
		s.scope = methodScope(e)
	}
	return s
}

// copyLive binds in s the variables of this scope and of its parents up
// to the first captured one, the outermost first so that an inner binding
// wins. It returns that captured scope, nil when there is none.
func (e *Env) copyLive(s *Env) *Env {
	if e.captured {
		return e
	}
	var captured *Env
	if e.parent != nil {
		captured = e.parent.copyLive(s)
	}
	e.copyOwn(s)
	return captured
}

// copyInto binds the variables of this scope and its parents in s, the
// outermost first so that an inner binding wins.
func (e *Env) copyInto(s *Env) {
	if e.parent != nil {
		e.parent.copyInto(s)
	}
	e.copyOwn(s)
}

// copyOwn binds the variables of this scope alone in s.
func (e *Env) copyOwn(s *Env) {
	for i, v := range e.slots {
		if !v.IsZero() {
			s.bindSym(e.layout.names[i], v)
		}
	}
	for name, v := range e.more {
		s.bindSym(name, v)
	}
}

// methodScope is the scope a promise belongs to: the method body's, or the
// top level's, that the current block scope sits in. A block is part of
// the method that runs it, so a method that fans out in a loop and then
// waits sees every promise.
func methodScope(env *Env) *Env {
	for env.parent != nil {
		env = env.parent
	}
	if env.scope != nil {
		return env.scope
	}
	return env
}

// Define binds a variable in this scope, shadowing an outer one: what a
// parameter does.
func (e *Env) Define(name string, v object.Value) { e.bind(name, v) }

// DefineSym is Define for a name that is interned.
func (e *Env) DefineSym(name object.Sym, v object.Value) { e.bindSym(name, v) }

// activation is a method's frame and its scope in one allocation.
type activation struct {
	frame Frame
	env   Env
}

// Frame is one method activation: who self is, which method runs, with what
// arguments and block, and the lexical scopes constants are looked up in.
// A block runs in the frame it was written in.
type Frame struct {
	Self   object.Value
	Method *object.Method
	// Owner is the class that defines Method, where super continues from.
	Owner *object.Class
	Args  []object.Value
	Kw    *object.Map
	Block object.Value // the block passed to the method, a *Proc, or nil
	// Lexical is the chain of classes and modules enclosing the code,
	// innermost last; nil at the top level.
	Lexical []*object.Class
	// DefTarget is the class or module a def inside this frame defines on:
	// the class whose body is being evaluated, or Object at the top level.
	DefTarget *object.Class
	// File is the source the frame's code is from when it is not the
	// script's: a library written in cameo. A def in the frame gives its
	// method the file, and an error the code raises is reported with it.
	File string
	// Reopened is the class DefTarget is a copy of while a body reopens
	// it, whose fields the body may declare again with the same type.
	Reopened *object.Class
	// Building is the instance initialize is filling, while it runs, and
	// BuildingExc the exception.
	Building    *object.Instance
	BuildingExc *object.Exception
	// Lambda marks the frame of a lambda call, where return stops.
	Lambda bool
	// ended is set when the call the frame belongs to returned. A proc
	// may outlive the method it was written in; its return then has
	// nowhere to go.
	ended bool
}

// Proc is a block or lambda: a body closed over the scope and frame it was
// written in. Strict is a lambda: arguments are counted, and return returns
// from the lambda.
type Proc struct {
	// IR is the body of a proc a script wrote, a proc of a unit; nil for
	// one made from a symbol or by a builtin (see Sym and Go).
	IR     *irProc
	Env    *Env
	Frame  *Frame
	Strict bool
	// Sym is set for a block made from a symbol (&:name): it calls that
	// method on its argument, and Block is nil. With Recv set it calls the
	// method on Recv with the block's arguments instead, as the proc of a
	// hash (&hash) calls [].
	Sym  string
	Recv object.Value
	// Go is set for a block a builtin makes: it runs the function with the
	// block's arguments, and Block is nil.
	Go func(args []object.Value) (object.Value, error)
}

// writtenProc makes the block that is proc index of unit, closed over env
// and fr. The proc and its body are one allocation.
func writtenProc(unit *cameopb.Unit, index uint32, env *Env, fr *Frame) *Proc {
	w := &struct {
		Proc
		ir irProc
	}{Proc: Proc{Env: env, Frame: fr}, ir: irProc{unit: unit, proc: unit.Procs[index], index: index}}
	w.IR = &w.ir
	return &w.Proc
}

// procValue wraps a proc as a value; proc and asProc read it back.
func procValue(p *Proc) object.Value { return object.Opaque(object.KindProc, unsafe.Pointer(p)) }

func proc(v object.Value) *Proc {
	p, ok := asProc(v)
	if !ok {
		panic("eval: " + v.Kind().String() + " read as proc")
	}
	return p
}

func asProc(v object.Value) (*Proc, bool) {
	if v.Kind() != object.KindProc {
		return nil, false
	}
	return (*Proc)(v.Opaque()), true
}

// written reports whether p has a body a script wrote: a block or a
// lambda, not a proc made from a symbol or by a builtin.
func (p *Proc) written() bool { return p.IR != nil }

// line is the line a written proc starts on.
func (p *Proc) line() int { return int(p.IR.proc.GetLine()) }

// arity is the number of required positional parameters of p, negative
// when it takes a splat or optional parameters, as Proc#arity reports.
func (in *Interp) arity(p *Proc) int {
	if !p.written() {
		if p.Go != nil {
			return -1
		}
		if !p.Recv.IsZero() {
			return 1
		}
		return -2
	}
	return in.code.irParams(p.IR).arity(p.Strict)
}
