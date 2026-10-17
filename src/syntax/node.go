// Package syntax is amber's front end: it parses a script with go-ruby-parser,
// turns the parser's tree into amber's own AST, rejects what amber leaves out,
// and rewrites chained assignment into copies. The language is in
// doc/design/amber.md.
//
// Every node carries the line it started on. The parser records lines per
// statement, so an expression's line is the line of the statement it is part
// of, which is what an error message needs.
package syntax

// Node is any node of amber's AST.
type Node interface{ Line() int }

// Pos is the source line a node started on, embedded in every node.
type Pos struct{ L int }

// Line returns the 1-based source line, or 0 when unknown.
func (p Pos) Line() int { return p.L }

// Program is a parsed, checked script.
type Program struct {
	Pos
	Body []Node
}

// Literals.

// Nil is the literal nil.
type Nil struct{ Pos }

// Bool is true or false.
type Bool struct {
	Pos
	Value bool
}

// Int is an integer literal. amber integers are 64-bit.
type Int struct {
	Pos
	Value int64
}

// Float is a floating-point literal.
type Float struct {
	Pos
	Value float64
}

// Str is a string literal without interpolation.
type Str struct {
	Pos
	Value string
}

// Interp is an interpolated string: Parts alternate Str and expressions, each
// converted with to_s and concatenated.
type Interp struct {
	Pos
	Parts []Node
}

// Sym is a symbol literal; Name has no leading colon.
type Sym struct {
	Pos
	Name string
}

// List is a list literal. An element may be a Splat.
type List struct {
	Pos
	Elems []Node
}

// Map is a map literal. An entry with a nil Key is a `**expr` double splat.
type Map struct {
	Pos
	Entries []MapEntry
}

// MapEntry is one key-value pair of a Map, or one keyword argument.
type MapEntry struct {
	Key   Node // nil for **Value
	Value Node
}

// Range is `Lo..Hi` or `Lo...Hi`; Lo or Hi may be nil for a beginless or
// endless range.
type Range struct {
	Pos
	Lo, Hi    Node
	Exclusive bool
}

// Self is the receiver of the current method.
type Self struct{ Pos }

// Splat is `*expr` in a list literal or an argument list.
type Splat struct {
	Pos
	Value Node
}

// Variables and constants.

// Var reads a local variable. A declared but unassigned local reads as nil.
type Var struct {
	Pos
	Name string
}

// Assign binds a local variable.
type Assign struct {
	Pos
	Name  string
	Value Node
}

// MultiAssign is `a, b = x, y` or `a, *b = list`. SplatIndex is the index of
// the `*name` target, or -1. Values holds the right-hand expressions; a single
// value that is a list is destructured.
type MultiAssign struct {
	Pos
	Names      []string
	SplatIndex int
	Values     []Node
}

// IVar reads a field of self through its instance variable.
type IVar struct {
	Pos
	Name string
}

// IVarAssign sets a field inside initialize. The checker rejects it anywhere
// else.
type IVarAssign struct {
	Pos
	Name  string
	Value Node
}

// Const reads a constant by its bare name, searched from the lexical scope
// outwards.
type Const struct {
	Pos
	Name string
}

// ScopedConst reads `Scope::Name`; a nil Scope is the top level (`::Name`).
type ScopedConst struct {
	Pos
	Scope Node
	Name  string
}

// ConstAssign binds a constant in the current lexical scope.
type ConstAssign struct {
	Pos
	Name  string
	Value Node
}

// Calls.

// Call is a method call. A nil Recv is a call on self (`puts x`, `foo(1)`).
// Args may contain Splat nodes; KwArgs are the keyword arguments; BlockArg is
// a `&expr` argument and Block a literal block, never both.
type Call struct {
	Pos
	Recv     Node
	Name     string
	Args     []Node
	KwArgs   []MapEntry
	BlockArg Node
	Block    *Block
	Safe     bool // &. : a nil receiver answers nil
}

// Block is a literal block or a lambda body.
type Block struct {
	Pos
	Params Params
	Body   []Node
}

// Params is a parameter list of a method, block or lambda.
type Params struct {
	Positional []Param
	Keywords   []KwParam
	KwRest     string // **name, or ""
	BlockParam string // &name, or ""
}

// Param is one positional parameter: required (Default nil, Splat false),
// optional (Default set) or the one splat (`*name`).
type Param struct {
	Name    string
	Default Node
	Splat   bool
}

// KwParam is a keyword parameter, required when Default is nil and Required.
type KwParam struct {
	Name     string
	Default  Node
	Required bool
}

// Lambda is `-> (x) { … }`, `lambda { … }` or `proc { … }`. Strict is true
// for the lambda forms: argument count is checked and return returns from the
// lambda.
type Lambda struct {
	Pos
	Block  *Block
	Strict bool
}

// Yield calls the block of the enclosing method.
type Yield struct {
	Pos
	Args   []Node
	KwArgs []MapEntry
}

// Super calls the method of the same name in the next ancestor. Forward is a
// bare `super`, passing the enclosing method's own arguments.
type Super struct {
	Pos
	Args     []Node
	KwArgs   []MapEntry
	Forward  bool
	BlockArg Node
	Block    *Block
}

// Control flow.

// Return leaves the enclosing method, or lambda, with Value (nil when absent).
type Return struct {
	Pos
	Value Node
}

// Break leaves the enclosing block's iteration or loop with Value.
type Break struct {
	Pos
	Value Node
}

// Next ends the current block iteration or loop pass with Value.
type Next struct {
	Pos
	Value Node
}

// Retry restarts the begin body from a rescue clause.
type Retry struct{ Pos }

// If is `if`/`unless`/ternary/modifier; elsif chains nest in Else.
type If struct {
	Pos
	Cond Node
	Then []Node
	Else []Node // nil when absent
}

// While is `while`/`until` (an until has its condition negated).
type While struct {
	Pos
	Cond Node
	Body []Node
}

// And is `a && b` / `a and b`; Or is `a || b` / `a or b`; Not is `!a` / `not a`.
type And struct {
	Pos
	Left, Right Node
}

// Or is `a || b`.
type Or struct {
	Pos
	Left, Right Node
}

// Not is `!a`.
type Not struct {
	Pos
	Value Node
}

// Case is `case subject; when …; else …; end`, or the subject-less form.
type Case struct {
	Pos
	Subject Node // nil for the condition form
	Whens   []When
	Else    []Node
}

// When is one `when` clause; a Cond may be a Splat.
type When struct {
	Conds []Node
	Body  []Node
}

// CaseIn is pattern matching: `case subject; in pattern; … end`.
type CaseIn struct {
	Pos
	Subject Node
	Clauses []InClause
	Else    []Node
}

// InClause is one `in PATTERN [if|unless GUARD] BODY`.
type InClause struct {
	Pattern  Pattern
	Guard    Node
	GuardNeg bool
	Body     []Node
}

// MatchPattern is `subject => pattern` (Test false, raises on mismatch) or
// `subject in pattern` (Test true, answers a boolean).
type MatchPattern struct {
	Pos
	Subject Node
	Pattern Pattern
	Test    bool
}

// Pattern is a case/in pattern.
type Pattern interface{ pattern() }

// ValuePattern matches when `Value === subject`.
type ValuePattern struct{ Value Node }

// BindPattern binds the subject to a local; `_` binds nothing.
type BindPattern struct{ Name string }

// ConstPattern matches when `subject.is_a?(Const)`.
type ConstPattern struct{ Const Node }

// BindingPattern is `sub => name`.
type BindingPattern struct {
	Sub  Pattern
	Name string
}

// ListPattern is `[pre, *splat, post]`, optionally `Const[…]`.
type ListPattern struct {
	Const     Node
	Pre       []Pattern
	HasSplat  bool
	SplatName string
	Post      []Pattern
}

// MapPattern is `{key: pattern, **rest}`, optionally `Const(key:)`. A nil
// value pattern binds the key's name.
type MapPattern struct {
	Const    Node
	Keys     []string
	Values   []Pattern
	HasRest  bool
	RestName string
	RestNil  bool
}

// AltPattern is `p1 | p2`.
type AltPattern struct{ Alts []Pattern }

// FindPattern is `[*pre, mid, *post]`.
type FindPattern struct {
	Const    Node
	PreName  string
	Mid      []Pattern
	PostName string
}

func (*ValuePattern) pattern()   {}
func (*BindPattern) pattern()    {}
func (*ConstPattern) pattern()   {}
func (*BindingPattern) pattern() {}
func (*ListPattern) pattern()    {}
func (*MapPattern) pattern()     {}
func (*AltPattern) pattern()     {}
func (*FindPattern) pattern()    {}

// Exceptions.

// Begin is `begin … rescue … else … ensure … end`, also the implicit begin of
// a method body with rescue clauses.
type Begin struct {
	Pos
	Body    []Node
	Rescues []Rescue
	Else    []Node
	Ensure  []Node
}

// Rescue is one rescue clause. Empty Classes rescue StandardError.
type Rescue struct {
	Classes []Node
	Var     string
	Body    []Node
}

// Definitions.

// Def defines a method on the enclosing class or module; Singleton is
// `def self.name`, a class method.
type Def struct {
	Pos
	Name      string
	Params    Params
	Body      []Node
	Singleton bool
}

// ClassDef is `class Name < Super … end`. Digest identifies the class by its
// normalised body: two definitions with equal digests are one class.
type ClassDef struct {
	Pos
	Name   string
	Super  Node // nil when absent
	Body   []Node
	Digest string
}

// ModuleDef is `module Name … end`.
type ModuleDef struct {
	Pos
	Name   string
	Body   []Node
	Digest string
}

// FieldDecl is a typed field declaration in a class body: `string :a, :b`,
// `list :tags, of: :string`, `map :scores, key: :string, value: :int`,
// `ref :origin, Point`, `enum :kind, values: [:a, :b]`.
type FieldDecl struct {
	Pos
	Type    string   // string, int, float, bool, bytes, list, set, map, ref, enum
	Names   []string // one or more field names
	Default Node     // default: expr, or nil
	Of      string   // list/set element type
	Key     string   // map key type
	Value   string   // map value type
	Ref     Node     // the class of a ref field
	Values  []string // the symbols of an enum field
}

// Reserved is `reserved <number> [, :name]` in a class body.
type Reserved struct {
	Pos
	Number int64
	Name   string
}

// Include is `include M` (or `extend M` when Extend) in a class or module body.
type Include struct {
	Pos
	Module Node
	Extend bool
}

// amber's own forms.

// With is a copy of Target with Key replaced by Value: what `target.key = v`
// and `target[key] = v` rewrite to. On an instance Key is a Sym naming a
// field; on a Map it is any key; on a List an index.
type With struct {
	Pos
	Target Node
	Key    Node
	Value  Node
}

// Async is `async recv, :method, *args`: it starts recv.method(*args) on an
// AsyncProcess and answers the process.
type Async struct {
	Pos
	Recv   Node
	Method Node
	Args   []Node
	KwArgs []MapEntry
}

// Await is `await process`, or `await recv, :method, *args` which starts the
// process and waits for it. When Process is nil the call form is meant.
type Await struct {
	Pos
	Process Node
	Recv    Node
	Method  Node
	Args    []Node
	KwArgs  []MapEntry
}

// AwaitAll is `await :all`: it waits for every process started in the current
// scope and answers their results as a list.
type AwaitAll struct{ Pos }
