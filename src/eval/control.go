package eval

import (
	"fmt"

	"github.com/radiospiel/cameo/src/object"
)

// control is a non-local exit travelling up the evaluator as an error:
// break, next, return and retry. A raised exception travels as an
// *object.Exception instead.
type control struct {
	kind   ctrlKind
	value  object.Value
	line   int
	target any // the *Frame a return goes to, the *Proc a break leaves
}

type ctrlKind int

const (
	ctrlBreak ctrlKind = iota
	ctrlNext
	ctrlReturn
	ctrlRetry
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
	return "an instance of " + v.Class().QualifiedName()
}
