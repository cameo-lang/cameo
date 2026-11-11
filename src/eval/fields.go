package eval

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
	"github.com/cameo-lang/cameo/src/syntax"
)

// evalFieldDecl declares typed fields in a class body: each gets the
// number its declaration gives, or the one protogen.FieldNumber derives
// from its name, its default checked against its type, and a reader.
func (in *Interp) evalFieldDecl(sn *syntax.FieldDecl, env *Env, fr *Frame) (object.Value, error) {
	n := &fieldDecl{Type: sn.Type, Names: sn.Names, Number: sn.Number, Of: sn.Of, Key: sn.Key, Value: sn.Value, Values: sn.Values, line: sn.Line()}
	if err := in.fieldTarget(n, fr); err != nil {
		return object.Value{}, err
	}
	typ, err := in.fieldType(n, func() (object.Value, error) { return in.eval(sn.Ref, env, fr) }, fr)
	if err != nil {
		return object.Value{}, err
	}
	var def object.Value
	if sn.Default != nil {
		v, err := in.eval(sn.Default, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		def = v
	}
	return in.declareFields(n, typ, def, fr)
}

// fieldDecl is a field declaration as its statement wrote it, but for the
// class of a ref and the default, which are evaluated.
type fieldDecl struct {
	Type   string
	Names  []string
	Number int64
	Of     string
	Key    string
	Value  string
	Values []string
	line   int
}

func (n *fieldDecl) Line() int { return n.line }

// fieldTarget is the error for a field declared outside a class body, or
// nil.
func (in *Interp) fieldTarget(n *fieldDecl, fr *Frame) error {
	cls := fr.DefTarget
	if cls == nil || cls.IsModule || !isClassValue(fr.Self, cls) {
		return in.raise(object.CameoError, n.Line(), "%s :%s: a field belongs to a class body", n.Type, n.Names[0])
	}
	return nil
}

// declareFields declares the fields of n in the class whose body fr runs,
// with the type typ and the default def, the zero Value for none.
func (in *Interp) declareFields(n *fieldDecl, typ *object.FieldType, def object.Value, fr *Frame) (object.Value, error) {
	cls := fr.DefTarget
	for _, name := range n.Names {
		if i, exists := cls.FieldIndex[name]; exists {
			if same, err := in.declaredBefore(cls, i, typ, def, n, fr); err != nil || same {
				if err != nil {
					return object.Value{}, err
				}
				continue
			}
			return object.Value{}, in.raise(object.CameoError, n.Line(), "field %s is already declared in %s", name, cls.QualifiedName())
		}
		for _, r := range cls.ReservedNames {
			if r == name {
				return object.Value{}, in.raise(object.CameoError, n.Line(), "field %s is reserved in %s", name, cls.QualifiedName())
			}
		}
		if protogen.ExceptionPart(cls, name) {
			return object.Value{}, in.raise(object.CameoError, n.Line(), "field %s: an exception of %s carries its %s already", name, cls.QualifiedName(), name)
		}
		if m := cls.Lookup(name); m != nil && m.Owner == cls {
			return object.Value{}, in.raise(object.CameoError, n.Line(), "field %s: %s already defines a method %s", name, cls.QualifiedName(), name)
		}
		number, err := in.fieldNumber(cls, name, n)
		if err != nil {
			return object.Value{}, err
		}
		f := &object.Field{Name: name, Number: number, Type: typ, Owner: cls}
		if !def.IsZero() {
			checked, err := in.checkField(f, def, n.Line())
			if err != nil {
				return object.Value{}, err
			}
			f.Default = checked
		}
		cls.FieldIndex[name] = len(cls.Fields)
		cls.Fields = append(cls.Fields, f)
		cls.Define(name, 0, reader(name))
	}
	return object.Nil, nil
}

// declaredBefore reports whether the field at index i of cls, which a
// body reopening the class declares again, came from the class reopened
// and the declaration gives it again: the same type, the same default,
// and the same number when it names one.
func (in *Interp) declaredBefore(cls *object.Class, i int, typ *object.FieldType, def object.Value, n *fieldDecl, fr *Frame) (bool, error) {
	f := cls.Fields[i]
	if fr.Reopened == nil || f.Owner != cls || describeType(f.Type) != describeType(typ) || (n.Number != 0 && n.Number != f.Number) {
		return false, nil
	}
	if j, ok := fr.Reopened.FieldIndex[f.Name]; !ok || j != i {
		return false, nil
	}
	if def.IsZero() || f.Default.IsZero() {
		return def.IsZero() == f.Default.IsZero(), nil
	}
	checked, err := in.checkField(f, def, n.Line())
	if err != nil {
		return false, err
	}
	return object.Equal(f.Default, checked), nil
}

// reader makes the native method that reads a field.
func reader(name string) object.NativeFn {
	return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		var v object.Value
		if exc, ok := self.AsException(); ok {
			v, _ = exc.Field(name)
		} else {
			v, _ = self.Instance().Field(name)
		}
		if v.IsZero() {
			return object.Nil, nil
		}
		return v, nil
	}
}

// fieldNumber returns the number of field name: the one n declares, which
// no field may hold and no reserved statement claim, or the one
// protogen.FieldNumber derives from the name.
func (in *Interp) fieldNumber(cls *object.Class, name string, n *fieldDecl) (int64, error) {
	taken := func(number int64) bool {
		for _, f := range cls.Fields {
			if f.Number == number {
				return true
			}
		}
		return slices.Contains(cls.ReservedNumbers, number)
	}
	if n.Number == 0 {
		return protogen.FieldNumber(name, taken), nil
	}
	if exceptionNumber(cls, n.Number) {
		return 0, in.raise(object.CameoError, n.Line(), "field %s: numbers 1 to %d are the exception's own", name, protogen.ExceptionNumbers)
	}
	for _, f := range cls.Fields {
		if f.Number == n.Number {
			return 0, in.raise(object.CameoError, n.Line(), "field %s: number %d is field %s's", name, n.Number, f.Name)
		}
	}
	if slices.Contains(cls.ReservedNumbers, n.Number) {
		return 0, in.raise(object.CameoError, n.Line(), "field %s: number %d is reserved in %s", name, n.Number, cls.QualifiedName())
	}
	return n.Number, nil
}

// fieldType builds a field's type from its declaration, resolving the
// classes it names now.
func (in *Interp) fieldType(n *fieldDecl, ref func() (object.Value, error), fr *Frame) (*object.FieldType, error) {
	if st := object.ScalarField(n.Type); st != nil {
		return st, nil
	}
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
		v, err := ref()
		if err != nil {
			return nil, err
		}
		cls, ok := v.AsClass()
		if !ok || cls.IsModule || cls.Digest == "" {
			return nil, in.raise(object.TypeError, n.Line(), "ref :%s: %s is not a class with fields", n.Names[0], typeName(v))
		}
		t.Ref = cls
	case "enum":
		t.Enum = n.Values
	case "proc":
		// A proc field holds a block; a class with one is no message (see
		// protogen.NoMessage).
	}
	return t, nil
}

// elementType resolves an element type name: a scalar kind, or a class.
func (in *Interp) elementType(name string, fr *Frame, line int) (*object.FieldType, error) {
	if st := object.ScalarField(name); st != nil {
		return st, nil
	}
	v, err := in.lookupConst(name, fr, line)
	if err != nil {
		return nil, err
	}
	cls, ok := v.AsClass()
	if !ok || cls.IsModule || cls.Digest == "" {
		return nil, in.raise(object.TypeError, line, "%s is not a class with fields", name)
	}
	return &object.FieldType{Kind: "ref", Ref: cls}, nil
}

// checkField checks a value against a field's declared type and returns
// the value as stored: an Integer given to a float field becomes a Float.
func (in *Interp) checkField(f *object.Field, v object.Value, line int) (object.Value, error) {
	if v.IsNil() {
		return object.Empty(f.Type), nil
	}
	out, ok := in.coerce(f.Type, v)
	if !ok {
		// A number of the right kind that its type cannot hold, the field's
		// own or an element's, is out of range rather than of a wrong type.
		if n, s, role, found := outOfRange(f.Type, v, ""); found {
			what := "takes " + describeType(f.Type)
			if role != "" {
				what = "takes " + article(s.Name) + " for each " + role
			}
			limits := s.Range()
			if s.Kind == object.ScalarFloat {
				limits = fmt.Sprintf("up to %g in magnitude", math.MaxFloat32)
			}
			return object.Value{}, in.raise(object.RangeError, line, "field %s of %s %s, %s; %s does not fit", f.Name, f.Owner.QualifiedName(), what, limits, inspectNumber(n))
		}
	}
	if !ok {
		return object.Value{}, in.raise(object.TypeError, line, "field %s of %s takes %s, not %s", f.Name, f.Owner.QualifiedName(), describeType(f.Type), typeName(v))
	}
	return out, nil
}

// coerce checks v against a type and returns the stored form.
func (in *Interp) coerce(t *object.FieldType, v object.Value) (object.Value, bool) {
	if t.Scalar != nil {
		return coerceScalar(t.Scalar, v)
	}
	switch t.Kind {
	case "proc":
		if v.Kind() == object.KindProc {
			return v, true
		}
		return object.Value{}, false
	case "list":
		l, ok := v.AsList()
		if !ok {
			return object.Value{}, false
		}
		out := make([]object.Value, len(l.Elems))
		for i, e := range l.Elems {
			c, ok := in.coerce(t.Elem, e)
			if !ok {
				return object.Value{}, false
			}
			out[i] = c
		}
		return object.NewList(out...), true
	case "set":
		s, ok := v.AsSet()
		if !ok {
			if l, isList := v.AsList(); isList {
				s = object.NewSet(l.Elems...)
			} else {
				return object.Value{}, false
			}
		}
		out := make([]object.Value, 0, s.Len())
		for _, e := range s.Elems() {
			c, ok := in.coerce(t.Elem, e)
			if !ok {
				return object.Value{}, false
			}
			out = append(out, c)
		}
		return object.SetValue(object.NewSet(out...)), true
	case "map":
		m, ok := v.AsMap()
		if !ok {
			return object.Value{}, false
		}
		b := object.NewMapBuilder()
		for i, k := range m.Keys() {
			key, ok := in.coerceKey(t.Key, k)
			if !ok {
				return object.Value{}, false
			}
			val, ok := in.coerce(t.Elem, m.Values()[i])
			if !ok {
				return object.Value{}, false
			}
			b.Set(key, val)
		}
		return object.MapValue(b.Map()), true
	case "ref":
		inst, ok := v.AsInstance()
		if !ok || !inst.Cls.IsA(t.Ref) {
			return object.Value{}, false
		}
		return v, true
	case "enum":
		var name string
		switch v.Kind() {
		case object.KindSymbol:
			x := v.Str()
			name = string(x)
		case object.KindString, object.KindBytes, object.KindASCII:
			x := v.Str()
			name = string(x)
		default:
			return object.Value{}, false
		}
		for _, e := range t.Enum {
			if e == name {
				return object.Symbol(name), true
			}
		}
		return object.Value{}, false
	}
	return object.Value{}, false
}

// coerceScalar checks v against a scalar type and returns the stored form.
func coerceScalar(t *object.ScalarType, v object.Value) (object.Value, bool) {
	switch t.Kind {
	case object.ScalarString:
		// A protobuf string is UTF-8: a string of another encoding fits
		// when its bytes are.
		s, ok := v.AsString()
		if !ok || v.Kind() != object.KindString && !utf8.ValidString(s) {
			return object.Value{}, false
		}
		return object.String(s), true
	case object.ScalarInteger:
		return v, t.Fits(v)
	case object.ScalarFloat:
		if !isNumber(v) {
			return object.Value{}, false
		}
		x, _ := v.AsNumber()
		if t.Bits == 32 {
			// Stored as the wire keeps it, so a value reads the same after
			// a round trip.
			f := float32(x)
			if math.IsInf(float64(f), 0) && !math.IsInf(x, 0) {
				return object.Value{}, false
			}
			x = float64(f)
		}
		return object.Float(x), true
	case object.ScalarBool:
		_, ok := v.AsBool()
		return v, ok
	case object.ScalarBytes:
		switch v.Kind() {
		case object.KindBytes:
			x := v.Bytes()
			return object.Bytes(x), true
		case object.KindString, object.KindASCII:
			x := v.Str()
			return object.Bytes([]byte(x)), true
		}
	}
	return object.Value{}, false
}

// outOfRange finds a number in v, or among its elements and keys, that is
// of the kind its scalar type takes but outside the type's range, and
// names where it was: role is "", "element" or "key".
func outOfRange(t *object.FieldType, v object.Value, role string) (object.Value, *object.ScalarType, string, bool) {
	if s := t.Scalar; s != nil {
		switch {
		case s.Kind == object.ScalarInteger && v.IsInteger() && !s.Fits(v),
			s.Kind == object.ScalarFloat && isNumber(v):
			if _, ok := coerceScalar(s, v); !ok {
				return v, s, role, true
			}
		}
		return object.Value{}, nil, "", false
	}
	var elems []object.Value
	switch v.Kind() {
	case object.KindList:
		v := v.List()
		elems = v.Elems
	case object.KindSet:
		v := v.Set()
		elems = v.Elems()
	case object.KindMap:
		m := v.Map()
		if k := object.ScalarField(t.Key); k != nil {
			for _, key := range m.Keys() {
				if n, s, r, found := outOfRange(k, key, "key"); found {
					return n, s, r, true
				}
			}
		}
		elems = m.Values()
	}
	if t.Elem == nil {
		return object.Value{}, nil, "", false
	}
	for _, e := range elems {
		if n, s, r, found := outOfRange(t.Elem, e, "element"); found {
			return n, s, r, true
		}
	}
	return object.Value{}, nil, "", false
}

// isNumber reports an Integer or a Float.
func isNumber(v object.Value) bool { return v.IsInteger() || v.Kind() == object.KindFloat }

// inspectNumber prints a number for an error message.
func inspectNumber(v object.Value) string {
	if v.IsInteger() {
		return object.IntegerDigits(v)
	}
	return strconv.FormatFloat(v.Float(), 'g', -1, 64)
}

// coerceKey checks a map key against the declared key type; a symbol is
// stored as the string it names.
func (in *Interp) coerceKey(kind string, k object.Value) (object.Value, bool) {
	t := object.Scalar(kind)
	if t == nil {
		return object.Value{}, false
	}
	if t.Kind == object.ScalarString && k.Kind() == object.KindSymbol {
		x := k.Str()
		return object.String(x), true
	}
	return coerceScalar(t, k)
}

// describeType names a field type in an error message.
func describeType(t *object.FieldType) string {
	if t.Scalar != nil {
		return article(t.Kind)
	}
	switch t.Kind {
	case "list", "set":
		return "a " + t.Kind + " of " + bare(t.Elem)
	case "map":
		return "a map of " + t.Key + " to " + bare(t.Elem)
	case "ref":
		return article(t.Ref.QualifiedName())
	case "enum":
		return "one of :" + strings.Join(t.Enum, ", :")
	case "proc":
		return "a Proc"
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

// article prefixes a noun with a or an. A u is taken as said "you", as in
// uint32 or User.
func article(noun string) string {
	if strings.ContainsRune("aeioAEIO", rune(noun[0])) {
		return "an " + noun
	}
	return "a " + noun
}

func (in *Interp) evalReserved(n *syntax.Reserved, fr *Frame) (object.Value, error) {
	return in.reserve(reserved{Number: n.Number, Name: n.Name, line: n.Line()}, fr)
}

// reserved is what a reserved statement declares: a field number, and a
// name or "".
type reserved struct {
	Number int64
	Name   string
	line   int
}

func (n reserved) Line() int { return n.line }

// reserve keeps a field number, and a name when n has one, from the
// fields of the class whose body fr runs.
func (in *Interp) reserve(n reserved, fr *Frame) (object.Value, error) {
	cls := fr.DefTarget
	if cls == nil || cls.IsModule || !isClassValue(fr.Self, cls) {
		return object.Value{}, in.raise(object.CameoError, n.Line(), "reserved belongs to a class body")
	}
	if exceptionNumber(cls, n.Number) {
		return object.Value{}, in.raise(object.CameoError, n.Line(), "reserved %d: numbers 1 to %d are the exception's own", n.Number, protogen.ExceptionNumbers)
	}
	if n.Name != "" && protogen.ExceptionPart(cls, n.Name) {
		return object.Value{}, in.raise(object.CameoError, n.Line(), "reserved :%s: an exception of %s carries its %s", n.Name, cls.QualifiedName(), n.Name)
	}
	for _, f := range cls.Fields {
		if f.Number == n.Number {
			return object.Value{}, in.raise(object.CameoError, n.Line(), "reserved %d: field %s already holds that number", n.Number, f.Name)
		}
		if n.Name != "" && f.Name == n.Name {
			return object.Value{}, in.raise(object.CameoError, n.Line(), "reserved :%s: a field of that name is declared", n.Name)
		}
	}
	cls.ReservedNumbers = append(cls.ReservedNumbers, n.Number)
	if n.Name != "" {
		cls.ReservedNames = append(cls.ReservedNames, n.Name)
	}
	return object.Nil, nil
}

func (in *Interp) evalInclude(n *syntax.Include, env *Env, fr *Frame) (object.Value, error) {
	if err := in.includeTarget(n.Extend, fr, n.Line()); err != nil {
		return object.Value{}, err
	}
	v, err := in.eval(n.Module, env, fr)
	if err != nil {
		return object.Value{}, err
	}
	return in.include(v, n.Extend, fr, n.Line())
}

// topLevelInclude reports whether an include in fr is the top level's,
// which takes Cameo alone; extend is never.
func (in *Interp) topLevelInclude(extend bool, fr *Frame) bool {
	return fr.DefTarget == in.mainClass && !extend
}

// includeTarget is the error for an include or extend outside a class or
// module body and not at the top level, or nil.
func (in *Interp) includeTarget(extend bool, fr *Frame, line int) error {
	if in.topLevelInclude(extend, fr) {
		return nil
	}
	if cls := fr.DefTarget; cls == nil || !isClassValue(fr.Self, cls) {
		return in.raise(object.CameoError, line, "include belongs to a class or module body")
	}
	return nil
}

// include adds the module v to the class whose body fr runs, or with
// extend to the class object; at the top level it takes Cameo.
func (in *Interp) include(v object.Value, extend bool, fr *Frame, line int) (object.Value, error) {
	if err := in.includeTarget(extend, fr, line); err != nil {
		return object.Value{}, err
	}
	cls := fr.DefTarget
	if in.topLevelInclude(extend, fr) {
		// At the top level include takes Cameo alone, and brings it to
		// every method, as Ruby's top-level include brings a module to
		// Object. Cameo::Runner belongs in the class that runs the work,
		// where a Ruby program includes it too.
		mod, ok := v.AsClass()
		if ok && mod == object.CameoRunnerModule {
			return object.Value{}, in.raise(object.CameoError, line, "include Cameo::Runner belongs in the class that runs the work; at the top level include Cameo")
		}
		if !ok || mod != object.CameoModule {
			return object.Value{}, in.raise(object.CameoError, line, "include at the top level takes Cameo; another module belongs in a class or module body")
		}
		if !in.cameoTop {
			in.mainClass.Include(object.CameoModule)
			in.cameoTop = true
			in.changedTop()
		}
		return object.Nil, nil
	}
	mod, ok := v.AsClass()
	if !ok || !mod.IsModule {
		return object.Value{}, in.raise(object.TypeError, line, "wrong argument type %s (expected Module)", typeName(v))
	}
	if mod == cls || mod.IsA(cls) {
		return object.Value{}, in.raise(object.ArgumentError, line, "cyclic include detected")
	}
	if extend {
		if mod.Extended != nil {
			if err := mod.Extended(in, cls); err != nil {
				return object.Value{}, withLine(err, line)
			}
		}
		cls.Extend(mod)
	} else {
		cls.Include(mod)
		if mod.Included != nil {
			if err := mod.Included(in, cls); err != nil {
				return object.Value{}, withLine(err, line)
			}
		}
	}
	return object.ClassValue(cls), nil
}

// evalWith is a copy of a value with one key replaced: what chained and
// index assignment rewrite to.
func (in *Interp) evalWith(n *syntax.With, env *Env, fr *Frame) (object.Value, error) {
	target, err := in.eval(n.Target, env, fr)
	if err != nil {
		return object.Value{}, err
	}
	key, err := in.eval(n.Key, env, fr)
	if err != nil {
		return object.Value{}, err
	}
	value, err := in.eval(n.Value, env, fr)
	if err != nil {
		return object.Value{}, err
	}
	out, err := in.with(target, key, value, n.Index)
	return out, withLine(err, n.Line())
}

// with returns target with key set to value: a field on an instance, an
// entry in a map, an element of a list. index is set for `target[key] =
// value`, which a library class may take in a native []=.
func (in *Interp) with(target, key, value object.Value, index bool) (object.Value, error) {
	switch target.Kind() {
	case object.KindInstance:
		t := target.Instance()
		if index {
			if m := userMethod(target, "[]="); m != nil && m.Native != nil {
				return m.Native(in, target, []object.Value{key, value}, nil, object.Value{})
			}
		}
		name, ok := key.AsSymbol()
		if !ok {
			if s, isStr := key.AsString(); isStr {
				name = s
			} else {
				return object.Value{}, in.raise(object.TypeError, 0, "a field of %s is named by a symbol, not %s", t.Cls.QualifiedName(), typeName(key))
			}
		}
		// A class may define with for its own instances, a library's to
		// refuse it: a URI has no setters, so u.host = "x" raises there.
		// An assignment reaches that with, a script's through its body. Or
		// a library class may define a native setter, name=, which an
		// assignment reaches this way: a handle such as a Logger changes
		// in place, a value returns its copy.
		if m := userMethod(target, "with"); m != nil {
			kw := object.NewMap().Put(object.Symbol(string(name)), value)
			if m.Native != nil {
				return m.Native(in, target, nil, kw, object.Value{})
			}
			return in.invoke(m, target, nil, kw, object.Value{}, 0)
		}
		if m := userMethod(target, string(name)+"="); m != nil && m.Native != nil {
			return m.Native(in, target, []object.Value{value}, nil, object.Value{})
		}
		idx, ok := t.Cls.FieldIndex[string(name)]
		if !ok {
			return object.Value{}, in.raise(object.NameError, 0, "%s has no field %s", t.Cls.QualifiedName(), name)
		}
		checked, err := in.checkField(t.Cls.Fields[idx], value, 0)
		if err != nil {
			return object.Value{}, err
		}
		fields := append([]object.Value{}, t.Fields...)
		fields[idx] = checked
		// The Message, what a library keeps on the instance, comes along.
		return object.InstanceValue(&object.Instance{Cls: t.Cls, Fields: fields, Message: t.Message}), nil
	case object.KindException:
		return in.withException(target.Exception(), key, value)
	case object.KindMap:
		t := target.Map()
		return object.MapValue(t.Put(key, value)), nil
	case object.KindList:
		t := target.List()
		i, ok := key.AsInt()
		if !ok {
			return object.Value{}, in.raise(object.TypeError, 0, "a list is indexed by an integer, not %s", typeName(key))
		}
		idx := int(i)
		if idx < 0 {
			idx += t.Len()
		}
		if idx < 0 || idx > t.Len() {
			return object.Value{}, in.raise(object.IndexError, 0, "index %d outside of list bounds: %d...%d", i, -t.Len(), t.Len())
		}
		elems := append([]object.Value{}, t.Elems...)
		if idx == t.Len() {
			elems = append(elems, value)
		} else {
			elems[idx] = value
		}
		return object.NewList(elems...), nil
	}
	return object.Value{}, in.raise(object.TypeError, 0, "cannot assign into %s", typeName(target))
}

// withException returns a copy of an exception with key set to value: its
// message, or a field its class declares. The copy keeps the line and the
// cause.
func (in *Interp) withException(t *object.Exception, key, value object.Value) (object.Value, error) {
	name, ok := key.AsSymbol()
	if !ok {
		if name, ok = key.AsString(); !ok {
			return object.Value{}, in.raise(object.TypeError, 0, "a field of %s is named by a symbol, not %s", t.Cls.QualifiedName(), typeName(key))
		}
	}
	out := t.Copy()
	if name == "message" {
		msg, ok := value.AsString()
		if !ok {
			return object.Value{}, in.raise(object.TypeError, 0, "the message of %s is a String, not %s", t.Cls.QualifiedName(), typeName(value))
		}
		out.Message = msg
		return object.ExceptionValue(out), nil
	}
	idx, ok := t.Cls.FieldIndex[name]
	if !ok {
		return object.Value{}, in.raise(object.NameError, 0, "%s has no field %s", t.Cls.QualifiedName(), name)
	}
	checked, err := in.checkField(t.Cls.Fields[idx], value, 0)
	if err != nil {
		return object.Value{}, err
	}
	out.Fields = defaultFields(t.Cls)
	copy(out.Fields, t.Fields)
	out.Fields[idx] = checked
	return object.ExceptionValue(out), nil
}

// withChanges is `value.with(key: new, …)` or `value.with { |v| { key:
// new, … } }`: a copy with every entry of the keywords, then of the map the
// block returns, applied.
func (in *Interp) withChanges(self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if len(args) > 0 {
		return object.Value{}, in.raise(object.ArgumentError, 0, "with takes the changes as keywords or from its block, not positional arguments")
	}
	if kw == nil && block.IsZero() {
		return object.Value{}, in.raise(object.ArgumentError, 0, "with needs the changes, as keywords or from its block")
	}
	switch self.Kind() {
	case object.KindInstance, object.KindException, object.KindMap, object.KindList:
	default:
		return object.Value{}, in.raise(object.TypeError, 0, "with copies an instance, an exception, a map or a list, not %s", typeName(self))
	}
	out := self
	var err error
	apply := func(m *object.Map) error {
		for i, k := range m.Keys() {
			if out, err = in.with(out, k, m.Values()[i], false); err != nil {
				return err
			}
		}
		return nil
	}
	if kw != nil {
		if err := apply(kw); err != nil {
			return object.Value{}, err
		}
	}
	if !block.IsZero() {
		changes, err := in.callBlock(block, []object.Value{out}, nil, object.Value{})
		if err != nil {
			return object.Value{}, err
		}
		m, ok := changes.AsMap()
		if !ok {
			return object.Value{}, in.raise(object.TypeError, 0, "with's block returns a map of the changes, not %s", typeName(changes))
		}
		if err := apply(m); err != nil {
			return object.Value{}, err
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
	for name, v := range super.Attrs {
		cls.Attrs[name] = v
	}
	return nil
}

// evalAttrDecl declares attributes of the class a body builds, each
// holding its default, nil without one, and gives the class a reader for
// each. A body reopening the class may declare an attribute it has again
// with the same default, which keeps its value; a subclass may declare an
// attribute its parent has, with a default of its own. The class must be
// open: a declaration after the body ended is refused as a write is.
func (in *Interp) evalAttrDecl(n *syntax.AttrDecl, env *Env, fr *Frame) (object.Value, error) {
	if err := in.attrTarget(n.Names, fr, n.Line()); err != nil {
		return object.Value{}, err
	}
	def := object.Nil
	if n.Default != nil {
		v, err := in.eval(n.Default, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		def = v
	}
	return in.declareAttrs(n.Names, def, fr, n.Line())
}

// attrTarget is the error for an attribute declared outside a class body,
// or after the body ended; nil otherwise.
func (in *Interp) attrTarget(names []string, fr *Frame, line int) error {
	cls := fr.DefTarget
	if cls == nil || cls.IsModule || !isClassValue(fr.Self, cls) {
		return in.raise(object.CameoError, line, "attribute :%s: an attribute belongs to a class body", names[0])
	}
	if !cls.Open {
		return in.raise(object.CameoError, line, "attribute :%s: %s is sealed: an attribute is declared while the class body runs", names[0], cls.QualifiedName())
	}
	return nil
}

// declareAttrs declares the attributes names of the class whose body fr
// runs, with the default def.
func (in *Interp) declareAttrs(names []string, def object.Value, fr *Frame, line int) (object.Value, error) {
	if err := in.attrTarget(names, fr, line); err != nil {
		return object.Value{}, err
	}
	cls := fr.DefTarget
	for _, name := range names {
		if before, owner := attrDefault(cls, name); owner != nil {
			switch {
			case fr.Reopened != nil && object.Equal(before, def):
				continue
			case fr.Reopened != nil:
				return object.Value{}, in.raise(object.CameoError, line, "attribute %s is already declared in %s with another default", name, cls.QualifiedName())
			case owner == cls:
				return object.Value{}, in.raise(object.CameoError, line, "attribute %s is already declared in %s", name, cls.QualifiedName())
			}
			// A subclass declares an inherited attribute with a default
			// of its own.
		}
		if m := cls.LookupSingleton(name); m != nil && m.Owner == cls {
			return object.Value{}, in.raise(object.CameoError, line, "attribute %s: %s already defines a class method %s", name, cls.QualifiedName(), name)
		}
		cls.Attrs[name] = def
		cls.AttrDefaults[name] = def
		cls.DefineSingleton(name, 0, attrReader(name))
	}
	return object.Nil, nil
}

// attrDefault returns the default an attribute was declared with, and the
// class that declared it, cls or an ancestor; nil when none did.
func attrDefault(cls *object.Class, name string) (object.Value, *object.Class) {
	for k := cls; k != nil; k = k.Super {
		if def, ok := k.AttrDefaults[name]; ok {
			return def, k
		}
	}
	return object.Value{}, nil
}

// attrReader reads an attribute of the class the method is called on: a
// subclass reads its own copy.
func attrReader(name string) object.NativeFn {
	return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		cls, ok := self.AsClass()
		if !ok {
			return object.Nil, nil
		}
		if v, ok := cls.Attrs[name]; ok {
			return v, nil
		}
		return object.Nil, nil
	}
}

// exceptionNumber reports whether number is one an exception class keeps
// for the parts of its message besides its fields.
func exceptionNumber(cls *object.Class, number int64) bool {
	return cls.IsA(object.ExceptionClass) && number <= protogen.ExceptionNumbers
}

// isClassValue reports whether v is the class cls itself.
func isClassValue(v object.Value, cls *object.Class) bool {
	c, ok := v.AsClass()
	return ok && c == cls
}
