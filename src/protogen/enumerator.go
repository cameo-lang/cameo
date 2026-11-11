package protogen

import (
	"errors"
	"fmt"

	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/object"
)

// An Enumerator produces its elements with Go code, which no message
// carries. What a message carries is the call that made it
// (object.Enumerator.Make): the receiver, the method and what it was
// called with. Values never change, so the reader makes an enumerator
// with the same elements by making the call again.

// EnumeratorMaker makes an enumerator again from the call that made it,
// for a decoder: it calls the method name on recv.
type EnumeratorMaker func(recv object.Value, name string, args []object.Value, kw *object.Map, block object.Value) (object.Value, error)

// WithEnumerators returns the decoder set to make enumerators through
// make; without it a message with one is refused.
func (d *Decoder) WithEnumerators(make EnumeratorMaker) *Decoder {
	d.enumerators = make
	return d
}

const (
	ofChain   = "chain"
	ofProduct = "product"
)

// enumerator is the message of an enumerator.
func (e *Encoder) enumerator(en *object.Enumerator) (*cameopb.Enumerator, error) {
	part := func(what string, v object.Value) (*cameopb.Value, error) {
		m, err := e.Message(v)
		if err != nil {
			return nil, unmade(what, err)
		}
		return m, nil
	}
	if en.Of != nil {
		out := &cameopb.Enumerator{Of: ofChain}
		if en.Of == object.ProductClass {
			out.Of = ofProduct
		}
		for _, p := range en.Parts {
			m, err := part("that combines", p)
			if err != nil {
				return nil, err
			}
			out.Parts = append(out.Parts, m)
		}
		return out, nil
	}
	if en.Make == "" {
		return nil, &NotSerialisable{Kind: "an Enumerator", Class: "Enumerator"}
	}
	out := &cameopb.Enumerator{Method: en.Make}
	if en.Make == "to_enum" {
		out.Method, out.ToEnum = en.Meth, true
	}
	var err error
	if out.Recv, err = part("of", en.Recv); err != nil {
		return nil, err
	}
	for _, a := range en.Args {
		m, err := part("with the argument", a)
		if err != nil {
			return nil, err
		}
		out.Args = append(out.Args, m)
	}
	if en.Kw != nil && len(en.Kw.Keys()) > 0 {
		kw, err := part("with the keywords", object.MapValue(en.Kw))
		if err != nil {
			return nil, err
		}
		out.Kw = kw.GetMap()
	}
	if !en.Block.IsZero() && !en.Block.IsNil() {
		if e.procs == nil {
			return nil, &NotSerialisable{Kind: "an Enumerator with a block", Class: "Enumerator"}
		}
		if out.Block, err = e.procs.EncodeProc(e, en.Block); err != nil {
			return nil, unmade("with a block,", err)
		}
	}
	return out, nil
}

// unmade is the error for an enumerator a part of whose call cannot be
// serialised: what says which part, why is the envelope's error for it.
func unmade(what string, why error) error {
	kind := why.Error()
	var ns *NotSerialisable
	if errors.As(why, &ns) {
		kind = ns.Kind
	}
	return &NotSerialisable{Kind: "an Enumerator " + what + " " + kind, Class: "Enumerator"}
}

// enumerator makes the enumerator of a message.
func (d *Decoder) enumerator(m *cameopb.Enumerator) (object.Value, error) {
	if d.enumerators == nil {
		return object.Value{}, fmt.Errorf("the message holds an Enumerator, which this reader does not decode")
	}
	var recv object.Value
	name := m.GetMethod()
	var args []object.Value
	var err error
	switch m.GetOf() {
	case ofChain:
		recv, name = object.ClassValue(object.ChainClass), "new"
	case ofProduct:
		recv, name = object.ClassValue(object.ProductClass), "new"
	case "":
		if recv, err = d.Value(m.GetRecv()); err != nil {
			return object.Value{}, err
		}
		if m.GetToEnum() {
			name, args = "to_enum", []object.Value{object.Symbol(m.GetMethod())}
		}
	default:
		return object.Value{}, fmt.Errorf("an Enumerator of %q is unknown", m.GetOf())
	}
	msgs := m.GetArgs()
	if m.GetOf() != "" {
		msgs = m.GetParts()
	}
	more, err := d.items(msgs)
	if err != nil {
		return object.Value{}, err
	}
	args = append(args, more...)
	var kw *object.Map
	if m.GetKw() != nil {
		v, err := d.Value(&cameopb.Value{Kind: &cameopb.Value_Map{Map: m.GetKw()}})
		if err != nil {
			return object.Value{}, err
		}
		// Keywords are symbols, which a message that is no dump carries
		// as strings.
		b := object.NewMapBuilder()
		for i, k := range v.Map().Keys() {
			if s, ok := k.AsString(); ok {
				k = object.Symbol(s)
			}
			b.Set(k, v.Map().Values()[i])
		}
		kw = b.Map()
	}
	var block object.Value
	if m.GetBlock() != nil {
		if d.procs == nil {
			return object.Value{}, fmt.Errorf("the message holds an Enumerator with a block, which this reader does not decode")
		}
		if block, err = d.procs.DecodeProc(d, m.GetBlock()); err != nil {
			return object.Value{}, err
		}
	}
	// Reading a message makes no call but one that makes an enumerator
	// and does nothing else.
	if m.GetOf() == "" && !object.MakesEnumerator(recv, name, more, block) {
		return object.Value{}, fmt.Errorf("the message holds an Enumerator of %s, a call that is not known to make one", name)
	}
	v, err := d.enumerators(recv, name, args, kw, block)
	if err != nil {
		return object.Value{}, fmt.Errorf("an Enumerator of %s: %w", name, err)
	}
	if v.Kind() != object.KindEnumerator {
		return object.Value{}, fmt.Errorf("%s made no Enumerator", name)
	}
	return v, nil
}
