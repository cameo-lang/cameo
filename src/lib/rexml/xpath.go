package rexml

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// An XPath 1.0 evaluator over the tree: location paths with every axis,
// the node tests, predicates, the operators, the core function library,
// variables and namespace prefixes. The results of a step are in the
// order the context nodes produced them, as REXML gives them, not sorted
// into document order.

type xkind int

const (
	xNodes xkind = iota
	xString
	xNumber
	xBool
)

// xval is an XPath value: a node-set, a string, a number or a boolean. A
// number remembers whether it is a count or a literal integer, which
// REXML returns as an Integer, where arithmetic gives a Float.
type xval struct {
	kind     xkind
	nodes    []*node
	s        string
	n        float64
	integral bool
	b        bool
}

func nodesVal(ns []*node) xval  { return xval{kind: xNodes, nodes: ns} }
func stringVal(s string) xval   { return xval{kind: xString, s: s} }
func numberVal(n float64) xval  { return xval{kind: xNumber, n: n} }
func integerVal(n float64) xval { return xval{kind: xNumber, n: n, integral: true} }
func boolVal(b bool) xval       { return xval{kind: xBool, b: b} }

func (v xval) toBool() bool {
	switch v.kind {
	case xNodes:
		return len(v.nodes) > 0
	case xString:
		return v.s != ""
	case xNumber:
		return v.n != 0 && !math.IsNaN(v.n)
	}
	return v.b
}

func (v xval) toString() string {
	switch v.kind {
	case xNodes:
		if len(v.nodes) == 0 {
			return ""
		}
		return v.nodes[0].stringValue()
	case xNumber:
		return numberString(v.n)
	case xBool:
		if v.b {
			return "true"
		}
		return "false"
	}
	return v.s
}

func (v xval) toNumber() float64 {
	switch v.kind {
	case xNodes, xString:
		return stringNumber(v.toString())
	case xBool:
		if v.b {
			return 1
		}
		return 0
	}
	return v.n
}

// numberString is XPath's string of a number: no fraction for an integer,
// NaN and Infinity by name.
func numberString(n float64) string {
	switch {
	case math.IsNaN(n):
		return "NaN"
	case math.IsInf(n, 1):
		return "Infinity"
	case math.IsInf(n, -1):
		return "-Infinity"
	case n == math.Trunc(n) && math.Abs(n) < 1e15:
		return strconv.FormatInt(int64(n), 10)
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}

// stringNumber is XPath's number of a string: NaN unless it is a number
// with optional surrounding whitespace.
func stringNumber(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return math.NaN()
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(isDigit(c) || c == '.' || c == '-' && i == 0) {
			return math.NaN()
		}
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return math.NaN()
	}
	return n
}

// Tokens.

type tokKind int

const (
	tEOF  tokKind = iota
	tName         // a name, or a keyword: and or div mod
	tNumber
	tLiteral
	tVariable
	tOp // / // [ ] ( ) . .. @ , :: | + - = != < <= > >= *
)

type token struct {
	kind tokKind
	text string
	num  float64
}

// xpathError is a path the parser cannot read.
type xpathError struct{ path, msg string }

func (e *xpathError) Error() string { return "Invalid XPath " + strconv.Quote(e.path) + ": " + e.msg }

func tokenize(path string) ([]token, error) {
	var toks []token
	// operand says the last token can end an operand, so a * or a name
	// after it is an operator.
	operand := false
	for i := 0; i < len(path); {
		c := path[i]
		switch {
		case isSpace(c):
			i++
			continue
		case c == '"' || c == '\'':
			end := strings.IndexByte(path[i+1:], c)
			if end < 0 {
				return nil, &xpathError{path, "unterminated literal"}
			}
			toks = append(toks, token{kind: tLiteral, text: path[i+1 : i+1+end]})
			i += end + 2
			operand = true
		case isDigit(c) || c == '.' && i+1 < len(path) && isDigit(path[i+1]):
			j := i
			for j < len(path) && (isDigit(path[j]) || path[j] == '.') {
				j++
			}
			n, err := strconv.ParseFloat(path[i:j], 64)
			if err != nil {
				return nil, &xpathError{path, "bad number " + path[i:j]}
			}
			toks = append(toks, token{kind: tNumber, text: path[i:j], num: n})
			i = j
			operand = true
		case c == '$':
			j := i + 1
			for j < len(path) && (isNameChar(path[j])) {
				j++
			}
			if j == i+1 {
				return nil, &xpathError{path, "missing variable name"}
			}
			toks = append(toks, token{kind: tVariable, text: path[i+1 : j]})
			i = j
			operand = true
		case strings.HasPrefix(path[i:], "//"), strings.HasPrefix(path[i:], ".."), strings.HasPrefix(path[i:], "::"),
			strings.HasPrefix(path[i:], "!="), strings.HasPrefix(path[i:], "<="), strings.HasPrefix(path[i:], ">="):
			toks = append(toks, token{kind: tOp, text: path[i : i+2]})
			operand = path[i] == '.'
			i += 2
		case strings.ContainsRune("/[](),@|+-=<>*.", rune(c)):
			text := string(c)
			if c == '*' && !operand {
				text = "*name" // a wildcard node test, not multiplication
			}
			toks = append(toks, token{kind: tOp, text: text})
			operand = c == ']' || c == ')' || c == '.' || text == "*name"
			i++
		case isNameStart(c):
			j := i
			// A name may hold one colon, as a prefix does; two are an axis.
			for j < len(path) && (isNameChar(path[j]) && path[j] != ':' || path[j] == ':' && j+1 < len(path) && path[j+1] != ':' && (j == 0 || path[j-1] != ':')) {
				j++
			}
			name := path[i:j]
			// prefix:* is a node test too.
			if j < len(path) && path[j] == '*' && strings.HasSuffix(name, ":") {
				name += "*"
				j++
			}
			if operand && (name == "and" || name == "or" || name == "div" || name == "mod") {
				toks = append(toks, token{kind: tOp, text: name})
				operand = false
			} else {
				toks = append(toks, token{kind: tName, text: name})
				operand = true
			}
			i = j
		default:
			return nil, &xpathError{path, "unexpected character " + strconv.Quote(string(c))}
		}
	}
	return append(toks, token{kind: tEOF}), nil
}

// The syntax tree.

type expr interface {
	eval(c *xctx) (xval, error)
}

// xctx is the evaluation context: the node, its position and the size of
// the current node list, the variables and the namespace prefixes.
type xctx struct {
	node      *node
	pos, size int
	vars      map[string]xval
	ns        map[string]string
	start     *node // the node the evaluation began at, for prefixes
}

func (c *xctx) with(n *node, pos, size int) *xctx {
	return &xctx{node: n, pos: pos, size: size, vars: c.vars, ns: c.ns, start: c.start}
}

type literal struct{ v xval }

func (e literal) eval(c *xctx) (xval, error) { return e.v, nil }

type variable struct{ name string }

func (e variable) eval(c *xctx) (xval, error) {
	v, ok := c.vars[e.name]
	if !ok {
		return xval{}, fmt.Errorf("undefined variable $%s", e.name)
	}
	return v, nil
}

type negate struct{ x expr }

func (e negate) eval(c *xctx) (xval, error) {
	v, err := e.x.eval(c)
	if err != nil {
		return xval{}, err
	}
	return numberVal(-v.toNumber()), nil
}

type binary struct {
	op   string
	l, r expr
}

func (e binary) eval(c *xctx) (xval, error) {
	l, err := e.l.eval(c)
	if err != nil {
		return xval{}, err
	}
	switch e.op {
	case "or":
		if l.toBool() {
			return boolVal(true), nil
		}
		r, err := e.r.eval(c)
		return boolVal(r.toBool()), err
	case "and":
		if !l.toBool() {
			return boolVal(false), nil
		}
		r, err := e.r.eval(c)
		return boolVal(r.toBool()), err
	}
	r, err := e.r.eval(c)
	if err != nil {
		return xval{}, err
	}
	switch e.op {
	case "=", "!=", "<", "<=", ">", ">=":
		return boolVal(compare(e.op, l, r)), nil
	case "|":
		out := append([]*node(nil), l.nodes...)
		seen := map[*node]bool{}
		for _, n := range out {
			seen[n] = true
		}
		for _, n := range r.nodes {
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
		return nodesVal(out), nil
	}
	a, b := l.toNumber(), r.toNumber()
	switch e.op {
	case "+":
		return numberVal(a + b), nil
	case "-":
		return numberVal(a - b), nil
	case "*":
		return numberVal(a * b), nil
	case "div":
		return numberVal(a / b), nil
	case "mod":
		return numberVal(math.Mod(a, b)), nil
	}
	return xval{}, fmt.Errorf("unknown operator %s", e.op)
}

// compare applies a comparison with XPath's rules for node-sets.
func compare(op string, l, r xval) bool {
	if l.kind == xNodes && r.kind == xNodes {
		for _, a := range l.nodes {
			for _, b := range r.nodes {
				if cmpStrings(op, a.stringValue(), b.stringValue()) {
					return true
				}
			}
		}
		return false
	}
	if l.kind == xNodes || r.kind == xNodes {
		set, other, flip := l, r, false
		if r.kind == xNodes {
			set, other, flip = r, l, true
		}
		for _, n := range set.nodes {
			nv := stringVal(n.stringValue())
			var ok bool
			if flip {
				ok = compare(op, other, nv)
			} else {
				ok = compare(op, nv, other)
			}
			if ok {
				return true
			}
		}
		return false
	}
	if op == "=" || op == "!=" {
		var eq bool
		switch {
		case l.kind == xBool || r.kind == xBool:
			eq = l.toBool() == r.toBool()
		case l.kind == xNumber || r.kind == xNumber:
			eq = l.toNumber() == r.toNumber()
		default:
			eq = l.toString() == r.toString()
		}
		return eq == (op == "=")
	}
	return cmpNumbers(op, l.toNumber(), r.toNumber())
}

func cmpStrings(op, a, b string) bool {
	switch op {
	case "=":
		return a == b
	case "!=":
		return a != b
	}
	return cmpNumbers(op, stringNumber(a), stringNumber(b))
}

func cmpNumbers(op string, a, b float64) bool {
	switch op {
	case "=":
		return a == b
	case "!=":
		return a != b
	case "<":
		return a < b
	case "<=":
		return a <= b
	case ">":
		return a > b
	case ">=":
		return a >= b
	}
	return false
}

// filter is a primary expression with predicates and an optional path
// after it: (//a)[1]/b, $set/c.
type filter struct {
	primary expr
	preds   []expr
	steps   []step
}

func (e filter) eval(c *xctx) (xval, error) {
	v, err := e.primary.eval(c)
	if err != nil {
		return xval{}, err
	}
	if len(e.preds) == 0 && len(e.steps) == 0 {
		return v, nil
	}
	if v.kind != xNodes {
		return xval{}, fmt.Errorf("a predicate or a path needs a node-set")
	}
	nodes := v.nodes
	for _, p := range e.preds {
		if nodes, err = applyPredicate(c, nodes, p, false); err != nil {
			return xval{}, err
		}
	}
	return walk(c, nodes, e.steps)
}

// path is a location path.
type path struct {
	absolute bool
	steps    []step
}

func (e path) eval(c *xctx) (xval, error) {
	start := c.node
	if e.absolute {
		start = c.node.top()
	}
	return walk(c, []*node{start}, e.steps)
}

func walk(c *xctx, nodes []*node, steps []step) (xval, error) {
	var err error
	for _, s := range steps {
		if nodes, err = s.apply(c, nodes); err != nil {
			return xval{}, err
		}
	}
	return nodesVal(nodes), nil
}

type step struct {
	axis  string
	test  nodeTest
	preds []expr
}

type nodeTest struct {
	kind string // name, node, text, comment, processing-instruction
	name string // for name: prefix:local, local, *, prefix:*; for processing-instruction: the target or ""
}

func reverseAxis(axis string) bool {
	return axis == "ancestor" || axis == "ancestor-or-self" || axis == "preceding" || axis == "preceding-sibling"
}

// apply runs a step from each node; the result holds each node once, in
// document order.
func (s step) apply(c *xctx, nodes []*node) ([]*node, error) {
	var out []*node
	seen := map[*node]bool{}
	for _, n := range nodes {
		cands := axisNodes(s.axis, n)
		var kept []*node
		for _, k := range cands {
			if s.test.matches(c, k, s.axis) {
				kept = append(kept, k)
			}
		}
		var err error
		for _, p := range s.preds {
			if kept, err = applyPredicate(c, kept, p, reverseAxis(s.axis)); err != nil {
				return nil, err
			}
		}
		for _, k := range kept {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	if len(nodes) > 1 && len(out) > 1 {
		sortDocumentOrder(out)
	}
	return out, nil
}

// sortDocumentOrder puts nodes in the order of the tree they are in; an
// attribute comes right after its element.
func sortDocumentOrder(nodes []*node) {
	order := map[*node]int{}
	var walk func(n *node)
	walk = func(n *node) {
		order[n] = len(order)
		for _, a := range n.attrs {
			order[a] = len(order)
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(nodes[0].top())
	sort.SliceStable(nodes, func(i, j int) bool { return order[nodes[i]] < order[nodes[j]] })
}

// applyPredicate keeps the nodes a predicate holds for; a number is a
// position, counted from the end on a reverse axis.
func applyPredicate(c *xctx, nodes []*node, p expr, reverse bool) ([]*node, error) {
	var out []*node
	size := len(nodes)
	for i, n := range nodes {
		pos := i + 1
		if reverse {
			pos = size - i
		}
		v, err := p.eval(c.with(n, pos, size))
		if err != nil {
			return nil, err
		}
		keep := v.toBool()
		if v.kind == xNumber {
			keep = v.n == float64(pos)
		}
		if keep {
			out = append(out, n)
		}
	}
	return out, nil
}

// axisNodes lists an axis from a node, in document order; a reverse axis
// is listed in document order too, its positions counted from the end.
func axisNodes(axis string, n *node) []*node {
	switch axis {
	case "child":
		return n.children
	case "attribute":
		if n.kind != kindElement {
			return nil
		}
		return n.attrs
	case "self":
		return []*node{n}
	case "parent":
		if n.parent == nil {
			return nil
		}
		return []*node{n.parent}
	case "ancestor":
		var out []*node
		for p := n.parent; p != nil; p = p.parent {
			out = append([]*node{p}, out...)
		}
		return out
	case "ancestor-or-self":
		return append(axisNodes("ancestor", n), n)
	case "descendant":
		var out []*node
		descend(n, &out)
		return out
	case "descendant-or-self":
		out := []*node{n}
		descend(n, &out)
		return out
	case "following-sibling":
		if n.parent == nil || n.kind == kindAttribute {
			return nil
		}
		i := n.index()
		return n.parent.children[i+1:]
	case "preceding-sibling":
		if n.parent == nil || n.kind == kindAttribute {
			return nil
		}
		i := n.index()
		return n.parent.children[:i]
	case "following":
		var out []*node
		for p := n; p != nil; p = p.parent {
			for _, s := range axisNodes("following-sibling", p) {
				out = append(out, s)
				descend(s, &out)
			}
		}
		return out
	case "preceding":
		var out []*node
		for p := n; p != nil; p = p.parent {
			var before []*node
			for _, s := range axisNodes("preceding-sibling", p) {
				before = append(before, s)
				descend(s, &before)
			}
			out = append(before, out...)
		}
		return out
	case "namespace":
		return nil
	}
	return nil
}

func descend(n *node, out *[]*node) {
	for _, c := range n.children {
		*out = append(*out, c)
		descend(c, out)
	}
}

// matches applies a node test. On the attribute axis a name names an
// attribute; elsewhere an element.
func (t nodeTest) matches(c *xctx, n *node, axis string) bool {
	switch t.kind {
	case "node":
		return true
	case "text":
		return n.isText()
	case "comment":
		return n.kind == kindComment
	case "processing-instruction":
		return n.kind == kindInstruction && (t.name == "" || n.target == t.name)
	}
	if axis == "attribute" {
		if n.kind != kindAttribute {
			return false
		}
		return t.name == "*" || n.name == t.name
	}
	if n.kind != kindElement {
		return false
	}
	if t.name == "*" {
		return true
	}
	prefix, local := "", t.name
	if i := strings.IndexByte(t.name, ':'); i >= 0 {
		prefix, local = t.name[:i], t.name[i+1:]
	}
	if local != "*" && n.localName() != local {
		return false
	}
	if prefix == "" {
		return n.prefix() == ""
	}
	uri, ok := c.ns[prefix]
	if !ok {
		uri, ok = c.start.namespace(prefix)
		if !ok {
			return n.prefix() == prefix
		}
	}
	return n.namespaceURI() == uri
}

// call is a function call.
type call struct {
	name string
	args []expr
}

func (e call) eval(c *xctx) (xval, error) {
	args := make([]xval, len(e.args))
	for i, a := range e.args {
		v, err := a.eval(c)
		if err != nil {
			return xval{}, err
		}
		args[i] = v
	}
	arg := func(i int) xval {
		if i < len(args) {
			return args[i]
		}
		return nodesVal([]*node{c.node})
	}
	str := func(i int) string { return arg(i).toString() }
	num := func(i int) float64 { return arg(i).toNumber() }
	nameOf := func(f func(*node) string) xval {
		v := arg(0)
		if v.kind != xNodes || len(v.nodes) == 0 {
			return stringVal("")
		}
		return stringVal(f(v.nodes[0]))
	}
	switch e.name {
	case "last":
		return integerVal(float64(c.size)), nil
	case "position":
		return integerVal(float64(c.pos)), nil
	case "count":
		if len(args) != 1 || args[0].kind != xNodes {
			return xval{}, fmt.Errorf("count() needs a node-set")
		}
		return integerVal(float64(len(args[0].nodes))), nil
	case "local-name":
		return nameOf(func(n *node) string {
			if n.kind == kindElement || n.kind == kindAttribute {
				return n.localName()
			}
			return ""
		}), nil
	case "namespace-uri":
		return nameOf(func(n *node) string { return n.namespaceURI() }), nil
	case "name":
		return nameOf(func(n *node) string {
			if n.kind == kindElement || n.kind == kindAttribute {
				return n.name
			}
			return ""
		}), nil
	case "string":
		return stringVal(str(0)), nil
	case "concat":
		var b strings.Builder
		for i := range args {
			b.WriteString(str(i))
		}
		return stringVal(b.String()), nil
	case "starts-with":
		return boolVal(strings.HasPrefix(str(0), str(1))), nil
	case "contains":
		return boolVal(strings.Contains(str(0), str(1))), nil
	case "substring-before":
		s, sep := str(0), str(1)
		i := strings.Index(s, sep)
		if i < 0 {
			return stringVal(""), nil
		}
		return stringVal(s[:i]), nil
	case "substring-after":
		s, sep := str(0), str(1)
		i := strings.Index(s, sep)
		if i < 0 {
			return stringVal(""), nil
		}
		return stringVal(s[i+len(sep):]), nil
	case "substring":
		return stringVal(substring(str(0), num(1), args, num)), nil
	case "string-length":
		return integerVal(float64(len([]rune(str(0))))), nil
	case "normalize-space":
		return stringVal(strings.Join(strings.Fields(str(0)), " ")), nil
	case "translate":
		return stringVal(translate(str(0), str(1), str(2))), nil
	case "boolean":
		return boolVal(arg(0).toBool()), nil
	case "not":
		return boolVal(!arg(0).toBool()), nil
	case "true":
		return boolVal(true), nil
	case "false":
		return boolVal(false), nil
	case "lang":
		for p := c.node; p != nil; p = p.parent {
			if a := p.attr("xml:lang"); a != nil {
				l := strings.ToLower(a.textValue())
				want := strings.ToLower(str(0))
				return boolVal(l == want || strings.HasPrefix(l, want+"-")), nil
			}
		}
		return boolVal(false), nil
	case "number":
		return numberVal(num(0)), nil
	case "sum":
		if len(args) != 1 || args[0].kind != xNodes {
			return xval{}, fmt.Errorf("sum() needs a node-set")
		}
		total := 0.0
		for _, n := range args[0].nodes {
			total += stringNumber(n.stringValue())
		}
		return numberVal(total), nil
	case "floor":
		return integerVal(math.Floor(num(0))), nil
	case "ceiling":
		return integerVal(math.Ceil(num(0))), nil
	case "round":
		return integerVal(math.Floor(num(0) + 0.5)), nil
	}
	return xval{}, fmt.Errorf("unknown function %s()", e.name)
}

// substring is XPath's: positions count from 1 and round.
func substring(s string, start float64, args []xval, num func(int) float64) string {
	runes := []rune(s)
	from := math.Floor(start + 0.5)
	to := math.Inf(1)
	if len(args) > 2 {
		to = from + math.Floor(num(2)+0.5)
	}
	var b strings.Builder
	for i, r := range runes {
		p := float64(i + 1)
		if p >= from && p < to {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func translate(s, from, to string) string {
	f, t := []rune(from), []rune(to)
	m := map[rune]int{}
	for i, r := range f {
		if _, seen := m[r]; !seen {
			m[r] = i
		}
	}
	var b strings.Builder
	for _, r := range s {
		i, ok := m[r]
		switch {
		case !ok:
			b.WriteRune(r)
		case i < len(t):
			b.WriteRune(t[i])
		}
	}
	return b.String()
}

// The parser.

type xparser struct {
	path string
	toks []token
	i    int
}

func (p *xparser) peek() token { return p.toks[p.i] }
func (p *xparser) next() token { t := p.toks[p.i]; p.i++; return t }
func (p *xparser) isOp(text string) bool {
	t := p.peek()
	return t.kind == tOp && t.text == text
}
func (p *xparser) fail(msg string) error { return &xpathError{p.path, msg} }

func (p *xparser) expect(text string) error {
	if !p.isOp(text) {
		return p.fail("expected " + text + " but got " + p.describe(p.peek()))
	}
	p.next()
	return nil
}

func (p *xparser) describe(t token) string {
	if t.kind == tEOF {
		return "the end"
	}
	return strconv.Quote(strings.TrimSuffix(t.text, "name"))
}

// compile parses a path.
func compile(path string) (expr, error) {
	toks, err := tokenize(path)
	if err != nil {
		return nil, err
	}
	p := &xparser{path: path, toks: toks}
	e, err := p.expr()
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tEOF {
		return nil, p.fail("unexpected " + p.describe(p.peek()))
	}
	return e, nil
}

func (p *xparser) expr() (expr, error) { return p.binaryLevel(0) }

var levels = [][]string{{"or"}, {"and"}, {"=", "!="}, {"<", "<=", ">", ">="}, {"+", "-"}, {"*", "div", "mod"}}

func (p *xparser) binaryLevel(level int) (expr, error) {
	if level == len(levels) {
		return p.unary()
	}
	l, err := p.binaryLevel(level + 1)
	if err != nil {
		return nil, err
	}
	for {
		var op string
		for _, o := range levels[level] {
			if p.isOp(o) {
				op = o
			}
		}
		if op == "" {
			return l, nil
		}
		p.next()
		r, err := p.binaryLevel(level + 1)
		if err != nil {
			return nil, err
		}
		l = binary{op: op, l: l, r: r}
	}
}

func (p *xparser) unary() (expr, error) {
	if p.isOp("-") {
		p.next()
		x, err := p.unary()
		if err != nil {
			return nil, err
		}
		return negate{x}, nil
	}
	return p.union()
}

func (p *xparser) union() (expr, error) {
	l, err := p.pathExpr()
	if err != nil {
		return nil, err
	}
	for p.isOp("|") {
		p.next()
		r, err := p.pathExpr()
		if err != nil {
			return nil, err
		}
		l = binary{op: "|", l: l, r: r}
	}
	return l, nil
}

// pathExpr is a location path, or a filter expression with a path after.
func (p *xparser) pathExpr() (expr, error) {
	t := p.peek()
	primary := t.kind == tLiteral || t.kind == tNumber || t.kind == tVariable || t.kind == tOp && t.text == "(" ||
		t.kind == tName && p.toks[p.i+1].kind == tOp && p.toks[p.i+1].text == "(" && !isNodeType(t.text)
	if !primary {
		return p.locationPath()
	}
	prim, err := p.primary()
	if err != nil {
		return nil, err
	}
	f := filter{primary: prim}
	for p.isOp("[") {
		pred, err := p.predicate()
		if err != nil {
			return nil, err
		}
		f.preds = append(f.preds, pred)
	}
	if p.isOp("/") || p.isOp("//") {
		if p.isOp("//") {
			f.steps = append(f.steps, step{axis: "descendant-or-self", test: nodeTest{kind: "node"}})
		}
		p.next()
		steps, err := p.relativePath()
		if err != nil {
			return nil, err
		}
		f.steps = append(f.steps, steps...)
	}
	return f, nil
}

func isNodeType(name string) bool {
	return name == "node" || name == "text" || name == "comment" || name == "processing-instruction"
}

func (p *xparser) primary() (expr, error) {
	t := p.next()
	switch t.kind {
	case tLiteral:
		return literal{stringVal(t.text)}, nil
	case tNumber:
		if strings.Contains(t.text, ".") {
			return literal{numberVal(t.num)}, nil
		}
		return literal{integerVal(t.num)}, nil
	case tVariable:
		return variable{t.text}, nil
	case tName:
		if err := p.expect("("); err != nil {
			return nil, err
		}
		c := call{name: t.text}
		for !p.isOp(")") {
			a, err := p.expr()
			if err != nil {
				return nil, err
			}
			c.args = append(c.args, a)
			if p.isOp(",") {
				p.next()
			} else if !p.isOp(")") {
				return nil, p.fail("expected , or ) in " + t.text + "()")
			}
		}
		p.next()
		return c, nil
	}
	// A parenthesised expression.
	e, err := p.expr()
	if err != nil {
		return nil, err
	}
	return e, p.expect(")")
}

func (p *xparser) predicate() (expr, error) {
	p.next() // [
	e, err := p.expr()
	if err != nil {
		return nil, err
	}
	return e, p.expect("]")
}

func (p *xparser) locationPath() (expr, error) {
	e := path{}
	switch {
	case p.isOp("/"):
		p.next()
		e.absolute = true
		t := p.peek()
		if t.kind == tEOF || t.kind == tOp && t.text != "." && t.text != ".." && t.text != "@" && t.text != "*name" {
			return e, nil
		}
	case p.isOp("//"):
		p.next()
		e.absolute = true
		e.steps = append(e.steps, step{axis: "descendant-or-self", test: nodeTest{kind: "node"}})
	}
	steps, err := p.relativePath()
	if err != nil {
		return nil, err
	}
	e.steps = append(e.steps, steps...)
	return e, nil
}

func (p *xparser) relativePath() ([]step, error) {
	var steps []step
	for {
		s, err := p.step()
		if err != nil {
			return nil, err
		}
		steps = append(steps, s)
		switch {
		case p.isOp("//"):
			p.next()
			steps = append(steps, step{axis: "descendant-or-self", test: nodeTest{kind: "node"}})
		case p.isOp("/"):
			p.next()
		default:
			return steps, nil
		}
	}
}

func (p *xparser) step() (step, error) {
	s := step{axis: "child"}
	t := p.peek()
	switch {
	case t.kind == tOp && t.text == ".":
		p.next()
		return step{axis: "self", test: nodeTest{kind: "node"}}, nil
	case t.kind == tOp && t.text == "..":
		p.next()
		return step{axis: "parent", test: nodeTest{kind: "node"}}, nil
	case t.kind == tOp && t.text == "@":
		p.next()
		s.axis = "attribute"
	case t.kind == tName && p.toks[p.i+1].kind == tOp && p.toks[p.i+1].text == "::":
		p.next()
		p.next()
		s.axis = t.text
		switch s.axis {
		case "child", "descendant", "descendant-or-self", "parent", "ancestor", "ancestor-or-self",
			"following-sibling", "preceding-sibling", "following", "preceding", "attribute", "self", "namespace":
		default:
			return s, p.fail("unknown axis " + s.axis)
		}
	}
	t = p.next()
	switch {
	case t.kind == tOp && t.text == "*name":
		s.test = nodeTest{kind: "name", name: "*"}
	case t.kind == tName && isNodeType(t.text) && p.isOp("("):
		p.next()
		s.test = nodeTest{kind: t.text}
		if t.text == "processing-instruction" && p.peek().kind == tLiteral {
			s.test.name = p.next().text
		}
		if err := p.expect(")"); err != nil {
			return s, err
		}
	case t.kind == tName:
		s.test = nodeTest{kind: "name", name: t.text}
	default:
		return s, p.fail("expected a node test but got " + p.describe(t))
	}
	for p.isOp("[") {
		pred, err := p.predicate()
		if err != nil {
			return s, err
		}
		s.preds = append(s.preds, pred)
	}
	return s, nil
}

// evaluate runs a path from a node with variables and namespaces.
func evaluate(pathText string, start *node, ns map[string]string, vars map[string]xval) (xval, error) {
	e, err := compile(pathText)
	if err != nil {
		return xval{}, err
	}
	c := &xctx{node: start, pos: 1, size: 1, vars: vars, ns: ns, start: start}
	return e.eval(c)
}
