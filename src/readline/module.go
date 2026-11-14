package readline

import (
	"errors"
	"github.com/cameo-lang/cameo/src/str"
	"io"

	"github.com/cameo-lang/cameo/src/object"
)

// StateName is the run state (object.Context.State) that holds the Editor a
// host gave the program, cameo.Program.Terminal.
const StateName = "readline"

var (
	// kernelModule is Cameo::Kernel, the module of what cameo gives every
	// script without a require.
	kernelModule = object.NewModule("Kernel")
	// module is Cameo::Kernel::Readline.
	module = object.NewModule("Readline")
)

// Install defines Cameo::Kernel and Cameo::Kernel::Readline, whose
// functions read from the Editor the host gave the program:
// readline(prompt = "", add_hist = false) and history. lib.Install calls it
// once.
func Install() {
	kernelModule.Lexical = object.CameoModule
	object.CameoModule.Consts["Kernel"] = object.ClassValue(kernelModule)
	module.Lexical = kernelModule
	kernelModule.Consts["Readline"] = object.ClassValue(module)

	// readline returns the line typed, without its newline, and nil at the
	// end of the input or where the host gave the program no terminal. A
	// Ctrl-C raises Interrupt, as in Ruby.
	module.DefineSingleton("readline", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 2 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "wrong number of arguments (given %d, expected 0..2)", len(args))
		}
		prompt := ""
		if len(args) > 0 {
			s, ok := args[0].AsString()
			if !ok {
				return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into String", args[0].Class().QualifiedName())
			}
			prompt = s
		}
		add := len(args) > 1 && args[1].Truthy()
		e := editor(ctx)
		if e == nil {
			return object.Nil, nil
		}
		line, err := e.ReadLine(prompt, add)
		switch {
		case err == io.EOF:
			return object.Nil, nil
		case errors.Is(err, ErrInterrupt):
			return object.Value{}, ctx.Raise(object.Interrupt, "")
		case err != nil:
			return object.Value{}, ctx.Raise(object.IOError, "%s", err)
		}
		return object.NewString(line, str.UTF8), nil
	})
	// history returns the lines in the history, the oldest first, and an
	// empty Array where the host gave the program no terminal.
	module.DefineSingleton("history", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		e := editor(ctx)
		if e == nil {
			return object.NewList(), nil
		}
		lines := e.History()
		out := make([]object.Value, len(lines))
		for i, l := range lines {
			out[i] = object.NewString(l, str.UTF8)
		}
		return object.NewList(out...), nil
	})
}

// editor returns the Editor the host gave the program, or nil.
func editor(ctx object.Context) *Editor {
	e, _ := ctx.State(StateName, func() any { return (*Editor)(nil) }).(*Editor)
	return e
}
