// Package yaml is Ruby's yaml library, built into cameo: the YAML module
// as its other name.
//
// It follows psych 5.2.2, the version Ruby 3.4 ships, on the libyaml it is
// built on: the libyaml package here is gopkg.in/yaml.v3's port of that C
// library, so a document is parsed, emitted, quoted, folded and refused as
// Ruby parses, emits, quotes, folds and refuses it. Psych's own layer, the
// scalar scanner, the class loader and the visitors, is written out in Go
// in scan.go, load.go and dump.go.
//
// What loads is plain data: nil, booleans, integers, floats, strings,
// symbols, arrays, hashes, and, when their class is permitted, dates,
// times, ranges, regexps, sets and instances of cameo classes by their
// fields. load and safe_load restrict classes as Psych's do; unsafe_load
// and load_stream do not. What dumps is any value; a Proc, a Random or a
// MatchData dumps as the empty object Psych writes. load_file and the
// other file methods are left out, as cameo reads no files; parse, the
// node tree, is left out too.
package yaml

import (
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	module            = object.NewModule("YAML")
	exception         = object.NewClass("Exception", object.RuntimeError)
	badAlias          = object.NewClass("BadAlias", exception)
	aliasesNotEnabled = object.NewClass("AliasesNotEnabled", badAlias)
	anchorNotDefined  = object.NewClass("AnchorNotDefined", badAlias)
	disallowedClass   = object.NewClass("DisallowedClass", exception)
	syntaxError       = object.NewClass("SyntaxError", exception)
)

// Install defines YAML. Ruby's YAML is an alias of its Psych module; cameo
// has the one name, so the errors are YAML::SyntaxError and kin.
// lib.Install calls it once.
func Install() {
	m := module
	object.ObjectClass.Consts["YAML"] = object.ClassValue(m)
	m.Consts["VERSION"] = object.String("5.2.2")
	m.Consts["LIBYAML_VERSION"] = object.String("0.2.5")
	for _, c := range []*object.Class{exception, badAlias, aliasesNotEnabled, anchorNotDefined, disallowedClass, syntaxError} {
		c.Lexical = m
		m.Consts[c.Name] = object.ClassValue(c)
	}
	for _, name := range []string{"file", "line", "column", "offset", "problem", "context"} {
		name := name
		syntaxError.Define(name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if v, ok := self.Exception().Attr(name); ok {
				return v, nil
			}
			return object.Nil, nil
		})
	}

	m.DefineSingleton("load", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return load(ctx, args, kw, loadOptions{restricted: true, classes: []string{"Symbol"}, fallback: object.Nil},
			"permitted_classes", "permitted_symbols", "aliases", "filename", "fallback", "symbolize_names", "freeze", "strict_integer")
	})
	m.DefineSingleton("safe_load", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return load(ctx, args, kw, loadOptions{restricted: true, fallback: object.Nil},
			"permitted_classes", "permitted_symbols", "aliases", "filename", "fallback", "symbolize_names", "freeze", "strict_integer")
	})
	m.DefineSingleton("unsafe_load", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return load(ctx, args, kw, loadOptions{aliases: true, fallback: object.False},
			"filename", "fallback", "symbolize_names", "freeze", "strict_integer")
	})
	m.DefineSingleton("load_stream", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
		}
		o := loadOptions{aliases: true, fallback: object.NewList()}
		if err := o.read(ctx, kw, "filename", "fallback", "symbolize_names", "freeze", "strict_integer"); err != nil {
			return object.Value{}, err
		}
		input, err := wantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		docs, err := documents(ctx, input, o.filename, false)
		if err != nil {
			return object.Value{}, err
		}
		if len(docs) == 0 {
			return o.fallback, nil
		}
		out := make([]object.Value, 0, len(docs))
		for _, doc := range docs {
			v, err := o.loader(ctx).accept(doc)
			if err != nil {
				return object.Value{}, err
			}
			if !block.IsZero() {
				if _, err := ctx.Yield(block, v); err != nil {
					return object.Value{}, err
				}
			}
			out = append(out, v)
		}
		if !block.IsZero() {
			return object.Nil, nil
		}
		return object.NewList(out...), nil
	})

	m.DefineSingleton("dump", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return dump(ctx, args, kw, false)
	})
	m.DefineSingleton("safe_dump", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return dump(ctx, args, kw, true)
	})
	m.DefineSingleton("dump_stream", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		d, err := newDumper(ctx, options{}, false)
		if err != nil {
			return object.Value{}, err
		}
		for _, v := range args {
			if err := d.push(v); err != nil {
				return object.Value{}, err
			}
		}
		s, err := d.finish()
		if err != nil {
			return object.Value{}, err
		}
		return object.String(s), nil
	})
	object.ObjectClass.Define("to_yaml", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return dump(ctx, append([]object.Value{self}, args...), kw, false)
	})
}

// loadOptions are a load's keywords, with the defaults of the method.
type loadOptions struct {
	restricted bool
	classes    []string
	symbols    []string
	aliases    bool
	filename   object.Value
	fallback   object.Value
	symbolize  bool
	strict     bool
}

// read takes the keywords, refusing one that is not among names as Ruby
// does.
func (o *loadOptions) read(ctx object.Context, kw *object.Map, names ...string) error {
	if o.filename.IsZero() {
		o.filename = object.Nil
	}
	if kw == nil {
		return nil
	}
	for i, k := range kw.Keys() {
		name, _ := k.AsSymbol()
		known := false
		for _, n := range names {
			known = known || n == name
		}
		if !known {
			inspect, _ := ctx.Inspect(k)
			return ctx.Raise(object.ArgumentError, "unknown keyword: %s", inspect)
		}
		v := kw.Values()[i]
		switch name {
		case "permitted_classes", "permitted_symbols":
			list, err := stdlib.ToList(ctx, v)
			if err != nil {
				return err
			}
			names := make([]string, 0, len(list))
			for _, e := range list {
				s, err := ctx.ToS(e)
				if err != nil {
					return err
				}
				names = append(names, s)
			}
			if name == "permitted_classes" {
				o.classes = names
			} else {
				o.symbols = names
			}
		case "aliases":
			o.aliases = v.Truthy()
		case "filename":
			o.filename = v
		case "fallback":
			o.fallback = v
		case "symbolize_names":
			o.symbolize = v.Truthy()
		case "strict_integer":
			o.strict = v.Truthy()
		}
	}
	return nil
}

// loader makes the class loader and visitor of these options.
func (o *loadOptions) loader(ctx object.Context) *loader {
	l := &loader{ctx: ctx, restricted: o.restricted, aliases: o.aliases, symbolizeNames: o.symbolize, strict: o.strict,
		classes: map[string]bool{}, symbols: map[string]bool{}}
	for _, c := range o.classes {
		l.classes[c] = true
	}
	for _, s := range o.symbols {
		l.symbols[s] = true
	}
	return l
}

// load is Psych.load, safe_load and unsafe_load: the first document of the
// string, or the fallback when there is none.
func load(ctx object.Context, args []object.Value, kw *object.Map, o loadOptions, names ...string) (object.Value, error) {
	if len(args) != 1 {
		return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
	}
	if err := o.read(ctx, kw, names...); err != nil {
		return object.Value{}, err
	}
	input, err := wantString(ctx, args[0])
	if err != nil {
		return object.Value{}, err
	}
	docs, err := documents(ctx, input, o.filename, true)
	if err != nil {
		return object.Value{}, err
	}
	if len(docs) == 0 {
		return o.fallback, nil
	}
	return o.loader(ctx).accept(docs[0])
}

// dump is Psych.dump and safe_dump: the value, then an options Hash or nil,
// and keywords.
func dump(ctx object.Context, args []object.Value, kw *object.Map, restricted bool) (object.Value, error) {
	if len(args) < 1 || len(args) > 3 {
		return object.Value{}, stdlib.ArgError(ctx, len(args), "1..3")
	}
	o := options{}
	for _, a := range args[1:] {
		if a.IsNil() {
			continue
		}
		m, ok := a.AsMap()
		if !ok {
			return object.Value{}, ctx.Raise(object.ArgumentError, "dump writes a String: an IO is not supported")
		}
		if err := o.read(ctx, m); err != nil {
			return object.Value{}, err
		}
	}
	if err := o.read(ctx, kw); err != nil {
		return object.Value{}, err
	}
	d, err := newDumper(ctx, o, restricted)
	if err != nil {
		return object.Value{}, err
	}
	if err := d.push(args[0]); err != nil {
		return object.Value{}, err
	}
	s, err := d.finish()
	if err != nil {
		return object.Value{}, err
	}
	return object.String(s), nil
}

// read takes dump's options; one it does not know is ignored, as Psych
// ignores it.
func (o *options) read(ctx object.Context, m *object.Map) error {
	if m == nil {
		return nil
	}
	for i, k := range m.Keys() {
		name, _ := k.AsSymbol()
		v := m.Values()[i]
		switch name {
		case "indentation":
			n, err := stdlib.WantInt(ctx, v, "")
			if err != nil {
				return err
			}
			o.indent = int(n)
		case "line_width":
			n, err := stdlib.WantInt(ctx, v, "")
			if err != nil {
				return err
			}
			o.width, o.hasWidth = int(n), true
		case "canonical":
			o.canonical = v.Truthy()
		case "header":
			o.header = v.Truthy()
		case "version":
			if list, ok := v.AsList(); ok && len(list.Elems) == 2 {
				major, _ := list.Elems[0].AsInt()
				minor, _ := list.Elems[1].AsInt()
				o.version = &[2]int{int(major), int(minor)}
			} else if s, ok := v.AsString(); ok {
				var major, minor int
				if n, _ := sscanVersion(s, &major, &minor); n == 2 {
					o.version = &[2]int{major, minor}
				}
			}
		case "permitted_classes":
			list, err := stdlib.ToList(ctx, v)
			if err != nil {
				return err
			}
			o.classes = list
		case "permitted_symbols":
			list, err := stdlib.ToList(ctx, v)
			if err != nil {
				return err
			}
			o.symbols = list
		}
	}
	return nil
}

// sscanVersion reads "1.1".
func sscanVersion(s string, major, minor *int) (int, error) {
	n := 0
	for i, part := range [...]*int{major, minor} {
		if i == 1 {
			dot := -1
			for j := 0; j < len(s); j++ {
				if s[j] == '.' {
					dot = j
					break
				}
			}
			if dot < 0 {
				return n, nil
			}
			s = s[dot+1:]
		}
		v := 0
		digits := 0
		for digits < len(s) && s[digits] >= '0' && s[digits] <= '9' {
			v = v*10 + int(s[digits]-'0')
			digits++
		}
		if digits == 0 {
			return n, nil
		}
		*part = v
		n++
	}
	return n, nil
}
