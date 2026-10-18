package amber

import (
	"errors"
	"fmt"

	"github.com/radiospiel/amber/src/object"
)

// Module is a module the host gives the script: a constant at its top
// level, as Math is, whose methods the host implements in Go. A script
// cannot redefine it.
type Module struct {
	p   *Program
	cls *object.Class
}

// Func is a host method. It gets the call's positional arguments and its
// keyword arguments as plain Go values: what ToGo gives, but a Symbol is a
// String, as everywhere a value leaves amber, and a Hash whose keys are all
// Strings or Symbols is a map[string]any. What it returns comes back
// through ToValue. An error raises an exception: an *Exception's class, or
// a RuntimeError with the error's text.
//
// Workers that async started call host methods on goroutines of their
// own, so a Func may run on several at once. RunContext stops the script
// only once a host method returns: one that blocks watches the context
// the host runs the script with.
type Func func(args []any, kwargs map[string]any) (any, error)

// Module defines the module name for the script, or returns it when it is
// defined already. Define modules after Load and before Run.
func (p *Program) Module(name string) *Module {
	if v, ok := p.in.Const(name); ok {
		if cls, ok := v.AsClass(); ok && cls.IsModule && cls.Digest == "" {
			return &Module{p: p, cls: cls}
		}
	}
	cls := object.NewModule(name)
	p.in.SetConst(name, object.ClassValue(cls))
	return &Module{p: p, cls: cls}
}

// Def defines the method name on the module: the script calls it as
// Module.name(args).
func (m *Module) Def(name string, fn Func) {
	m.cls.DefineSingleton(name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		in := make([]any, len(args))
		for i, a := range args {
			g, err := m.p.toHost(a)
			if err != nil {
				return object.Value{}, ctx.Raise(object.ArgumentError, "%s", err)
			}
			in[i] = g
		}
		kwargs := map[string]any{}
		if kw != nil {
			for i, k := range kw.Keys() {
				g, err := m.p.toHost(kw.Values()[i])
				if err != nil {
					return object.Value{}, ctx.Raise(object.ArgumentError, "%s", err)
				}
				kwargs[k.Str()] = g
			}
		}
		out, err := fn(in, kwargs)
		if err != nil {
			var exc *Exception
			if errors.As(err, &exc) {
				return object.Value{}, ctx.Raise(exc.cls, "%s", exc.Message)
			}
			return object.Value{}, ctx.Raise(object.RuntimeError, "%s", err)
		}
		v, err := m.p.ToValue(out)
		if err != nil {
			return object.Value{}, ctx.Raise(object.TypeError, "%s.%s returned %s", m.cls.Name, name, err)
		}
		return v, nil
	})
}

// Exception defines the exception class name in the module, a
// StandardError, that the script rescues as Module::name. Its New makes the
// error a host method returns to raise it.
func (m *Module) Exception(name string) *ExceptionClass {
	cls := object.NewClass(name, object.StandardError)
	cls.Lexical = m.cls
	m.cls.Consts[name] = object.ClassValue(cls)
	return &ExceptionClass{cls: cls}
}

// ExceptionClass is an exception class a host module defines.
type ExceptionClass struct{ cls *object.Class }

// New is an error that raises the class, with the message, when a host
// method returns it.
func (c *ExceptionClass) New(format string, args ...any) *Exception {
	return &Exception{cls: c.cls, Message: fmt.Sprintf(format, args...)}
}

// Exception is an exception a host method raises.
type Exception struct {
	cls     *object.Class
	Message string
}

func (e *Exception) Error() string {
	return e.Message + " (" + e.cls.QualifiedName() + ")"
}

// toHost converts a value for a host method; see Func.
func (p *Program) toHost(v object.Value) (any, error) {
	switch v.Kind() {
	case object.KindSymbol:
		return v.Str(), nil
	case object.KindList:
		elems := v.List().Elems
		out := make([]any, len(elems))
		for i, e := range elems {
			g, err := p.toHost(e)
			if err != nil {
				return nil, err
			}
			out[i] = g
		}
		return out, nil
	case object.KindSet:
		elems := v.Set().Elems()
		out := make([]any, len(elems))
		for i, e := range elems {
			g, err := p.toHost(e)
			if err != nil {
				return nil, err
			}
			out[i] = g
		}
		return out, nil
	case object.KindMap:
		m := v.Map()
		named := true
		for _, k := range m.Keys() {
			_, str := k.AsString()
			_, sym := k.AsSymbol()
			named = named && (str || sym)
		}
		if named {
			out := make(map[string]any, m.Len())
			for i, k := range m.Keys() {
				g, err := p.toHost(m.Values()[i])
				if err != nil {
					return nil, err
				}
				out[k.Str()] = g
			}
			return out, nil
		}
		out := make(map[any]any, m.Len())
		for i, k := range m.Keys() {
			key, err := p.toHost(k)
			if err != nil {
				return nil, err
			}
			switch key.(type) {
			case []any, map[string]any, map[any]any:
				return nil, fmt.Errorf("a Hash with %s keys cannot leave amber", k.Class().QualifiedName())
			}
			g, err := p.toHost(m.Values()[i])
			if err != nil {
				return nil, err
			}
			out[key] = g
		}
		return out, nil
	}
	return p.ToGo(v)
}
