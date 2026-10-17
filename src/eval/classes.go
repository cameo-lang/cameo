package eval

import (
	"github.com/radiospiel/amber/src/object"
	"github.com/radiospiel/amber/src/syntax"
)

func (in *Interp) readIVar(name string, fr *Frame, line int) (object.Value, error) {
	inst, ok := fr.Self.AsInstance()
	if !ok {
		return object.Value{}, in.raise(object.NameError, line, "@%s: %s has no fields", name, typeName(fr.Self))
	}
	v, ok := inst.Field(name)
	if !ok {
		return object.Value{}, in.raise(object.NameError, line, "@%s: %s has no field %s", name, inst.Cls.QualifiedName(), name)
	}
	if v.IsZero() {
		return object.Nil, nil
	}
	return v, nil
}

func (in *Interp) writeIVar(name string, v object.Value, fr *Frame, line int) error {
	inst := fr.Building
	if inst == nil {
		return in.raise(object.AmberError, line, "@%s = …: a field is set in initialize only", name)
	}
	idx, ok := inst.Cls.FieldIndex[name]
	if !ok {
		return in.raise(object.NameError, line, "@%s = …: %s has no field %s", name, inst.Cls.QualifiedName(), name)
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
	in.constTable(fr)[name] = v
	return nil
}

// constTable is where a definition in fr binds its name: the innermost
// lexical class's constants, or the interpreter's top-level ones.
func (in *Interp) constTable(fr *Frame) map[string]object.Value {
	if len(fr.Lexical) > 0 {
		return fr.Lexical[len(fr.Lexical)-1].Consts
	}
	return in.consts
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
	if existing, ok := table[n.Name]; ok && existing.Owner == target && existing.Body != nil {
		if target == in.mainClass {
			return object.Value{}, in.raise(object.AmberError, n.Line(), "method %s is already defined at the top level", n.Name)
		}
		return object.Value{}, in.raise(object.AmberError, n.Line(), "method %s is already defined in %s", n.Name, target.QualifiedName())
	}
	if existing, ok := table[n.Name]; ok && existing.Native != nil && existing.Owner == target {
		if _, isField := target.FieldIndex[n.Name]; isField {
			return object.Value{}, in.raise(object.AmberError, n.Line(), "def %s: %s is a field of %s, and fields read themselves", n.Name, n.Name, target.QualifiedName())
		}
		return object.Value{}, in.raise(object.AmberError, n.Line(), "cannot replace the builtin method %s of %s", n.Name, target.Name)
	}
	m := &object.Method{Name: n.Name, Owner: target, Body: &compiledDef{def: n, body: in.code.body(n, n.Body)}, Arity: -1, Lexical: fr.Lexical}
	if n.Singleton {
		target.SetSingletonMethod(m)
	} else {
		target.SetMethod(m)
	}
	return object.Symbol(n.Name), nil
}

// evalClassDef binds a class to its name. The same definition (by digest)
// evaluated again returns the class already bound; a different one makes a
// new class and rebinds the name, leaving the old class to its instances.
func (in *Interp) evalClassDef(n *syntax.ClassDef, env *Env, fr *Frame) (object.Value, error) {
	table := in.constTable(fr)
	super := object.ObjectClass
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
	if existing, ok := table[n.Name].AsClass(); ok {
		if existing.Digest == n.Digest && existing.Super == super {
			return object.ClassValue(existing), nil
		}
	}
	if _, builtin := object.ObjectClass.Consts[n.Name]; builtin && len(fr.Lexical) == 0 {
		return object.Value{}, in.raise(object.AmberError, n.Line(), "cannot redefine the builtin class %s", n.Name)
	}
	cls := object.NewClass(n.Name, super)
	cls.Digest = n.Digest
	if len(fr.Lexical) > 0 {
		cls.Lexical = fr.Lexical[len(fr.Lexical)-1]
	}
	if err := in.inheritFields(cls, super); err != nil {
		return object.Value{}, withLine(err, n.Line())
	}
	if err := in.evalClassBody(cls, n.Body, env, fr); err != nil {
		return object.Value{}, err
	}
	table[n.Name] = object.ClassValue(cls)
	return object.ClassValue(cls), nil
}

func (in *Interp) evalModuleDef(n *syntax.ModuleDef, env *Env, fr *Frame) (object.Value, error) {
	table := in.constTable(fr)
	if existing, ok := table[n.Name].AsClass(); ok && existing.IsModule {
		if existing.Digest == n.Digest {
			return object.ClassValue(existing), nil
		}
	}
	if _, builtin := object.ObjectClass.Consts[n.Name]; builtin && len(fr.Lexical) == 0 {
		return object.Value{}, in.raise(object.AmberError, n.Line(), "cannot redefine the builtin module %s", n.Name)
	}
	mod := object.NewModule(n.Name)
	mod.Digest = n.Digest
	if len(fr.Lexical) > 0 {
		mod.Lexical = fr.Lexical[len(fr.Lexical)-1]
	}
	if err := in.evalClassBody(mod, n.Body, env, fr); err != nil {
		return object.Value{}, err
	}
	table[n.Name] = object.ClassValue(mod)
	return object.ClassValue(mod), nil
}

// evalClassBody runs a class or module body with the class as self and as
// the target of def, and with the class added to the lexical scopes.
func (in *Interp) evalClassBody(cls *object.Class, body []syntax.Node, env *Env, fr *Frame) error {
	lexical := append(append([]*object.Class{}, fr.Lexical...), cls)
	bodyFrame := &Frame{Self: object.ClassValue(cls), Lexical: lexical, DefTarget: cls}
	_, err := in.evalBody(body, NewEnv(nil), bodyFrame)
	return err
}
