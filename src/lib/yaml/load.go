package yaml

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/cameo-lang/cameo/src/lib/yaml/libyaml"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

// This file is Psych's loading side: the parser's events become a tree of
// nodes, as Psych::Nodes, and the tree becomes values, as
// Psych::Visitors::ToRuby does it, under a class loader that is restricted
// (load, safe_load) or not (unsafe_load, load_stream).

// node is a node of a document: a scalar, a sequence, a mapping or an
// alias.
type node struct {
	kind   libyaml.EventType
	tag    string
	anchor string
	value  string
	// plain and quoted are the scalar's implicit flags: Psych's Scalar#plain
	// and #quoted.
	plain, quoted bool
	children      []*node
}

// parseError turns the parser's refusal into Psych::SyntaxError, whose
// position is libyaml's context mark, one-based.
func parseError(ctx object.Context, file object.Value, err error) error {
	e, ok := err.(*libyaml.Error)
	if !ok {
		return err
	}
	name := "<unknown>"
	if !file.IsNil() {
		name = file.Str()
	}
	problem := e.Problem
	if e.Context != "" {
		problem += " " + e.Context
	}
	line, col := e.ContextMark.Line+1, e.ContextMark.Column+1
	attrs := map[string]object.Value{
		"file":    file,
		"line":    object.Int(line),
		"column":  object.Int(col),
		"offset":  object.Int(e.Offset),
		"problem": object.String(e.Problem),
		"context": object.Nil,
	}
	if e.Context != "" {
		attrs["context"] = object.String(e.Context)
	}
	return &object.Exception{Cls: syntaxError, Attrs: attrs,
		Message: "(" + name + "): " + problem + " at line " + strconv.Itoa(line) + " column " + strconv.Itoa(col)}
}

// documents reads the documents of input, each one's root node. With
// first, it stops after the first document, as Psych.parse does, so a
// later document's error is not raised.
func documents(ctx object.Context, input string, file object.Value, first bool) ([]*node, error) {
	p := libyaml.NewParser([]byte(input))
	var docs []*node
	var stack []*node
	for {
		ev, err := p.Next()
		if err != nil {
			return nil, parseError(ctx, file, err)
		}
		var n *node
		switch ev.Type {
		case libyaml.StreamEnd:
			return docs, nil
		case libyaml.StreamStart, libyaml.DocumentStart:
			continue
		case libyaml.DocumentEnd:
			if first {
				return docs, nil
			}
			continue
		case libyaml.Scalar:
			n = &node{kind: ev.Type, tag: ev.Tag, anchor: ev.Anchor, value: ev.Value, plain: ev.Implicit, quoted: ev.QuotedImplicit}
		case libyaml.Alias:
			n = &node{kind: ev.Type, anchor: ev.Anchor}
		case libyaml.SequenceStart, libyaml.MappingStart:
			n = &node{kind: ev.Type, tag: ev.Tag, anchor: ev.Anchor}
		case libyaml.SequenceEnd, libyaml.MappingEnd:
			stack = stack[:len(stack)-1]
			continue
		}
		if len(stack) == 0 {
			docs = append(docs, n)
		} else {
			parent := stack[len(stack)-1]
			parent.children = append(parent.children, n)
		}
		if n.kind == libyaml.SequenceStart || n.kind == libyaml.MappingStart {
			stack = append(stack, n)
		}
	}
}

// loader builds values from nodes: Psych::Visitors::ToRuby with its class
// loader.
type loader struct {
	ctx object.Context
	// restricted is Psych::ClassLoader::Restricted: only classes names
	// may be loaded, and only symbols when symbols is not empty.
	restricted     bool
	classes        map[string]bool
	symbols        map[string]bool
	aliases        bool
	symbolizeNames bool
	strict         bool
	anchors        map[string]object.Value
}

// disallowed is Psych::DisallowedClass.
func disallowed(action, class string) error {
	return &object.Exception{Cls: disallowedClass, Message: "Tried to " + action + " unspecified class: " + class}
}

// permit checks that the class name may be loaded, as the restricted class
// loader's find does; the class need not exist.
func (l *loader) permit(name string) error {
	if l.restricted && !l.classes[name] {
		return disallowed("load", name)
	}
	return nil
}

// find is the class loader's load: the class name, permitted and existing.
// A missing class is Ruby's ArgumentError.
func (l *loader) find(name string) (*object.Class, error) {
	if name == "" {
		return nil, nil
	}
	if err := l.permit(name); err != nil {
		return nil, err
	}
	cls := classNamed(l.ctx, name)
	if cls == nil {
		return nil, l.ctx.Raise(object.ArgumentError, "undefined class/module %s", name)
	}
	return cls, nil
}

// classNamed resolves A::B from the top level, where the script's own
// classes are.
func classNamed(ctx object.Context, name string) *object.Class {
	var scope *object.Class
	for i, part := range strings.Split(name, "::") {
		var v object.Value
		var ok bool
		if i == 0 {
			if v, ok = ctx.Const(part); !ok {
				v, ok = object.ObjectClass.Consts[part]
			}
		} else {
			v, ok = scope.Consts[part]
		}
		if !ok {
			return nil
		}
		cls, ok := v.AsClass()
		if !ok {
			return nil
		}
		scope = cls
	}
	return scope
}

// symbolize is the class loader's: a Symbol, when Symbol may be loaded and
// the name is among the permitted symbols if any are.
func (l *loader) symbolize(name string) (object.Value, error) {
	if l.restricted && len(l.symbols) > 0 && !l.symbols[name] {
		return object.Value{}, disallowed("load", "Symbol")
	}
	if err := l.permit("Symbol"); err != nil {
		return object.Value{}, err
	}
	return object.Symbol(name), nil
}

// register remembers an anchored node's value for the aliases after it.
func (l *loader) register(n *node, v object.Value) object.Value {
	if n.anchor != "" {
		if l.anchors == nil {
			l.anchors = map[string]object.Value{}
		}
		l.anchors[n.anchor] = v
	}
	return v
}

// accept is the value of a node.
func (l *loader) accept(n *node) (object.Value, error) {
	switch n.kind {
	case libyaml.Alias:
		if !l.aliases {
			return object.Value{}, &object.Exception{Cls: aliasesNotEnabled,
				Message: "Alias parsing was not enabled. To enable it, pass `aliases: true` to `YAML::load` or `YAML::safe_load`."}
		}
		v, ok := l.anchors[n.anchor]
		if !ok {
			return object.Value{}, &object.Exception{Cls: anchorNotDefined, Message: "An alias referenced an unknown anchor: " + n.anchor}
		}
		return v, nil
	case libyaml.Scalar:
		v, err := l.scalar(n)
		if err != nil {
			return object.Value{}, err
		}
		return l.register(n, v), nil
	case libyaml.SequenceStart:
		return l.sequence(n)
	}
	return l.mapping(n)
}

var (
	strTag   = regexp.MustCompile(`^!(?:str|ruby/string)(?::(.*))?$`)
	symTag   = regexp.MustCompile(`^!ruby/sym(bol)?:?(.*)?$`)
	regexpRe = regexp.MustCompile(`(?s)^/(.*)/([mixn]*)$`)
	objTag   = regexp.MustCompile(`^!ruby/object:?(.*)?$`)
	structRe = regexp.MustCompile(`^!ruby/struct:?(.*)?$`)
	excTag   = regexp.MustCompile(`^!ruby/exception:?(.*)?$`)
	arrTag   = regexp.MustCompile(`^!(?:seq|ruby/array):(.*)$`)
	hashTag  = regexp.MustCompile(`^!(?:map|ruby/hash):(.*)$`)
)

// scalar is ToRuby#deserialize.
func (l *loader) scalar(n *node) (object.Value, error) {
	if n.quoted {
		return object.String(n.value), nil
	}
	if n.tag == "" {
		return l.resolve(tokenize(n.value, l.strict))
	}
	switch tag := n.tag; {
	case tag == "!binary" || tag == "tag:yaml.org,2002:binary":
		return object.Bytes(lenientBase64(n.value)), nil
	case tag == "tag:yaml.org,2002:str":
		return object.String(n.value), nil
	case strTag.MatchString(tag):
		if _, err := l.find(strTag.FindStringSubmatch(tag)[1]); err != nil {
			return object.Value{}, err
		}
		return object.String(n.value), nil
	case tag == "!ruby/object:BigDecimal", tag == "!ruby/object:Complex", tag == "!ruby/object:Rational":
		name := strings.TrimPrefix(tag, "!ruby/object:")
		if err := l.permit(name); err != nil {
			return object.Value{}, err
		}
		return object.Value{}, l.ctx.Raise(object.TypeError, "cameo has no %s to load", name)
	case tag == "!ruby/object:DateTime":
		// Psych loads DateTime, then Time to read the timestamp.
		if err := l.permit("DateTime"); err != nil {
			return object.Value{}, err
		}
		if err := l.permit("Time"); err != nil {
			return object.Value{}, err
		}
		return l.dateTime(n.value)
	case tag == "!ruby/encoding":
		return l.ctx.Call(object.ClassValue(object.EncodingClass), "find", object.String(n.value))
	case tag == "!ruby/class" || tag == "!ruby/module":
		cls, err := l.find(n.value)
		if err != nil {
			return object.Value{}, err
		}
		if cls == nil {
			return object.Nil, nil
		}
		return object.ClassValue(cls), nil
	case tag == "tag:yaml.org,2002:float" || tag == "!float":
		v, err := l.resolve(tokenize(n.value, l.strict))
		if err != nil {
			return object.Value{}, err
		}
		return l.ctx.Call(object.ClassValue(object.ObjectClass), "Float", v)
	case tag == "!ruby/regexp":
		if err := l.permit("Regexp"); err != nil {
			return object.Value{}, err
		}
		m := regexpRe.FindStringSubmatch(n.value)
		if m == nil {
			return object.Value{}, l.ctx.Raise(object.ArgumentError, "invalid regexp %q", n.value)
		}
		options := 0
		for _, c := range m[2] {
			switch c {
			case 'x':
				options |= object.RegexpExtended
			case 'i':
				options |= object.RegexpIgnoreCase
			case 'm':
				options |= object.RegexpMultiline
			}
		}
		re, err := object.NewRegexp(m[1], options)
		if err != nil {
			return object.Value{}, l.ctx.Raise(object.RegexpError, "%s", err.Error())
		}
		return object.RegexpValue(re), nil
	case tag == "!ruby/range":
		if err := l.permit("Range"); err != nil {
			return object.Value{}, err
		}
		return l.rangeOf(n.value)
	case symTag.MatchString(tag):
		return l.symbolize(n.value)
	}
	return l.resolve(tokenize(n.value, l.strict))
}

// rangeOf reads "1..3" or "a...b": each end is a plain scalar.
func (l *loader) rangeOf(s string) (object.Value, error) {
	i := strings.Index(s, "..")
	if i < 0 {
		return object.Value{}, l.ctx.Raise(object.ArgumentError, "bad value for range")
	}
	dots := ".."
	if strings.HasPrefix(s[i:], "...") {
		dots = "..."
	}
	lo, err := l.resolve(tokenize(s[:i], l.strict))
	if err != nil {
		return object.Value{}, err
	}
	hi, err := l.resolve(tokenize(s[i+len(dots):], l.strict))
	if err != nil {
		return object.Value{}, err
	}
	return object.RangeValue(&object.Range{Lo: lo, Hi: hi, Exclusive: dots == "..."}), nil
}

// dateTime reads what Psych dumps a DateTime as, through the date
// library's DateTime.strptime.
func (l *loader) dateTime(s string) (object.Value, error) {
	dt, ok := object.ObjectClass.Consts["DateTime"]
	if !ok {
		return object.Value{}, l.ctx.Raise(object.ArgumentError, "undefined class/module DateTime")
	}
	s = strings.TrimSuffix(s, " Z") + map[bool]string{true: " +00:00", false: ""}[strings.HasSuffix(s, " Z")]
	return l.ctx.Call(dt, "strptime", object.String(s), object.String("%Y-%m-%d %H:%M:%S.%N %z"))
}

// resolve is the value of a tokenized plain scalar, with the class checks
// Psych's scanner makes through its class loader: a Symbol, a Date and a
// Time are loaded only when their class may be.
func (l *loader) resolve(sc scalar) (object.Value, error) {
	switch sc.kind {
	case kindNil:
		return object.Nil, nil
	case kindBool:
		return object.Bool(sc.b), nil
	case kindInt:
		if sc.i.IsInt64() {
			return object.Int(sc.i.Int64()), nil
		}
		return object.BigInt(sc.i), nil
	case kindFloat:
		return object.Float(sc.f), nil
	case kindSymbol:
		return l.symbolize(sc.str)
	case kindDate:
		if err := l.permit("Date"); err != nil {
			return object.Value{}, err
		}
		if !sc.valid {
			return object.String(sc.str), nil
		}
		date, ok := object.ObjectClass.Consts["Date"]
		cls, isClass := date.AsClass()
		if !ok || !isClass {
			return object.Value{}, l.ctx.Raise(object.ArgumentError, "undefined class/module Date")
		}
		return l.ctx.Call(date, "new", object.Int(sc.y), object.Int(sc.m), object.Int(sc.d), cls.Consts["GREGORIAN"])
	case kindTime:
		if err := l.permit("Time"); err != nil {
			return object.Value{}, err
		}
		if !sc.valid {
			return object.String(sc.str), nil
		}
		return object.TimeValue(sc.at()), nil
	}
	return object.String(sc.str), nil
}

// lenientBase64 is unpack("m"): the base64 characters count, in groups of
// four, whatever is between them; a group cut short yields the bytes it
// holds.
func lenientBase64(s string) []byte {
	var out []byte
	var acc uint32
	n := 0
	flush := func() {
		switch n {
		case 2:
			out = append(out, byte(acc>>4))
		case 3:
			out = append(out, byte(acc>>10), byte(acc>>2))
		}
		acc, n = 0, 0
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		var v byte
		switch {
		case c >= 'A' && c <= 'Z':
			v = c - 'A'
		case c >= 'a' && c <= 'z':
			v = c - 'a' + 26
		case c >= '0' && c <= '9':
			v = c - '0' + 52
		case c == '+':
			v = 62
		case c == '/':
			v = 63
		case c == '=':
			if n >= 2 {
				flush()
			}
			continue
		default:
			continue
		}
		acc = acc<<6 | uint32(v)
		if n++; n == 4 {
			out = append(out, byte(acc>>16), byte(acc>>8), byte(acc))
			acc, n = 0, 0
		}
	}
	flush()
	return out
}

// sequence is ToRuby#visit_Psych_Nodes_Sequence.
func (l *loader) sequence(n *node) (object.Value, error) {
	switch tag := n.tag; {
	case tag == "!omap" || tag == "tag:yaml.org,2002:omap":
		m := object.NewMap()
		for _, c := range n.children {
			pair, err := l.accept(c)
			if err != nil {
				return object.Value{}, err
			}
			if pm, ok := pair.AsMap(); ok {
				for i, k := range pm.Keys() {
					m = m.Put(k, pm.Values()[i])
				}
			}
		}
		return l.register(n, object.MapValue(m)), nil
	case arrTag.MatchString(tag):
		if _, err := l.find(arrTag.FindStringSubmatch(tag)[1]); err != nil {
			return object.Value{}, err
		}
	}
	elems := make([]object.Value, 0, len(n.children))
	for _, c := range n.children {
		v, err := l.accept(c)
		if err != nil {
			return object.Value{}, err
		}
		elems = append(elems, v)
	}
	return l.register(n, object.NewList(elems...)), nil
}

// pairs reads a mapping's keys and values.
func (l *loader) pairs(n *node) (keys, values []object.Value, err error) {
	for i := 0; i+1 < len(n.children); i += 2 {
		k, err := l.accept(n.children[i])
		if err != nil {
			return nil, nil, err
		}
		v, err := l.accept(n.children[i+1])
		if err != nil {
			return nil, nil, err
		}
		keys = append(keys, k)
		values = append(values, v)
	}
	return keys, values, nil
}

// mapping is ToRuby#visit_Psych_Nodes_Mapping.
func (l *loader) mapping(n *node) (object.Value, error) {
	if n.tag == "" {
		return l.hash(n, false)
	}
	switch tag := n.tag; {
	case structRe.MatchString(tag):
		name := structRe.FindStringSubmatch(tag)[1]
		if name == "" {
			return object.Value{}, l.ctx.Raise(object.ArgumentError, "cameo has no Struct to load")
		}
		return l.object(n, name)
	case objTag.MatchString(tag):
		name := objTag.FindStringSubmatch(tag)[1]
		switch name {
		case "Complex", "Rational":
			if err := l.permit(name); err != nil {
				return object.Value{}, err
			}
			return object.Value{}, l.ctx.Raise(object.TypeError, "cameo has no %s to load", name)
		case "Hash":
			return l.hash(n, false)
		case "Set":
			if err := l.permit("Set"); err != nil {
				return object.Value{}, err
			}
			return l.set(n)
		}
		return l.object(n, name)
	case strTag.MatchString(tag) || tag == "tag:yaml.org,2002:str":
		var cls string
		if m := strTag.FindStringSubmatch(tag); m != nil {
			cls = m[1]
		}
		if _, err := l.find(cls); err != nil {
			return object.Value{}, err
		}
		keys, values, err := l.pairs(n)
		if err != nil {
			return object.Value{}, err
		}
		for i, k := range keys {
			if s, ok := k.AsString(); ok && s == "str" {
				return l.register(n, values[i]), nil
			}
		}
		return object.Nil, nil
	case arrTag.MatchString(tag):
		if _, err := l.find(arrTag.FindStringSubmatch(tag)[1]); err != nil {
			return object.Value{}, err
		}
		keys, values, err := l.pairs(n)
		if err != nil {
			return object.Value{}, err
		}
		for i, k := range keys {
			if s, ok := k.AsString(); ok && s == "internal" {
				return l.register(n, values[i]), nil
			}
		}
		return object.NewList(), nil
	case tag == "!ruby/range":
		if err := l.permit("Range"); err != nil {
			return object.Value{}, err
		}
		keys, values, err := l.pairs(n)
		if err != nil {
			return object.Value{}, err
		}
		r := &object.Range{Lo: object.Nil, Hi: object.Nil}
		for i, k := range keys {
			switch s, _ := k.AsString(); s {
			case "begin":
				r.Lo = values[i]
			case "end":
				r.Hi = values[i]
			case "excl":
				r.Exclusive = values[i].Truthy()
			}
		}
		return l.register(n, object.RangeValue(r)), nil
	case excTag.MatchString(tag):
		name := excTag.FindStringSubmatch(tag)[1]
		cls, err := l.find(name)
		if err != nil {
			return object.Value{}, err
		}
		if cls == nil {
			cls = object.ExceptionClass
		}
		keys, values, err := l.pairs(n)
		if err != nil {
			return object.Value{}, err
		}
		msg := object.Nil
		for i, k := range keys {
			if s, ok := k.AsString(); ok && s == "message" {
				msg = values[i]
			}
		}
		v, err := l.ctx.Call(object.ClassValue(cls), "new", msg)
		if err != nil {
			return object.Value{}, err
		}
		return l.register(n, v), nil
	case tag == "!set" || tag == "tag:yaml.org,2002:set":
		// Ruby loads a set as Psych::Set, a Hash; cameo has no Psych name and
		// gives the Hash itself, under the name Set.
		if err := l.permit("Set"); err != nil {
			return object.Value{}, err
		}
		return l.hash(n, true)
	case tag == "!omap" || tag == "tag:yaml.org,2002:omap":
		return l.hash(n, true)
	case hashTag.MatchString(tag):
		if _, err := l.find(hashTag.FindStringSubmatch(tag)[1]); err != nil {
			return object.Value{}, err
		}
		return l.hash(n, true)
	case strings.HasPrefix(tag, "!ruby/hash-with-ivars"), strings.HasPrefix(tag, "!ruby/marshalable:"):
		name := tag[strings.Index(tag, ":")+1:]
		if !strings.Contains(tag, ":") {
			name = "Hash"
		}
		if _, err := l.find(name); err != nil {
			return object.Value{}, err
		}
		return object.Value{}, l.ctx.Raise(object.ArgumentError, "Cannot deserialize %s", name)
	}
	return l.hash(n, false)
}

// hash is revive_hash: the pairs into a Hash, with the merge key << and
// symbolized names where asked. tagged says the mapping had a tag, which
// keeps its keys as they are.
func (l *loader) hash(n *node, tagged bool) (object.Value, error) {
	m := object.NewMap()
	for i := 0; i+1 < len(n.children); i += 2 {
		kn, vn := n.children[i], n.children[i+1]
		k, err := l.accept(kn)
		if err != nil {
			return object.Value{}, err
		}
		v, err := l.accept(vn)
		if err != nil {
			return object.Value{}, err
		}
		if s, ok := k.AsString(); ok && s == "<<" && kn.tag != "tag:yaml.org,2002:str" {
			switch vn.kind {
			case libyaml.Alias, libyaml.MappingStart:
				if vm, ok := v.AsMap(); ok {
					m = m.Merge(vm)
					continue
				}
			case libyaml.SequenceStart:
				if list, ok := v.AsList(); ok {
					merged := object.NewMap()
					allMaps := true
					for j := len(list.Elems) - 1; j >= 0; j-- {
						em, ok := list.Elems[j].AsMap()
						if !ok {
							allMaps = false
							break
						}
						merged = merged.Merge(em)
					}
					if allMaps {
						m = m.Merge(merged)
						continue
					}
				}
			}
			m = m.Put(k, v)
			continue
		}
		if !tagged && l.symbolizeNames {
			if s, ok := k.AsString(); ok {
				k = object.Symbol(s)
			}
		}
		m = m.Put(k, v)
	}
	return l.register(n, object.MapValue(m)), nil
}

// set reads what Ruby's Set dumps as: a mapping with a "hash" of the
// elements to true.
func (l *loader) set(n *node) (object.Value, error) {
	keys, values, err := l.pairs(n)
	if err != nil {
		return object.Value{}, err
	}
	s := object.NewSet()
	for i, k := range keys {
		if name, ok := k.AsString(); ok && name == "hash" {
			if hm, ok := values[i].AsMap(); ok {
				s = object.NewSet(hm.Keys()...)
			}
		}
	}
	return l.register(n, object.SetValue(s)), nil
}

// object revives !ruby/object:Name: an instance of a cameo class, its
// fields set from the mapping's keys through with, which checks their
// types; a key that is not a field is with's NameError, where Ruby would
// set an instance variable.
func (l *loader) object(n *node, name string) (object.Value, error) {
	cls := object.ObjectClass
	if name != "" {
		var err error
		if cls, err = l.find(name); err != nil {
			return object.Value{}, err
		}
	}
	keys, values, err := l.pairs(n)
	if err != nil {
		return object.Value{}, err
	}
	inst := object.InstanceValue(&object.Instance{Cls: cls, Fields: make([]object.Value, len(cls.Fields))})
	if len(keys) == 0 {
		return l.register(n, inst), nil
	}
	changes := object.NewMap()
	for i, k := range keys {
		s, err := l.ctx.ToS(k)
		if err != nil {
			return object.Value{}, err
		}
		changes = changes.Put(object.Symbol(s), values[i])
	}
	v, err := l.ctx.CallWith(inst, "with", nil, changes, object.Value{})
	if err != nil {
		return object.Value{}, err
	}
	return l.register(n, v), nil
}

// wantString is the yaml argument: a String, or Ruby's TypeError.
func wantString(ctx object.Context, v object.Value) (string, error) {
	return stdlib.WantString(ctx, v)
}
