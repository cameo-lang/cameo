package rexml

import (
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/object"
)

// kind is what a node is. The tree is REXML's: a Document holds an
// XMLDecl, a DocType, Comments, Instructions, Texts and one root Element;
// an Element holds Attributes and children of every kind but these three.
type kind int

const (
	kindDocument kind = iota
	kindElement
	kindText
	kindCData
	kindComment
	kindInstruction
	kindXMLDecl
	kindDocType
	kindAttribute
)

// node is one node of the tree: a handle, as REXML's objects are, so the
// same Value stands for it each time a script reaches it.
type node struct {
	kind     kind
	parent   *node
	children []*node // document, element
	attrs    []*node // element: in order of definition
	name     string  // element and attribute: the name as written, with its prefix
	// text, cdata, comment, attribute: the content. A Text's and an
	// Attribute's content is in its normalized form, with entity
	// references, as REXML keeps it.
	value string
	raw   bool // text: the content came in as it is, so it is written as it is
	// instruction: target and content
	target  string
	content string
	// xmldecl
	version    string
	encoding   string
	standalone string
	writeDecl  bool // false for the default declaration, which writes nothing
	// doctype
	externalID string
	pub        string
	sys        string
	subset     string
	entities   []entity
	val        object.Value // the Value of this node, once a script has it
	doc        *document    // the document's settings, for a text's entities
	// elements and attributes are the Values of the Elements and the
	// Attributes views of an element, once a script has them.
	elements   object.Value
	attributes object.Value
}

type entity struct{ name, value string }

// document holds what a parse's options set.
type document struct {
	raw     bool
	doctype *node
}

func (n *node) element() bool { return n.kind == kindElement }

// isText says a node is a Text or a CData, which REXML derives from Text.
func (n *node) isText() bool { return n.kind == kindText || n.kind == kindCData }

// prefix and localName split an element's or attribute's name.
func (n *node) prefix() string {
	if i := strings.IndexByte(n.name, ':'); i >= 0 {
		return n.name[:i]
	}
	return ""
}

func (n *node) localName() string {
	if i := strings.IndexByte(n.name, ':'); i >= 0 {
		return n.name[i+1:]
	}
	return n.name
}

// root is the document's root element, nil for none.
func (n *node) root() *node {
	d := n.document()
	if d == nil {
		return nil
	}
	for _, c := range d.children {
		if c.element() {
			return c
		}
	}
	return nil
}

// document is the Document the node is in, nil for a detached tree.
func (n *node) document() *node {
	for p := n; p != nil; p = p.parent {
		if p.kind == kindDocument {
			return p
		}
	}
	return nil
}

// top is the node's topmost ancestor, which REXML calls root_node.
func (n *node) top() *node {
	p := n
	for p.parent != nil {
		p = p.parent
	}
	return p
}

// owner is the element an attribute belongs to; an element's own parent
// otherwise.
func (n *node) owner() *node { return n.parent }

// add appends a child and sets its parent, detaching it from a former one.
func (n *node) add(c *node) {
	c.detach()
	c.parent = n
	n.children = append(n.children, c)
}

// insert puts a child at an index.
func (n *node) insert(i int, c *node) {
	c.detach()
	c.parent = n
	if i < 0 {
		i = 0
	}
	if i > len(n.children) {
		i = len(n.children)
	}
	n.children = append(n.children, nil)
	copy(n.children[i+1:], n.children[i:])
	n.children[i] = c
}

// detach removes the node from its parent.
func (n *node) detach() {
	if n.parent == nil {
		return
	}
	p := n.parent
	for i, c := range p.children {
		if c == n {
			p.children = append(p.children[:i], p.children[i+1:]...)
			break
		}
	}
	n.parent = nil
}

// index is the node's position among its parent's children, -1 for none.
func (n *node) index() int {
	if n.parent == nil {
		return -1
	}
	for i, c := range n.parent.children {
		if c == n {
			return i
		}
	}
	return -1
}

// elementChildren are the children that are elements.
func (n *node) elementChildren() []*node {
	var out []*node
	for _, c := range n.children {
		if c.element() {
			out = append(out, c)
		}
	}
	return out
}

// attr finds an attribute by its name as written.
func (n *node) attr(name string) *node {
	for _, a := range n.attrs {
		if a.name == name {
			return a
		}
	}
	return nil
}

// setAttr adds an attribute or changes the one of that name, keeping
// its place.
func (n *node) setAttr(name, normalized string) *node {
	if a := n.attr(name); a != nil {
		a.value = normalized
		return a
	}
	a := &node{kind: kindAttribute, name: name, value: normalized, parent: n, doc: n.doc}
	n.attrs = append(n.attrs, a)
	return a
}

func (n *node) deleteAttr(name string) *node {
	for i, a := range n.attrs {
		if a.name == name {
			n.attrs = append(n.attrs[:i], n.attrs[i+1:]...)
			a.parent = nil
			return a
		}
	}
	return nil
}

// namespace resolves a prefix at this element: the URI of xmlns:prefix,
// or of xmlns for the empty prefix, on it or an ancestor; "" for none.
func (n *node) namespace(prefix string) (string, bool) {
	want := "xmlns"
	if prefix != "" {
		want = "xmlns:" + prefix
	}
	for p := n; p != nil; p = p.parent {
		if p.kind == kindAttribute {
			continue
		}
		if a := p.attr(want); a != nil {
			return unnormalize(a.value, p.doc), true
		}
	}
	return "", false
}

// namespaces are the prefixes in scope with their URIs, nearest first
// winning, in order of definition from the top down.
func (n *node) namespaces() (names []string, uris map[string]string) {
	uris = map[string]string{}
	var chain []*node
	for p := n; p != nil; p = p.parent {
		if p.element() {
			chain = append(chain, p)
		}
	}
	for i := len(chain) - 1; i >= 0; i-- {
		for _, a := range chain[i].attrs {
			var prefix string
			switch {
			case a.name == "xmlns":
				prefix = "xmlns"
			case strings.HasPrefix(a.name, "xmlns:"):
				prefix = a.name[6:]
			default:
				continue
			}
			if _, seen := uris[prefix]; !seen {
				names = append(names, prefix)
			}
			uris[prefix] = unnormalize(a.value, a.doc)
		}
	}
	return names, uris
}

// namespaceURI is the URI of the element's or attribute's own prefix. An
// unprefixed attribute has none; an unprefixed element has the default
// namespace in scope.
func (n *node) namespaceURI() string {
	prefix := n.prefix()
	if prefix == "xmlns" {
		return ""
	}
	if prefix == "" && n.kind == kindAttribute {
		return ""
	}
	at := n
	if n.kind == kindAttribute {
		at = n.parent
	}
	if at == nil {
		return ""
	}
	uri, _ := at.namespace(prefix)
	return uri
}

// stringValue is XPath's string-value: the text of a text node, the value
// of an attribute, the concatenated text under an element or document.
func (n *node) stringValue() string {
	switch n.kind {
	case kindText, kindCData, kindAttribute:
		return n.textValue()
	case kindComment:
		return n.value
	case kindInstruction:
		return n.content
	case kindElement, kindDocument:
		var b strings.Builder
		n.collectText(&b)
		return b.String()
	}
	return ""
}

func (n *node) collectText(b *strings.Builder) {
	for _, c := range n.children {
		switch {
		case c.isText():
			b.WriteString(c.textValue())
		case c.element():
			c.collectText(b)
		}
	}
}

// textValue is a Text's or Attribute's value with its entity references
// resolved, what Text#value and Attribute#value give.
func (n *node) textValue() string {
	if n.kind == kindCData {
		return n.value
	}
	return unnormalize(n.value, n.doc)
}

// firstText is the first Text child, what Element#get_text gives.
func (n *node) firstText() *node {
	for _, c := range n.children {
		if c.isText() {
			return c
		}
	}
	return nil
}

// Normalization, REXML's Text.normalize and Text.unnormalize.

// normalize escapes the five characters, leaving an entity reference as
// it is, as Text.new does to a string that is not raw.
func normalize(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '&':
			if end := entityEnd(s, i); end > 0 {
				b.WriteString(s[i:end])
				i = end - 1
			} else {
				b.WriteString("&amp;")
			}
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// entityEnd is the index after an entity reference that starts at i, 0
// for none: &name; &#123; or &#x1F;.
func entityEnd(s string, i int) int {
	j := i + 1
	if j < len(s) && s[j] == '#' {
		j++
		hex := j < len(s) && (s[j] == 'x' || s[j] == 'X')
		if hex {
			j++
		}
		start := j
		for j < len(s) && (isDigit(s[j]) || hex && isHexLetter(s[j])) {
			j++
		}
		if j == start || j >= len(s) || s[j] != ';' {
			return 0
		}
		return j + 1
	}
	start := j
	for j < len(s) && isNameChar(s[j]) {
		j++
	}
	if j == start || j >= len(s) || s[j] != ';' {
		return 0
	}
	return j + 1
}

func isDigit(c byte) bool     { return c >= '0' && c <= '9' }
func isHexLetter(c byte) bool { return c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }
func isNameStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c == ':' || c >= 0x80
}
func isNameChar(c byte) bool { return isNameStart(c) || isDigit(c) || c == '-' || c == '.' }

var predefined = map[string]string{"lt": "<", "gt": ">", "amp": "&", "quot": "\"", "apos": "'"}

// unnormalize resolves the entity references: the five, numeric ones, and
// those the document's DOCTYPE declares. An unknown one stays as it is.
func unnormalize(s string, doc *document) string {
	if !strings.Contains(s, "&") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '&' {
			b.WriteByte(c)
			continue
		}
		end := entityEnd(s, i)
		if end == 0 {
			b.WriteByte(c)
			continue
		}
		ref := s[i+1 : end-1]
		switch {
		case strings.HasPrefix(ref, "#x") || strings.HasPrefix(ref, "#X"):
			n, err := strconv.ParseUint(ref[2:], 16, 32)
			if err != nil || !utf8.ValidRune(rune(n)) {
				b.WriteString(s[i:end])
			} else {
				b.WriteRune(rune(n))
			}
		case strings.HasPrefix(ref, "#"):
			n, err := strconv.ParseUint(ref[1:], 10, 32)
			if err != nil || !utf8.ValidRune(rune(n)) {
				b.WriteString(s[i:end])
			} else {
				b.WriteRune(rune(n))
			}
		default:
			if v, ok := predefined[ref]; ok {
				b.WriteString(v)
			} else if v, ok := declaredEntity(ref, doc); ok {
				b.WriteString(unnormalize(v, nil))
			} else {
				b.WriteString(s[i:end])
			}
		}
		i = end - 1
	}
	return b.String()
}

// declaredEntity looks an entity up in the document's DOCTYPE.
func declaredEntity(name string, doc *document) (string, bool) {
	if doc == nil || doc.doctype == nil {
		return "", false
	}
	for _, e := range doc.doctype.entities {
		if e.name == name {
			return e.value, true
		}
	}
	return "", false
}

// Writing: REXML's Default and Pretty formatters.

// formatter writes a tree as REXML::Formatters::Default does, or as
// Pretty does when indent is 0 or more.
type formatter struct {
	indent  int // -1 for Default
	ieHack  bool
	compact bool
	width   int
	level   int
	b       *strings.Builder
}

func newDefault(ieHack bool) *formatter {
	return &formatter{indent: -1, ieHack: ieHack, b: &strings.Builder{}}
}

func newPretty(indent int, ieHack bool) *formatter {
	return &formatter{indent: indent, ieHack: ieHack, width: 80, b: &strings.Builder{}}
}

func (f *formatter) pretty() bool { return f.indent >= 0 }

func (f *formatter) pad() {
	if f.pretty() {
		f.b.WriteString(strings.Repeat(" ", f.level))
	}
}

// write renders a node; the result accumulates in f.b.
func (f *formatter) write(n *node) {
	switch n.kind {
	case kindDocument:
		f.writeDocument(n)
	case kindElement:
		f.writeElement(n)
	case kindText:
		f.writeText(n)
	case kindCData:
		f.pad()
		f.b.WriteString("<![CDATA[" + n.value + "]]>")
	case kindComment:
		f.pad()
		f.b.WriteString("<!--" + n.value + "-->")
	case kindInstruction:
		f.pad()
		f.b.WriteString(instructionString(n))
	case kindXMLDecl:
		f.b.WriteString(declString(n))
	case kindDocType:
		f.b.WriteString(doctypeString(n))
	case kindAttribute:
		f.b.WriteString(attributeString(n))
	}
}

func (f *formatter) writeDocument(n *node) {
	if !f.pretty() {
		for _, c := range n.children {
			f.write(c)
		}
		return
	}
	// A Text child of the document is whitespace between the parts; the
	// parts go one per line, but for a declaration that writes nothing.
	first := true
	for _, c := range n.children {
		if c.kind == kindText {
			continue
		}
		if !first {
			f.b.WriteString("\n")
		}
		if c.kind != kindXMLDecl || c.writeDecl {
			first = false
		}
		f.write(c)
	}
}

func (f *formatter) writeElement(n *node) {
	f.pad()
	f.b.WriteString("<" + n.name)
	attrs := n.attrs
	if !f.pretty() {
		// The default formatter writes the attributes in name order.
		attrs = append([]*node(nil), attrs...)
		sort.SliceStable(attrs, func(i, j int) bool { return attrs[i].name < attrs[j].name })
	}
	for _, a := range attrs {
		f.b.WriteString(" " + attributeString(a))
	}
	if len(n.children) == 0 {
		if f.ieHack {
			f.b.WriteString(" ")
		}
		f.b.WriteString("/>")
		return
	}
	f.b.WriteString(">")
	switch {
	case !f.pretty():
		for _, c := range n.children {
			f.write(c)
		}
	case f.compact && f.allText(n) && f.fitsOnLine(n):
	default:
		f.b.WriteString("\n")
		f.level += f.indent
		for _, c := range n.children {
			if c.kind == kindText && strings.TrimSpace(c.value) == "" {
				continue
			}
			f.write(c)
			f.b.WriteString("\n")
		}
		f.level -= f.indent
		f.pad()
	}
	f.b.WriteString("</" + n.name + ">")
}

func (f *formatter) allText(n *node) bool {
	for _, c := range n.children {
		if !c.isText() {
			return false
		}
	}
	return true
}

// fitsOnLine writes an element's text children on the current line when
// they are shorter than the width, and reports whether it did.
func (f *formatter) fitsOnLine(n *node) bool {
	sub := &formatter{indent: f.indent, ieHack: f.ieHack, compact: f.compact, width: f.width, b: &strings.Builder{}}
	for _, c := range n.children {
		sub.write(c)
	}
	s := sub.b.String()
	if len(s) >= f.width {
		return false
	}
	f.b.WriteString(s)
	return true
}

func (f *formatter) writeText(n *node) {
	s := textString(n)
	if !f.pretty() {
		f.b.WriteString(s)
		return
	}
	// Pretty folds the whitespace, wraps at the width and indents each
	// line.
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r == '\r' || r == '\f' || r == '\v' {
			return ' '
		}
		return r
	}, s)
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	s = wrap(s, f.width-f.level)
	pad := strings.Repeat(" ", f.level)
	f.b.WriteString(pad + strings.ReplaceAll(s, "\n", "\n"+pad))
}

// wrap breaks a string at the last space before the width, as REXML's
// Pretty does, until the rest fits.
func wrap(s string, width int) string {
	var parts []string
	for len(s) > width {
		at := strings.LastIndex(s[:min(width+1, len(s))], " ")
		if at < 0 {
			break
		}
		parts = append(parts, s[:at])
		s = s[at+1:]
	}
	parts = append(parts, s)
	return strings.Join(parts, "\n")
}

// textString is a Text's to_s: its normalized content, written as it is
// when raw.
func textString(n *node) string {
	if n.kind == kindCData {
		return "<![CDATA[" + n.value + "]]>"
	}
	return n.value
}

// attributeString is an Attribute's to_string, name='value' with the
// value's apostrophes as entity references.
func attributeString(a *node) string {
	return a.name + "='" + strings.ReplaceAll(a.value, "'", "&apos;") + "'"
}

func declString(n *node) string {
	if !n.writeDecl {
		return ""
	}
	s := "<?xml version='" + n.version + "'"
	if n.encoding != "" {
		s += " encoding='" + n.encoding + "'"
	}
	if n.standalone != "" {
		s += " standalone='" + n.standalone + "'"
	}
	return s + "?>"
}

func instructionString(n *node) string {
	if n.content == "" {
		return "<?" + n.target + "?>"
	}
	return "<?" + n.target + " " + n.content + "?>"
}

func doctypeString(n *node) string {
	s := "<!DOCTYPE " + n.name
	if n.externalID != "" {
		s += " " + n.externalID
		if n.pub != "" {
			s += " " + quoteID(n.pub)
		}
		if n.sys != "" {
			s += " " + quoteID(n.sys)
		}
	}
	if n.subset != "" {
		s += " [" + n.subset + "]"
	}
	return s + ">"
}

func quoteID(s string) string {
	if strings.Contains(s, "\"") {
		return "'" + s + "'"
	}
	return "\"" + s + "\""
}

// String is a node's to_s: the default formatter's rendering.
func (n *node) String() string {
	switch n.kind {
	case kindText, kindCData, kindComment:
		// A CData's and a Comment's to_s is the content; written in a
		// tree, they get their brackets.
		return n.value
	case kindAttribute:
		return strings.ReplaceAll(n.value, "'", "&apos;")
	}
	f := newDefault(false)
	f.write(n)
	return f.b.String()
}

// inspect is a node's inspect in REXML's forms.
func (n *node) inspect() string {
	switch n.kind {
	case kindDocument:
		if len(n.children) == 0 {
			return "<UNDEFINED/>"
		}
		return "<UNDEFINED> ... </>"
	case kindElement:
		var b strings.Builder
		b.WriteString("<" + n.name)
		for _, a := range n.attrs {
			b.WriteString(" " + attributeString(a))
		}
		if len(n.children) == 0 {
			b.WriteString("/>")
		} else {
			b.WriteString("> ... </>")
		}
		return b.String()
	case kindAttribute:
		return attributeString(n)
	case kindText, kindCData:
		return strconv.Quote(n.String())
	}
	return n.String()
}

// deepCopy copies a node and what is under it, detached.
func (n *node) deepCopy() *node {
	c := *n
	c.parent = nil
	c.val, c.elements, c.attributes = object.Value{}, object.Value{}, object.Value{}
	c.children = nil
	c.attrs = nil
	for _, a := range n.attrs {
		ac := *a
		ac.parent = &c
		ac.val = object.Value{}
		c.attrs = append(c.attrs, &ac)
	}
	for _, ch := range n.children {
		cc := ch.deepCopy()
		cc.parent = &c
		c.children = append(c.children, cc)
	}
	return &c
}

// shallowCopy copies an element with its attributes but no children.
func (n *node) shallowCopy() *node {
	c := n.deepCopy()
	c.children = nil
	return c
}

// xpath is the path to the node from the document, as Element#xpath
// gives it: /library/book[2].
func (n *node) xpath() string {
	if n.kind == kindDocument {
		return ""
	}
	if n.kind == kindAttribute {
		return n.parent.xpath() + "/@" + n.name
	}
	var parts []string
	for p := n; p != nil && p.kind != kindDocument; p = p.parent {
		step := p.name
		switch p.kind {
		case kindText, kindCData:
			step = "text()"
		case kindComment:
			step = "comment()"
		case kindInstruction:
			step = "processing-instruction()"
		}
		if p.parent != nil {
			pos, count := 0, 0
			for _, s := range p.parent.children {
				same := s.kind == p.kind && (p.kind != kindElement || s.name == p.name)
				if same {
					count++
				}
				if s == p {
					pos = count
				}
			}
			if count > 1 {
				step += "[" + strconv.Itoa(pos) + "]"
			}
		}
		parts = append([]string{step}, parts...)
	}
	return "/" + strings.Join(parts, "/")
}
