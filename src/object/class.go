package object

import (
	"maps"
	"math/rand/v2"
	"sort"
	"sync/atomic"
)

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
	// lookup caches the ancestors and the resolved methods; see cache.
	lookup atomic.Pointer[lookupCache]
}

// generation counts every change to any class's methods or ancestry. A
// class changes only while its body is being evaluated (and the builtin
// ones while the standard library installs), so after loading the count
// stands still and every lookup cache stays valid.
var generation atomic.Uint64

// lookupCache is what a class has for lookups under one generation:
// its ancestors, and the instance methods resolved so far by name, with
// nil for a name no ancestor defines. A cache is never modified once
// stored; a miss stores a copy with the new entry, so readers on other
// goroutines never see a map being written.
type lookupCache struct {
	gen       uint64
	ancestors []*Class
	methods   map[string]*Method
}

// cache returns the class's lookup cache for the current generation,
// building a fresh one when the stored one is stale.
func (c *Class) cache() *lookupCache {
	gen := generation.Load()
	if lc := c.lookup.Load(); lc != nil && lc.gen == gen {
		return lc
	}
	lc := &lookupCache{gen: gen, ancestors: c.ancestors(), methods: map[string]*Method{}}
	c.lookup.Store(lc)
	return lc
}

// FieldType is the declared type of a field. A scalar type's comes from
// ScalarField, which sets Scalar.
type FieldType struct {
	Kind  string     // a scalar type's name (see ScalarType), or list, set, map, ref, enum
	Elem  *FieldType // list and set element, map value
	Key   string     // map key: a scalar type that MapKey
	Ref   *Class     // the class of a ref field (resolved when the class is built)
	Enum  []string   // the symbols of an enum field
	Named string     // an element or ref type named by a constant, resolved later
	// Scalar is the scalar type when Kind names one, nil otherwise.
	Scalar *ScalarType
}

// Field is one declared field of a class.
type Field struct {
	Name    string
	Number  int64 // the protobuf field number
	Type    *FieldType
	Default Value // zero when the field has no default (then it reads as nil)
	// Owner is the class that declared the field, which may be a superclass.
	Owner *Class
}

// Method is a method: a native one implemented in Go, or a user one whose
// body the evaluator runs. Body is the evaluator's representation (a
// *syntax.Def), which this package does not interpret. A user method that
// `cameo build` compiled has both: Native runs it, and Body takes the calls
// Native leaves to the interpreter.
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
	if c.Name == "" {
		// A class from Class.new before a constant names it.
		return "#<Class>"
	}
	if c.Lexical != nil && c.Lexical != ObjectClass {
		return c.Lexical.QualifiedName() + "::" + c.Name
	}
	return c.Name
}

// Define adds a native instance method.
func (c *Class) Define(name string, arity int, fn NativeFn) {
	c.SetMethod(&Method{Name: name, Owner: c, Native: fn, Arity: arity})
}

// DefineSingleton adds a native class method.
func (c *Class) DefineSingleton(name string, arity int, fn NativeFn) {
	c.SetSingletonMethod(&Method{Name: name, Owner: c, Native: fn, Arity: arity})
}

// SetMethod adds or replaces the instance method m.Name. Every change to a
// class's methods or ancestry goes through one of the Set*, Include and
// Extend methods, which is what keeps the lookup caches honest.
func (c *Class) SetMethod(m *Method) {
	c.Methods[m.Name] = m
	generation.Add(1)
}

// SetSingletonMethod adds or replaces the class method m.Name.
func (c *Class) SetSingletonMethod(m *Method) {
	c.SingletonMethods[m.Name] = m
	generation.Add(1)
}

// Include appends mod to the included modules.
func (c *Class) Include(mod *Class) {
	c.Includes = append(c.Includes, mod)
	generation.Add(1)
}

// Extend appends mod to the extended modules.
func (c *Class) Extend(mod *Class) {
	c.Extends = append(c.Extends, mod)
	generation.Add(1)
}

// Ancestors is the method resolution order: the class, its included modules
// (the last included first), then the superclass's ancestors. The slice is
// the cached one and must not be modified.
func (c *Class) Ancestors() []*Class {
	return c.cache().ancestors
}

// ancestors computes the method resolution order.
func (c *Class) ancestors() []*Class {
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

// Lookup finds the instance method name along the ancestors, or returns
// nil. Each name is resolved once per class and generation.
func (c *Class) Lookup(name string) *Method {
	lc := c.cache()
	if m, ok := lc.methods[name]; ok {
		return m
	}
	var m *Method
	for _, k := range lc.ancestors {
		if found, ok := k.Methods[name]; ok {
			m = found
			break
		}
	}
	methods := make(map[string]*Method, len(lc.methods)+1)
	maps.Copy(methods, lc.methods)
	methods[name] = m
	c.lookup.Store(&lookupCache{gen: lc.gen, ancestors: lc.ancestors, methods: methods})
	return m
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

	ObjectClass  = NewClass("Object", nil)
	ModuleClass  = NewClass("Module", ObjectClass)
	ClassClass   = NewClass("Class", ModuleClass)
	NilClass     = NewClass("NilClass", ObjectClass)
	BoolClass    = NewClass("Boolean", ObjectClass)
	IntegerClass = NewClass("Integer", ObjectClass)
	FloatClass   = NewClass("Float", ObjectClass)
	StringClass  = NewClass("String", ObjectClass)
	SymbolClass  = NewClass("Symbol", ObjectClass)
	// A list and a map are Ruby's Array and Hash: those are the names a
	// script sees, in inspect, in errors and as constants.
	ListClass      = NewClass("Array", ObjectClass)
	MapClass       = NewClass("Hash", ObjectClass)
	SetClass       = NewClass("Set", ObjectClass)
	RangeClass     = NewClass("Range", ObjectClass)
	TimeClass      = NewClass("Time", ObjectClass)
	RegexpClass    = NewClass("Regexp", ObjectClass)
	MatchDataClass = NewClass("MatchData", ObjectClass)
	ProcClass      = NewClass("Proc", ObjectClass)
	// EnumeratorClass is Enumerator; LazyClass, Enumerator::Lazy, is
	// bound inside it.
	// EncodingClass is Encoding: an instance names one, as its one field.
	EncodingClass = NewClass("Encoding", ObjectClass)
	// PathnameClass is Pathname: an instance holds its path, as its one
	// field.
	PathnameClass   = NewClass("Pathname", ObjectClass)
	EnumeratorClass = NewClass("Enumerator", ObjectClass)
	LazyClass       = NewClass("Lazy", EnumeratorClass)
	ExceptionClass  = NewClass("Exception", ObjectClass)
	// Enumerable is the module List, Map, Set and Range include.
	Enumerable = NewModule("Enumerable")
	// Comparable is the module Integer, Float, String, Symbol and Time include.
	Comparable = NewModule("Comparable")
	// MathModule is Math: module functions and constants.
	MathModule = NewModule("Math")
	// ProcessModule is Process, which reads the clocks.
	ProcessModule = NewModule("Process")
	// CameoModule is Cameo, which a script includes for async, await and
	// the promises they settle.
	CameoModule = NewModule("Cameo")
	// CameoRunnerModule is Cameo::Runner, which brings Cameo to the class
	// that includes it: the name a class includes to run its work on cameo
	// and, through the Ruby library of the same name, on CRuby.
	CameoRunnerModule = NewModule("Runner")

	StandardError     = NewClass("StandardError", ExceptionClass)
	RuntimeError      = NewClass("RuntimeError", StandardError)
	ArgumentError     = NewClass("ArgumentError", StandardError)
	TypeError         = NewClass("TypeError", StandardError)
	NameError         = NewClass("NameError", StandardError)
	NoMethodError     = NewClass("NoMethodError", NameError)
	IndexError        = NewClass("IndexError", StandardError)
	KeyError          = NewClass("KeyError", IndexError)
	RangeError        = NewClass("RangeError", StandardError)
	FloatDomainError  = NewClass("FloatDomainError", RangeError)
	ZeroDivisionError = NewClass("ZeroDivisionError", StandardError)
	StopIteration     = NewClass("StopIteration", IndexError)
	NoMatchingPattern = NewClass("NoMatchingPatternError", StandardError)
	FrozenError       = NewClass("FrozenError", RuntimeError)
	RegexpError       = NewClass("RegexpError", StandardError)
	LocalJumpError    = NewClass("LocalJumpError", StandardError)
	CameoError        = NewClass("CameoError", StandardError)
	// ScriptError and LoadError are outside StandardError, so a bare
	// rescue does not catch a failed require, as in Ruby.
	ScriptError = NewClass("ScriptError", ExceptionClass)
	LoadError   = NewClass("LoadError", ScriptError)
	// NotImplementedError is a ScriptError, as in Ruby, so a bare rescue
	// does not catch it either.
	NotImplementedErr = NewClass("NotImplementedError", ScriptError)
	SyntaxError       = NewClass("SyntaxError", ScriptError)
	// UncaughtThrowError is what throw raises without a catch for its tag.
	UncaughtThrowError = NewClass("UncaughtThrowError", ArgumentError)
	// SystemExit is what exit and abort raise; the command ends with its
	// status when nothing rescues it.
	SystemExit = NewClass("SystemExit", ExceptionClass)
	// The classes below are Ruby's for what cameo does not do: I/O,
	// threads, fibers, signals and system calls. They exist so a script can
	// name them in a rescue; the interpreter raises none of them.
	NoMemoryError    = NewClass("NoMemoryError", ExceptionClass)
	SecurityError    = NewClass("SecurityError", ExceptionClass)
	SignalException  = NewClass("SignalException", ExceptionClass)
	Interrupt        = NewClass("Interrupt", SignalException)
	SystemStackError = NewClass("SystemStackError", ExceptionClass)
	IOError          = NewClass("IOError", StandardError)
	EOFError         = NewClass("EOFError", IOError)
	FiberError       = NewClass("FiberError", StandardError)
	ThreadError      = NewClass("ThreadError", StandardError)
	ClosedQueueError = NewClass("ClosedQueueError", StopIteration)
	SystemCallError  = NewClass("SystemCallError", StandardError)
	// EncodingError and the three below it, bound in Encoding, are what
	// combining and converting strings of different encodings raises.
	EncodingError            = NewClass("EncodingError", StandardError)
	CompatibilityError       = NewClass("CompatibilityError", EncodingError)
	UndefinedConversionError = NewClass("UndefinedConversionError", EncodingError)
	InvalidByteSequenceError = NewClass("InvalidByteSequenceError", EncodingError)
	// MathDomainError is Math::DomainError, bound in Math, not at the top.
	MathDomainError = NewClass("DomainError", ArgumentError)
)

func init() {
	ListClass.Include(Enumerable)
	MapClass.Include(Enumerable)
	SetClass.Include(Enumerable)
	RangeClass.Include(Enumerable)
	EnumeratorClass.Include(Enumerable)
	LazyClass.Lexical = EnumeratorClass
	for _, c := range []*Class{CompatibilityError, UndefinedConversionError, InvalidByteSequenceError} {
		c.Lexical = EncodingClass
		EncodingClass.Consts[c.Name] = ClassValue(c)
	}
	EnumeratorClass.Consts["Lazy"] = ClassValue(LazyClass)
	CameoRunnerModule.Lexical = CameoModule
	CameoRunnerModule.Include(CameoModule)
	CameoModule.Consts["Runner"] = ClassValue(CameoRunnerModule)
	IntegerClass.Include(Comparable)
	FloatClass.Include(Comparable)
	StringClass.Include(Comparable)
	SymbolClass.Include(Comparable)
	TimeClass.Include(Comparable)
	BasicClasses = []*Class{
		ObjectClass, ModuleClass, ClassClass, NilClass, BoolClass, IntegerClass, FloatClass,
		StringClass, SymbolClass, ListClass, MapClass, SetClass, RangeClass, TimeClass, RegexpClass, MatchDataClass, EnumeratorClass, EncodingClass, PathnameClass, ProcClass,
		ExceptionClass, Enumerable, Comparable, MathModule, ProcessModule, CameoModule,
		StandardError, RuntimeError, ArgumentError, TypeError, NameError, NoMethodError, KeyError,
		IndexError, RangeError, ZeroDivisionError, StopIteration, NoMatchingPattern, FrozenError,
		NotImplementedErr, LocalJumpError, CameoError, RegexpError, FloatDomainError, ScriptError, LoadError, EncodingError,
		SyntaxError, UncaughtThrowError, SystemExit, NoMemoryError, SecurityError, SignalException, Interrupt,
		SystemStackError, IOError, EOFError, FiberError, ThreadError, ClosedQueueError, SystemCallError,
	}
	for _, c := range BasicClasses {
		ObjectClass.Consts[c.Name] = ClassValue(c)
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
	// Block makes a block a builtin passes to a method: calling it runs f
	// with the block's arguments.
	Block(f func(args []Value) (Value, error)) Value
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
	// Rand returns the interpreter's random number generator, and Srand
	// reseeds it, returning the previous seed. A generator is not shared
	// between goroutines: an async process has its own.
	Rand() *rand.Rand
	Srand(seed uint64) uint64
}
