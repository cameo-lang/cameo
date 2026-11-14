package csv

import (
	"github.com/cameo-lang/cameo/src/str"
	"strconv"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

// CSV::Table is the rows of a parse with headers, read by row, by column,
// or either. It is a value: by_col and by_row return a copy in that mode,
// << and delete return the changed copy.

// table is the Go side of a CSV::Table.
type table struct {
	rows []object.Value // CSV::Row values
	// mode is "col_or_row", "col" or "row".
	mode string
	// headers are the table's when it has no rows.
	headers []object.Value
}

func newTable(rows []object.Value, mode string, headers []object.Value) object.Value {
	return object.InstanceValue(&object.Instance{Cls: tableClass, Message: &table{rows: rows, mode: mode, headers: headers}})
}

func isTable(v object.Value) bool {
	inst, ok := v.AsInstance()
	return ok && inst.Cls == tableClass
}

func tableOf(v object.Value) *table { return v.Instance().Message.(*table) }

func (t *table) copy() object.Value { return newTable(t.rows, t.mode, t.headers) }

// headersOf is Table#headers: the first row's, or the ones given.
func (t *table) headersOf() []object.Value {
	if len(t.rows) > 0 {
		return rowOf(t.rows[0]).allHeaders()
	}
	return t.headers
}

// column is the fields under a header, or at an index, down the rows.
func (t *table) column(ctx object.Context, key object.Value) (object.Value, error) {
	out := make([]object.Value, 0, len(t.rows))
	for _, r := range t.rows {
		v, _, err := rowOf(r).lookup(ctx, []object.Value{key})
		if err != nil {
			return object.Value{}, err
		}
		out = append(out, v)
	}
	return object.NewList(out...), nil
}

// rowAt is a row by index, nil when out of range.
func (t *table) rowAt(i int64) object.Value {
	if i < 0 {
		i += int64(len(t.rows))
	}
	if i < 0 || int(i) >= len(t.rows) {
		return object.Nil
	}
	return t.rows[i]
}

// byRow says whether an index reads a row in the table's mode.
func (t *table) byRow(key object.Value) bool {
	if t.mode == "row" {
		return true
	}
	if t.mode == "col" {
		return false
	}
	_, isInt := key.AsInt()
	_, isRange := key.AsRange()
	return isInt || isRange
}

// lookup is Table#[].
func (t *table) lookup(ctx object.Context, key object.Value) (object.Value, error) {
	if t.byRow(key) {
		if r, ok := key.AsRange(); ok {
			return ctx.Call(object.NewList(t.rows...), "[]", object.RangeValue(r))
		}
		i, err := stdlib.WantInt(ctx, key, "")
		if err != nil {
			return object.Value{}, err
		}
		return t.rowAt(i), nil
	}
	return t.column(ctx, key)
}

// with is Table#[]=: by row, a Row or an Array of fields under the
// table's headers at an index, the one past the last appending; by
// column, an Array sets each row's field under the header, or at the
// index, and any other value sets every row's.
func (t *table) with(ctx object.Context, key, value object.Value) (object.Value, error) {
	rows := append([]object.Value(nil), t.rows...)
	if t.byRow(key) {
		i, err := stdlib.WantInt(ctx, key, "")
		if err != nil {
			return object.Value{}, err
		}
		if i < 0 {
			i += int64(len(rows))
		}
		if i < 0 || int(i) > len(rows) {
			return object.Value{}, ctx.Raise(object.IndexError, "index %d outside of table bounds: %d...%d", i, -len(rows), len(rows))
		}
		r := value
		if list, ok := value.AsList(); ok {
			r = newRow(t.headersOf(), list.Elems, false)
		} else if !isRow(value) {
			return object.Value{}, ctx.Raise(object.TypeError, "a row is an Array or a CSV::Row, not %s", kindName(value))
		}
		if int(i) == len(rows) {
			return newTable(append(rows, r), t.mode, t.headers), nil
		}
		rows[i] = r
		return newTable(rows, t.mode, t.headers), nil
	}
	list, isList := value.AsList()
	for i, r := range rows {
		v := value
		if isList {
			v = object.Nil
			if i < len(list.Elems) {
				v = list.Elems[i]
			}
		}
		out, err := rowOf(r).with(ctx, key, v)
		if err != nil {
			return object.Value{}, err
		}
		rows[i] = out
	}
	return newTable(rows, t.mode, t.headers), nil
}

// toArray is Table#to_a: the headers, then each row's fields, header rows
// left out.
func (t *table) toArray() []object.Value {
	out := []object.Value{object.NewList(t.headersOf()...)}
	for _, r := range t.rows {
		if rowOf(r).headerRow {
			continue
		}
		out = append(out, object.NewList(rowOf(r).fields...))
	}
	return out
}

// toCSV is Table#to_csv: the headers unless write_headers: false, then
// the first limit: rows, or all of them.
func (t *table) toCSV(ctx object.Context, kw *object.Map) (object.Value, error) {
	o := defaults()
	o.writeHeaders = true
	limit := -1
	if kw != nil {
		if v, ok := kw.Get(object.Symbol("limit")); ok {
			n, err := stdlib.WantInt(ctx, v, "")
			if err != nil {
				return object.Value{}, err
			}
			limit = int(n)
			kw = kw.Without(object.Symbol("limit"))
		}
	}
	if err := o.read(ctx, kw); err != nil {
		return object.Value{}, err
	}
	writeHeaders := o.writeHeaders
	if kw != nil {
		if v, ok := kw.Get(object.Symbol("write_headers")); ok {
			writeHeaders = v.Truthy()
		}
	}
	o.writeHeaders = false
	w := newWriter(ctx, &o)
	rows := t.toArray()
	if limit >= 0 && limit < len(rows)-1 {
		rows = rows[:limit+1]
	}
	if !writeHeaders {
		rows = rows[1:]
	}
	for _, r := range rows {
		if err := w.line(r.List().Elems); err != nil {
			return object.Value{}, err
		}
	}
	return object.NewString(w.b.String(), str.UTF8), nil
}

func installTable() {
	c := tableClass
	c.Lexical = module
	module.Consts["Table"] = object.ClassValue(c)
	c.Include(object.Enumerable)

	c.DefineSingleton("new", -1, func(ctx object.Context, s object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
		}
		rows, err := stdlib.ToList(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		for _, r := range rows {
			if !isRow(r) {
				return object.Value{}, ctx.Raise(object.TypeError, "a CSV::Table holds CSV::Row values, not %s", kindName(r))
			}
		}
		var headers []object.Value
		if kw != nil {
			if v, ok := kw.Get(object.Symbol("headers")); ok {
				if headers, err = stdlib.ToList(ctx, v); err != nil {
					return object.Value{}, err
				}
			}
		}
		return newTable(rows, "col_or_row", headers), nil
	})
	def := func(name string, arity int, f func(ctx object.Context, t *table, args []object.Value, kw *object.Map, block object.Value) (object.Value, error)) {
		c.Define(name, arity, func(ctx object.Context, s object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return f(ctx, tableOf(s), args, kw, block)
		})
	}
	def("headers", 0, func(ctx object.Context, t *table, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewList(t.headersOf()...), nil
	})
	def("mode", 0, func(ctx object.Context, t *table, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Symbol(t.mode), nil
	})
	for name, mode := range map[string]string{"by_col": "col", "by_row": "row", "by_col_or_row": "col_or_row"} {
		mode := mode
		def(name, 0, func(ctx object.Context, t *table, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return newTable(t.rows, mode, t.headers), nil
		})
	}
	def("[]", 1, func(ctx object.Context, t *table, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return t.lookup(ctx, args[0])
	})
	def("[]=", 2, func(ctx object.Context, t *table, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return t.with(ctx, args[0], args[1])
	})
	def("values_at", -1, func(ctx object.Context, t *table, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 0 && t.byRow(args[0]) {
			return ctx.Call(object.NewList(t.rows...), "values_at", args...)
		}
		out := make([]object.Value, len(t.rows))
		for i, r := range t.rows {
			v, err := ctx.Call(r, "values_at", args...)
			if err != nil {
				return object.Value{}, err
			}
			out[i] = v
		}
		return object.NewList(out...), nil
	})
	def("dig", -1, func(ctx object.Context, t *table, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 {
			return object.Value{}, stdlib.ArgError(ctx, 0, "1+")
		}
		v, err := t.lookup(ctx, args[0])
		if err != nil || len(args) == 1 || v.IsNil() {
			return v, err
		}
		return ctx.Call(v, "dig", args[1:]...)
	})
	size := func(ctx object.Context, t *table, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(len(t.rows)), nil
	}
	def("size", 0, size)
	def("length", 0, size)
	def("to_a", 0, func(ctx object.Context, t *table, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewList(t.toArray()...), nil
	})
	toCSV := func(ctx object.Context, t *table, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return t.toCSV(ctx, kw)
	}
	def("to_csv", -1, toCSV)
	def("to_s", -1, toCSV)
	// inspect shows the first five rows, as Ruby's to_csv(limit: 5) does.
	def("inspect", 0, func(ctx object.Context, t *table, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		body, err := t.toCSV(ctx, object.NewMap().Put(object.Symbol("limit"), object.Int(5)))
		if err != nil {
			return object.Value{}, err
		}
		return object.NewString("#<CSV::Table mode:"+t.mode+" row_count:"+strconv.Itoa(len(t.toArray()))+">\n"+body.Str(), str.UTF8), nil
	})
	def("each", 0, func(ctx object.Context, t *table, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		var items []object.Value
		if t.mode == "col" {
			for _, h := range t.headersOf() {
				col, err := t.column(ctx, h)
				if err != nil {
					return object.Value{}, err
				}
				items = append(items, object.NewList(h, col))
			}
		} else {
			items = t.rows
		}
		if block.IsZero() {
			return object.EnumeratorValue(&object.Enumerator{Recv: t.copy(), Meth: "each", Each: func(ctx object.Context, yield func(object.Value) error) error {
				for _, it := range items {
					if err := yield(it); err != nil {
						return err
					}
				}
				return nil
			}}), nil
		}
		for _, it := range items {
			var err error
			if t.mode == "col" {
				_, err = ctx.Yield(block, it.List().Elems...)
			} else {
				_, err = ctx.Yield(block, it)
			}
			if err != nil {
				return object.Value{}, err
			}
		}
		return t.copy(), nil
	})
	def("==", 1, func(ctx object.Context, t *table, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if !isTable(args[0]) {
			return object.False, nil
		}
		o := tableOf(args[0])
		if len(o.rows) != len(t.rows) {
			return object.False, nil
		}
		for i := range t.rows {
			eq, err := ctx.Equal(t.rows[i], o.rows[i])
			if err != nil || !eq {
				return object.False, err
			}
		}
		return object.True, nil
	})
	// << and push return the table with the row added; delete returns it
	// without a row or a column. This deviates from Ruby's behaviour: Ruby's
	// delete returns what it removed, cameo's returns the table.
	push := func(ctx object.Context, t *table, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		rows := append([]object.Value(nil), t.rows...)
		for _, a := range args {
			switch {
			case isRow(a):
				rows = append(rows, a)
			case a.Kind() == object.KindMap:
				m := a.Map()
				fields := make([]object.Value, len(t.headersOf()))
				for i, h := range t.headersOf() {
					v, ok := m.Get(h)
					if !ok {
						v = object.Nil
					}
					fields[i] = v
				}
				rows = append(rows, newRow(t.headersOf(), fields, false))
			default:
				fields, err := stdlib.ToList(ctx, a)
				if err != nil {
					return object.Value{}, err
				}
				rows = append(rows, newRow(t.headersOf(), fields, false))
			}
		}
		return newTable(rows, t.mode, t.headers), nil
	}
	def("<<", 1, push)
	def("push", -1, push)
	def("delete", -1, func(ctx object.Context, t *table, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		rows := t.rows
		headers := t.headers
		for _, key := range args {
			if t.byRow(key) {
				i, err := stdlib.WantInt(ctx, key, "")
				if err != nil {
					return object.Value{}, err
				}
				if i < 0 {
					i += int64(len(rows))
				}
				if i >= 0 && int(i) < len(rows) {
					rows = append(append([]object.Value(nil), rows[:i]...), rows[i+1:]...)
				}
				continue
			}
			out := make([]object.Value, len(rows))
			for j, r := range rows {
				v, err := ctx.Call(r, "delete", key)
				if err != nil {
					return object.Value{}, err
				}
				out[j] = v
			}
			rows = out
			if len(rows) == 0 {
				kept := []object.Value{}
				for _, h := range headers {
					eq, err := ctx.Equal(h, key)
					if err != nil {
						return object.Value{}, err
					}
					if !eq {
						kept = append(kept, h)
					}
				}
				headers = kept
			}
		}
		return newTable(rows, t.mode, headers), nil
	})
	def("delete_if", 0, func(ctx object.Context, t *table, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return object.Value{}, ctx.Raise(object.LocalJumpError, "no block given (yield)")
		}
		kept := []object.Value{}
		for _, r := range t.rows {
			drop, err := ctx.Yield(block, r)
			if err != nil {
				return object.Value{}, err
			}
			if !drop.Truthy() {
				kept = append(kept, r)
			}
		}
		return newTable(kept, t.mode, t.headers), nil
	})
}
