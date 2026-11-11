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
}

// param is one positional parameter: required (Default nil, Splat false),
// optional (Default set) or the one splat.
type param struct {
	Name string
	// sym is Name interned, set for a parameter of a proc of a unit.
	sym     object.Sym
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
	actual, _ := c.params.LoadOrStore(p.proc, out)
	return actual.(*params)
}

// paramsOf returns the parameter list of the proc p. The interpreter
// keeps the lists it asked for by unit and index, so a call reads its
// list without the lock-free map every interpreter of the run shares.
func (in *Interp) paramsOf(p *irProc) *params {
	if p.unit != in.paramsUnit {
		list, ok := in.paramsBy[p.unit]
		if !ok {
			if in.paramsBy == nil {
				in.paramsBy = map[*cameopb.Unit][]*params{}
			}
			list = make([]*params, len(p.unit.Procs))
			in.paramsBy[p.unit] = list
		}
		in.paramsUnit, in.paramsList = p.unit, list
	}
	if ps := in.paramsList[p.index]; ps != nil {
		return ps
	}
	ps := in.code.irParams(p)
	in.paramsList[p.index] = ps
	return ps
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

// symsOf returns the names of unit as Syms, index for index. The run's
// interpreters share the list of a unit, and each keeps the one it asked
// for last.
func (in *Interp) symsOf(unit *cameopb.Unit) []object.Sym {
	if unit == in.symsUnit {
		return in.symsList
	}
	var list []object.Sym
	if known, ok := in.code.syms.Load(unit); ok {
		list = known.([]object.Sym)
	} else {
		list = make([]object.Sym, len(unit.Names))
		for i, name := range unit.Names {
			list[i] = object.Intern(name)
		}
		in.code.syms.Store(unit, list)
	}
	in.symsUnit, in.symsList = unit, list
	return list
}
