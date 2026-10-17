package object

import "sort"

// Class is a class or a module. A module is a class that has no superclass,
// no instances and no fields, and that other classes include.
type Class struct {
	Name     string
	IsModule bool
	Super    *Class // nil for a module, and for Object itself
	// Digest identifies a user definition by its body; "" for a builtin class.
	Digest string
	// Methods are instance methods; SingletonMethods the class methods
	// (`def self.x`, and the methods modules give through extend).
	Methods          map[string]*Method
	SingletonMethods map[string]*Method
	// Consts are the constants defined inside the class body: nested classes
	// and modules, and plain constants.
	Consts map[string]Value
	// Includes are the modules included, in inclusion order; Extends those
	// extended.
	Includes []*Class
	Extends  []*Class
	// Lexical is the class or module whose body defined this one, which gives
	// the qualified name (Geo::Point) and the constant lookup path.
	Lexical *Class
	// Fields are the declared fields in declaration order (classes only).
	Fields     []*Field
	FieldIndex map[string]int
	// Reserved holds the reserved field numbers and names.
	ReservedNumbers []int64
	ReservedNames   []string
}

// FieldType is the declared type of a field.
type FieldType struct {
	Kind  string     // string, int, float, bool, bytes, list, set, map, ref, enum
	Elem  *FieldType // list and set element, map value
	Key   string     // map key: string, int, bool
	Ref   *Class     // the class of a ref field (resolved when the class is built)
	Enum  []string   // the symbols of an enum field
	Named string     // an element or ref type named by a constant, resolved later
}

// Field is one declared field of a class.
type Field struct {
	Name    string
	Number  int64 // the protobuf field number
	Type    *FieldType
	Default Value // nil when the field has no default (then it reads as nil)
	// Owner is the class that declared the field, which may be a superclass.
	Owner *Class
}

// Method is a method: a native one implemented in Go, or a user one whose
// body the evaluator runs. Body is the evaluator's representation (a
// *syntax.Def), which this package does not interpret.
type Method struct {
	Name   string
	Owner  *Class
	Native NativeFn
	Body   any
	// Arity is the number of positional parameters a native method takes:
	// -1 for any number. User methods carry their parameters in Body.
	Arity int
	// Lexical is the chain of classes and modules enclosing a user method's
	// definition, innermost last, which constant lookup walks.
	Lexical []*Class
}

// NativeFn implements a method in Go. kw is nil when no keyword arguments
// were passed; block is nil when none was.
type NativeFn func(ctx Context, self Value, args []Value, kw *Map, block Value) (Value, error)

// NewClass makes a class with no methods.
func NewClass(name string, super *Class) *Class {
	return &Class{
		Name:             name,
		Super:            super,
		Methods:          map[string]*Method{},
		SingletonMethods: map[string]*Method{},
		Consts:           map[string]Value{},
		FieldIndex:       map[string]int{},
	}
}

// NewModule makes a module with no methods.
func NewModule(name string) *Class {
	m := NewClass(name, nil)
	m.IsModule = true
	return m
}

func (c *Class) Class() *Class {
	if c.IsModule {
		return ModuleClass
	}
	return ClassClass
}

// QualifiedName is the name with its lexical modules: Geo::Point.
func (c *Class) QualifiedName() string {
	if c.Lexical != nil && c.Lexical != ObjectClass {
		return c.Lexical.QualifiedName() + "::" + c.Name
	}
	return c.Name
}

// Define adds a native instance method.
func (c *Class) Define(name string, arity int, fn NativeFn) {
	c.Methods[name] = &Method{Name: name, Owner: c, Native: fn, Arity: arity}
}

// DefineSingleton adds a native class method.
func (c *Class) DefineSingleton(name string, arity int, fn NativeFn) {
	c.SingletonMethods[name] = &Method{Name: name, Owner: c, Native: fn, Arity: arity}
}

// Ancestors is the method resolution order: the class, its included modules
// (the last included first), then the superclass's ancestors.
func (c *Class) Ancestors() []*Class {
	var out []*Class
	seen := map[*Class]bool{}
	var walk func(k *Class)
	walk = func(k *Class) {
		if k == nil || seen[k] {
			return
		}
		seen[k] = true
		out = append(out, k)
		for i := len(k.Includes) - 1; i >= 0; i-- {
			walk(k.Includes[i])
		}
		walk(k.Super)
	}
	walk(c)
	return out
}

// Lookup finds the instance method name along the ancestors.
func (c *Class) Lookup(name string) *Method {
	for _, k := range c.Ancestors() {
		if m, ok := k.Methods[name]; ok {
			return m
		}
	}
	return nil
}

// LookupAfter finds the instance method name in the ancestors after owner:
// what super calls from a method owner defines.
func (c *Class) LookupAfter(owner *Class, name string) *Method {
	found := false
	for _, k := range c.Ancestors() {
		if found {
			if m, ok := k.Methods[name]; ok {
				return m
			}
		}
		if k == owner {
			found = true
		}
	}
	return nil
}

// LookupSingleton finds the class method name: on the class, the modules it
// extends, then its superclass.
func (c *Class) LookupSingleton(name string) *Method {
	for k := c; k != nil; k = k.Super {
		if m, ok := k.SingletonMethods[name]; ok {
			return m
		}
		for i := len(k.Extends) - 1; i >= 0; i-- {
			if m := k.Extends[i].Lookup(name); m != nil {
				return m
			}
		}
	}
	return nil
}

// IsA reports whether c is k or descends from it or includes it.
func (c *Class) IsA(k *Class) bool {
	for _, a := range c.Ancestors() {
		if a == k {
			return true
		}
	}
	return false
}

// MethodNames lists the instance methods defined on this class alone, sorted.
func (c *Class) MethodNames() []string {
	names := make([]string, 0, len(c.Methods))
	for n := range c.Methods {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// The builtin classes. Their methods are installed by the stdlib package.
var (
	BasicClasses []*Class

	ObjectClass    = NewClass("Object", nil)
	ModuleClass    = NewClass("Module", ObjectClass)
	ClassClass     = NewClass("Class", ModuleClass)
	NilClass       = NewClass("NilClass", ObjectClass)
	BoolClass      = NewClass("Boolean", ObjectClass)
	IntegerClass   = NewClass("Integer", ObjectClass)
	FloatClass     = NewClass("Float", ObjectClass)
	StringClass    = NewClass("String", ObjectClass)
	SymbolClass    = NewClass("Symbol", ObjectClass)
	BytesClass     = NewClass("Bytes", ObjectClass)
	ListClass      = NewClass("Array", ObjectClass)
	MapClass       = NewClass("Hash", ObjectClass)
	SetClass       = NewClass("Set", ObjectClass)
	RangeClass     = NewClass("Range", ObjectClass)
	ProcClass      = NewClass("Proc", ObjectClass)
	ProcessClass   = NewClass("AsyncProcess", ObjectClass)
	ExceptionClass = NewClass("Exception", ObjectClass)
	// Enumerable is the module List, Map, Set and Range include.
	Enumerable = NewModule("Enumerable")
	// Comparable is the module Integer, Float and String include.
	Comparable = NewModule("Comparable")

	StandardError     = NewClass("StandardError", ExceptionClass)
	RuntimeError      = NewClass("RuntimeError", StandardError)
	ArgumentError     = NewClass("ArgumentError", StandardError)
	TypeError         = NewClass("TypeError", StandardError)
	NameError         = NewClass("NameError", StandardError)
	NoMethodError     = NewClass("NoMethodError", NameError)
	KeyError          = NewClass("KeyError", StandardError)
	IndexError        = NewClass("IndexError", StandardError)
	RangeError        = NewClass("RangeError", StandardError)
	ZeroDivisionError = NewClass("ZeroDivisionError", StandardError)
	StopIteration     = NewClass("StopIteration", StandardError)
	NoMatchingPattern = NewClass("NoMatchingPatternError", StandardError)
	FrozenError       = NewClass("FrozenError", RuntimeError)
	NotImplementedErr = NewClass("NotImplementedError", StandardError)
	LocalJumpError    = NewClass("LocalJumpError", StandardError)
	AmberError        = NewClass("AmberError", StandardError)
)

func init() {
	ListClass.Includes = []*Class{Enumerable}
	MapClass.Includes = []*Class{Enumerable}
	SetClass.Includes = []*Class{Enumerable}
	RangeClass.Includes = []*Class{Enumerable}
	IntegerClass.Includes = []*Class{Comparable}
	FloatClass.Includes = []*Class{Comparable}
	StringClass.Includes = []*Class{Comparable}
	BasicClasses = []*Class{
		ObjectClass, ModuleClass, ClassClass, NilClass, BoolClass, IntegerClass, FloatClass,
		StringClass, SymbolClass, BytesClass, ListClass, MapClass, SetClass, RangeClass, ProcClass,
		ProcessClass, ExceptionClass, Enumerable, Comparable,
		StandardError, RuntimeError, ArgumentError, TypeError, NameError, NoMethodError, KeyError,
		IndexError, RangeError, ZeroDivisionError, StopIteration, NoMatchingPattern, FrozenError,
		NotImplementedErr, LocalJumpError, AmberError,
	}
	for _, c := range BasicClasses {
		if c != ObjectClass {
			ObjectClass.Consts[c.Name] = c
		}
	}
}

// Context is what a native method can ask of the evaluator.
type Context interface {
	// Call invokes name on recv with positional arguments and no block.
	Call(recv Value, name string, args ...Value) (Value, error)
	// CallWith invokes name on recv with keyword arguments and a block.
	CallWith(recv Value, name string, args []Value, kw *Map, block Value) (Value, error)
	// Yield calls a block (a *Proc value) with arguments.
	Yield(block Value, args ...Value) (Value, error)
	// Raise makes an exception of class cls with a formatted message.
	Raise(cls *Class, format string, args ...any) error
	// Inspect and ToS render a value, dispatching to user-defined inspect
	// and to_s where a class has them.
	Inspect(v Value) (string, error)
	ToS(v Value) (string, error)
	// Equal and Compare are structural equality and ordering, dispatching to
	// user-defined == and <=> where a class has them.
	Equal(a, b Value) (bool, error)
	Compare(a, b Value) (int, error)
	// Write writes to the script's standard output or standard error.
	Write(stderr bool, s string) error
	// RespondsTo reports whether v has a method name.
	RespondsTo(v Value, name string) bool
}
