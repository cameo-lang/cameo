package rexml

import (
	"strconv"
	"strings"
)

// options are what Document.new takes besides the source.
type options struct {
	ignoreWhitespace []string // the element names, or ["*"] for :all
	compress         []string
	raw              []string
}

func (o *options) applies(names []string, element string) bool {
	for _, n := range names {
		if n == "*" || n == element {
			return true
		}
	}
	return false
}

// parseError is REXML::ParseException's content: the message, and where
// the parser was.
type parseError struct {
	msg      string
	line     int
	position int
	rest     string
}

// Error renders the message as REXML does, with the position and the last
// unconsumed characters.
func (e *parseError) Error() string {
	rest := e.rest
	if len(rest) > 80 {
		rest = rest[:80]
	}
	return e.msg + "\nLine: " + strconv.Itoa(e.line) + "\nPosition: " + strconv.Itoa(e.position) + "\nLast 80 unconsumed characters:\n" + rest
}

// parser reads a document from a string.
type parser struct {
	s    string
	pos  int
	opts *options
	doc  *node
}

func (p *parser) fail(msg string) *parseError {
	return &parseError{msg: msg, line: strings.Count(p.s[:p.pos], "\n") + 1, position: p.pos, rest: p.s[p.pos:]}
}

func (p *parser) has(prefix string) bool { return strings.HasPrefix(p.s[p.pos:], prefix) }

func (p *parser) eof() bool { return p.pos >= len(p.s) }

func (p *parser) skipSpace() {
	for !p.eof() && isSpace(p.s[p.pos]) {
		p.pos++
	}
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// until consumes up to a terminator, returning what came before it, or
// fails with msg when it is missing.
func (p *parser) until(term, msg string) (string, *parseError) {
	i := strings.Index(p.s[p.pos:], term)
	if i < 0 {
		return "", p.fail(msg)
	}
	out := p.s[p.pos : p.pos+i]
	p.pos += i + len(term)
	return out, nil
}

// name reads an XML name.
func (p *parser) name() (string, *parseError) {
	start := p.pos
	if p.eof() || !isNameStart(p.s[p.pos]) {
		return "", p.fail("malformed XML: missing tag start")
	}
	for !p.eof() && isNameChar(p.s[p.pos]) {
		p.pos++
	}
	return p.s[start:p.pos], nil
}

// parse builds the document. Text between the document's parts is kept
// as REXML keeps it, so that it writes back as it came.
func parse(src string, opts *options) (*node, *parseError) {
	settings := &document{raw: opts.applies(opts.raw, "")}
	doc := &node{kind: kindDocument, doc: settings}
	p := &parser{s: src, opts: opts, doc: doc}
	var stack []*node
	current := doc
	hasRoot := false
	for !p.eof() {
		switch {
		case p.has("<?xml") && p.pos == 0:
			p.pos += 5
			decl, err := p.xmlDecl()
			if err != nil {
				return nil, err
			}
			doc.add(decl)
		case p.has("<?"):
			p.pos += 2
			target, err := p.name()
			if err != nil {
				return nil, err
			}
			body, err := p.until("?>", "Malformed XML: Unclosed processing instruction")
			if err != nil {
				return nil, err
			}
			current.add(&node{kind: kindInstruction, target: target, content: strings.TrimSpace(body), doc: settings})
		case p.has("<!--"):
			p.pos += 4
			body, err := p.until("-->", "Unclosed comment")
			if err != nil {
				return nil, err
			}
			current.add(&node{kind: kindComment, value: body, doc: settings})
		case p.has("<![CDATA["):
			if current == doc {
				return nil, p.fail("Malformed XML: Content at the start of the document (got '<![CDATA[')")
			}
			p.pos += 9
			body, err := p.until("]]>", "Malformed CDATA: Missing end ']]>'")
			if err != nil {
				return nil, err
			}
			current.add(&node{kind: kindCData, value: body, doc: settings})
		case p.has("<!DOCTYPE"):
			p.pos += 9
			dt, err := p.doctype()
			if err != nil {
				return nil, err
			}
			settings.doctype = dt
			doc.add(dt)
		case p.has("</"):
			p.pos += 2
			name, err := p.name()
			if err != nil {
				return nil, err
			}
			p.skipSpace()
			if !p.has(">") {
				return nil, p.fail("Missing end tag for '" + name + "'")
			}
			p.pos++
			if current == doc {
				return nil, p.fail("Missing end tag for '" + name + "'")
			}
			if current.name != name {
				return nil, p.fail("Missing end tag for '" + current.name + "' (got '" + name + "')")
			}
			current = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
		case p.has("<"):
			p.pos++
			el, closed, err := p.startTag(settings)
			if err != nil {
				if err.msg == "malformed XML: missing tag start" {
					err.position--
					err.rest = p.s[err.position:]
				}
				return nil, err
			}
			if current == doc {
				if hasRoot {
					return nil, p.fail("attempted adding second root element to document")
				}
				hasRoot = true
			}
			current.add(el)
			if !closed {
				stack = append(stack, current)
				current = el
			}
		default:
			text := p.s[p.pos:]
			if i := strings.IndexByte(text, '<'); i >= 0 {
				text = text[:i]
			}
			p.pos += len(text)
			if current == doc && strings.TrimSpace(text) != "" {
				got := strings.TrimSpace(text)
				if hasRoot {
					return nil, p.fail("Malformed XML: Extra content at the end of the document (got '" + got + "')")
				}
				return nil, p.fail("Malformed XML: Content at the start of the document (got '" + got + "')")
			}
			if current == doc {
				// Whitespace after the root element is dropped, as REXML
				// drops it; before it, it is kept.
				if !hasRoot {
					doc.add(&node{kind: kindText, value: text, doc: settings})
				}
				continue
			}
			// A parsed Text is raw: its content is already normalized.
			t := &node{kind: kindText, value: text, doc: settings, raw: true}
			if opts.applies(opts.ignoreWhitespace, current.name) && strings.TrimSpace(text) == "" {
				continue
			}
			if opts.applies(opts.compress, current.name) {
				t.value = squeezeSpace(text)
			}
			current.add(t)
		}
	}
	if current != doc {
		return nil, p.fail("Missing end tag for '/" + current.name + "'")
	}
	return doc, nil
}

// squeezeSpace folds each run of one whitespace character to one, as
// compress_whitespace does.
func squeezeSpace(s string) string {
	var b strings.Builder
	var last byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isSpace(c) && c == last {
			continue
		}
		b.WriteByte(c)
		last = c
	}
	return b.String()
}

// startTag reads a tag after its <: the name, the attributes, and whether
// it closed itself.
func (p *parser) startTag(settings *document) (el *node, closed bool, err *parseError) {
	name, err := p.name()
	if err != nil {
		return nil, false, err
	}
	el = &node{kind: kindElement, name: name, doc: settings}
	for {
		p.skipSpace()
		switch {
		case p.eof():
			return nil, false, p.fail("Missing end tag for '" + name + "'")
		case p.has("/>"):
			p.pos += 2
			return el, true, nil
		case p.has(">"):
			p.pos++
			return el, false, nil
		}
		aname, err := p.name()
		if err != nil {
			return nil, false, p.fail("Missing attribute name in <" + name + ">")
		}
		p.skipSpace()
		if !p.has("=") {
			return nil, false, p.fail("Missing attribute equal: <" + aname + ">")
		}
		p.pos++
		p.skipSpace()
		if p.eof() || p.s[p.pos] != '"' && p.s[p.pos] != '\'' {
			return nil, false, p.fail("Missing attribute value start quote: <" + aname + ">")
		}
		q := p.s[p.pos : p.pos+1]
		p.pos++
		value, err := p.until(q, "Missing attribute value end quote: <"+aname+">")
		if err != nil {
			return nil, false, err
		}
		if el.attr(aname) != nil {
			return nil, false, p.fail("Duplicate attribute " + strconv.Quote(aname))
		}
		el.attrs = append(el.attrs, &node{kind: kindAttribute, name: aname, value: value, parent: el, doc: settings})
	}
}

// xmlDecl reads <?xml version='1.0' encoding='UTF-8' standalone='yes'?>
// after its <?xml.
func (p *parser) xmlDecl() (*node, *parseError) {
	body, err := p.until("?>", "Malformed XML: Unclosed XML declaration")
	if err != nil {
		return nil, err
	}
	n := &node{kind: kindXMLDecl, version: "1.0", writeDecl: true}
	for _, key := range []string{"version", "encoding", "standalone"} {
		i := strings.Index(body, key)
		if i < 0 {
			continue
		}
		rest := strings.TrimLeft(body[i+len(key):], " \t\n\r")
		if !strings.HasPrefix(rest, "=") {
			continue
		}
		rest = strings.TrimLeft(rest[1:], " \t\n\r")
		if rest == "" {
			continue
		}
		q := rest[0]
		if q != '"' && q != '\'' {
			continue
		}
		end := strings.IndexByte(rest[1:], q)
		if end < 0 {
			continue
		}
		v := rest[1 : 1+end]
		switch key {
		case "version":
			n.version = v
		case "encoding":
			n.encoding = strings.ToUpper(v)
		case "standalone":
			n.standalone = v
		}
	}
	return n, nil
}

// doctype reads <!DOCTYPE name [external id] [[subset]]> after its
// <!DOCTYPE, with the entity declarations of the subset.
func (p *parser) doctype() (*node, *parseError) {
	p.skipSpace()
	name, err := p.name()
	if err != nil {
		return nil, err
	}
	n := &node{kind: kindDocType, name: name}
	p.skipSpace()
	if p.has("PUBLIC") || p.has("SYSTEM") {
		n.externalID = p.s[p.pos : p.pos+6]
		p.pos += 6
		p.skipSpace()
		first, err := p.quoted()
		if err != nil {
			return nil, err
		}
		if n.externalID == "PUBLIC" {
			n.pub = first
			p.skipSpace()
			if !p.has(">") && !p.has("[") {
				if n.sys, err = p.quoted(); err != nil {
					return nil, err
				}
			}
		} else {
			n.sys = first
		}
		p.skipSpace()
	}
	if p.has("[") {
		p.pos++
		subset, err := p.until("]", "Malformed DOCTYPE: Missing ']'")
		if err != nil {
			return nil, err
		}
		n.subset = subset
		n.entities = parseEntities(subset)
		p.skipSpace()
	}
	if !p.has(">") {
		return nil, p.fail("Malformed DOCTYPE: Missing '>'")
	}
	p.pos++
	return n, nil
}

func (p *parser) quoted() (string, *parseError) {
	if p.eof() || p.s[p.pos] != '"' && p.s[p.pos] != '\'' {
		return "", p.fail("Malformed DOCTYPE: Missing quote")
	}
	q := p.s[p.pos : p.pos+1]
	p.pos++
	return p.until(q, "Malformed DOCTYPE: Missing end quote")
}

// parseEntities finds the general entity declarations of an internal
// subset: <!ENTITY name "value">.
func parseEntities(subset string) []entity {
	var out []entity
	rest := subset
	for {
		i := strings.Index(rest, "<!ENTITY")
		if i < 0 {
			return out
		}
		rest = strings.TrimLeft(rest[i+8:], " \t\n\r")
		if strings.HasPrefix(rest, "%") {
			continue
		}
		end := 0
		for end < len(rest) && isNameChar(rest[end]) {
			end++
		}
		name := rest[:end]
		rest = strings.TrimLeft(rest[end:], " \t\n\r")
		if rest == "" || rest[0] != '"' && rest[0] != '\'' {
			continue
		}
		q := rest[0]
		close := strings.IndexByte(rest[1:], q)
		if close < 0 {
			return out
		}
		out = append(out, entity{name: name, value: rest[1 : 1+close]})
		rest = rest[2+close:]
	}
}
