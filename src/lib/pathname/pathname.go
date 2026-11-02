// Package pathname is Ruby's pathname library, built into cameo: Pathname,
// an immutable path with Ruby's +, join, cleanpath, relative_path_from
// and the readers of its parts. The path arithmetic is src/stdlib's,
// shared with File; everything that reads the file system (exist?, read,
// children, glob) is left out.
package pathname

import (
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

// Install defines Pathname and Kernel#Pathname. lib.Install calls it
// once.
func Install() {
	c := object.PathnameClass
	arg := func(ctx object.Context, v object.Value) (string, error) {
		if p, ok := stdlib.PathnameOf(v); ok {
			return p, nil
		}
		return stdlib.WantString(ctx, v)
	}
	newPath := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
		}
		p, err := arg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if strings.IndexByte(p, 0) >= 0 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "pathname contains null byte")
		}
		return stdlib.PathnameValue(p), nil
	}
	c.DefineSingleton("new", -1, newPath)
	object.ObjectClass.Define("Pathname", -1, newPath)
	self := func(v object.Value) string { p, _ := stdlib.PathnameOf(v); return p }
	unary := func(name string, f func(string) object.Value) {
		c.Define(name, 0, func(ctx object.Context, s object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return f(self(s)), nil
		})
	}
	unary("to_s", func(p string) object.Value { return object.String(p) })
	unary("to_path", func(p string) object.Value { return object.String(p) })
	unary("absolute?", func(p string) object.Value { return object.Bool(strings.HasPrefix(p, "/")) })
	unary("relative?", func(p string) object.Value { return object.Bool(!strings.HasPrefix(p, "/")) })
	unary("root?", func(p string) object.Value {
		_, _, ok := stdlib.ChopBasename(p)
		return object.Bool(!ok && strings.Contains(p, "/"))
	})
	unary("cleanpath", func(p string) object.Value { return stdlib.PathnameValue(stdlib.CleanPath(p)) })
	unary("parent", func(p string) object.Value { return stdlib.PathnameValue(stdlib.PlusPath(p, "..")) })
	unary("basename", func(p string) object.Value { return stdlib.PathnameValue(stdlib.Basename(p, "")) })
	unary("dirname", func(p string) object.Value { return stdlib.PathnameValue(stdlib.Dirname(p)) })
	unary("extname", func(p string) object.Value { return object.String(stdlib.Extname(p)) })
	unary("each_filename", func(p string) object.Value {
		_, ns := stdlib.PathNames(p)
		return stdlib.StringList(ns)
	})
	unary("empty?", func(p string) object.Value { return object.Bool(p == "") })
	plus := func(ctx object.Context, s object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		other, err := arg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return stdlib.PathnameValue(stdlib.PlusPath(self(s), other)), nil
	}
	c.Define("+", 1, plus)
	c.Define("/", 1, plus)
	// join adds each argument in turn, an absolute one starting afresh.
	c.Define("join", -1, func(ctx object.Context, s object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 {
			return s, nil
		}
		result, err := arg(ctx, args[len(args)-1])
		if err != nil {
			return object.Value{}, err
		}
		for i := len(args) - 2; i >= 0 && !strings.HasPrefix(result, "/"); i-- {
			p, err := arg(ctx, args[i])
			if err != nil {
				return object.Value{}, err
			}
			result = stdlib.PlusPath(p, result)
		}
		if strings.HasPrefix(result, "/") {
			return stdlib.PathnameValue(result), nil
		}
		return stdlib.PathnameValue(stdlib.PlusPath(self(s), result)), nil
	})
	c.Define("sub_ext", 1, func(ctx object.Context, s object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		repl, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		p := self(s)
		return stdlib.PathnameValue(p[:len(p)-len(stdlib.Extname(p))] + repl), nil
	})
	c.Define("relative_path_from", 1, func(ctx object.Context, s object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		base, err := arg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		destPrefix, dest := stdlib.PathNames(stdlib.CleanPath(self(s)))
		basePrefix, bases := stdlib.PathNames(stdlib.CleanPath(base))
		if destPrefix != basePrefix {
			return object.Value{}, ctx.Raise(object.ArgumentError, "different prefix: %s and %s", object.QuoteString(destPrefix), object.QuoteString(stdlib.CleanPath(base)))
		}
		for len(dest) > 0 && len(bases) > 0 && dest[0] == bases[0] {
			dest, bases = dest[1:], bases[1:]
		}
		for _, b := range bases {
			if b == ".." {
				return object.Value{}, ctx.Raise(object.ArgumentError, "base_directory has ..: %s", object.QuoteString(stdlib.CleanPath(base)))
			}
		}
		rel := []string{}
		for range bases {
			rel = append(rel, "..")
		}
		rel = append(rel, dest...)
		if len(rel) == 0 {
			return stdlib.PathnameValue("."), nil
		}
		return stdlib.PathnameValue(stdlib.JoinPaths(rel)), nil
	})
	c.Define("<=>", 1, func(ctx object.Context, s object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if r, ok := Compare(s, args[0]); ok {
			return object.Int(r), nil
		}
		return object.Nil, nil
	})
}

// Compare orders two Pathnames as Ruby does, with / before any
// other character; ok is false unless both are Pathnames.
func Compare(a, b object.Value) (int, bool) {
	x, ok := stdlib.PathnameOf(a)
	y, ok2 := stdlib.PathnameOf(b)
	if !ok || !ok2 {
		return 0, false
	}
	key := func(p string) string { return strings.ReplaceAll(p, "/", "\x00") }
	return strings.Compare(key(x), key(y)), true
}
