// Package getoptlong is Ruby's getoptlong library, built into cameo:
// GetoptLong, the GNU getopt_long way of reading a command line, with
// each, get, ordering, quiet and the errors of a bad command line.
//
// It follows getoptlong 0.2.1, the version Ruby 3.4 ships. In Ruby
// GetoptLong takes the options out of ARGV as it goes, leaving the
// operands; ARGV is a value here, so the parser works on a copy and the
// operands are left in rest, which Ruby does not have.
package getoptlong

import (
	_ "embed"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	class       = object.NewClass("GetoptLong", object.ObjectClass)
	errorClass  = object.NewClass("Error", object.StandardError)
	ambiguous   = object.NewClass("AmbiguousOption", errorClass)
	needlessArg = object.NewClass("NeedlessArgument", errorClass)
	missingArg  = object.NewClass("MissingArgument", errorClass)
	invalidOpt  = object.NewClass("InvalidOption", errorClass)
)

const (
	noArgument       = 0
	requiredArgument = 1
	optionalArgument = 2

	requireOrder  = 0
	permute       = 1
	returnInOrder = 2
)

// option is one declared option: its names, the first of which is
// canonical, and whether it takes an argument.
type option struct {
	names []string
	arg   int
}

// getopt is a GetoptLong's state: a handle, as Ruby's object is.
type getopt struct {
	options    []*option
	ordering   int
	quiet      bool
	args       []string // what is left to read
	rest       []string // the operands, in order
	started    bool
	terminated bool
	errorMsg   object.Value // the last error's message, Nil for none
	errorCls   object.Value // the last error's class, Nil for none
}

func getoptOf(v object.Value) *getopt { return v.Instance().Message.(*getopt) }

// find looks an option name up, completing a unique prefix of a long one.
func (g *getopt) find(name string) (*option, error) {
	var hits []*option
	for _, o := range g.options {
		for _, n := range o.names {
			if n == name {
				return o, nil
			}
			if strings.HasPrefix(name, "--") && strings.HasPrefix(n, name) {
				hits = append(hits, o)
				break
			}
		}
	}
	if len(hits) == 1 {
		return hits[0], nil
	}
	if len(hits) > 1 {
		return nil, g.fail(ambiguous, "option `"+name+"' is ambiguous")
	}
	return nil, g.fail(invalidOpt, "unrecognized option `"+name+"'")
}

func (g *getopt) fail(cls *object.Class, msg string) error {
	g.errorMsg = object.NewString(msg, object.UTF8)
	g.errorCls = object.ClassValue(cls)
	return &object.Exception{Cls: cls, Message: msg}
}

// get reads the next option: its canonical name and its argument, "" for
// none, or ok false at the end.
func (g *getopt) get(ctx object.Context) (name, arg string, ok bool, err error) {
	g.started = true
	for len(g.args) > 0 && !g.terminated {
		a := g.args[0]
		g.args = g.args[1:]
		switch {
		case a == "--":
			g.rest = append(g.rest, g.args...)
			g.args = nil
			g.terminated = true
			return "", "", false, nil
		case strings.HasPrefix(a, "--"):
			text, value, hasValue := strings.Cut(a, "=")
			o, err := g.find(text)
			if err != nil {
				return "", "", false, err
			}
			switch o.arg {
			case noArgument:
				if hasValue {
					return "", "", false, g.fail(needlessArg, "option `"+text+"' doesn't allow an argument")
				}
			case requiredArgument:
				if !hasValue {
					if len(g.args) == 0 {
						return "", "", false, g.fail(missingArg, "option `"+text+"' requires an argument")
					}
					value, g.args = g.args[0], g.args[1:]
				}
			case optionalArgument:
				if !hasValue && len(g.args) > 0 && !strings.HasPrefix(g.args[0], "-") {
					value, g.args = g.args[0], g.args[1:]
				}
			}
			return o.names[0], value, true, nil
		case strings.HasPrefix(a, "-") && len(a) > 1:
			name := a[:2]
			o, err := g.find(name)
			if err != nil {
				return "", "", false, g.fail(invalidOpt, "invalid option -- "+name[1:])
			}
			attached := a[2:]
			value := ""
			switch o.arg {
			case noArgument:
				if attached != "" {
					g.args = append([]string{"-" + attached}, g.args...)
				}
			case requiredArgument:
				value = attached
				if attached == "" {
					if len(g.args) == 0 {
						return "", "", false, g.fail(missingArg, "option requires an argument -- "+name[1:])
					}
					value, g.args = g.args[0], g.args[1:]
				}
			case optionalArgument:
				value = attached
				if attached == "" && len(g.args) > 0 && !strings.HasPrefix(g.args[0], "-") {
					value, g.args = g.args[0], g.args[1:]
				}
			}
			return o.names[0], value, true, nil
		default:
			switch g.ordering {
			case requireOrder:
				g.rest = append(g.rest, a)
				g.rest = append(g.rest, g.args...)
				g.args = nil
				g.terminated = true
				return "", "", false, nil
			case returnInOrder:
				return "", a, true, nil
			default:
				g.rest = append(g.rest, a)
			}
		}
	}
	g.terminated = true
	return "", "", false, nil
}

// report prints an error as Ruby does, "program: message", unless quiet.
func (g *getopt) report(ctx object.Context, err error) error {
	if g.quiet {
		return err
	}
	ex, ok := err.(*object.Exception)
	if !ok {
		return err
	}
	prog := "cameo"
	if f, ok := ctx.(interface{ File() string }); ok && f.File() != "" {
		prog = f.File()
	}
	if werr := ctx.Write(true, prog+": "+ex.Message+"\n"); werr != nil {
		return werr
	}
	return err
}

func pair(name, arg string) object.Value {
	return object.NewList(object.NewString(name, object.UTF8), object.NewString(arg, object.UTF8))
}

func strings2(ss []string) object.Value {
	out := make([]object.Value, len(ss))
	for i, s := range ss {
		out[i] = object.NewString(s, object.UTF8)
	}
	return object.NewList(out...)
}

// Source is Getopt::Options and Getopt::Switch, written in cameo: a class
// body declares its switches with on, and parse folds the command line
// into an instance. require runs it at the top level.
//
//go:embed getoptlong.cameo
var Source string

// Install defines GetoptLong. lib.Install calls it once.
func Install() {
	c := class
	object.DefineNative("GetoptLong", object.ClassValue(c))
	// each yields [option, argument] pairs, so reduce, map and to_a fold
	// the command line into a value; a block cannot write into its
	// surroundings.
	c.Include(object.Enumerable)
	c.Consts["VERSION"] = object.NewString("0.2.1", object.ASCII)
	c.Consts["NO_ARGUMENT"] = object.Int(noArgument)
	c.Consts["REQUIRED_ARGUMENT"] = object.Int(requiredArgument)
	c.Consts["OPTIONAL_ARGUMENT"] = object.Int(optionalArgument)
	c.Consts["REQUIRE_ORDER"] = object.Int(requireOrder)
	c.Consts["PERMUTE"] = object.Int(permute)
	c.Consts["RETURN_IN_ORDER"] = object.Int(returnInOrder)
	c.Consts["ORDERINGS"] = object.NewList(object.Int(requireOrder), object.Int(permute), object.Int(returnInOrder))
	c.Consts["ARGUMENT_FLAGS"] = object.NewList(object.Int(noArgument), object.Int(requiredArgument), object.Int(optionalArgument))
	for _, e := range []*object.Class{errorClass, ambiguous, needlessArg, missingArg, invalidOpt} {
		e.Lexical = c
		c.Consts[e.Name] = object.ClassValue(e)
	}
	c.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		g := &getopt{ordering: permute, errorMsg: object.Nil, errorCls: object.Nil}
		seen := map[string]bool{}
		for _, a := range args {
			list, err := stdlib.ToList(ctx, a)
			if err != nil {
				return object.Value{}, err
			}
			if len(list) == 0 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "no argument-flag")
			}
			o := &option{arg: -1}
			for _, e := range list {
				if e.IsString() {
					name := e.Str()
					if !strings.HasPrefix(name, "-") || name == "-" || name == "--" || strings.HasPrefix(name, "-") && !strings.HasPrefix(name, "--") && len(name) != 2 {
						return object.Value{}, ctx.Raise(object.ArgumentError, "an invalid option `%s'", name)
					}
					if seen[name] {
						return object.Value{}, ctx.Raise(object.ArgumentError, "option redefined `%s'", name)
					}
					seen[name] = true
					o.names = append(o.names, name)
					continue
				}
				n, err := stdlib.WantInt(ctx, e, "")
				if err != nil {
					return object.Value{}, err
				}
				if n < noArgument || n > optionalArgument {
					return object.Value{}, ctx.Raise(object.ArgumentError, "no argument-flag")
				}
				if o.arg >= 0 {
					return object.Value{}, ctx.Raise(object.ArgumentError, "too many argument-flags")
				}
				o.arg = int(n)
			}
			if o.arg < 0 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "no argument-flag")
			}
			if len(o.names) == 0 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "no option name")
			}
			g.options = append(g.options, o)
		}
		// args: gives the arguments to read instead of ARGV, which Ruby's
		// has no way to say, since it takes them out of ARGV itself.
		argv, given := object.Value{}, false
		if kw != nil {
			argv, given = kw.Get(object.Symbol("args"))
		}
		if !given {
			argv, given = ctx.Const("ARGV")
		}
		if given {
			list, err := stdlib.ToList(ctx, argv)
			if err != nil {
				return object.Value{}, err
			}
			for _, e := range list {
				s, err := stdlib.WantString(ctx, e)
				if err != nil {
					return object.Value{}, err
				}
				g.args = append(g.args, s)
			}
		}
		return object.InstanceValue(&object.Instance{Cls: c, Message: g}), nil
	})
	def := func(name string, arity int, f func(ctx object.Context, g *getopt, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error)) {
		c.Define(name, arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return f(ctx, getoptOf(self), self, args, kw, block)
		})
	}
	get := func(ctx object.Context, g *getopt, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, arg, ok, err := g.get(ctx)
		if err != nil {
			return object.Value{}, g.report(ctx, err)
		}
		if !ok {
			return object.Nil, nil
		}
		return pair(name, arg), nil
	}
	def("get", 0, get)
	def("get_option", 0, get)
	each := func(ctx object.Context, g *getopt, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for {
			name, arg, ok, err := g.get(ctx)
			if err != nil {
				return object.Value{}, g.report(ctx, err)
			}
			if !ok {
				return object.Nil, nil
			}
			if _, err := ctx.Yield(block, object.NewString(name, object.UTF8), object.NewString(arg, object.UTF8)); err != nil {
				return object.Value{}, err
			}
		}
	}
	def("each", 0, each)
	def("each_option", 0, each)
	def("rest", 0, func(ctx object.Context, g *getopt, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return strings2(g.rest), nil
	})
	def("terminate", 0, func(ctx object.Context, g *getopt, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		g.rest = append(g.rest, g.args...)
		g.args = nil
		g.terminated = true
		return self, nil
	})
	def("terminated?", 0, func(ctx object.Context, g *getopt, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(g.terminated), nil
	})
	def("quiet", 0, func(ctx object.Context, g *getopt, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(g.quiet), nil
	})
	def("quiet?", 0, func(ctx object.Context, g *getopt, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(g.quiet), nil
	})
	def("quiet=", 1, func(ctx object.Context, g *getopt, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		g.quiet = args[0].Truthy()
		return self, nil
	})
	def("ordering", 0, func(ctx object.Context, g *getopt, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(g.ordering), nil
	})
	def("ordering=", 1, func(ctx object.Context, g *getopt, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if g.started {
			return object.Value{}, ctx.Raise(object.RuntimeError, "invoke ordering=, but option processing has already started")
		}
		n, err := stdlib.WantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if n < requireOrder || n > returnInOrder {
			return object.Value{}, ctx.Raise(object.ArgumentError, "invalid ordering `%d'", n)
		}
		g.ordering = int(n)
		return self, nil
	})
	// error is the class of the last error, nil for none; error? is the
	// same, as in Ruby.
	for _, name := range []string{"error", "error?"} {
		def(name, 0, func(ctx object.Context, g *getopt, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return g.errorCls, nil
		})
	}
	def("error_message", 0, func(ctx object.Context, g *getopt, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return g.errorMsg, nil
	})
	def("inspect", 0, func(ctx object.Context, g *getopt, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewString("#<GetoptLong>", object.ASCII), nil
	})
}
