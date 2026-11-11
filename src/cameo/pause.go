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

// Pause asks the running script to pause before its next top-level
// statement; Run then returns ErrPaused and Dump captures the script. A
// statement that never ends is never paused. The limits and the dump are
// in doc/design/pause.md.
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
	// where it paused, its top-level variables and constants, its
	// promises. Resume reads it.
	Data []byte
	// Line is the line of the statement the script continues with.
	Line int
	// Missing names the variables and constants the dump leaves out,
	// each with what it held: "f (a Proc cannot be serialised)".
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
	return out, nil
}

// Resume loads a dump as a program that continues the script from where
// it paused: Run runs the statements after the pause point. The script's
// definitions before it run again first, with their output going to
// stdout and stderr.
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
	bindings := func(title string, bindings []*cameopb.Dump_Binding) error {
		fmt.Fprintf(&b, "%s:", title)
		if len(bindings) == 0 {
			b.WriteString(" none\n")
			return nil
		}
		b.WriteString("\n")
		for _, binding := range bindings {
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
