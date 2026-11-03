package yaml

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/lib/yaml/libyaml"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

// This file is Psych's dumping side, Psych::Visitors::YAMLTree and its
// restricted form: a value becomes emitter events, with the tags, implicit
// flags and style hints Psych gives, and the emitter decides the rest.

// dumper walks values into the emitter.
type dumper struct {
	ctx object.Context
	em  *libyaml.Emitter
	// lineWidth is the option, -1 when none: a String longer than it is
	// folded.
	lineWidth int
	version   *[2]int
	// restricted is RestrictedYAMLTree: only permitted classes and
	// symbols are dumped.
	restricted bool
	permitted  map[*object.Class]bool
	symbols    map[string]bool
}

// Options are what dump, safe_dump and to_yaml take.
type options struct {
	indent, width int
	hasWidth      bool
	canonical     bool
	header        bool
	version       *[2]int
	classes       []object.Value
	symbols       []object.Value
}

// newDumper makes a dumper with its emitter, as Psych.dump sets them up.
func newDumper(ctx object.Context, o options, restricted bool) (*dumper, error) {
	width := 0
	if o.hasWidth {
		width = o.width
	}
	d := &dumper{ctx: ctx, em: libyaml.NewEmitter(o.indent, width, o.canonical), lineWidth: -1, restricted: restricted}
	if o.hasWidth {
		switch {
		case o.width == -1:
			// Unlimited, as libyaml reads it.
		case o.width < -1:
			return nil, ctx.Raise(object.ArgumentError, "Invalid line_width %d, must be non-negative or -1 for unlimited.", o.width)
		default:
			d.lineWidth = o.width
		}
	}
	if o.header {
		d.version = &[2]int{1, 1}
	}
	if o.version != nil {
		d.version = o.version
	}
	if restricted {
		d.permitted = map[*object.Class]bool{
			object.BoolClass: true, object.NilClass: true,
			object.IntegerClass: true, object.FloatClass: true, object.StringClass: true,
			object.ListClass: true, object.MapClass: true,
		}
		for _, c := range o.classes {
			if cls, ok := c.AsClass(); ok {
				d.permitted[cls] = true
			}
		}
		d.symbols = map[string]bool{}
		for _, s := range o.symbols {
			if name, ok := s.AsSymbol(); ok {
				d.symbols[name] = true
			}
		}
	}
	if err := d.em.Emit(libyaml.Event{Type: libyaml.StreamStart}); err != nil {
		return nil, err
	}
	return d, nil
}

// push dumps one document.
func (d *dumper) push(v object.Value) error {
	if err := d.em.Emit(libyaml.Event{Type: libyaml.DocumentStart, Version: d.version}); err != nil {
		return err
	}
	if err := d.accept(v); err != nil {
		return err
	}
	return d.em.Emit(libyaml.Event{Type: libyaml.DocumentEnd, Implicit: true})
}

// finish ends the stream and returns what was written.
func (d *dumper) finish() (string, error) {
	if err := d.em.Emit(libyaml.Event{Type: libyaml.StreamEnd}); err != nil {
		return "", err
	}
	return d.em.String(), nil
}

func (d *dumper) scalar(value, tag string, plain, quoted bool, style libyaml.Style) error {
	return d.em.Emit(libyaml.Event{Type: libyaml.Scalar, Value: value, Tag: tag, Implicit: plain, QuotedImplicit: quoted, Style: style})
}

func (d *dumper) startMapping(tag string, implicit bool) error {
	return d.em.Emit(libyaml.Event{Type: libyaml.MappingStart, Tag: tag, Implicit: implicit, Style: libyaml.Block})
}

func (d *dumper) endMapping() error { return d.em.Emit(libyaml.Event{Type: libyaml.MappingEnd}) }

// plainStr emits a key or a word Psych writes as a plain scalar.
func (d *dumper) plainStr(s string) error { return d.scalar(s, "", true, false, libyaml.Any) }

// accept dumps one value.
func (d *dumper) accept(v object.Value) error {
	if d.restricted && v.Kind() != object.KindSymbol && !d.permitted[v.Class()] {
		return disallowed("dump", v.Class().QualifiedName())
	}
	switch v.Kind() {
	case object.KindNone, object.KindNil:
		return d.scalar("", "tag:yaml.org,2002:null", true, false, libyaml.Any)
	case object.KindBool:
		return d.plainStr(strconv.FormatBool(v.Truthy()))
	case object.KindInt:
		return d.plainStr(strconv.FormatInt(v.Int(), 10))
	case object.KindBig:
		return d.plainStr(v.Big().String())
	case object.KindFloat:
		f := v.Float()
		switch {
		case math.IsNaN(f):
			return d.plainStr(".nan")
		case math.IsInf(f, 1):
			return d.plainStr(".inf")
		case math.IsInf(f, -1):
			return d.plainStr("-.inf")
		}
		return d.plainStr(object.FormatFloat(f))
	case object.KindString, object.KindASCII, object.KindBytes:
		return d.string(v)
	case object.KindSymbol:
		name := v.Str()
		if d.restricted && !d.permitted[object.SymbolClass] && !d.symbols[name] {
			inspect, err := d.ctx.Inspect(v)
			if err != nil {
				return err
			}
			return disallowed("dump", "Symbol("+inspect+")")
		}
		if name == "" {
			return d.scalar("", "!ruby/symbol", false, false, libyaml.Any)
		}
		return d.plainStr(":" + name)
	case object.KindList:
		return d.sequence(v.List().Elems, "", true)
	case object.KindEnumerator:
		elems, err := stdlib.ToList(d.ctx, v)
		if err != nil {
			return err
		}
		return d.sequence(elems, "", true)
	case object.KindMap:
		m := v.Map()
		return d.mapping(m.Keys(), m.Values(), "", true)
	case object.KindSet:
		if err := d.startMapping("!ruby/object:Set", false); err != nil {
			return err
		}
		if err := d.plainStr("hash"); err != nil {
			return err
		}
		elems := v.Set().Elems()
		trues := make([]object.Value, len(elems))
		for i := range trues {
			trues[i] = object.True
		}
		if err := d.mapping(elems, trues, "", true); err != nil {
			return err
		}
		return d.endMapping()
	case object.KindRange:
		r := v.Range()
		return d.mapping(
			[]object.Value{object.String("begin"), object.String("end"), object.String("excl")},
			[]object.Value{r.Lo, r.Hi, object.Bool(r.Exclusive)}, "!ruby/range", false)
	case object.KindRegexp:
		return d.scalar(v.Regexp().Inspect(), "!ruby/regexp", false, false, libyaml.Any)
	case object.KindTime:
		t := v.Time()
		return d.plainStr(formatTime(t, t.Location() == time.UTC))
	case object.KindClass:
		cls, _ := v.AsClass()
		tag := "!ruby/class"
		if cls.IsModule {
			tag = "!ruby/module"
		}
		return d.scalar(cls.QualifiedName(), tag, false, false, libyaml.SingleQuoted)
	case object.KindException:
		e := v.Exception()
		if err := d.startMapping("!ruby/exception:"+e.Cls.QualifiedName(), false); err != nil {
			return err
		}
		if err := d.plainStr("message"); err != nil {
			return err
		}
		if err := d.accept(object.String(e.Message)); err != nil {
			return err
		}
		if err := d.plainStr("backtrace"); err != nil {
			return err
		}
		if err := d.accept(object.Nil); err != nil {
			return err
		}
		for _, f := range e.Cls.Fields {
			if fv, ok := e.Attr(f.Name); ok {
				if err := d.plainStr(f.Name); err != nil {
					return err
				}
				if err := d.accept(fv); err != nil {
					return err
				}
			}
		}
		return d.endMapping()
	case object.KindInstance:
		return d.instance(v)
	}
	// A Proc, a Random, a MatchData: Psych writes an object with no
	// instance variables, an empty mapping under the class's tag.
	return d.mapping(nil, nil, "!ruby/object:"+v.Class().QualifiedName(), false)
}

// instance dumps an instance of a cameo class as Psych dumps an object:
// its fields under !ruby/object:Name. A Date is its day, a DateTime
// Psych's tagged timestamp, an Encoding its name.
func (d *dumper) instance(v object.Value) error {
	inst := v.Instance()
	switch name := inst.Cls.QualifiedName(); {
	case name == "Date":
		s, err := d.ctx.ToS(v)
		if err != nil {
			return err
		}
		return d.plainStr(s)
	case name == "DateTime":
		s, err := d.ctx.Call(v, "strftime", object.String("%Y-%m-%d %H:%M:%S.%9N %:z"))
		if err != nil {
			return err
		}
		formatted := s.Str()
		if off, err := d.ctx.Call(v, "offset"); err == nil && off.Kind() != object.KindFloat {
			if i, _ := off.AsInt(); i == 0 {
				formatted = strings.TrimSuffix(formatted, "+00:00") + "Z"
			}
		}
		return d.scalar(formatted, "!ruby/object:DateTime", false, false, libyaml.Any)
	case inst.Cls == object.EncodingClass:
		s, err := d.ctx.ToS(v)
		if err != nil {
			return err
		}
		return d.scalar(s, "!ruby/encoding", false, false, libyaml.Any)
	}
	tag := "!ruby/object"
	if inst.Cls != object.ObjectClass {
		tag += ":" + inst.Cls.QualifiedName()
	}
	// The field names are written plain, as Psych writes instance variable
	// names, not through the String rules.
	if err := d.startMapping(tag, false); err != nil {
		return err
	}
	for i, f := range inst.Cls.Fields {
		if err := d.plainStr(f.Name); err != nil {
			return err
		}
		v := object.Nil
		if i < len(inst.Fields) && !inst.Fields[i].IsZero() {
			v = inst.Fields[i]
		} else if !f.Default.IsZero() {
			v = f.Default
		}
		if err := d.accept(v); err != nil {
			return err
		}
	}
	return d.endMapping()
}

func (d *dumper) sequence(elems []object.Value, tag string, implicit bool) error {
	if err := d.em.Emit(libyaml.Event{Type: libyaml.SequenceStart, Tag: tag, Implicit: implicit, Style: libyaml.Block}); err != nil {
		return err
	}
	for _, e := range elems {
		if err := d.accept(e); err != nil {
			return err
		}
	}
	return d.em.Emit(libyaml.Event{Type: libyaml.SequenceEnd})
}

func (d *dumper) mapping(keys, values []object.Value, tag string, implicit bool) error {
	if err := d.startMapping(tag, implicit); err != nil {
		return err
	}
	for i, k := range keys {
		if err := d.accept(k); err != nil {
			return err
		}
		if err := d.accept(values[i]); err != nil {
			return err
		}
	}
	return d.endMapping()
}

// formatTime is Psych's format_time.
func formatTime(t time.Time, utc bool) string {
	if utc {
		return stdlib.Strftime(t, "%Y-%m-%d %H:%M:%S.%9N Z")
	}
	return stdlib.Strftime(t, "%Y-%m-%d %H:%M:%S.%9N %:z")
}

var (
	// nonWordLine is /^[^[:word:]][^"]*$/ with Ruby's anchors: some line
	// starts with what is not a word character, a newline included, and
	// runs to a line's end without a double quote.
	nonWordLine = regexp.MustCompile(`(?m)^[^` + word + `][^"]*$`)
	// octalish is /\A0[0-7]*[89]/: looks octal, is not.
	octalish = regexp.MustCompile(`\A0[0-7]*[89]`)
)

// string is visit_String: the style Psych asks of the emitter for a
// String, and the emitter decides what the style allows.
func (d *dumper) string(v object.Value) error {
	s := v.Str()
	plain, quote := true, true
	style := libyaml.Plain
	tag := ""
	switch {
	case v.Kind() == object.KindBytes && !asciiOnly(s):
		s = stdBase64(s)
		tag, style, plain, quote = "!binary", libyaml.Literal, false, false
	case innerNewline(s):
		style = libyaml.Literal
	case s == "<<":
		tag, style, plain, quote = "tag:yaml.org,2002:str", libyaml.SingleQuoted, false, false
	case s == "y" || s == "Y" || s == "n" || s == "N":
		style = libyaml.DoubleQuoted
	case d.lineWidth >= 0 && utf8.RuneCountInString(s) > d.lineWidth:
		style = libyaml.Folded
	case nonWordLine.MatchString(s):
		style = libyaml.DoubleQuoted
	case tokenize(s, false).kind != kindString || octalish.MatchString(s):
		style = libyaml.SingleQuoted
	}
	return d.scalar(s, tag, plain, quote, style)
}

// innerNewline is /\n(?!\Z)/: a newline that is not the string's last
// character, nor the one before a last newline.
func innerNewline(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' && !(i+1 == len(s) || i+2 == len(s) && s[len(s)-1] == '\n') {
			return true
		}
	}
	return false
}

func asciiOnly(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// stdBase64 is [s].pack("m0").
func stdBase64(s string) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var b strings.Builder
	for i := 0; i < len(s); i += 3 {
		var n uint32
		k := 0
		for ; k < 3 && i+k < len(s); k++ {
			n = n<<8 | uint32(s[i+k])
		}
		n <<= 8 * uint(3-k)
		for j := 0; j < 4; j++ {
			if j <= k {
				b.WriteByte(chars[n>>(18-6*uint(j))&63])
			} else {
				b.WriteByte('=')
			}
		}
	}
	return b.String()
}
