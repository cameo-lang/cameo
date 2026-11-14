package csv

import (
	"encoding/csv"
	"github.com/cameo-lang/cameo/src/str"
	"strings"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/object"
)

// writer turns rows into CSV text with Go's encoding/csv writer.
type writer struct {
	ctx object.Context
	o   *options
	b   strings.Builder
	// headers are the row's headers when the options give them, or the
	// first row written with headers: true; a Hash row is laid out by them.
	headers      []object.Value
	headersKnown bool
	wroteHeaders bool
}

func newWriter(ctx object.Context, o *options) *writer {
	w := &writer{ctx: ctx, o: o}
	if list, ok := o.headers.AsList(); ok {
		w.headers, w.headersKnown = list.Elems, true
	} else if o.headers.IsString() {
		p := newParser(ctx, o, o.headers.Str())
		if fields, err := p.next(); err == nil {
			w.headers, w.headersKnown = fields, true
		}
	}
	return w
}

func (w *writer) rowSep() string {
	if w.o.rowSep == "" {
		return "\n"
	}
	return w.o.rowSep
}

// add writes one row: an Array, a CSV::Row, or a Hash laid out by the
// headers.
func (w *writer) add(row object.Value) error {
	if w.headersKnown && w.o.writeHeaders && !w.wroteHeaders {
		w.wroteHeaders = true
		if err := w.line(w.headers); err != nil {
			return err
		}
	}
	var fields []object.Value
	switch {
	case isRow(row):
		fields = rowOf(row).fields
	case row.Kind() == object.KindMap:
		if !w.headersKnown {
			return w.ctx.Raise(object.ArgumentError, "a Hash row needs headers")
		}
		m := row.Map()
		fields = make([]object.Value, len(w.headers))
		for i, h := range w.headers {
			v, ok := m.Get(h)
			if !ok {
				v = object.Nil
			}
			fields[i] = v
		}
	default:
		list, ok := row.AsList()
		if !ok {
			return w.ctx.Raise(object.TypeError, "a row is an Array, a Hash or a CSV::Row, not %s", kindName(row))
		}
		fields = list.Elems
		if !w.headersKnown && w.o.headers.Kind() == object.KindBool && w.o.headers.Truthy() {
			// The first row is the headers, and the header line when
			// write_headers asks for one: it is written once.
			w.headers, w.headersKnown = fields, true
			w.wroteHeaders = w.o.writeHeaders
		}
	}
	return w.line(fields)
}

// line writes the fields as one line through Go's encoding/csv writer,
// which quotes a field that holds the separator, a quote or a line break.
func (w *writer) line(fields []object.Value) error {
	strs := make([]string, len(fields))
	for i, f := range fields {
		s, err := w.field(f)
		if err != nil {
			return err
		}
		strs[i] = s
	}
	cw := csv.NewWriter(&w.b)
	cw.Comma, _ = utf8.DecodeRuneInString(w.o.colSep)
	cw.UseCRLF = w.rowSep() == "\r\n"
	if err := cw.Write(strs); err != nil {
		return w.ctx.Raise(object.ArgumentError, "%s", err.Error())
	}
	cw.Flush()
	return cw.Error()
}

// field is one field's text: nil and the empty String as the options say.
func (w *writer) field(f object.Value) (string, error) {
	switch {
	case f.IsNil():
		if !w.o.writeNil.IsZero() {
			return w.ctx.ToS(w.o.writeNil)
		}
		return "", nil
	case f.IsString() && f.Str() == "":
		if !w.o.writeEmpty.IsZero() {
			return w.ctx.ToS(w.o.writeEmpty)
		}
		return "", nil
	case f.IsString():
		return f.Str(), nil
	}
	return w.ctx.ToS(f)
}

// generateLine is CSV.generate_line: one row as text.
func generateLine(ctx object.Context, o *options, row object.Value) (object.Value, error) {
	w := newWriter(ctx, o)
	if err := w.add(row); err != nil {
		return object.Value{}, err
	}
	return object.NewString(w.b.String(), str.UTF8), nil
}
