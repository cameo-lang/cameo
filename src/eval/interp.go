// Package eval runs cameo programs: it compiles the syntax package's AST
// into closures, one per node, and runs them over the object package's
// values. The design is in doc/design/cameo.md.
package eval

import (
	"errors"
	"fmt"
	"io"
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
	stdout, stderr       io.Writer
	outMu                *sync.Mutex
	file                 string
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
	// stop ends the run, and is shared with the workers; see Stop.
	stop *stopper
	// catches are the tags of the catch blocks running, innermost last.
	catches []*catchTag
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
	}
	in.rng = object.NewRandom(object.NewSeed())
	return in
}

// Rand implements object.Context.
func (in *Interp) Rand() *object.Random { return in.rng }

var installOnce sync.Once

// Run evaluates a program and returns the error that ended it: a raised
// exception, or nil.
func (in *Interp) Run(file string, prog *syntax.Program) error {
	in.file = file
	in.code.compiled = CompiledFor(prog)
	fr := in.topFrame()
	var v object.Value = object.Nil
	var err error
	for i := in.from; i < len(prog.Body); i++ {
		if in.pause.Load() {
			in.paused, in.next, in.nextLine = true, i, prog.Body[i].Line()
			return ErrPaused
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
		if c.kind == ctrlReturn {
			return nil
		}
		return in.raise(object.LocalJumpError, c.line, "%s from the top level", c.kind)
	}
	return err
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
