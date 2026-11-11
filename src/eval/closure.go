package eval

import (
	"errors"
	"fmt"
	"sort"

	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/ir"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
)

// A Proc in the envelope: the Closure message of cameo/value.proto. A
// written proc travels as the proc of a unit and its binding, the locals
// its code reads, by value, and the frame it was written in, by the names
// of its classes and its method. doc/design/ir.md has the design.

// uses is what the code of a proc, and of the blocks written in it, needs
// from the binding it runs in.
type uses struct {
	// reads are the locals the code may read.
	reads []string
	// self is set when the code names self, sets a field or calls super;
	// a call without a receiver alone does not set it.
	self bool
	// block is set when the code yields, and args when it has a bare
	// super: each uses what the method the proc was written in was called
	// with.
	block, args bool
}

// usesOf returns what the proc p needs, found once per program.
func (c *code) usesOf(p *irProc) *uses {
	if u, ok := c.uses.Load(p.proc); ok {
		return u.(*uses)
	}
	u := &uses{}
	reads := map[string]bool{}
	seen := map[*cameopb.Proc]bool{}
	var walkProc func(*cameopb.Proc)
	var walkPattern func(*cameopb.Pattern)
	var walk func([]*cameopb.Code)
	seqs := func(ss ...*cameopb.Seq) {
		for _, s := range ss {
			walk(s.GetCode())
		}
	}
	walkPattern = func(pt *cameopb.Pattern) {
		switch k := pt.GetKind().(type) {
		case *cameopb.Pattern_Value:
			seqs(k.Value)
		case *cameopb.Pattern_Const:
			seqs(k.Const)
		case *cameopb.Pattern_Binding:
			walkPattern(k.Binding.GetSub())
		case *cameopb.Pattern_Alt:
			for _, alt := range k.Alt.GetAlts() {
				walkPattern(alt)
			}
		case *cameopb.Pattern_List:
			seqs(k.List.GetConst())
			for _, sub := range append(append([]*cameopb.Pattern{}, k.List.GetPre()...), k.List.GetPost()...) {
				walkPattern(sub)
			}
		case *cameopb.Pattern_Find:
			seqs(k.Find.GetConst())
			for _, sub := range k.Find.GetMid() {
				walkPattern(sub)
			}
		case *cameopb.Pattern_Map:
			seqs(k.Map.GetConst())
			for _, entry := range k.Map.GetEntries() {
				if entry.GetValue() != nil {
					walkPattern(entry.GetValue())
				}
			}
		}
	}
	walk = func(code []*cameopb.Code) {
		for _, ins := range code {
			switch ins.Op {
			case cameopb.Op_LOAD, cameopb.Op_VAR_OR_CALL, cameopb.Op_DEFINED_LOCAL:
				reads[p.unit.Names[ins.Args[0].GetName()]] = true
			case cameopb.Op_SELF, cameopb.Op_FIELD_SET:
				u.self = true
			case cameopb.Op_SUPER:
				u.self = true
				if has(ins, cameopb.Flag_FLAG_FORWARD) {
					u.args = true
				}
				// A super passes the method's block on unless it names
				// another.
				u.block = true
			case cameopb.Op_YIELD, cameopb.Op_NEED_BLOCK, cameopb.Op_BLOCK_GIVEN, cameopb.Op_DEFINED_YIELD:
				u.block = true
			}
			for _, a := range ins.Args {
				switch k := a.GetKind().(type) {
				case *cameopb.Arg_Proc:
					// A block or lambda written here captures from this
					// scope. A method or class body has a scope of its
					// own, and reads nothing of this one.
					switch ins.Op {
					case cameopb.Op_LAMBDA, cameopb.Op_SEND, cameopb.Op_SUPER:
						walkProc(p.unit.Procs[k.Proc])
					}
				case *cameopb.Arg_Pattern:
					walkPattern(k.Pattern)
				}
			}
			seqs(ins.Code...)
		}
	}
	walkProc = func(proc *cameopb.Proc) {
		if seen[proc] {
			return
		}
		seen[proc] = true
		for _, param := range proc.GetParams().GetPositional() {
			seqs(param.GetDefault())
		}
		for _, param := range proc.GetParams().GetKeywords() {
			seqs(param.GetDefault())
		}
		walk(proc.GetCode())
	}
	walkProc(p.proc)
	for name := range reads {
		u.reads = append(u.reads, name)
	}
	sort.Strings(u.reads)
	actual, _ := c.uses.LoadOrStore(p.proc, u)
	return actual.(*uses)
}

// unencodable is the error for a Proc whose binding holds something the
// envelope cannot carry: what names the part, why is the envelope's own
// error for it.
func unencodable(what string, why error) error {
	kind := why.Error()
	var ns *protogen.NotSerialisable
	if errors.As(why, &ns) {
		kind = ns.Kind
	}
	return &protogen.NotSerialisable{Kind: "a Proc " + what + ", " + kind, Class: "Proc"}
}

// EncodeProc implements protogen.ProcCodec.
func (in *Interp) EncodeProc(e *protogen.Encoder, v object.Value) (*cameopb.Closure, error) {
	p := proc(v)
	switch {
	case p.Go != nil:
		return nil, &protogen.NotSerialisable{Kind: "a Proc a builtin made", Class: "Proc"}
	case p.Sym != "":
		c := &cameopb.Closure{Symbol: p.Sym, Strict: p.Strict}
		if !p.Recv.IsZero() {
			recv, err := e.Message(p.Recv)
			if err != nil {
				return nil, unencodable("of a value", err)
			}
			c.Recv = recv
		}
		return c, nil
	}
	unit := p.IR.unit
	c := &cameopb.Closure{Unit: unit.GetDigest(), Strict: p.Strict}
	for i, proc := range unit.GetProcs() {
		if proc == p.IR.proc {
			c.Proc = uint32(i)
		}
	}
	if e.CarryUnit(unit.GetDigest()) {
		c.Code = unit
	}
	needs := in.code.usesOf(p.IR)
	binding, err := in.encodeFrame(e, p.Frame, needs)
	if err != nil {
		return nil, err
	}
	// The locals in sight, an inner one shadowing an outer one, of which
	// the code's reads travel.
	flat := &Env{}
	p.Env.copyInto(flat)
	for _, name := range needs.reads {
		val, ok := flat.lookup(name)
		if !ok {
			continue
		}
		if held, isProc := asProc(val); isProc && held == p {
			c.SelfName = name
			continue
		}
		local := &cameopb.Binding_Local{Name: name}
		if cls, isClass := val.AsClass(); isClass {
			local.Class = cls.QualifiedName()
		} else if local.Value, err = e.Message(val); err != nil {
			return nil, unencodable("that captured "+name, err)
		}
		binding.Locals = append(binding.Locals, local)
	}
	c.Binding = binding
	return c, nil
}

// encodeFrame is the frame a proc was written in as a binding without its
// locals.
func (in *Interp) encodeFrame(e *protogen.Encoder, fr *Frame, needs *uses) (*cameopb.Binding, error) {
	b := &cameopb.Binding{File: fr.File}
	switch {
	case isMain(fr.Self):
	case fr.Self.Kind() == object.KindClass:
		b.SelfClass = fr.Self.Cls().QualifiedName()
	default:
		self, err := e.Message(fr.Self)
		switch {
		case err == nil:
			b.Self = self
		case needs.self:
			return nil, unencodable("whose self is "+typeName(fr.Self), err)
		}
		// Code that never names self runs with the top-level self where
		// its own cannot travel.
	}
	if m := fr.Method; m != nil {
		ref := &cameopb.Binding_Method{Name: m.Name}
		if m.Owner != nil && m.Owner != in.mainClass {
			ref.Owner = m.Owner.QualifiedName()
			ref.Singleton = m.Owner.SingletonMethods[m.Name] == m
		}
		b.Method = ref
	}
	for _, cls := range fr.Lexical {
		b.Lexical = append(b.Lexical, cls.QualifiedName())
	}
	if fr.DefTarget != nil && fr.DefTarget != in.mainClass {
		b.DefTarget = fr.DefTarget.QualifiedName()
	}
	if needs.args {
		for _, a := range fr.Args {
			msg, err := e.Message(a)
			if err != nil {
				return nil, unencodable("with a super that passes on an argument", err)
			}
			b.Args = append(b.Args, msg)
		}
		if fr.Kw != nil {
			kw, err := e.Message(object.MapValue(fr.Kw))
			if err != nil {
				return nil, unencodable("with a super that passes on a keyword", err)
			}
			b.Kw = kw.GetMap()
		}
	}
	if needs.block && !fr.Block.IsZero() {
		block, err := e.Message(fr.Block)
		if err != nil {
			return nil, unencodable("that yields to its method's block", err)
		}
		b.Block = block
	}
	return b, nil
}

// isMain reports whether v is the top-level self.
func isMain(v object.Value) bool {
	inst, ok := v.AsInstance()
	return ok && inst.Cls.Digest == "main"
}

// DecodeProc implements protogen.ProcCodec.
func (in *Interp) DecodeProc(d *protogen.Decoder, c *cameopb.Closure) (object.Value, error) {
	if c.GetSymbol() != "" {
		p := &Proc{Sym: c.GetSymbol(), Strict: c.GetStrict()}
		if c.GetRecv() != nil {
			recv, err := d.Value(c.GetRecv())
			if err != nil {
				return object.Value{}, err
			}
			p.Recv = recv
		}
		return procValue(p), nil
	}
	unit, err := in.unit(c)
	if err != nil {
		return object.Value{}, err
	}
	if int(c.GetProc()) >= len(unit.GetProcs()) {
		return object.Value{}, fmt.Errorf("unit %s has no proc %d", c.GetUnit(), c.GetProc())
	}
	fr, err := in.decodeFrame(d, c.GetBinding())
	if err != nil {
		return object.Value{}, err
	}
	env := &Env{captured: true}
	for _, local := range c.GetBinding().GetLocals() {
		var v object.Value
		if name := local.GetClass(); name != "" {
			cls := in.classNamed(name)
			if cls == nil {
				return object.Value{}, fmt.Errorf("no class %s", name)
			}
			v = object.ClassValue(cls)
		} else if v, err = d.Value(local.GetValue()); err != nil {
			return object.Value{}, fmt.Errorf("local %s: %w", local.GetName(), err)
		}
		env.bind(local.GetName(), v)
	}
	p := &Proc{IR: &irProc{unit: unit, proc: unit.Procs[c.GetProc()]}, Env: env, Frame: fr, Strict: c.GetStrict()}
	v := procValue(p)
	if name := c.GetSelfName(); name != "" {
		env.bind(name, v)
	}
	return v, nil
}

// unit returns the unit a closure refers to: the one it carries, which
// the program then keeps for the closures that follow, or one the program
// has.
func (in *Interp) unit(c *cameopb.Closure) (*cameopb.Unit, error) {
	if known, ok := in.code.units.Load(c.GetUnit()); ok {
		return known.(*cameopb.Unit), nil
	}
	unit := c.GetCode()
	if unit == nil {
		return nil, fmt.Errorf("the Proc's code, unit %s, is neither in the message nor in this program", c.GetUnit())
	}
	if unit.GetVersion() != ir.Version {
		return nil, fmt.Errorf("the Proc's code is IR version %d, and this cameo runs version %d", unit.GetVersion(), ir.Version)
	}
	digest, err := ir.Digest(unit)
	if err != nil {
		return nil, err
	}
	if digest != c.GetUnit() {
		return nil, fmt.Errorf("the Proc names unit %s and carries unit %s", c.GetUnit(), digest)
	}
	actual, _ := in.code.units.LoadOrStore(digest, unit)
	return actual.(*cameopb.Unit), nil
}

// decodeFrame makes the frame of a decoded proc. No call is running it,
// so a return in the proc raises LocalJumpError, unless the proc was
// written at the top level, whose frame lasts as long as the run.
func (in *Interp) decodeFrame(d *protogen.Decoder, b *cameopb.Binding) (*Frame, error) {
	fr := &Frame{File: b.GetFile(), DefTarget: in.mainClass}
	class := func(name string) (*object.Class, error) {
		if cls := in.classNamed(name); cls != nil {
			return cls, nil
		}
		return nil, fmt.Errorf("no class %s", name)
	}
	var err error
	switch {
	case b.GetSelfClass() != "":
		cls, err := class(b.GetSelfClass())
		if err != nil {
			return nil, err
		}
		fr.Self = object.ClassValue(cls)
	case b.GetSelf() != nil:
		if fr.Self, err = d.Value(b.GetSelf()); err != nil {
			return nil, err
		}
	default:
		fr.Self = object.InstanceValue(in.main)
	}
	for _, name := range b.GetLexical() {
		cls, err := class(name)
		if err != nil {
			return nil, err
		}
		fr.Lexical = append(fr.Lexical, cls)
	}
	if name := b.GetDefTarget(); name != "" {
		if fr.DefTarget, err = class(name); err != nil {
			return nil, err
		}
	}
	if ref := b.GetMethod(); ref != nil {
		owner := in.mainClass
		if ref.GetOwner() != "" {
			if owner, err = class(ref.GetOwner()); err != nil {
				return nil, err
			}
		}
		m := owner.Methods[ref.GetName()]
		if ref.GetSingleton() {
			m = owner.SingletonMethods[ref.GetName()]
		}
		if m == nil {
			return nil, fmt.Errorf("no method %s in %s", ref.GetName(), owner.QualifiedName())
		}
		fr.Method, fr.Owner, fr.ended = m, m.Owner, true
	}
	for _, a := range b.GetArgs() {
		v, err := d.Value(a)
		if err != nil {
			return nil, err
		}
		fr.Args = append(fr.Args, v)
	}
	if b.GetKw() != nil {
		kw, err := d.Value(&cameopb.Value{Kind: &cameopb.Value_Map{Map: b.GetKw()}})
		if err != nil {
			return nil, err
		}
		fr.Kw = kw.Map()
	}
	if b.GetBlock() != nil {
		if fr.Block, err = d.Value(b.GetBlock()); err != nil {
			return nil, err
		}
	}
	return fr, nil
}
