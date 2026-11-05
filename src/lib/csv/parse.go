package csv

import (
	"encoding/csv"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/object"
)

// parser reads the rows of a string one at a time with Go's encoding/csv,
// and gives them Ruby's shape: an unquoted empty field is nil, a quoted
// one the empty String, an empty line is an empty row, and a bad line is
// CSV::MalformedCSVError with Ruby's words.
type parser struct {
	ctx object.Context
	o   *options
	s   string
	r   *csv.Reader
	// lineStarts are the byte offsets where each line of s starts, so a
	// field's position from the reader becomes an offset into s.
	lineStarts []int
	offset     int // what the reader has consumed
	lineNo     int // lines wholly consumed
	done       bool
	// pendingEmpty counts the empty rows the reader skipped before stash,
	// or at the end; they come out first.
	pendingEmpty int
	stash        []object.Value
	stashLine    string
	stashQuoted  []bool
	hasStash     bool
	// quoted says, for each field of the last row read, whether it was
	// quoted, which CSV::FieldInfo#quoted? tells a converter.
	quoted []bool
	rows   int // rows read so far; Ruby's line numbers count them
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

func newParser(ctx object.Context, o *options, s string) *parser {
	if o.strip {
		s = stripAroundQuotes(s, o.stripped, o.colSep[0])
	}
	p := &parser{ctx: ctx, o: o, s: s}
	p.lineStarts = []int{0}
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			p.lineStarts = append(p.lineStarts, i+1)
		}
	}
	p.reset()
	return p
}

// reset starts reading from the top again, as CSV#rewind does.
func (p *parser) reset() {
	r := csv.NewReader(strings.NewReader(p.s))
	r.Comma, _ = utf8.DecodeRuneInString(p.o.colSep)
	r.LazyQuotes = p.o.liberal
	r.FieldsPerRecord = -1
	p.r = r
	p.offset, p.lineNo, p.rows, p.pendingEmpty = 0, 0, 0, 0
	p.done, p.hasStash = false, false
	p.line, p.headers, p.headersSet = "", nil, false
	p.pendingHeaderRow = object.Value{}
}

// malformed is CSV::MalformedCSVError, with Ruby's message and line: the
// rows read so far, the empty lines the reader skipped before the bad
// one counting as rows, and one.
func (p *parser) malformed(problem string, skipped int) error {
	line := p.rows + skipped + 1
	return &object.Exception{Cls: malformedError, Message: problem + " in line " + strconv.Itoa(line) + ".",
		Attrs: map[string]object.Value{"line_number": object.Int(line)}}
}

// translate gives a reader's error Ruby's words.
func (p *parser) translate(err error) error {
	var pe *csv.ParseError
	if !errors.As(err, &pe) {
		return p.malformed(err.Error(), 0)
	}
	skipped := max(pe.StartLine-p.lineNo-1, 0)
	switch {
	case errors.Is(pe.Err, csv.ErrBareQuote):
		return p.malformed("Illegal quoting", skipped)
	case errors.Is(pe.Err, csv.ErrQuote):
		if strings.Count(p.s[p.offset:], `"`)%2 == 1 {
			return p.malformed("Unclosed quoted field", skipped)
		}
		return p.malformed("Any value after quoted field isn't allowed", skipped)
	}
	return p.malformed(pe.Err.Error(), skipped)
}

// rowSep is the row separator CSV#row_sep reports: the first line break
// of the input when the options leave it to auto, "\n" when there is none.
func (p *parser) rowSep() string {
	if p.o.rowSep != "" {
		return p.o.rowSep
	}
	if i := strings.IndexByte(p.s, '\n'); i > 0 && p.s[i-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}

// eof reports whether every row has been read.
func (p *parser) eof() bool {
	if p.pendingEmpty > 0 || p.hasStash {
		return false
	}
	return p.done || p.offset >= len(p.s)
}

// next reads the next row's raw fields: nil at the end of the input. An
// unquoted empty field is a nil Value, a quoted one the empty String.
func (p *parser) next() ([]object.Value, error) {
	for {
		fields, raw, quoted, err := p.record()
		if err != nil {
			return nil, err
		}
		if fields == nil {
			return nil, nil
		}
		p.line, p.quoted = raw, quoted
		p.rows++
		if p.o.skipLines != nil && p.o.skipLines.MatchAt(strings.TrimRight(raw, "\r\n"), 0) != nil {
			continue
		}
		if p.o.skipBlanks && len(fields) == 0 {
			continue
		}
		return fields, nil
	}
}

// record is the next row with its raw text and which of its fields were
// quoted: the empty rows the reader skipped first, then the record it
// read, nil at the end.
func (p *parser) record() ([]object.Value, string, []bool, error) {
	if p.pendingEmpty > 0 {
		p.pendingEmpty--
		return []object.Value{}, "", nil, nil
	}
	if p.hasStash {
		p.hasStash = false
		return p.stash, p.stashLine, p.stashQuoted, nil
	}
	if p.done {
		return nil, "", nil, nil
	}
	rec, err := p.r.Read()
	if err == io.EOF {
		p.done = true
		// The reader skips empty lines; each line break left over is one.
		tail := p.s[p.offset:]
		p.pendingEmpty = strings.Count(tail, "\n")
		if p.pendingEmpty == 0 && strings.Count(tail, "\r") > 0 {
			p.pendingEmpty = strings.Count(tail, "\r")
		}
		p.offset = len(p.s)
		return p.record()
	}
	if err != nil {
		return nil, "", nil, p.translate(err)
	}
	line, _ := p.r.FieldPos(0)
	skipped := line - p.lineNo - 1
	start := p.offset
	for i := 0; i < skipped; i++ {
		if j := strings.IndexByte(p.s[start:], '\n'); j >= 0 {
			start += j + 1
		}
	}
	end := int(p.r.InputOffset())
	raw := p.s[start:end]
	p.offset = end
	p.lineNo = strings.Count(p.s[:end], "\n")
	fields := make([]object.Value, len(rec))
	quoted := make([]bool, len(rec))
	for i, f := range rec {
		fl, fc := p.r.FieldPos(i)
		at := p.lineStarts[fl-1] + fc - 1
		quoted[i] = at < len(p.s) && p.s[at] == '"'
		if p.o.strip && !quoted[i] {
			f = strings.TrimFunc(f, func(r rune) bool { return r < 128 && p.o.stripped(byte(r)) })
		}
		switch {
		case f == "" && quoted[i]:
			fields[i] = p.o.emptyField()
		case f == "":
			fields[i] = p.o.nilField()
		default:
			fields[i] = object.String(f)
		}
	}
	if skipped > 0 {
		p.pendingEmpty = skipped
		p.stash, p.stashLine, p.stashQuoted, p.hasStash = fields, raw, quoted, true
		return p.record()
	}
	return fields, raw, quoted, nil
}

// stripAroundQuotes takes the characters strip: names away from around a
// quoted field, which Ruby's parser strips and Go's reader refuses: the
// ones before its opening quote and after its closing one, up to the
// separator or the line break. An unquoted field is stripped once read.
func stripAroundQuotes(s string, stripped func(byte) bool, sep byte) string {
	var b strings.Builder
	ends := func(c byte) bool { return c == sep || c == '\n' || c == '\r' }
	skip := func(i int) int {
		for i < len(s) && stripped(s[i]) && !ends(s[i]) {
			i++
		}
		return i
	}
	for i := 0; i < len(s); {
		if j := skip(i); j < len(s) && s[j] == '"' {
			b.WriteByte('"')
			j++
			for j < len(s) {
				if s[j] == '"' && (j+1 >= len(s) || s[j+1] != '"') {
					break
				}
				if s[j] == '"' {
					b.WriteString(`""`)
					j += 2
					continue
				}
				b.WriteByte(s[j])
				j++
			}
			if j < len(s) {
				b.WriteByte('"')
				j++
			}
			i = skip(j)
			continue
		}
		for i < len(s) && !ends(s[i]) {
			b.WriteByte(s[i])
			i++
		}
		if i < len(s) {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// stripped says whether strip: takes a character away: the ones it
// names, or whitespace.
func (o *options) stripped(c byte) bool {
	if o.stripChars != "" {
		return strings.IndexByte(o.stripChars, c) >= 0
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

// convertHeaders turns the header row's fields into headers through the
// header converters.
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
		if fields[i], err = p.convert(f, p.o.converters, i, object.Nil, p.quoted[i]); err != nil {
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
		sub := newParser(p.ctx, p.o, p.o.headers.Str())
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
		v, err := p.convert(f, p.o.converters, i, header, p.quoted[i])
		if err != nil {
			return object.Value{}, err
		}
		converted[i] = v
	}
	return newRow(p.headers, converted, false), nil
}
