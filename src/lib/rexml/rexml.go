// Package rexml is Ruby's rexml library, built into cameo: REXML::Document
// and the tree under it, XPath over it, and the formatters that write it.
//
// It follows rexml 3.4.0, the version Ruby 3.4 ships: Document.new on a
// String with the context options, Element with its attributes, children
// and text, Elements and Attributes, Text, CData, Comment, Instruction,
// XMLDecl and DocType, XPath.first, match and each with the whole of XPath
// 1.0, Element#to_s and Document#write with the Default and Pretty
// formatters, and ParseException with REXML's messages. The nodes are
// handles, as REXML's objects are: add_element, text= and the rest change
// the tree for every holder, and == is identity. A Text compares by its
// content, as in Ruby.
package rexml

import (
	"math"
	"strconv"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	module       = object.NewModule("REXML")
	nodeMod      = object.NewModule("Node")
	childCls     = object.NewClass("Child", object.ObjectClass)
	parentCls    = object.NewClass("Parent", childCls)
	elementCls   = object.NewClass("Element", parentCls)
	documentCls  = object.NewClass("Document", elementCls)
	textCls      = object.NewClass("Text", childCls)
	cdataCls     = object.NewClass("CData", textCls)
	commentCls   = object.NewClass("Comment", childCls)
	instrCls     = object.NewClass("Instruction", childCls)
	xmlDeclCls   = object.NewClass("XMLDecl", childCls)
	docTypeCls   = object.NewClass("DocType", parentCls)
	attributeCls = object.NewClass("Attribute", object.ObjectClass)
	elementsCls  = object.NewClass("Elements", object.ObjectClass)
	attrsCls     = object.NewClass("Attributes", object.ObjectClass)
	xpathCls     = object.NewClass("XPath", object.ObjectClass)
	sourceCls    = object.NewClass("Source", object.ObjectClass)
	formatters   = object.NewModule("Formatters")
	defaultCls   = object.NewClass("Default", object.ObjectClass)
	prettyCls    = object.NewClass("Pretty", defaultCls)
	parseErrCls  = object.NewClass("ParseException", object.RuntimeError)
)

func classFor(k kind) *object.Class {
	switch k {
	case kindDocument:
		return documentCls
	case kindElement:
		return elementCls
	case kindText:
		return textCls
	case kindCData:
		return cdataCls
	case kindComment:
		return commentCls
	case kindInstruction:
		return instrCls
	case kindXMLDecl:
		return xmlDeclCls
	case kindDocType:
		return docTypeCls
	}
	return attributeCls
}

// valueOf is the Value of a node: the same one each time.
func valueOf(n *node) object.Value {
	if n == nil {
		return object.Nil
	}
	if n.val.IsZero() {
		n.val = object.InstanceValue(&object.Instance{Cls: classFor(n.kind), Message: n})
	}
	return n.val
}

func nodeOf(v object.Value) *node { return v.Instance().Message.(*node) }

// nodeArg is the node a Value holds, if it is one.
func nodeArg(v object.Value) (*node, bool) {
	if v.Kind() != object.KindInstance {
		return nil, false
	}
	n, ok := v.Instance().Message.(*node)
	return n, ok
}

func values(ns []*node) object.Value {
	out := make([]object.Value, len(ns))
	for i, n := range ns {
		out[i] = valueOf(n)
	}
	return object.NewList(out...)
}

// elementsOf and attributesOf are the views of an element.
func elementsOf(n *node) object.Value {
	if n.elements.IsZero() {
		n.elements = object.InstanceValue(&object.Instance{Cls: elementsCls, Message: n})
	}
	return n.elements
}

func attributesOf(n *node) object.Value {
	if n.attributes.IsZero() {
		n.attributes = object.InstanceValue(&object.Instance{Cls: attrsCls, Message: n})
	}
	return n.attributes
}

// text reads a String or Symbol argument.
func text(ctx object.Context, v object.Value) (string, error) {
	if v.Kind() == object.KindSymbol {
		return v.Str(), nil
	}
	return stdlib.WantString(ctx, v)
}

func symbolOf(v object.Value) string {
	if v.Kind() == object.KindSymbol || v.IsString() {
		return v.Str()
	}
	return ""
}

func raise(ctx object.Context, cls *object.Class, msg string) error {
	return &object.Exception{Cls: cls, Message: msg}
}

func parseException(e *parseError) error {
	return &object.Exception{Cls: parseErrCls, Message: e.Error(), Attrs: map[string]object.Value{
		"line":     object.Int(int64(e.line)),
		"position": object.Int(int64(e.position)),
		"context":  object.NewList(object.Int(int64(e.position)), object.Int(int64(e.line)), object.Int(int64(e.line))),
	}}
}

// emit writes to an output: the standard output for nil, else through <<
// or write, so a StringIO or an IO takes it.
func emit(ctx object.Context, output object.Value, s string) error {
	if output.IsZero() || output.IsNil() {
		return ctx.Write(false, s)
	}
	if ctx.RespondsTo(output, "<<") {
		_, err := ctx.Call(output, "<<", object.String(s))
		return err
	}
	_, err := ctx.Call(output, "write", object.String(s))
	return err
}

// xpathFrom runs a path from a node, with the namespaces and variables a
// script passed.
func xpathFrom(ctx object.Context, n *node, pathText string, nsArg, varsArg object.Value) (xval, error) {
	ns := map[string]string{}
	if m, ok := nsArg.AsMap(); ok {
		for i, k := range m.Keys() {
			ns[symbolOf(k)] = m.Values()[i].Str()
		}
	}
	vars := map[string]xval{}
	if m, ok := varsArg.AsMap(); ok {
		for i, k := range m.Keys() {
			v := m.Values()[i]
			switch {
			case v.IsString() || v.Kind() == object.KindSymbol:
				vars[symbolOf(k)] = stringVal(v.Str())
			case v.Kind() == object.KindBool:
				vars[symbolOf(k)] = boolVal(v.Truthy())
			default:
				if f, ok := v.AsFloat(); ok {
					vars[symbolOf(k)] = numberVal(f)
				} else if i, ok := v.AsInt(); ok {
					vars[symbolOf(k)] = integerVal(float64(i))
				} else if nd, ok := nodeArg(v); ok {
					vars[symbolOf(k)] = nodesVal([]*node{nd})
				} else {
					s, err := ctx.ToS(v)
					if err != nil {
						return xval{}, err
					}
					vars[symbolOf(k)] = stringVal(s)
				}
			}
		}
	}
	v, err := evaluate(pathText, n, ns, vars)
	if err != nil {
		if _, isPath := err.(*xpathError); isPath {
			return xval{}, raise(ctx, parseErrCls, err.Error())
		}
		return xval{}, raise(ctx, parseErrCls, "Invalid XPath "+strconvQuote(pathText)+": "+err.Error())
	}
	return v, nil
}

func strconvQuote(s string) string { return "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\"" }

// xvalue converts an XPath value for a script.
func xvalue(v xval) object.Value {
	switch v.kind {
	case xNodes:
		return values(v.nodes)
	case xString:
		return object.String(v.s)
	case xBool:
		return object.Bool(v.b)
	}
	if v.integral && !math.IsNaN(v.n) && !math.IsInf(v.n, 0) {
		return object.Int(int64(v.n))
	}
	return object.Float(v.n)
}

// match is the node-set of a path from a node, in REXML's order.
func match(ctx object.Context, n *node, pathText string, nsArg, varsArg object.Value) ([]*node, error) {
	v, err := xpathFrom(ctx, n, pathText, nsArg, varsArg)
	if err != nil {
		return nil, err
	}
	if v.kind != xNodes {
		return nil, nil
	}
	return v.nodes, nil
}

// elementsMatching are the element children of n a path selects, or all
// of them for none.
func elementsMatching(ctx object.Context, n *node, pathArg object.Value) ([]*node, error) {
	if pathArg.IsZero() || pathArg.IsNil() {
		return n.elementChildren(), nil
	}
	p, err := text(ctx, pathArg)
	if err != nil {
		return nil, err
	}
	found, err := match(ctx, n, p, object.Nil, object.Nil)
	if err != nil {
		return nil, err
	}
	var out []*node
	for _, f := range found {
		if f.element() {
			out = append(out, f)
		}
	}
	return out, nil
}

// options reads Document.new's and Element.new's context Hash.
func contextOptions(ctx object.Context, v object.Value) (*options, error) {
	o := &options{}
	m, ok := v.AsMap()
	if !ok {
		return o, nil
	}
	for i, k := range m.Keys() {
		val := m.Values()[i]
		var names []string
		switch {
		case val.Kind() == object.KindSymbol && val.Str() == "all":
			names = []string{"*"}
		case val.IsNil() || val.Kind() == object.KindBool && !val.Truthy():
		default:
			list, err := stdlib.ToList(ctx, val)
			if err != nil {
				return nil, err
			}
			for _, e := range list {
				names = append(names, symbolOf(e))
			}
		}
		switch symbolOf(k) {
		case "raw":
			o.raw = names
		case "compress_whitespace":
			o.compress = names
		case "ignore_whitespace_nodes":
			o.ignoreWhitespace = names
		}
	}
	return o, nil
}

// newText makes a Text as Text.new does: normalized unless raw, when a
// bare & or < is refused.
func newText(ctx object.Context, s string, raw bool, doc *document) (*node, error) {
	if raw {
		for i := 0; i < len(s); i++ {
			if s[i] == '<' || s[i] == '&' && entityEnd(s, i) == 0 {
				return nil, raise(ctx, object.RuntimeError, "Illegal character "+strconvQuote(string(s[i]))+" in raw string "+strconvQuote(s))
			}
		}
		return &node{kind: kindText, value: s, raw: true, doc: doc}, nil
	}
	return &node{kind: kindText, value: normalize(s), doc: doc}, nil
}

// addAttributes sets attributes from a Hash.
func addAttributes(ctx object.Context, n *node, m *object.Map) error {
	for i, k := range m.Keys() {
		name, err := text(ctx, k)
		if err != nil {
			return err
		}
		v, err := ctx.ToS(m.Values()[i])
		if err != nil {
			return err
		}
		n.setAttr(name, normalize(v))
	}
	return nil
}

// Install defines REXML. lib.Install calls it once.
func Install() {
	m := module
	object.ObjectClass.Consts["REXML"] = object.ClassValue(m)
	m.Consts["VERSION"] = object.String("3.4.0")
	m.Consts["Formatters"] = object.ClassValue(formatters)
	formatters.Lexical = m
	for _, c := range []*object.Class{nodeMod, childCls, parentCls, elementCls, documentCls, textCls, cdataCls, commentCls,
		instrCls, xmlDeclCls, docTypeCls, attributeCls, elementsCls, attrsCls, xpathCls, sourceCls, parseErrCls} {
		c.Lexical = m
		m.Consts[c.Name] = object.ClassValue(c)
	}
	for _, c := range []*object.Class{defaultCls, prettyCls} {
		c.Lexical = formatters
		formatters.Consts[c.Name] = object.ClassValue(c)
	}
	for _, c := range []*object.Class{childCls, attributeCls} {
		c.Include(nodeMod)
	}
	elementsCls.Include(object.Enumerable)
	attrsCls.Include(object.Enumerable)
	parentCls.Include(object.Enumerable)
	installErrors()
	installDocument()
	installElement()
	installChildren()
	installCollections()
	installXPath()
	installFormatters()
}

func installErrors() {
	for _, name := range []string{"line", "position", "context"} {
		name := name
		parseErrCls.Define(name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if v, ok := self.Exception().Attr(name); ok {
				return v, nil
			}
			return object.Nil, nil
		})
	}
	sourceCls.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 {
			return object.Value{}, stdlib.ArgError(ctx, 0, "1..2")
		}
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.InstanceValue(&object.Instance{Cls: sourceCls, Message: s}), nil
	})
}

type nodeFn func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error)

func def(c *object.Class, name string, arity int, f nodeFn) {
	c.Define(name, arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return f(ctx, nodeOf(self), self, args, kw, block)
	})
}

func arg(args []object.Value, i int) object.Value {
	if i < len(args) {
		return args[i]
	}
	return object.Value{}
}

func given(v object.Value) bool { return !v.IsZero() && !v.IsNil() }

// writeArgs reads write's arguments: (output, indent, transitive, ie_hack)
// positionally or as keywords.
func writeArgs(ctx object.Context, args []object.Value, kw *object.Map) (output object.Value, indent int, ieHack bool, err error) {
	indent = -1
	output = arg(args, 0)
	if len(args) > 1 && given(args[1]) {
		n, err := stdlib.WantInt(ctx, args[1], "")
		if err != nil {
			return output, 0, false, err
		}
		indent = int(n)
	}
	if len(args) > 3 {
		ieHack = args[3].Truthy()
	}
	if kw != nil {
		if v, ok := kw.Get(object.Symbol("output")); ok {
			output = v
		}
		if v, ok := kw.Get(object.Symbol("indent")); ok && given(v) {
			n, err := stdlib.WantInt(ctx, v, "")
			if err != nil {
				return output, 0, false, err
			}
			indent = int(n)
		}
		if v, ok := kw.Get(object.Symbol("ie_hack")); ok {
			ieHack = v.Truthy()
		}
	}
	return output, indent, ieHack, nil
}

func render(n *node, indent int, ieHack bool) string {
	var f *formatter
	if indent >= 0 {
		f = newPretty(indent, ieHack)
	} else {
		f = newDefault(ieHack)
	}
	f.write(n)
	return f.b.String()
}

func installDocument() {
	c := documentCls
	c.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "0..2")
		}
		optsArg := arg(args, 1)
		if !given(optsArg) && kw != nil && kw.Len() > 0 {
			optsArg = object.MapValue(kw)
		}
		opts, err := contextOptions(ctx, optsArg)
		if err != nil {
			return object.Value{}, err
		}
		src := arg(args, 0)
		if !given(src) {
			return valueOf(&node{kind: kindDocument, doc: &document{}}), nil
		}
		var s string
		if src.Kind() == object.KindInstance && src.Class() == sourceCls {
			s = src.Instance().Message.(string)
		} else if nd, ok := nodeArg(src); ok && nd.kind == kindDocument {
			return valueOf(nd.deepCopy()), nil
		} else if s, err = stdlib.WantString(ctx, src); err != nil {
			return object.Value{}, err
		}
		d, perr := parse(s, opts)
		if perr != nil {
			return object.Value{}, parseException(perr)
		}
		return valueOf(d), nil
	})
	def(c, "root", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return valueOf(n.root()), nil
	})
	decl := func(n *node) *node {
		for _, ch := range n.children {
			if ch.kind == kindXMLDecl {
				return ch
			}
		}
		return &node{kind: kindXMLDecl, version: "1.0", encoding: "UTF-8"}
	}
	def(c, "xml_decl", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return valueOf(decl(n)), nil
	})
	def(c, "version", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(decl(n).version), nil
	})
	def(c, "encoding", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		d := decl(n)
		if d.encoding == "" {
			return object.String("UTF-8"), nil
		}
		return object.String(d.encoding), nil
	})
	def(c, "stand_alone?", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		d := decl(n)
		if d.standalone == "" {
			return object.Nil, nil
		}
		return object.String(d.standalone), nil
	})
	def(c, "doctype", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for _, ch := range n.children {
			if ch.kind == kindDocType {
				return valueOf(ch), nil
			}
		}
		return object.Nil, nil
	})
	def(c, "name", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(""), nil
	})
	def(c, "expanded_name", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(""), nil
	})
	def(c, "node_type", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Symbol("document"), nil
	})
	def(c, "document", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	// add puts a child in the document: a declaration first, one root
	// element only.
	add := func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		ch, ok := nodeArg(arg(args, 0))
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "a Document takes a node, not %s", stdlib.KindName(arg(args, 0)))
		}
		switch ch.kind {
		case kindXMLDecl:
			for i, old := range n.children {
				if old.kind == kindXMLDecl {
					n.children = append(n.children[:i], n.children[i+1:]...)
					break
				}
			}
			n.insert(0, ch)
		case kindDocType:
			at := 0
			if len(n.children) > 0 && n.children[0].kind == kindXMLDecl {
				at = 1
			}
			n.insert(at, ch)
		case kindElement:
			if n.root() != nil {
				return object.Value{}, raise(ctx, object.RuntimeError, "attempted adding second root element to document")
			}
			n.add(ch)
		case kindAttribute:
			return object.Value{}, ctx.Raise(object.TypeError, "a Document takes no attribute")
		default:
			n.add(ch)
		}
		return self, nil
	}
	def(c, "add", 1, add)
	def(c, "<<", 1, add)
	def(c, "to_s", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.String()), nil
	})
	def(c, "clone", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return valueOf(n.deepCopy()), nil
	})
}

// installElement defines Element, which Document inherits.
func installElement() {
	c := elementCls
	c.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 3 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..3")
		}
		var n *node
		if src, ok := nodeArg(args[0]); ok && src.element() {
			n = src.shallowCopy()
		} else {
			name, err := text(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			n = &node{kind: kindElement, name: name}
		}
		if p, ok := nodeArg(arg(args, 1)); ok {
			p.add(n)
			n.doc = p.doc
		}
		if n.doc == nil {
			n.doc = &document{}
		}
		if opts, err := contextOptions(ctx, arg(args, 2)); err != nil {
			return object.Value{}, err
		} else if opts.applies(opts.raw, "") {
			n.doc = &document{raw: true}
		}
		return valueOf(n), nil
	})
	def(c, "name", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.localName()), nil
	})
	def(c, "expanded_name", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.name), nil
	})
	def(c, "prefix", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.prefix()), nil
	})
	def(c, "namespace", -1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		prefix := n.prefix()
		if given(arg(args, 0)) {
			var err error
			if prefix, err = text(ctx, args[0]); err != nil {
				return object.Value{}, err
			}
		}
		uri, ok := n.namespace(prefix)
		if !ok {
			if prefix == "" {
				return object.String(""), nil
			}
			return object.Nil, nil
		}
		return object.String(uri), nil
	})
	def(c, "namespaces", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		names, uris := n.namespaces()
		m := object.NewMap()
		for _, name := range names {
			m = m.Put(object.String(name), object.String(uris[name]))
		}
		return object.MapValue(m), nil
	})
	def(c, "attributes", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return attributesOf(n), nil
	})
	def(c, "elements", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return elementsOf(n), nil
	})
	children := func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return values(n.children), nil
	}
	def(c, "children", 0, children)
	def(c, "to_a", 0, children)
	def(c, "each", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for _, ch := range append([]*node(nil), n.children...) {
			if _, err := ctx.Yield(block, valueOf(ch)); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	def(c, "size", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(int64(len(n.children))), nil
	})
	def(c, "length", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(int64(len(n.children))), nil
	})
	def(c, "index", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if ch, ok := nodeArg(args[0]); ok && ch.parent == n {
			return object.Int(int64(ch.index())), nil
		}
		return object.Nil, nil
	})
	def(c, "parent", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return valueOf(n.parent), nil
	})
	def(c, "document", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return valueOf(n.document()), nil
	})
	def(c, "root", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if r := n.root(); r != nil {
			return valueOf(r), nil
		}
		return valueOf(n.top()), nil
	})
	def(c, "root_node", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return valueOf(n.top()), nil
	})
	def(c, "node_type", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Symbol("element"), nil
	})
	def(c, "xpath", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.xpath()), nil
	})
	def(c, "whitespace", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.True, nil
	})
	def(c, "raw", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(n.doc != nil && n.doc.raw), nil
	})
	// Text.
	textOf := func(ctx object.Context, n *node, pathArg object.Value) (*node, error) {
		if given(pathArg) {
			p, err := text(ctx, pathArg)
			if err != nil {
				return nil, err
			}
			found, err := match(ctx, n, p, object.Nil, object.Nil)
			if err != nil || len(found) == 0 {
				return nil, err
			}
			if found[0].isText() {
				return found[0], nil
			}
			return found[0].firstText(), nil
		}
		return n.firstText(), nil
	}
	def(c, "text", -1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		t, err := textOf(ctx, n, arg(args, 0))
		if err != nil || t == nil {
			return object.Nil, err
		}
		return object.String(t.textValue()), nil
	})
	def(c, "get_text", -1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		t, err := textOf(ctx, n, arg(args, 0))
		if err != nil {
			return object.Value{}, err
		}
		return valueOf(t), nil
	})
	ofKind := func(keep func(*node) bool) nodeFn {
		return func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			var out []*node
			for _, ch := range n.children {
				if keep(ch) {
					out = append(out, ch)
				}
			}
			return values(out), nil
		}
	}
	def(c, "texts", 0, ofKind(func(n *node) bool { return n.isText() }))
	def(c, "cdatas", 0, ofKind(func(n *node) bool { return n.kind == kindCData }))
	def(c, "comments", 0, ofKind(func(n *node) bool { return n.kind == kindComment }))
	def(c, "instructions", 0, ofKind(func(n *node) bool { return n.kind == kindInstruction }))
	def(c, "has_elements?", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(len(n.elementChildren()) > 0), nil
	})
	def(c, "has_text?", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(n.firstText() != nil), nil
	})
	def(c, "has_attributes?", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(len(n.attrs) > 0), nil
	})
	// text= replaces the first Text child, or adds one; nil removes it.
	def(c, "text=", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		old := n.firstText()
		if args[0].IsNil() {
			if old != nil {
				old.detach()
			}
			return self, nil
		}
		t, err := textArg(ctx, n, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if old != nil {
			n.insert(old.index(), t)
			old.detach()
		} else {
			n.add(t)
		}
		return self, nil
	})
	def(c, "add_text", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		// A String joins a Text that ends the children, as in REXML.
		if args[0].IsString() {
			if last := lastChild(n); last != nil && last.kind == kindText {
				last.value += normalize(args[0].Str())
				return self, nil
			}
		}
		t, err := textArg(ctx, n, args[0])
		if err != nil {
			return object.Value{}, err
		}
		n.add(t)
		return self, nil
	})
	def(c, "add_element", -1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		var el *node
		if e, ok := nodeArg(args[0]); ok && e.element() {
			el = e
		} else {
			name, err := text(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			el = &node{kind: kindElement, name: name, doc: n.doc}
		}
		if n.kind == kindDocument && n.root() != nil {
			return object.Value{}, raise(ctx, object.RuntimeError, "attempted adding second root element to document")
		}
		attrs := arg(args, 1)
		if !given(attrs) && kw != nil && kw.Len() > 0 {
			attrs = object.MapValue(kw)
		}
		if m, ok := attrs.AsMap(); ok {
			if err := addAttributes(ctx, el, m); err != nil {
				return object.Value{}, err
			}
		}
		n.add(el)
		return valueOf(el), nil
	})
	add := func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		ch, ok := nodeArg(args[0])
		if !ok {
			t, err := textArg(ctx, n, args[0])
			if err != nil {
				return object.Value{}, err
			}
			ch = t
		}
		if ch.kind == kindAttribute {
			n.setAttr(ch.name, ch.value)
			return self, nil
		}
		n.add(ch)
		return self, nil
	}
	def(c, "add", 1, add)
	def(c, "<<", 1, add)
	def(c, "push", 1, add)
	def(c, "unshift", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		ch, ok := nodeArg(args[0])
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "unshift takes a node, not %s", stdlib.KindName(args[0]))
		}
		n.insert(0, ch)
		return self, nil
	})
	def(c, "add_attribute", -1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		if a, ok := nodeArg(args[0]); ok && a.kind == kindAttribute {
			n.setAttr(a.name, a.value)
			return valueOf(n.attr(a.name)), nil
		}
		name, err := text(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		v, err := ctx.ToS(arg(args, 1))
		if err != nil {
			return object.Value{}, err
		}
		return valueOf(n.setAttr(name, normalize(v))), nil
	})
	def(c, "add_attributes", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if m, ok := args[0].AsMap(); ok {
			return self, addAttributes(ctx, n, m)
		}
		list, err := stdlib.ToList(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		for _, pair := range list {
			kv, err := stdlib.ToList(ctx, pair)
			if err != nil {
				return object.Value{}, err
			}
			if len(kv) != 2 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "add_attributes takes pairs of name and value")
			}
			name, err := text(ctx, kv[0])
			if err != nil {
				return object.Value{}, err
			}
			v, err := ctx.ToS(kv[1])
			if err != nil {
				return object.Value{}, err
			}
			n.setAttr(name, normalize(v))
		}
		return self, nil
	})
	def(c, "delete_attribute", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name := ""
		if a, ok := nodeArg(args[0]); ok {
			name = a.name
		} else {
			var err error
			if name, err = text(ctx, args[0]); err != nil {
				return object.Value{}, err
			}
		}
		return valueOf(n.deleteAttr(name)), nil
	})
	def(c, "attribute", -1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 {
			return object.Value{}, stdlib.ArgError(ctx, 0, "1..2")
		}
		name, err := text(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if given(arg(args, 1)) {
			// With a namespace, the attribute whose prefix names it.
			uri, err := text(ctx, args[1])
			if err != nil {
				return object.Value{}, err
			}
			for _, a := range n.attrs {
				if a.localName() == name && a.namespaceURI() == uri {
					return valueOf(a), nil
				}
			}
			return object.Nil, nil
		}
		return valueOf(n.attr(name)), nil
	})
	def(c, "[]", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if i, ok := args[0].AsInt(); ok {
			if i < 0 {
				i += int64(len(n.children))
			}
			if i < 0 || i >= int64(len(n.children)) {
				return object.Nil, nil
			}
			return valueOf(n.children[i]), nil
		}
		name, err := text(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if a := n.attr(name); a != nil {
			return object.String(a.textValue()), nil
		}
		return object.Nil, nil
	})
	// Elements by path.
	def(c, "each_element", -1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		els, err := elementsMatching(ctx, n, arg(args, 0))
		if err != nil {
			return object.Value{}, err
		}
		for _, e := range els {
			if _, err := ctx.Yield(block, valueOf(e)); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	def(c, "get_elements", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		els, err := elementsMatching(ctx, n, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return values(els), nil
	})
	def(c, "each_element_with_attribute", -1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 {
			return object.Value{}, stdlib.ArgError(ctx, 0, "1..4")
		}
		key, err := text(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return eachWith(ctx, n, self, args[1:], block, func(e *node, value string, hasValue bool) bool {
			a := e.attr(key)
			return a != nil && (!hasValue || a.textValue() == value)
		})
	})
	def(c, "each_element_with_text", -1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return eachWith(ctx, n, self, args, block, func(e *node, value string, hasValue bool) bool {
			t := e.firstText()
			return t != nil && (!hasValue || t.textValue() == value)
		})
	})
	def(c, "delete_element", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		el, err := findElement(ctx, n, args[0])
		if err != nil || el == nil {
			return object.Nil, err
		}
		el.detach()
		return valueOf(el), nil
	})
	def(c, "delete", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		ch, ok := nodeArg(args[0])
		if !ok || ch.parent != n {
			return object.Nil, nil
		}
		ch.detach()
		return valueOf(ch), nil
	})
	def(c, "delete_at", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		i, err := stdlib.WantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if i < 0 || i >= int64(len(n.children)) {
			return object.Nil, nil
		}
		ch := n.children[i]
		ch.detach()
		return valueOf(ch), nil
	})
	def(c, "replace_child", 2, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		old, ok := nodeArg(args[0])
		repl, ok2 := nodeArg(args[1])
		if !ok || !ok2 || old.parent != n {
			return object.Value{}, ctx.Raise(object.ArgumentError, "replace_child takes a child of this element and a node")
		}
		n.insert(old.index(), repl)
		old.detach()
		return self, nil
	})
	insert := func(after bool) nodeFn {
		return func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			at, err := findElement(ctx, n, args[0])
			if err != nil {
				return object.Value{}, err
			}
			if at == nil {
				return object.Value{}, ctx.Raise(object.ArgumentError, "no such child")
			}
			ch, ok := nodeArg(args[1])
			if !ok {
				return object.Value{}, ctx.Raise(object.TypeError, "insert takes a node, not %s", stdlib.KindName(args[1]))
			}
			i := at.index()
			if after {
				i++
			}
			n.insert(i, ch)
			return self, nil
		}
	}
	def(c, "insert_before", 2, insert(false))
	def(c, "insert_after", 2, insert(true))
	def(c, "remove", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n.detach()
		return self, nil
	})
	def(c, "each_recursive", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		var walk func(*node) error
		walk = func(p *node) error {
			for _, ch := range append([]*node(nil), p.children...) {
				if !ch.element() {
					continue
				}
				if _, err := ctx.Yield(block, valueOf(ch)); err != nil {
					return err
				}
				if err := walk(ch); err != nil {
					return err
				}
			}
			return nil
		}
		return self, walk(n)
	})
	def(c, "find_first_recursive", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		var found *node
		var walk func(*node) error
		walk = func(p *node) error {
			for _, ch := range p.children {
				if !ch.element() || found != nil {
					continue
				}
				r, err := ctx.Yield(block, valueOf(ch))
				if err != nil {
					return err
				}
				if r.Truthy() {
					found = ch
					return nil
				}
				if err := walk(ch); err != nil {
					return err
				}
			}
			return nil
		}
		if err := walk(n); err != nil {
			return object.Value{}, err
		}
		return valueOf(found), nil
	})
	// Siblings.
	sibling := func(step int, elementsOnly bool) nodeFn {
		return func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if n.parent == nil {
				return object.Nil, nil
			}
			sibs := n.parent.children
			for i := n.index() + step; i >= 0 && i < len(sibs); i += step {
				if !elementsOnly || sibs[i].element() {
					return valueOf(sibs[i]), nil
				}
			}
			return object.Nil, nil
		}
	}
	def(c, "next_element", 0, sibling(1, true))
	def(c, "previous_element", 0, sibling(-1, true))
	def(c, "index_in_parent", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(int64(n.index() + 1)), nil
	})
	// Rendering.
	def(c, "to_s", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.String()), nil
	})
	def(c, "inspect", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.inspect()), nil
	})
	def(c, "write", -1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		output, indent, ieHack, err := writeArgs(ctx, args, kw)
		if err != nil {
			return object.Value{}, err
		}
		return self, emit(ctx, output, render(n, indent, ieHack))
	})
	def(c, "deep_clone", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return valueOf(n.deepCopy()), nil
	})
	def(c, "clone", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return valueOf(n.shallowCopy()), nil
	})
	installIdentity(childCls)
	installIdentity(attributeCls)
	installSiblings(childCls)
	installSiblings(attributeCls)
}

// installIdentity makes == identity for the nodes of a class; Text
// compares by content, as in REXML.
func installIdentity(c *object.Class) {
	for _, name := range []string{"==", "equal?", "eql?"} {
		name := name
		c.Define(name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			a := nodeOf(self)
			b, ok := nodeArg(args[0])
			if !ok {
				return object.False, nil
			}
			if a.isText() && b.isText() && name == "==" {
				return object.Bool(textString(a) == textString(b)), nil
			}
			return object.Bool(a == b), nil
		})
	}
	c.Define("hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(int64(len(nodeOf(self).String()))), nil
	})
}

// installSiblings gives every node its parent, siblings and removal.
func installSiblings(c *object.Class) {
	if c != attributeCls {
		def(c, "parent", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return valueOf(n.parent), nil
		})
	}
	def(c, "document", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return valueOf(n.document()), nil
	})
	def(c, "root_node", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return valueOf(n.top()), nil
	})
	def(c, "xpath", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.xpath()), nil
	})
	sib := func(step int) nodeFn {
		return func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if n.parent == nil || n.kind == kindAttribute {
				return object.Nil, nil
			}
			i := n.index() + step
			if i < 0 || i >= len(n.parent.children) {
				return object.Nil, nil
			}
			return valueOf(n.parent.children[i]), nil
		}
	}
	for _, name := range []string{"next_sibling", "next_sibling_node"} {
		def(c, name, 0, sib(1))
	}
	for _, name := range []string{"previous_sibling", "previous_sibling_node"} {
		def(c, name, 0, sib(-1))
	}
	def(c, "remove", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if n.kind == kindAttribute {
			if n.parent != nil {
				n.parent.deleteAttr(n.name)
			}
			return self, nil
		}
		n.detach()
		return self, nil
	})
	def(c, "to_s", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.String()), nil
	})
	def(c, "inspect", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.inspect()), nil
	})
	def(c, "write", -1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		output, indent, ieHack, err := writeArgs(ctx, args, kw)
		if err != nil {
			return object.Value{}, err
		}
		return self, emit(ctx, output, render(n, indent, ieHack))
	})
}

func lastChild(n *node) *node {
	if len(n.children) == 0 {
		return nil
	}
	return n.children[len(n.children)-1]
}

// textArg makes the Text a String or Text argument stands for.
func textArg(ctx object.Context, n *node, v object.Value) (*node, error) {
	if t, ok := nodeArg(v); ok {
		return t, nil
	}
	s, err := stdlib.WantString(ctx, v)
	if err != nil {
		return nil, err
	}
	return newText(ctx, s, n.doc != nil && n.doc.raw, n.doc)
}

// findElement is the element an argument names: the element itself, a
// path to it, or its 1-based index among the element children.
func findElement(ctx object.Context, n *node, v object.Value) (*node, error) {
	if e, ok := nodeArg(v); ok {
		return e, nil
	}
	if i, ok := v.AsInt(); ok {
		els := n.elementChildren()
		if i < 1 || i > int64(len(els)) {
			return nil, nil
		}
		return els[i-1], nil
	}
	els, err := elementsMatching(ctx, n, v)
	if err != nil || len(els) == 0 {
		return nil, err
	}
	return els[0], nil
}

// eachWith yields the element children a test holds for, with REXML's
// (value, max, name) arguments after the key.
func eachWith(ctx object.Context, n *node, self object.Value, args []object.Value, block object.Value, test func(e *node, value string, hasValue bool) bool) (object.Value, error) {
	value, hasValue := "", false
	if given(arg(args, 0)) {
		v, err := ctx.ToS(args[0])
		if err != nil {
			return object.Value{}, err
		}
		value, hasValue = v, true
	}
	limit := int64(0)
	if given(arg(args, 1)) {
		var err error
		if limit, err = stdlib.WantInt(ctx, args[1], ""); err != nil {
			return object.Value{}, err
		}
	}
	els, err := elementsMatching(ctx, n, arg(args, 2))
	if err != nil {
		return object.Value{}, err
	}
	count := int64(0)
	for _, e := range els {
		if !test(e, value, hasValue) {
			continue
		}
		if _, err := ctx.Yield(block, valueOf(e)); err != nil {
			return object.Value{}, err
		}
		count++
		if limit > 0 && count >= limit {
			break
		}
	}
	return self, nil
}

// installChildren defines Text, CData, Comment, Instruction, XMLDecl,
// DocType and Attribute.
func installChildren() {
	textCls.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 6 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..6")
		}
		if t, ok := nodeArg(args[0]); ok && t.isText() {
			return valueOf(&node{kind: kindText, value: t.value, raw: t.raw, doc: t.doc}), nil
		}
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		parent, _ := nodeArg(arg(args, 2))
		raw := arg(args, 3).Truthy()
		var doc *document
		if parent != nil {
			doc = parent.doc
			if doc != nil && doc.raw && arg(args, 3).IsZero() {
				raw = true
			}
		}
		t, err := newText(ctx, s, raw, doc)
		if err != nil {
			return object.Value{}, err
		}
		if parent != nil {
			parent.add(t)
		}
		return valueOf(t), nil
	})
	textCls.DefineSingleton("normalize", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 {
			return object.Value{}, stdlib.ArgError(ctx, 0, "1..3")
		}
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.String(normalize(s)), nil
	})
	textCls.DefineSingleton("unnormalize", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 {
			return object.Value{}, stdlib.ArgError(ctx, 0, "1..4")
		}
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.String(unnormalize(s, nil)), nil
	})
	def(textCls, "value", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.textValue()), nil
	})
	def(textCls, "value=", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if n.kind == kindCData {
			n.value = s
		} else {
			n.value = normalize(s)
		}
		return self, nil
	})
	def(textCls, "<<", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if n.kind == kindCData {
			n.value += s
		} else {
			n.value += normalize(s)
		}
		return self, nil
	})
	def(textCls, "empty?", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(n.value == ""), nil
	})
	def(textCls, "raw", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(n.raw), nil
	})
	def(textCls, "node_type", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Symbol("text"), nil
	})
	def(textCls, "clone", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return valueOf(n.deepCopy()), nil
	})
	cdataCls.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 3 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..3")
		}
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		n := &node{kind: kindCData, value: s}
		if parent, ok := nodeArg(arg(args, 2)); ok {
			parent.add(n)
		}
		return valueOf(n), nil
	})
	commentCls.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		n := &node{kind: kindComment, value: s}
		if parent, ok := nodeArg(arg(args, 1)); ok {
			parent.add(n)
		}
		return valueOf(n), nil
	})
	def(commentCls, "string", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.value), nil
	})
	def(commentCls, "string=", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := stdlib.WantString(ctx, args[0])
		n.value = s
		return self, err
	})
	def(commentCls, "node_type", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Symbol("comment"), nil
	})
	instrCls.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		target, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		n := &node{kind: kindInstruction, target: target}
		if given(arg(args, 1)) {
			if n.content, err = stdlib.WantString(ctx, args[1]); err != nil {
				return object.Value{}, err
			}
		}
		return valueOf(n), nil
	})
	def(instrCls, "target", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.target), nil
	})
	def(instrCls, "content", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if n.content == "" {
			return object.Nil, nil
		}
		return object.String(n.content), nil
	})
	def(instrCls, "node_type", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Symbol("processing_instruction"), nil
	})
	xmlDeclCls.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 3 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "0..3")
		}
		n := &node{kind: kindXMLDecl, version: "1.0", writeDecl: true}
		for i, field := range []*string{&n.version, &n.encoding, &n.standalone} {
			if given(arg(args, i)) {
				s, err := ctx.ToS(args[i])
				if err != nil {
					return object.Value{}, err
				}
				*field = s
			}
		}
		n.encoding = strings.ToUpper(n.encoding)
		return valueOf(n), nil
	})
	xmlDeclCls.DefineSingleton("default", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return valueOf(&node{kind: kindXMLDecl, version: "1.0"}), nil
	})
	def(xmlDeclCls, "version", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.version), nil
	})
	def(xmlDeclCls, "encoding", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if n.encoding == "" {
			return object.String("UTF-8"), nil
		}
		return object.String(n.encoding), nil
	})
	def(xmlDeclCls, "standalone", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if n.standalone == "" {
			return object.Nil, nil
		}
		return object.String(n.standalone), nil
	})
	def(xmlDeclCls, "writethis", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(n.writeDecl), nil
	})
	def(xmlDeclCls, "nowrite", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n.writeDecl = false
		return self, nil
	})
	def(xmlDeclCls, "dowrite", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n.writeDecl = true
		return self, nil
	})
	def(xmlDeclCls, "node_type", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Symbol("xmldecl"), nil
	})
	def(docTypeCls, "name", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.name), nil
	})
	strOrNil := func(get func(*node) string) nodeFn {
		return func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if s := get(n); s != "" {
				return object.String(s), nil
			}
			return object.Nil, nil
		}
	}
	def(docTypeCls, "external_id", 0, strOrNil(func(n *node) string { return n.externalID }))
	def(docTypeCls, "public", 0, strOrNil(func(n *node) string { return n.pub }))
	def(docTypeCls, "system", 0, strOrNil(func(n *node) string { return n.sys }))
	def(docTypeCls, "entities", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		m := object.NewMap()
		for _, name := range []string{"gt", "lt", "quot", "apos"} {
			m = m.Put(object.String(name), object.String(predefined[name]))
		}
		for _, e := range n.entities {
			m = m.Put(object.String(e.name), object.String(e.value))
		}
		return object.MapValue(m), nil
	})
	def(docTypeCls, "entity", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := text(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		for _, e := range n.entities {
			if e.name == name {
				return object.String(e.value), nil
			}
		}
		if v, ok := predefined[name]; ok && name != "amp" {
			return object.String(v), nil
		}
		return object.Nil, nil
	})
	def(docTypeCls, "node_type", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Symbol("doctype"), nil
	})
	// Attribute.
	attributeCls.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 3 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..3")
		}
		var n *node
		if a, ok := nodeArg(args[0]); ok && a.kind == kindAttribute {
			n = &node{kind: kindAttribute, name: a.name, value: a.value}
			if given(arg(args, 1)) {
				v, err := ctx.ToS(args[1])
				if err != nil {
					return object.Value{}, err
				}
				n.value = normalize(v)
			}
		} else {
			name, err := text(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			v, err := ctx.ToS(arg(args, 1))
			if err != nil {
				return object.Value{}, err
			}
			n = &node{kind: kindAttribute, name: name, value: normalize(v)}
		}
		if parent, ok := nodeArg(arg(args, 2)); ok && parent.element() {
			n.parent = parent
			parent.attrs = append(parent.attrs, n)
		}
		return valueOf(n), nil
	})
	def(attributeCls, "name", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.localName()), nil
	})
	def(attributeCls, "expanded_name", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.name), nil
	})
	def(attributeCls, "prefix", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.prefix()), nil
	})
	def(attributeCls, "namespace", -1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		prefix := n.prefix()
		if given(arg(args, 0)) {
			var err error
			if prefix, err = text(ctx, args[0]); err != nil {
				return object.Value{}, err
			}
		}
		if prefix == "" || n.parent == nil {
			return object.String(""), nil
		}
		uri, ok := n.parent.namespace(prefix)
		if !ok {
			return object.Nil, nil
		}
		return object.String(uri), nil
	})
	def(attributeCls, "value", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.textValue()), nil
	})
	def(attributeCls, "normalized", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(n.value), nil
	})
	def(attributeCls, "to_string", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(attributeString(n)), nil
	})
	def(attributeCls, "element", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return valueOf(n.parent), nil
	})
	def(attributeCls, "node_type", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Symbol("attribute"), nil
	})
	def(attributeCls, "clone", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return valueOf(&node{kind: kindAttribute, name: n.name, value: n.value, doc: n.doc}), nil
	})
}

// installCollections defines Elements and Attributes, the views of an
// element's children and attributes.
func installCollections() {
	e := elementsCls
	def(e, "parent", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return valueOf(n), nil
	})
	def(e, "[]", -1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		if i, ok := args[0].AsInt(); ok {
			if i < 1 {
				return object.Value{}, raise(ctx, object.RuntimeError, "index ("+strconv.FormatInt(i, 10)+") must be >= 1")
			}
			els := n.elementChildren()
			if given(arg(args, 1)) {
				name, err := text(ctx, args[1])
				if err != nil {
					return object.Value{}, err
				}
				var named []*node
				for _, el := range els {
					if el.name == name {
						named = append(named, el)
					}
				}
				els = named
			}
			if i > int64(len(els)) {
				return object.Nil, nil
			}
			return valueOf(els[i-1]), nil
		}
		els, err := elementsMatching(ctx, n, args[0])
		if err != nil || len(els) == 0 {
			return object.Nil, err
		}
		return valueOf(els[0]), nil
	})
	def(e, "[]=", 2, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		old, err := findElement(ctx, n, args[0])
		if err != nil {
			return object.Value{}, err
		}
		repl, ok := nodeArg(args[1])
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "an Element, not %s", stdlib.KindName(args[1]))
		}
		if old == nil {
			n.add(repl)
		} else {
			n.insert(old.index(), repl)
			old.detach()
		}
		return self, nil
	})
	add := func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if el, ok := nodeArg(arg(args, 0)); ok {
			n.add(el)
			return valueOf(el), nil
		}
		name, err := text(ctx, arg(args, 0))
		if err != nil {
			return object.Value{}, err
		}
		el := &node{kind: kindElement, name: name, doc: n.doc}
		n.add(el)
		return valueOf(el), nil
	}
	def(e, "add", -1, add)
	def(e, "<<", 1, add)
	def(e, "delete", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		el, err := findElement(ctx, n, args[0])
		if err != nil || el == nil {
			return object.Nil, err
		}
		el.detach()
		return valueOf(el), nil
	})
	def(e, "delete_all", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		els, err := elementsMatching(ctx, n, args[0])
		if err != nil {
			return object.Value{}, err
		}
		for _, el := range els {
			el.detach()
		}
		return values(els), nil
	})
	def(e, "each", -1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		els, err := elementsMatching(ctx, n, arg(args, 0))
		if err != nil {
			return object.Value{}, err
		}
		if block.IsZero() {
			return object.EnumeratorValue(&object.Enumerator{Recv: self, Meth: "each", Each: func(ctx object.Context, yield func(object.Value) error) error {
				for _, el := range els {
					if err := yield(valueOf(el)); err != nil {
						return err
					}
				}
				return nil
			}}), nil
		}
		for _, el := range els {
			if _, err := ctx.Yield(block, valueOf(el)); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	def(e, "to_a", -1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		els, err := elementsMatching(ctx, n, arg(args, 0))
		if err != nil {
			return object.Value{}, err
		}
		return values(els), nil
	})
	for _, name := range []string{"size", "length"} {
		def(e, name, 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Int(int64(len(n.elementChildren()))), nil
		})
	}
	def(e, "empty?", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(len(n.elementChildren()) == 0), nil
	})
	def(e, "index", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		el, ok := nodeArg(args[0])
		if !ok {
			return object.Int(-1), nil
		}
		for i, c := range n.elementChildren() {
			if c == el {
				return object.Int(int64(i + 1)), nil
			}
		}
		return object.Int(-1), nil
	})
	def(e, "inspect", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := ctx.Inspect(values(n.elementChildren()))
		return object.String(s), err
	})
	a := attrsCls
	def(a, "[]", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := text(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if at := n.attr(name); at != nil {
			return object.String(at.textValue()), nil
		}
		return object.Nil, nil
	})
	def(a, "[]=", 2, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := text(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if args[1].IsNil() {
			n.deleteAttr(name)
			return self, nil
		}
		v, err := ctx.ToS(args[1])
		if err != nil {
			return object.Value{}, err
		}
		n.setAttr(name, normalize(v))
		return self, nil
	})
	def(a, "get_attribute", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := text(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return valueOf(n.attr(name)), nil
	})
	addAttr := func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		at, ok := nodeArg(args[0])
		if !ok || at.kind != kindAttribute {
			return object.Value{}, ctx.Raise(object.TypeError, "an Attribute, not %s", stdlib.KindName(args[0]))
		}
		n.setAttr(at.name, at.value)
		return self, nil
	}
	def(a, "add", 1, addAttr)
	def(a, "<<", 1, addAttr)
	def(a, "delete", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name := ""
		if at, ok := nodeArg(args[0]); ok {
			name = at.name
		} else {
			var err error
			if name, err = text(ctx, args[0]); err != nil {
				return object.Value{}, err
			}
		}
		n.deleteAttr(name)
		return valueOf(n), nil
	})
	def(a, "delete_all", 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := text(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		var out []*node
		for _, at := range append([]*node(nil), n.attrs...) {
			if at.localName() == name || at.name == name {
				out = append(out, n.deleteAttr(at.name))
			}
		}
		return values(out), nil
	})
	def(a, "each", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		pairs := func(yield func(object.Value) error) error {
			for _, at := range append([]*node(nil), n.attrs...) {
				if err := yield(object.NewList(object.String(at.name), object.String(at.textValue()))); err != nil {
					return err
				}
			}
			return nil
		}
		if block.IsZero() {
			return object.EnumeratorValue(&object.Enumerator{Recv: self, Meth: "each", Each: func(ctx object.Context, yield func(object.Value) error) error { return pairs(yield) }}), nil
		}
		return self, pairs(func(v object.Value) error {
			_, err := ctx.Yield(block, v.List().Elems...)
			return err
		})
	})
	def(a, "each_attribute", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for _, at := range append([]*node(nil), n.attrs...) {
			if _, err := ctx.Yield(block, valueOf(at)); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	for _, name := range []string{"size", "length"} {
		def(a, name, 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Int(int64(len(n.attrs))), nil
		})
	}
	def(a, "empty?", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(len(n.attrs) == 0), nil
	})
	def(a, "keys", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		out := make([]object.Value, len(n.attrs))
		for i, at := range n.attrs {
			out[i] = object.String(at.name)
		}
		return object.NewList(out...), nil
	})
	def(a, "values", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return values(n.attrs), nil
	})
	def(a, "to_a", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return values(n.attrs), nil
	})
	for _, name := range []string{"has_key?", "include?", "key?", "member?"} {
		def(a, name, 1, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			key, err := text(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			return object.Bool(n.attr(key) != nil), nil
		})
	}
	def(a, "prefixes", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		var out []object.Value
		for _, at := range n.attrs {
			if strings.HasPrefix(at.name, "xmlns:") {
				out = append(out, object.String(at.name[6:]))
			}
		}
		return object.NewList(out...), nil
	})
	def(a, "namespaces", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		m := object.NewMap()
		for _, at := range n.attrs {
			if at.name == "xmlns" {
				m = m.Put(object.String("xmlns"), object.String(at.textValue()))
			} else if strings.HasPrefix(at.name, "xmlns:") {
				m = m.Put(object.String(at.name[6:]), object.String(at.textValue()))
			}
		}
		return object.MapValue(m), nil
	})
	def(a, "to_h", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		m := object.NewMap()
		for _, at := range n.attrs {
			m = m.Put(object.String(at.name), object.String(at.textValue()))
		}
		return object.MapValue(m), nil
	})
	def(a, "inspect", 0, func(ctx object.Context, n *node, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		m := object.NewMap()
		for _, at := range n.attrs {
			m = m.Put(object.String(at.name), object.String(at.textValue()))
		}
		s, err := ctx.Inspect(object.MapValue(m))
		return object.String(s), err
	})
}

// installXPath defines XPath.first, match and each.
func installXPath() {
	run := func(ctx object.Context, args []object.Value) ([]*node, xval, error) {
		if len(args) < 1 || len(args) > 4 {
			return nil, xval{}, stdlib.ArgError(ctx, len(args), "1..4")
		}
		n, ok := nodeArg(args[0])
		if !ok {
			return nil, xval{}, ctx.Raise(object.TypeError, "XPath starts at a node, not %s", stdlib.KindName(args[0]))
		}
		p := "*"
		if given(arg(args, 1)) {
			var err error
			if p, err = text(ctx, args[1]); err != nil {
				return nil, xval{}, err
			}
		}
		v, err := xpathFrom(ctx, n, p, arg(args, 2), arg(args, 3))
		if err != nil {
			return nil, xval{}, err
		}
		return v.nodes, v, nil
	}
	xpathCls.DefineSingleton("first", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		nodes, v, err := run(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		if v.kind != xNodes {
			return xvalue(v), nil
		}
		if len(nodes) == 0 {
			return object.Nil, nil
		}
		return valueOf(nodes[0]), nil
	})
	xpathCls.DefineSingleton("match", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		nodes, v, err := run(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		if v.kind != xNodes {
			return object.NewList(xvalue(v)), nil
		}
		return values(nodes), nil
	})
	xpathCls.DefineSingleton("each", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		nodes, v, err := run(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		if v.kind != xNodes {
			_, err := ctx.Yield(block, xvalue(v))
			return object.Nil, err
		}
		for _, n := range nodes {
			if _, err := ctx.Yield(block, valueOf(n)); err != nil {
				return object.Value{}, err
			}
		}
		return object.Nil, nil
	})
}

// installFormatters defines Formatters::Default and Pretty, handles with
// a write method.
func installFormatters() {
	fmtOf := func(v object.Value) *formatter { return v.Instance().Message.(*formatter) }
	defaultCls.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "0..1")
		}
		return object.InstanceValue(&object.Instance{Cls: defaultCls, Message: newDefault(arg(args, 0).Truthy())}), nil
	})
	prettyCls.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "0..2")
		}
		indent := int64(2)
		if given(arg(args, 0)) {
			var err error
			if indent, err = stdlib.WantInt(ctx, args[0], ""); err != nil {
				return object.Value{}, err
			}
		}
		return object.InstanceValue(&object.Instance{Cls: prettyCls, Message: newPretty(int(indent), arg(args, 1).Truthy())}), nil
	})
	defaultCls.Define("write", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, ok := nodeArg(args[0])
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "a formatter writes a node, not %s", stdlib.KindName(args[0]))
		}
		f := fmtOf(self)
		out := &formatter{indent: f.indent, ieHack: f.ieHack, compact: f.compact, width: f.width, b: &strings.Builder{}}
		out.write(n)
		return args[1], emit(ctx, args[1], out.b.String())
	})
	prettyCls.Define("compact", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(fmtOf(self).compact), nil
	})
	prettyCls.Define("compact=", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		fmtOf(self).compact = args[0].Truthy()
		return self, nil
	})
	prettyCls.Define("width", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(int64(fmtOf(self).width)), nil
	})
	prettyCls.Define("width=", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		w, err := stdlib.WantInt(ctx, args[0], "")
		fmtOf(self).width = int(w)
		return self, err
	})
}
