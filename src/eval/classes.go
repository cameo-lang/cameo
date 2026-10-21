package eval

import (
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/syntax"
)

// readField reads a field of self, as `self.name` does.
func (in *Interp) readField(name string, fr *Frame, line int) (object.Value, error) {
	inst, ok := fr.Self.AsInstance()
	if !ok {
		return object.Value{}, in.raise(object.NameError, line, "self.%s: %s has no fields", name, typeName(fr.Self))
	}
	v, ok := inst.Field(name)
	if !ok {
		return object.Value{}, in.raise(object.NameError, line, "self.%s: %s has no field %s", name, inst.Cls.QualifiedName(), name)
	}
	if v.IsZero() {
		return object.Nil, nil
	}
	return v, nil
}

// writeField sets a field of the instance initialize is building, as
// `self.name = v` does.
func (in *Interp) writeField(name string, v object.Value, fr *Frame, line int) error {
	inst := fr.Building
	if inst == nil {
		return in.raise(object.CameoError, line, "self.%s = …: a field is set in initialize only", name)
	}
	idx, ok := inst.Cls.FieldIndex[name]
	if !ok {
		return in.raise(object.NameError, line, "self.%s = …: %s has no field %s", name, inst.Cls.QualifiedName(), name)
	}
	checked, err := in.checkField(inst.Cls.Fields[idx], v, line)
	if err != nil {
		return err
	}
	inst.Fields[idx] = checked
	return nil
}

// lookupConst finds a constant: in the lexical scopes innermost first, then
// along the innermost class's ancestors, then at the top level.
func (in *Interp) lookupConst(name string, fr *Frame, line int) (object.Value, error) {
	for i := len(fr.Lexical) - 1; i >= 0; i-- {
		if v, ok := fr.Lexical[i].Consts[name]; ok {
			return v, nil
		}
	}
	if len(fr.Lexical) > 0 {
		for _, a := range fr.Lexical[len(fr.Lexical)-1].Ancestors() {
			if v, ok := a.Consts[name]; ok {
				return v, nil
			}
		}
	}
	if v, ok := in.consts[name]; ok {
		return v, nil
	}
	if v, ok := object.ObjectClass.Consts[name]; ok {
		return v, nil
	}
	return object.Value{}, in.raise(object.NameError, line, "uninitialized constant %s", name)
}

func (in *Interp) evalScopedConst(n *syntax.ScopedConst, env *Env, fr *Frame) (object.Value, error) {
	if n.Scope == nil {
		if v, ok := in.consts[n.Name]; ok {
			return v, nil
		}
		if v, ok := object.ObjectClass.Consts[n.Name]; ok {
			return v, nil
		}
		return object.Value{}, in.raise(object.NameError, n.Line(), "uninitialized constant %s", n.Name)
	}
	v, err := in.eval(n.Scope, env, fr)
	if err != nil {
		return object.Value{}, err
	}
	scope, ok := v.AsClass()
	if !ok {
		return object.Value{}, in.raise(object.TypeError, n.Line(), "%s is not a class/module", typeName(v))
	}
	for _, a := range scope.Ancestors() {
		if v, ok := a.Consts[n.Name]; ok {
			return v, nil
		}
	}
	return object.Value{}, in.raise(object.NameError, n.Line(), "uninitialized constant %s::%s", scope.QualifiedName(), n.Name)
}

// defineConst binds a constant in the innermost lexical scope. A constant
// is a value bound to a name: binding it again replaces the binding.
func (in *Interp) defineConst(name string, v object.Value, fr *Frame, line int) error {
	if _, builtin := object.ObjectClass.Consts[name]; builtin && len(fr.Lexical) == 0 {
		return in.raise(object.CameoError, line, "cannot reassign the builtin constant %s", name)
	}
	// A class from Class.new takes the name of the first constant it is
	// bound to, as in Ruby.
	if cls, ok := v.AsClass(); ok && cls.Name == "" {
		cls.Name = name
		if len(fr.Lexical) > 0 {
			cls.Lexical = fr.Lexical[len(fr.Lexical)-1]
		}
	}
	in.bindConst(fr, name, v)
	return nil
}

// newClass is Class.new(superclass = Object) { body }: a class built at
// run time, its block run as the class body with the class as self and as
// the target of def, closing over the block's locals. It is identified by
// its block's digest, and named by the first constant it is bound to.
func (in *Interp) newClass(args []object.Value, block object.Value) (object.Value, error) {
	if len(args) > 1 {
		return object.Value{}, in.raise(object.ArgumentError, 0, "wrong number of arguments (given %d, expected 0..1)", len(args))
	}
	super := object.ObjectClass
	if len(args) == 1 {
		s, ok := args[0].AsClass()
		if !ok || s.IsModule {
			return object.Value{}, in.raise(object.TypeError, 0, "superclass must be a Class (%s given)", typeName(args[0]))
		}
		super = s
	}
	cls := object.NewClass("", super)
	cls.Digest = "new:" + super.QualifiedName()
	if err := in.inheritFields(cls, super); err != nil {
		return object.Value{}, err
	}
	if block.IsZero() {
		return object.ClassValue(cls), nil
	}
	p, ok := asProc(block)
	if !ok || p.Block == nil {
		return object.Value{}, in.raise(object.TypeError, 0, "Class.new takes a literal block")
	}
	cls.Digest = "new:" + syntax.Digest(p.Block)
	env := NewEnv(p.Env)
	if len(p.Block.Params.Positional) > 0 {
		env.Define(p.Block.Params.Positional[0].Name, object.ClassValue(cls))
	}
	lexical := append(append([]*object.Class{}, p.Frame.Lexical...), cls)
	bodyFrame := &Frame{Self: object.ClassValue(cls), Lexical: lexical, DefTarget: cls}
	if _, err := in.evalBody(p.Block.Body, env, bodyFrame); err != nil {
		return object.Value{}, err
	}
	return object.ClassValue(cls), nil
}

// constTable is where a definition in fr binds its name: the innermost
// lexical class's constants, or the interpreter's top-level ones.
func (in *Interp) constTable(fr *Frame) map[string]object.Value {
	if len(fr.Lexical) > 0 {
		return fr.Lexical[len(fr.Lexical)-1].Consts
	}
	return in.consts
}

// bindConst binds name to v where a definition in fr binds its name.
func (in *Interp) bindConst(fr *Frame, name string, v object.Value) {
	in.constTable(fr)[name] = v
	if len(fr.Lexical) == 0 {
		in.changedTop()
	}
}

func (in *Interp) evalDef(n *syntax.Def, fr *Frame) (object.Value, error) {
	target := fr.DefTarget
	if target == nil {
		target = in.mainClass
	}
	table := target.Methods
	if n.Singleton {
		table = target.SingletonMethods
	}
	// A second def of a name replaces the first, as in Ruby. The same
	// definition evaluated again, as a loop around it does, changes nothing,
	// and leaves the method caches as they are.
	if existing, ok := table[n.Name]; ok && existing.Owner == target && existing.Body != nil {
		if cd, ok := existing.Body.(*compiledDef); ok && cd.def == n {
			return object.Symbol(n.Name), nil
		}
	}
	if existing, ok := table[n.Name]; ok && existing.Native != nil && existing.Owner == target {
		if _, isField := target.FieldIndex[n.Name]; isField {
			return object.Value{}, in.raise(object.CameoError, n.Line(), "def %s: %s is a field of %s, and fields read themselves", n.Name, n.Name, target.QualifiedName())
		}
		return object.Value{}, in.raise(object.CameoError, n.Line(), "cannot replace the builtin method %s of %s", n.Name, target.Name)
	}
	m := &object.Method{Name: n.Name, Owner: target, Body: &compiledDef{def: n, body: in.code.body(n, n.Body)}, Arity: -1, Lexical: fr.Lexical}
	if n.Singleton {
		target.SetSingletonMethod(m)
	} else {
		target.SetMethod(m)
	}
	if target == in.mainClass {
		in.changedTop()
	}
	return object.Symbol(n.Name), nil
}

// evalClassDef binds a class to its name and runs its body. A class
// already bound to the name is reopened: the body runs on a copy, which
// replaces it, so instances made before keep the class they were made
// with. The definition that last ran on the class, evaluated again,
// returns the class as it is. The name is bound to the class while its
// body runs, as in Ruby, and back to what it was when the body raises.
func (in *Interp) evalClassDef(n *syntax.ClassDef, env *Env, fr *Frame) (object.Value, error) {
	table := in.constTable(fr)
	var super *object.Class
	if n.Super != nil {
		v, err := in.eval(n.Super, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		cls, ok := v.AsClass()
		if !ok || cls.IsModule {
			return object.Value{}, in.raise(object.TypeError, n.Line(), "superclass must be a Class (%s given)", typeName(v))
		}
		super = cls
	}
	previous, bound := table[n.Name]
	existing, _ := previous.AsClass()
	if existing != nil && !existing.IsModule && ranLast(existing, n.Digest) && (super == nil || existing.Super == super) {
		return object.ClassValue(existing), nil
	}
	if _, builtin := object.ObjectClass.Consts[n.Name]; builtin && len(fr.Lexical) == 0 {
		return object.Value{}, in.raise(object.CameoError, n.Line(), "cannot redefine the builtin class %s", n.Name)
	}
	if hostDefined(previous) {
		return object.Value{}, in.raise(object.CameoError, n.Line(), "cannot redefine the module %s, which the host defines", n.Name)
	}
	var cls *object.Class
	switch {
	case existing != nil && existing.IsModule:
		return object.Value{}, in.raise(object.TypeError, n.Line(), "%s is not a class", n.Name)
	case existing != nil:
		if super != nil && super != existing.Super {
			return object.Value{}, in.raise(object.TypeError, n.Line(), "superclass mismatch for class %s", n.Name)
		}
		cls = existing.Copy()
		cls.Digest = existing.Digest + "+" + n.Digest
	default:
		if super == nil {
			super = object.ObjectClass
		}
		cls = object.NewClass(n.Name, super)
		cls.Digest = n.Digest
		if len(fr.Lexical) > 0 {
			cls.Lexical = fr.Lexical[len(fr.Lexical)-1]
		}
		if err := in.inheritFields(cls, super); err != nil {
			return object.Value{}, withLine(err, n.Line())
		}
	}
	if err := in.evalBound(cls, existing, n.Body, env, fr, previous, bound); err != nil {
		return object.Value{}, err
	}
	return object.ClassValue(cls), nil
}

// evalModuleDef binds a module to its name and runs its body, reopening
// a module already bound to the name as evalClassDef reopens a class.
func (in *Interp) evalModuleDef(n *syntax.ModuleDef, env *Env, fr *Frame) (object.Value, error) {
	table := in.constTable(fr)
	previous, bound := table[n.Name]
	existing, _ := previous.AsClass()
	if existing != nil && existing.IsModule && ranLast(existing, n.Digest) {
		return object.ClassValue(existing), nil
	}
	if _, builtin := object.ObjectClass.Consts[n.Name]; builtin && len(fr.Lexical) == 0 {
		return object.Value{}, in.raise(object.CameoError, n.Line(), "cannot redefine the builtin module %s", n.Name)
	}
	if hostDefined(previous) {
		return object.Value{}, in.raise(object.CameoError, n.Line(), "cannot redefine the module %s, which the host defines", n.Name)
	}
	var mod *object.Class
	switch {
	case existing != nil && !existing.IsModule:
		return object.Value{}, in.raise(object.TypeError, n.Line(), "%s is not a module", n.Name)
	case existing != nil:
		mod = existing.Copy()
		mod.Digest = existing.Digest + "+" + n.Digest
	default:
		mod = object.NewModule(n.Name)
		mod.Digest = n.Digest
		if len(fr.Lexical) > 0 {
			mod.Lexical = fr.Lexical[len(fr.Lexical)-1]
		}
	}
	if err := in.evalBound(mod, existing, n.Body, env, fr, previous, bound); err != nil {
		return object.Value{}, err
	}
	return object.ClassValue(mod), nil
}

// ranLast reports whether the definition whose digest is given is the
// last one that ran on cls: its first, or the last that reopened it.
func ranLast(cls *object.Class, digest string) bool {
	return cls.Digest == digest || strings.HasSuffix(cls.Digest, "+"+digest)
}

// evalBound binds cls to its name and runs its body, with the fields cls
// had from reopened as fields the body may declare again. When the body
// raises, the name is bound to previous again, or to nothing when it
// was not bound.
func (in *Interp) evalBound(cls, reopened *object.Class, body []syntax.Node, env *Env, fr *Frame, previous object.Value, bound bool) error {
	in.bindConst(fr, cls.Name, object.ClassValue(cls))
	if err := in.evalClassBody(cls, reopened, body, env, fr); err != nil {
		if bound {
			in.bindConst(fr, cls.Name, previous)
		} else {
			delete(in.constTable(fr), cls.Name)
			if len(fr.Lexical) == 0 {
				in.changedTop()
			}
		}
		return err
	}
	return nil
}

// hostDefined reports whether v is a module the host defined: a class
// value without the digest every definition in a script has.
func hostDefined(v object.Value) bool {
	cls, ok := v.AsClass()
	return ok && cls.IsModule && cls.Digest == ""
}

// evalClassBody runs a class or module body with the class as self and as
// the target of def, and with the class added to the lexical scopes.
// reopened is the class cls is a copy of, or nil.
func (in *Interp) evalClassBody(cls, reopened *object.Class, body []syntax.Node, env *Env, fr *Frame) error {
	lexical := append(append([]*object.Class{}, fr.Lexical...), cls)
	bodyFrame := &Frame{Self: object.ClassValue(cls), Lexical: lexical, DefTarget: cls, Reopened: reopened}
	_, err := in.evalBody(body, NewEnv(nil), bodyFrame)
	return err
}

// defined is `defined?(expr)`: what kind of thing expr names, in Ruby's
// words, or nil when it names nothing. It evaluates no more than it must:
// a constant to see that it resolves, a call's receiver but not the call.
func (in *Interp) defined(n syntax.Node, env *Env, fr *Frame) (object.Value, error) {
	missing := func(err error) bool {
		exc, ok := err.(*object.Exception)
		return ok && exc.Cls.IsA(object.NameError)
	}
	switch n := n.(type) {
	case *syntax.Var:
		if _, ok := env.Get(n.Name); ok {
			return object.String("local-variable"), nil
		}
		return object.Nil, nil
	case *syntax.Const, *syntax.ScopedConst:
		if _, err := in.eval(n, env, fr); err != nil {
			if missing(err) {
				return object.Nil, nil
			}
			return object.Value{}, err
		}
		return object.String("constant"), nil
	case *syntax.Call:
		recv := fr.Self
		if n.Recv != nil {
			v, err := in.eval(n.Recv, env, fr)
			if err != nil {
				if _, ok := err.(*object.Exception); ok {
					return object.Nil, nil
				}
				return object.Value{}, err
			}
			recv = v
		}
		if recv.Class().Lookup(n.Name) != nil {
			return object.String("method"), nil
		}
		return object.Nil, nil
	case *syntax.Yield:
		if fr.Block.IsZero() {
			return object.Nil, nil
		}
		return object.String("yield"), nil
	case *syntax.Self:
		return object.String("self"), nil
	case *syntax.Assign, *syntax.FieldAssign, *syntax.ConstAssign, *syntax.MultiAssign:
		return object.String("assignment"), nil
	}
	return object.String("expression"), nil
}

// scopedFrame is the frame `class A::B` defines B in: fr with module A as
// its innermost lexical scope, so B becomes A's constant. A nil scope
// leaves fr as it is.
func (in *Interp) scopedFrame(scope syntax.Node, env *Env, fr *Frame, line int) (*Frame, error) {
	if scope == nil {
		return fr, nil
	}
	v, err := in.eval(scope, env, fr)
	if err != nil {
		return nil, err
	}
	mod, ok := v.AsClass()
	if !ok {
		return nil, in.raise(object.TypeError, line, "%s is not a class/module", typeName(v))
	}
	// A builtin module, or a module or class the host registered, is
	// shared by every program in the process: nothing is defined in it.
	if mod.Digest == "" || strings.HasPrefix(mod.Digest, "host:") {
		return nil, in.raise(object.CameoError, line, "cannot define anything inside %s, which the script did not define", mod.QualifiedName())
	}
	out := *fr
	out.Lexical = append(append([]*object.Class{}, fr.Lexical...), mod)
	return &out, nil
}

// evalAlias gives a method a second name, in the class being defined or at
// the top level. The new name must be free: a method is defined once.
func (in *Interp) evalAlias(n *syntax.Alias, fr *Frame) (object.Value, error) {
	target := fr.DefTarget
	m := target.Lookup(n.Old)
	if m == nil {
		return object.Value{}, in.raise(object.NameError, n.Line(), "undefined method '%s' for %s", n.Old, target.QualifiedName())
	}
	if _, taken := target.Methods[n.New]; taken {
		return object.Value{}, in.raise(object.CameoError, n.Line(), "method %s is already defined in %s", n.New, target.QualifiedName())
	}
	alias := *m
	alias.Name = n.New
	target.SetMethod(&alias)
	if target == in.mainClass {
		in.changedTop()
	}
	return object.Nil, nil
}
