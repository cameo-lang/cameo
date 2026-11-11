// Package ir is cameo's intermediate representation: the listing of a
// unit, and the compiler from the syntax package's AST. The messages are
// in package cameopb (cameo/ir.proto), and the design in doc/design/ir.md.
package ir

import (
	"fmt"
	"math/bits"
	"strconv"
	"strings"

	"github.com/cameo-lang/cameo/src/cameopb"
)

// Version is the IR version this cameo compiles to and runs. A dump is
// resumed by a cameo of the same version.
const Version = 1

// Listing returns a unit as text, one instruction per line, the lists an
// instruction carries indented under it:
//
//	proc #1 double_all(list)
//	  LOAD list
//	  LAMBDA #2
//	  SEND map 0 block
func Listing(u *cameopb.Unit) string {
	l := &lister{unit: u}
	for i, p := range u.GetProcs() {
		if i > 0 {
			l.b.WriteByte('\n')
		}
		l.proc(i, p)
	}
	return l.b.String()
}

type lister struct {
	unit *cameopb.Unit
	b    strings.Builder
}

// name returns the name at index i of the unit, or a marker for an index
// the unit does not have.
func (l *lister) name(i uint32) string {
	if int(i) < len(l.unit.GetNames()) {
		return l.unit.Names[i]
	}
	return fmt.Sprintf("<name %d>", i)
}

func (l *lister) proc(i int, p *cameopb.Proc) {
	fmt.Fprintf(&l.b, "proc #%d %s", i, p.GetName())
	if params := l.params(p.GetParams()); params != "" {
		fmt.Fprintf(&l.b, "(%s)", params)
	}
	if p.GetShared() {
		l.b.WriteString(" shared")
	}
	l.b.WriteByte('\n')
	for _, param := range p.GetParams().GetPositional() {
		if param.GetDefault() != nil {
			fmt.Fprintf(&l.b, "  default %s:\n", l.name(param.GetName()))
			l.seq(param.GetDefault().GetCode(), 2)
		}
	}
	for _, param := range p.GetParams().GetKeywords() {
		if param.GetDefault() != nil {
			fmt.Fprintf(&l.b, "  default %s:\n", l.name(param.GetName()))
			l.seq(param.GetDefault().GetCode(), 2)
		}
	}
	l.seq(p.GetCode(), 1)
}

// params renders a parameter list as Ruby writes it, a default as `= …`.
func (l *lister) params(p *cameopb.Params) string {
	if p == nil {
		return ""
	}
	var out []string
	for _, param := range p.GetPositional() {
		switch {
		case param.GetSplat():
			out = append(out, "*"+l.name(param.GetName()))
		case param.GetDefault() != nil:
			out = append(out, l.name(param.GetName())+" = …")
		default:
			out = append(out, l.name(param.GetName()))
		}
	}
	for _, param := range p.GetKeywords() {
		s := l.name(param.GetName()) + ":"
		if param.GetDefault() != nil {
			s += " …"
		}
		out = append(out, s)
	}
	if p.KwRest != nil {
		out = append(out, "**"+l.name(p.GetKwRest()))
	}
	if p.Block != nil {
		out = append(out, "&"+l.name(p.GetBlock()))
	}
	return strings.Join(out, ", ")
}

func (l *lister) seq(code []*cameopb.Code, depth int) {
	for _, c := range code {
		l.code(c, depth)
	}
}

func (l *lister) code(c *cameopb.Code, depth int) {
	indent := strings.Repeat("  ", depth)
	l.b.WriteString(indent)
	l.b.WriteString(c.GetOp().String())
	for _, a := range c.GetArgs() {
		l.b.WriteByte(' ')
		l.b.WriteString(l.arg(a))
	}
	for _, f := range flagNames(c.GetFlags()) {
		l.b.WriteByte(' ')
		l.b.WriteString(f)
	}
	l.b.WriteByte('\n')
	for i, s := range c.GetCode() {
		if label := seqLabel(c, i); label != "" {
			fmt.Fprintf(&l.b, "%s  %s:\n", indent, label)
			l.seq(s.GetCode(), depth+2)
		} else {
			l.seq(s.GetCode(), depth+1)
		}
	}
}

func (l *lister) arg(a *cameopb.Arg) string {
	switch k := a.GetKind().(type) {
	case *cameopb.Arg_Constant:
		return constant(k.Constant)
	case *cameopb.Arg_Name:
		if name := l.name(k.Name); name != "" {
			return name
		}
		return `""`
	case *cameopb.Arg_Proc:
		return fmt.Sprintf("#%d", k.Proc)
	case *cameopb.Arg_Int:
		return strconv.FormatInt(k.Int, 10)
	case *cameopb.Arg_Pattern:
		return l.pattern(k.Pattern)
	case *cameopb.Arg_Field:
		return l.field(k.Field)
	}
	return "?"
}

// constant renders a literal value as Ruby's inspect does.
func constant(v *cameopb.Constant) string {
	switch k := v.GetKind().(type) {
	case nil:
		return "nil"
	case *cameopb.Constant_BoolValue:
		return strconv.FormatBool(k.BoolValue)
	case *cameopb.Constant_IntValue:
		return strconv.FormatInt(k.IntValue, 10)
	case *cameopb.Constant_BigValue:
		return k.BigValue
	case *cameopb.Constant_FloatValue:
		s := strconv.FormatFloat(k.FloatValue, 'g', -1, 64)
		if !strings.ContainsAny(s, ".eEIN") {
			s += ".0"
		}
		return s
	case *cameopb.Constant_StringValue:
		return strconv.Quote(k.StringValue)
	case *cameopb.Constant_BytesValue:
		return "b" + strconv.Quote(string(k.BytesValue))
	}
	return fmt.Sprintf("<%T>", v.GetKind())
}

// pattern renders a pattern in the syntax of case/in, with … for the code
// inside it.
func (l *lister) pattern(p *cameopb.Pattern) string {
	list := func(ps []*cameopb.Pattern) []string {
		out := make([]string, len(ps))
		for i, p := range ps {
			out[i] = l.pattern(p)
		}
		return out
	}
	constOf := func(s *cameopb.Seq) string {
		if s == nil {
			return ""
		}
		return "…"
	}
	rest := func(name *uint32) string {
		if name == nil {
			return "*"
		}
		return "*" + l.name(*name)
	}
	switch k := p.GetKind().(type) {
	case *cameopb.Pattern_Value:
		return "…"
	case *cameopb.Pattern_Bind:
		return l.name(k.Bind.GetName())
	case *cameopb.Pattern_Const:
		return "…"
	case *cameopb.Pattern_Binding:
		return l.pattern(k.Binding.GetSub()) + " => " + l.name(k.Binding.GetName())
	case *cameopb.Pattern_List:
		parts := list(k.List.GetPre())
		if k.List.GetHasSplat() {
			parts = append(parts, rest(k.List.SplatName))
		}
		parts = append(parts, list(k.List.GetPost())...)
		return constOf(k.List.GetConst()) + "[" + strings.Join(parts, ", ") + "]"
	case *cameopb.Pattern_Map:
		var parts []string
		for _, e := range k.Map.GetEntries() {
			s := l.name(e.GetKey()) + ":"
			if e.GetValue() != nil {
				s += " " + l.pattern(e.GetValue())
			}
			parts = append(parts, s)
		}
		switch {
		case k.Map.GetRestNil():
			parts = append(parts, "**nil")
		case k.Map.GetHasRest() && k.Map.RestName != nil:
			parts = append(parts, "**"+l.name(k.Map.GetRestName()))
		case k.Map.GetHasRest():
			parts = append(parts, "**")
		}
		return constOf(k.Map.GetConst()) + "{" + strings.Join(parts, ", ") + "}"
	case *cameopb.Pattern_Alt:
		return strings.Join(list(k.Alt.GetAlts()), " | ")
	case *cameopb.Pattern_Find:
		parts := append([]string{rest(k.Find.PreName)}, list(k.Find.GetMid())...)
		parts = append(parts, rest(k.Find.PostName))
		return constOf(k.Find.GetConst()) + "[" + strings.Join(parts, ", ") + "]"
	}
	return "?"
}

// field renders a field declaration as a class body writes it.
func (l *lister) field(f *cameopb.FieldDecl) string {
	parts := []string{f.GetType()}
	for _, n := range f.GetNames() {
		parts = append(parts, ":"+n)
	}
	s := strings.Join(parts, " ")
	if f.GetOf() != "" {
		s += " of:" + f.GetOf()
	}
	if f.GetKey() != "" {
		s += " key:" + f.GetKey()
	}
	if f.GetValue() != "" {
		s += " value:" + f.GetValue()
	}
	if len(f.GetValues()) > 0 {
		s += " values:[" + strings.Join(f.GetValues(), ",") + "]"
	}
	if f.GetNumber() != 0 {
		s += " number:" + strconv.FormatInt(f.GetNumber(), 10)
	}
	return "(" + s + ")"
}

// flagNames returns the names of the flags set in flags, lowercase, in
// the order of their bits.
func flagNames(flags uint64) []string {
	var out []string
	for flags != 0 {
		bit := uint64(1) << bits.TrailingZeros64(flags)
		flags &^= bit
		name, ok := cameopb.Flag_name[int32(bit)]
		if !ok {
			out = append(out, fmt.Sprintf("flag:%#x", bit))
			continue
		}
		out = append(out, strings.ToLower(strings.TrimPrefix(name, "FLAG_")))
	}
	return out
}

// seqLabel names list i of instruction c, "" for a list that needs no
// name: the one list of AND, OR and SAFE.
func seqLabel(c *cameopb.Code, i int) string {
	switch c.GetOp() {
	case cameopb.Op_IF:
		return pick(i, "then", "else")
	case cameopb.Op_WHILE:
		return pick(i, "cond", "body")
	case cameopb.Op_AND, cameopb.Op_OR, cameopb.Op_SAFE:
		return ""
	case cameopb.Op_DEFINED_CONST:
		return "expr"
	case cameopb.Op_DEFINED_METHOD:
		return "recv"
	case cameopb.Op_BEGIN:
		if i < 3 {
			return pick(i, "body", "else", "ensure")
		}
		return clauseLabel("rescue", i-3, "classes", "")
	case cameopb.Op_CASE:
		if c.GetFlags()&uint64(cameopb.Flag_FLAG_ELSE) != 0 && i == len(c.GetCode())-1 {
			return "else"
		}
		return clauseLabel("when", i, "conds", "")
	case cameopb.Op_CASE_IN:
		if c.GetFlags()&uint64(cameopb.Flag_FLAG_ELSE) != 0 && i == len(c.GetCode())-1 {
			return "else"
		}
		return clauseLabel("in", i, "guard", "")
	}
	return strconv.Itoa(i)
}

func pick(i int, labels ...string) string {
	if i < len(labels) {
		return labels[i]
	}
	return strconv.Itoa(i)
}

// clauseLabel names list i of the pairs of lists a clause has: "when 0
// conds" for the first of clause 0 and "when 0" for its body.
func clauseLabel(word string, i int, first, second string) string {
	s := fmt.Sprintf("%s %d", word, i/2)
	part := second
	if i%2 == 0 {
		part = first
	}
	if part != "" {
		s += " " + part
	}
	return s
}
