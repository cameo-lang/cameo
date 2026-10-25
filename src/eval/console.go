package eval

import (
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/syntax"
)

// Eval runs the statements of prog at the top level of the run, as a
// line typed into cameo irb, and returns the value of the last one. The
// program ran, or has not: a run that is under way is another goroutine's.
// file names the script in errors when no run named it.
func (in *Interp) Eval(file string, prog *syntax.Program) (object.Value, error) {
	if in.file == "" {
		in.file = file
	}
	v, err := in.runStatements(prog.Body, in.top, in.topFrame())
	if c, ok := err.(*control); ok {
		return object.Value{}, in.raise(object.LocalJumpError, c.line, "%s from the top level", c.kind)
	}
	return v, err
}

// runStatements runs statements with env and fr and returns the last
// value; a return among them ends them with its value. A snippet's
// statements run once, so their thunks are not kept, as a script's are;
// the methods and classes they define are, as any.
func (in *Interp) runStatements(body []syntax.Node, env *Env, fr *Frame) (object.Value, error) {
	var v object.Value = object.Nil
	for _, n := range body {
		var err error
		if v, err = compile(n)(in, env, fr); err != nil {
			if c, ok := err.(*control); ok && c.kind == ctrlReturn {
				return c.value, nil
			}
			return object.Value{}, err
		}
	}
	return v, nil
}

// Prelude is what a top-level snippet is parsed behind so that its bare
// names read as the run's variables; see localsPrelude.
func (in *Interp) Prelude() string { return localsPrelude(in.top) }

// localsPrelude returns what a snippet is parsed behind so that the parser
// takes the variables bound in env for variables, as it would had it seen
// them assigned: "x = nil if false; " for each, on the snippet's first
// line, an assignment that never runs. Ruby's parser reads a bare name as
// a variable only once it saw it assigned. (`x = x` would not do: this
// parser resolves the right side before it binds the left.)
func localsPrelude(env *Env) string {
	var b strings.Builder
	seen := map[string]bool{}
	for e := env; e != nil; e = e.parent {
		e.each(func(name string, _ object.Value) {
			if !seen[name] {
				seen[name] = true
				b.WriteString(name + " = nil if false; ")
			}
		})
	}
	return b.String()
}
