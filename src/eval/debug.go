package eval

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/syntax"
)

// The debugger stops a run before a statement and talks to the user on a
// console, with the commands of Ruby's debug library (rdbg): step, next,
// finish, continue, breakpoints on a line or a method, the backtrace, the
// frames' locals, and any expression, evaluated in the frame it stopped
// in. It is on once the script required "debug", or the run started with
// `cameo debug`; `debugger` and `binding.break` then stop before the next
// statement. Every statement compiled while it is on reports its line
// (code.statement), and every method and block call its frame (invoke,
// callBlock), so a debugger switched on by require sees the code that
// compiles after it. Workers async starts have no debugger.

// Debugger is the debugger of one run; see Interp.Debug.
type Debugger struct {
	in      *Interp
	console *bufio.Scanner
	out     io.Writer
	// lines are the script's lines, which list prints.
	lines []string
	// stack is the frames of the run, the innermost last; selected is
	// the one the commands read, as up and down move it.
	stack    []*dbgFrame
	selected int
	// mode says when the run stops next: after a step, at the next line of
	// the frame of depth stopDepth or above after next, below it after
	// finish, at a breakpoint alone after continue. pending stops before
	// the next statement whatever the mode: the first one, or the one
	// after debugger.
	mode      stepMode
	stopDepth int
	pending   bool
	breaks    []breakpoint
	nextBreak int
	// evaluating is set while a command's expression runs, whose
	// statements and calls the debugger does not stop at.
	evaluating bool
	// closed is set once the console ended: the run continues without
	// stops.
	closed bool
	// quit is set when the user quit, which ends the run with ErrStopped.
	quit bool
}

type stepMode int

const (
	modeContinue stepMode = iota
	modeStep
	modeNext
	modeFinish
)

// dbgFrame is one frame of the run: a method, a block, or the top level.
type dbgFrame struct {
	name string
	env  *Env
	fr   *Frame
	line int
}

// breakpoint stops the run at a line, or at the first line of a method.
type breakpoint struct {
	id     int
	line   int
	method string
}

// Debug switches the debugger on, talking on console and out, with the
// script's source for list, and makes the run stop before its first
// statement when stop is set. It is called before Run, or by require
// "debug" during it.
func (in *Interp) Debug(console io.Reader, out io.Writer, source string, stop bool) *Debugger {
	if in.dbg == nil {
		in.dbg = &Debugger{in: in, console: bufio.NewScanner(console), out: out}
		in.dbg.console.Buffer(make([]byte, 1<<20), 1<<20)
		in.code.debug.Store(true)
		// With the debugger on, debugger and binding.break are defined
		// as if the script had required "debug".
		if in.loaded == nil {
			in.loaded = map[string]bool{}
		}
		in.loaded["debug"] = true
		if in.dbgTop != nil {
			in.dbg.stack = []*dbgFrame{in.dbgTop}
		}
	}
	if source != "" {
		in.dbg.lines = strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	}
	if stop {
		in.dbg.pending = true
	}
	return in.dbg
}

// Debugging reports whether the debugger is on, and Debugger returns it.
func (in *Interp) Debugging() bool     { return in.dbg != nil }
func (in *Interp) Debugger() *Debugger { return in.dbg }

// SetConsole names where a debugger the script switches on with require
// "debug" talks; without one, its stops do nothing.
func (in *Interp) SetConsole(console io.Reader, out io.Writer, source string) {
	in.console, in.consoleOut, in.source = console, out, source
}

// Break is what debugger and binding.break do: the run stops before the
// next statement when the debugger is on.
func (in *Interp) Break() {
	if in.dbg != nil && !in.dbg.closed {
		in.dbg.pending = true
	}
}

// enter pushes the frame of a call; leave pops it.
func (d *Debugger) enter(name string, env *Env, fr *Frame) {
	if d.evaluating {
		return
	}
	d.stack = append(d.stack, &dbgFrame{name: name, env: env, fr: fr})
	if d.closed {
		return
	}
	for _, b := range d.breaks {
		if b.method != "" && (b.method == name || strings.HasSuffix(name, "#"+b.method) || strings.HasSuffix(name, "."+b.method)) {
			d.pending = true
		}
	}
}

func (d *Debugger) leave() {
	if d.evaluating || len(d.stack) == 0 {
		return
	}
	d.stack = d.stack[:len(d.stack)-1]
}

// depth is how many frames are on the stack.
func (d *Debugger) depth() int { return len(d.stack) }

// line is called before a statement runs: it records the line and stops
// when the mode, a breakpoint or a pending stop says to.
func (d *Debugger) line(line int, env *Env, fr *Frame) error {
	if d.evaluating || d.closed {
		return nil
	}
	if len(d.stack) == 0 {
		d.stack = append(d.stack, &dbgFrame{name: "<main>", env: env, fr: fr})
	}
	top := d.stack[len(d.stack)-1]
	top.line, top.env, top.fr = line, env, fr
	stop := d.pending
	switch d.mode {
	case modeStep:
		stop = true
	case modeNext:
		stop = stop || d.depth() <= d.stopDepth
	case modeFinish:
		stop = stop || d.depth() < d.stopDepth
	}
	for _, b := range d.breaks {
		if b.line == line {
			stop = true
		}
	}
	if !stop {
		return nil
	}
	d.pending = false
	d.mode = modeContinue
	return d.repl()
}

// repl shows where the run stopped and runs commands until one lets the
// run go on.
func (d *Debugger) repl() error {
	d.selected = len(d.stack) - 1
	d.list(d.selected, 0)
	d.backtrace(1)
	for {
		fmt.Fprint(d.out, "(rdbg) ")
		if !d.console.Scan() {
			fmt.Fprintln(d.out)
			d.closed = true
			return nil
		}
		input := strings.TrimSpace(d.console.Text())
		cmd, arg, _ := strings.Cut(input, " ")
		arg = strings.TrimSpace(arg)
		switch cmd {
		case "":
		case "s", "step":
			d.mode = modeStep
			return nil
		case "n", "next":
			d.mode, d.stopDepth = modeNext, d.depth()
			return nil
		case "fin", "finish":
			d.mode, d.stopDepth = modeFinish, d.depth()
			return nil
		case "c", "continue":
			d.mode = modeContinue
			return nil
		case "q", "quit", "q!", "quit!", "kill", "kill!":
			d.quit = true
			d.in.Stop()
			return ErrStopped
		case "b", "break":
			d.addBreak(arg)
		case "del", "delete":
			d.deleteBreak(arg)
		case "bt", "backtrace", "where":
			d.backtrace(len(d.stack))
		case "l", "list":
			d.listCommand(arg)
		case "i", "info":
			d.info(arg)
		case "u", "up":
			d.move(-1)
		case "d", "down":
			d.move(1)
		case "f", "frame":
			n, err := strconv.Atoi(arg)
			if err != nil || n < 0 || n >= len(d.stack) {
				fmt.Fprintf(d.out, "no frame %q\n", arg)
				continue
			}
			d.selected = len(d.stack) - 1 - n
			d.backtrace(1)
		case "p", "pp", "e", "eval":
			d.print(arg, cmd == "pp")
		case "h", "help":
			fmt.Fprint(d.out, help)
		default:
			d.print(input, false)
		}
	}
}

const help = `step (s)         run to the next statement, into calls
next (n)         run to the next statement in this frame, over calls
finish (fin)     run until this frame returns
continue (c)     run until a breakpoint, debugger or binding.break
break (b) LINE   stop before line LINE; break METHOD, break Class#METHOD
                 stop at the method's first line; break alone lists them
delete (del) N   delete breakpoint N; delete alone deletes all
backtrace (bt)   the frames, innermost first
list (l) [LINE]  the source around the current line, or around LINE
info (i)         the locals and self of the frame
up (u), down (d) select the outer or inner frame; frame (f) N selects #N
p EXPR, pp EXPR  evaluate EXPR in the frame; any other input is an EXPR
quit (q)         end the run
`

// addBreak adds a breakpoint from its argument: a line, file:line, or a
// method name, Class#method or method; an empty argument lists them.
func (d *Debugger) addBreak(arg string) {
	if arg == "" {
		if len(d.breaks) == 0 {
			fmt.Fprintln(d.out, "no breakpoints")
		}
		for _, b := range d.breaks {
			fmt.Fprintln(d.out, d.describeBreak(b))
		}
		return
	}
	// file:line names a line; Foo::Bar.baz names a method.
	if i := strings.LastIndex(arg, ":"); i >= 0 {
		if _, err := strconv.Atoi(arg[i+1:]); err == nil {
			arg = arg[i+1:]
		}
	}
	d.nextBreak++
	b := breakpoint{id: d.nextBreak}
	if n, err := strconv.Atoi(arg); err == nil {
		b.line = n
	} else {
		b.method = arg
	}
	d.breaks = append(d.breaks, b)
	fmt.Fprintln(d.out, d.describeBreak(b))
}

func (d *Debugger) describeBreak(b breakpoint) string {
	if b.method != "" {
		return fmt.Sprintf("#%d  BP - Method  %s", b.id, b.method)
	}
	return fmt.Sprintf("#%d  BP - Line  %s:%d", b.id, d.in.file, b.line)
}

func (d *Debugger) deleteBreak(arg string) {
	if arg == "" {
		d.breaks = nil
		fmt.Fprintln(d.out, "deleted all breakpoints")
		return
	}
	n, err := strconv.Atoi(arg)
	if err != nil {
		fmt.Fprintf(d.out, "no breakpoint %q\n", arg)
		return
	}
	for i, b := range d.breaks {
		if b.id == n {
			d.breaks = append(d.breaks[:i], d.breaks[i+1:]...)
			fmt.Fprintf(d.out, "deleted breakpoint #%d\n", n)
			return
		}
	}
	fmt.Fprintf(d.out, "no breakpoint #%d\n", n)
}

// backtrace prints the innermost n frames, the selected one marked.
func (d *Debugger) backtrace(n int) {
	for i := len(d.stack) - 1; i >= 0 && i >= len(d.stack)-n; i-- {
		f := d.stack[i]
		mark := "  "
		if i == d.selected {
			mark = "=>"
		}
		fmt.Fprintf(d.out, "%s#%d\t%s at %s:%d\n", mark, len(d.stack)-1-i, d.frameName(f), d.in.file, f.line)
	}
}

// frameName is a frame's name with its arguments, as rdbg shows it:
// fib(n=3).
func (d *Debugger) frameName(f *dbgFrame) string {
	if strings.HasPrefix(f.name, "block in ") || f.fr == nil || f.fr.Method == nil {
		return f.name
	}
	cd, ok := f.fr.Method.Body.(*compiledDef)
	if !ok {
		return f.name
	}
	var args []string
	for _, p := range cd.def.Params.Positional {
		if v, ok := f.env.lookup(p.Name); ok {
			args = append(args, p.Name+"="+d.inspect(v))
		}
	}
	for _, p := range cd.def.Params.Keywords {
		if v, ok := f.env.lookup(p.Name); ok {
			args = append(args, p.Name+": "+d.inspect(v))
		}
	}
	return f.name + "(" + strings.Join(args, ", ") + ")"
}

// listCommand lists around the selected frame's line, or around the line
// named.
func (d *Debugger) listCommand(arg string) {
	if arg == "" {
		d.list(d.selected, 0)
		return
	}
	n, err := strconv.Atoi(arg)
	if err != nil {
		fmt.Fprintf(d.out, "list takes a line, not %q\n", arg)
		return
	}
	d.list(d.selected, n)
}

// list prints the source around line around, the selected frame's line
// when around is 0, with the frame's line marked.
func (d *Debugger) list(frame, around int) {
	if len(d.lines) == 0 {
		return
	}
	current := 0
	if frame >= 0 && frame < len(d.stack) {
		current = d.stack[frame].line
	}
	if around == 0 {
		around = current
	}
	from, to := around-4, around+5
	if from < 1 {
		from, to = 1, min(len(d.lines), 10)
	}
	if to > len(d.lines) {
		to = len(d.lines)
	}
	fmt.Fprintf(d.out, "[%d, %d] in %s\n", from, to, d.in.file)
	for i := from; i <= to; i++ {
		mark := "  "
		if i == current {
			mark = "=>"
		}
		fmt.Fprintf(d.out, "%s %4d| %s\n", mark, i, d.lines[i-1])
	}
}

// info prints the locals of the selected frame and its self.
func (d *Debugger) info(arg string) {
	if arg == "b" || arg == "breakpoints" {
		d.addBreak("")
		return
	}
	f := d.stack[d.selected]
	if f.fr != nil {
		fmt.Fprintf(d.out, "%%self = %s\n", d.inspect(f.fr.Self))
	}
	names := map[string]object.Value{}
	for e := f.env; e != nil; e = e.parent {
		e.each(func(name string, v object.Value) {
			if _, seen := names[name]; !seen {
				names[name] = v
			}
		})
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	for _, name := range sorted {
		fmt.Fprintf(d.out, "%s = %s\n", name, d.inspect(names[name]))
	}
}

// move selects the frame by steps away from the selected one: -1 is the
// caller.
func (d *Debugger) move(by int) {
	n := d.selected + by
	if n < 0 || n >= len(d.stack) {
		fmt.Fprintln(d.out, "no such frame")
		return
	}
	d.selected = n
	d.backtrace(len(d.stack))
}

// print evaluates src in the selected frame and prints its value, as p or
// pp would.
func (d *Debugger) print(src string, pretty bool) {
	f := d.stack[d.selected]
	v, err := d.in.evalIn(src, f.env, f.fr)
	if err != nil {
		if exc, ok := err.(*object.Exception); ok {
			fmt.Fprintf(d.out, "eval error: %s (%s)\n", exc.Message, exc.Cls.QualifiedName())
		} else {
			fmt.Fprintf(d.out, "eval error: %s\n", err)
		}
		return
	}
	s := d.inspect(v)
	if pretty {
		if ps, err := d.in.callPP(v); err == nil {
			s = ps
		}
	}
	fmt.Fprintf(d.out, "=> %s\n", s)
}

func (d *Debugger) inspect(v object.Value) string {
	s, err := d.in.Inspect(object.OrNil(v))
	if err != nil {
		return "(" + err.Error() + ")"
	}
	return s
}

// callPP renders v as pp prints it, through the pp library.
func (in *Interp) callPP(v object.Value) (string, error) {
	if m := userMethod(v, "pretty_inspect"); m != nil {
		r, err := in.invoke(m, v, nil, nil, object.Value{}, 0)
		if err != nil {
			return "", err
		}
		return in.ToS(r)
	}
	return in.Inspect(v)
}

// evalIn runs src, parsed as a program, with env and fr, the debugger's
// stops off, and returns its last value.
func (in *Interp) evalIn(src string, env *Env, fr *Frame) (object.Value, error) {
	prog, err := syntax.Parse("(rdbg)", localsPrelude(env)+src)
	if err != nil {
		return object.Value{}, err
	}
	if in.dbg != nil {
		in.dbg.evaluating = true
		defer func() { in.dbg.evaluating = false }()
	}
	return in.runStatements(prog.Body, env, fr)
}

// Quit reports whether the debugger ended the run.
func (d *Debugger) Quit() bool { return d.quit }
