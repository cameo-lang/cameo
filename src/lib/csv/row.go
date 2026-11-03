package csv

import (
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

// CSV::Row is a row with its headers: an instance holding the headers and
// the fields, and whether it is the header row. It is a value: []=, << and
// delete return the changed copy, for the variable to take.

// row is the Go side of a CSV::Row.
type row struct {
	headers   []object.Value
	fields    []object.Value
	headerRow bool
}

func newRow(headers, fields []object.Value, headerRow bool) object.Value {
	return object.InstanceValue(&object.Instance{Cls: rowClass, Message: &row{headers: headers, fields: fields, headerRow: headerRow}})
}

func isRow(v object.Value) bool {
	inst, ok := v.AsInstance()
	return ok && inst.Cls == rowClass
}

func rowOf(v object.Value) *row { return v.Instance().Message.(*row) }

// size is the longer of headers and fields: a short row reads nil fields,
// a long one nil headers.
func (r *row) size() int { return max(len(r.headers), len(r.fields)) }

// allHeaders is Row#headers: the headers, nil for a field beyond them.
func (r *row) allHeaders() []object.Value {
	out := make([]object.Value, r.size())
	for i := range out {
		out[i] = r.header(i)
	}
	return out
}

func (r *row) header(i int) object.Value {
	if i < len(r.headers) {
		return r.headers[i]
	}
	return object.Nil
}

func (r *row) field(i int) object.Value {
	if i < len(r.fields) {
		return r.fields[i]
	}
	return object.Nil
}

// index finds a header, from an offset: -1 when absent.
func (r *row) index(ctx object.Context, header object.Value, from int) (int, error) {
	for i := from; i < len(r.headers); i++ {
		eq, err := ctx.Equal(r.headers[i], header)
		if err != nil {
			return 0, err
		}
		if eq {
			return i, nil
		}
	}
	return -1, nil
}

// lookup is Row#[]: a field by index or by header, the header searched
// from an offset.
func (r *row) lookup(ctx object.Context, args []object.Value) (object.Value, int, error) {
	if len(args) < 1 || len(args) > 2 {
		return object.Value{}, 0, stdlib.ArgError(ctx, len(args), "1..2")
	}
	if i, ok := args[0].AsInt(); ok {
		if i < 0 {
			i += int64(r.size())
		}
		if i < 0 || int(i) >= r.size() {
			return object.Nil, -1, nil
		}
		return r.field(int(i)), int(i), nil
	}
	from := 0
	if len(args) == 2 {
		n, err := stdlib.WantInt(ctx, args[1], "")
		if err != nil {
			return object.Value{}, 0, err
		}
		from = int(n)
	}
	i, err := r.index(ctx, args[0], from)
	if err != nil || i < 0 {
		return object.Nil, -1, err
	}
	return r.field(i), i, nil
}

// pairs lists [header, field] for each position.
func (r *row) pairs() []object.Value {
	out := make([]object.Value, r.size())
	for i := range out {
		out[i] = object.NewList(r.header(i), r.field(i))
	}
	return out
}

// toHash is Row#to_h: the first field under each header.
func (r *row) toHash(ctx object.Context) (object.Value, error) {
	m := object.NewMap()
	for i := 0; i < r.size(); i++ {
		h := r.header(i)
		if m.Has(h) {
			continue
		}
		m = m.Put(h, r.field(i))
	}
	return object.MapValue(m), nil
}

// with returns the row with a field set, by header or index: a new header
// is appended.
func (r *row) with(ctx object.Context, key, value object.Value) (object.Value, error) {
	fields := append([]object.Value(nil), r.fields...)
	headers := append([]object.Value(nil), r.headers...)
	if i, ok := key.AsInt(); ok {
		for int(i) >= len(fields) {
			fields = append(fields, object.Nil)
		}
		fields[i] = value
		return newRow(headers, fields, r.headerRow), nil
	}
	i, err := r.index(ctx, key, 0)
	if err != nil {
		return object.Value{}, err
	}
	if i < 0 {
		for len(fields) < len(headers) {
			fields = append(fields, object.Nil)
		}
		headers = append(headers, key)
		fields = append(fields, value)
		return newRow(headers, fields, r.headerRow), nil
	}
	for i >= len(fields) {
		fields = append(fields, object.Nil)
	}
	fields[i] = value
	return newRow(headers, fields, r.headerRow), nil
}

// push is Row#<<: a [header, field] pair, a one-pair Hash, or a field
// under a nil header.
func (r *row) push(ctx object.Context, v object.Value) (object.Value, error) {
	headers := append([]object.Value(nil), r.headers...)
	fields := append([]object.Value(nil), r.fields...)
	for len(fields) < len(headers) {
		fields = append(fields, object.Nil)
	}
	switch {
	case v.Kind() == object.KindList && len(v.List().Elems) == 2:
		headers = append(headers, v.List().Elems[0])
		fields = append(fields, v.List().Elems[1])
	case v.Kind() == object.KindMap:
		m := v.Map()
		for i, k := range m.Keys() {
			headers = append(headers, k)
			fields = append(fields, m.Values()[i])
		}
	default:
		headers = append(headers, object.Nil)
		fields = append(fields, v)
	}
	return newRow(headers, fields, r.headerRow), nil
}

// inspect is Row#inspect: #<CSV::Row "a":1 "b":2>.
func (r *row) inspect(ctx object.Context) (string, error) {
	var b strings.Builder
	b.WriteString("#<CSV::Row")
	for i := 0; i < r.size(); i++ {
		h, f := r.header(i), r.field(i)
		var hs string
		if name, ok := h.AsSymbol(); ok {
			hs = name
		} else {
			var err error
			if hs, err = ctx.Inspect(h); err != nil {
				return "", err
			}
		}
		fs, err := ctx.Inspect(f)
		if err != nil {
			return "", err
		}
		b.WriteString(" " + hs + ":" + fs)
	}
	b.WriteString(">")
	return b.String(), nil
}

func installRow() {
	c := rowClass
	c.Lexical = module
	module.Consts["Row"] = object.ClassValue(c)
	c.Include(object.Enumerable)
	self := func(v object.Value) *row { return rowOf(v) }

	c.DefineSingleton("new", -1, func(ctx object.Context, s object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 2 || len(args) > 3 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "2..3")
		}
		headers, err := stdlib.ToList(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		fields, err := stdlib.ToList(ctx, args[1])
		if err != nil {
			return object.Value{}, err
		}
		headerRow := len(args) == 3 && args[2].Truthy()
		return newRow(headers, fields, headerRow), nil
	})
	def := func(name string, arity int, f func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error)) {
		c.Define(name, arity, func(ctx object.Context, s object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return f(ctx, self(s), args, kw, block)
		})
	}
	lookup := func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		v, _, err := r.lookup(ctx, args)
		return v, err
	}
	def("[]", -1, lookup)
	def("field", -1, lookup)
	def("fetch", -1, func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		i, err := r.index(ctx, args[0], 0)
		if err != nil {
			return object.Value{}, err
		}
		if i >= 0 {
			return r.field(i), nil
		}
		if !block.IsZero() {
			return ctx.Yield(block, args[0])
		}
		if len(args) == 2 {
			return args[1], nil
		}
		inspect, _ := ctx.Inspect(args[0])
		return object.Value{}, &object.Exception{Cls: object.KeyError, Message: "key not found: " + strings.Trim(inspect, `"`),
			Attrs: map[string]object.Value{"key": args[0], "receiver": newRow(r.headers, r.fields, r.headerRow)}}
	})
	def("headers", 0, func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewList(r.allHeaders()...), nil
	})
	def("fields", -1, func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 {
			return object.NewList(r.fields...), nil
		}
		out := make([]object.Value, 0, len(args))
		for _, a := range args {
			v, _, err := r.lookup(ctx, []object.Value{a})
			if err != nil {
				return object.Value{}, err
			}
			out = append(out, v)
		}
		return object.NewList(out...), nil
	})
	c.Define("values_at", -1, c.Methods["fields"].Native)
	toH := func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return r.toHash(ctx)
	}
	def("to_h", 0, toH)
	def("to_hash", 0, toH)
	def("deconstruct_keys", 1, toH)
	def("to_a", 0, func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewList(r.pairs()...), nil
	})
	def("deconstruct", 0, func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewList(r.fields...), nil
	})
	toCSV := func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o := defaults()
		if err := o.read(ctx, kw); err != nil {
			return object.Value{}, err
		}
		return generateLine(ctx, &o, object.NewList(r.fields...))
	}
	def("to_csv", -1, toCSV)
	def("to_s", -1, toCSV)
	size := func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(r.size()), nil
	}
	def("size", 0, size)
	def("length", 0, size)
	hasHeader := func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
		}
		i, err := r.index(ctx, args[0], 0)
		return object.Bool(i >= 0), err
	}
	for _, name := range []string{"header?", "has_key?", "include?", "key?", "member?"} {
		def(name, 1, hasHeader)
	}
	def("field?", 1, func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for _, f := range r.fields {
			eq, err := ctx.Equal(f, args[0])
			if err != nil {
				return object.Value{}, err
			}
			if eq {
				return object.True, nil
			}
		}
		return object.False, nil
	})
	def("header_row?", 0, func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(r.headerRow), nil
	})
	def("field_row?", 0, func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(!r.headerRow), nil
	})
	def("index", -1, func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		from := 0
		if len(args) == 2 {
			n, err := stdlib.WantInt(ctx, args[1], "")
			if err != nil {
				return object.Value{}, err
			}
			from = int(n)
		}
		i, err := r.index(ctx, args[0], from)
		if err != nil || i < 0 {
			return object.Nil, err
		}
		return object.Int(i), nil
	})
	def("dig", -1, func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 {
			return object.Value{}, stdlib.ArgError(ctx, 0, "1+")
		}
		v, _, err := r.lookup(ctx, args[:1])
		if err != nil || len(args) == 1 || v.IsNil() {
			return v, err
		}
		return ctx.Call(v, "dig", args[1:]...)
	})
	each := func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		pairs := r.pairs()
		if block.IsZero() {
			recv := newRow(r.headers, r.fields, r.headerRow)
			return object.EnumeratorValue(&object.Enumerator{Recv: recv, Meth: "each", Each: func(ctx object.Context, yield func(object.Value) error) error {
				for _, p := range pairs {
					if err := yield(p); err != nil {
						return err
					}
				}
				return nil
			}}), nil
		}
		for _, p := range pairs {
			if _, err := ctx.Yield(block, p.List().Elems...); err != nil {
				return object.Value{}, err
			}
		}
		return newRow(r.headers, r.fields, r.headerRow), nil
	}
	def("each", 0, each)
	def("each_pair", 0, each)
	def("==", 1, func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if !isRow(args[0]) {
			return object.False, nil
		}
		other := rowOf(args[0])
		if r.size() != other.size() {
			return object.False, nil
		}
		for i := 0; i < r.size(); i++ {
			eqH, err := ctx.Equal(r.header(i), other.header(i))
			if err != nil {
				return object.Value{}, err
			}
			eqF, err := ctx.Equal(r.field(i), other.field(i))
			if err != nil {
				return object.Value{}, err
			}
			if !eqH || !eqF {
				return object.False, nil
			}
		}
		return object.True, nil
	})
	def("inspect", 0, func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := r.inspect(ctx)
		return object.String(s), err
	})
	// The changing methods return the copy, for the variable to take:
	// row = row << ["c", 3]; row["a"] = 5 rebinds row.
	def("[]=", -1, func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "2")
		}
		return r.with(ctx, args[0], args[1])
	})
	push := func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
		}
		return r.push(ctx, args[0])
	}
	def("<<", 1, push)
	def("push", 1, push)
	def("delete", -1, func(ctx object.Context, r *row, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		_, i, err := r.lookup(ctx, args)
		if err != nil || i < 0 {
			return newRow(r.headers, r.fields, r.headerRow), err
		}
		headers := append(append([]object.Value(nil), r.headers[:min(i, len(r.headers))]...), r.headers[min(i+1, len(r.headers)):]...)
		fields := append(append([]object.Value(nil), r.fields[:min(i, len(r.fields))]...), r.fields[min(i+1, len(r.fields)):]...)
		return newRow(headers, fields, r.headerRow), nil
	})
}
