package stdlib

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/object"
)

// JSON follows Ruby 3.4's json library, version 2.9.1: its output byte for
// byte, its parser's grammar, and the messages of its errors, which name
// the input from where its parser gave up. Ruby's generator passes a State
// to a to_json a class defines; cameo passes a Hash of the same options,
// depth included, which Ruby's to_json methods accept as well.
//
// An instance of a cameo class without its own to_json is a JSON object of
// its fields, where Ruby writes the string of to_s; see
// doc/cameo-vs-ruby.md.

var (
	jsonModule       = object.NewModule("JSON")
	jsonError        = object.NewClass("JSONError", object.StandardError)
	jsonParserError  = object.NewClass("ParserError", jsonError)
	jsonNestingError = object.NewClass("NestingError", jsonParserError)
	jsonGeneratorErr = object.NewClass("GeneratorError", jsonError)
	// jsonToJSON is the builtin to_json, which the generator does not call.
	jsonToJSON *object.Method
)

func installJSON() {
	m := jsonModule
	object.ObjectClass.Consts["JSON"] = object.ClassValue(m)
	m.Consts["VERSION"] = object.String("2.9.1")
	for _, c := range []*object.Class{jsonError, jsonParserError, jsonNestingError, jsonGeneratorErr} {
		c.Lexical = m
		m.Consts[c.Name] = object.ClassValue(c)
	}

	m.DefineSingleton("parse", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return jsonParseCall(ctx, args, kw, jsonParser{maxNesting: 100})
	})
	m.DefineSingleton("parse!", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return jsonParseCall(ctx, args, kw, jsonParser{allowNaN: true})
	})
	m.DefineSingleton("load", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if s, ok := args[0].AsString(); args[0].IsNil() || ok && s == "" {
			return object.Nil, nil
		}
		return jsonParseCall(ctx, args, nil, jsonParser{allowNaN: true})
	})
	m.DefineSingleton("generate", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return jsonGenerateCall(ctx, args, kw, jsonGenerator{maxNesting: 100})
	})
	m.DefineSingleton("pretty_generate", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return jsonGenerateCall(ctx, args, kw, jsonGenerator{indent: "  ", space: " ", objectNL: "\n", arrayNL: "\n", maxNesting: 100})
	})
	// JSON.dump(obj, io = nil, limit = nil) writes to io when it has a write
	// method; an Integer in io's place is the limit.
	m.DefineSingleton("dump", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 || len(args) > 3 {
			return object.Value{}, argError(ctx, len(args), "1..3")
		}
		g := jsonGenerator{allowNaN: true}
		io, limit := object.Nil, object.Nil
		if len(args) > 1 {
			io = args[1]
		}
		if len(args) > 2 {
			limit = args[2]
		}
		if !io.IsNil() && limit.IsNil() && !ctx.RespondsTo(io, "write") {
			io, limit = object.Nil, io
		}
		if !limit.IsNil() {
			n, err := wantInt(ctx, limit, "")
			if err != nil {
				return object.Value{}, err
			}
			g.maxNesting = int(n)
		}
		out, err := jsonGenerateCall(ctx, args[:1], nil, g)
		if exc, ok := err.(*object.Exception); ok && exc.Cls == jsonNestingError {
			return object.Value{}, ctx.Raise(object.ArgumentError, "exceed depth limit")
		}
		if err != nil || io.IsNil() {
			return out, err
		}
		if _, err := ctx.Call(io, "write", out); err != nil {
			return object.Value{}, err
		}
		return io, nil
	})
	// JSON[x] and JSON(x) parse a String and generate anything else.
	either := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 {
			return object.Value{}, argError(ctx, 0, "1..2")
		}
		if args[0].IsString() {
			return jsonParseCall(ctx, args, kw, jsonParser{maxNesting: 100})
		}
		return jsonGenerateCall(ctx, args, kw, jsonGenerator{maxNesting: 100})
	}
	m.DefineSingleton("[]", -1, either)
	def(object.ObjectClass, "JSON", -1, either)

	// One to_json on Object serves every class; a class that defines its
	// own is called from the generator instead.
	def(object.ObjectClass, "to_json", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return jsonGenerateCall(ctx, append([]object.Value{self}, args...), kw, jsonGenerator{maxNesting: 100})
	})
	jsonToJSON = object.ObjectClass.Methods["to_json"]
}

// jsonOptions reads the options of a call: a Hash after the positional
// arguments, keywords, or both.
func jsonOptions(args []object.Value, kw *object.Map) []*object.Map {
	var opts []*object.Map
	for _, a := range args {
		if a.Kind() == object.KindMap {
			opts = append(opts, a.Map())
		}
	}
	if kw != nil {
		opts = append(opts, kw)
	}
	return opts
}

func jsonOption(opts []*object.Map, name string) (object.Value, bool) {
	var v object.Value
	found := false
	for _, o := range opts {
		if got, ok := o.Get(object.Symbol(name)); ok {
			v, found = got, true
		} else if got, ok := o.Get(object.String(name)); ok {
			v, found = got, true
		}
	}
	return v, found
}

// jsonNesting reads a max_nesting option: false, nil or 0 turn the check
// off.
func jsonNesting(ctx object.Context, v object.Value) (int, error) {
	if !v.Truthy() {
		return 0, nil
	}
	n, err := wantInt(ctx, v, "")
	return int(n), err
}

func jsonParseCall(ctx object.Context, args []object.Value, kw *object.Map, p jsonParser) (object.Value, error) {
	if len(args) == 0 || len(args) > 2 {
		return object.Value{}, argError(ctx, len(args), "1..2")
	}
	if !args[0].IsString() {
		return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into String", kindName(args[0]))
	}
	opts := jsonOptions(args[1:], kw)
	if v, ok := jsonOption(opts, "symbolize_names"); ok {
		p.symbolize = v.Truthy()
	}
	if v, ok := jsonOption(opts, "allow_nan"); ok {
		p.allowNaN = v.Truthy()
	}
	if v, ok := jsonOption(opts, "max_nesting"); ok {
		n, err := jsonNesting(ctx, v)
		if err != nil {
			return object.Value{}, err
		}
		p.maxNesting = n
	}
	p.ctx, p.src = ctx, args[0].Str()
	return p.parse()
}

func jsonGenerateCall(ctx object.Context, args []object.Value, kw *object.Map, g jsonGenerator) (object.Value, error) {
	if len(args) == 0 || len(args) > 2 {
		return object.Value{}, argError(ctx, len(args), "1..2")
	}
	opts := jsonOptions(args[1:], kw)
	for name, field := range map[string]*string{"indent": &g.indent, "space": &g.space, "space_before": &g.spaceBefore, "object_nl": &g.objectNL, "array_nl": &g.arrayNL} {
		if v, ok := jsonOption(opts, name); ok && !v.IsNil() {
			s, err := wantString(ctx, v)
			if err != nil {
				return object.Value{}, err
			}
			*field = s
		}
	}
	for name, field := range map[string]*bool{"allow_nan": &g.allowNaN, "ascii_only": &g.asciiOnly, "script_safe": &g.scriptSafe, "escape_slash": &g.scriptSafe} {
		if v, ok := jsonOption(opts, name); ok {
			*field = v.Truthy()
		}
	}
	if v, ok := jsonOption(opts, "max_nesting"); ok {
		n, err := jsonNesting(ctx, v)
		if err != nil {
			return object.Value{}, err
		}
		g.maxNesting = n
	}
	depth := 0
	if v, ok := jsonOption(opts, "depth"); ok && !v.IsNil() {
		n, err := wantInt(ctx, v, "")
		if err != nil {
			return object.Value{}, err
		}
		depth = int(n)
	}
	g.ctx = ctx
	if err := g.value(args[0], depth); err != nil {
		return object.Value{}, err
	}
	return object.String(g.b.String()), nil
}

// jsonGenerator writes a value as JSON, with the options of Ruby's
// JSON::State.
type jsonGenerator struct {
	ctx                                           object.Context
	b                                             strings.Builder
	indent, space, spaceBefore, objectNL, arrayNL string
	allowNaN, asciiOnly, scriptSafe               bool
	maxNesting                                    int
}

// state is the generator's options at depth as a Hash, what a to_json a
// class defines receives.
func (g *jsonGenerator) state(depth int) object.Value {
	return object.NewMapBuilder().
		Set(object.Symbol("indent"), object.String(g.indent)).
		Set(object.Symbol("space"), object.String(g.space)).
		Set(object.Symbol("space_before"), object.String(g.spaceBefore)).
		Set(object.Symbol("object_nl"), object.String(g.objectNL)).
		Set(object.Symbol("array_nl"), object.String(g.arrayNL)).
		Set(object.Symbol("allow_nan"), object.Bool(g.allowNaN)).
		Set(object.Symbol("ascii_only"), object.Bool(g.asciiOnly)).
		Set(object.Symbol("script_safe"), object.Bool(g.scriptSafe)).
		Set(object.Symbol("max_nesting"), object.Int(g.maxNesting)).
		Set(object.Symbol("depth"), object.Int(depth)).
		Value()
}

func (g *jsonGenerator) value(v object.Value, depth int) error {
	if m := v.Class().Lookup("to_json"); m != nil && m != jsonToJSON {
		out, err := g.ctx.Call(v, "to_json", g.state(depth))
		if err != nil {
			return err
		}
		if !out.IsString() {
			return g.ctx.Raise(object.TypeError, "wrong argument type %s (expected String)", kindName(out))
		}
		g.b.WriteString(out.Str())
		return nil
	}
	switch v.Kind() {
	case object.KindNone, object.KindNil:
		g.b.WriteString("null")
	case object.KindBool:
		g.b.WriteString(strconv.FormatBool(v.Truthy()))
	case object.KindInt:
		g.b.WriteString(strconv.FormatInt(v.Int(), 10))
	case object.KindBig:
		g.b.WriteString(v.Big().String())
	case object.KindFloat:
		return g.float(v.Float())
	case object.KindString, object.KindBytes, object.KindASCII, object.KindSymbol:
		return g.string(v)
	case object.KindList:
		return g.list(v.List().Elems, depth)
	case object.KindMap:
		m := v.Map()
		return g.object(m.Keys(), m.Values(), depth)
	case object.KindInstance:
		inst := v.Instance()
		keys := make([]object.Value, len(inst.Fields))
		for i, f := range inst.Cls.Fields {
			keys[i] = object.String(f.Name)
		}
		return g.object(keys, inst.Fields, depth)
	default:
		s, err := g.ctx.ToS(v)
		if err != nil {
			return err
		}
		return g.string(object.String(s))
	}
	return nil
}

func (g *jsonGenerator) float(f float64) error {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		if !g.allowNaN {
			return g.ctx.Raise(jsonGeneratorErr, "%s not allowed in JSON", object.FormatFloat(f))
		}
	}
	g.b.WriteString(object.FormatFloat(f))
	return nil
}

// nest checks the depth an array or object opens at; Ruby's message names
// the depth outside it.
func (g *jsonGenerator) nest(depth int) error {
	if g.maxNesting > 0 && depth > g.maxNesting {
		return g.ctx.Raise(jsonNestingError, "nesting of %d is too deep", depth-1)
	}
	return nil
}

// close ends an array or object: the closing newline and, after one, the
// indent of the depth outside it.
func (g *jsonGenerator) close(nl string, depth int, bracket string) {
	if nl != "" {
		g.b.WriteString(nl + strings.Repeat(g.indent, depth-1))
	}
	g.b.WriteString(bracket)
}

func (g *jsonGenerator) list(elems []object.Value, depth int) error {
	depth++
	if err := g.nest(depth); err != nil {
		return err
	}
	if len(elems) == 0 {
		g.b.WriteString("[]")
		return nil
	}
	g.b.WriteString("[" + g.arrayNL)
	for i, el := range elems {
		if i > 0 {
			g.b.WriteString("," + g.arrayNL)
		}
		g.b.WriteString(strings.Repeat(g.indent, depth))
		if err := g.value(el, depth); err != nil {
			return err
		}
	}
	g.close(g.arrayNL, depth, "]")
	return nil
}

func (g *jsonGenerator) object(keys, values []object.Value, depth int) error {
	depth++
	if err := g.nest(depth); err != nil {
		return err
	}
	if len(keys) == 0 {
		g.b.WriteString("{}")
		return nil
	}
	g.b.WriteString("{" + g.objectNL)
	for i, k := range keys {
		if i > 0 {
			g.b.WriteString("," + g.objectNL)
		}
		g.b.WriteString(strings.Repeat(g.indent, depth))
		if !k.IsString() && k.Kind() != object.KindSymbol {
			s, err := g.ctx.ToS(k)
			if err != nil {
				return err
			}
			k = object.String(s)
		}
		if err := g.string(k); err != nil {
			return err
		}
		g.b.WriteString(g.spaceBefore + ":" + g.space)
		if err := g.value(values[i], depth); err != nil {
			return err
		}
	}
	g.close(g.objectNL, depth, "}")
	return nil
}

// string writes a String or a Symbol's name as a JSON string. It must be
// UTF-8: a binary String is read as UTF-8, as Ruby's json 2.9 does.
func (g *jsonGenerator) string(v object.Value) error {
	s := v.Str()
	if !utf8.ValidString(s) {
		if v.Kind() == object.KindBytes {
			bad := s
			for i := 0; i < len(s); {
				r, size := utf8.DecodeRuneInString(s[i:])
				if r == utf8.RuneError && size == 1 {
					bad = s[i : i+1]
					break
				}
				i += size
			}
			return g.ctx.Raise(jsonGeneratorErr, "%s from ASCII-8BIT to UTF-8", object.QuoteString(bad))
		}
		return g.ctx.Raise(jsonGeneratorErr, "source sequence is illegal/malformed utf-8")
	}
	g.b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			g.b.WriteString(`\"`)
		case r == '\\':
			g.b.WriteString(`\\`)
		case r == '\b':
			g.b.WriteString(`\b`)
		case r == '\f':
			g.b.WriteString(`\f`)
		case r == '\n':
			g.b.WriteString(`\n`)
		case r == '\r':
			g.b.WriteString(`\r`)
		case r == '\t':
			g.b.WriteString(`\t`)
		case r < 0x20:
			fmt.Fprintf(&g.b, `\u%04x`, r)
		case r == '/' && g.scriptSafe:
			g.b.WriteString(`\/`)
		case (r == 0x2028 || r == 0x2029) && g.scriptSafe, r >= 0x80 && g.asciiOnly && r < 0x10000:
			fmt.Fprintf(&g.b, `\u%04x`, r)
		case r >= 0x10000 && g.asciiOnly:
			r -= 0x10000
			fmt.Fprintf(&g.b, `\u%04x\u%04x`, 0xd800+(r>>10), 0xdc00+(r&0x3ff))
		default:
			g.b.WriteRune(r)
		}
	}
	g.b.WriteByte('"')
	return nil
}

// jsonParser reads JSON as Ruby's parser does. A string, number, literal
// or object it cannot read is a failure the caller reports at the start of
// that value; an array, a NaN that is not allowed and a broken surrogate
// pair raise where they are, so a ParserError names the same place as
// Ruby's.
type jsonParser struct {
	ctx                 object.Context
	src                 string
	symbolize, allowNaN bool
	maxNesting          int
}

// jsonFailed is a value the parser could not read, to be reported by the
// caller.
type jsonFailed struct{}

func (jsonFailed) Error() string { return "json: value not read" }

func (p *jsonParser) unexpected(at int) error {
	return p.ctx.Raise(jsonParserError, "unexpected token at '%s'", jsonExcerpt(p.src[at:]))
}

// jsonExcerpt is the part of the input a ParserError quotes: at most 32
// bytes, and nothing from a NUL byte on, since Ruby's message is a C
// string.
func jsonExcerpt(s string) string {
	if len(s) > 32 {
		s = s[:32]
	}
	if i := strings.IndexByte(s, 0); i >= 0 {
		s = s[:i]
	}
	return s
}

func (p *jsonParser) parse() (object.Value, error) {
	i, bad := p.space(0)
	if bad >= 0 || i == len(p.src) {
		return object.Value{}, p.unexpected(max(bad, i))
	}
	v, end, err := p.value(i, 0)
	if _, failed := err.(jsonFailed); failed {
		return object.Value{}, p.unexpected(i)
	}
	if err != nil {
		return object.Value{}, err
	}
	end, bad = p.space(end)
	if bad >= 0 {
		return object.Value{}, p.unexpected(bad)
	}
	if end < len(p.src) {
		return object.Value{}, p.unexpected(end)
	}
	return v, nil
}

// space skips whitespace and comments from i. It returns where they end,
// and the index of a broken comment, or -1.
func (p *jsonParser) space(i int) (int, int) {
	s := p.src
	for i < len(s) {
		switch s[i] {
		case ' ', '\t', '\n', '\r':
			i++
		case '/':
			switch {
			case i+1 == len(s):
				return i, len(s)
			case s[i+1] == '*':
				end := strings.Index(s[i+2:], "*/")
				if end < 0 {
					return i, len(s)
				}
				i += 2 + end + 2
			case s[i+1] == '/':
				end := strings.IndexByte(s[i+2:], '\n')
				if end < 0 {
					return i, len(s)
				}
				i += 2 + end + 1
			default:
				return i, i + 1
			}
		default:
			return i, -1
		}
	}
	return i, -1
}

// value reads the value at s[i], which is not whitespace, at a nesting of
// depth, and returns it and where it ends.
func (p *jsonParser) value(i, depth int) (object.Value, int, error) {
	s := p.src[i:]
	switch {
	case s[0] == '"':
		return p.string(i)
	case s[0] == '[':
		return p.array(i, depth+1)
	case s[0] == '{':
		return p.object(i, depth+1)
	case strings.HasPrefix(s, "null"):
		return object.Nil, i + 4, nil
	case strings.HasPrefix(s, "true"):
		return object.True, i + 4, nil
	case strings.HasPrefix(s, "false"):
		return object.False, i + 5, nil
	}
	for _, lit := range []string{"NaN", "Infinity", "-Infinity"} {
		if strings.HasPrefix(s, lit) {
			if !p.allowNaN {
				return object.Value{}, 0, p.unexpected(i)
			}
			f, _ := strconv.ParseFloat(lit, 64)
			return object.Float(f), i + len(lit), nil
		}
	}
	return p.number(i)
}

// number reads -?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?, not
// followed by a digit: an Integer, or a Float with a fraction or an
// exponent.
func (p *jsonParser) number(i int) (object.Value, int, error) {
	s := p.src
	j := i
	digits := func() int {
		start := j
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		return j - start
	}
	if j < len(s) && s[j] == '-' {
		j++
	}
	if j < len(s) && s[j] == '0' {
		j++
	} else if digits() == 0 {
		return object.Value{}, 0, jsonFailed{}
	}
	float := false
	if j < len(s) && s[j] == '.' {
		j++
		float = true
		if digits() == 0 {
			return object.Value{}, 0, jsonFailed{}
		}
	}
	if j < len(s) && (s[j] == 'e' || s[j] == 'E') {
		j++
		float = true
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if digits() == 0 {
			return object.Value{}, 0, jsonFailed{}
		}
	}
	if j < len(s) && s[j] >= '0' && s[j] <= '9' {
		return object.Value{}, 0, jsonFailed{}
	}
	text := s[i:j]
	if float {
		f, _ := strconv.ParseFloat(text, 64)
		return object.Float(f), j, nil
	}
	if n, err := strconv.ParseInt(text, 10, 64); err == nil {
		return object.Int(n), j, nil
	}
	b, _ := new(big.Int).SetString(text, 10)
	return object.BigInt(b), j, nil
}

// string reads the string that opens at s[i]. An escape of a character
// JSON does not name is the character; a lone low surrogate is written as
// its three bytes, and a high one must be followed by another \u escape.
func (p *jsonParser) string(i int) (object.Value, int, error) {
	s := p.src
	var b strings.Builder
	for j := i + 1; j < len(s); {
		c := s[j]
		switch {
		case c == '"':
			return object.String(b.String()), j + 1, nil
		case c < 0x20:
			return object.Value{}, 0, jsonFailed{}
		case c != '\\':
			b.WriteByte(c)
			j++
			continue
		}
		if j+1 == len(s) || s[j+1] < 0x20 {
			return object.Value{}, 0, jsonFailed{}
		}
		if esc := s[j+1]; esc != 'u' {
			b.WriteByte(jsonUnescape(esc))
			j += 2
			continue
		}
		r, ok := jsonHex4(s, j+2)
		if !ok {
			return object.Value{}, 0, jsonFailed{}
		}
		if r >= 0xd800 && r <= 0xdbff {
			if !strings.HasPrefix(s[j+6:], `\u`) {
				return object.Value{}, 0, p.ctx.Raise(jsonParserError, "incomplete surrogate pair at '%s'", jsonExcerpt(s[j:]))
			}
			lo, ok := jsonHex4(s, j+8)
			if !ok {
				return object.Value{}, 0, jsonFailed{}
			}
			b.WriteString(encodeCodepoint((r&0x3ff)<<10 | lo&0x3ff + 0x10000))
			j += 12
			continue
		}
		b.WriteString(encodeCodepoint(r))
		j += 6
	}
	return object.Value{}, 0, jsonFailed{}
}

// jsonUnescape is the byte a backslash escape other than \u stands for:
// the control characters JSON names, and any other byte itself.
func jsonUnescape(esc byte) byte {
	switch esc {
	case 'b':
		return '\b'
	case 'f':
		return '\f'
	case 'n':
		return '\n'
	case 'r':
		return '\r'
	case 't':
		return '\t'
	}
	return esc
}

func jsonHex4(s string, i int) (rune, bool) {
	if i+4 > len(s) {
		return 0, false
	}
	n, err := strconv.ParseUint(s[i:i+4], 16, 32)
	if err != nil {
		return 0, false
	}
	return rune(n), true
}

func (p *jsonParser) nest(depth int) error {
	if p.maxNesting > 0 && depth > p.maxNesting {
		return p.ctx.Raise(jsonNestingError, "nesting of %d is too deep", depth)
	}
	return nil
}

// array reads the array that opens at s[i], raising where it breaks.
func (p *jsonParser) array(i, depth int) (object.Value, int, error) {
	if err := p.nest(depth); err != nil {
		return object.Value{}, 0, err
	}
	var elems []object.Value
	j, bad := p.space(i + 1)
	if bad >= 0 {
		return object.Value{}, 0, p.unexpected(bad)
	}
	if j < len(p.src) && p.src[j] == ']' {
		return object.NewList(), j + 1, nil
	}
	for {
		if j == len(p.src) {
			return object.Value{}, 0, p.unexpected(j)
		}
		v, end, err := p.value(j, depth)
		if _, failed := err.(jsonFailed); failed {
			return object.Value{}, 0, p.unexpected(j)
		}
		if err != nil {
			return object.Value{}, 0, err
		}
		elems = append(elems, v)
		if j, bad = p.space(end); bad >= 0 {
			return object.Value{}, 0, p.unexpected(bad)
		}
		switch {
		case j < len(p.src) && p.src[j] == ']':
			return object.NewList(elems...), j + 1, nil
		case j < len(p.src) && p.src[j] == ',':
			if j, bad = p.space(j + 1); bad >= 0 {
				return object.Value{}, 0, p.unexpected(bad)
			}
		default:
			return object.Value{}, 0, p.unexpected(j)
		}
	}
}

// object reads the object that opens at s[i]; anything wrong inside it is
// a failure of the whole object, but for what raises on its own.
func (p *jsonParser) object(i, depth int) (object.Value, int, error) {
	if err := p.nest(depth); err != nil {
		return object.Value{}, 0, err
	}
	out := object.NewMapBuilder()
	j, bad := p.space(i + 1)
	if bad >= 0 {
		return object.Value{}, 0, jsonFailed{}
	}
	if j < len(p.src) && p.src[j] == '}' {
		return out.Value(), j + 1, nil
	}
	for {
		if j == len(p.src) || p.src[j] != '"' {
			return object.Value{}, 0, jsonFailed{}
		}
		key, end, err := p.string(j)
		if err != nil {
			return object.Value{}, 0, err
		}
		if p.symbolize {
			key = object.Symbol(key.Str())
		}
		if j, bad = p.space(end); bad >= 0 || j == len(p.src) || p.src[j] != ':' {
			return object.Value{}, 0, jsonFailed{}
		}
		if j, bad = p.space(j + 1); bad >= 0 || j == len(p.src) {
			return object.Value{}, 0, jsonFailed{}
		}
		v, end, err := p.value(j, depth)
		if err != nil {
			return object.Value{}, 0, err
		}
		out.Set(key, v)
		if j, bad = p.space(end); bad >= 0 || j == len(p.src) {
			return object.Value{}, 0, jsonFailed{}
		}
		switch p.src[j] {
		case '}':
			return out.Value(), j + 1, nil
		case ',':
			if j, bad = p.space(j + 1); bad >= 0 {
				return object.Value{}, 0, jsonFailed{}
			}
		default:
			return object.Value{}, 0, jsonFailed{}
		}
	}
}
