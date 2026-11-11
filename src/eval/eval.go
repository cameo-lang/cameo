package eval

import (
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

// What several instructions of the IR evaluator (vm.go) share: string and
// regexp interpolation, splats, destructuring and `===`.

// interpolate concatenates the n parts of an interpolated string: part
// evaluates each, which is converted with to_s before the next one is
// evaluated.
func (in *Interp) interpolate(n, line int, part func(i int) (object.Value, error)) (object.Value, error) {
	var b strings.Builder
	// The result takes the encoding its parts are compatible in, as
	// Ruby's does: a binary part in UTF-8 text makes it binary.
	// UTF-8 parts alone, the usual case, need no looking at.
	kind := object.KindString
	for i := 0; i < n; i++ {
		v, err := part(i)
		if err != nil {
			return object.Value{}, err
		}
		s, err := in.ToS(v)
		if err != nil {
			return object.Value{}, withLine(err, line)
		}
		if pk := v.Kind(); v.IsString() && (pk != kind || kind != object.KindString) {
			sofar := b.String()
			next, ok := stdlib.CombineEncodings(kind, sofar == "", object.ASCIIOnly(sofar), pk, s == "", object.ASCIIOnly(s))
			if !ok {
				return object.Value{}, in.raise(object.CompatibilityError, line, "%s", stdlib.IncompatibleEncodings(kind, pk))
			}
			kind = next
		}
		b.WriteString(s)
	}
	return object.StringOf(kind, b.String()), nil
}

// newRegexp makes the regexp of a literal from its source and options.
func (in *Interp) newRegexp(source string, options, line int) (object.Value, error) {
	re, err := object.NewRegexp(source, options)
	if err != nil {
		return object.Value{}, withLine(stdlib.RegexpFailure(in, err, source), line)
	}
	return object.RegexpValue(re), nil
}

// interpolateRegexp makes a regexp of the n parts of a literal with
// interpolation: part evaluates each, a regexp embeds as its to_s,
// (?flags:source), as in Ruby, and any other value as its to_s before the
// next part is evaluated.
func (in *Interp) interpolateRegexp(n, options, line int, part func(i int) (object.Value, error)) (object.Value, error) {
	var b strings.Builder
	for i := 0; i < n; i++ {
		v, err := part(i)
		if err != nil {
			return object.Value{}, err
		}
		if re, ok := v.AsRegexp(); ok {
			b.WriteString(re.String())
			continue
		}
		s, err := in.ToS(v)
		if err != nil {
			return object.Value{}, err
		}
		b.WriteString(s)
	}
	return in.newRegexp(b.String(), options, line)
}

// splat appends what `*v` stands for to out: the elements of a list, a
// set, a range or an enumerator, nothing for nil, and any other value
// itself.
func (in *Interp) splat(out []object.Value, v object.Value, line int) ([]object.Value, error) {
	switch v.Kind() {
	case object.KindList:
		return append(out, v.List().Elems...), nil
	case object.KindSet:
		return append(out, v.Set().Elems()...), nil
	case object.KindRange:
		list, err := in.rangeToList(v.Range(), line)
		if err != nil {
			return nil, err
		}
		return append(out, list.Elems...), nil
	case object.KindEnumerator:
		err := v.Enumerator().Each(in, func(el object.Value) error {
			out = append(out, el)
			return nil
		})
		return out, err
	}
	if v.IsNil() {
		return out, nil
	}
	return append(out, v), nil
}

// assignNames destructures values over names with an optional splat.
func assignNames(names []string, splat int, values []object.Value, env *Env) {
	if splat < 0 {
		for i, name := range names {
			if i < len(values) {
				env.Set(name, values[i])
			} else {
				env.Set(name, object.Nil)
			}
		}
		return
	}
	after := len(names) - splat - 1
	for i := 0; i < splat; i++ {
		env.Set(names[i], at(values, i))
	}
	rest := []object.Value{}
	if len(values) > splat+after {
		rest = append(rest, values[splat:len(values)-after]...)
	}
	env.Set(names[splat], object.NewList(rest...))
	for i := 0; i < after; i++ {
		env.Set(names[splat+1+i], at(values, len(values)-after+i))
	}
}

func at(values []object.Value, i int) object.Value {
	if i >= 0 && i < len(values) {
		return values[i]
	}
	return object.Nil
}

// caseEqual is `pattern === value`: class membership for a class, inclusion
// for a range, a call for a proc, equality otherwise.
func (in *Interp) caseEqual(pattern, v object.Value) (bool, error) {
	switch pattern.Kind() {
	case object.KindClass:
		p := pattern.Cls()
		return v.Class().IsA(p), nil
	case object.KindRange:
		p := pattern.Range()
		return in.rangeIncludes(p, v)
	case object.KindRegexp:
		switch v.Kind() {
		case object.KindString, object.KindBytes, object.KindASCII, object.KindSymbol:
			return pattern.Regexp().MatchAt(v.Str(), 0) != nil, nil
		}
		return false, nil
	case object.KindProc:
		p := proc(pattern)
		r, err := in.callBlock(procValue(p), []object.Value{v}, nil, object.Value{})
		if err != nil {
			return false, err
		}
		return r.Truthy(), nil
	}
	return in.Equal(pattern, v)
}
