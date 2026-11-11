package eval

import (
	"fmt"

	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
)

// Handles in a dump: a handle is a library's value that changes, a
// StringIO, and its state is the library's Go data. A class whose library
// can put its handles through a dump has a State hook (object.StateHook).
// The dump keeps each handle once and refers to it by index, so what
// held one handle holds one after the resume.

// handleDump collects the handles a dump holds, as its encoders meet
// them.
type handleDump struct {
	in      *Interp
	classes *classDump
	// enc encodes what a handle's library saved of it.
	enc  *protogen.Encoder
	ids  map[*object.Instance]uint32
	list []*cameopb.Dump_Handle
}

// index returns the index, plus one, of the handle inst is among the
// dump's, and has the dump hold it when it is new; zero for an instance
// that is no handle a dump holds.
func (hd *handleDump) index(inst *object.Instance) (uint32, error) {
	hook := inst.Cls.State
	if hook == nil {
		return 0, nil
	}
	if i, ok := hd.ids[inst]; ok {
		return i + 1, nil
	}
	state, err := hook.Save(hd.in, object.InstanceValue(inst))
	if err != nil {
		return 0, err
	}
	// The handle has its index before its state is encoded: a state
	// that holds the handle itself refers to it.
	i := uint32(len(hd.list))
	h := &cameopb.Dump_Handle{Class: hd.classes.ref(inst.Cls)}
	hd.ids[inst], hd.list = i, append(hd.list, h)
	if h.State, err = hd.enc.Message(state); err != nil {
		delete(hd.ids, inst)
		return 0, fmt.Errorf("what %s holds: %w", article(inst.Cls.QualifiedName()), err)
	}
	return i + 1, nil
}

// handleTable is the handles of the dump a run resumed from. Resume makes
// them all, so the run and its workers only read the table.
type handleTable struct {
	msgs   []*cameopb.Dump_Handle
	made   []object.Value
	making []bool
}

// handlesOf readies the table of a dump's handles; nil for a dump
// without any. A decoder made after it is set reads references to them.
func handlesOf(msgs []*cameopb.Dump_Handle) *handleTable {
	if len(msgs) == 0 {
		return nil
	}
	return &handleTable{msgs: msgs, made: make([]object.Value, len(msgs)), making: make([]bool, len(msgs))}
}

// restoreHandles makes the handles of a dump again, each through the
// hook of its class.
func (in *Interp) restoreHandles(dec *protogen.Decoder) error {
	if in.handles == nil {
		return nil
	}
	for i := range in.handles.msgs {
		if _, err := in.handle(dec, uint32(i)+1); err != nil {
			return err
		}
	}
	return nil
}

// handle returns the handle a dump refers to with ref, an index plus one.
func (in *Interp) handle(dec *protogen.Decoder, ref uint32) (object.Value, error) {
	ht := in.handles
	if ht == nil || ref == 0 || int(ref) > len(ht.msgs) {
		return object.Value{}, fmt.Errorf("the dump has no handle %d", int(ref)-1)
	}
	i := ref - 1
	if !ht.made[i].IsZero() {
		return ht.made[i], nil
	}
	msg := ht.msgs[i]
	if ht.making[i] {
		return object.Value{}, fmt.Errorf("handle %d of the dump, %s, holds itself", i, article(msg.GetClass()))
	}
	ht.making[i] = true
	cls := in.classRef(msg.GetClass())
	if cls == nil || cls.State == nil {
		return object.Value{}, fmt.Errorf("handle %d of the dump is %s, which this cameo cannot make again", i, article(msg.GetClass()))
	}
	state, err := dec.Value(msg.GetState())
	if err != nil {
		return object.Value{}, fmt.Errorf("handle %d of the dump, %s: %w", i, article(msg.GetClass()), err)
	}
	v, err := cls.State.Restore(in, state)
	if err != nil {
		return object.Value{}, fmt.Errorf("handle %d of the dump, %s: %w", i, article(msg.GetClass()), err)
	}
	ht.made[i] = v
	return v, nil
}
