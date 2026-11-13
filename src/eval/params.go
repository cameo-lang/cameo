package eval

import (
	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/object"
)

// params is the parameter list of a method, block or lambda as bindParams
// reads it, made from the AST's or from the IR's: a default is a function
// that evaluates it in the scope the parameters are being bound in.
type params struct {
	Positional []param
	Keywords   []kwParam
	KwRest     string // **name, or ""
	BlockParam string // &name, or ""
	// plain is set when every parameter is a required positional one.
	plain bool
	// layout is the slots of the scope a call of the proc runs in: its
	// parameters, then the variables its code names. reads is the slots
	// of the variables the proc reads, itself or in the blocks written
	// in it: what a block of it captures. Both are set for a proc of a
	// unit.
	layout *layout
	reads  []int32
	// contained is set when nothing a call of the proc makes can keep
	// its frame or its scope after the call ended: its code writes no
	// block, lambda, method or class and starts no async call. The
	// frame, the scope and the arguments of such a call are not
	// allocated (methodAct).
	contained bool
}

// param is one positional parameter: required (Default nil, Splat false),
// optional (Default set) or the one splat.
type param struct {
	Name string
	// sym is Name interned and slot its slot in the layout, set for a
	// parameter of a proc of a unit.
	sym     object.Sym
	slot    int32
	Default defaultFn
	Splat   bool
}

// kwParam is a keyword parameter, required of a method or lambda when
// Default is nil.
type kwParam struct {
	Name    string
	Default defaultFn
}

type defaultFn func(in *Interp, env *Env, fr *Frame) (object.Value, error)

// irParams returns the parameter list of the proc p, made once per
// program.
func (c *code) irParams(p *irProc) *params {
	if ps, ok := c.params.Load(p.proc); ok {
		return ps.(*params)
	}
	unit, src := p.unit, p.proc.GetParams()
	out := &params{}
	deflt := func(seq *cameopb.Seq) defaultFn {
		if seq == nil {
			return nil
		}
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			return in.runList(unit, seq.GetCode(), env, fr)
		}
	}
	for _, sp := range src.GetPositional() {
		out.Positional = append(out.Positional, param{Name: unit.Names[sp.GetName()], sym: object.Intern(unit.Names[sp.GetName()]), Splat: sp.GetSplat(), Default: deflt(sp.GetDefault())})
	}
	for _, sk := range src.GetKeywords() {
		out.Keywords = append(out.Keywords, kwParam{Name: unit.Names[sk.GetName()], Default: deflt(sk.GetDefault())})
	}
	if src != nil && src.KwRest != nil {
		out.KwRest = unit.Names[src.GetKwRest()]
	}
	if src != nil && src.Block != nil {
		out.BlockParam = unit.Names[src.GetBlock()]
	}
	out.plain = len(out.Keywords) == 0 && out.KwRest == "" && out.BlockParam == ""
	for _, param := range out.Positional {
		if param.Splat || param.Default != nil {
			out.plain = false
		}
	}
	names := make([]object.Sym, 0, len(out.Positional)+4)
	for _, param := range out.Positional {
		names = append(names, param.sym)
	}
	for _, kw := range out.Keywords {
		names = append(names, object.Intern(kw.Name))
	}
	for _, name := range []string{out.KwRest, out.BlockParam} {
		if name != "" {
			names = append(names, object.Intern(name))
		}
	}
	names = locals(unit, p.proc, names)
	reads := c.usesOf(p).reads
	for _, name := range reads {
		names = append(names, object.Intern(name))
	}
	out.layout = newLayout(names)
	out.contained = contained(p.proc)
	for i := range out.Positional {
		out.Positional[i].slot = int32(out.layout.slot(out.Positional[i].sym))
	}
	for _, name := range reads {
		out.reads = append(out.reads, int32(out.layout.slot(object.Intern(name))))
	}
	actual, _ := c.params.LoadOrStore(p.proc, out)
	return actual.(*params)
}

// contained returns whether no instruction of proc, its parameters'
// defaults included, names a proc or is an async call or an await. A
// block, a lambda, a method and a class body are procs an instruction
// names, and they are what keeps a frame (Proc.Frame) or reaches a scope
// later (Env.scope, for the promises of a block); an async call keeps
// its promise in the scope.
func contained(proc *cameopb.Proc) bool {
	var free func([]*cameopb.Code) bool
	free = func(code []*cameopb.Code) bool {
		for _, ins := range code {
			if awaits(ins) {
				return false
			}
			for _, a := range ins.Args {
				if _, ok := a.GetKind().(*cameopb.Arg_Proc); ok {
					return false
				}
			}
			for _, seq := range ins.Code {
				if !free(seq.GetCode()) {
					return false
				}
			}
		}
		return true
	}
	for _, param := range proc.GetParams().GetPositional() {
		if !free(param.GetDefault().GetCode()) {
			return false
		}
	}
	for _, param := range proc.GetParams().GetKeywords() {
		if !free(param.GetDefault().GetCode()) {
			return false
		}
	}
	return free(proc.GetCode())
}

// locals adds to names the variables the code of proc names, the code of
// its parameters' defaults too: what it reads and binds, the exception a
// rescue clause names and what a pattern binds. A block written in the
// proc has a scope of its own.
func locals(unit *cameopb.Unit, proc *cameopb.Proc, names []object.Sym) []object.Sym {
	add := func(i uint32) {
		if name := unit.Names[i]; name != "" {
			names = append(names, object.Intern(name))
		}
	}
	var walk func([]*cameopb.Code)
	var pattern func(*cameopb.Pattern)
	pattern = func(pt *cameopb.Pattern) {
		switch k := pt.GetKind().(type) {
		case *cameopb.Pattern_Value:
			walk(k.Value.GetCode())
		case *cameopb.Pattern_Const:
			walk(k.Const.GetCode())
		case *cameopb.Pattern_Bind:
			add(k.Bind.GetName())
		case *cameopb.Pattern_Binding:
			add(k.Binding.GetName())
			pattern(k.Binding.GetSub())
		case *cameopb.Pattern_Alt:
			for _, alt := range k.Alt.GetAlts() {
				pattern(alt)
			}
		case *cameopb.Pattern_List:
			walk(k.List.GetConst().GetCode())
			for _, sub := range k.List.GetPre() {
				pattern(sub)
			}
			for _, sub := range k.List.GetPost() {
				pattern(sub)
			}
			if k.List.SplatName != nil {
				add(k.List.GetSplatName())
			}
		case *cameopb.Pattern_Find:
			walk(k.Find.GetConst().GetCode())
			for _, sub := range k.Find.GetMid() {
				pattern(sub)
			}
			if k.Find.PreName != nil {
				add(k.Find.GetPreName())
			}
			if k.Find.PostName != nil {
				add(k.Find.GetPostName())
			}
		case *cameopb.Pattern_Map:
			walk(k.Map.GetConst().GetCode())
			for _, entry := range k.Map.GetEntries() {
				if entry.GetValue() == nil {
					add(entry.GetKey())
				} else {
					pattern(entry.GetValue())
				}
			}
			if k.Map.RestName != nil {
				add(k.Map.GetRestName())
			}
		}
	}
	walk = func(code []*cameopb.Code) {
		for _, ins := range code {
			switch ins.Op {
			case cameopb.Op_LOAD, cameopb.Op_STORE, cameopb.Op_VAR_OR_CALL, cameopb.Op_DEFINED_LOCAL:
				add(ins.Args[0].GetName())
			case cameopb.Op_BEGIN:
				// The name of the exception of each rescue clause.
				for i := 0; 3+2*i < len(ins.Code) && i < len(ins.Args); i++ {
					add(ins.Args[i].GetName())
				}
			}
			for _, a := range ins.Args {
				if k, ok := a.GetKind().(*cameopb.Arg_Pattern); ok {
					pattern(k.Pattern)
				}
			}
			for _, seq := range ins.Code {
				walk(seq.GetCode())
			}
		}
	}
	for _, param := range proc.GetParams().GetPositional() {
		walk(param.GetDefault().GetCode())
	}
	for _, param := range proc.GetParams().GetKeywords() {
		walk(param.GetDefault().GetCode())
	}
	walk(proc.GetCode())
	return names
}

// paramsOf returns the parameter list of the proc p. The interpreter
// keeps the lists it asked for by unit and index, so a call reads its
// list without the lock-free map every interpreter of the run shares.
func (in *Interp) paramsOf(p *irProc) *params {
	if ps := in.paramsAt(p.unit, p.index); ps != nil {
		return ps
	}
	ps := in.code.irParams(p)
	in.paramsList[p.index] = ps
	return ps
}

// procParams is paramsOf for proc index of unit.
func (in *Interp) procParams(unit *cameopb.Unit, index uint32) *params {
	if ps := in.paramsAt(unit, index); ps != nil {
		return ps
	}
	ps := in.code.irParams(procOf(unit, index))
	in.paramsList[index] = ps
	return ps
}

// paramsAt returns the parameter list the interpreter keeps for proc
// index of unit, nil when it has not asked for it yet; the lists of the
// unit are then the ones it reads next.
func (in *Interp) paramsAt(unit *cameopb.Unit, index uint32) *params {
	if unit != in.paramsUnit {
		list, ok := in.paramsBy[unit]
		if !ok {
			if in.paramsBy == nil {
				in.paramsBy = map[*cameopb.Unit][]*params{}
				in.flatBy = map[*cameopb.Unit][]lowered{}
			}
			list = make([]*params, len(unit.Procs))
			in.paramsBy[unit] = list
			in.flatBy[unit] = make([]lowered, len(unit.Procs))
		}
		in.paramsUnit, in.paramsList, in.flatList = unit, list, in.flatBy[unit]
	}
	return in.paramsList[index]
}

// arity is Proc#arity of a parameter list: the number of required
// positional parameters, negative when it takes a splat or optional
// parameters, or, unless strict, keywords.
func (p *params) arity(strict bool) int {
	required, variable := 0, false
	for _, param := range p.Positional {
		switch {
		case param.Splat, param.Default != nil:
			variable = true
		default:
			required++
		}
	}
	if variable || (!strict && len(p.Keywords) > 0) {
		return -(required + 1)
	}
	return required
}

// unitNames is what the evaluator keeps of each name of a unit, index
// for index: its Sym, and whether a call of it can be one of Ruby's
// mutators.
type unitNames struct {
	syms     []object.Sym
	mutators []bool
}

// symsOf returns the names of unit as the evaluator reads them. The
// run's interpreters share them, and each keeps the unit it asked for
// last.
func (in *Interp) symsOf(unit *cameopb.Unit) *unitNames {
	if unit == in.symsUnit {
		return in.symsList
	}
	var names *unitNames
	if known, ok := in.code.syms.Load(unit); ok {
		names = known.(*unitNames)
	} else {
		names = &unitNames{syms: make([]object.Sym, len(unit.Names)), mutators: make([]bool, len(unit.Names))}
		for i, name := range unit.Names {
			names.syms[i] = object.Intern(name)
			names.mutators[i] = mayMutate(name)
		}
		in.code.syms.Store(unit, names)
	}
	in.symsUnit, in.symsList = unit, names
	return names
}

// flatOfProc returns the flat code of the proc p, nil when it runs from
// cursors. The interpreter keeps what it lowered by unit and index.
func (in *Interp) flatOfProc(p *irProc) *flatCode {
	ps := in.paramsOf(p)
	if f := &in.flatList[p.index]; f.lowered {
		return f.code
	}
	code := in.flatOfLaid(p.unit, p.proc.GetCode(), true, ps.layout, nil)
	in.flatList[p.index] = lowered{code: code, lowered: true}
	return code
}

// lowered is the flat code of a list, nil for a list that runs from
// cursors, once lowering was tried.
type lowered struct {
	code    *flatCode
	lowered bool
}

// flatOf returns the flat code of the list code of unit, nil when it runs
// from cursors. The run's interpreters share what was lowered, by the
// list's first instruction.
func (in *Interp) flatOf(unit *cameopb.Unit, code []*cameopb.Code, body bool) *flatCode {
	return in.flatOfLaid(unit, code, body, nil, nil)
}

// flatOfLaid is flatOf for a list that runs in a scope with the layout
// lay, whose variables its flat code reads and writes by slot. With top
// set the scope is that one, the top level's, whose layout is its own:
// lowering gives each variable the list names a slot in it.
func (in *Interp) flatOfLaid(unit *cameopb.Unit, code []*cameopb.Code, body bool, lay *layout, top *Env) *flatCode {
	if len(code) == 0 {
		return nil
	}
	if known, ok := in.code.flat.Load(code[0]); ok {
		return known.(*flatCode)
	}
	flat := lower(code, in.symsOf(unit), body, lay, top)
	actual, _ := in.code.flat.LoadOrStore(code[0], flat)
	return actual.(*flatCode)
}
