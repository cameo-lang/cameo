package eval

import (
	"fmt"
	"math"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/syntax"
)

func (in *Interp) evalCall(n *syntax.Call, env *Env, fr *Frame) (object.Value, error) {
	recv := fr.Self
	selfCall := n.Recv == nil
	if selfCall && n.Name == "block_given?" && len(n.Args) == 0 {
		return object.Bool(!fr.Block.IsZero()), nil
	}
	if n.Recv != nil {
		v, err := in.eval(n.Recv, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		recv = v
		if n.Safe && recv.IsNil() {
			return object.Nil, nil
		}
	}
	args, kw, err := in.evalCallArgs(n.Args, n.KwArgs, env, fr, n.Line())
	if err != nil {
		return object.Value{}, err
	}
	var block object.Value
	switch {
	case n.Block != nil:
		block = procValue(&Proc{Block: n.Block, Env: env, Frame: fr})
	case n.BlockArg != nil:
		v, err := in.eval(n.BlockArg, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		block, err = in.toProc(v, n.Line())
		if err != nil {
			return object.Value{}, err
		}
	}
	v, err := in.call(fr, recv, n.Name, args, kw, block, selfCall, n.Line())
	return v, withLine(err, n.Line())
}

// binaryOp returns the fast path for an arithmetic or comparison operator
// on two integers or two floats, or nil for any other name. The
// fast path returns nil when the operands are not both integers or both
// floats, and the call goes the ordinary way. It is sound because Integer
// and Float are builtin classes, which a script can neither reopen nor
// replace a method of, so `+` on an integer can only ever mean this. The
// compiler reaches it after its raw integer path (compile.go).
func binaryOp(name string) func(a, b object.Value) object.Value {
	switch name {
	case "+":
		return func(a, b object.Value) object.Value {
			if x, y, ok := ints(a, b); ok {
				if r, ok := object.AddInt(x, y); ok {
					return object.Int(r)
				}
				return object.Value{}
			}
			if x, y, ok := floats(a, b); ok {
				return object.Float(x + y)
			}
			return object.Value{}
		}
	case "-":
		return func(a, b object.Value) object.Value {
			if x, y, ok := ints(a, b); ok {
				if r, ok := object.SubInt(x, y); ok {
					return object.Int(r)
				}
				return object.Value{}
			}
			if x, y, ok := floats(a, b); ok {
				return object.Float(x - y)
			}
			return object.Value{}
		}
	case "*":
		return func(a, b object.Value) object.Value {
			if x, y, ok := ints(a, b); ok {
				if r, ok := object.MulInt(x, y); ok {
					return object.Int(r)
				}
				return object.Value{}
			}
			if x, y, ok := floats(a, b); ok {
				return object.Float(x * y)
			}
			return object.Value{}
		}
	case "/":
		// Integer division floors, as Ruby's does; a zero divisor, the one
		// quotient that overflows and the floats go the ordinary way.
		return func(a, b object.Value) object.Value {
			if x, y, ok := ints(a, b); ok && y != 0 && (y != -1 || x != math.MinInt64) {
				q := x / y
				if x%y != 0 && (x < 0) != (y < 0) {
					q--
				}
				return object.Int(q)
			}
			return object.Value{}
		}
	case "%":
		return func(a, b object.Value) object.Value {
			if x, y, ok := ints(a, b); ok && y != 0 {
				m := x % y
				if m != 0 && (m < 0) != (y < 0) {
					m += y
				}
				return object.Int(m)
			}
			return object.Value{}
		}
	case "<":
		return func(a, b object.Value) object.Value {
			if x, y, ok := ints(a, b); ok {
				return object.Bool(x < y)
			}
			if x, y, ok := floats(a, b); ok {
				return object.Bool(x < y)
			}
			return object.Value{}
		}
	case "<=":
		return func(a, b object.Value) object.Value {
			if x, y, ok := ints(a, b); ok {
				return object.Bool(x <= y)
			}
			if x, y, ok := floats(a, b); ok {
				return object.Bool(x <= y)
			}
			return object.Value{}
		}
	case ">":
		return func(a, b object.Value) object.Value {
			if x, y, ok := ints(a, b); ok {
				return object.Bool(x > y)
			}
			if x, y, ok := floats(a, b); ok {
				return object.Bool(x > y)
			}
			return object.Value{}
		}
	case ">=":
		return func(a, b object.Value) object.Value {
			if x, y, ok := ints(a, b); ok {
				return object.Bool(x >= y)
			}
			if x, y, ok := floats(a, b); ok {
				return object.Bool(x >= y)
			}
			return object.Value{}
		}
	case "==":
		return func(a, b object.Value) object.Value {
			if x, y, ok := ints(a, b); ok {
				return object.Bool(x == y)
			}
			if x, y, ok := floats(a, b); ok {
				return object.Bool(x == y)
			}
			return object.Value{}
		}
	case "!=":
		return func(a, b object.Value) object.Value {
			if x, y, ok := ints(a, b); ok {
				return object.Bool(x != y)
			}
			if x, y, ok := floats(a, b); ok {
				return object.Bool(x != y)
			}
			return object.Value{}
		}
	}
	return nil
}

func ints(a, b object.Value) (int64, int64, bool) {
	x, ok := a.AsInt()
	if !ok {
		return 0, 0, false
	}
	y, ok := b.AsInt()
	return int64(x), int64(y), ok
}

func floats(a, b object.Value) (float64, float64, bool) {
	x, ok := a.AsFloat()
	if !ok {
		return 0, 0, false
	}
	y, ok := b.AsFloat()
	return float64(x), float64(y), ok
}

// toProc turns a &argument into a block: a proc as it is, a symbol into a
// proc calling that method, nil into no block.
func (in *Interp) toProc(v object.Value, line int) (object.Value, error) {
	switch v.Kind() {
	case object.KindProc:
		v := proc(v)
		return procValue(v), nil
	case object.KindSymbol:
		v := v.Str()
		return procValue(&Proc{Sym: v}), nil
	case object.KindMap:
		return hashProc(v), nil
	}
	if v.IsNil() {
		return object.Value{}, nil
	}
	return object.Value{}, in.raise(object.TypeError, line, "wrong argument type %s (expected Proc)", v.Class().Name)
}

// Call implements object.Context.
func (in *Interp) Call(recv object.Value, name string, args ...object.Value) (object.Value, error) {
	return in.call(nil, recv, name, args, nil, object.Value{}, false, 0)
}

// CallTop invokes a method the way a bare call in the script would: on the
// top-level self, reaching the script's top-level methods.
func (in *Interp) CallTop(name string, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	return in.call(nil, object.InstanceValue(in.main), name, args, kw, block, true, 0)
}

// CallWith implements object.Context.
func (in *Interp) CallWith(recv object.Value, name string, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	return in.call(nil, recv, name, args, kw, block, false, 0)
}

// RespondsTo implements object.Context.
func (in *Interp) RespondsTo(v object.Value, name string) bool {
	return in.findMethod(v, name) != nil
}

// findMethod resolves name on a receiver: class methods for a class, then
// the instance methods of its class.
func (in *Interp) findMethod(recv object.Value, name string) *object.Method {
	if cls, ok := recv.AsClass(); ok {
		if m := cls.LookupSingleton(name); m != nil {
			return m
		}
	}
	return recv.Class().Lookup(name)
}

// call dispatches name on recv. selfCall marks a call written without a
// receiver, which may reach a top-level method from anywhere.
func (in *Interp) call(fr *Frame, recv object.Value, name string, args []object.Value, kw *object.Map, block object.Value, selfCall bool, line int) (object.Value, error) {
	m := in.findMethod(recv, name)
	if m == nil && selfCall {
		m = in.mainClass.Methods[name]
		// Cameo, included at the top level, answers in every method.
		if m == nil && in.cameoTop {
			m = object.CameoModule.Lookup(name)
		}
	}
	if m == nil {
		if name == "initialize" {
			return object.Nil, nil
		}
		// A class's own method_missing takes a call to a method it lacks,
		// with the name as a symbol first; Object's raises.
		if mm := recv.Class().Lookup("method_missing"); mm != nil && mm.Native == nil {
			return in.invoke(mm, recv, append([]object.Value{object.Symbol(name)}, args...), kw, block, line)
		}
		return object.Value{}, in.raise(object.NoMethodError, line, "undefined method '%s' for %s", name, typeName(recv))
	}
	if m.Native != nil {
		if m.Arity >= 0 && len(args) != m.Arity {
			return object.Value{}, in.raise(object.ArgumentError, line, "wrong number of arguments (given %d, expected %d)", len(args), m.Arity)
		}
		v, err := m.Native(in, recv, args, kw, block)
		if err != nil {
			if c, ok := err.(*control); ok && c.kind == ctrlBreak && c.target != nil && c.target == blockTarget(block) {
				return c.value, nil
			}
			return object.Value{}, err
		}
		return v, nil
	}
	return in.invoke(m, recv, args, kw, block, line)
}

// invoke runs a user method.
func (in *Interp) invoke(m *object.Method, recv object.Value, args []object.Value, kw *object.Map, block object.Value, line int) (object.Value, error) {
	if err := in.stopped(); err != nil {
		return object.Value{}, err
	}
	cd := m.Body.(*compiledDef)
	def := cd.def
	act := &activation{frame: Frame{
		Self:    recv,
		Method:  m,
		Owner:   m.Owner,
		Args:    args,
		Kw:      kw,
		Block:   block,
		Lexical: m.Lexical,
	}}
	fr, env := &act.frame, &act.env
	if len(m.Lexical) > 0 {
		fr.DefTarget = m.Lexical[len(m.Lexical)-1]
	} else {
		fr.DefTarget = object.ObjectClass
	}
	if inst, ok := recv.AsInstance(); ok && def.Name == "initialize" {
		fr.Building = inst
	}
	if err := in.bindParams(&def.Params, args, kw, block, env, fr, true, m.Name, line); err != nil {
		return object.Value{}, err
	}
	v, err := cd.body(in, env, fr)
	if len(env.promises) > 0 {
		in.promises.end(env.promises)
	}
	if err != nil {
		if c, ok := err.(*control); ok {
			switch {
			case c.kind == ctrlReturn && c.target == fr:
				return c.value, nil
			case c.kind == ctrlBreak && c.target != nil && c.target == blockTarget(block):
				return c.value, nil
			}
		}
		return object.Value{}, err
	}
	return v, nil
}

// bindParams binds arguments to parameters in env. strict is a method or
// lambda, where the argument count must fit; a block is lenient.
func (in *Interp) bindParams(p *syntax.Params, args []object.Value, kw *object.Map, block object.Value, env *Env, fr *Frame, strict bool, name string, line int) error {
	// Keywords given to a method with no keyword parameters are one more
	// positional argument, a map.
	if kw != nil && len(p.Keywords) == 0 && p.KwRest == "" {
		args = append(append([]object.Value(nil), args...), object.MapValue(kw))
		kw = nil
	}
	required, optional, splat := 0, 0, -1
	for i, param := range p.Positional {
		switch {
		case param.Splat:
			splat = i
		case param.Default != nil:
			optional++
		default:
			required++
		}
	}
	if !strict && len(args) == 1 && len(p.Positional) > 1 {
		if l, ok := args[0].AsList(); ok {
			args = l.Elems
		}
	}
	if strict {
		if len(args) < required || (splat < 0 && len(args) > required+optional) {
			return in.raise(object.ArgumentError, line, "wrong number of arguments (given %d, expected %s)", len(args), expected(required, optional, splat >= 0))
		}
	}
	// How many optional parameters get a value: what is left after the
	// required ones.
	fill := len(args) - required
	if fill < 0 {
		fill = 0
	}
	if fill > optional {
		fill = optional
	}
	post := 0
	if splat >= 0 {
		for _, param := range p.Positional[splat+1:] {
			if !param.Splat && param.Default == nil {
				post++
			}
		}
	}
	next := 0
	take := func() object.Value {
		if next < len(args) {
			v := args[next]
			next++
			return v
		}
		return object.Nil
	}
	for i, param := range p.Positional {
		switch {
		case param.Splat:
			n := len(args) - next - post
			if n < 0 {
				n = 0
			}
			rest := append([]object.Value{}, args[next:next+n]...)
			next += n
			if param.Name != "" {
				env.Define(param.Name, object.NewList(rest...))
			}
		case param.Default != nil:
			if fill > 0 {
				fill--
				env.Define(param.Name, take())
			} else {
				v, err := in.eval(param.Default, env, fr)
				if err != nil {
					return err
				}
				env.Define(param.Name, v)
			}
		default:
			_ = i
			env.Define(param.Name, take())
		}
	}
	if len(p.Keywords) > 0 || p.KwRest != "" {
		used := map[string]bool{}
		for _, k := range p.Keywords {
			var v object.Value
			found := false
			if kw != nil {
				v, found = kw.Get(object.Symbol(k.Name))
			}
			switch {
			case found:
				env.Define(k.Name, v)
				used[k.Name] = true
			case k.Default != nil:
				d, err := in.eval(k.Default, env, fr)
				if err != nil {
					return err
				}
				env.Define(k.Name, d)
			case strict:
				return in.raise(object.ArgumentError, line, "missing keyword: :%s", k.Name)
			default:
				env.Define(k.Name, object.Nil)
			}
		}
		rest := object.NewMapBuilder()
		if kw != nil {
			for i, key := range kw.Keys() {
				s, isSym := key.AsSymbol()
				if isSym && used[string(s)] {
					continue
				}
				if p.KwRest == "" {
					if strict {
						if isSym {
							return in.raise(object.ArgumentError, line, "unknown keyword: :%s", s)
						}
						return in.raise(object.ArgumentError, line, "wrong number of arguments (given %d, expected %s)", len(args)+1, expected(required, optional, splat >= 0))
					}
					continue
				}
				rest.Set(key, kw.Values()[i])
			}
		}
		if p.KwRest != "" {
			env.Define(p.KwRest, object.MapValue(rest.Map()))
		}
	}
	if p.BlockParam != "" {
		if block.IsZero() {
			env.Define(p.BlockParam, object.Nil)
		} else {
			env.Define(p.BlockParam, block)
		}
	}
	return nil
}

func expected(required, optional int, splat bool) string {
	switch {
	case splat:
		return fmt.Sprintf("%d+", required)
	case optional > 0:
		return fmt.Sprintf("%d..%d", required, required+optional)
	}
	return fmt.Sprint(required)
}

// Yield implements object.Context.
func (in *Interp) Yield(block object.Value, args ...object.Value) (object.Value, error) {
	return in.callBlock(block, args, nil, object.Value{})
}

// Block makes a block that runs f with the arguments it is called with.
func (in *Interp) Block(f func(args []object.Value) (object.Value, error)) object.Value {
	return procValue(&Proc{Go: f})
}

// callBlock runs a block or lambda with arguments.
func (in *Interp) callBlock(block object.Value, args []object.Value, kw *object.Map, inner object.Value) (object.Value, error) {
	if err := in.stopped(); err != nil {
		return object.Value{}, err
	}
	p, ok := asProc(block)
	if !ok {
		return object.Value{}, in.raise(object.TypeError, 0, "no block given")
	}
	if p.Go != nil {
		return p.Go(args)
	}
	if p.Sym != "" {
		if !p.Recv.IsZero() {
			return in.call(nil, p.Recv, p.Sym, args, kw, inner, false, 0)
		}
		if len(args) == 0 {
			return object.Value{}, in.raise(object.ArgumentError, 0, "no receiver given")
		}
		return in.call(nil, args[0], p.Sym, args[1:], kw, inner, false, 0)
	}
	env := NewEnv(p.Env)
	fr := p.Frame
	if p.Strict {
		copy := *p.Frame
		copy.Lambda = true
		fr = &copy
	}
	if err := in.bindParams(&p.Block.Params, args, kw, inner, env, fr, p.Strict, "block", p.Block.Line()); err != nil {
		return object.Value{}, err
	}
	v, err := in.code.body(p.Block, p.Block.Body)(in, env, fr)
	if err != nil {
		if c, ok := err.(*control); ok {
			switch {
			case c.kind == ctrlNext && c.target == nil:
				return c.value, nil
			case c.kind == ctrlBreak && c.target == nil:
				if p.Strict {
					return c.value, nil
				}
				c.target = p
				return object.Value{}, c
			case c.kind == ctrlReturn && p.Strict && c.target == p.Frame:
				// A return inside a lambda body targets the lambda's own
				// frame copy; the body saw the copy, so this is unreachable
				// unless the return came from a nested block closed over
				// the original frame. Treat it as the lambda's return.
				return c.value, nil
			case c.kind == ctrlReturn && p.Strict && c.target == fr:
				return c.value, nil
			}
		}
		return object.Value{}, err
	}
	return v, nil
}

func (in *Interp) evalSuper(n *syntax.Super, env *Env, fr *Frame) (object.Value, error) {
	if fr.Method == nil {
		return object.Value{}, in.raise(object.LocalJumpError, n.Line(), "super called outside of method")
	}
	var args []object.Value
	var kw *object.Map
	var block object.Value = fr.Block
	if n.Forward {
		args, kw = fr.Args, fr.Kw
	} else {
		var err error
		args, kw, err = in.evalCallArgs(n.Args, n.KwArgs, env, fr, n.Line())
		if err != nil {
			return object.Value{}, err
		}
	}
	if n.Block != nil {
		block = procValue(&Proc{Block: n.Block, Env: env, Frame: fr})
	} else if n.BlockArg != nil {
		v, err := in.eval(n.BlockArg, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		if block, err = in.toProc(v, n.Line()); err != nil {
			return object.Value{}, err
		}
	}
	var m *object.Method
	if cls, ok := fr.Self.AsClass(); ok && fr.Method.Owner.SingletonMethods[fr.Method.Name] == fr.Method {
		for k := cls.Super; k != nil && m == nil; k = k.Super {
			m = k.LookupSingleton(fr.Method.Name)
		}
	} else {
		m = fr.Self.Class().LookupAfter(fr.Owner, fr.Method.Name)
	}
	if m == nil {
		if fr.Method.Name == "initialize" {
			return object.Nil, nil
		}
		return object.Value{}, in.raise(object.NoMethodError, n.Line(), "super: no superclass method '%s' for %s", fr.Method.Name, typeName(fr.Self))
	}
	if m.Native != nil {
		v, err := m.Native(in, fr.Self, args, kw, block)
		return v, withLine(err, n.Line())
	}
	v, err := in.invoke(m, fr.Self, args, kw, block, n.Line())
	return v, withLine(err, n.Line())
}

// blockTarget is what a break out of block targets: the proc itself, as
// callBlock records it on the control.
func blockTarget(block object.Value) any {
	if p, ok := asProc(block); ok {
		return p
	}
	return nil
}
