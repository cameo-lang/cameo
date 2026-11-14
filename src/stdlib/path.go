package stdlib

import (
	"github.com/cameo-lang/cameo/src/str"
	"os"
	"path"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
)

// Path arithmetic without the file system: File.join, basename, dirname,
// extname, split, expand_path and absolute_path. Only expand_path reads
// anything, the working directory and $HOME, to resolve a relative path or
// ~. The functions are exported for src/lib/pathname, which builds
// Pathname on them; PathnameOf and PathnameValue are here because File
// takes a Pathname where it takes a path.

// Basename is File.basename: the last component, trailing slashes
// dropped, without suffix when it ends in it, or without its extension
// for ".*".
func Basename(p, suffix string) string {
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
		if ext := Extname(base); ext != "" {
			base = base[:len(base)-len(ext)]
		}
	case suffix != "" && suffix != base && strings.HasSuffix(base, suffix):
		base = base[:len(base)-len(suffix)]
	}
	return base
}

// Dirname is File.dirname: the path without its last component, "." for
// none and "/" at the root.
func Dirname(p string) string {
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

// Extname is File.extname: the last component's extension, dot included,
// "" for none and for a name that only starts with a dot.
func Extname(p string) string {
	base := strings.TrimLeft(Basename(p, ""), ".")
	i := strings.LastIndexByte(base, '.')
	if i < 0 {
		return ""
	}
	return base[i:]
}

// JoinPaths is File.join: the parts with a / between each two that have
// none, and the separators of the right one where both have them.
func JoinPaths(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		// Where both sides have separators, the right side's stay.
		switch {
		case strings.HasPrefix(p, "/"):
			out = strings.TrimRight(out, "/") + p
		case strings.HasSuffix(out, "/"):
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

// ChopBasename splits a path into what precedes its last component and
// that component, and is false at the root or for an empty path, after
// Ruby's Pathname#chop_basename.
func ChopBasename(p string) (string, string, bool) {
	base := Basename(p, "")
	if base == "" || base == "/" {
		return "", "", false
	}
	return p[:strings.LastIndex(p, base)], base, true
}

// PlusPath is Pathname#+, after pathname.rb: a relative second path joins
// the first, its leading .. taking components off the first.
func PlusPath(path1, path2 string) string {
	prefix2 := path2
	var index2 []int
	var names2 []string
	for {
		pre, base, ok := ChopBasename(prefix2)
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
		pre, base1, ok := ChopBasename(prefix1)
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
	_, _, r1 := ChopBasename(prefix1)
	if !r1 && strings.Contains(Basename(prefix1, ""), "/") {
		for len(names2) > 0 && names2[0] == ".." {
			index2, names2 = index2[1:], names2[1:]
		}
	}
	if len(names2) > 0 {
		suffix2 := path2[index2[0]:]
		if r1 {
			return JoinPaths([]string{prefix1, suffix2})
		}
		return prefix1 + suffix2
	}
	if r1 {
		return prefix1
	}
	return Dirname(prefix1)
}

// CleanPath is Pathname#cleanpath: . and the .. that follow a name
// removed, without asking the file system about links.
func CleanPath(p string) string {
	var names []string
	pre := p
	for {
		before, base, ok := ChopBasename(pre)
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
	if strings.Contains(Basename(pre, ""), "/") {
		for len(names) > 0 && names[0] == ".." {
			names = names[1:]
		}
	}
	rel := JoinPaths(names)
	switch {
	case rel == "":
		return Dirname(pre)
	case strings.Contains(pre, "/"):
		prefix := Dirname(pre)
		if Basename(prefix+"a", "") != "a" {
			prefix = JoinPaths([]string{prefix, ""})
		}
		return prefix + rel
	}
	return pre + rel
}

// PathNames lists a cleaned path's components after its prefix, without ".".
func PathNames(p string) (string, []string) {
	var out []string
	for {
		pre, base, ok := ChopBasename(p)
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
	f.Consts["SEPARATOR"] = object.NewString("/", str.ASCII)
	f.Consts["Separator"] = object.NewString("/", str.ASCII)
	f.Consts["PATH_SEPARATOR"] = object.NewString(":", str.ASCII)
	f.Consts["ALT_SEPARATOR"] = object.Nil
	// The open, lock and fnmatch flags, with Linux's values. cameo opens
	// files only to read them (Cameo::IO::FileIO, which takes a mode
	// String); they are here for code that names them.
	for name, v := range map[string]int64{
		"RDONLY": 0, "WRONLY": 1, "RDWR": 2, "CREAT": 0o100, "EXCL": 0o200, "NOCTTY": 0o400,
		"TRUNC": 0o1000, "APPEND": 0o2000, "NONBLOCK": 0o4000,
		"LOCK_SH": 1, "LOCK_EX": 2, "LOCK_NB": 4, "LOCK_UN": 8,
		"FNM_NOESCAPE": 1, "FNM_PATHNAME": 2, "FNM_DOTMATCH": 4, "FNM_CASEFOLD": 8, "FNM_EXTGLOB": 16, "FNM_SYSCASE": 0,
	} {
		f.Consts[name] = object.Int(v)
	}
	strOf := func(ctx object.Context, v object.Value) (string, error) {
		if p, ok := PathnameOf(v); ok {
			return p, nil
		}
		return WantString(ctx, v)
	}
	f.DefineSingleton("basename", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, ArgError(ctx, len(args), "1..2")
		}
		p, err := strOf(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		suffix := ""
		if len(args) == 2 {
			if suffix, err = WantString(ctx, args[1]); err != nil {
				return object.Value{}, err
			}
		}
		return object.NewString(Basename(p, suffix), str.UTF8), nil
	})
	f.DefineSingleton("dirname", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, ArgError(ctx, len(args), "1..2")
		}
		p, err := strOf(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		levels := int64(1)
		if len(args) == 2 {
			if levels, err = WantInt(ctx, args[1], ""); err != nil {
				return object.Value{}, err
			}
			if levels < 0 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "negative level: %d", levels)
			}
		}
		for i := int64(0); i < levels; i++ {
			p = Dirname(p)
		}
		return object.NewString(p, str.UTF8), nil
	})
	f.DefineSingleton("extname", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		p, err := strOf(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.NewString(Extname(p), str.UTF8), nil
	})
	f.DefineSingleton("split", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		p, err := strOf(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(object.NewString(Dirname(p), str.UTF8), object.NewString(Basename(p, ""), str.UTF8)), nil
	})
	f.DefineSingleton("join", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		// An array joins into one part, as File.join(*array).
		var join func(args []object.Value) (string, error)
		join = func(args []object.Value) (string, error) {
			parts := make([]string, len(args))
			for i, a := range args {
				var err error
				if l, ok := a.AsList(); ok {
					parts[i], err = join(l.Elems)
				} else {
					parts[i], err = strOf(ctx, a)
				}
				if err != nil {
					return "", err
				}
				if strings.IndexByte(parts[i], 0) >= 0 {
					return "", ctx.Raise(object.ArgumentError, "string contains null byte")
				}
			}
			return JoinPaths(parts), nil
		}
		s, err := join(args)
		return object.NewString(s, str.UTF8), err
	})
	expand := func(home bool) object.NativeFn {
		return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) < 1 || len(args) > 2 {
				return object.Value{}, ArgError(ctx, len(args), "1..2")
			}
			p, err := strOf(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			dir := ""
			if len(args) == 2 && !args[1].IsNil() {
				if dir, err = strOf(ctx, args[1]); err != nil {
					return object.Value{}, err
				}
			}
			return object.NewString(expandPath(p, dir, home), str.UTF8), nil
		}
	}
	f.DefineSingleton("expand_path", -1, expand(true))
	f.DefineSingleton("absolute_path", -1, expand(false))
	f.DefineSingleton("absolute_path?", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		p, err := strOf(ctx, args[0])
		return object.Bool(strings.HasPrefix(p, "/")), err
	})
}

// PathnameOf returns the path a Pathname holds, and false for a value that
// is no Pathname.
func PathnameOf(v object.Value) (string, bool) {
	if inst, ok := v.AsInstance(); ok && inst.Cls == object.PathnameClass {
		return inst.Fields[0].Str(), true
	}
	return "", false
}

// PathnameValue makes a Pathname of a path.
func PathnameValue(p string) object.Value {
	return object.InstanceValue(&object.Instance{Cls: object.PathnameClass, Fields: []object.Value{object.NewString(p, str.UTF8)}})
}
