package cameo

import "github.com/cameo-lang/cameo/src/syntax"

// Eval runs src, a snippet of cameo, at the top level of the script: its
// variables, methods and classes are the script's, as a line typed into
// cameo irb. It returns the value of the last expression as p would print
// it, or the error that ended it. Call it before Run, or after it
// returned.
func (p *Program) Eval(src string) (string, error) {
	prog, err := syntax.Parse(p.file, p.in.Prelude()+src)
	if err != nil {
		return "", err
	}
	v, err := p.in.Eval(p.file, prog)
	if err != nil {
		return "", &RunError{Err: err, Message: p.in.FormatError(err)}
	}
	return p.in.Inspect(v)
}

// Completions returns the name being typed at the end of before, a line
// typed into cameo irb up to the cursor, and the names it can be completed
// to at the top level of the script, sorted: after a dot, the methods of a
// receiver that is a variable, a constant or a literal; else the
// variables, methods, constants and keywords. It runs nothing but the
// lookup of a constant or a literal.
func (p *Program) Completions(before string) (word string, names []string) {
	return p.in.Completions(before)
}
