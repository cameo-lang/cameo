package cameo

import "io"

// Console names where the debugger talks when the script switches it on
// with require "debug": it reads commands from console and writes to out.
// Without one, debugger and binding.break stop nothing.
func (p *Program) Console(console io.Reader, out io.Writer) {
	p.in.SetConsole(console, out, p.src)
}

// Debug switches the debugger on, talking on console and out, and makes
// Run stop before the script's first statement: what cameo debug does.
// The debugger's commands are those of Ruby's debug library; see
// doc/debugger.md.
func (p *Program) Debug(console io.Reader, out io.Writer) {
	p.in.SetConsole(console, out, p.src)
	p.in.Debug(console, out, p.src, true)
}

// Quit reports whether the debugger's quit command ended the run: Run then
// returned an error wrapping eval.ErrStopped, which is not a failure of
// the script.
func (p *Program) Quit() bool {
	return p.in.Debugging() && p.in.Debugger().Quit()
}
