package syntax

import (
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"

	"github.com/go-ruby-parser/parser"
	"github.com/go-ruby-parser/parser/ast"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/rx"
)

// context names where the converter stands in the source, which decides what
// a construct means: a `string :x` call is a field declaration in a class
// body and nothing else, `@x = v` is allowed in initialize alone.
type context int

const (
	ctxTop context = iota
	ctxClass
	ctxModule
	ctxDef
	ctxBlock
)

type frame struct {
	kind context
	name string // the method name for ctxDef, the class or module name otherwise
}

type converter struct {
	file  string
	lines map[ast.Node]int
	line  int
	errs  ErrorList
	stack []frame
	// interpolating counts the regexp interpolations being converted, in
	// which a bare name may be a local the parser did not see.
	interpolating int
	// lent names the hash parameters of the Hash.new blocks being
	// converted, which the block must not store into.
	lent []string
	// bangs names the bang methods the script defines, which are calls
	// like any other; any other bang method is one of Ruby's mutators.
	bangs map[string]bool
}

func (c *converter) fail(format string, args ...any) {
	c.errs = append(c.errs, &Error{File: c.file, Line: c.line, Msg: fmt.Sprintf(format, args...)})
}

func (c *converter) pos() Pos { return Pos{L: c.line} }

func (c *converter) push(kind context, name string) { c.stack = append(c.stack, frame{kind, name}) }
func (c *converter) pop()                           { c.stack = c.stack[:len(c.stack)-1] }

// top is the innermost frame.
func (c *converter) top() frame { return c.stack[len(c.stack)-1] }

// inClassBody reports whether the innermost frame is a class body itself,
// not a method inside it.
func (c *converter) inClassBody() bool { return c.top().kind == ctxClass }

// inClassOrModuleBody is inClassBody for modules too.
func (c *converter) inClassOrModuleBody() bool {
	k := c.top().kind
	return k == ctxClass || k == ctxModule
}

// enclosingDef returns the innermost method frame, looking through blocks,
// and false when there is none before a class, module or the top level.
func (c *converter) enclosingDef() (frame, bool) {
	for i := len(c.stack) - 1; i >= 0; i-- {
		switch c.stack[i].kind {
		case ctxDef:
			return c.stack[i], true
		case ctxBlock:
			continue
		default:
			return frame{}, false
		}
	}
	return frame{}, false
}

// insideClass reports whether a class or module body encloses the current
// position at any depth.
func (c *converter) insideClass() bool {
	for _, f := range c.stack {
		if f.kind == ctxClass || f.kind == ctxModule {
			return true
		}
	}
	return false
}

// body converts a statement sequence, keeping each statement's own line and
// restoring the enclosing one afterwards.
func (c *converter) body(nodes []ast.Node) []Node {
	saved := c.line
	out := make([]Node, 0, len(nodes))
	for i, n := range nodes {
		if l, ok := c.lines[n]; ok {
			c.line = l
		}
		if e := c.expr(n); e != nil {
			if call, ok := e.(*Call); ok && i < len(nodes)-1 {
				call.Dropped = true
			}
			out = append(out, e)
		}
	}
	c.line = saved
	return out
}

// exprs converts a list of expressions that are not statements.
func (c *converter) exprs(nodes []ast.Node) []Node {
	out := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		if e := c.expr(n); e != nil {
			out = append(out, e)
		}
	}
	return out
}

// expr converts one node. It returns nil after reporting an error, and the
// caller drops the node; the program is not evaluated when errors exist.
func (c *converter) expr(n ast.Node) Node {
	if l, ok := c.lines[n]; ok {
		c.line = l
	}
	switch n := n.(type) {
	case nil:
		return nil
	case *ast.IntLit:
		return &Int{c.pos(), n.Value}
	case *ast.BignumLit:
		return &BigInt{c.pos(), n.Val.String()}
	case *ast.FloatLit:
		return &Float{c.pos(), n.Value}
	case *ast.RationalLit:
		c.fail("rational literals are not supported")
		return nil
	case *ast.ImaginaryLit:
		c.fail("imaginary literals are not supported")
		return nil
	case *ast.StringLit:
		return &Str{c.pos(), n.Value}
	case *ast.StrInterp:
		return &Interp{c.pos(), c.exprs(n.Parts)}
	case *ast.SymbolLit:
		return &Sym{c.pos(), n.Name}
	case *ast.RegexpLit:
		return c.regexp(n)
	case *ast.XStr:
		c.fail("backtick commands are not supported: cameo has no shell")
		return nil
	case *ast.ArrayLit:
		return &List{c.pos(), c.exprs(n.Elems)}
	case *ast.HashLit:
		return &Map{c.pos(), c.entries(n)}
	case *ast.RangeLit:
		return &Range{c.pos(), c.expr(n.Lo), c.expr(n.Hi), n.Exclusive}
	case *ast.BoolLit:
		return &Bool{c.pos(), n.Value}
	case *ast.NilLit:
		return &Nil{c.pos()}
	case *ast.SelfLit:
		return &Self{c.pos()}
	case *ast.VarRef:
		return &Var{c.pos(), n.Name}
	case *ast.Assign:
		value := c.expr(n.Value)
		markRebinds(value, &Var{Name: n.Name})
		return &Assign{c.pos(), n.Name, value}
	case *ast.OpAssign:
		return c.opAssign(n)
	case *ast.MultiAssign:
		if n.Targets != nil {
			c.fail("multiple assignment binds local variables only")
			return nil
		}
		return &MultiAssign{c.pos(), n.Names, n.SplatIndex, c.exprs(n.Values)}
	case *ast.BinaryExpr:
		return c.binary(n)
	case *ast.UnaryExpr:
		return c.unary(n)
	case *ast.Call:
		return c.call(n)
	case *ast.SplatArg:
		return &Splat{c.pos(), c.expr(n.Value)}
	case *ast.BlockPass:
		c.fail("a &block argument belongs in an argument list")
		return nil
	case *ast.ForwardArgs:
		c.fail("argument forwarding with ... is not supported")
		return nil
	case *ast.If:
		return c.ifExpr(n)
	case *ast.While:
		return &While{c.pos(), c.expr(n.Cond), c.body(n.Body)}
	case *ast.For:
		return c.forLoop(n)
	case *ast.MethodDef:
		return c.def(n)
	case *ast.Return:
		return &Return{c.pos(), c.expr(n.Value)}
	case *ast.Break:
		return &Break{c.pos(), c.expr(n.Value)}
	case *ast.Next:
		return &Next{c.pos(), c.expr(n.Value)}
	case *ast.Retry:
		return &Retry{c.pos()}
	case *ast.ConstRef:
		return &Const{c.pos(), n.Name}
	case *ast.ScopedConst:
		return c.scopedConst(n)
	case *ast.ConstAssign:
		if _, inDef := c.enclosingDef(); inDef {
			c.fail("a constant cannot be assigned inside a method")
			return nil
		}
		return &ConstAssign{c.pos(), n.Name, c.expr(n.Value)}
	case *ast.ScopedConstAssign:
		c.fail("assignment to a scoped constant is not supported: define it inside its module")
		return nil
	case *ast.GVarRef, *ast.GVarAssign:
		c.fail("global variables are not supported")
		return nil
	case *ast.CVarRef, *ast.CVarAssign:
		c.fail("class variables are not supported")
		return nil
	case *ast.IvarRef:
		name := strings.TrimPrefix(n.Name, "@")
		c.fail("@%s: cameo has no instance variables; a field is read with its reader, %s or self.%s", name, name, name)
		return nil
	case *ast.IvarAssign:
		name := strings.TrimPrefix(n.Name, "@")
		c.fail("@%s = …: cameo has no instance variables; initialize sets a field with self.%s = …", name, name)
		return nil
	case *ast.ClassDef:
		return c.classDef(n)
	case *ast.ModuleDef:
		return c.moduleDef(n)
	case *ast.SingletonClassDef:
		c.fail("class << self is not supported: define class methods with def self.name")
		return nil
	case *ast.Super:
		if _, inDef := c.enclosingDef(); !inDef {
			c.fail("super outside a method")
			return nil
		}
		s := &Super{Pos: c.pos(), Forward: n.Forward}
		s.Args, s.KwArgs, s.BlockArg = c.args(n.Args)
		if n.Block != nil {
			s.Block = c.block(n.Block)
		}
		return s
	case *ast.Yield:
		if _, inDef := c.enclosingDef(); !inDef {
			c.fail("yield outside a method")
			return nil
		}
		y := &Yield{Pos: c.pos()}
		var blockArg Node
		y.Args, y.KwArgs, blockArg = c.args(n.Args)
		if blockArg != nil {
			c.fail("yield takes no &block")
		}
		return y
	case *ast.Begin:
		return c.begin(n)
	case *ast.Case:
		return c.caseExpr(n)
	case *ast.CaseIn:
		return c.caseIn(n)
	case *ast.MatchPattern:
		return &MatchPattern{c.pos(), c.expr(n.Subject), c.pattern(n.Pattern), n.Bool}
	case *ast.Alias:
		if n.NewNameExpr != nil || n.OldNameExpr != nil {
			c.fail("alias with a computed name is not supported")
			return nil
		}
		return &Alias{c.pos(), n.NewName, n.OldName}
	case *ast.Undef:
		c.fail("undef is not supported: a method is defined once")
		return nil
	default:
		c.fail("unsupported syntax: %T", n)
		return nil
	}
}

// entries converts a hash literal's pairs; a nil key is a double splat.
func (c *converter) entries(h *ast.HashLit) []MapEntry {
	out := make([]MapEntry, 0, len(h.Keys))
	for i, k := range h.Keys {
		var key Node
		if k != nil {
			key = c.expr(k)
		}
		if k == nil && h.Values[i] == nil {
			// A bare `**` passes the anonymous keyword rest on.
			out = append(out, MapEntry{Value: c.anonymous(kwRestParam)})
			continue
		}
		out = append(out, MapEntry{Key: key, Value: c.expr(h.Values[i])})
	}
	return out
}

func (c *converter) opAssign(n *ast.OpAssign) Node {
	read := &Var{c.pos(), n.Name}
	value := c.expr(n.Value)
	var rhs Node
	switch n.Op {
	case "||":
		rhs = &Or{c.pos(), read, value}
	case "&&":
		rhs = &And{c.pos(), read, value}
	case "<<":
		rhs = &Call{Pos: c.pos(), Recv: read, Name: n.Op, Args: []Node{value}, Rebinds: true}
	default:
		rhs = &Call{Pos: c.pos(), Recv: read, Name: n.Op, Args: []Node{value}}
	}
	return &Assign{c.pos(), n.Name, rhs}
}

func (c *converter) binary(n *ast.BinaryExpr) Node {
	switch n.Op {
	case "&&":
		return &And{c.pos(), c.expr(n.Left), c.expr(n.Right)}
	case "||":
		return &Or{c.pos(), c.expr(n.Left), c.expr(n.Right)}
	}
	return &Call{Pos: c.pos(), Recv: c.expr(n.Left), Name: n.Op, Args: []Node{c.expr(n.Right)}}
}

func (c *converter) unary(n *ast.UnaryExpr) Node {
	switch n.Op {
	case "!":
		return &Not{c.pos(), c.expr(n.Operand)}
	case "-":
		switch lit := n.Operand.(type) {
		case *ast.IntLit:
			return &Int{c.pos(), -lit.Value}
		case *ast.BignumLit:
			return &BigInt{c.pos(), new(big.Int).Neg(lit.Val).String()}
		case *ast.FloatLit:
			return &Float{c.pos(), -lit.Value}
		}
		return &Call{Pos: c.pos(), Recv: c.expr(n.Operand), Name: "-@"}
	case "+":
		return c.expr(n.Operand)
	}
	return &Call{Pos: c.pos(), Recv: c.expr(n.Operand), Name: n.Op}
}

func (c *converter) ifExpr(n *ast.If) Node {
	out := &If{Pos: c.pos(), Cond: c.expr(n.Cond), Then: c.body(n.Then)}
	tail := out
	for _, e := range n.Elsifs {
		next := &If{Pos: c.pos(), Cond: c.expr(e.Cond), Then: c.body(e.Body)}
		tail.Else = []Node{next}
		tail = next
	}
	if n.Else != nil {
		tail.Else = c.body(n.Else)
	}
	return out
}

func (c *converter) scopedConst(n *ast.ScopedConst) Node {
	var scope Node
	if !n.Global {
		scope = c.expr(n.Recv)
	}
	return &ScopedConst{c.pos(), scope, n.Name}
}

// fieldAssign is `self.name = value`, which sets a field while initialize
// builds the instance; anywhere else there is nothing to set.
func (c *converter) fieldAssign(name string, value Node) Node {
	if !c.inInitialize() {
		c.fail("self.%s = …: a field is set in initialize only; elsewhere use with", name)
		return nil
	}
	markRebinds(value, &Call{Recv: &Self{}, Name: name})
	return &FieldAssign{c.pos(), name, value}
}

// markRebinds sets Rebinds on value when it is a call on target, the
// variable or field the assignment binds: `a = a << 1` rebinds a to Ruby's
// result.
func markRebinds(value, target Node) {
	call, ok := value.(*Call)
	if !ok || call.Recv == nil {
		return
	}
	switch t := target.(type) {
	case *Var:
		if r, ok := call.Recv.(*Var); ok && r.Name == t.Name {
			call.Rebinds = true
		}
	case *Call:
		if r, ok := call.Recv.(*Call); ok && r.Name == t.Name && len(r.Args) == 0 && r.Block == nil {
			if _, self := r.Recv.(*Self); self {
				call.Rebinds = true
			}
		}
	}
}

// inInitialize reports a position inside a class's initialize.
func (c *converter) inInitialize() bool {
	def, ok := c.enclosingDef()
	return ok && def.name == "initialize" && c.insideClass()
}

// Refused is why cameo refuses a call of name whatever its receiver: a
// bang method of Ruby's core, or a method cameo leaves out. ok is false for
// any other name.
func Refused(name string) (msg string, ok bool) {
	if alt, listed := unsupported[name]; listed {
		return fmt.Sprintf("%s is not supported; use %s", name, alt), true
	}
	if IsBang(name) {
		return BangMessage("x", name), true
	}
	return "", false
}

// IsBang reports whether name is a bang method, such as compact! or
// merge!; the operators ! and != and exit! are not.
func IsBang(name string) bool {
	return strings.HasSuffix(name, "!") && name != "!" && name != "!=" && name != "exit!"
}

// BangMessage is the error for a call of a bang method of Ruby's core on
// recv, which names the copying form.
func BangMessage(recv, name string) string {
	return fmt.Sprintf("This is not supported, use %s = %s.%s instead", recv, recv, strings.TrimSuffix(name, "!"))
}

// MutationMessage is the error for a call of one of Ruby's mutators on a
// value of the kind it changes, naming the rebinding and the non-mutating
// forms.
func MutationMessage(name string) string {
	msg := "cameo does not support mutating calls without rebinding"
	alts := []string{}
	if rebind, ok := rebindForms[name]; ok {
		alts = append(alts, rebind)
	}
	if alt, ok := mutationAlternatives[name]; ok {
		alts = append(alts, alt)
	}
	if len(alts) > 0 {
		msg += "; use " + strings.Join(alts, ", ")
	}
	return msg
}

// rebindForms are the mutators whose Ruby value is the changed receiver,
// so that assigning it back is the rebinding form: `a = a << 1`. The
// others return what they removed or found, which rebinding would bind
// instead.
var rebindForms = map[string]string{
	"<<":      "a = a << 1",
	"push":    "a = a.push(x)",
	"unshift": "a = a.unshift(x)",
	"insert":  "a = a.insert(i, x)",
	"concat":  "a = a.concat(x)",
	"prepend": "a = a.prepend(x)",
	"replace": "a = a.replace(x)",
	"update":  "a = a.update(x)",
	"merge":   "a = a.merge(x)",
	"add":     "a = a.add(x)",
}

// ReturnsCopy reports whether name is a mutator of Ruby's that cameo keeps
// as a method returning the changed copy: force_encoding, which nothing
// else replaces.
func ReturnsCopy(name string) bool { return name == "force_encoding" }

// DroppedMessage is the error for a call of such a method whose value the
// script drops.
func DroppedMessage(name string) string {
	return fmt.Sprintf("%s returns a copy in cameo and changes nothing, and this call drops it; use x = x.%s(...)", name, name)
}

// mutationAlternatives are the non-mutating forms of Ruby's mutators,
// which the evaluator names when a script calls one.
var mutationAlternatives = map[string]string{
	"<<":              "+, or append",
	"push":            "+, or append",
	"pop":             "last and take",
	"shift":           "first and drop",
	"unshift":         "[x] + list",
	"insert":          "a literal with a splat, or +",
	"concat":          "+",
	"prepend":         "+",
	"delete":          "without, or - on a set",
	"delete_at":       "take and drop",
	"delete_if":       "reject",
	"keep_if":         "select",
	"clear":           "an empty literal",
	"store":           "put",
	"update":          "merge",
	"replace":         "assignment",
	"add":             "| [x]",
	"add?":            "| [x]",
	"delete?":         "- [x]",
	"merge":           "|",
	"subtract":        "-",
	"setbyte":         "byteslice and +",
	"bytesplice":      "byteslice and +",
	"append_as_bytes": "+",
}

// unsupported are methods cameo leaves out on every receiver, with what
// to use instead.
var unsupported = map[string]string{
	"fill":                  "a list literal",
	"[]=":                   "assignment, which copies",
	"define_method":         "def",
	"instance_variable_set": "initialize",
	"instance_variable_get": "the field's reader",
	"instance_eval":         "nothing",
	"class_eval":            "nothing",
	"eval":                  "nothing",
	"require_relative":      "nothing: a script is one file",
	"load":                  "nothing: a script is one file",
	"fork":                  "async",
	"exit!":                 "exit",
}

// libraryFunctions are module functions of a library cameo builds in whose
// names read as a mutating or unsupported call: JSON.parse! changes nothing,
// and JSON.load reads a string, not a file.
var libraryFunctions = map[string]map[string]bool{"JSON": {"parse!": true, "load": true}}

// isLibraryFunction reports a call of one of libraryFunctions.
func isLibraryFunction(n *ast.Call) bool {
	r, isConst := n.Recv.(*ast.ConstRef)
	return isConst && libraryFunctions[r.Name][n.Name]
}

// compositeFieldTypes are the typed field helpers a class body takes
// besides the scalar types of object.Scalar.
var compositeFieldTypes = map[string]bool{"list": true, "set": true, "map": true, "ref": true, "enum": true}

// isFieldType reports a typed field helper.
func isFieldType(name string) bool { return compositeFieldTypes[name] || object.Scalar(name) != nil }

func (c *converter) call(n *ast.Call) Node {
	name := n.Name
	if c.interpolating > 0 && n.Recv == nil && len(n.Args) == 0 && n.Block == nil && !n.Paren {
		return &VarOrCall{c.pos(), name}
	}
	if cnst, ok := n.Recv.(*ast.ConstRef); ok && cnst.Name == "Cameo" && (name == "async" || name == "await") {
		if name == "async" {
			return c.async(n, true)
		}
		return c.await(n, true)
	}
	if n.Recv == nil {
		switch name {
		case "defined?":
			if len(n.Args) == 1 {
				return &Defined{c.pos(), c.expr(n.Args[0])}
			}
		case "lambda", "proc":
			if n.Block == nil || len(n.Args) > 0 {
				c.fail("%s takes a block and nothing else", name)
				return nil
			}
			return &Lambda{c.pos(), c.block(n.Block), name == "lambda"}
		case "async":
			return c.async(n, false)
		case "await":
			return c.await(n, false)
		case "attr_reader", "attr_writer", "attr_accessor", "attr":
			c.fail("%s is not supported: a field declaration defines its reader, and there are no setters", name)
			return nil
		case "private", "protected", "public", "module_function":
			c.fail("%s is not supported: every method is public", name)
			return nil
		case "reserved":
			if c.inClassBody() {
				return c.reserved(n)
			}
		case "include", "extend":
			// At the top level include takes Cameo, which the evaluator
			// checks; see Include.
			if c.inClassOrModuleBody() || name == "include" && c.top().kind == ctxTop {
				return c.include(n)
			}
		}
		if isFieldType(name) && len(n.Args) > 0 {
			if _, isSym := n.Args[0].(*ast.SymbolLit); isSym {
				if !c.inClassBody() {
					c.fail("%s :%s declares a field, which only a class body can do", name, n.Args[0].(*ast.SymbolLit).Name)
					return nil
				}
				return c.fieldDecl(n)
			}
		}
	}
	lib := isLibraryFunction(n)
	if IsBang(name) && !lib && !c.bangs[name] {
		c.fail("%s", BangMessage(receiverName(n.Recv), name))
		return nil
	}
	if msg, refused := Refused(name); refused && name != "[]=" && !lib && !c.bangs[name] {
		c.fail("%s", msg)
		return nil
	}
	if isSetter(name) {
		return c.assignment(n)
	}
	if name == "tap" && n.Block != nil && len(n.Block.Body) > 0 {
		if _, isHash := n.Block.Body[len(n.Block.Body)-1].(*ast.HashLit); isHash {
			c.fail("tap returns its receiver unchanged; for a copy with changes use with")
			return nil
		}
	}
	call := &Call{Pos: c.pos(), Name: name, Safe: n.Safe}
	if n.Recv != nil {
		call.Recv = c.expr(n.Recv)
	}
	call.Args, call.KwArgs, call.BlockArg = c.args(n.Args)
	if n.Block != nil {
		if call.BlockArg != nil {
			c.fail("both a &block argument and a literal block")
		}
		if r, isConst := n.Recv.(*ast.ConstRef); isConst && r.Name == "Hash" && name == "new" && len(n.Block.Params) > 0 {
			c.lent = append(c.lent, n.Block.Params[0])
			defer func() { c.lent = c.lent[:len(c.lent)-1] }()
		}
		if r, isConst := n.Recv.(*ast.ConstRef); isConst && r.Name == "Class" && name == "new" {
			// Class.new's block is a class body: defs and field
			// declarations belong to the class it builds.
			call.Block = c.blockAs(n.Block, ctxClass)
		} else {
			call.Block = c.block(n.Block)
		}
	}
	return call
}

// isSetter reports whether a call name is an attribute or index assignment:
// `x=` or `[]=`, as opposed to the comparison operators that also end in =.
func isSetter(name string) bool {
	if name == "[]=" {
		return true
	}
	if !strings.HasSuffix(name, "=") || len(name) < 2 {
		return false
	}
	switch name {
	case "==", "!=", "<=", ">=", "===":
		return false
	}
	return true
}

// args splits a call's argument list into positional arguments (with splats),
// keyword arguments, and the &block argument.
func (c *converter) args(nodes []ast.Node) (args []Node, kwargs []MapEntry, blockArg Node) {
	for i, a := range nodes {
		switch a := a.(type) {
		case *ast.SplatArg:
			if a.Value == nil {
				args = append(args, &Splat{c.pos(), c.anonymous(restParam)})
				continue
			}
			args = append(args, &Splat{c.pos(), c.expr(a.Value)})
		case *ast.BlockPass:
			if a.Value == nil {
				blockArg = c.anonymous(blockParam)
				continue
			}
			blockArg = c.expr(a.Value)
		case *ast.HashLit:
			if !a.Braced && i == len(nodes)-1 || (!a.Braced && i >= len(nodes)-2 && isBlockPass(nodes[len(nodes)-1])) {
				kwargs = append(kwargs, c.entries(a)...)
			} else {
				args = append(args, c.expr(a))
			}
		case *ast.ForwardArgs:
			// `...` passes the enclosing method's forwarded arguments on.
			args = append(args, &Splat{c.pos(), c.anonymous(restParam)})
			kwargs = append(kwargs, MapEntry{Value: c.anonymous(kwRestParam)})
			blockArg = c.anonymous(blockParam)
		default:
			if e := c.expr(a); e != nil {
				args = append(args, e)
			}
		}
	}
	return
}

func isBlockPass(n ast.Node) bool {
	_, ok := n.(*ast.BlockPass)
	return ok
}

func (c *converter) block(b *ast.Block) *Block { return c.blockAs(b, ctxBlock) }

// blockAs converts a block whose body is in the given context: a block's
// own, or a class body's for Class.new.
func (c *converter) blockAs(b *ast.Block, kind context) *Block {
	saved := c.line
	if b.Line != 0 {
		c.line = b.Line
	}
	out := &Block{Pos: c.pos()}
	for i, name := range b.Params {
		p := Param{Name: name, Splat: i == b.SplatIndex}
		if i < len(b.Defaults) && b.Defaults[i] != nil {
			p.Default = c.expr(b.Defaults[i])
		}
		out.Params.Positional = append(out.Params.Positional, p)
	}
	out.Params.BlockParam = b.BlockParam
	c.push(kind, "")
	out.Body = c.body(b.Body)
	c.pop()
	c.line = saved
	return out
}

func (c *converter) params(d *ast.MethodDef) Params {
	var p Params
	for i, name := range d.Params {
		param := Param{Name: name, Splat: i == d.SplatIndex}
		if i < len(d.Defaults) && d.Defaults[i] != nil {
			param.Default = c.expr(d.Defaults[i])
		}
		p.Positional = append(p.Positional, param)
	}
	for _, k := range d.KwParams {
		kw := KwParam{Name: k.Name, Required: k.Default == nil}
		if k.Default != nil {
			kw.Default = c.expr(k.Default)
		}
		p.Keywords = append(p.Keywords, kw)
	}
	p.KwRest = d.KwRest
	p.BlockParam = d.BlockParam
	if d.Forward {
		// `...` takes the anonymous rest, keyword rest and block, under the
		// names a bare `*`, `**` and `&` take, which no script can write.
		p.Positional = append(p.Positional, Param{Name: restParam, Splat: true})
		p.KwRest, p.BlockParam = kwRestParam, blockParam
	}
	return p
}

// The parameters `*`, `**` and `&` without a name, and `...`, bind these
// names, which are not identifiers and so cannot clash with a script's.
const (
	restParam   = "*"
	kwRestParam = "**"
	blockParam  = "&"
)

// anonymous reads an anonymous parameter, which a bare `*`, `**` or `&`
// argument passes on.
func (c *converter) anonymous(name string) Node { return &Var{c.pos(), name} }

// forLoop rewrites `for x in xs ... end` into `xs.each { |v| x = v ... }`,
// declaring x first, since a for loop's variables outlive it in Ruby.
func (c *converter) forLoop(n *ast.For) Node {
	if n.Target != nil {
		c.fail("for with a nested or non-local target is not supported")
		return nil
	}
	const item = "for"
	var bind Node
	if len(n.Vars) == 1 {
		bind = &Assign{c.pos(), n.Vars[0], &Var{c.pos(), item}}
	} else {
		bind = &MultiAssign{c.pos(), n.Vars, -1, []Node{&Var{c.pos(), item}}}
	}
	block := &Block{Pos: c.pos()}
	block.Params.Positional = []Param{{Name: item}}
	c.push(ctxBlock, "")
	block.Body = append([]Node{bind}, c.body(n.Body)...)
	c.pop()
	out := []Node{}
	for _, v := range n.Vars {
		out = append(out, &Assign{c.pos(), v, &Nil{c.pos()}})
	}
	iter := c.expr(n.Iter)
	out = append(out, &Call{Pos: c.pos(), Recv: iter, Name: "each", Block: block}, iter)
	return &Seq{c.pos(), out}
}

func (c *converter) def(d *ast.MethodDef) Node {
	if d.Recv != nil {
		c.fail("def %s on an object: methods belong to classes", d.Name)
		return nil
	}
	if isSetter(d.Name) {
		c.fail("def %s: setters are not supported; a field changes through with", d.Name)
		return nil
	}
	if _, inDef := c.enclosingDef(); inDef {
		c.fail("def %s inside a method", d.Name)
		return nil
	}
	singleton := d.Singleton
	switch {
	case singleton && c.top().kind == ctxTop:
		// At the top level self is main, and its methods are the top-level
		// methods.
		singleton = false
	case singleton && !c.inClassOrModuleBody():
		c.fail("def self.%s outside a class body", d.Name)
		return nil
	}
	out := &Def{Pos: c.pos(), Name: d.Name, Singleton: singleton}
	out.Params = c.params(d)
	c.push(ctxDef, d.Name)
	out.Body = c.body(d.Body)
	c.pop()
	return out
}

func (c *converter) classDef(d *ast.ClassDef) Node {
	var scope Node
	if d.NamePath != nil {
		if scope = c.scopeOf(d.NamePath); scope == nil {
			c.fail("class %s with a top-level scope (::) is not supported", d.Name)
			return nil
		}
	}
	if _, inDef := c.enclosingDef(); inDef {
		c.fail("class %s inside a method", d.Name)
		return nil
	}
	out := &ClassDef{Pos: c.pos(), Name: d.Name, Scope: scope}
	switch {
	case d.SuperExpr != nil:
		out.Super = c.expr(d.SuperExpr)
	case d.Super != "":
		out.Super = &Const{c.pos(), d.Super}
	}
	c.push(ctxClass, d.Name)
	out.Body = c.body(d.Body)
	c.pop()
	out.Digest = digest(out)
	return out
}

func (c *converter) moduleDef(d *ast.ModuleDef) Node {
	var scope Node
	if d.NamePath != nil {
		if scope = c.scopeOf(d.NamePath); scope == nil {
			c.fail("module %s with a top-level scope (::) is not supported", d.Name)
			return nil
		}
	}
	if _, inDef := c.enclosingDef(); inDef {
		c.fail("module %s inside a method", d.Name)
		return nil
	}
	out := &ModuleDef{Pos: c.pos(), Name: d.Name, Scope: scope}
	c.push(ctxModule, d.Name)
	out.Body = c.body(d.Body)
	c.pop()
	out.Digest = digest(out)
	return out
}

func (c *converter) begin(b *ast.Begin) Node {
	out := &Begin{Pos: c.pos(), Body: c.body(b.Body)}
	for _, r := range b.Rescues {
		if r.VarTarget != nil {
			c.fail("rescue => target: an exception is caught into a local variable only")
			continue
		}
		out.Rescues = append(out.Rescues, Rescue{Classes: c.exprs(r.Classes), Var: r.Var, Body: c.body(r.Body)})
	}
	if b.ElseBody != nil {
		out.Else = c.body(b.ElseBody)
	}
	if b.EnsureBody != nil {
		out.Ensure = c.body(b.EnsureBody)
	}
	return out
}

func (c *converter) caseExpr(n *ast.Case) Node {
	out := &Case{Pos: c.pos(), Subject: c.expr(n.Subject)}
	for _, w := range n.Whens {
		out.Whens = append(out.Whens, When{Conds: c.exprs(w.Conds), Body: c.body(w.Body)})
	}
	if n.Else != nil {
		out.Else = c.body(n.Else)
	}
	return out
}

func (c *converter) caseIn(n *ast.CaseIn) Node {
	out := &CaseIn{Pos: c.pos(), Subject: c.expr(n.Subject)}
	for _, cl := range n.Clauses {
		out.Clauses = append(out.Clauses, InClause{
			Pattern:  c.pattern(cl.Pattern),
			Guard:    c.expr(cl.Guard),
			GuardNeg: cl.GuardNeg,
			Body:     c.body(cl.Body),
		})
	}
	if n.Else != nil {
		out.Else = c.body(n.Else)
	}
	return out
}

func (c *converter) patterns(ps []ast.Pattern) []Pattern {
	out := make([]Pattern, 0, len(ps))
	for _, p := range ps {
		out = append(out, c.pattern(p))
	}
	return out
}

func (c *converter) pattern(p ast.Pattern) Pattern {
	switch p := p.(type) {
	case nil:
		return nil
	case *ast.ValuePattern:
		return &ValuePattern{c.expr(p.Value)}
	case *ast.BindPattern:
		return &BindPattern{p.Name}
	case *ast.ConstPattern:
		return &ConstPattern{c.expr(p.Const)}
	case *ast.BindingPattern:
		return &BindingPattern{c.pattern(p.Sub), p.Name}
	case *ast.ArrayPattern:
		return &ListPattern{Const: c.expr(p.Const), Pre: c.patterns(p.Pre), HasSplat: p.HasSplat, SplatName: p.SplatName, Post: c.patterns(p.Post)}
	case *ast.HashPattern:
		return &MapPattern{Const: c.expr(p.Const), Keys: p.Keys, Values: c.patterns(p.Values), HasRest: p.HasRest, RestName: p.RestName, RestNil: p.RestNil}
	case *ast.AltPattern:
		// go-ruby-parser v0.11.1 attaches `=> name` to the last alternative,
		// `A | (B => name)`; Ruby binds the whole alternation,
		// `(A | B) => name`.
		last := len(p.Alts) - 1
		if b, ok := p.Alts[last].(*ast.BindingPattern); ok {
			alts := append(append([]ast.Pattern{}, p.Alts[:last]...), b.Sub)
			return &BindingPattern{&AltPattern{c.patterns(alts)}, b.Name}
		}
		return &AltPattern{c.patterns(p.Alts)}
	case *ast.FindPattern:
		return &FindPattern{Const: c.expr(p.Const), PreName: p.PreName, Mid: c.patterns(p.Mid), PostName: p.PostName}
	}
	c.fail("unsupported pattern: %T", p)
	return nil
}

// assignment rewrites `target.name = value` and `target[key] = value` into a
// rebinding of the variable the chain starts from, with every link copied:
// `a.x.y = 3` becomes `a = With(a, :x, With(a.x, :y, 3))`.
func (c *converter) assignment(n *ast.Call) Node {
	var key, value Node
	if n.Name == "[]=" {
		if len(n.Args) != 2 {
			c.fail("index assignment takes one index and one value")
			return nil
		}
		key, value = c.expr(n.Args[0]), c.expr(n.Args[1])
	} else {
		if len(n.Args) != 1 {
			c.fail("%s takes one value", n.Name)
			return nil
		}
		key = &Sym{c.pos(), strings.TrimSuffix(n.Name, "=")}
		value = c.expr(n.Args[0])
	}
	if n.Recv == nil {
		c.fail("%s: cameo has no setters", n.Name)
		return nil
	}
	return c.update(n.Recv, key, n.Name != "[]=", value, n)
}

// update binds a copy of recv with key replaced by value to wherever recv came
// from, following attribute and index reads down to a local variable, or to
// self in initialize. attr tells that key names an attribute, `.key`, rather
// than an index, `[key]`.
func (c *converter) update(recv ast.Node, key Node, attr bool, value Node, origin *ast.Call) Node {
	with := func(target Node) Node { return &With{c.pos(), target, key, value} }
	switch r := recv.(type) {
	case *ast.VarRef:
		if slices.Contains(c.lent, r.Name) {
			c.fail("Hash.new's block cannot store into its hash %s: cameo values are immutable; return the default instead", r.Name)
			return nil
		}
		return &Assign{c.pos(), r.Name, with(&Var{c.pos(), r.Name})}
	case *ast.IvarRef:
		name := strings.TrimPrefix(r.Name, "@")
		c.fail("@%s: cameo has no instance variables; initialize sets a field with self.%s = …", name, name)
		return nil
	case *ast.SelfLit:
		// self.name = value, or the end of a chain such as self.a.b = value.
		if !attr {
			c.fail("self[…] = …: cameo has no setters; initialize sets a field by name, self.name = …")
			return nil
		}
		return c.fieldAssign(key.(*Sym).Name, value)
	case *ast.Call:
		if r.Recv != nil && r.Block == nil && !r.Safe {
			switch {
			case len(r.Args) == 0 && isPlainName(r.Name):
				return c.update(r.Recv, &Sym{c.pos(), r.Name}, true, with(c.expr(r)), origin)
			case r.Name == "[]" && len(r.Args) == 1:
				return c.update(r.Recv, c.expr(r.Args[0]), false, with(c.expr(r)), origin)
			}
		}
	}
	c.fail("%s: the left side of an assignment must start with a local variable, so the copy has somewhere to go", describeSetter(origin))
	return nil
}

func describeSetter(n *ast.Call) string {
	if n.Name == "[]=" {
		return "…[…] = …"
	}
	return "…." + strings.TrimSuffix(n.Name, "=") + " = …"
}

// isPlainName reports whether a method name is an identifier that can be a
// field reader: letters, digits and underscores, no operator or suffix.
func isPlainName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// async converts `async recv, :method, *args`, and Cameo.async when
// qualified, which needs no include Cameo.
func (c *converter) async(n *ast.Call, qualified bool) Node {
	if n.Block != nil {
		c.fail("async takes no block: a worker runs a method of a value")
		return nil
	}
	args, kwargs, blockArg := c.args(n.Args)
	if blockArg != nil {
		c.fail("async takes no &block: a worker runs a method of a value")
		return nil
	}
	if len(args) < 2 {
		c.fail("async takes a receiver, a method name and the arguments: async worker, :run, item")
		return nil
	}
	if _, isSplat := args[0].(*Splat); isSplat {
		c.fail("async: the receiver cannot be a splat")
		return nil
	}
	return &Async{Pos: c.pos(), Qualified: qualified, Recv: args[0], Method: args[1], Args: args[2:], KwArgs: kwargs}
}

func (c *converter) await(n *ast.Call, qualified bool) Node {
	if n.Block != nil {
		c.fail("await takes no block")
		return nil
	}
	args, kwargs, blockArg := c.args(n.Args)
	if blockArg != nil {
		c.fail("await takes no &block")
		return nil
	}
	switch {
	case len(args) == 0:
		c.fail("await takes a promise, a list of promises, :all, or a receiver with a method name and arguments")
		return nil
	case len(args) == 1:
		if s, ok := args[0].(*Sym); ok && s.Name == "all" {
			return &AwaitAll{c.pos(), qualified}
		}
		if len(kwargs) > 0 {
			c.fail("await promise takes no keyword arguments")
			return nil
		}
		return &Await{Pos: c.pos(), Qualified: qualified, Promise: args[0]}
	}
	if _, isSplat := args[0].(*Splat); isSplat {
		c.fail("await: the receiver cannot be a splat")
		return nil
	}
	return &Await{Pos: c.pos(), Qualified: qualified, Recv: args[0], Method: args[1], Args: args[2:], KwArgs: kwargs}
}

// maxDeclaredNumber is the highest number: a field declaration takes, as
// protogen.MaxDeclaredNumber, which syntax does not import.
const maxDeclaredNumber = 4096

func (c *converter) fieldDecl(n *ast.Call) Node {
	decl := &FieldDecl{Pos: c.pos(), Type: n.Name}
	args, kwargs, blockArg := c.args(n.Args)
	if blockArg != nil || n.Block != nil {
		c.fail("%s: a field declaration takes no block", n.Name)
		return nil
	}
	for _, a := range args {
		switch a := a.(type) {
		case *Sym:
			decl.Names = append(decl.Names, a.Name)
		case *Const, *ScopedConst:
			if decl.Type == "ref" && decl.Ref == nil {
				decl.Ref = a
			} else {
				c.fail("%s: a field is named by a symbol", n.Name)
				return nil
			}
		default:
			c.fail("%s: a field is named by a symbol", n.Name)
			return nil
		}
	}
	if len(decl.Names) == 0 {
		c.fail("%s: a field declaration names at least one field", n.Name)
		return nil
	}
	for _, kw := range kwargs {
		key, ok := kw.Key.(*Sym)
		if !ok {
			c.fail("%s: options are keywords", n.Name)
			return nil
		}
		switch key.Name {
		case "default":
			decl.Default = kw.Value
		case "number":
			num, ok := kw.Value.(*Int)
			if !ok || num.Value < 1 || num.Value > maxDeclaredNumber {
				c.fail("%s: number: is an integer from 1 to %d", n.Name, maxDeclaredNumber)
				return nil
			}
			decl.Number = num.Value
		case "of":
			if decl.Type != "list" && decl.Type != "set" {
				c.fail("%s: of: belongs to list and set fields", n.Name)
				return nil
			}
			decl.Of = c.elementType(n.Name, kw.Value)
		case "key":
			if decl.Type != "map" {
				c.fail("%s: key: belongs to map fields", n.Name)
				return nil
			}
			decl.Key = c.elementType(n.Name, kw.Value)
		case "value":
			if decl.Type != "map" {
				c.fail("%s: value: belongs to map fields", n.Name)
				return nil
			}
			decl.Value = c.elementType(n.Name, kw.Value)
		case "values":
			if decl.Type != "enum" {
				c.fail("%s: values: belongs to enum fields", n.Name)
				return nil
			}
			list, ok := kw.Value.(*List)
			if !ok || len(list.Elems) == 0 {
				c.fail("enum: values: lists the symbols the field accepts")
				return nil
			}
			for _, e := range list.Elems {
				s, ok := e.(*Sym)
				if !ok {
					c.fail("enum: values: lists symbols")
					return nil
				}
				decl.Values = append(decl.Values, s.Name)
			}
		default:
			c.fail("%s: unknown option %s:", n.Name, key.Name)
			return nil
		}
	}
	if decl.Number != 0 && len(decl.Names) > 1 {
		c.fail("%s: number: belongs to a declaration of one field", n.Name)
		return nil
	}
	switch decl.Type {
	case "list", "set":
		if decl.Of == "" {
			c.fail("%s :%s needs of: the element type", decl.Type, decl.Names[0])
			return nil
		}
	case "map":
		if decl.Key == "" || decl.Value == "" {
			c.fail("map :%s needs key: and value: types", decl.Names[0])
			return nil
		}
		if k := object.Scalar(decl.Key); k == nil || !k.MapKey() {
			c.fail("map :%s: a key is a string, a bool or an integer type of 32 or 64 bits", decl.Names[0])
			return nil
		}
	case "ref":
		if decl.Ref == nil {
			c.fail("ref :%s needs the class it refers to: ref :%s, Point", decl.Names[0], decl.Names[0])
			return nil
		}
	case "enum":
		if len(decl.Values) == 0 {
			c.fail("enum :%s needs values: the symbols it accepts", decl.Names[0])
			return nil
		}
	}
	return decl
}

// elementType reads a type option (`of: :string`, `key: :int`, `of: Point`)
// into a type name: a scalar type, or a class name.
func (c *converter) elementType(decl string, v Node) string {
	switch v := v.(type) {
	case *Sym:
		if object.Scalar(v.Name) != nil {
			return v.Name
		}
		c.fail("%s: %s is not an element type; use a scalar type (:%s) or a class", decl, v.Name, strings.Join(object.ScalarNames(), ", :"))
	case *Const:
		return v.Name
	default:
		c.fail("%s: an element type is a symbol or a class name", decl)
	}
	return ""
}

func (c *converter) reserved(n *ast.Call) Node {
	args, kwargs, _ := c.args(n.Args)
	if n.Block != nil || len(kwargs) > 0 || len(args) == 0 || len(args) > 2 {
		c.fail("reserved takes a field number and optionally a field name: reserved 2, :nickname")
		return nil
	}
	num, ok := args[0].(*Int)
	if !ok || num.Value < 1 {
		c.fail("reserved: the field number is a positive integer")
		return nil
	}
	out := &Reserved{Pos: c.pos(), Number: num.Value}
	if len(args) == 2 {
		name, ok := args[1].(*Sym)
		if !ok {
			c.fail("reserved: the field name is a symbol")
			return nil
		}
		out.Name = name.Name
	}
	return out
}

func (c *converter) include(n *ast.Call) Node {
	args, kwargs, _ := c.args(n.Args)
	if n.Block != nil || len(kwargs) > 0 || len(args) != 1 {
		c.fail("%s takes one module", n.Name)
		return nil
	}
	switch args[0].(type) {
	case *Const, *ScopedConst:
	default:
		c.fail("%s takes a module name", n.Name)
		return nil
	}
	return &Include{Pos: c.pos(), Module: args[0], Extend: n.Name == "extend"}
}

// scopeOf is the module `class A::B` defines B in: A, or nil for a
// leading `::`.
func (c *converter) scopeOf(path ast.Node) Node {
	sc, ok := path.(*ast.ScopedConst)
	if !ok || sc.Recv == nil {
		return nil
	}
	return c.expr(sc.Recv)
}

// receiverName is how the error for a bang method names its receiver: a
// variable or a constant by its name, anything else as x.
func receiverName(recv ast.Node) string {
	switch r := recv.(type) {
	case *ast.VarRef:
		return r.Name
	case *ast.ConstRef:
		return r.Name
	}
	return "x"
}

// regexp converts a regexp literal. The parser leaves `#{...}` in the
// source, so the converter cuts it out and parses each piece itself.
func (c *converter) regexp(n *ast.RegexpLit) Node {
	options := 0
	for _, f := range n.Flags {
		switch f {
		case 'i':
			options |= 1
		case 'x':
			options |= 2
		case 'm':
			options |= 4
		case 'n':
			options |= 32
		case 'u':
			options |= 16
		}
	}
	var parts []Node
	var lit strings.Builder
	src := n.Source
	for i := 0; i < len(src); i++ {
		switch {
		case src[i] == '\\' && i+1 < len(src):
			lit.WriteString(src[i : i+2])
			i++
		case src[i] == '#' && i+1 < len(src) && src[i+1] == '{':
			end, depth := -1, 0
			for j := i + 2; j < len(src); j++ {
				if src[j] == '{' {
					depth++
				} else if src[j] == '}' {
					if depth == 0 {
						end = j
						break
					}
					depth--
				}
			}
			if end < 0 {
				lit.WriteByte(src[i])
				continue
			}
			tree, err := parser.Parse(src[i+2 : end])
			if err != nil {
				c.fail("in a regexp's interpolation: %v", err)
				return nil
			}
			if lit.Len() > 0 {
				parts = append(parts, &Str{c.pos(), lit.String()})
				lit.Reset()
			}
			var body []Node
			c.interpolating++
			for _, stmt := range tree.Body {
				body = append(body, c.expr(stmt))
			}
			c.interpolating--
			parts = append(parts, &Seq{c.pos(), body})
			i = end
		default:
			lit.WriteByte(src[i])
		}
	}
	if lit.Len() > 0 || len(parts) == 0 {
		parts = append(parts, &Str{c.pos(), lit.String()})
	}
	// A literal without interpolation is checked now: one Go's regexp
	// cannot match is refused like any construct cameo leaves out.
	if s, ok := parts[0].(*Str); ok && len(parts) == 1 {
		var unsupported *rx.Unsupported
		if _, _, err := rx.Translate(s.Value, options); errors.As(err, &unsupported) {
			c.fail("%s", unsupported.Error())
			return nil
		}
	}
	return &Regexp{c.pos(), parts, options}
}
