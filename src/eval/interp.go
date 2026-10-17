// Package eval runs amber programs: it compiles the syntax package's AST
// into closures, one per node, and runs them over the object package's
// values. The design is in doc/design/amber.md.
package eval

import (
	"fmt"
	"io"
	"sync"

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
}

// New makes an interpreter writing to stdout and stderr.
func New(stdout, stderr io.Writer) *Interp {
	installOnce.Do(installStdlib)
	mainClass := object.NewClass("Object", object.ObjectClass)
	mainClass.Digest = "main"
	return &Interp{
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
	}
}

var installOnce sync.Once

// Run evaluates a program and returns the error that ended it: a raised
// exception, or nil.
func (in *Interp) Run(file string, prog *syntax.Program) error {
	in.file = file
	fr := &Frame{Self: object.InstanceValue(in.main), DefTarget: in.mainClass}
	_, err := in.code.body(prog, prog.Body)(in, in.top, fr)
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
