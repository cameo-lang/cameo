package cameo

import (
	"fmt"
	"io"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/eval"
)

// ErrPaused is what Run returns once Pause asked the script to pause. It
// is not an exception, so no rescue catches it.
var ErrPaused = eval.ErrPaused

// Pause asks the running script to pause before its next instruction,
// inside a method or a loop as well as between two statements; Run then
// returns ErrPaused and Dump captures the script. A script inside a
// builtin's call of a block pauses when the builtin returns. The limits
// and the dump are in doc/design/pause.md.
func (p *Program) Pause() { p.in.Pause() }

// Finish runs the blocks at_exit registered since the script ended, by a
// Call or a console line, and returns the error they end with, a
// *RunError, or nil. Run runs the blocks the script registered; a host
// that drives the program after Run calls Finish when it is done with it.
func (p *Program) Finish() error {
	if err := p.in.Finish(); err != nil {
		return conv{p.in}.runError(err)
	}
	return nil
}

// Dump is a paused script, as Dump captures it.
type Dump struct {
	// Data is a cameo.Dump protobuf message in its wire form: the script,
	// where it paused, its top-level variables and constants, its classes
	// and their code, its promises. Resume reads it.
	Data []byte
	// Line is the line of the statement the script continues with.
	Line int
	// Missing names the variables and constants the dump leaves out,
	// each with what it held: "f (an Enumerator cannot be serialised)".
	Missing []string
}

// Dump captures a paused script. It waits for the calls async started,
// so their results travel settled.
func (p *Program) Dump() (*Dump, error) {
	d, err := p.in.Dump(p.src)
	if err != nil {
		return nil, err
	}
	data, err := proto.Marshal(d)
	if err != nil {
		return nil, err
	}
	out := &Dump{Data: data, Line: int(d.Line)}
	for _, b := range append(append([]*cameopb.Dump_Binding{}, d.Variables...), d.Constants...) {
		if b.Missing != "" {
			out.Missing = append(out.Missing, fmt.Sprintf("%s (%s)", b.Name, b.Missing))
		}
	}
	for _, b := range d.AtExit {
		if b.Missing != "" {
			out.Missing = append(out.Missing, fmt.Sprintf("an at_exit block (%s)", b.Missing))
		}
	}
	out.Missing = append(out.Missing, d.Missing...)
	return out, nil
}

// Resume loads a dump as a program that continues the script from where
// it paused: Run runs the statements after the pause point. The script's
// classes and methods are in the dump, so nothing before the pause point
// runs again. A dump resumes on a cameo of the IR version that wrote it.
func Resume(data []byte, stdout, stderr io.Writer) (*Program, error) {
	d, err := readDump(data)
	if err != nil {
		return nil, err
	}
	prog, err := Load(d.File, d.Source, stdout, stderr)
	if err != nil {
		return nil, err
	}
	if err := prog.in.Resume(prog.prog, d); err != nil {
		return nil, conv{prog.in}.runError(err)
	}
	return prog, nil
}

// InspectDump describes a dump: the script, where it paused, and every
// variable, constant and promise it holds, each as p would print it.
func InspectDump(data []byte) (string, error) {
	d, err := readDump(data)
	if err != nil {
		return "", err
	}
	prog, err := Resume(data, io.Discard, io.Discard)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "script: %s\n", d.File)
	fmt.Fprintf(&b, "paused: before line %d\n", d.Line)
	// The calls the run paused in, the innermost first; a run that
	// paused in a statement of the top level has none to show.
	if len(d.Stack) > 1 {
		b.WriteString("inside:\n")
		for i := len(d.Stack) - 1; i > 0; i-- {
			a := d.Stack[i]
			name := "the top level"
			var m *cameopb.Binding_Method
			if a.Kind == cameopb.Dump_Activation_BUILTIN {
				m = a.Builtin.GetMethod()
			} else if int(a.Frame) < len(d.Frames) {
				m = d.Frames[a.Frame].Method
			}
			switch {
			case m == nil:
			case m.Owner == "":
				name = m.Name
			case m.Singleton:
				name = m.Owner + "." + m.Name
			default:
				name = m.Owner + "#" + m.Name
			}
			switch a.Kind {
			case cameopb.Dump_Activation_BLOCK:
				fmt.Fprintf(&b, "  a block in %s, line %d\n", name, a.Line)
			case cameopb.Dump_Activation_BUILTIN:
				// A builtin is between two calls of its block.
				fmt.Fprintf(&b, "  %s\n", name)
			default:
				fmt.Fprintf(&b, "  %s, line %d\n", name, a.Line)
			}
		}
	}
	bindings := func(title string, bindings []*cameopb.Dump_Binding) error {
		fmt.Fprintf(&b, "%s:", title)
		if len(bindings) == 0 {
			b.WriteString(" none\n")
			return nil
		}
		b.WriteString("\n")
		for _, binding := range bindings {
			// A class under its own name is listed with the classes.
			if binding.Class != "" && binding.Class == binding.Name {
				continue
			}
			if binding.Missing != "" {
				fmt.Fprintf(&b, "  %s = (%s)\n", binding.Name, binding.Missing)
				continue
			}
			v, _, err := prog.in.Restore(binding)
			if err != nil {
				return err
			}
			s, err := prog.in.Inspect(v)
			if err != nil {
				return err
			}
			fmt.Fprintf(&b, "  %s = %s\n", binding.Name, s)
		}
		return nil
	}
	if err := bindings("variables", d.Variables); err != nil {
		return "", err
	}
	if err := bindings("constants", d.Constants); err != nil {
		return "", err
	}
	b.WriteString("classes:")
	if len(d.Classes) == 0 {
		b.WriteString(" none\n")
	} else {
		b.WriteString("\n")
	}
	for _, c := range d.Classes {
		kind := "class"
		if c.Module {
			kind = "module"
		}
		fmt.Fprintf(&b, "  %s %s\n", kind, c.Id)
	}
	b.WriteString("promises:")
	if len(d.Promises) == 0 {
		b.WriteString(" none\n")
	} else {
		b.WriteString("\n")
	}
	for _, ps := range d.Promises {
		method := ps.Method
		if method == "" {
			method = "promise"
		}
		if ps.Failure != nil {
			fmt.Fprintf(&b, "  %d %s raised %s (%s)\n", ps.Id, method, ps.Failure.Message, ps.Failure.Class)
			continue
		}
		v, _, err := prog.in.Restore(&cameopb.Dump_Binding{Value: ps.Value})
		if err != nil {
			return "", err
		}
		s, err := prog.in.Inspect(v)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "  %d %s = %s\n", ps.Id, method, s)
	}
	if n := len(d.AtExit); n > 0 {
		fmt.Fprintf(&b, "at_exit: %d\n", n)
	}
	return b.String(), nil
}

// readDump reads a cameo.Dump message from its wire form.
func readDump(data []byte) (*cameopb.Dump, error) {
	d := &cameopb.Dump{}
	if err := proto.Unmarshal(data, d); err != nil {
		return nil, fmt.Errorf("not a cameo dump: %w", err)
	}
	return d, nil
}
