// Package amber is the host API: load and run amber scripts from Go. The
// language is in doc/design/amber.md.
package amber

import (
	"io"
	"os"

	"github.com/radiospiel/amber/src/eval"
	"github.com/radiospiel/amber/src/syntax"
)

// Run parses, checks and runs a script, writing its output to stdout and
// stderr. It returns a syntax.ErrorList when the script does not check, or
// the exception that ended it, formatted by Format.
func Run(file, src string, stdout, stderr io.Writer) error {
	prog, err := syntax.Parse(file, src)
	if err != nil {
		return err
	}
	in := eval.New(stdout, stderr)
	if err := in.Run(file, prog); err != nil {
		return &RunError{Err: err, Message: in.FormatError(err)}
	}
	return nil
}

// RunFile runs the script at path.
func RunFile(path string, stdout, stderr io.Writer) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return Run(path, string(src), stdout, stderr)
}

// RunError is an exception that ended a script, with its message formatted
// for the terminal: "script.arb:12: message (Class)".
type RunError struct {
	Err     error
	Message string
}

func (e *RunError) Error() string { return e.Message }
func (e *RunError) Unwrap() error { return e.Err }
