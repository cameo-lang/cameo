// Package optparse is Ruby's optparse library, built into cameo:
// OptionParser, with on, on_tail and on_head, separator, banner, version,
// help, parse, order and permute and their bang forms, getopts, and the
// errors of a bad command line.
//
// It follows optparse 0.6.0, the version Ruby 3.4 ships: the switch
// forms, the argument types and completions, the abbreviation of long
// options, the help text with its summary width and indent, the built-in
// --help and --version, and ParseError with its kinds and messages. ARGV
// is a value, so parse! returns the operands and leaves it as it is, and
// the options go through the blocks; into: fills a Hash in place in Ruby
// and is refused here, getopts gives the Hash instead.
package optparse

import (
	"github.com/cameo-lang/cameo/src/str"
	"math/big"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	class        = object.NewClass("OptionParser", object.ObjectClass)
	parseError   = object.NewClass("ParseError", object.RuntimeError)
	invalidOpt   = object.NewClass("InvalidOption", parseError)
	missingArg   = object.NewClass("MissingArgument", parseError)
	invalidArg   = object.NewClass("InvalidArgument", parseError)
	needlessArg  = object.NewClass("NeedlessArgument", parseError)
	ambiguousOpt = object.NewClass("AmbiguousOption", parseError)
	ambiguousArg = object.NewClass("AmbiguousArgument", parseError)
)

// argKind says whether a switch takes an argument.
type argKind int

const (
	noArg argKind = iota
	required
	optional
)

// item is one line of the summary: a switch, or a separator's text.
type item struct {
	sep    string
	isSep  bool
	sw     *sw
	isTail bool
	isHead bool
}

// sw is one switch: its names, its argument and how it converts, and the
// block it calls.
type sw struct {
	shorts    []string // without the dash
	longs     []string // without the dashes
	negatable bool     // --[no-]name
	noForm    bool     // defined as --no-name: seen gives false
	arg       string   // the placeholder, "" for none
	argText   string   // the placeholder as written for the help, its separator included: "=NAME", " [N]", "[=N]"
	kind      argKind
	conv      *converter
	desc      []string
	block     object.Value
}

// converter turns an argument's text into the value the block gets.
type converter struct {
	typ   string         // Integer, Float, Numeric, String, Array, TrueClass, FalseClass, or ""
	enum  []object.Value // the permitted values, completed by prefix
	table *object.Map    // a completion table
}

// parser is an OptionParser's state: a handle, like Ruby's object.
type parser struct {
	banner      object.Value // Nil until set
	program     string
	version     object.Value
	release     object.Value
	width       int
	indent      string
	items       []item
	hasHelp     bool
	hasVersion  bool
	defaultArgv object.Value
	// options holds what the last parse saw, by the key Ruby's into:
	// gives a switch: the first long name as a symbol, the short one
	// without one. A block cannot write into its surroundings, so parse
	// records every switch here, and calls the block as well.
	options *object.Map
}

// key is the name options records a switch under: the one into: gives
// it, a --no-thing switch keeping its no-.
func (s *sw) key() object.Value {
	if len(s.longs) > 0 {
		if s.noForm {
			return object.Symbol("no-" + s.longs[0])
		}
		return object.Symbol(s.longs[0])
	}
	return object.Symbol(s.shorts[0])
}

func parserOf(v object.Value) *parser { return v.Instance().Message.(*parser) }

// fail makes a ParseError of cls about the arguments, with Ruby's message.
func fail(cls *object.Class, reason string, args ...string) error {
	list := make([]object.Value, len(args))
	for i, a := range args {
		list[i] = object.NewString(a, str.UTF8)
	}
	return &object.Exception{Cls: cls, Message: reason + ": " + strings.Join(args, " "),
		Attrs: map[string]object.Value{"args": object.NewList(list...), "reason": object.NewString(reason, str.UTF8)}}
}

// programName is the script's name without its extension, as $0 gives it.
func programName(ctx object.Context) string {
	if f, ok := ctx.(interface{ File() string }); ok && f.File() != "" {
		base := filepath.Base(f.File())
		return strings.TrimSuffix(base, filepath.Ext(base))
	}
	return "cameo"
}

func (p *parser) bannerText(ctx object.Context) string {
	if !p.banner.IsNil() && !p.banner.IsZero() {
		s, _ := ctx.ToS(p.banner)
		return s
	}
	return "Usage: " + p.program + " [options]"
}

// define reads on's arguments: the switch strings, a type or completion,
// and the description lines.
func (p *parser) define(ctx object.Context, args []object.Value, block object.Value) (*sw, error) {
	s := &sw{block: block}
	for _, a := range args {
		switch {
		case a.IsString():
			text := a.Str()
			if strings.HasPrefix(text, "-") && s.desc == nil {
				if err := s.addSwitch(ctx, text); err != nil {
					return nil, err
				}
				continue
			}
			s.desc = append(s.desc, text)
		case a.Kind() == object.KindClass:
			cls, _ := a.AsClass()
			name := cls.QualifiedName()
			switch name {
			case "Boolean":
				s.conv = &converter{typ: "TrueClass"}
			case "Integer", "Float", "Numeric", "String", "Array", "TrueClass", "FalseClass":
				s.conv = &converter{typ: name}
			default:
				return nil, ctx.Raise(object.ArgumentError, "unsupported argument type: %s", name)
			}
		case a.Kind() == object.KindList:
			s.conv = &converter{enum: a.List().Elems}
		case a.Kind() == object.KindMap:
			s.conv = &converter{table: a.Map()}
		case a.IsNil():
		default:
			return nil, ctx.Raise(object.ArgumentError, "unsupported argument type: %s", stdlib.KindName(a))
		}
	}
	if len(s.shorts) == 0 && len(s.longs) == 0 {
		return nil, ctx.Raise(object.ArgumentError, "no switch given")
	}
	if s.conv != nil && s.kind == noArg {
		s.kind = required
	}
	return s, nil
}

// addSwitch reads one switch string: -v, -nNAME, -n NAME, -n [NAME],
// --name, --name NAME, --name=NAME, --name [NAME], --name[=NAME],
// --[no-]name, --no-name.
func (s *sw) addSwitch(ctx object.Context, text string) error {
	name, rest := text, ""
	i := strings.IndexAny(text, " =[")
	if i == 2 && strings.HasPrefix(text, "--[") {
		// The [ of --[no-]name is not an argument; the argument may follow.
		i = strings.IndexAny(text[3:], " =[")
		if i >= 0 {
			i += 3
		}
	}
	written := ""
	if i >= 0 {
		name, rest, written = text[:i], strings.TrimLeft(text[i:], " ="), text[i:]
	}
	if strings.HasPrefix(name, "--") {
		long := name[2:]
		switch {
		case strings.HasPrefix(long, "[no-]"):
			s.negatable = true
			long = long[5:]
		case strings.HasPrefix(long, "no-"):
			s.noForm = true
			long = long[3:]
		}
		s.longs = append(s.longs, long)
	} else if strings.HasPrefix(name, "-") && len(name) >= 2 {
		s.shorts = append(s.shorts, name[1:2])
		if len(name) > 2 && rest == "" {
			rest, written = name[2:], name[2:] // -nNAME
		}
	} else {
		return ctx.Raise(object.ArgumentError, "invalid switch: %s", text)
	}
	if rest != "" {
		if strings.HasPrefix(rest, "[") {
			s.kind = optional
			s.arg = strings.TrimPrefix(strings.TrimSuffix(strings.TrimPrefix(rest, "["), "]"), "=")
		} else {
			s.kind = required
			s.arg = rest
		}
		if s.argText == "" {
			s.argText = written
		}
	}
	return nil
}

// summary is the switch's column in the help: shorts, longs and the
// argument placeholder.
func (s *sw) summary() string {
	var b strings.Builder
	if len(s.shorts) > 0 {
		for i, sh := range s.shorts {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString("-" + sh)
		}
		if len(s.longs) > 0 {
			b.WriteString(", ")
		}
	} else {
		b.WriteString("    ")
	}
	for i, l := range s.longs {
		if i > 0 {
			b.WriteString(", ")
		}
		switch {
		case s.negatable:
			b.WriteString("--[no-]" + l)
		case s.noForm:
			b.WriteString("--no-" + l)
		default:
			b.WriteString("--" + l)
		}
	}
	// The placeholder as it was written: --name=NAME, --delay[=N], -nNAME.
	b.WriteString(s.argText)
	return b.String()
}

// lines are the help lines of a switch under a width and an indent.
func (s *sw) lines(width int, indent string) []string {
	left := s.summary()
	var out []string
	descIndent := indent + strings.Repeat(" ", width+1)
	if len(s.desc) == 0 {
		return []string{indent + left}
	}
	if len(left) > width {
		out = append(out, indent+left)
		for _, d := range s.desc {
			out = append(out, descIndent+d)
		}
		return out
	}
	out = append(out, indent+left+strings.Repeat(" ", width-len(left))+" "+s.desc[0])
	for _, d := range s.desc[1:] {
		out = append(out, descIndent+d)
	}
	return out
}

// summarize is the help's lines, heads first and tails last.
func (p *parser) summarize() []string {
	var out []string
	order := [][]item{nil, nil, nil}
	for _, it := range p.items {
		switch {
		case it.isHead:
			order[0] = append(order[0], it)
		case it.isTail:
			order[2] = append(order[2], it)
		default:
			order[1] = append(order[1], it)
		}
	}
	for _, group := range order {
		for _, it := range group {
			if it.isSep {
				out = append(out, it.sep)
				continue
			}
			out = append(out, it.sw.lines(p.width, p.indent)...)
		}
	}
	return out
}

func (p *parser) help(ctx object.Context) string {
	var b strings.Builder
	b.WriteString(p.bannerText(ctx) + "\n")
	for _, l := range p.summarize() {
		b.WriteString(strings.TrimRight(l, " ") + "\n")
	}
	return b.String()
}

func (p *parser) ver(ctx object.Context) object.Value {
	if p.version.IsZero() || p.version.IsNil() {
		return object.Nil
	}
	version, _ := ctx.ToS(p.version)
	s := p.program + " " + version
	if !p.release.IsZero() && !p.release.IsNil() {
		release, _ := ctx.ToS(p.release)
		s += " (" + release + ")"
	}
	return object.NewString(s, str.UTF8)
}

// switches lists the switches, heads first and tails last.
func (p *parser) switches() []*sw {
	var out []*sw
	for _, it := range p.items {
		if !it.isSep {
			out = append(out, it.sw)
		}
	}
	return out
}

// long finds a long option by name, completing a unique prefix; neg says
// the no- form was used.
func (p *parser) long(name string) (s *sw, neg bool, err error) {
	type hit struct {
		s    *sw
		neg  bool
		full string
	}
	var exact, prefixes []hit
	for _, s := range p.switches() {
		for _, l := range s.longs {
			forms := []hit{{s: s}}
			if s.negatable {
				forms = []hit{{s: s}, {s: s, neg: true}}
			}
			for _, h := range forms {
				full := l
				if h.neg || s.noForm {
					full = "no-" + l
				}
				h.full = full
				if full == name {
					exact = append(exact, h)
				} else if strings.HasPrefix(full, name) {
					prefixes = append(prefixes, h)
				}
			}
		}
	}
	if len(exact) > 0 {
		return exact[0].s, exact[0].neg, nil
	}
	if len(prefixes) == 0 {
		return nil, false, fail(invalidOpt, "invalid option", "--"+name)
	}
	// A prefix completes as Ruby's Completion completes it: to the one
	// candidate, to the one switch every candidate names, or to the
	// shortest candidate when it is a prefix of all the others, as --lis
	// is --list beside --list-all.
	shortest := prefixes[0]
	for _, h := range prefixes[1:] {
		if len(h.full) < len(shortest.full) {
			shortest = h
		}
	}
	for _, h := range prefixes {
		if h.s != shortest.s && !strings.HasPrefix(h.full, shortest.full) {
			return nil, false, fail(ambiguousOpt, "ambiguous option", "--"+name)
		}
	}
	return shortest.s, shortest.neg, nil
}

func (p *parser) short(name string) *sw {
	for _, s := range p.switches() {
		for _, sh := range s.shorts {
			if sh == name {
				return s
			}
		}
	}
	return nil
}

// convert turns the argument text into the block's value.
func (c *converter) convert(ctx object.Context, shown []string, text string) (object.Value, error) {
	bad := func() error { return fail(invalidArg, "invalid argument", shown...) }
	if c == nil {
		return object.NewString(text, str.UTF8), nil
	}
	if c.table != nil {
		var hits []int
		keys := c.table.Keys()
		for i, k := range keys {
			ks, _ := ctx.ToS(k)
			if ks == text {
				return c.table.Values()[i], nil
			}
			if strings.HasPrefix(ks, text) {
				hits = append(hits, i)
			}
		}
		if len(hits) == 1 {
			return c.table.Values()[hits[0]], nil
		}
		if len(hits) > 1 {
			return object.Value{}, fail(ambiguousArg, "ambiguous argument", shown...)
		}
		return object.Value{}, bad()
	}
	if c.enum != nil {
		var hits []object.Value
		for _, e := range c.enum {
			es, err := ctx.ToS(e)
			if err != nil {
				return object.Value{}, err
			}
			if es == text {
				return e, nil
			}
			if strings.HasPrefix(es, text) {
				hits = append(hits, e)
			}
		}
		if len(hits) == 1 {
			return hits[0], nil
		}
		if len(hits) > 1 {
			return object.Value{}, fail(ambiguousArg, "ambiguous argument", shown...)
		}
		return object.Value{}, bad()
	}
	switch c.typ {
	case "Integer":
		n, ok := parseInt(text)
		if !ok {
			return object.Value{}, bad()
		}
		return n, nil
	case "Float":
		f, err := strconv.ParseFloat(strings.ReplaceAll(text, "_", ""), 64)
		if err != nil {
			return object.Value{}, bad()
		}
		return object.Float(f), nil
	case "Numeric":
		if n, ok := parseInt(text); ok {
			return n, nil
		}
		if num, den, ok := strings.Cut(text, "/"); ok {
			a, errA := strconv.ParseFloat(num, 64)
			b, errB := strconv.ParseFloat(den, 64)
			if errA == nil && errB == nil && b != 0 {
				return object.Float(a / b), nil
			}
		}
		f, err := strconv.ParseFloat(strings.ReplaceAll(text, "_", ""), 64)
		if err != nil {
			return object.Value{}, bad()
		}
		return object.Float(f), nil
	case "Array":
		parts := strings.Split(text, ",")
		out := make([]object.Value, len(parts))
		for i, part := range parts {
			out[i] = object.NewString(part, str.UTF8)
		}
		return object.NewList(out...), nil
	case "TrueClass", "FalseClass":
		switch strings.ToLower(text) {
		case "yes", "true", "+", "":
			return object.Bool(c.typ == "TrueClass"), nil
		case "no", "false", "-", "nil":
			return object.Bool(c.typ != "TrueClass"), nil
		}
		return object.Value{}, bad()
	}
	return object.NewString(text, str.UTF8), nil
}

// parseInt is Kernel#Integer on an option's text.
func parseInt(text string) (object.Value, bool) {
	s := strings.ReplaceAll(text, "_", "")
	if s == "" || strings.Contains(text, "__") {
		return object.Value{}, false
	}
	neg := false
	if s[0] == '-' || s[0] == '+' {
		neg = s[0] == '-'
		s = s[1:]
	}
	base := 10
	switch {
	case len(s) > 2 && (s[:2] == "0x" || s[:2] == "0X"):
		base, s = 16, s[2:]
	case len(s) > 2 && (s[:2] == "0b" || s[:2] == "0B"):
		base, s = 2, s[2:]
	case len(s) > 2 && (s[:2] == "0o" || s[:2] == "0O"):
		base, s = 8, s[2:]
	case len(s) > 1 && s[0] == '0':
		base, s = 8, s[1:]
	}
	n, ok := new(big.Int).SetString(s, base)
	if !ok {
		return object.Value{}, false
	}
	if neg {
		n.Neg(n)
	}
	if n.IsInt64() {
		return object.Int(n.Int64()), true
	}
	return object.BigInt(n), true
}

// parse walks the arguments: options to their blocks or into store, the
// operands returned. order stops at the first operand.
func (p *parser) parse(ctx object.Context, argv []object.Value, order bool, store func(s *sw, v object.Value)) ([]object.Value, error) {
	var rest []object.Value
	args := make([]string, len(argv))
	for i, a := range argv {
		s, err := stdlib.WantString(ctx, a)
		if err != nil {
			return nil, err
		}
		args[i] = s
	}
	deliver := func(s *sw, v object.Value) error {
		if store != nil {
			store(s, v)
		}
		if p.options != nil {
			p.options = p.options.Put(s.key(), v)
		}
		if !s.block.IsZero() && !s.block.IsNil() {
			_, err := ctx.Yield(s.block, v)
			return err
		}
		return nil
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			for _, r := range args[i+1:] {
				rest = append(rest, object.NewString(r, str.UTF8))
			}
			return rest, nil
		case strings.HasPrefix(a, "--"):
			name, value, hasValue := strings.Cut(a[2:], "=")
			s, neg, err := p.long(name)
			if err != nil {
				if handled, berr := p.builtin(ctx, name, hasValue, a); handled {
					if berr != nil {
						return nil, berr
					}
					continue
				}
				return nil, err
			}
			shown := []string{a}
			switch s.kind {
			case noArg:
				if hasValue {
					return nil, fail(needlessArg, "needless argument", a)
				}
				if err := deliver(s, object.Bool(!neg && !s.noForm)); err != nil {
					return nil, err
				}
			case required:
				if !hasValue {
					if i+1 >= len(args) {
						return nil, fail(missingArg, "missing argument", a)
					}
					i++
					value = args[i]
					shown = []string{a, value}
				}
				v, err := s.conv.convert(ctx, shown, value)
				if err != nil {
					return nil, err
				}
				if err := deliver(s, v); err != nil {
					return nil, err
				}
			case optional:
				if !hasValue && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					i++
					value, hasValue = args[i], true
					shown = []string{a, value}
				}
				if !hasValue {
					if err := deliver(s, object.Nil); err != nil {
						return nil, err
					}
					continue
				}
				v, err := s.conv.convert(ctx, shown, value)
				if err != nil {
					return nil, err
				}
				if err := deliver(s, v); err != nil {
					return nil, err
				}
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			// A cluster of short options, the last of which may take the
			// rest as its argument.
			for j := 1; j < len(a); j++ {
				name := a[j : j+1]
				s := p.short(name)
				if s == nil {
					// A short option no switch has completes to a long one,
					// as -n is --name when that is the one completion, as
					// Ruby's parse_in_order falls back; then the builtins.
					if long, _, err := p.long(name); err == nil {
						s = long
					} else if handled, berr := p.builtin(ctx, name, false, a); handled {
						if berr != nil {
							return nil, berr
						}
						break
					} else {
						return nil, fail(invalidOpt, "invalid option", "-"+name)
					}
				}
				attached := a[j+1:]
				switch s.kind {
				case noArg:
					if err := deliver(s, object.Bool(!s.noForm)); err != nil {
						return nil, err
					}
					continue
				case required:
					value, shown := attached, []string{"-" + name + attached}
					if attached == "" {
						if i+1 >= len(args) {
							return nil, fail(missingArg, "missing argument", "-"+name)
						}
						i++
						value = args[i]
						shown = []string{"-" + name, value}
					}
					v, err := s.conv.convert(ctx, shown, value)
					if err != nil {
						return nil, err
					}
					if err := deliver(s, v); err != nil {
						return nil, err
					}
				case optional:
					if attached == "" {
						if err := deliver(s, object.Nil); err != nil {
							return nil, err
						}
					} else {
						v, err := s.conv.convert(ctx, []string{"-" + name + attached}, attached)
						if err != nil {
							return nil, err
						}
						if err := deliver(s, v); err != nil {
							return nil, err
						}
					}
				}
				j = len(a)
			}
		default:
			if order {
				for _, r := range args[i:] {
					rest = append(rest, object.NewString(r, str.UTF8))
				}
				return rest, nil
			}
			rest = append(rest, object.NewString(a, str.UTF8))
		}
	}
	return rest, nil
}

// builtin handles --help, -h, --version and -v when the parser defines
// none of its own: the text goes to the standard output and the script
// exits; --help with an argument is refused, and --version without a
// version aborts as Ruby's does, with the message on the standard error.
func (p *parser) builtin(ctx object.Context, name string, hasValue bool, shown string) (handled bool, err error) {
	switch {
	case (name == "help" || name == "h") && !p.hasHelp:
		if hasValue {
			return true, fail(needlessArg, "needless argument", shown)
		}
		if err := ctx.Write(false, p.help(ctx)); err != nil {
			return true, err
		}
	case (name == "version" || name == "v") && !p.hasVersion:
		if p.version.IsZero() || p.version.IsNil() {
			if err := ctx.Write(true, p.program+": version unknown\n"); err != nil {
				return true, err
			}
			return true, &object.Exception{Cls: object.SystemExit, Message: "exit", Attrs: map[string]object.Value{"status": object.Int(1)}}
		}
		if err := ctx.Write(false, p.ver(ctx).Str()+"\n"); err != nil {
			return true, err
		}
	default:
		return false, nil
	}
	return true, &object.Exception{Cls: object.SystemExit, Message: "exit", Attrs: map[string]object.Value{"status": object.Int(0)}}
}

// Install defines OptionParser. lib.Install calls it once.
func Install() {
	c := class
	object.DefineNative("OptionParser", object.ClassValue(c))
	c.Consts["Version"] = object.NewString("0.6.0", str.ASCII)
	for _, e := range []*object.Class{parseError, invalidOpt, missingArg, invalidArg, needlessArg, ambiguousOpt, ambiguousArg} {
		e.Lexical = c
		c.Consts[e.Name] = object.ClassValue(e)
	}
	for _, name := range []string{"args", "reason"} {
		name := name
		parseError.Define(name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if v, ok := self.Exception().Attr(name); ok {
				return v, nil
			}
			return object.Nil, nil
		})
	}
	c.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 3 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "0..3")
		}
		p := &parser{banner: object.Nil, version: object.Nil, release: object.Nil, width: 32, indent: "    ", program: programName(ctx)}
		if len(args) >= 1 {
			p.banner = args[0]
		}
		if len(args) >= 2 && !args[1].IsNil() {
			n, err := stdlib.WantInt(ctx, args[1], "")
			if err != nil {
				return object.Value{}, err
			}
			p.width = int(n)
		}
		if len(args) == 3 && !args[2].IsNil() {
			s, err := stdlib.WantString(ctx, args[2])
			if err != nil {
				return object.Value{}, err
			}
			p.indent = s
		}
		v := object.InstanceValue(&object.Instance{Cls: c, Message: p})
		if !block.IsZero() {
			if _, err := ctx.Yield(block, v); err != nil {
				return object.Value{}, err
			}
		}
		return v, nil
	})
	def := func(name string, arity int, f func(ctx object.Context, p *parser, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error)) {
		c.Define(name, arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return f(ctx, parserOf(self), self, args, kw, block)
		})
	}
	on := func(where int) func(ctx object.Context, p *parser, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return func(ctx object.Context, p *parser, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			s, err := p.define(ctx, args, block)
			if err != nil {
				return object.Value{}, err
			}
			for _, l := range s.longs {
				if l == "help" {
					p.hasHelp = true
				}
				if l == "version" {
					p.hasVersion = true
				}
			}
			for _, sh := range s.shorts {
				if sh == "h" {
					p.hasHelp = true
				}
			}
			p.items = append(p.items, item{sw: s, isHead: where < 0, isTail: where > 0})
			return self, nil
		}
	}
	def("on", -1, on(0))
	def("define", -1, on(0))
	def("on_tail", -1, on(1))
	def("define_tail", -1, on(1))
	def("on_head", -1, on(-1))
	def("define_head", -1, on(-1))
	def("separator", 1, func(ctx object.Context, p *parser, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := ctx.ToS(args[0])
		if err != nil {
			return object.Value{}, err
		}
		p.items = append(p.items, item{sep: s, isSep: true})
		return self, nil
	})
	getset := func(name string, get func(p *parser) object.Value, set func(ctx object.Context, p *parser, v object.Value) error) {
		def(name, 0, func(ctx object.Context, p *parser, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return get(p), nil
		})
		def(name+"=", 1, func(ctx object.Context, p *parser, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return self, set(ctx, p, args[0])
		})
	}
	getset("banner", func(p *parser) object.Value {
		if p.banner.IsNil() {
			return object.NewString("Usage: "+p.program+" [options]", str.UTF8)
		}
		return p.banner
	}, func(ctx object.Context, p *parser, v object.Value) error { p.banner = v; return nil })
	getset("program_name", func(p *parser) object.Value { return object.NewString(p.program, str.UTF8) }, func(ctx object.Context, p *parser, v object.Value) error {
		s, err := ctx.ToS(v)
		p.program = s
		return err
	})
	getset("version", func(p *parser) object.Value { return p.version }, func(ctx object.Context, p *parser, v object.Value) error { p.version = v; return nil })
	getset("release", func(p *parser) object.Value { return p.release }, func(ctx object.Context, p *parser, v object.Value) error { p.release = v; return nil })
	getset("summary_width", func(p *parser) object.Value { return object.Int(p.width) }, func(ctx object.Context, p *parser, v object.Value) error {
		n, err := stdlib.WantInt(ctx, v, "")
		p.width = int(n)
		return err
	})
	getset("summary_indent", func(p *parser) object.Value { return object.NewString(p.indent, str.UTF8) }, func(ctx object.Context, p *parser, v object.Value) error {
		s, err := stdlib.WantString(ctx, v)
		p.indent = s
		return err
	})
	help := func(ctx object.Context, p *parser, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewString(p.help(ctx), str.UTF8), nil
	}
	def("help", 0, help)
	def("to_s", 0, help)
	def("summarize", -1, func(ctx object.Context, p *parser, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		lines := p.summarize()
		out := make([]object.Value, len(lines))
		for i, l := range lines {
			out[i] = object.NewString(strings.TrimRight(l, " ")+"\n", str.UTF8)
		}
		return object.NewList(out...), nil
	})
	def("to_a", 0, func(ctx object.Context, p *parser, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		out := []object.Value{object.NewString(p.bannerText(ctx), str.UTF8)}
		for _, l := range p.summarize() {
			out = append(out, object.NewString(strings.TrimRight(l, " ")+"\n", str.UTF8))
		}
		return object.NewList(out...), nil
	})
	def("ver", 0, func(ctx object.Context, p *parser, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return p.ver(ctx), nil
	})
	parseWith := func(order bool) func(ctx object.Context, p *parser, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return func(ctx object.Context, p *parser, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if kw != nil {
				if _, ok := kw.Get(object.Symbol("into")); ok {
					return object.Value{}, ctx.Raise(object.ArgumentError, "into: fills a Hash in place, which a value cannot be; read options after the parse, or use getopts, which returns the Hash")
				}
			}
			var argv []object.Value
			switch len(args) {
			case 0:
				v, ok := ctx.Const("ARGV")
				if ok {
					argv = v.List().Elems
				}
			case 1:
				// One Array is the arguments; one String is the one argument.
				if list, ok := args[0].AsList(); ok {
					argv = list.Elems
				} else {
					argv = args
				}
			default:
				argv = args
			}
			p.options = object.NewMap()
			rest, err := p.parse(ctx, argv, order, nil)
			if err != nil {
				return object.Value{}, err
			}
			return object.NewList(rest...), nil
		}
	}
	def("options", 0, func(ctx object.Context, p *parser, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if p.options == nil {
			return object.MapValue(object.NewMap()), nil
		}
		return object.MapValue(p.options), nil
	})
	for _, name := range []string{"parse", "parse!", "permute", "permute!"} {
		def(name, -1, parseWith(false))
	}
	for _, name := range []string{"order", "order!"} {
		def(name, -1, parseWith(true))
	}
	def("getopts", -1, func(ctx object.Context, p *parser, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		// getopts(argv = ARGV, "ab:", "name:", "flag"): the first spec names
		// the short options, a colon after one means an argument; the others
		// are long options. A flag defaults to false, an argument to nil.
		var argv []object.Value
		if len(args) > 0 {
			if list, ok := args[0].AsList(); ok {
				argv, args = list.Elems, args[1:]
			}
		}
		if argv == nil {
			if v, ok := ctx.Const("ARGV"); ok {
				argv = v.List().Elems
			}
		}
		result := object.NewMap()
		q := &parser{banner: object.Nil, version: object.Nil, release: object.Nil, width: p.width, indent: p.indent, program: p.program}
		keys := map[*sw]string{}
		for i, spec := range args {
			text, err := stdlib.WantString(ctx, spec)
			if err != nil {
				return object.Value{}, err
			}
			if i == 0 {
				for j := 0; j < len(text); j++ {
					if text[j] == ':' {
						continue
					}
					name := text[j : j+1]
					s := &sw{shorts: []string{name}}
					if j+1 < len(text) && text[j+1] == ':' {
						s.kind, s.arg = required, "VAL"
						result = result.Put(object.NewString(name, str.UTF8), object.Nil)
					} else {
						result = result.Put(object.NewString(name, str.UTF8), object.False)
					}
					keys[s] = name
					q.items = append(q.items, item{sw: s})
				}
				continue
			}
			name, isArg := strings.CutSuffix(text, ":")
			s := &sw{longs: []string{name}}
			if isArg {
				s.kind, s.arg = required, "VAL"
				result = result.Put(object.NewString(name, str.UTF8), object.Nil)
			} else {
				result = result.Put(object.NewString(name, str.UTF8), object.False)
			}
			keys[s] = name
			q.items = append(q.items, item{sw: s})
		}
		q.items = append(q.items, p.items...)
		if _, err := q.parse(ctx, argv, false, func(s *sw, v object.Value) {
			if k, ok := keys[s]; ok {
				result = result.Put(object.NewString(k, str.UTF8), v)
			}
		}); err != nil {
			return object.Value{}, err
		}
		return object.MapValue(result), nil
	})
	def("inspect", 0, func(ctx object.Context, p *parser, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewString("#<OptionParser "+p.bannerText(ctx)+">", str.UTF8), nil
	})
}
