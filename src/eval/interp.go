// Package eval runs amber programs: it compiles the syntax package's AST
// into closures, one per node, and runs them over the object package's
// values. The design is in doc/design/amber.md.
package eval

import (
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"sync"
	"sync/atomic"

	"github.com/radiospiel/amber/src/object"
	"github.com/radiospiel/amber/src/protogen"
	"github.com/radiospiel/amber/src/syntax"
)

// Interp is one interpreter: a top-level scope, the standard output and
// error streams, and the constants the script defines. Workers started by
// async get interpreters of their own.
type Interp struct {
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
	// procs is every process the script started, shared with its workers,
	// so the run can wait for all of them and report the failures nobody
	// awaited.
	procs *processList
	// code is the compiled program, shared with the workers.
	code *code
	// rng is this interpreter's random number generator and seed the seed
	// it started from; see Rand and Srand.
	rng  *rand.Rand
	seed uint64
	// loaded is the libraries require has loaded, which a second require
	// of answers false.
	loaded map[string]bool
	// stop is set to end the run, and shared with the workers; see Stop.
	stop *atomic.Bool
	// last is the value of the last top-level expression Run evaluated.
	last object.Value
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
func (in *Interp) SetConst(name string, v object.Value) { in.consts[name] = v }

// ErrStopped ends a run that Stop asked to end. It is not an exception, so
// no rescue catches it.
var ErrStopped = errors.New("the script was stopped")

// Stop ends the run, and the run of every worker it started: each stops
// at its next loop pass, method call or block call with ErrStopped.
func (in *Interp) Stop() { in.stop.Store(true) }

// stopped returns ErrStopped once Stop was called.
func (in *Interp) stopped() error {
	if in.stop.Load() {
		return ErrStopped
	}
	return nil
}

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
		procs:     &processList{},
		code:      &code{},
		stop:      &atomic.Bool{},
	}
	in.Srand(rand.Uint64())
	return in
}

// Rand implements object.Context.
func (in *Interp) Rand() *rand.Rand { return in.rng }

// Srand implements object.Context: a new generator from seed.
func (in *Interp) Srand(seed uint64) uint64 {
	old := in.seed
	in.seed = seed
	in.rng = rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	return old
}

var installOnce sync.Once

// Run evaluates a program and returns the error that ended it: a raised
// exception, or nil.
func (in *Interp) Run(file string, prog *syntax.Program) error {
	in.file = file
	fr := &Frame{Self: object.InstanceValue(in.main), DefTarget: in.mainClass}
	v, err := in.code.body(prog, prog.Body)(in, in.top, fr)
	if err == nil {
		in.last = v
	}
	if c, ok := err.(*control); ok {
		if c.kind == ctrlReturn {
			return nil
		}
		return in.raise(object.LocalJumpError, c.line, "%s from the top level", c.kind)
	}
	return err
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

// FormatError renders an error the way the amber binary reports it: the
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
