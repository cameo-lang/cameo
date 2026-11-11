package protogen

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/object"
)

// The envelope, cameo.Value, carries any serialisable value. It is what
// async serialises and what the host API takes and returns. Its messages
// are written in src/cameopb/cameo/value.proto.

// envelope returns the file of the envelope, which the message of an
// exception class imports.
func envelope() protoreflect.FileDescriptor { return cameopb.File_cameo_value_proto }

// ProcCodec carries Procs through the envelope. A Proc is the evaluator's,
// opaque here, so the evaluator is what turns one into a Closure message
// and back.
type ProcCodec interface {
	EncodeProc(e *Encoder, v object.Value) (*cameopb.Closure, error)
	DecodeProc(d *Decoder, c *cameopb.Closure) (object.Value, error)
}

// Encoder serialises values into the envelope.
type Encoder struct {
	reg *Registry
	// procs encodes Procs; without it a Proc cannot be encoded.
	procs ProcCodec
	// carryUnits makes the first closure of a unit carry the unit's code,
	// for a reader that does not have it; sent holds the digests of the
	// units this encoder has carried.
	carryUnits bool
	sent       map[string]bool
	// classes is told the class of every instance and exception encoded.
	classes func(*object.Class)
	// symbols makes a Symbol travel as one, and not as a String.
	symbols bool
}

// NewEncoder makes an encoder over the registry's classes.
func NewEncoder(reg *Registry) *Encoder { return &Encoder{reg: reg} }

// WithProcs returns the encoder set to encode Procs through codec. With
// carryUnits set, the messages it makes carry the code their closures
// refer to, each unit once, so that a program that never compiled the
// code can run them; without, the reader must have the units already, as
// a worker of the same program does.
func (e *Encoder) WithProcs(codec ProcCodec, carryUnits bool) *Encoder {
	e.procs, e.carryUnits, e.sent = codec, carryUnits, map[string]bool{}
	return e
}

// WithClasses returns the encoder set to tell seen the class of every
// instance and exception it encodes, for a writer that carries the
// classes beside the values, as a dump does.
func (e *Encoder) WithClasses(seen func(*object.Class)) *Encoder {
	e.classes = seen
	return e
}

// WithSymbols returns the encoder set to write a Symbol as a symbol,
// which reads back as one. A message for another program does not: there
// a symbol is a string, the one form every reader has. A dump does, for
// a run to resume with the values it had.
func (e *Encoder) WithSymbols() *Encoder {
	e.symbols = true
	return e
}

// CarryUnit reports whether a closure of the unit with the given digest
// is to carry the unit's code: the first one this encoder makes, when it
// carries units at all.
func (e *Encoder) CarryUnit(digest string) bool {
	if !e.carryUnits || e.sent[digest] {
		return false
	}
	e.sent[digest] = true
	return true
}

// Encode is the wire form of a value in the envelope. A Proc or a class
// cannot be encoded. An exception travels as an Object, as an instance
// does.
func (e *Encoder) Encode(v object.Value) ([]byte, error) {
	msg, err := e.Message(v)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(msg)
}

// Message is a value as a cameo.Value message.
func (e *Encoder) Message(v object.Value) (*cameopb.Value, error) {
	switch v.Kind() {
	case object.KindBool:
		return &cameopb.Value{Kind: &cameopb.Value_BoolValue{BoolValue: v.Bool()}}, nil
	case object.KindInt:
		return &cameopb.Value{Kind: &cameopb.Value_IntValue{IntValue: int64(v.Int())}}, nil
	case object.KindBig:
		return &cameopb.Value{Kind: &cameopb.Value_BigValue{BigValue: v.Big().String()}}, nil
	case object.KindFloat:
		return &cameopb.Value{Kind: &cameopb.Value_FloatValue{FloatValue: float64(v.Float())}}, nil
	case object.KindSymbol:
		if e.symbols {
			return &cameopb.Value{Kind: &cameopb.Value_SymbolValue{SymbolValue: string(v.Str())}}, nil
		}
		return &cameopb.Value{Kind: &cameopb.Value_StringValue{StringValue: string(v.Str())}}, nil
	case object.KindString, object.KindASCII:
		return &cameopb.Value{Kind: &cameopb.Value_StringValue{StringValue: string(v.Str())}}, nil
	case object.KindBytes:
		return &cameopb.Value{Kind: &cameopb.Value_BytesValue{BytesValue: []byte(v.Bytes())}}, nil
	case object.KindList:
		items, err := e.items(v.List().Elems)
		if err != nil {
			return nil, err
		}
		return &cameopb.Value{Kind: &cameopb.Value_List{List: &cameopb.List{Items: items}}}, nil
	case object.KindSet:
		items, err := e.items(v.Set().Elems())
		if err != nil {
			return nil, err
		}
		return &cameopb.Value{Kind: &cameopb.Value_Set{Set: &cameopb.Set{Items: items}}}, nil
	case object.KindMap:
		m, err := e.mapMessage(v.Map())
		if err != nil {
			return nil, err
		}
		return &cameopb.Value{Kind: &cameopb.Value_Map{Map: m}}, nil
	case object.KindRange:
		r := v.Range()
		out := &cameopb.Range{Exclusive: r.Exclusive}
		var err error
		if !r.Lo.IsZero() {
			if out.Lo, err = e.Message(r.Lo); err != nil {
				return nil, err
			}
		}
		if !r.Hi.IsZero() {
			if out.Hi, err = e.Message(r.Hi); err != nil {
				return nil, err
			}
		}
		return &cameopb.Value{Kind: &cameopb.Value_Range{Range: out}}, nil
	case object.KindTime:
		return &cameopb.Value{Kind: &cameopb.Value_Time{Time: timeMessage(v.Time())}}, nil
	case object.KindRegexp:
		r := v.Regexp()
		return &cameopb.Value{Kind: &cameopb.Value_Regexp{Regexp: &cameopb.Regexp{Source: r.Source, Options: int32(r.Options)}}}, nil
	case object.KindInstance, object.KindException:
		var cls *object.Class
		var data []byte
		var err error
		if exc, ok := v.AsException(); ok {
			cls = exc.Cls
			data, err = e.reg.MarshalException(exc)
		} else {
			inst := v.Instance()
			cls = inst.Cls
			data, err = e.reg.Marshal(inst)
		}
		if err != nil {
			return nil, err
		}
		if e.classes != nil {
			e.classes(cls)
		}
		return &cameopb.Value{Kind: &cameopb.Value_Object{Object: &cameopb.Object{Class: cls.QualifiedName(), Digest: cls.Digest, Message: data}}}, nil
	case object.KindProc:
		if e.procs == nil {
			break
		}
		closure, err := e.procs.EncodeProc(e, v)
		if err != nil {
			return nil, err
		}
		return &cameopb.Value{Kind: &cameopb.Value_Closure{Closure: closure}}, nil
	}
	if v.IsNil() {
		return &cameopb.Value{}, nil
	}
	return nil, &NotSerialisable{Kind: article(kindName(v)), Class: kindName(v)}
}

func (e *Encoder) items(elems []object.Value) ([]*cameopb.Value, error) {
	out := make([]*cameopb.Value, len(elems))
	for i, el := range elems {
		m, err := e.Message(el)
		if err != nil {
			return nil, err
		}
		out[i] = m
	}
	return out, nil
}

func (e *Encoder) mapMessage(m *object.Map) (*cameopb.Map, error) {
	out := &cameopb.Map{Entries: make([]*cameopb.Map_Entry, 0, len(m.Keys()))}
	for i, k := range m.Keys() {
		km, err := e.Message(k)
		if err != nil {
			return nil, err
		}
		vm, err := e.Message(m.Values()[i])
		if err != nil {
			return nil, err
		}
		out.Entries = append(out.Entries, &cameopb.Map_Entry{Key: km, Value: vm})
	}
	value, proc := m.Default()
	if !proc.IsZero() {
		// The default proc travels as any Proc does, where the encoder
		// carries procs.
		if e.procs == nil {
			return nil, &NotSerialisable{Kind: "a Hash with a default proc", Class: "Hash", DefaultProc: true}
		}
		closure, err := e.procs.EncodeProc(e, proc)
		if err != nil {
			return nil, err
		}
		out.DefaultProc = closure
	}
	if !value.IsZero() {
		dm, err := e.Message(value)
		if err != nil {
			return nil, err
		}
		out.Default = dm
	}
	return out, nil
}

// timeMessage is a time as its instant, its UTC offset and its zone.
func timeMessage(t time.Time) *cameopb.Time {
	_, off := t.Zone()
	var zone string
	switch loc := t.Location(); loc {
	case time.UTC:
		zone = "UTC"
	case time.Local:
		zone = "local"
	default:
		// A named zone travels by its name, so that the time keeps
		// its daylight saving; a fixed offset has none.
		zone = loc.String()
	}
	return &cameopb.Time{Seconds: t.Unix(), Nanos: int32(t.Nanosecond()), Offset: int32(off), Zone: zone}
}

// NotSerialisable is the error for a value the envelope cannot carry: a
// proc, a class, a Hash with a default proc, an instance of a class with
// a proc field, or one of a class without fields, a library's. Kind
// names it with its article, "a Proc", for a message about what was done
// with it; Class is the value's class, which a message in Ruby's words
// names, and DefaultProc is set for the Hash; Why, when set, is the whole
// explanation, which Error gives instead.
type NotSerialisable struct {
	Kind, Why   string
	Class       string
	DefaultProc bool
}

func (e *NotSerialisable) Error() string {
	if e.Why != "" {
		return e.Why
	}
	return e.Kind + " cannot be serialised"
}

// article prefixes a noun with "a" or "an".
func article(noun string) string {
	if noun != "" && strings.ContainsRune("aeioAEIO", rune(noun[0])) {
		return "an " + noun
	}
	return "a " + noun
}

// ClassResolver finds the class an Object message names, by qualified name
// and digest; nil when there is none.
type ClassResolver func(name, digest string) *object.Class

// Decoder reads values out of the envelope.
type Decoder struct {
	reg     *Registry
	resolve ClassResolver
	// procs decodes closures; without it a message with one is refused.
	procs ProcCodec
}

// NewDecoder makes a decoder that finds classes through resolve.
func NewDecoder(reg *Registry, resolve ClassResolver) *Decoder {
	return &Decoder{reg: reg, resolve: resolve}
}

// WithProcs returns the decoder set to decode closures through codec.
func (d *Decoder) WithProcs(codec ProcCodec) *Decoder {
	d.procs = codec
	return d
}

// Decode reads a value from its wire form.
func (d *Decoder) Decode(data []byte) (object.Value, error) {
	msg := &cameopb.Value{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return object.Value{}, err
	}
	return d.Value(msg)
}

// reflected reads a cameo.Value message that is not the generated type:
// the field of a class's message, which is a dynamic message.
func (d *Decoder) reflected(msg protoreflect.Message) (object.Value, error) {
	if v, ok := msg.Interface().(*cameopb.Value); ok {
		return d.Value(v)
	}
	data, err := proto.Marshal(msg.Interface())
	if err != nil {
		return object.Value{}, err
	}
	return d.Decode(data)
}

// Value reads a cameo.Value message.
func (d *Decoder) Value(msg *cameopb.Value) (object.Value, error) {
	switch k := msg.GetKind().(type) {
	case nil:
		return object.Nil, nil
	case *cameopb.Value_BoolValue:
		return object.Bool(k.BoolValue), nil
	case *cameopb.Value_IntValue:
		return object.Int(k.IntValue), nil
	case *cameopb.Value_BigValue:
		b, ok := new(big.Int).SetString(k.BigValue, 10)
		if !ok {
			return object.Value{}, fmt.Errorf("cameo: %q is not an integer", k.BigValue)
		}
		return object.BigInt(b), nil
	case *cameopb.Value_FloatValue:
		return object.Float(k.FloatValue), nil
	case *cameopb.Value_StringValue:
		return object.String(k.StringValue), nil
	case *cameopb.Value_SymbolValue:
		return object.Symbol(k.SymbolValue), nil
	case *cameopb.Value_BytesValue:
		return object.Bytes(append([]byte{}, k.BytesValue...)), nil
	case *cameopb.Value_List:
		items, err := d.items(k.List.GetItems())
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(items...), nil
	case *cameopb.Value_Set:
		items, err := d.items(k.Set.GetItems())
		if err != nil {
			return object.Value{}, err
		}
		return object.SetValue(object.NewSet(items...)), nil
	case *cameopb.Value_Map:
		b := object.NewMapBuilder()
		for _, entry := range k.Map.GetEntries() {
			key, err := d.Value(entry.GetKey())
			if err != nil {
				return object.Value{}, err
			}
			v, err := d.Value(entry.GetValue())
			if err != nil {
				return object.Value{}, err
			}
			b.Set(key, v)
		}
		out := b.Map()
		if k.Map.GetDefault() != nil {
			value, err := d.Value(k.Map.GetDefault())
			if err != nil {
				return object.Value{}, err
			}
			out = out.WithDefault(value, object.Value{})
		}
		if closure := k.Map.GetDefaultProc(); closure != nil {
			if d.procs == nil {
				return object.Value{}, fmt.Errorf("the message holds a Hash with a default proc, which this reader does not decode")
			}
			proc, err := d.procs.DecodeProc(d, closure)
			if err != nil {
				return object.Value{}, err
			}
			out = out.WithDefault(object.Value{}, proc)
		}
		return object.MapValue(out), nil
	case *cameopb.Value_Range:
		out := &object.Range{Exclusive: k.Range.GetExclusive()}
		var err error
		if k.Range.GetLo() != nil {
			if out.Lo, err = d.Value(k.Range.GetLo()); err != nil {
				return object.Value{}, err
			}
		}
		if k.Range.GetHi() != nil {
			if out.Hi, err = d.Value(k.Range.GetHi()); err != nil {
				return object.Value{}, err
			}
		}
		return object.RangeValue(out), nil
	case *cameopb.Value_Time:
		return object.TimeValue(timeOf(k.Time)), nil
	case *cameopb.Value_Regexp:
		r, err := object.NewRegexp(k.Regexp.GetSource(), int(k.Regexp.GetOptions()))
		if err != nil {
			return object.Value{}, err
		}
		return object.RegexpValue(r), nil
	case *cameopb.Value_Object:
		name, digest := k.Object.GetClass(), k.Object.GetDigest()
		cls := d.resolve(name, digest)
		if cls == nil {
			return object.Value{}, fmt.Errorf("no class %s (digest %s) is loaded", name, digest)
		}
		data := k.Object.GetMessage()
		if exceptionParts(cls) != nil {
			exc, err := d.UnmarshalException(cls, data)
			if err != nil {
				return object.Value{}, err
			}
			return object.ExceptionValue(exc), nil
		}
		inst, err := d.reg.Unmarshal(cls, data)
		if err != nil {
			return object.Value{}, err
		}
		return object.InstanceValue(inst), nil
	case *cameopb.Value_Closure:
		if d.procs == nil {
			return object.Value{}, fmt.Errorf("the message holds a Proc, which this reader does not decode")
		}
		return d.procs.DecodeProc(d, k.Closure)
	}
	return object.Value{}, fmt.Errorf("unknown envelope kind %T", msg.GetKind())
}

// timeOf reads a time from its message.
func timeOf(m *cameopb.Time) time.Time {
	t := time.Unix(m.GetSeconds(), int64(m.GetNanos()))
	fixed := func() time.Time { return t.In(time.FixedZone("", int(m.GetOffset()))) }
	switch zone := m.GetZone(); zone {
	case "UTC":
		return t.UTC()
	case "local":
		return t.Local()
	case "":
		return fixed()
	default:
		// A name this binary cannot resolve keeps the offset the
		// sender had, as a fixed zone.
		if loc, err := time.LoadLocation(zone); err == nil {
			return t.In(loc)
		}
		return fixed()
	}
}

func (d *Decoder) items(msgs []*cameopb.Value) ([]object.Value, error) {
	out := make([]object.Value, len(msgs))
	for i, m := range msgs {
		v, err := d.Value(m)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func kindName(v object.Value) string {
	if v.IsNil() {
		return "nil"
	}
	return v.Class().QualifiedName()
}
