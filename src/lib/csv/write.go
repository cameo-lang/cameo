package csv

import (
	"strings"

	"github.com/cameo-lang/cameo/src/object"
)

// writer turns rows into CSV text as CSV's writer does: a field is quoted
// when it holds the column separator, the quote, a line break, or is the
// empty string, or always with force_quotes.
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
		p := &parser{ctx: ctx, o: o, s: o.headers.Str()}
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
			w.headers, w.headersKnown = fields, true
		}
	}
	return w.line(fields)
}

// line writes the fields as one line.
func (w *writer) line(fields []object.Value) error {
	for i, f := range fields {
		if i > 0 {
			w.b.WriteString(w.o.colSep)
		}
		s, err := w.field(f)
		if err != nil {
			return err
		}
		w.b.WriteString(s)
	}
	w.b.WriteString(w.rowSep())
	return nil
}

// field is one field's text, quoted when it has to be.
func (w *writer) field(f object.Value) (string, error) {
	var s string
	switch {
	case f.IsNil():
		if !w.o.writeNil.IsZero() {
			s = w.o.writeNil.Str()
			break
		}
		if w.o.forceQuotes && w.o.quoteChar != "" {
			return w.o.quoteChar + w.o.quoteChar, nil
		}
		return "", nil
	case f.IsString() && f.Str() == "":
		if !w.o.writeEmpty.IsZero() {
			s = w.o.writeEmpty.Str()
			break
		}
		if (w.o.quoteEmpty || w.o.forceQuotes) && w.o.quoteChar != "" {
			return w.o.quoteChar + w.o.quoteChar, nil
		}
		return "", nil
	case f.IsString():
		s = f.Str()
	default:
		var err error
		if s, err = w.ctx.ToS(f); err != nil {
			return "", err
		}
	}
	q := w.o.quoteChar
	if q == "" {
		return s, nil
	}
	if w.o.forceQuotes || strings.Contains(s, q) || strings.Contains(s, w.o.colSep) || strings.ContainsAny(s, "\r\n") {
		return q + strings.ReplaceAll(s, q, q+q) + q, nil
	}
	return s, nil
}

// generateLine is CSV.generate_line: one row as text.
func generateLine(ctx object.Context, o *options, row object.Value) (object.Value, error) {
	w := newWriter(ctx, o)
	if err := w.add(row); err != nil {
		return object.Value{}, err
	}
	return object.String(w.b.String()), nil
}
