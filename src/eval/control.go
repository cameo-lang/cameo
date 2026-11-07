package eval

import (
	"fmt"
	"maps"
	"slices"

	"github.com/cameo-lang/cameo/src/lib"
	"github.com/cameo-lang/cameo/src/object"
)

// control is a non-local exit travelling up the evaluator as an error:
// break, next, redo, return, retry, and a throw to its catch. A raised exception travels as an
// *object.Exception instead.
type control struct {
	kind   ctrlKind
	value  object.Value
	line   int
	target any // the *Frame a return goes to, the *Proc a break leaves, the *catchTag a throw ends
}

type ctrlKind int

const (
	ctrlBreak ctrlKind = iota
	ctrlNext
	ctrlReturn
	ctrlRetry
	ctrlRedo
	ctrlThrow
)

func (k ctrlKind) String() string {
	switch k {
	case ctrlBreak:
		return "break"
	case ctrlNext:
		return "next"
	case ctrlReturn:
		return "return"
	case ctrlRetry:
		return "retry"
	case ctrlRedo:
		return "redo"
	case ctrlThrow:
		return "throw"
	}
	return "control"
}

func (c *control) Error() string { return "unexpected " + c.kind.String() }

// raise makes an exception of class cls raised on line.
func (in *Interp) raise(cls *object.Class, line int, format string, args ...any) error {
	return &object.Exception{Cls: cls, Message: fmt.Sprintf(format, args...), Line: line}
}

// Raise implements object.Context.
func (in *Interp) Raise(cls *object.Class, format string, args ...any) error {
	return in.raise(cls, 0, format, args...)
}

// withLine records the line an exception came from when it has none yet.
func withLine(err error, line int) error {
	if exc, ok := err.(*object.Exception); ok && exc.Line == 0 {
		exc.Line = line
	}
	if c, ok := err.(*control); ok && c.line == 0 {
		c.line = line
	}
	return err
}

// typeName names a value's kind in error messages, as Ruby 3.4 does: "nil",
// "true", "an instance of Integer".
func typeName(v object.Value) string {
	switch v.Kind() {
	case object.KindBool:
		v := v.Bool()
		if v {
			return "true"
		}
		return "false"
	case object.KindClass:
		v := v.Cls()
		if v.IsModule {
			return "module " + v.QualifiedName()
		}
		return "class " + v.QualifiedName()
	}
	if v.IsNil() {
		return "nil"
	}
	if inst, ok := v.AsInstance(); ok && inst.Cls.Digest == "main" {
		return "main"
	}
	return "an instance of " + v.Class().QualifiedName()
}

// noMethod makes the error for a call of name that recv has no method
// for: a NoMethodError, or a NameError for a bare name, which may have
// been meant as a local. Either carries the name and the receiver, and a
// NoMethodError the arguments.
func (in *Interp) noMethod(recv object.Value, name string, args []object.Value, vcall bool, line int) error {
	attrs := map[string]object.Value{"name": object.Symbol(name), "receiver": recv}
	if vcall {
		return &object.Exception{Cls: object.NameError, Line: line, Attrs: attrs,
			Message: fmt.Sprintf("undefined local variable or method '%s' for %s", name, typeName(recv))}
	}
	attrs["args"] = object.NewList(append([]object.Value{}, args...)...)
	return &object.Exception{Cls: object.NoMethodError, Line: line, Attrs: attrs,
		Message: fmt.Sprintf("undefined method '%s' for %s%s%s", name, typeName(recv), in.requireHint(name), ioHint(recv, name))}
}

// ioHint says how to allow the I/O a class method of a module of I/O
// needs, when recv extends one that defines name: File.read without file
// I/O. "" otherwise.
func ioHint(recv object.Value, name string) string {
	cls, ok := recv.AsClass()
	if !ok {
		return ""
	}
	for k := cls; k != nil; k = k.Super {
		for _, e := range k.Extends {
			if e.IOKind != "" && e.Lookup(name) != nil {
				return fmt.Sprintf(": the host allows no %s I/O; run the script with cameo --io=%s, or let the program say AllowIO(%q)%s", e.IOKind, e.IOKind, e.IOKind, map[bool]string{true: " and give it a file system with FileSystem"}[e.IOKind == "file"])
			}
		}
	}
	return ""
}

// requireHint says which library to require for name, when a library the
// script has not required brings a function of that name to the top level
// (lib.TopLevel): `: require "minispec" first`, or "".
func (in *Interp) requireHint(name string) string {
	for _, l := range slices.Sorted(maps.Keys(lib.TopLevel)) {
		if in.loaded[l] {
			continue
		}
		for _, m := range lib.TopLevel[l] {
			if m.Lookup(name) != nil {
				return fmt.Sprintf(": require %q first", l)
			}
		}
	}
	return ""
}

// privateMethod is the NoMethodError for a function of Kernel called with
// a receiver: "private method 'puts' called for class File", as Ruby's.
func (in *Interp) privateMethod(recv object.Value, name string, line int) error {
	attrs := map[string]object.Value{"name": object.Symbol(name), "receiver": recv}
	return &object.Exception{Cls: object.NoMethodError, Line: line, Attrs: attrs,
		Message: fmt.Sprintf("private method '%s' called for %s", name, typeName(recv))}
}
