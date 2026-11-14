package csv

import (
	"strconv"

	"github.com/cameo-lang/cameo/src/object"
)

// A CSV object, CSV.new(string), is a handle: it reads the string's rows
// one at a time with shift, and takes rows with << to add to the string.
// It is the one changing value of the library, as an IO is in Ruby.

type handle struct {
	p *parser
	w *writer
}

func handleOf(v object.Value) *handle { return v.Instance().Message.(*handle) }

func newHandle(ctx object.Context, input string, o *options) object.Value {
	oc := *o
	h := &handle{p: newParser(ctx, &oc, input), w: newWriter(ctx, &oc)}
	return object.InstanceValue(&object.Instance{Cls: csvClass, Message: h})
}

// shift is the next row, nil at the end.
func (h *handle) shift() (object.Value, error) { return h.p.shift() }

// readAll reads the rest: an Array of rows, or a Table with headers.
func (h *handle) readAll(ctx object.Context) (object.Value, error) {
	var rows []object.Value
	for {
		r, err := h.shift()
		if err != nil {
			return object.Value{}, err
		}
		if r.IsNil() {
			break
		}
		rows = append(rows, r)
	}
	if !h.p.o.headers.IsNil() {
		return newTable(rows, "col_or_row", h.p.headers), nil
	}
	return object.NewList(rows...), nil
}

func installHandle() {
	c := csvClass
	c.Include(object.Enumerable)
	def := func(name string, arity int, f func(ctx object.Context, h *handle, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error)) {
		c.Define(name, arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return f(ctx, handleOf(self), self, args, kw, block)
		})
	}
	shift := func(ctx object.Context, h *handle, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return h.shift()
	}
	def("shift", 0, shift)
	def("gets", 0, shift)
	def("readline", 0, shift)
	read := func(ctx object.Context, h *handle, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return h.readAll(ctx)
	}
	def("read", 0, read)
	def("readlines", 0, read)
	def("each", 0, func(ctx object.Context, h *handle, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return object.EnumeratorValue(&object.Enumerator{Recv: self, Meth: "each", Each: func(ctx object.Context, yield func(object.Value) error) error {
				for {
					r, err := h.shift()
					if err != nil {
						return err
					}
					if r.IsNil() {
						return nil
					}
					if err := yield(r); err != nil {
						return err
					}
				}
			}}), nil
		}
		for {
			r, err := h.shift()
			if err != nil {
				return object.Value{}, err
			}
			if r.IsNil() {
				return self, nil
			}
			if _, err := ctx.Yield(block, r); err != nil {
				return object.Value{}, err
			}
		}
	})
	def("lineno", 0, func(ctx object.Context, h *handle, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(h.p.rows), nil
	})
	def("line", 0, func(ctx object.Context, h *handle, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if h.p.rows == 0 {
			return object.Nil, nil
		}
		return object.NewString(h.p.line, object.UTF8), nil
	})
	eof := func(ctx object.Context, h *handle, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(h.p.eof()), nil
	}
	def("eof?", 0, eof)
	def("eof", 0, eof)
	def("rewind", 0, func(ctx object.Context, h *handle, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		h.p.reset()
		return object.Nil, nil
	})
	def("string", 0, func(ctx object.Context, h *handle, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewString(h.p.s+h.w.b.String(), object.UTF8), nil
	})
	def("headers", 0, func(ctx object.Context, h *handle, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if h.p.headersSet {
			return object.NewList(h.p.headers...), nil
		}
		return h.p.o.headers, nil
	})
	def("col_sep", 0, func(ctx object.Context, h *handle, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewString(h.p.o.colSep, object.UTF8), nil
	})
	def("row_sep", 0, func(ctx object.Context, h *handle, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewString(h.p.rowSep(), object.UTF8), nil
	})
	def("quote_char", 0, func(ctx object.Context, h *handle, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if h.p.o.quoteChar == "" {
			return object.Nil, nil
		}
		return object.NewString(h.p.o.quoteChar, object.UTF8), nil
	})
	add := func(ctx object.Context, h *handle, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for _, row := range args {
			if err := h.w.add(row); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	}
	def("<<", 1, add)
	def("add_row", 1, add)
	def("puts", -1, add)
	def("inspect", 0, func(ctx object.Context, h *handle, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		q := "nil"
		if h.p.o.quoteChar != "" {
			q = strconv.Quote(h.p.o.quoteChar)
		}
		return object.NewString("#<CSV io_type:StringIO encoding:UTF-8 lineno:"+strconv.Itoa(h.p.rows)+
				" col_sep:"+strconv.Quote(h.p.o.colSep)+" row_sep:"+strconv.Quote(h.p.rowSep())+" quote_char:"+q+">", object.UTF8),
			nil
	})
}
