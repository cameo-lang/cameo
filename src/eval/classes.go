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

// writeField sets a field of the instance initialize is building, or an
// attribute of the class whose body is running, as `self.name = v` does.
func (in *Interp) writeField(name string, v object.Value, fr *Frame, line int) error {
	var cls *object.Class
	var fields []object.Value
	switch {
	case fr.Self.Kind() == object.KindClass:
		return in.writeAttr(name, v, fr, line)
	case fr.Building != nil:
		cls, fields = fr.Building.Cls, fr.Building.Fields
	case fr.BuildingExc != nil:
		cls, fields = fr.BuildingExc.Cls, fr.BuildingExc.Fields
	default:
		return in.raise(object.CameoError, line, "self.%s = …: a field is set in initialize, and a class attribute while the class body runs", name)
	}
	idx, ok := cls.FieldIndex[name]
	if !ok {
		return in.raise(object.NameError, line, "self.%s = …: %s has no field %s", name, cls.QualifiedName(), name)
	}
	checked, err := in.checkField(cls.Fields[idx], v, line)
	if err != nil {
		return err
	}
	fields[idx] = checked
	return nil
}

// writeAttr sets an attribute of the class self is, while its body runs:
// from the body itself, or from a class method the body calls. A class is
// sealed when its body ends, as an instance is when initialize returns.
func (in *Interp) writeAttr(name string, v object.Value, fr *Frame, line int) error {
	cls, _ := fr.Self.AsClass()
	if _, ok := cls.Attrs[name]; !ok {
		return in.raise(object.NameError, line, "self.%s = …: %s has no attribute %s", name, cls.QualifiedName(), name)
	}
	if !cls.Open {
		return in.raise(object.CameoError, line, "self.%s = …: %s is sealed: an attribute is set while the class body runs", name, cls.QualifiedName())
	}
	cls.Attrs[name] = v
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
			// Object's constants are the program's top level, whose own,
			// in.consts, come first: a module a library opened is the
			// program's copy there (libraryOpens).
			if a == object.ObjectClass {
				if v, ok := in.consts[name]; ok {
					return v, nil
				}
			}
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
	return object.Value{}, uninitialized(object.ObjectClass, name, name, line)
}

// uninitialized makes the NameError for a constant scope lacks, which
// the message names as shown.
func uninitialized(scope *object.Class, name, shown string, line int) error {
	return &object.Exception{Cls: object.NameError, Line: line, Message: "uninitialized constant " + shown,
		Attrs: map[string]object.Value{"name": object.Symbol(name), "receiver": object.ClassValue(scope)}}
}

func (in *Interp) evalScopedConst(n *syntax.ScopedConst, env *Env, fr *Frame) (object.Value, error) {
	if n.Scope == nil {
		return in.topConst(n.Name, n.Line())
	}
	v, err := in.eval(n.Scope, env, fr)
	if err != nil {
		return object.Value{}, err
	}
	return in.scopedConst(v, n.Name, n.Line())
}

// topConst is `::Name`: the top-level constant name.
func (in *Interp) topConst(name string, line int) (object.Value, error) {
	if v, ok := in.consts[name]; ok {
		return v, nil
	}
	if v, ok := object.ObjectClass.Consts[name]; ok {
		return v, nil
	}
	return object.Value{}, uninitialized(object.ObjectClass, name, name, line)
}

// scopedConst is `Scope::Name`: the constant name of the class or module
// v, or of one of its ancestors.
func (in *Interp) scopedConst(v object.Value, name string, line int) (object.Value, error) {
	scope, ok := v.AsClass()
	if !ok {
		return object.Value{}, in.raise(object.TypeError, line, "%s is not a class/module", typeName(v))
	}
	for _, a := range scope.Ancestors() {
		if v, ok := a.Consts[name]; ok {
			return v, nil
		}
	}
	return object.Value{}, uninitialized(scope, name, scope.QualifiedName()+"::"+name, line)
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
	if msg, refused := subclassRefused(super); refused {
		return object.Value{}, in.raise(object.CameoError, 0, "Class.new(%s): %s", super.Name, msg)
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
	if !ok || !p.written() {
		return object.Value{}, in.raise(object.TypeError, 0, "Class.new takes a literal block")
	}
	ps, compiled := in.procCode(p)
	if p.IR != nil {
		cls.Digest = "new:" + p.IR.proc.GetDigest()
	} else {
		cls.Digest = "new:" + syntax.Digest(p.Block)
	}
	env := NewEnv(p.Env)
	if len(ps.Positional) > 0 {
		env.Define(ps.Positional[0].Name, object.ClassValue(cls))
	}
	lexical := append(append([]*object.Class{}, p.Frame.Lexical...), cls)
	bodyFrame := &Frame{Self: object.ClassValue(cls), Lexical: lexical, DefTarget: cls, File: p.Frame.File}
	cls.Open = true
	var err error
	if compiled != nil {
		_, err = in.runBody(compiled, env, bodyFrame)
	} else {
		_, err = in.evalBody(p.Block.Body, env, bodyFrame)
	}
	cls.Open = false
	if err != nil {
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
	return in.defineMethod(n.Name, n.Singleton, n, fr, n.Line())
}

// sameBody reports whether a and b are the same definition of a method: a
// *syntax.Def, or the proc of a unit.
func sameBody(a, b any) bool {
	if pa, ok := a.(*irProc); ok {
		pb, ok := b.(*irProc)
		return ok && pa.proc == pb.proc
	}
	return a == b
}

// defineMethod defines the method name on the class a def in fr defines
// on, or with singleton set as a class method of it. body is the method's
// definition: a *syntax.Def the tree walker runs, or an *irProc.
func (in *Interp) defineMethod(name string, singleton bool, body any, fr *Frame, line int) (object.Value, error) {
	target := fr.DefTarget
	if target == nil {
		target = in.mainClass
	}
	table := target.Methods
	if singleton {
		table = target.SingletonMethods
	}
	// A second def of a name replaces the first, as in Ruby. The same
	// definition evaluated again, as a loop around it does, changes nothing,
	// and leaves the method caches as they are.
	if existing, ok := table[name]; ok && existing.Owner == target && existing.Body != nil && sameBody(existing.Body, body) {
		return object.Symbol(name), nil
	}
	if existing, ok := table[name]; ok && existing.Native != nil && existing.Body == nil && existing.Owner == target {
		if _, isField := target.FieldIndex[name]; isField {
			return object.Value{}, in.raise(object.CameoError, line, "def %s: %s is a field of %s, and fields read themselves", name, name, target.QualifiedName())
		}
		if _, isAttr := target.AttrDefaults[name]; isAttr && singleton {
			return object.Value{}, in.raise(object.CameoError, line, "def self.%s: %s is an attribute of %s, and attributes read themselves", name, name, target.QualifiedName())
		}
		return object.Value{}, in.raise(object.CameoError, line, "cannot replace the builtin method %s of %s", name, target.Name)
	}
	m := &object.Method{Name: name, Owner: target, Body: body, Arity: -1, Lexical: fr.Lexical, File: fr.File}
	// A method `cameo build` compiled runs as Go, and keeps its body for
	// the calls the Go code leaves to the interpreter: a top-level method
	// by its name, a class's as Class#name.
	if !singleton {
		key := name
		if target != in.mainClass {
			key = target.QualifiedName() + "#" + name
		}
		m.Native = in.code.compiled[key]
	}
	if singleton {
		target.SetSingletonMethod(m)
	} else {
		target.SetMethod(m)
	}
	if target == in.mainClass {
		in.changedTop()
	}
	return object.Symbol(name), nil
}

// methodCode returns what a call of the user method m runs: the name it
// was defined under, its parameter list, and its IR, nil when it runs on
// the tree walker, whose definition def then is.
func (in *Interp) methodCode(m *object.Method) (name string, ps *params, compiled *irProc, def *syntax.Def) {
	if p, ok := m.Body.(*irProc); ok {
		return p.proc.GetName(), in.code.irParams(p), p, nil
	}
	def = m.Body.(*syntax.Def)
	if compiled = in.code.irDef(def); compiled != nil {
		return def.Name, in.code.irParams(compiled), compiled, def
	}
	return def.Name, in.code.astParams(&def.Params), nil, def
}

// evalClassDef binds a class to its name and runs its body. A class
// already bound to the name is reopened: the body runs on a copy, which
// replaces it, so instances made before keep the class they were made
// with. The definition that last ran on the class, evaluated again,
// returns the class as it is. The name is bound to the class while its
// body runs, as in Ruby, and back to what it was when the body raises.
func (in *Interp) evalClassDef(n *syntax.ClassDef, env *Env, fr *Frame) (object.Value, error) {
	var super object.Value
	if n.Super != nil {
		v, err := in.eval(n.Super, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		super = v
	}
	return in.defineClass(classDecl{Name: n.Name, Digest: n.Digest, line: n.Line()}, super, n.Super != nil, in.astBody(n.Body), fr)
}

// classDecl is what a class or module statement declares: the name, the
// digest of the definition, and its line.
type classDecl struct {
	Name, Digest string
	line         int
}

func (d classDecl) Line() int { return d.line }

// bodyFn runs the body of a class or module in the scope and frame made
// for it.
type bodyFn func(env *Env, fr *Frame) (object.Value, error)

// astBody is the body of a definition the tree walker runs.
func (in *Interp) astBody(body []syntax.Node) bodyFn {
	return func(env *Env, fr *Frame) (object.Value, error) { return in.evalBody(body, env, fr) }
}

// defineClass is evalClassDef once the superclass is evaluated: v, when
// hasSuper says the statement names one.
func (in *Interp) defineClass(n classDecl, v object.Value, hasSuper bool, body bodyFn, fr *Frame) (object.Value, error) {
	table := in.constTable(fr)
	var super *object.Class
	if hasSuper {
		cls, ok := v.AsClass()
		if !ok || cls.IsModule {
			return object.Value{}, in.raise(object.TypeError, n.Line(), "superclass must be a Class (%s given)", typeName(v))
		}
		super = cls
	}
	if msg, refused := subclassRefused(super); refused {
		return object.Value{}, in.raise(object.CameoError, n.Line(), "class %s < %s: %s", n.Name, super.Name, msg)
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
	if err := in.evalBound(cls, existing, body, fr, previous, bound); err != nil {
		return object.Value{}, err
	}
	return object.ClassValue(cls), nil
}

// evalModuleDef binds a module to its name and runs its body, reopening
// a module already bound to the name as evalClassDef reopens a class.
func (in *Interp) evalModuleDef(n *syntax.ModuleDef, env *Env, fr *Frame) (object.Value, error) {
	return in.defineModule(classDecl{Name: n.Name, Digest: n.Digest, line: n.Line()}, in.astBody(n.Body), fr)
}

// defineModule binds the module n declares to its name and runs its body.
func (in *Interp) defineModule(n classDecl, body bodyFn, fr *Frame) (object.Value, error) {
	table := in.constTable(fr)
	previous, bound := table[n.Name]
	existing, _ := previous.AsClass()
	if existing != nil && existing.IsModule && ranLast(existing, n.Digest) {
		return object.ClassValue(existing), nil
	}
	if !bound && len(fr.Lexical) == 0 {
		if native := in.libraryOpens(fr, n.Name); native != nil {
			// A library reopens the module Go defined, on a copy, as it
			// would one a script defined: see libraryOpens.
			mod := native.Copy()
			mod.Digest = "+" + n.Digest
			if err := in.evalBound(mod, native, body, fr, previous, bound); err != nil {
				return object.Value{}, err
			}
			return object.ClassValue(mod), nil
		}
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
	if err := in.evalBound(mod, existing, body, fr, previous, bound); err != nil {
		return object.Value{}, err
	}
	return object.ClassValue(mod), nil
}

// libraryOpens returns the module Go defined at the top level under name,
// when the code in fr is a library's, written in cameo, and the program
// has no module of that name of its own yet; nil otherwise. A library
// opens such a module to add to it, `module M` or `class M::C` in its
// source where its Go package defined M, as a script reopens a module it
// defined: on a copy, which the program binds to the name in its own
// constants, so the module every program in the process shares stays as
// Go defined it and other programs see none of the library. A script
// still cannot: the core and the host's modules are not a script's to
// change. A class Go defined at the top level is not opened so, since
// its instances, which Go makes, would keep the class Go defined, without
// what a copy added; one inside the module, M::C, reopens on the
// copy as any class bound in a module does, which works where Go makes
// its instances of the class new was called on.
func (in *Interp) libraryOpens(fr *Frame, name string) *object.Class {
	if fr.File == "" {
		return nil
	}
	if _, own := in.consts[name]; own {
		return nil
	}
	v, ok := object.ObjectClass.Consts[name]
	if !ok {
		return nil
	}
	mod, ok := v.AsClass()
	if !ok || !mod.IsModule || mod.Digest != "" {
		return nil
	}
	return mod
}

// scopeDigest names the library a copy libraryOpens made belongs to.
func scopeDigest(fr *Frame) string { return "lib:" + fr.File }

// ranLast reports whether the definition whose digest is given is the
// last one that ran on cls: its first, or the last that reopened it.
func ranLast(cls *object.Class, digest string) bool {
	return cls.Digest == digest || strings.HasSuffix(cls.Digest, "+"+digest)
}

// evalBound binds cls to its name and runs its body, with the fields cls
// had from reopened as fields the body may declare again. When the body
// raises, the name is bound to previous again, or to nothing when it
// was not bound.
func (in *Interp) evalBound(cls, reopened *object.Class, body bodyFn, fr *Frame, previous object.Value, bound bool) error {
	in.bindConst(fr, cls.Name, object.ClassValue(cls))
	if err := in.evalClassBody(cls, reopened, body, fr); err != nil {
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

// subclassRefused is syntax.SubclassRefused for the builtin class super,
// which the checker sees only at the top level.
func subclassRefused(super *object.Class) (string, bool) {
	if super != object.BasicObjectClass {
		return "", false
	}
	return syntax.SubclassRefused(super.Name)
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
func (in *Interp) evalClassBody(cls, reopened *object.Class, body bodyFn, fr *Frame) error {
	lexical := append(append([]*object.Class{}, fr.Lexical...), cls)
	bodyFrame := &Frame{Self: object.ClassValue(cls), Lexical: lexical, DefTarget: cls, Reopened: reopened, File: fr.File}
	bodyEnv := NewEnv(nil)
	cls.Open = true
	defer func() { cls.Open = false }()
	if in.dbg != nil {
		// A class body is a frame of its own, as in Ruby, so next at
		// the class statement steps over it.
		in.dbg.enter("<class:"+cls.Name+">", bodyEnv, bodyFrame)
		defer in.dbg.leave()
	}
	_, err := body(bodyEnv, bodyFrame)
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
	return in.frameIn(v, fr, line)
}

// frameIn is scopedFrame once the scope is evaluated to v.
func (in *Interp) frameIn(v object.Value, fr *Frame, line int) (*Frame, error) {
	mod, ok := v.AsClass()
	if !ok {
		return nil, in.raise(object.TypeError, line, "%s is not a class/module", typeName(v))
	}
	// A library defines inside a module Go defined in the copy the
	// program keeps of it (libraryOpens).
	if native := in.libraryOpens(fr, mod.Name); native == mod {
		mod = native.Copy()
		mod.Digest = "+" + scopeDigest(fr)
		in.bindConst(&Frame{}, mod.Name, object.ClassValue(mod))
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
	return in.alias(n.New, n.Old, fr, n.Line())
}

// alias gives the method old the second name name.
func (in *Interp) alias(name, old string, fr *Frame, line int) (object.Value, error) {
	target := fr.DefTarget
	m := target.Lookup(old)
	if m == nil {
		return object.Value{}, in.raise(object.NameError, line, "undefined method '%s' for %s", old, target.QualifiedName())
	}
	if _, taken := target.Methods[name]; taken {
		return object.Value{}, in.raise(object.CameoError, line, "method %s is already defined in %s", name, target.QualifiedName())
	}
	alias := *m
	alias.Name = name
	target.SetMethod(&alias)
	if target == in.mainClass {
		in.changedTop()
	}
	return object.Nil, nil
}
