// Package csv is Ruby's csv library, built into cameo: CSV.parse and
// parse_line, generate and generate_line, CSV::Row and CSV::Table, with
// headers, converters, quoting and the separators as options.
//
// It follows csv 3.3.2, the version Ruby 3.4 ships. A Row and a Table are
// values: []=, << and delete return the changed copy for the variable to
// take, where Ruby changes them in place. A CSV object, CSV.new(string),
// is a handle that reads the string a row at a time and takes rows with
// <<, as an IO does. CSV.read, foreach, open, table and the other file
// methods are left out, as cameo reads no files.
package csv

import (
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	csvClass = object.NewClass("CSV", object.ObjectClass)
	// module is the class the library's constants hang off: CSV itself.
	module         = csvClass
	rowClass       = object.NewClass("Row", object.ObjectClass)
	tableClass     = object.NewClass("Table", object.ObjectClass)
	fieldInfoClass = object.NewClass("FieldInfo", object.ObjectClass)
	malformedError = object.NewClass("MalformedCSVError", object.RuntimeError)
)

// kindName names a value's kind for a message.
func kindName(v object.Value) string { return stdlib.KindName(v) }

// Install defines CSV. lib.Install calls it once.
func Install() {
	// CSV is a class in Ruby; here the class carries the module functions
	// as singleton methods, and holds Row, Table and the errors.
	c := csvClass
	object.ObjectClass.Consts["CSV"] = object.ClassValue(c)
	c.Consts["VERSION"] = object.String("3.3.2")
	malformedError.Lexical = c
	c.Consts["MalformedCSVError"] = object.ClassValue(malformedError)
	malformedError.Define("line_number", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if v, ok := self.Exception().Attr("line_number"); ok {
			return v, nil
		}
		return object.Nil, nil
	})
	malformedError.Define("lineno", 0, malformedError.Methods["line_number"].Native)
	fieldInfoClass.Lexical = c
	c.Consts["FieldInfo"] = object.ClassValue(fieldInfoClass)
	for i, name := range []string{"index", "line", "header", "quoted?"} {
		i := i
		fieldInfoClass.Define(name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return self.Instance().Fields[i], nil
		})
	}
	installRow()
	installTable()
	installHandle()

	// input reads the string to parse: nil is Ruby's ArgumentError.
	input := func(ctx object.Context, v object.Value) (string, error) {
		if v.IsNil() {
			return "", ctx.Raise(object.ArgumentError, "Cannot parse nil as CSV")
		}
		return stdlib.WantString(ctx, v)
	}
	c.DefineSingleton("parse", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
		}
		s, err := input(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		o := defaults()
		if err := o.read(ctx, kw); err != nil {
			return object.Value{}, err
		}
		h := handleOf(newHandle(ctx, s, &o))
		if block.IsZero() {
			return h.readAll(ctx)
		}
		for {
			r, err := h.shift()
			if err != nil {
				return object.Value{}, err
			}
			if r.IsNil() {
				return object.Nil, nil
			}
			if _, err := ctx.Yield(block, r); err != nil {
				return object.Value{}, err
			}
		}
	})
	parseLine := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
		}
		s, err := input(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		o := defaults()
		if err := o.read(ctx, kw); err != nil {
			return object.Value{}, err
		}
		return handleOf(newHandle(ctx, s, &o)).shift()
	}
	c.DefineSingleton("parse_line", -1, parseLine)
	object.StringClass.Define("parse_csv", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return parseLine(ctx, self, []object.Value{self}, kw, block)
	})

	generateLineFn := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
		}
		o := defaults()
		if err := o.read(ctx, kw); err != nil {
			return object.Value{}, err
		}
		return generateLine(ctx, &o, args[0])
	}
	c.DefineSingleton("generate_line", -1, generateLineFn)
	object.ListClass.Define("to_csv", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return generateLineFn(ctx, self, []object.Value{self}, kw, block)
	})
	c.DefineSingleton("generate_lines", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
		}
		rows, err := stdlib.ToList(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		o := defaults()
		if err := o.read(ctx, kw); err != nil {
			return object.Value{}, err
		}
		w := newWriter(ctx, &o)
		for _, r := range rows {
			if err := w.add(r); err != nil {
				return object.Value{}, err
			}
		}
		return object.String(w.b.String()), nil
	})
	c.DefineSingleton("generate", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "0..1")
		}
		if block.IsZero() {
			return object.Value{}, ctx.Raise(object.LocalJumpError, "no block given (yield)")
		}
		start := ""
		if len(args) == 1 {
			s, err := stdlib.WantString(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			start = s
		}
		o := defaults()
		if err := o.read(ctx, kw); err != nil {
			return object.Value{}, err
		}
		v := newHandle(ctx, start, &o)
		if _, err := ctx.Yield(block, v); err != nil {
			return object.Value{}, err
		}
		h := handleOf(v)
		return object.String(h.p.s + h.w.b.String()), nil
	})
	newCSV := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "0..1")
		}
		start := ""
		if len(args) == 1 && !args[0].IsNil() {
			s, err := stdlib.WantString(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			start = s
		}
		o := defaults()
		if err := o.read(ctx, kw); err != nil {
			return object.Value{}, err
		}
		return newHandle(ctx, start, &o), nil
	}
	c.DefineSingleton("new", -1, newCSV)
	c.DefineSingleton("instance", -1, newCSV)
}
