package stdlib

import (
	"os"
	"path"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
)

// Path arithmetic without the file system: File.join, basename, dirname,
// extname, split, expand_path and absolute_path, and Pathname, an
// immutable path with Ruby's +, join, cleanpath and relative_path_from.
// Only expand_path reads anything, the working directory and $HOME, to
// resolve a relative path or ~.

// basename is File.basename: the last component, trailing slashes
// dropped, without suffix when it ends in it, or without its extension
// for ".*".
func basename(p, suffix string) string {
	trimmed := strings.TrimRight(p, "/")
	if trimmed == "" {
		if p == "" {
			return ""
		}
		return "/"
	}
	base := trimmed[strings.LastIndexByte(trimmed, '/')+1:]
	switch {
	case suffix == ".*":
		if ext := extname(base); ext != "" {
			base = base[:len(base)-len(ext)]
		}
	case suffix != "" && suffix != base && strings.HasSuffix(base, suffix):
		base = base[:len(base)-len(suffix)]
	}
	return base
}

// dirname is File.dirname: the path without its last component, "." for
// none and "/" at the root.
func dirname(p string) string {
	trimmed := strings.TrimRight(p, "/")
	if trimmed == "" {
		if strings.HasPrefix(p, "/") {
			return "/"
		}
		return "."
	}
	i := strings.LastIndexByte(trimmed, '/')
	if i < 0 {
		return "."
	}
	dir := strings.TrimRight(trimmed[:i], "/")
	if dir == "" {
		return "/"
	}
	// Leading slashes count as one, as on Unix.
	if strings.HasPrefix(dir, "//") {
		dir = "/" + strings.TrimLeft(dir, "/")
	}
	return dir
}

// extname is File.extname: the last component's extension, dot included,
// "" for none and for a name that only starts with a dot.
func extname(p string) string {
	base := strings.TrimLeft(basename(p, ""), ".")
	i := strings.LastIndexByte(base, '.')
	if i < 0 {
		return ""
	}
	return base[i:]
}

// joinPaths is File.join: the parts with one / between each two, however
// many they end or start with.
func joinPaths(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		switch {
		case strings.HasSuffix(out, "/"):
			out += strings.TrimLeft(p, "/")
		case strings.HasPrefix(p, "/"):
			out += p
		default:
			out += "/" + p
		}
	}
	return out
}

// expandPath is File.expand_path: p made absolute against dir, or the
// working directory, with ~ for $HOME when home says so, . and ..
// resolved.
func expandPath(p, dir string, home bool) string {
	if home && (p == "~" || strings.HasPrefix(p, "~/")) {
		p = os.Getenv("HOME") + p[1:]
	}
	if !strings.HasPrefix(p, "/") {
		if dir == "" {
			dir, _ = os.Getwd()
		} else {
			dir = expandPath(dir, "", home)
		}
		p = dir + "/" + p
	}
	return path.Clean(p)
}

// chopBasename splits a path into what precedes its last component and
// that component, and is false at the root or for an empty path, after
// Ruby's Pathname#chop_basename.
func chopBasename(p string) (string, string, bool) {
	base := basename(p, "")
	if base == "" || base == "/" {
		return "", "", false
	}
	return p[:strings.LastIndex(p, base)], base, true
}

// plusPath is Pathname#+, after pathname.rb: a relative second path joins
// the first, its leading .. taking components off the first.
func plusPath(path1, path2 string) string {
	prefix2 := path2
	var index2 []int
	var names2 []string
	for {
		pre, base, ok := chopBasename(prefix2)
		if !ok {
			break
		}
		prefix2 = pre
		index2 = append([]int{len(pre)}, index2...)
		names2 = append([]string{base}, names2...)
	}
	if prefix2 != "" {
		return path2
	}
	prefix1 := path1
	for {
		for len(names2) > 0 && names2[0] == "." {
			index2, names2 = index2[1:], names2[1:]
		}
		pre, base1, ok := chopBasename(prefix1)
		if !ok {
			break
		}
		prefix1 = pre
		if base1 == "." {
			continue
		}
		if base1 == ".." || len(names2) == 0 || names2[0] != ".." {
			prefix1 += base1
			break
		}
		index2, names2 = index2[1:], names2[1:]
	}
	_, _, r1 := chopBasename(prefix1)
	if !r1 && strings.Contains(basename(prefix1, ""), "/") {
		for len(names2) > 0 && names2[0] == ".." {
			index2, names2 = index2[1:], names2[1:]
		}
	}
	if len(names2) > 0 {
		suffix2 := path2[index2[0]:]
		if r1 {
			return joinPaths([]string{prefix1, suffix2})
		}
		return prefix1 + suffix2
	}
	if r1 {
		return prefix1
	}
	return dirname(prefix1)
}

// cleanPath is Pathname#cleanpath: . and the .. that follow a name
// removed, without asking the file system about links.
func cleanPath(p string) string {
	var names []string
	pre := p
	for {
		before, base, ok := chopBasename(pre)
		if !ok {
			break
		}
		pre = before
		switch {
		case base == ".":
		case base == "..":
			names = append([]string{base}, names...)
		case len(names) > 0 && names[0] == "..":
			names = names[1:]
		default:
			names = append([]string{base}, names...)
		}
	}
	if strings.Contains(basename(pre, ""), "/") {
		for len(names) > 0 && names[0] == ".." {
			names = names[1:]
		}
	}
	rel := joinPaths(names)
	switch {
	case rel == "":
		return dirname(pre)
	case strings.Contains(pre, "/"):
		prefix := dirname(pre)
		if basename(prefix+"a", "") != "a" {
			prefix = joinPaths([]string{prefix, ""})
		}
		return prefix + rel
	}
	return pre + rel
}

// names lists a cleaned path's components after its prefix, without ".".
func names(p string) (string, []string) {
	var out []string
	for {
		pre, base, ok := chopBasename(p)
		if !ok {
			return p, out
		}
		p = pre
		if base != "." {
			out = append([]string{base}, out...)
		}
	}
}

func installPaths() {
	f := object.NewClass("File", object.ObjectClass)
	object.ObjectClass.Consts["File"] = object.ClassValue(f)
	f.Consts["SEPARATOR"] = object.String("/")
	f.Consts["Separator"] = object.String("/")
	str := func(ctx object.Context, v object.Value) (string, error) {
		if p, ok := pathnameOf(v); ok {
			return p, nil
		}
		return wantString(ctx, v)
	}
	f.DefineSingleton("basename", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, argError(ctx, len(args), "1..2")
		}
		p, err := str(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		suffix := ""
		if len(args) == 2 {
			if suffix, err = wantString(ctx, args[1]); err != nil {
				return object.Value{}, err
			}
		}
		return object.String(basename(p, suffix)), nil
	})
	f.DefineSingleton("dirname", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, argError(ctx, len(args), "1..2")
		}
		p, err := str(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		levels := int64(1)
		if len(args) == 2 {
			if levels, err = wantInt(ctx, args[1], ""); err != nil {
				return object.Value{}, err
			}
			if levels < 0 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "negative level: %d", levels)
			}
		}
		for i := int64(0); i < levels; i++ {
			p = dirname(p)
		}
		return object.String(p), nil
	})
	f.DefineSingleton("extname", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		p, err := str(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.String(extname(p)), nil
	})
	f.DefineSingleton("split", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		p, err := str(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(object.String(dirname(p)), object.String(basename(p, ""))), nil
	})
	f.DefineSingleton("join", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		var parts []string
		var add func(v object.Value) error
		add = func(v object.Value) error {
			if l, ok := v.AsList(); ok {
				for _, e := range l.Elems {
					if err := add(e); err != nil {
						return err
					}
				}
				return nil
			}
			s, err := str(ctx, v)
			parts = append(parts, s)
			return err
		}
		for _, a := range args {
			if err := add(a); err != nil {
				return object.Value{}, err
			}
		}
		return object.String(joinPaths(parts)), nil
	})
	expand := func(home bool) object.NativeFn {
		return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) < 1 || len(args) > 2 {
				return object.Value{}, argError(ctx, len(args), "1..2")
			}
			p, err := str(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			dir := ""
			if len(args) == 2 && !args[1].IsNil() {
				if dir, err = str(ctx, args[1]); err != nil {
					return object.Value{}, err
				}
			}
			return object.String(expandPath(p, dir, home)), nil
		}
	}
	f.DefineSingleton("expand_path", -1, expand(true))
	f.DefineSingleton("absolute_path", -1, expand(false))
	f.DefineSingleton("absolute_path?", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		p, err := str(ctx, args[0])
		return object.Bool(strings.HasPrefix(p, "/")), err
	})
	installPathname()
}

// pathnameOf reads a Pathname's path.
func pathnameOf(v object.Value) (string, bool) {
	if inst, ok := v.AsInstance(); ok && inst.Cls == object.PathnameClass {
		return inst.Fields[0].Str(), true
	}
	return "", false
}

// PathnameValue makes a Pathname of a path.
func PathnameValue(p string) object.Value {
	return object.InstanceValue(&object.Instance{Cls: object.PathnameClass, Fields: []object.Value{object.String(p)}})
}

func installPathname() {
	c := object.PathnameClass
	arg := func(ctx object.Context, v object.Value) (string, error) {
		if p, ok := pathnameOf(v); ok {
			return p, nil
		}
		return wantString(ctx, v)
	}
	newPath := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, argError(ctx, len(args), "1")
		}
		p, err := arg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if strings.IndexByte(p, 0) >= 0 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "pathname contains null byte")
		}
		return PathnameValue(p), nil
	}
	c.DefineSingleton("new", -1, newPath)
	object.ObjectClass.Define("Pathname", -1, newPath)
	self := func(v object.Value) string { p, _ := pathnameOf(v); return p }
	unary := func(name string, f func(string) object.Value) {
		def(c, name, 0, func(ctx object.Context, s object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return f(self(s)), nil
		})
	}
	unary("to_s", func(p string) object.Value { return object.String(p) })
	unary("to_path", func(p string) object.Value { return object.String(p) })
	unary("absolute?", func(p string) object.Value { return object.Bool(strings.HasPrefix(p, "/")) })
	unary("relative?", func(p string) object.Value { return object.Bool(!strings.HasPrefix(p, "/")) })
	unary("root?", func(p string) object.Value {
		_, _, ok := chopBasename(p)
		return object.Bool(!ok && strings.Contains(p, "/"))
	})
	unary("cleanpath", func(p string) object.Value { return PathnameValue(cleanPath(p)) })
	unary("parent", func(p string) object.Value { return PathnameValue(plusPath(p, "..")) })
	unary("basename", func(p string) object.Value { return PathnameValue(basename(p, "")) })
	unary("dirname", func(p string) object.Value { return PathnameValue(dirname(p)) })
	unary("extname", func(p string) object.Value { return object.String(extname(p)) })
	unary("each_filename", func(p string) object.Value {
		_, ns := names(p)
		return stringList(ns)
	})
	unary("empty?", func(p string) object.Value { return object.Bool(p == "") })
	plus := func(ctx object.Context, s object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		other, err := arg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return PathnameValue(plusPath(self(s), other)), nil
	}
	def(c, "+", 1, plus)
	def(c, "/", 1, plus)
	// join adds each argument in turn, an absolute one starting afresh.
	def(c, "join", -1, func(ctx object.Context, s object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
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
			result = plusPath(p, result)
		}
		if strings.HasPrefix(result, "/") {
			return PathnameValue(result), nil
		}
		return PathnameValue(plusPath(self(s), result)), nil
	})
	def(c, "sub_ext", 1, func(ctx object.Context, s object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		repl, err := wantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		p := self(s)
		return PathnameValue(p[:len(p)-len(extname(p))] + repl), nil
	})
	def(c, "relative_path_from", 1, func(ctx object.Context, s object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		base, err := arg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		destPrefix, dest := names(cleanPath(self(s)))
		basePrefix, bases := names(cleanPath(base))
		if destPrefix != basePrefix {
			return object.Value{}, ctx.Raise(object.ArgumentError, "different prefix: %s and %s", object.QuoteString(destPrefix), object.QuoteString(cleanPath(base)))
		}
		for len(dest) > 0 && len(bases) > 0 && dest[0] == bases[0] {
			dest, bases = dest[1:], bases[1:]
		}
		for _, b := range bases {
			if b == ".." {
				return object.Value{}, ctx.Raise(object.ArgumentError, "base_directory has ..: %s", object.QuoteString(cleanPath(base)))
			}
		}
		rel := []string{}
		for range bases {
			rel = append(rel, "..")
		}
		rel = append(rel, dest...)
		if len(rel) == 0 {
			return PathnameValue("."), nil
		}
		return PathnameValue(joinPaths(rel)), nil
	})
	def(c, "<=>", 1, func(ctx object.Context, s object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if r, ok := ComparePathnames(s, args[0]); ok {
			return object.Int(r), nil
		}
		return object.Nil, nil
	})
}

// ComparePathnames orders two Pathnames as Ruby does, with / before any
// other character; ok is false unless both are Pathnames.
func ComparePathnames(a, b object.Value) (int, bool) {
	x, ok := pathnameOf(a)
	y, ok2 := pathnameOf(b)
	if !ok || !ok2 {
		return 0, false
	}
	key := func(p string) string { return strings.ReplaceAll(p, "/", "\x00") }
	return strings.Compare(key(x), key(y)), true
}
