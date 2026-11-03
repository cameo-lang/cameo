package csv

import (
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

// options are CSV's keywords, for parsing and for writing.
type options struct {
	colSep string
	// rowSep is "" for :auto: the first line break of the input when
	// parsing, "\n" when writing.
	rowSep string
	// quoteChar is "" for quote_char: nil, which turns quoting off.
	quoteChar string
	// headers is nil (none), true, an Array of headers or a String to
	// parse them from.
	headers          object.Value
	returnHeaders    bool
	writeHeaders     bool
	converters       []converter
	headerConverters []converter
	skipBlanks       bool
	skipLines        *object.Regexp
	liberal          bool
	strip            bool
	stripChars       string
	nilValue         object.Value
	emptyValue       object.Value
	writeNil         object.Value
	writeEmpty       object.Value
	forceQuotes      bool
	quoteEmpty       bool
}

// converter is a field or header converter: one of CSV's by name, or a
// Proc of the script's.
type converter struct {
	name string
	proc object.Value
}

func defaults() options {
	return options{colSep: ",", quoteChar: `"`, headers: object.Nil, quoteEmpty: true}
}

// keywords are what the options take; others are Ruby's ArgumentError.
var keywords = map[string]bool{
	"col_sep": true, "row_sep": true, "quote_char": true, "headers": true, "return_headers": true,
	"write_headers": true, "header_converters": true, "converters": true, "skip_blanks": true,
	"skip_lines": true, "liberal_parsing": true, "strip": true, "nil_value": true, "empty_value": true,
	"write_nil_value": true, "write_empty_value": true, "force_quotes": true, "quote_empty": true,
	"field_size_limit": true, "max_field_size": true, "unconverted_fields": true, "encoding": true,
	"write_converters": true, "strict_duplicate_headers": true,
}

// read takes the keywords of a call.
func (o *options) read(ctx object.Context, kw *object.Map) error {
	if kw == nil {
		return nil
	}
	for i, k := range kw.Keys() {
		name, _ := k.AsSymbol()
		if !keywords[name] {
			inspect, _ := ctx.Inspect(k)
			return ctx.Raise(object.ArgumentError, "unknown keyword: %s", inspect)
		}
		v := kw.Values()[i]
		switch name {
		case "col_sep":
			s, err := stdlib.WantString(ctx, v)
			if err != nil {
				return err
			}
			if s == "" {
				return ctx.Raise(object.ArgumentError, ":col_sep must be 1 or more characters: \"\"")
			}
			o.colSep = s
		case "row_sep":
			if sym, ok := v.AsSymbol(); ok && sym == "auto" {
				o.rowSep = ""
				continue
			}
			s, err := stdlib.WantString(ctx, v)
			if err != nil {
				return err
			}
			o.rowSep = s
		case "quote_char":
			if v.IsNil() {
				o.quoteChar = ""
				continue
			}
			s, err := stdlib.WantString(ctx, v)
			if err != nil {
				return err
			}
			if len([]rune(s)) != 1 {
				return ctx.Raise(object.ArgumentError, ":quote_char has to be nil or a single character String")
			}
			o.quoteChar = s
		case "headers":
			o.headers = v
		case "return_headers":
			o.returnHeaders = v.Truthy()
		case "write_headers":
			o.writeHeaders = v.Truthy()
		case "converters":
			list, err := convertersOf(ctx, v, builtinConverters)
			if err != nil {
				return err
			}
			o.converters = append(o.converters, list...)
		case "header_converters":
			list, err := convertersOf(ctx, v, builtinHeaderConverters)
			if err != nil {
				return err
			}
			o.headerConverters = append(o.headerConverters, list...)
		case "skip_blanks":
			o.skipBlanks = v.Truthy()
		case "skip_lines":
			if v.IsNil() {
				continue
			}
			if re, ok := v.AsRegexp(); ok {
				o.skipLines = re
				continue
			}
			s, err := stdlib.WantString(ctx, v)
			if err != nil {
				return err
			}
			re, err := object.NewRegexp(regexpQuote(s), 0)
			if err != nil {
				return ctx.Raise(object.RegexpError, "%s", err.Error())
			}
			o.skipLines = re
		case "liberal_parsing":
			o.liberal = v.Truthy()
		case "strip":
			if s, ok := v.AsString(); ok {
				o.strip, o.stripChars = true, s
			} else {
				o.strip = v.Truthy()
			}
		case "nil_value":
			o.nilValue = v
		case "empty_value":
			o.emptyValue = v
		case "write_nil_value":
			o.writeNil = v
		case "write_empty_value":
			o.writeEmpty = v
		case "force_quotes":
			o.forceQuotes = v.Truthy()
		case "quote_empty":
			o.quoteEmpty = v.Truthy()
		}
	}
	return nil
}

// regexpQuote escapes s for a Regexp that matches it as text.
func regexpQuote(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(`\.+*?()|[]{}^$`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// convertersOf reads a converter option: a name, a Proc, or an Array of
// them.
func convertersOf(ctx object.Context, v object.Value, known map[string][]string) ([]converter, error) {
	if v.IsNil() {
		return nil, nil
	}
	var items []object.Value
	if list, ok := v.AsList(); ok {
		items = list.Elems
	} else {
		items = []object.Value{v}
	}
	var out []converter
	for _, item := range items {
		if name, ok := item.AsSymbol(); ok {
			names, ok := known[name]
			if !ok {
				return nil, ctx.Raise(object.ArgumentError, "unknown converter name: :%s", name)
			}
			for _, n := range names {
				out = append(out, converter{name: n})
			}
			continue
		}
		if name, ok := item.AsString(); ok {
			names, ok := known[name]
			if !ok {
				return nil, ctx.Raise(object.ArgumentError, "unknown converter name: %q", name)
			}
			for _, n := range names {
				out = append(out, converter{name: n})
			}
			continue
		}
		if item.Kind() != object.KindProc {
			return nil, ctx.Raise(object.TypeError, "a converter is a name or a Proc, not %s", stdlib.KindName(item))
		}
		out = append(out, converter{proc: item})
	}
	return out, nil
}

// builtinConverters are CSV::Converters, each as the converters it runs.
var builtinConverters = map[string][]string{
	"integer":   {"integer"},
	"float":     {"float"},
	"numeric":   {"integer", "float"},
	"date":      {"date"},
	"date_time": {"date_time"},
	"time":      {"time"},
	"all":       {"date_time", "integer", "float"},
}

// builtinHeaderConverters are CSV::HeaderConverters.
var builtinHeaderConverters = map[string][]string{
	"downcase":   {"downcase"},
	"symbol":     {"symbol"},
	"symbol_raw": {"symbol_raw"},
}
