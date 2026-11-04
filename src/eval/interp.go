// Package eval runs cameo programs: it compiles the syntax package's AST
// into closures, one per node, and runs them over the object package's
// values. The design is in doc/design/cameo.md.
package eval

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
	"github.com/cameo-lang/cameo/src/syntax"
)

// Interp is one interpreter: a top-level scope, the standard output and
// error streams, and the constants the script defines. Workers started by
// async get interpreters of their own.
type Interp struct {
	// IgnoreMutationErrors makes a call of one of Ruby's mutators return
	// its result on a copy instead of raising; see mutate.go.
	IgnoreMutationErrors bool
	// DNS is the resolver resolv looks names up with; nil leaves the
	// program without lookups, and one raises Resolv::ResolvError.
	DNS *net.Resolver
	// HTTP is the client net/http makes requests with; nil leaves the
	// program without network access, and a request raises SocketError.
	HTTP           *http.Client
	stdout, stderr io.Writer
	outMu          *sync.Mutex
	file           string
	// mainClass is the class of the top-level self: top-level defs are its
	// methods, so one interpreter's definitions never reach another's.
	mainClass *object.Class
	main      *object.Instance
	top       *Env
	// consts are the script's top-level constants, looked up after the
	// lexical scopes and before the builtin classes.
	consts map[string]object.Value
	proto  *protogen.Registry
	// promises is every promise of the run, shared with its workers, so the
	// run can wait for the calls async started and report the failures
	// nobody awaited.
	promises *promises
	// cameoTop is set once the script included Cameo at the top level,
	// which brings async and the rest of Cameo to every method.
	cameoTop bool
	// snap is the top level as the workers async starts see it, shared by
	// every worker started since the top level last changed; nil when it
	// changed. A worker's own top level is its snapshot's, which it never
	// changes.
	snap *topLevel
	// code is the compiled program, shared with the workers.
	code *code
	// rng is this interpreter's generator; see Rand.
	rng *object.Random
	// loaded is the libraries require has loaded, which a second require
	// of answers false.
	loaded map[string]bool
	// exits are the blocks at_exit registered, shared with the workers; Run
	// runs them when the script ends.
	exits *exitHandlers
	// states are what libraries keep for the run, by name; see State.
	states *libStates
	// stop ends the run, and is shared with the workers; see Stop.
	stop *stopper
	// catches are the tags of the catch blocks running, innermost last.
	catches []*catchTag
	// rescuing is the exception the innermost running rescue clause
	// rescued, Ruby's $!; nil outside one.
	rescuing *object.Exception
	// last is the value of the last top-level expression Run evaluated.
	last object.Value
	// interpreted counts the calls of compiled methods running on the
	// interpreter, which their own calls do too; see reinvoke.
	interpreted int
	// pause is set by Pause; paused once Run returned ErrPaused, with
	// next the index of the top-level statement the run continues with
	// and nextLine its line. from is where Run starts: the pause point
	// of a resumed run, 0 otherwise. See pause.go.
	pause    atomic.Bool
	paused   bool
	next     int
	nextLine int
	from     int
	// dbg is the debugger, once Debug switched it on; dbgTop the top-level
	// frame it shows, while Run runs. console, consoleOut and source are
	// what a debugger the script switches on with require "debug" talks
	// on, and lists; see SetConsole.
	dbg        *Debugger
	dbgTop     *dbgFrame
	console    io.Reader
	consoleOut io.Writer
	source     string
}

// Last returns the value of the script's last top-level expression, once
// Run returned without an error; nil before.
func (in *Interp) Last() object.Value { return object.OrNil(in.last) }

// Const returns the top-level constant name the script or the host
// defined.
func (in *Interp) Const(name string) (object.Value, bool) {
	v, ok := in.consts[name]
	return v, ok
}

// File is the script's path, as Run was given it; "" before Run.
func (in *Interp) File() string { return in.file }

// SetConst defines a top-level constant before the script runs: a module
// the host gives it.
func (in *Interp) SetConst(name string, v object.Value) {
	in.consts[name] = v
	in.changedTop()
}

// ErrStopped ends a run that Stop asked to end. It is not an exception, so
// no rescue catches it.
var ErrStopped = errors.New("the script was stopped")

// Stop ends the run, and the run of every worker it started: each stops
// at its next loop pass, method call or block call with ErrStopped.
func (in *Interp) Stop() { in.stop.stop() }

// stopped returns ErrStopped once Stop was called.
func (in *Interp) stopped() error {
	if in.stop.flag.Load() {
		return ErrStopped
	}
	return nil
}

// stopper is a run's stop: a flag the loops read, and channels that wake
// whatever awaits a promise. Stop closes them all. A waiter selects on the
// one its promise's id picks: a select locks every channel it waits on, so
// one channel for the whole run is a lock every waiter takes.
type stopper struct {
	flag atomic.Bool
	once sync.Once
	chs  [shards]chan struct{}
}

func newStopper() *stopper {
	s := &stopper{}
	for i := range s.chs {
		s.chs[i] = make(chan struct{})
	}
	return s
}

func (s *stopper) stop() {
	s.flag.Store(true)
	s.once.Do(func() {
		for _, ch := range s.chs {
			close(ch)
		}
	})
}

// ch returns the channel that wakes the waiters of promise id.
func (s *stopper) ch(id int64) <-chan struct{} { return s.chs[shardOf(id)] }

// New makes an interpreter writing to stdout and stderr.
func New(stdout, stderr io.Writer) *Interp {
	installOnce.Do(installStdlib)
	mainClass := object.NewClass("Object", object.ObjectClass)
	mainClass.Digest = "main"
	in := &Interp{
		stdout:    stdout,
		stderr:    stderr,
		outMu:     &sync.Mutex{},
		mainClass: mainClass,
		main:      &object.Instance{Cls: mainClass},
		top:       NewEnv(nil),
		consts:    map[string]object.Value{},
		proto:     protogen.NewRegistry(),
		promises:  newPromises(),
		code:      &code{},
		stop:      newStopper(),
		exits:     &exitHandlers{},
		states:    &libStates{m: map[string]any{}},
	}
	in.rng = object.NewRandom(object.NewSeed())
	return in
}

// Rand implements object.Context.
func (in *Interp) Rand() *object.Random { return in.rng }

// Resolver is what resolv looks names up with; see DNS.
func (in *Interp) Resolver() *net.Resolver { return in.DNS }

// HTTPClient is the client net/http makes requests with, or nil when the
// program has no network access; the library reads it from its Context.
func (in *Interp) HTTPClient() *http.Client { return in.HTTP }

// Done is closed when the run is stopped, so that a request in flight is
// cancelled with it.
func (in *Interp) Done() <-chan struct{} { return in.stop.chs[0] }

var installOnce sync.Once

// Run evaluates a program and returns the error that ended it: a raised
// exception, or nil.
func (in *Interp) Run(file string, prog *syntax.Program) error {
	in.file = file
	in.code.compiled = CompiledFor(prog)
	fr := in.topFrame()
	in.dbgTop = &dbgFrame{name: "<main>", env: in.top, fr: fr}
	if in.dbg != nil {
		in.dbg.stack = []*dbgFrame{in.dbgTop}
	}
	var v object.Value = object.Nil
	var err error
	for i := in.from; i < len(prog.Body); i++ {
		if in.pause.Load() {
			in.paused, in.next, in.nextLine = true, i, prog.Body[i].Line()
			return ErrPaused
		}
		if in.dbg != nil {
			if err = in.dbg.line(prog.Body[i].Line(), in.top, fr); err != nil {
				break
			}
		}
		if v, err = in.code.expr(prog.Body[i])(in, in.top, fr); err != nil {
			break
		}
	}
	if err == nil {
		in.last = v
	}
	// A run that ends cleanly ends when every call async started has; a
	// failure nobody awaited goes to standard error. One that raised
	// reports its error at once.
	if err == nil {
		err = in.waitStarted()
	}
	if c, ok := err.(*control); ok {
		err = nil
		if c.kind != ctrlReturn {
			err = in.raise(object.LocalJumpError, c.line, "%s from the top level", c.kind)
		}
	}
	// A run stopped from outside ends at once; any other runs the blocks
	// at_exit registered.
	if errors.Is(err, ErrStopped) {
		return err
	}
	return in.runExitHandlers(err)
}

// topFrame is the frame the top-level statements run in.
func (in *Interp) topFrame() *Frame {
	return &Frame{Self: object.InstanceValue(in.main), DefTarget: in.mainClass}
}

// Write writes to standard output, or to standard error when stderr is set.
func (in *Interp) Write(stderr bool, s string) error {
	in.outMu.Lock()
	defer in.outMu.Unlock()
	w := in.stdout
	if stderr {
		w = in.stderr
	}
	_, err := io.WriteString(w, s)
	return err
}

// FormatError renders an error the way the cameo binary reports it: the
// script, the line, the message and the exception class.
func (in *Interp) FormatError(err error) string {
	if exc, ok := err.(*object.Exception); ok {
		where := in.file
		if exc.Line > 0 {
			where = fmt.Sprintf("%s:%d", in.file, exc.Line)
		}
		return fmt.Sprintf("%s: %s (%s)", where, exc.Message, exc.Cls.QualifiedName())
	}
	return err.Error()
}

// exitHandlers are the blocks at_exit registered, in registration order.
// Workers register on the main interpreter's list.
type exitHandlers struct {
	mu     sync.Mutex
	blocks []object.Value
}

func (h *exitHandlers) push(block object.Value) {
	h.mu.Lock()
	h.blocks = append(h.blocks, block)
	h.mu.Unlock()
}

// pop removes and returns the last block registered, and false when there
// is none.
func (h *exitHandlers) pop() (object.Value, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.blocks) == 0 {
		return object.Value{}, false
	}
	block := h.blocks[len(h.blocks)-1]
	h.blocks = h.blocks[:len(h.blocks)-1]
	return block, true
}

// AtExit implements object.Context.
func (in *Interp) AtExit(block object.Value) { in.exits.push(block) }

// runExitHandlers runs the blocks at_exit registered, the last first, once
// the script ended with err, and returns the error the run ends with. The
// rules are Ruby's: a handler registered while they run goes next; a
// handler's exit sets the status, and the script's own exception is still
// reported; a handler's other exception is written to standard error, the
// handlers after it still run, and a run that would otherwise have ended
// cleanly, or by exit, ends with status 1.
func (in *Interp) runExitHandlers(err error) error {
	for {
		block, ok := in.exits.pop()
		if !ok {
			return err
		}
		_, herr := in.Yield(block)
		if herr == nil {
			continue
		}
		if c, ok := herr.(*control); ok {
			herr = in.raise(object.LocalJumpError, c.line, "%s from the top level", c.kind)
		}
		exc, ok := herr.(*object.Exception)
		if !ok {
			return herr
		}
		if exc.Cls.IsA(object.SystemExit) {
			if scriptExc, ok := err.(*object.Exception); ok && !scriptExc.Cls.IsA(object.SystemExit) {
				in.Write(true, in.FormatError(scriptExc)+"\n")
			}
			err = exc
			continue
		}
		in.Write(true, in.FormatError(exc)+"\n")
		if scriptExc, ok := err.(*object.Exception); err == nil || ok && scriptExc.Cls.IsA(object.SystemExit) {
			err = &object.Exception{Cls: object.SystemExit, Message: "exit", Attrs: map[string]object.Value{"status": object.Int(1)}}
		}
	}
}

// libStates are the Go values libraries keep for a run, shared with the
// workers.
type libStates struct {
	mu sync.Mutex
	m  map[string]any
}

// State implements object.Context.
func (in *Interp) State(name string, init func() any) any {
	in.states.mu.Lock()
	defer in.states.mu.Unlock()
	v, ok := in.states.m[name]
	if !ok {
		v = init()
		in.states.m[name] = v
	}
	return v
}
