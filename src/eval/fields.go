package eval

import (
	"strings"

	"github.com/radiospiel/amber/src/object"
	"github.com/radiospiel/amber/src/syntax"
)

// evalFieldDecl declares typed fields in a class body: each gets the next
// free field number, its default checked against its type, and a reader.
func (in *Interp) evalFieldDecl(n *syntax.FieldDecl, env *Env, fr *Frame) (object.Value, error) {
	cls := fr.DefTarget
	if cls == nil || cls.IsModule || fr.Self != cls {
		return nil, in.raise(object.AmberError, n.Line(), "%s :%s: a field belongs to a class body", n.Type, n.Names[0])
	}
	typ, err := in.fieldType(n, env, fr)
	if err != nil {
		return nil, err
	}
	var def object.Value
	if n.Default != nil {
		v, err := in.eval(n.Default, env, fr)
		if err != nil {
			return nil, err
		}
		def = v
	}
	for _, name := range n.Names {
		if _, exists := cls.FieldIndex[name]; exists {
			return nil, in.raise(object.AmberError, n.Line(), "field %s is already declared in %s", name, cls.QualifiedName())
		}
		for _, r := range cls.ReservedNames {
			if r == name {
				return nil, in.raise(object.AmberError, n.Line(), "field %s is reserved in %s", name, cls.QualifiedName())
			}
		}
		if m := cls.Lookup(name); m != nil && m.Owner == cls {
			return nil, in.raise(object.AmberError, n.Line(), "field %s: %s already defines a method %s", name, cls.QualifiedName(), name)
		}
		f := &object.Field{Name: name, Number: in.nextFieldNumber(cls), Type: typ, Owner: cls}
		if def != nil {
			checked, err := in.checkField(f, def, n.Line())
			if err != nil {
				return nil, err
			}
			f.Default = checked
		}
		cls.FieldIndex[name] = len(cls.Fields)
		cls.Fields = append(cls.Fields, f)
		cls.Define(name, 0, reader(name))
	}
	return object.Nil, nil
}

// reader makes the native method that reads a field.
func reader(name string) object.NativeFn {
	return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		inst := self.(*object.Instance)
		v, _ := inst.Field(name)
		if v == nil {
			return object.Nil, nil
		}
		return v, nil
	}
}

// nextFieldNumber is the lowest field number no field holds and no
// reserved statement claims.
func (in *Interp) nextFieldNumber(cls *object.Class) int64 {
	used := map[int64]bool{}
	for _, f := range cls.Fields {
		used[f.Number] = true
	}
	for _, r := range cls.ReservedNumbers {
		used[r] = true
	}
	for n := int64(1); ; n++ {
		if !used[n] {
			return n
		}
	}
}

// fieldType builds a field's type from its declaration, resolving the
// classes it names now.
func (in *Interp) fieldType(n *syntax.FieldDecl, env *Env, fr *Frame) (*object.FieldType, error) {
	t := &object.FieldType{Kind: n.Type}
	switch n.Type {
	case "list", "set":
		elem, err := in.elementType(n.Of, fr, n.Line())
		if err != nil {
			return nil, err
		}
		t.Elem = elem
	case "map":
		t.Key = n.Key
		elem, err := in.elementType(n.Value, fr, n.Line())
		if err != nil {
			return nil, err
		}
		t.Elem = elem
	case "ref":
		v, err := in.eval(n.Ref, env, fr)
		if err != nil {
			return nil, err
		}
		cls, ok := v.(*object.Class)
		if !ok || cls.IsModule || cls.Digest == "" {
			return nil, in.raise(object.TypeError, n.Line(), "ref :%s: %s is not a class with fields", n.Names[0], typeName(v))
		}
		t.Ref = cls
	case "enum":
		t.Enum = n.Values
	}
	return t, nil
}

// elementType resolves an element type name: a scalar kind, or a class.
func (in *Interp) elementType(name string, fr *Frame, line int) (*object.FieldType, error) {
	switch name {
	case "string", "int", "float", "bool", "bytes":
		return &object.FieldType{Kind: name}, nil
	}
	v, err := in.lookupConst(name, fr, line)
	if err != nil {
		return nil, err
	}
	cls, ok := v.(*object.Class)
	if !ok || cls.IsModule || cls.Digest == "" {
		return nil, in.raise(object.TypeError, line, "%s is not a class with fields", name)
	}
	return &object.FieldType{Kind: "ref", Ref: cls}, nil
}

// checkField checks a value against a field's declared type and returns
// the value as stored: an Integer given to a float field becomes a Float.
func (in *Interp) checkField(f *object.Field, v object.Value, line int) (object.Value, error) {
	if v == object.Nil {
		return emptyFor(f.Type), nil
	}
	out, ok := in.coerce(f.Type, v)
	if !ok {
		return nil, in.raise(object.TypeError, line, "field %s of %s takes %s, not %s", f.Name, f.Owner.QualifiedName(), describeType(f.Type), typeName(v))
	}
	return out, nil
}

// emptyFor is what a list, set or map field holds when given nil: the
// empty collection, since the wire form cannot tell absent from empty. Any
// other field holds nil.
func emptyFor(t *object.FieldType) object.Value {
	switch t.Kind {
	case "list":
		return object.NewList()
	case "set":
		return object.NewSet()
	case "map":
		return object.NewMap()
	}
	return object.Nil
}

// coerce checks v against a type and returns the stored form.
func (in *Interp) coerce(t *object.FieldType, v object.Value) (object.Value, bool) {
	switch t.Kind {
	case "string":
		_, ok := v.(object.String)
		return v, ok
	case "int":
		_, ok := v.(object.Int)
		return v, ok
	case "float":
		switch x := v.(type) {
		case object.Float:
			return x, true
		case object.Int:
			return object.Float(x), true
		}
		return nil, false
	case "bool":
		_, ok := v.(object.Bool)
		return v, ok
	case "bytes":
		switch x := v.(type) {
		case object.Bytes:
			return x, true
		case object.String:
			return object.Bytes([]byte(x)), true
		}
		return nil, false
	case "list":
		l, ok := v.(*object.List)
		if !ok {
			return nil, false
		}
		out := make([]object.Value, len(l.Elems))
		for i, e := range l.Elems {
			c, ok := in.coerce(t.Elem, e)
			if !ok {
				return nil, false
			}
			out[i] = c
		}
		return object.NewList(out...), true
	case "set":
		s, ok := v.(*object.Set)
		if !ok {
			if l, isList := v.(*object.List); isList {
				s = object.NewSet(l.Elems...)
			} else {
				return nil, false
			}
		}
		out := make([]object.Value, 0, s.Len())
		for _, e := range s.Elems() {
			c, ok := in.coerce(t.Elem, e)
			if !ok {
				return nil, false
			}
			out = append(out, c)
		}
		return object.NewSet(out...), true
	case "map":
		m, ok := v.(*object.Map)
		if !ok {
			return nil, false
		}
		b := object.NewMapBuilder()
		for i, k := range m.Keys() {
			key, ok := in.coerceKey(t.Key, k)
			if !ok {
				return nil, false
			}
			val, ok := in.coerce(t.Elem, m.Values()[i])
			if !ok {
				return nil, false
			}
			b.Set(key, val)
		}
		return b.Map(), true
	case "ref":
		inst, ok := v.(*object.Instance)
		if !ok || !inst.Cls.IsA(t.Ref) {
			return nil, false
		}
		return v, true
	case "enum":
		var name string
		switch x := v.(type) {
		case object.Symbol:
			name = string(x)
		case object.String:
			name = string(x)
		default:
			return nil, false
		}
		for _, e := range t.Enum {
			if e == name {
				return object.Symbol(name), true
			}
		}
		return nil, false
	}
	return nil, false
}

// coerceKey checks a map key against the declared key kind; a symbol is
// stored as the string it names.
func (in *Interp) coerceKey(kind string, k object.Value) (object.Value, bool) {
	switch kind {
	case "string":
		switch x := k.(type) {
		case object.String:
			return x, true
		case object.Symbol:
			return object.String(x), true
		}
	case "int":
		_, ok := k.(object.Int)
		return k, ok
	case "bool":
		_, ok := k.(object.Bool)
		return k, ok
	}
	return nil, false
}

// describeType names a field type in an error message.
func describeType(t *object.FieldType) string {
	switch t.Kind {
	case "string", "int", "float", "bool", "bytes":
		return article(t.Kind)
	case "list", "set":
		return "a " + t.Kind + " of " + bare(t.Elem)
	case "map":
		return "a map of " + t.Key + " to " + bare(t.Elem)
	case "ref":
		return article(t.Ref.QualifiedName())
	case "enum":
		return "one of :" + strings.Join(t.Enum, ", :")
	}
	return t.Kind
}

// bare names an element type without its article.
func bare(t *object.FieldType) string {
	if t.Kind == "ref" {
		return t.Ref.QualifiedName()
	}
	if t.Kind == "enum" {
		return describeType(t)
	}
	return t.Kind
}

// article prefixes a noun with a or an.
func article(noun string) string {
	if strings.ContainsRune("aeiouAEIOU", rune(noun[0])) {
		return "an " + noun
	}
	return "a " + noun
}

func (in *Interp) evalReserved(n *syntax.Reserved, fr *Frame) (object.Value, error) {
	cls := fr.DefTarget
	if cls == nil || cls.IsModule || fr.Self != cls {
		return nil, in.raise(object.AmberError, n.Line(), "reserved belongs to a class body")
	}
	for _, f := range cls.Fields {
		if f.Number == n.Number {
			return nil, in.raise(object.AmberError, n.Line(), "reserved %d: field %s already holds that number", n.Number, f.Name)
		}
		if n.Name != "" && f.Name == n.Name {
			return nil, in.raise(object.AmberError, n.Line(), "reserved :%s: a field of that name is declared", n.Name)
		}
	}
	cls.ReservedNumbers = append(cls.ReservedNumbers, n.Number)
	if n.Name != "" {
		cls.ReservedNames = append(cls.ReservedNames, n.Name)
	}
	return object.Nil, nil
}

func (in *Interp) evalInclude(n *syntax.Include, env *Env, fr *Frame) (object.Value, error) {
	cls := fr.DefTarget
	if cls == nil || fr.Self != cls {
		return nil, in.raise(object.AmberError, n.Line(), "include belongs to a class or module body")
	}
	v, err := in.eval(n.Module, env, fr)
	if err != nil {
		return nil, err
	}
	mod, ok := v.(*object.Class)
	if !ok || !mod.IsModule {
		return nil, in.raise(object.TypeError, n.Line(), "wrong argument type %s (expected Module)", typeName(v))
	}
	if mod == cls || mod.IsA(cls) {
		return nil, in.raise(object.ArgumentError, n.Line(), "cyclic include detected")
	}
	if n.Extend {
		cls.Extend(mod)
	} else {
		cls.Include(mod)
	}
	return cls, nil
}

// evalWith is a copy of a value with one key replaced: what chained and
// index assignment rewrite to.
func (in *Interp) evalWith(n *syntax.With, env *Env, fr *Frame) (object.Value, error) {
	target, err := in.eval(n.Target, env, fr)
	if err != nil {
		return nil, err
	}
	key, err := in.eval(n.Key, env, fr)
	if err != nil {
		return nil, err
	}
	value, err := in.eval(n.Value, env, fr)
	if err != nil {
		return nil, err
	}
	out, err := in.with(target, key, value)
	return out, withLine(err, n.Line())
}

// with returns target with key set to value: a field on an instance, an
// entry in a map, an element of a list.
func (in *Interp) with(target, key, value object.Value) (object.Value, error) {
	switch t := target.(type) {
	case *object.Instance:
		name, ok := key.(object.Symbol)
		if !ok {
			if s, isStr := key.(object.String); isStr {
				name = object.Symbol(s)
			} else {
				return nil, in.raise(object.TypeError, 0, "a field of %s is named by a symbol, not %s", t.Cls.QualifiedName(), typeName(key))
			}
		}
		idx, ok := t.Cls.FieldIndex[string(name)]
		if !ok {
			return nil, in.raise(object.NameError, 0, "%s has no field %s", t.Cls.QualifiedName(), name)
		}
		checked, err := in.checkField(t.Cls.Fields[idx], value, 0)
		if err != nil {
			return nil, err
		}
		fields := append([]object.Value{}, t.Fields...)
		fields[idx] = checked
		return &object.Instance{Cls: t.Cls, Fields: fields}, nil
	case *object.Map:
		return t.Put(key, value), nil
	case *object.List:
		i, ok := key.(object.Int)
		if !ok {
			return nil, in.raise(object.TypeError, 0, "a list is indexed by an integer, not %s", typeName(key))
		}
		idx := int(i)
		if idx < 0 {
			idx += t.Len()
		}
		if idx < 0 || idx > t.Len() {
			return nil, in.raise(object.IndexError, 0, "index %d outside of list bounds: %d...%d", i, -t.Len(), t.Len())
		}
		elems := append([]object.Value{}, t.Elems...)
		if idx == t.Len() {
			elems = append(elems, value)
		} else {
			elems[idx] = value
		}
		return object.NewList(elems...), nil
	}
	return nil, in.raise(object.TypeError, 0, "cannot assign into %s", typeName(target))
}

// tap is `value.tap { |v| { key: new, … } }`: a copy with every entry of
// the map the block returns applied.
func (in *Interp) tap(self object.Value, block object.Value) (object.Value, error) {
	if block == nil {
		return nil, in.raise(object.LocalJumpError, 0, "no block given (tap)")
	}
	switch self.(type) {
	case *object.Instance, *object.Map, *object.List:
	default:
		return nil, in.raise(object.TypeError, 0, "tap copies an instance, a map or a list, not %s", typeName(self))
	}
	changes, err := in.callBlock(block, []object.Value{self}, nil, nil)
	if err != nil {
		return nil, err
	}
	m, ok := changes.(*object.Map)
	if !ok {
		return nil, in.raise(object.TypeError, 0, "tap's block returns a map of the changes, not %s", typeName(changes))
	}
	out := self
	for i, k := range m.Keys() {
		if out, err = in.with(out, k, m.Values()[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// inheritFields copies a superclass's fields into a new class, keeping
// their numbers, so a subclass extends its parent's message.
func (in *Interp) inheritFields(cls, super *object.Class) error {
	for _, f := range super.Fields {
		cls.FieldIndex[f.Name] = len(cls.Fields)
		cls.Fields = append(cls.Fields, f)
	}
	cls.ReservedNumbers = append(cls.ReservedNumbers, super.ReservedNumbers...)
	cls.ReservedNames = append(cls.ReservedNames, super.ReservedNames...)
	return nil
}
