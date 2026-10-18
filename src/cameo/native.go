package cameo

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"google.golang.org/protobuf/proto"

	"github.com/cameo-lang/cameo/src/eval"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
)

// Class is a native class: a class a Go host registers, whose fields are
// those of a generated protobuf message T and whose methods are Go
// functions. Its instances are immutable like every other; they reach Go
// as values of T.
type Class[T proto.Message] struct {
	cls       *object.Class
	prototype T
}

// Method is a method of a native class. self is the instance it is called
// on as a T, a copy the method may change without changing the instance:
// a method that "changes" it returns the changed copy, which comes back as
// a new instance. Arguments, keywords, the result and errors convert as a
// host method's do; see Func.
type Method[T proto.Message] func(self T, args []any, kwargs map[string]any) (any, error)

var (
	// loaded is set by the first Load: from then on scripts read the
	// constants Register writes.
	loaded atomic.Bool

	registryMu sync.Mutex
	// registeredModules are the modules Register made for the namespaces
	// of the classes it registered.
	registeredModules = map[*object.Class]bool{}
	// registered are the native classes by qualified name.
	registered = map[string]*object.Class{}
)

var constName = regexp.MustCompile(`^[A-Z][A-Za-z0-9_]*$`)

// Register makes a native class named name, such as Geo::Point, for the
// message prototype is an instance of, as protoc-gen-go generates it:
// Register("Geo::Point", &geopb.Point{}). The modules of a qualified name
// are made as they are needed, and shared by the classes registered in
// them. Every program loaded afterwards has the class, as it has Math; no
// script redefines it.
//
// The fields are the message's: scalars, repeated fields as lists, maps
// with a scalar key, enums numbered from 0, and messages whose class was
// registered before. A field without presence that is not set reads as its
// zero value, as in Go. Point.new(x: 1) takes the fields as keywords.
//
// Register panics when the name or the message does not fit, or when a
// program was loaded already: call it from an init function.
func Register[T proto.Message](name string, prototype T) *Class[T] {
	fail := func(format string, args ...any) {
		panic(fmt.Sprintf("cameo.Register(%q): %s", name, fmt.Sprintf(format, args...)))
	}
	if loaded.Load() {
		fail("register classes before the first Load")
	}
	parts := strings.Split(name, "::")
	for _, p := range parts {
		if !constName.MatchString(p) {
			fail("%q is not a constant name", p)
		}
	}
	md := prototype.ProtoReflect().Descriptor()
	if protogen.NativeClass(md.FullName()) != nil {
		fail("the message %s is registered already", md.FullName())
	}

	registryMu.Lock()
	defer registryMu.Unlock()
	consts := object.ObjectClass.Consts
	var lexical *object.Class
	for _, p := range parts[:len(parts)-1] {
		if v, ok := consts[p]; ok {
			mod, isClass := v.AsClass()
			if !isClass || !registeredModules[mod] {
				fail("%s is a builtin constant, not a namespace of registered classes", p)
			}
			lexical, consts = mod, mod.Consts
			continue
		}
		mod := object.NewModule(p)
		mod.Lexical = lexical
		registeredModules[mod] = true
		consts[p] = object.ClassValue(mod)
		lexical, consts = mod, mod.Consts
	}
	clsName := parts[len(parts)-1]
	if _, exists := consts[clsName]; exists {
		fail("%s is defined already", name)
	}

	cls := object.NewClass(clsName, object.ObjectClass)
	cls.Lexical = lexical
	// A digest of its own: a native class is not a builtin one, which has
	// none, and async finds it again by name and digest.
	cls.Digest = "host:" + string(md.FullName())
	fields, err := protogen.Fields(cls, md, protogen.NativeClass)
	if err != nil {
		fail("%v", err)
	}
	for _, f := range fields {
		cls.FieldIndex[f.Name] = len(cls.Fields)
		cls.Fields = append(cls.Fields, f)
		cls.Define(f.Name, 0, fieldReader(f.Name))
	}
	protogen.RegisterNative(cls, prototype)
	consts[clsName] = object.ClassValue(cls)
	registered[cls.QualifiedName()] = cls
	return &Class[T]{cls: cls, prototype: prototype}
}

// DefineMethod defines the instance method name: point.name(args).
func (c *Class[T]) DefineMethod(name string, fn Method[T]) *Class[T] {
	c.cls.Define(name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		cv := conv{ctx.(*eval.Interp)}
		inst, ok := self.AsInstance()
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "%s#%s called on %s", c.cls.QualifiedName(), name, self.Class().QualifiedName())
		}
		msg, err := cv.message(inst, c.prototype)
		if err != nil {
			return object.Value{}, ctx.Raise(object.TypeError, "%s#%s: %s", c.cls.QualifiedName(), name, err)
		}
		return hostCall(ctx, cv, args, kw, func(args []any, kwargs map[string]any) (any, error) {
			return fn(msg.(T), args, kwargs)
		})
	})
	return c
}

// DefineClassMethod defines the class method name: Point.name(args). A
// constructor other than new is one: it returns a T.
func (c *Class[T]) DefineClassMethod(name string, fn Func) *Class[T] {
	c.cls.DefineSingleton(name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return hostCall(ctx, conv{ctx.(*eval.Interp)}, args, kw, fn)
	})
	return c
}

// message is inst as a value of the prototype's type: a copy of an
// instance of the native class, or of a class that inherits from it, whose
// message has the same fields under another name.
func (c conv) message(inst *object.Instance, prototype proto.Message) (proto.Message, error) {
	msg, err := c.in.Proto().GoMessage(inst)
	if err != nil {
		return nil, err
	}
	if msg.ProtoReflect().Descriptor().FullName() == prototype.ProtoReflect().Descriptor().FullName() {
		return msg, nil
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}
	out := prototype.ProtoReflect().New().Interface()
	if err := proto.Unmarshal(data, out); err != nil {
		return nil, err
	}
	return out, nil
}

// fieldReader is the method that reads a field of a native class.
func fieldReader(name string) object.NativeFn {
	return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		v, _ := self.Instance().Field(name)
		return object.OrNil(v), nil
	}
}
