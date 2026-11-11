package eval

import (
	"fmt"
	"strings"

	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/ir"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
)

// The classes of a run in its dump, and back: the classes and modules the
// script defined, as data, so that a resumed run has them without running
// a definition again. doc/design/pause.md describes the dump, and
// dump.proto how one class refers to another.

// classDump collects the classes a dump carries while the dump is made: a
// class is carried once something of the dump refers to it.
type classDump struct {
	in *Interp
	// ids holds the id of every class the dump carries, order the classes
	// in the order they were first referred to.
	ids   map[*object.Class]string
	taken map[string]bool
	order []*object.Class
	// missing says what of the classes the dump leaves out.
	missing []string
}

func (in *Interp) newClassDump() *classDump {
	return &classDump{in: in, ids: map[*object.Class]string{}, taken: map[string]bool{}}
}

// carried reports whether the dump carries cls: a class or module a script
// defined, or the copy a library written in cameo made of one Go defined.
// A builtin class and one the host registered are the process's, and are
// referred to by name.
func (cd *classDump) carried(cls *object.Class) bool {
	return scriptDefined(cls) && cls != cd.in.mainClass
}

// nativeNamed finds a class Go defined or the host registered by its
// qualified name; nil when there is none.
func nativeNamed(name string) *object.Class {
	var cls *object.Class
	consts := object.ObjectClass.Consts
	for _, part := range strings.Split(name, "::") {
		var ok bool
		if cls, ok = consts[part].AsClass(); !ok {
			return nil
		}
		consts = cls.Consts
	}
	return cls
}

// copied reports whether a class with the digest is a copy of one Go
// defined, made for a library written in cameo to add to (libraryOpens).
func copied(digest string) bool { return strings.HasPrefix(digest, "+") }

// ref returns the reference to cls for the dump, and has the dump carry
// cls when it is a class the dump carries; "" for no class.
func (cd *classDump) ref(cls *object.Class) string {
	if cls == nil || cls == cd.in.mainClass {
		return ""
	}
	if id, ok := cd.ids[cls]; ok {
		return id
	}
	name := cls.QualifiedName()
	bound := cls.Name != "" && cd.in.classNamed(name) == cls
	if !cd.carried(cls) {
		switch {
		case bound:
			return name
		case cls.Name != "" && nativeNamed(name) == cls:
			return "::" + name
		}
		cd.missing = append(cd.missing, fmt.Sprintf("%s (a class no constant names)", name))
		return ""
	}
	id := name
	if !bound {
		id = name + "@" + cls.Digest
	}
	for base, n := id, 2; cd.taken[id]; n++ {
		id = fmt.Sprintf("%s#%d", base, n)
	}
	cd.ids[cls], cd.taken[id] = id, true
	cd.order = append(cd.order, cls)
	return id
}

// binding is the dump's form of one variable or constant.
func (cd *classDump) binding(enc *protogen.Encoder, name string, v object.Value) *cameopb.Dump_Binding {
	b := &cameopb.Dump_Binding{Name: name}
	if isMain(v) {
		b.Main = true
		return b
	}
	if b.Building = cd.in.dumping.buildingIn(v); b.Building != 0 {
		return b
	}
	if cls, ok := v.AsClass(); ok {
		if b.Class = cd.ref(cls); b.Class == "" {
			b.Missing = "a class no constant names"
		}
		return b
	}
	msg, err := enc.Message(v)
	if err != nil {
		b.Missing = err.Error()
		return b
	}
	b.Value = msg
	return b
}

// bindings is the dump's form of a table of constants or attributes, by
// name. what names the table for the report of a value left out.
func (cd *classDump) bindings(enc *protogen.Encoder, what string, table map[string]object.Value) []*cameopb.Dump_Binding {
	var out []*cameopb.Dump_Binding
	for _, name := range sortedKeys(table) {
		b := cd.binding(enc, name, table[name])
		if b.Missing != "" {
			cd.missing = append(cd.missing, fmt.Sprintf("%s%s (%s)", what, name, b.Missing))
		}
		out = append(out, b)
	}
	return out
}

// classes returns the classes the dump carries. Making the form of one
// may refer to more, which are carried too.
func (cd *classDump) classes(enc *protogen.Encoder) []*cameopb.Dump_Class {
	var out []*cameopb.Dump_Class
	for i := 0; i < len(cd.order); i++ {
		out = append(out, cd.class(enc, cd.order[i]))
	}
	return out
}

// class is the dump's form of cls.
func (cd *classDump) class(enc *protogen.Encoder, cls *object.Class) *cameopb.Dump_Class {
	c := &cameopb.Dump_Class{Id: cd.ids[cls], Module: cls.IsModule, Digest: cls.Digest, Lexical: cd.ref(cls.Lexical)}
	if cls.Name != "" {
		c.Name = cls.QualifiedName()
	}
	// A copy of a class Go defined lists what it has beyond that class,
	// which the run that resumes has too.
	includes, extends, fields := cls.Includes, cls.Extends, cls.Fields
	if copied(cls.Digest) {
		native := nativeNamed(c.Name)
		if native == nil || len(native.Includes) > len(includes) || len(native.Extends) > len(extends) || len(native.Fields) > len(fields) {
			cd.missing = append(cd.missing, fmt.Sprintf("%s (a copy of a class this cameo does not define)", c.Id))
			return c
		}
		includes, extends, fields = includes[len(native.Includes):], extends[len(native.Extends):], fields[len(native.Fields):]
	} else {
		c.Super = cd.ref(cls.Super)
	}
	for _, mod := range includes {
		c.Includes = append(c.Includes, cd.ref(mod))
	}
	for _, mod := range extends {
		c.Extends = append(c.Extends, cd.ref(mod))
	}
	for _, f := range fields {
		field := &cameopb.Dump_Field{Name: f.Name, Number: f.Number, Type: cd.fieldType(f.Type), Owner: cd.ref(f.Owner)}
		if f.Owner == cls && !f.Default.IsZero() {
			def, err := enc.Message(f.Default)
			if err != nil {
				cd.missing = append(cd.missing, fmt.Sprintf("the default of %s.%s (%s)", c.Id, f.Name, err))
			}
			field.Default = def
		}
		c.Fields = append(c.Fields, field)
	}
	c.ReservedNumbers = append(c.ReservedNumbers, cls.ReservedNumbers...)
	c.ReservedNames = append(c.ReservedNames, cls.ReservedNames...)
	c.Methods = cd.methods(cls)
	c.Consts = cd.bindings(enc, c.Id+"::", cls.Consts)
	c.Attrs = cd.bindings(enc, "attribute "+c.Id+".", cls.Attrs)
	c.AttrDefaults = cd.bindings(enc, "the default of attribute "+c.Id+".", cls.AttrDefaults)
	return c
}

// methods returns the methods cls holds that the dump carries, by name:
// the ones a script defined, and the ones a script's alias named. Any
// other method Go implements comes back with what made it: the field or
// the attribute it reads, the module whose include defined it, or the
// class Go defined that cls is a copy of.
func (cd *classDump) methods(cls *object.Class) []*cameopb.Dump_Method {
	var out []*cameopb.Dump_Method
	add := func(table map[string]*object.Method, singleton bool) {
		for _, name := range sortedKeys(table) {
			m := table[name]
			dm := &cameopb.Dump_Method{Name: name, Singleton: singleton}
			if m.Owner != cls {
				dm.Owner = cd.ref(m.Owner)
			}
			code, ok := m.Body.(*irProc)
			switch {
			case ok:
				dm.Unit, dm.File = code.unit.GetDigest(), m.File
				for i, p := range code.unit.GetProcs() {
					if p == code.proc {
						dm.Proc = uint32(i)
					}
				}
				for _, l := range m.Lexical {
					dm.Lexical = append(dm.Lexical, cd.ref(l))
				}
			case m.AliasOf != "":
				dm.AliasOf = m.AliasOf
			default:
				continue
			}
			out = append(out, dm)
		}
	}
	add(cls.Methods, false)
	add(cls.SingletonMethods, true)
	return out
}

func (cd *classDump) fieldType(t *object.FieldType) *cameopb.Dump_FieldType {
	if t == nil {
		return nil
	}
	return &cameopb.Dump_FieldType{Kind: t.Kind, Elem: cd.fieldType(t.Elem), Key: t.Key, Ref: cd.ref(t.Ref), Enum: t.Enum, Named: t.Named}
}

// restoreUnits gives the program the units a dump carries, each checked
// against its digest. A unit the program compiled itself stays.
func (in *Interp) restoreUnits(units []*cameopb.Unit) error {
	for _, unit := range units {
		if unit.GetVersion() != ir.Version {
			return fmt.Errorf("the code of %s in the dump is IR version %d, and this cameo runs version %d", unit.GetFile(), unit.GetVersion(), ir.Version)
		}
		digest, err := ir.Digest(unit)
		if err != nil {
			return err
		}
		if digest != unit.GetDigest() {
			return fmt.Errorf("the code of %s in the dump names digest %s and has %s", unit.GetFile(), unit.GetDigest(), digest)
		}
		in.code.units.LoadOrStore(digest, unit)
	}
	return nil
}

// classRef finds the class a dump refers to with ref, once restoreClasses
// made the classes of the dump; nil when there is none.
func (in *Interp) classRef(ref string) *object.Class {
	if cls := in.restored[ref]; cls != nil {
		return cls
	}
	if name, native := strings.CutPrefix(ref, "::"); native {
		return nativeNamed(name)
	}
	return in.classNamed(ref)
}

// restoreClasses makes the classes of a dump. A class may refer to any
// other, so it makes every class first, then gives each its place and its
// superclass, binds the constants that are classes, and only then gives
// the classes their modules, fields and methods. What the classes hold as
// values, which may be instances of the classes, restoreClassValues
// restores once the classes are whole.
func (in *Interp) restoreClasses(d *cameopb.Dump) error {
	in.restored = map[string]*object.Class{}
	byID := map[string]*cameopb.Dump_Class{}
	for _, c := range d.GetClasses() {
		cls, err := restoreShell(c)
		if err != nil {
			return err
		}
		in.restored[c.GetId()], byID[c.GetId()] = cls, c
	}
	find := func(of, what, ref string) (*object.Class, error) {
		cls := in.classRef(ref)
		if cls == nil {
			return nil, fmt.Errorf("%s of %s, %s, is not a class of the dump or of this cameo", what, of, ref)
		}
		return cls, nil
	}
	for _, c := range d.GetClasses() {
		cls := in.restored[c.GetId()]
		var err error
		if c.GetLexical() != "" {
			if cls.Lexical, err = find(c.GetId(), "the module around", c.GetLexical()); err != nil {
				return err
			}
		}
		if c.GetSuper() != "" {
			if cls.Super, err = find(c.GetId(), "the superclass", c.GetSuper()); err != nil {
				return err
			}
		}
	}
	bind := func(of string, bs []*cameopb.Dump_Binding, table map[string]object.Value) error {
		for _, b := range bs {
			if b.GetClass() == "" {
				continue
			}
			cls, err := find(of, "the constant "+b.GetName(), b.GetClass())
			if err != nil {
				return err
			}
			table[b.GetName()] = object.ClassValue(cls)
		}
		return nil
	}
	if err := bind("the top level", d.GetConstants(), in.consts); err != nil {
		return err
	}
	for _, c := range d.GetClasses() {
		if err := bind(c.GetId(), c.GetConsts(), in.restored[c.GetId()].Consts); err != nil {
			return err
		}
	}
	// A class takes an inherited field from the class that declared it,
	// which has its fields first.
	fielded := map[string]bool{}
	var fields func(c *cameopb.Dump_Class) error
	fields = func(c *cameopb.Dump_Class) error {
		if fielded[c.GetId()] {
			return nil
		}
		fielded[c.GetId()] = true
		cls := in.restored[c.GetId()]
		for _, f := range c.GetFields() {
			var field *object.Field
			if f.GetOwner() == c.GetId() {
				typ, err := in.restoreFieldType(f.GetType())
				if err != nil {
					return fmt.Errorf("field %s of %s: %w", f.GetName(), c.GetId(), err)
				}
				field = &object.Field{Name: f.GetName(), Number: f.GetNumber(), Type: typ, Owner: cls}
				cls.Define(f.GetName(), 0, reader(f.GetName()))
			} else {
				if oc := byID[f.GetOwner()]; oc != nil {
					if err := fields(oc); err != nil {
						return err
					}
				}
				owner, err := find(c.GetId(), "the class that declared field "+f.GetName(), f.GetOwner())
				if err != nil {
					return err
				}
				i, ok := owner.FieldIndex[f.GetName()]
				if !ok {
					return fmt.Errorf("%s has no field %s for %s to inherit", f.GetOwner(), f.GetName(), c.GetId())
				}
				field = owner.Fields[i]
			}
			cls.FieldIndex[f.GetName()] = len(cls.Fields)
			cls.Fields = append(cls.Fields, field)
		}
		return nil
	}
	for _, c := range d.GetClasses() {
		cls := in.restored[c.GetId()]
		for _, ref := range c.GetIncludes() {
			mod, err := find(c.GetId(), "a module included", ref)
			if err != nil {
				return err
			}
			cls.Include(mod)
			// What including the module does to a class, it does again:
			// the methods Singleton gives a class are Go's.
			if mod.Included != nil {
				if err := mod.Included(in, cls); err != nil {
					return err
				}
			}
		}
		for _, ref := range c.GetExtends() {
			mod, err := find(c.GetId(), "a module extended", ref)
			if err != nil {
				return err
			}
			if mod.Extended != nil {
				if err := mod.Extended(in, cls); err != nil {
					return err
				}
			}
			cls.Extend(mod)
		}
		if err := fields(c); err != nil {
			return err
		}
		cls.ReservedNumbers = append([]int64{}, c.GetReservedNumbers()...)
		cls.ReservedNames = append([]string{}, c.GetReservedNames()...)
	}
	// An alias names a method of another class, which has its methods
	// first: the ones the script defined, then the aliases.
	for _, aliases := range []bool{false, true} {
		for _, c := range d.GetClasses() {
			if err := in.restoreMethods(in.restored[c.GetId()], c, aliases); err != nil {
				return err
			}
		}
		if err := in.restoreMethods(in.mainClass, d.GetTop(), aliases); err != nil {
			return err
		}
	}
	in.changedTop()
	return nil
}

// restoreShell makes the class or module c describes, with its name and
// its digest and nothing else: a copy of the class Go defined when it is
// one, which has what Go gave that class.
func restoreShell(c *cameopb.Dump_Class) (*object.Class, error) {
	path := strings.Split(c.GetName(), "::")
	name := path[len(path)-1]
	var cls *object.Class
	switch {
	case copied(c.GetDigest()):
		native := nativeNamed(c.GetName())
		if native == nil {
			return nil, fmt.Errorf("%s is a copy of a class this cameo does not define", c.GetId())
		}
		cls = native.Copy()
	case c.GetModule():
		cls = object.NewModule(name)
	default:
		cls = object.NewClass(name, object.ObjectClass)
	}
	cls.Digest = c.GetDigest()
	return cls, nil
}

func (in *Interp) restoreFieldType(t *cameopb.Dump_FieldType) (*object.FieldType, error) {
	if t == nil {
		return nil, nil
	}
	if scalar := object.ScalarField(t.GetKind()); scalar != nil {
		return scalar, nil
	}
	elem, err := in.restoreFieldType(t.GetElem())
	if err != nil {
		return nil, err
	}
	out := &object.FieldType{Kind: t.GetKind(), Elem: elem, Key: t.GetKey(), Enum: t.GetEnum(), Named: t.GetNamed()}
	if ref := t.GetRef(); ref != "" {
		if out.Ref = in.classRef(ref); out.Ref == nil {
			return nil, fmt.Errorf("%s is not a class of the dump or of this cameo", ref)
		}
	}
	return out, nil
}

// restoreMethods gives cls the methods c lists: with aliases unset the
// ones with code, each with the proc of a unit as its body, with aliases
// set the ones an alias named.
func (in *Interp) restoreMethods(cls *object.Class, c *cameopb.Dump_Class, aliases bool) error {
	of := c.GetId()
	if cls == in.mainClass {
		of = "the top level"
	}
	for _, dm := range c.GetMethods() {
		if (dm.GetAliasOf() != "") != aliases {
			continue
		}
		owner := cls
		if ref := dm.GetOwner(); ref != "" {
			if owner = in.classRef(ref); owner == nil {
				return fmt.Errorf("%s of %s is a method of %s, which is not a class of the dump or of this cameo", dm.GetName(), of, ref)
			}
		}
		if aliases {
			m := owner.Methods[dm.GetAliasOf()]
			if m == nil {
				return fmt.Errorf("%s of %s is another name of %s#%s, which this cameo does not have", dm.GetName(), of, owner.QualifiedName(), dm.GetAliasOf())
			}
			alias := *m
			alias.Name, alias.AliasOf = dm.GetName(), dm.GetAliasOf()
			cls.SetMethod(&alias)
			continue
		}
		known, ok := in.code.units.Load(dm.GetUnit())
		if !ok {
			return fmt.Errorf("the code of %s of %s, unit %s, is not in the dump", dm.GetName(), of, dm.GetUnit())
		}
		unit := known.(*cameopb.Unit)
		if int(dm.GetProc()) >= len(unit.GetProcs()) {
			return fmt.Errorf("unit %s has no proc %d for %s of %s", dm.GetUnit(), dm.GetProc(), dm.GetName(), of)
		}
		m := &object.Method{Name: dm.GetName(), Owner: owner, Body: &irProc{unit: unit, proc: unit.Procs[dm.GetProc()]}, Arity: -1, File: dm.GetFile()}
		for _, ref := range dm.GetLexical() {
			l := in.classRef(ref)
			if l == nil {
				return fmt.Errorf("%s of %s is defined inside %s, which is not a class of the dump or of this cameo", dm.GetName(), of, ref)
			}
			m.Lexical = append(m.Lexical, l)
		}
		if dm.GetSingleton() {
			cls.SetSingletonMethod(m)
			continue
		}
		// A method cameo build compiled runs as Go, as when it was
		// defined (defineMethod).
		key := m.Name
		if cls != in.mainClass {
			key = cls.QualifiedName() + "#" + m.Name
		}
		m.Native = in.code.compiled[key]
		cls.SetMethod(m)
	}
	return nil
}

// restoreClassValues restores what the classes of a dump hold as values:
// the defaults of their fields, their constants and their attributes.
func (in *Interp) restoreClassValues(dec *protogen.Decoder, classes []*cameopb.Dump_Class) error {
	for _, c := range classes {
		cls := in.restored[c.GetId()]
		for _, f := range c.GetFields() {
			if f.GetOwner() != c.GetId() || f.GetDefault() == nil {
				continue
			}
			def, err := dec.Value(f.GetDefault())
			if err != nil {
				return fmt.Errorf("the default of %s.%s: %w", c.GetId(), f.GetName(), err)
			}
			cls.Fields[cls.FieldIndex[f.GetName()]].Default = def
		}
		values := func(what string, bs []*cameopb.Dump_Binding, table map[string]object.Value) error {
			for _, b := range bs {
				v, ok, err := in.restore(dec, b)
				if err != nil {
					return fmt.Errorf("%s %s of %s: %w", what, b.GetName(), c.GetId(), err)
				}
				if ok {
					table[b.GetName()] = v
				}
			}
			return nil
		}
		if err := values("constant", c.GetConsts(), cls.Consts); err != nil {
			return err
		}
		if err := values("attribute", c.GetAttrs(), cls.Attrs); err != nil {
			return err
		}
		if err := values("the default of attribute", c.GetAttrDefaults(), cls.AttrDefaults); err != nil {
			return err
		}
		for _, b := range c.GetAttrDefaults() {
			cls.DefineSingleton(b.GetName(), 0, attrReader(b.GetName()))
		}
	}
	return nil
}
