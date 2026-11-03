package csv

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
)

// parser reads the rows of a string one at a time, as CSV's parser does,
// with Ruby's rules for quoting and its errors.
type parser struct {
	ctx  object.Context
	o    *options
	s    string
	pos  int
	rows int // rows read so far; Ruby's line numbers count them
	// line is the last raw line read, as CSV#line.
	line string
	// headers is nil until the header row is known, when the options ask
	// for headers.
	headers    []object.Value
	headersSet bool
	// pendingHeaderRow holds the header row to return when return_headers
	// is on.
	pendingHeaderRow object.Value
}

// malformed is CSV::MalformedCSVError, with Ruby's message and line.
func (p *parser) malformed(problem string) error {
	line := p.rows + 1
	return &object.Exception{Cls: malformedError, Message: problem + " in line " + strconv.Itoa(line) + ".",
		Attrs: map[string]object.Value{"line_number": object.Int(line)}}
}

// rowSep is the row separator, found in the input when the options leave
// it to auto: the first line break, "\n" when there is none.
func (p *parser) rowSep() string {
	if p.o.rowSep != "" {
		return p.o.rowSep
	}
	for i := 0; i < len(p.s); i++ {
		switch p.s[i] {
		case '\r':
			if i+1 < len(p.s) && p.s[i+1] == '\n' {
				return "\r\n"
			}
			return "\r"
		case '\n':
			return "\n"
		}
	}
	return "\n"
}

// eof reports whether every row has been read.
func (p *parser) eof() bool { return p.pos >= len(p.s) }

// next reads the next row's raw fields: nil at the end of the input. An
// unquoted empty field is a nil Value, a quoted one the empty String.
func (p *parser) next() ([]object.Value, error) {
	sep := p.rowSep()
	for !p.eof() {
		start := p.pos
		fields, err := p.row(sep)
		if err != nil {
			return nil, err
		}
		end := p.pos
		p.line = p.s[start:end]
		p.rows++
		if p.o.skipLines != nil && p.o.skipLines.MatchAt(strings.TrimSuffix(p.line, sep), 0) != nil {
			continue
		}
		if p.o.skipBlanks && len(fields) == 0 {
			continue
		}
		return fields, nil
	}
	return nil, nil
}

// row reads one row from pos through its row separator.
func (p *parser) row(sep string) ([]object.Value, error) {
	fields := []object.Value{}
	if strings.HasPrefix(p.s[p.pos:], sep) {
		p.pos += len(sep)
		return fields, nil
	}
	for {
		if p.o.strip {
			p.skipStrip()
		}
		var field object.Value
		var err error
		if p.o.quoteChar != "" && strings.HasPrefix(p.s[p.pos:], p.o.quoteChar) {
			field, err = p.quoted(sep)
		} else {
			field, err = p.unquoted(sep)
		}
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
		switch {
		case strings.HasPrefix(p.s[p.pos:], p.o.colSep):
			p.pos += len(p.o.colSep)
			if p.eof() {
				fields = append(fields, p.o.nilField())
				return fields, nil
			}
		case strings.HasPrefix(p.s[p.pos:], sep):
			p.pos += len(sep)
			return fields, nil
		case p.eof():
			return fields, nil
		default:
			return nil, p.malformed("Any value after quoted field isn't allowed")
		}
	}
}

// skipStrip skips the spaces strip: drops around a field.
func (p *parser) skipStrip() {
	for !p.eof() && p.stripped(p.s[p.pos]) {
		p.pos++
	}
}

func (p *parser) stripped(c byte) bool {
	if p.o.stripChars != "" {
		return strings.IndexByte(p.o.stripChars, c) >= 0
	}
	return c == ' ' || c == '\t' || c == '\f' || c == '\v'
}

// nilField is what an empty unquoted field reads as.
func (o *options) nilField() object.Value {
	if !o.nilValue.IsZero() {
		return o.nilValue
	}
	return object.Nil
}

// emptyField is what a quoted empty field reads as.
func (o *options) emptyField() object.Value {
	if !o.emptyValue.IsZero() {
		return o.emptyValue
	}
	return object.String("")
}

// quoted reads a quoted field from its opening quote through its closing
// one; a doubled quote is one quote.
func (p *parser) quoted(sep string) (object.Value, error) {
	q := p.o.quoteChar
	p.pos += len(q)
	var b strings.Builder
	for {
		i := strings.Index(p.s[p.pos:], q)
		if i < 0 {
			return object.Value{}, p.malformed("Unclosed quoted field")
		}
		b.WriteString(p.s[p.pos : p.pos+i])
		p.pos += i + len(q)
		if strings.HasPrefix(p.s[p.pos:], q) {
			b.WriteString(q)
			p.pos += len(q)
			continue
		}
		break
	}
	if p.o.strip {
		p.skipStrip()
	}
	if b.Len() == 0 {
		return p.o.emptyField(), nil
	}
	return object.String(b.String()), nil
}

// unquoted reads a field up to the column or row separator, refusing a
// quote inside it and a line break that is not the row separator.
func (p *parser) unquoted(sep string) (object.Value, error) {
	start := p.pos
	for !p.eof() {
		rest := p.s[p.pos:]
		if strings.HasPrefix(rest, p.o.colSep) || strings.HasPrefix(rest, sep) {
			break
		}
		if p.o.quoteChar != "" && strings.HasPrefix(rest, p.o.quoteChar) && !p.o.liberal {
			return object.Value{}, p.malformed("Illegal quoting")
		}
		if c := rest[0]; c == '\r' || c == '\n' {
			return object.Value{}, p.malformed("Unquoted fields do not allow new line <" + strconv.Quote(string(c)) + ">")
		}
		p.pos++
	}
	s := p.s[start:p.pos]
	if p.o.strip {
		s = strings.TrimRightFunc(s, func(r rune) bool { return r < 128 && p.stripped(byte(r)) })
	}
	if s == "" {
		return p.o.nilField(), nil
	}
	return object.String(s), nil
}

var (
	dateMatcher     = regexp.MustCompile(`\A(?:(\w+,?\s+)?\w+\s+\d{1,2},?\s+\d{2,4}|\d{4}-\d{2}-\d{2})\z`)
	dateTimeMatcher = regexp.MustCompile(`\A(?:(\w+,?\s+)?\w+\s+\d{1,2}\s+\d{1,2}:\d{1,2}:\d{1,2},?\s+\d{2,4}|\d{4}-\d{2}-\d{2}\s\d{2}:\d{2}:\d{2}|\d{4}-\d{2}-\d{2}(?:T\d{2}:\d{2}(?::\d{2}(?:\.\d+)?(?:[+-]\d{2}(?::\d{2})|Z)?)?)?)\z`)
	nonWord         = regexp.MustCompile(`[^\s\w]+`)
	spaces          = regexp.MustCompile(`\s+`)
)

// fieldInfo is what a two-argument converter receives: CSV::FieldInfo.
func fieldInfo(index, line int, header object.Value, quoted bool) object.Value {
	return object.InstanceValue(&object.Instance{Cls: fieldInfoClass,
		Fields: []object.Value{object.Int(index), object.Int(line), header, object.Bool(quoted)}})
}

// convert runs the converters over a field, stopping once one has made it
// something other than a String, as CSV does.
func (p *parser) convert(field object.Value, convs []converter, index int, header object.Value, quoted bool) (object.Value, error) {
	for _, c := range convs {
		if !field.IsString() {
			break
		}
		var err error
		field, err = p.convertOne(field, c, index, header, quoted)
		if err != nil {
			return object.Value{}, err
		}
	}
	return field, nil
}

// kernel calls a Kernel function, Integer or Float, on a value.
func kernel(ctx object.Context, name string, v object.Value) (object.Value, error) {
	return ctx.Call(object.ClassValue(object.ObjectClass), name, v)
}

// classParse calls Class.parse on a field, the Date, DateTime or Time of
// the libraries, giving the field back when it fails as CSV's rescue
// does.
func classParse(ctx object.Context, class string, field object.Value) object.Value {
	cls, ok := object.ObjectClass.Consts[class]
	if !ok {
		return field
	}
	v, err := ctx.Call(cls, "parse", field)
	if err != nil {
		return field
	}
	return v
}

func (p *parser) convertOne(field object.Value, c converter, index int, header object.Value, quoted bool) (object.Value, error) {
	s := field.Str()
	switch c.name {
	case "integer":
		if v, err := kernel(p.ctx, "Integer", field); err == nil {
			return v, nil
		}
		return field, nil
	case "float":
		if v, err := kernel(p.ctx, "Float", field); err == nil {
			return v, nil
		}
		return field, nil
	case "date":
		if dateMatcher.MatchString(s) {
			return classParse(p.ctx, "Date", field), nil
		}
		return field, nil
	case "date_time":
		if dateTimeMatcher.MatchString(s) {
			return classParse(p.ctx, "DateTime", field), nil
		}
		return field, nil
	case "time":
		if dateTimeMatcher.MatchString(s) {
			return classParse(p.ctx, "Time", field), nil
		}
		return field, nil
	case "downcase":
		return object.String(strings.ToLower(s)), nil
	case "symbol":
		t := strings.ToLower(s)
		t = nonWord.ReplaceAllString(t, "")
		t = strings.TrimSpace(t)
		t = spaces.ReplaceAllString(t, "_")
		return object.Symbol(t), nil
	case "symbol_raw":
		return object.Symbol(s), nil
	}
	arity, err := p.ctx.Call(c.proc, "arity")
	if err != nil {
		return object.Value{}, err
	}
	if n, _ := arity.AsInt(); n == 1 {
		return p.ctx.Yield(c.proc, field)
	}
	return p.ctx.Yield(c.proc, field, fieldInfo(index, p.rows, header, quoted))
}

// headerRow turns the header row's fields into headers through the header
// converters.
func (p *parser) convertHeaders(fields []object.Value) ([]object.Value, error) {
	out := make([]object.Value, len(fields))
	for i, f := range fields {
		h, err := p.convert(f, p.o.headerConverters, i, object.Nil, false)
		if err != nil {
			return nil, err
		}
		out[i] = h
	}
	return out, nil
}

// shift is CSV#shift: the next row as an Array, or, with headers, as a
// CSV::Row; nil at the end.
func (p *parser) shift() (object.Value, error) {
	if !p.pendingHeaderRow.IsZero() {
		row := p.pendingHeaderRow
		p.pendingHeaderRow = object.Value{}
		return row, nil
	}
	if !p.o.headers.IsNil() && !p.headersSet {
		if err := p.readHeaders(); err != nil {
			return object.Value{}, err
		}
		if !p.pendingHeaderRow.IsZero() {
			return p.shift()
		}
	}
	fields, err := p.next()
	if err != nil {
		return object.Value{}, err
	}
	if fields == nil {
		return object.Nil, nil
	}
	if p.headersSet {
		return p.dataRow(fields)
	}
	for i, f := range fields {
		if fields[i], err = p.convert(f, p.o.converters, i, object.Nil, false); err != nil {
			return object.Value{}, err
		}
	}
	return object.NewList(fields...), nil
}

// readHeaders takes the headers from the options or from the first row.
func (p *parser) readHeaders() error {
	var raw []object.Value
	switch {
	case p.o.headers.Kind() == object.KindBool:
		fields, err := p.next()
		if err != nil {
			return err
		}
		if fields == nil {
			return nil
		}
		raw = fields
	case p.o.headers.IsString():
		sub := &parser{ctx: p.ctx, o: p.o, s: p.o.headers.Str()}
		fields, err := sub.next()
		if err != nil {
			return err
		}
		raw = fields
	default:
		list, ok := p.o.headers.AsList()
		if !ok {
			return p.ctx.Raise(object.ArgumentError, ":headers must be true, an Array or a String")
		}
		raw = list.Elems
	}
	headers, err := p.convertHeaders(raw)
	if err != nil {
		return err
	}
	p.headers, p.headersSet = headers, true
	if p.o.returnHeaders {
		p.pendingHeaderRow = newRow(headers, raw, true)
	}
	return nil
}

// dataRow makes the CSV::Row of a row's fields under the headers.
func (p *parser) dataRow(fields []object.Value) (object.Value, error) {
	converted := make([]object.Value, len(fields))
	for i, f := range fields {
		var header object.Value = object.Nil
		if i < len(p.headers) {
			header = p.headers[i]
		}
		v, err := p.convert(f, p.o.converters, i, header, false)
		if err != nil {
			return object.Value{}, err
		}
		converted[i] = v
	}
	return newRow(p.headers, converted, false), nil
}
