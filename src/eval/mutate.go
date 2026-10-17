package eval

import (
	"strings"

	"github.com/radiospiel/amber/src/object"
	"github.com/radiospiel/amber/src/stdlib"
	"github.com/radiospiel/amber/src/syntax"
)

// A mutating call on a local, `x << y` or `x.map! { }`, computes the
// changed copy and rebinds the local to it, as `h[k] = v` does: amber values
// never change, so another variable holding the old value keeps it. The
// call's own value is Ruby's: the receiver for most, the removed element
// for pop, shift and delete, nil from a bang method that changed nothing.

// mutation computes what a mutating call leaves and returns: next is the
// changed receiver, result the call's value.
type mutation func(in *Interp, fr *Frame, cur object.Value, args []object.Value, kw *object.Map, block object.Value, line int) (next, result object.Value, err error)

// evalMutCall runs a MutCall's steps and rebinds its target to what the
// mutating ones leave.
func (in *Interp) evalMutCall(n *syntax.MutCall, env *Env, fr *Frame) (object.Value, error) {
	cur, err := in.eval(n.Target, env, fr)
	if err != nil {
		return object.Value{}, err
	}
	var result object.Value
	owned, changed := true, false
	for _, step := range n.Steps {
		args, kw, block, err := in.stepArgs(step, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		if m := mutationFor(cur, step.Name); m != nil {
			next, res, err := m(in, fr, cur, args, kw, block, step.Line())
			if err != nil {
				return object.Value{}, withLine(err, step.Line())
			}
			cur, result = next, res
			changed = changed || owned
			continue
		}
		// A step that does not change its receiver is an ordinary call; a
		// mutation after it changes only the call's fresh result.
		v, err := in.call(fr, cur, step.Name, args, kw, block, false, step.Line())
		if err != nil {
			return object.Value{}, withLine(err, step.Line())
		}
		if owned && changed {
			if err := in.rebind(n.Target, cur, env, fr); err != nil {
				return object.Value{}, err
			}
		}
		owned = false
		cur, result = v, v
	}
	if owned && changed {
		if err := in.rebind(n.Target, cur, env, fr); err != nil {
			return object.Value{}, err
		}
	}
	return result, nil
}

func (in *Interp) rebind(target syntax.Node, v object.Value, env *Env, fr *Frame) error {
	switch t := target.(type) {
	case *syntax.Var:
		env.Set(t.Name, v)
		return nil
	case *syntax.Field:
		return in.writeField(t.Name, v, fr, t.Line())
	}
	return in.raise(object.AmberError, 0, "cannot rebind %T", target)
}

// stepArgs evaluates a step's arguments and block, as evalCall does.
func (in *Interp) stepArgs(n *syntax.Call, env *Env, fr *Frame) ([]object.Value, *object.Map, object.Value, error) {
	args, kw, err := in.evalCallArgs(n.Args, n.KwArgs, env, fr, n.Line())
	if err != nil {
		return nil, nil, object.Value{}, err
	}
	var block object.Value
	switch {
	case n.Block != nil:
		block = procValue(&Proc{Block: n.Block, Env: env, Frame: fr})
	case n.BlockArg != nil:
		v, err := in.eval(n.BlockArg, env, fr)
		if err != nil {
			return nil, nil, object.Value{}, err
		}
		if block, err = in.toProc(v, n.Line()); err != nil {
			return nil, nil, object.Value{}, err
		}
	}
	return args, kw, block, nil
}

// mutationFor is what name does to a value of cur's kind when it changes
// it, or nil when it does not: a String's delete returns a new string, an
// Integer's << shifts, and an instance's methods are its own.
func mutationFor(cur object.Value, name string) mutation {
	if m := mutations[cur.Kind()][name]; m != nil {
		return m
	}
	switch cur.Kind() {
	case object.KindList, object.KindString, object.KindMap, object.KindSet, object.KindBytes, object.KindASCII:
	default:
		return nil
	}
	if !strings.HasSuffix(name, "!") || name == "slice!" {
		return nil
	}
	base := strings.TrimSuffix(name, "!")
	if cur.Class().Lookup(base) == nil {
		return nil
	}
	return bang(base, bangReturnsSelf[name])
}

// bang is a bang method through its copying twin: x.uniq! is x.uniq,
// returning nil when it changed nothing, as most bang methods do in Ruby,
// or the receiver for those that always return it.
func bang(base string, self bool) mutation {
	return func(in *Interp, fr *Frame, cur object.Value, args []object.Value, kw *object.Map, block object.Value, line int) (object.Value, object.Value, error) {
		next, err := in.call(fr, cur, base, args, kw, block, false, line)
		if err != nil {
			return object.Value{}, object.Value{}, err
		}
		switch {
		case cur.Kind() == object.KindSet && next.Kind() == object.KindList:
			// Set#map returns a list; Set#map! keeps a set.
			next = object.SetValue(object.NewSet(next.List().Elems...))
		case cur.Kind() == object.KindMap && next.Kind() == object.KindMap:
			// Changed in place, a hash keeps its default.
			next = object.MapValue(keepDefault(cur.Map(), next.Map()))
		}
		if !self && object.Equal(cur, next) {
			return cur, object.Nil, nil
		}
		return next, next, nil
	}
}

func keepDefault(from, out *object.Map) *object.Map {
	value, proc := from.Default()
	if value.IsZero() && proc.IsZero() {
		return out
	}
	return out.WithDefault(value, proc)
}

// bangReturnsSelf are the bang methods that return the receiver whether
// or not they changed it.
var bangReturnsSelf = map[string]bool{
	"map!": true, "collect!": true, "sort!": true, "sort_by!": true, "shuffle!": true,
	"reverse!": true, "rotate!": true, "transform_values!": true, "transform_keys!": true,
	"merge!": true, "succ!": true, "next!": true, "scrub!": true,
}

// returns wraps a mutation whose value is the changed receiver.
func returnsSelf(f func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error)) mutation {
	return func(in *Interp, fr *Frame, cur object.Value, args []object.Value, kw *object.Map, block object.Value, line int) (object.Value, object.Value, error) {
		next, err := f(in, fr, cur, args, block, line)
		return next, next, err
	}
}

func listOf(v object.Value) []object.Value { return v.List().Elems }

func concatLists(parts ...[]object.Value) object.Value {
	out := []object.Value{}
	for _, p := range parts {
		out = append(out, p...)
	}
	return object.NewList(out...)
}

// via is a mutation through a copying method of the receiver, keeping a
// hash's default.
func via(name string) mutation {
	return func(in *Interp, fr *Frame, cur object.Value, args []object.Value, kw *object.Map, block object.Value, line int) (object.Value, object.Value, error) {
		next, err := in.call(fr, cur, name, args, kw, block, false, line)
		if err != nil {
			return object.Value{}, object.Value{}, err
		}
		if cur.Kind() == object.KindMap && next.Kind() == object.KindMap {
			next = object.MapValue(keepDefault(cur.Map(), next.Map()))
		}
		return next, next, nil
	}
}

func wantArgs(in *Interp, args []object.Value, n int, line int) error {
	if len(args) != n {
		return in.raise(object.ArgumentError, line, "wrong number of arguments (given %d, expected %d)", len(args), n)
	}
	return nil
}

var mutations map[object.Kind]map[string]mutation

func init() {
	list := map[string]mutation{
		"<<": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			if err := wantArgs(in, args, 1, line); err != nil {
				return object.Value{}, err
			}
			return concatLists(listOf(cur), args), nil
		}),
		"push": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			return concatLists(listOf(cur), args), nil
		}),
		"unshift": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			return concatLists(args, listOf(cur)), nil
		}),
		"insert": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			if len(args) < 1 {
				return object.Value{}, in.raise(object.ArgumentError, line, "wrong number of arguments (given 0, expected 1+)")
			}
			i, ok := args[0].AsInt()
			if !ok {
				return object.Value{}, in.raise(object.TypeError, line, "no implicit conversion of %s into Integer", typeName(args[0]))
			}
			es := listOf(cur)
			if i < 0 {
				i += int64(len(es)) + 1
				if i < 0 {
					return object.Value{}, in.raise(object.IndexError, line, "index %d too small for array; minimum: -%d", i-int64(len(es))-1, len(es)+1)
				}
			}
			for int64(len(es)) < i {
				es = append(append([]object.Value{}, es...), object.Nil)
			}
			return concatLists(es[:i], args[1:], es[i:]), nil
		}),
		"concat": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			parts := [][]object.Value{listOf(cur)}
			for _, a := range args {
				l, ok := a.AsList()
				if !ok {
					return object.Value{}, in.raise(object.TypeError, line, "no implicit conversion of %s into Array", typeName(a))
				}
				parts = append(parts, l.Elems)
			}
			return concatLists(parts...), nil
		}),
		"pop":   takeEnd(true),
		"shift": takeEnd(false),
		"delete": func(in *Interp, fr *Frame, cur object.Value, args []object.Value, kw *object.Map, block object.Value, line int) (object.Value, object.Value, error) {
			if err := wantArgs(in, args, 1, line); err != nil {
				return object.Value{}, object.Value{}, err
			}
			out := []object.Value{}
			result := object.Nil
			for _, e := range listOf(cur) {
				eq, err := in.Equal(e, args[0])
				if err != nil {
					return object.Value{}, object.Value{}, err
				}
				if eq {
					result = e
					continue
				}
				out = append(out, e)
			}
			if result.IsNil() && !block.IsZero() {
				r, err := in.Yield(block, args[0])
				return cur, r, err
			}
			return object.NewList(out...), result, nil
		},
		"delete_at": func(in *Interp, fr *Frame, cur object.Value, args []object.Value, kw *object.Map, block object.Value, line int) (object.Value, object.Value, error) {
			if err := wantArgs(in, args, 1, line); err != nil {
				return object.Value{}, object.Value{}, err
			}
			i, ok := args[0].AsInt()
			if !ok {
				return object.Value{}, object.Value{}, in.raise(object.TypeError, line, "no implicit conversion of %s into Integer", typeName(args[0]))
			}
			es := listOf(cur)
			if i < 0 {
				i += int64(len(es))
			}
			if i < 0 || i >= int64(len(es)) {
				return cur, object.Nil, nil
			}
			return concatLists(es[:i], es[i+1:]), es[i], nil
		},
		"delete_if": via("reject"),
		"keep_if":   via("select"),
		"clear": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			return object.NewList(), nil
		}),
		"replace": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			if err := wantArgs(in, args, 1, line); err != nil {
				return object.Value{}, err
			}
			l, ok := args[0].AsList()
			if !ok {
				return object.Value{}, in.raise(object.TypeError, line, "no implicit conversion of %s into Array", typeName(args[0]))
			}
			return object.ListValue(l), nil
		}),
	}
	str := map[string]mutation{
		"<<": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			if err := wantArgs(in, args, 1, line); err != nil {
				return object.Value{}, err
			}
			return appendStrings(in, cur, args, line)
		}),
		"concat": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			return appendStrings(in, cur, args, line)
		}),
		"prepend": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			var b strings.Builder
			for _, a := range args {
				s, ok := a.AsString()
				if !ok {
					return object.Value{}, in.raise(object.TypeError, line, "no implicit conversion of %s into String", typeName(a))
				}
				b.WriteString(s)
			}
			return object.String(b.String() + cur.Str()), nil
		}),
		"insert": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			if err := wantArgs(in, args, 2, line); err != nil {
				return object.Value{}, err
			}
			i, ok := args[0].AsInt()
			s, ok2 := args[1].AsString()
			if !ok || !ok2 {
				return object.Value{}, in.raise(object.TypeError, line, "insert takes an index and a string")
			}
			rs := []rune(cur.Str())
			if i < 0 {
				i += int64(len(rs)) + 1
			}
			if i < 0 || i > int64(len(rs)) {
				return object.Value{}, in.raise(object.IndexError, line, "index %d out of string", args[0].Int())
			}
			return object.String(string(rs[:i]) + s + string(rs[i:])), nil
		}),
		"replace": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			if err := wantArgs(in, args, 1, line); err != nil {
				return object.Value{}, err
			}
			if _, ok := args[0].AsString(); !ok {
				return object.Value{}, in.raise(object.TypeError, line, "no implicit conversion of %s into String", typeName(args[0]))
			}
			return args[0], nil
		}),
		"clear": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			return object.String(""), nil
		}),
	}
	hash := map[string]mutation{
		"store": func(in *Interp, fr *Frame, cur object.Value, args []object.Value, kw *object.Map, block object.Value, line int) (object.Value, object.Value, error) {
			if err := wantArgs(in, args, 2, line); err != nil {
				return object.Value{}, object.Value{}, err
			}
			return object.MapValue(cur.Map().Put(args[0], args[1])), args[1], nil
		},
		"delete": func(in *Interp, fr *Frame, cur object.Value, args []object.Value, kw *object.Map, block object.Value, line int) (object.Value, object.Value, error) {
			if err := wantArgs(in, args, 1, line); err != nil {
				return object.Value{}, object.Value{}, err
			}
			m := cur.Map()
			v, ok := m.Get(args[0])
			if !ok {
				if !block.IsZero() {
					r, err := in.Yield(block, args[0])
					return cur, r, err
				}
				return cur, object.Nil, nil
			}
			return object.MapValue(keepDefault(m, m.Without(args[0]))), v, nil
		},
		"shift": func(in *Interp, fr *Frame, cur object.Value, args []object.Value, kw *object.Map, block object.Value, line int) (object.Value, object.Value, error) {
			k, v, rest, ok := cur.Map().Shift()
			if !ok {
				return cur, object.Nil, nil
			}
			return object.MapValue(rest), object.NewList(k, v), nil
		},
		"delete_if": via("reject"),
		"keep_if":   via("select"),
		"update":    via("merge"),
		"clear": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			return object.MapValue(keepDefault(cur.Map(), object.NewMap())), nil
		}),
		"replace": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			if err := wantArgs(in, args, 1, line); err != nil {
				return object.Value{}, err
			}
			if _, ok := args[0].AsMap(); !ok {
				return object.Value{}, in.raise(object.TypeError, line, "no implicit conversion of %s into Hash", typeName(args[0]))
			}
			return args[0], nil
		}),
	}
	set := map[string]mutation{
		"<<":  via("add"),
		"add": via("add"),
		"add?": func(in *Interp, fr *Frame, cur object.Value, args []object.Value, kw *object.Map, block object.Value, line int) (object.Value, object.Value, error) {
			if err := wantArgs(in, args, 1, line); err != nil {
				return object.Value{}, object.Value{}, err
			}
			if cur.Set().Has(args[0]) {
				return cur, object.Nil, nil
			}
			next := object.SetValue(cur.Set().With(args[0]))
			return next, next, nil
		},
		"delete": via("without"),
		"delete?": func(in *Interp, fr *Frame, cur object.Value, args []object.Value, kw *object.Map, block object.Value, line int) (object.Value, object.Value, error) {
			if err := wantArgs(in, args, 1, line); err != nil {
				return object.Value{}, object.Value{}, err
			}
			if !cur.Set().Has(args[0]) {
				return cur, object.Nil, nil
			}
			next := object.SetValue(cur.Set().Without(args[0]))
			return next, next, nil
		},
		"merge": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			s := cur.Set()
			for _, a := range args {
				elems, err := in.elemsOf(a, line)
				if err != nil {
					return object.Value{}, err
				}
				for _, e := range elems {
					s = s.With(e)
				}
			}
			return object.SetValue(s), nil
		}),
		"subtract": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			if err := wantArgs(in, args, 1, line); err != nil {
				return object.Value{}, err
			}
			elems, err := in.elemsOf(args[0], line)
			if err != nil {
				return object.Value{}, err
			}
			s := cur.Set()
			for _, e := range elems {
				s = s.Without(e)
			}
			return object.SetValue(s), nil
		}),
		"delete_if": via("reject"),
		"keep_if":   via("select"),
		"clear": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			return object.SetValue(object.NewSet()), nil
		}),
		"replace": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			if err := wantArgs(in, args, 1, line); err != nil {
				return object.Value{}, err
			}
			elems, err := in.elemsOf(args[0], line)
			if err != nil {
				return object.Value{}, err
			}
			return object.SetValue(object.NewSet(elems...)), nil
		}),
		"reset": returnsSelf(func(in *Interp, fr *Frame, cur object.Value, args []object.Value, block object.Value, line int) (object.Value, error) {
			return cur, nil
		}),
	}
	// force_encoding changes what a string is, text or bytes, and returns
	// it changed.
	str["force_encoding"] = via("force_encoding")
	mutations = map[object.Kind]map[string]mutation{
		object.KindList: list, object.KindString: str, object.KindMap: hash, object.KindSet: set,
		object.KindBytes: str, object.KindASCII: str,
	}
}

// takeEnd is pop (from the end) and shift (from the start), with an
// optional count.
func takeEnd(end bool) mutation {
	return func(in *Interp, fr *Frame, cur object.Value, args []object.Value, kw *object.Map, block object.Value, line int) (object.Value, object.Value, error) {
		es := listOf(cur)
		if len(args) == 0 {
			if len(es) == 0 {
				return cur, object.Nil, nil
			}
			if end {
				return object.NewList(es[:len(es)-1]...), es[len(es)-1], nil
			}
			return object.NewList(es[1:]...), es[0], nil
		}
		n, ok := args[0].AsInt()
		if !ok || n < 0 {
			return object.Value{}, object.Value{}, in.raise(object.ArgumentError, line, "negative array size")
		}
		k := min(int(n), len(es))
		if end {
			return object.NewList(es[:len(es)-k]...), object.NewList(es[len(es)-k:]...), nil
		}
		return object.NewList(es[k:]...), object.NewList(es[:k]...), nil
	}
}

// appendStrings is String#<< and #concat: strings, or integers as
// codepoints.
func appendStrings(in *Interp, cur object.Value, args []object.Value, line int) (object.Value, error) {
	var b strings.Builder
	b.WriteString(cur.Str())
	kind, ascii := cur.Kind(), object.ASCIIOnly(cur.Str())
	for _, a := range args {
		switch a.Kind() {
		case object.KindString, object.KindBytes, object.KindASCII:
			partASCII := object.ASCIIOnly(a.Str())
			next, ok := stdlib.CombineEncodings(kind, b.Len() == 0, ascii, a.Kind(), a.Str() == "", partASCII)
			if !ok {
				return object.Value{}, in.raise(object.CompatibilityError, line, "%s", stdlib.IncompatibleEncodings(kind, a.Kind()))
			}
			kind, ascii = next, ascii && partASCII
			b.WriteString(a.Str())
		case object.KindInt:
			if c := a.Int(); c < 0 || c > 0x10ffff {
				return object.Value{}, in.raise(object.RangeError, line, "%d out of char range", c)
			}
			b.WriteRune(rune(a.Int()))
		case object.KindBig:
			return object.Value{}, in.raise(object.RangeError, line, "bignum out of char range")
		default:
			return object.Value{}, in.raise(object.TypeError, line, "no implicit conversion of %s into String", typeName(a))
		}
	}
	return object.StringOf(kind, b.String()), nil
}

// elemsOf is the elements of a list, set, range or map, as Set#merge
// takes them.
func (in *Interp) elemsOf(v object.Value, line int) ([]object.Value, error) {
	switch v.Kind() {
	case object.KindList:
		return v.List().Elems, nil
	case object.KindSet:
		return v.Set().Elems(), nil
	}
	r, err := in.call(nil, v, "to_a", nil, nil, object.Value{}, false, line)
	if err != nil {
		return nil, err
	}
	l, ok := r.AsList()
	if !ok {
		return nil, in.raise(object.TypeError, line, "value must be enumerable")
	}
	return l.Elems, nil
}
