package uri

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

type fn = object.NativeFn

// installModule defines URI's module functions and Kernel#URI.
func installModule() {
	m := uriModule
	m.DefineSingleton("parse", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return parse(ctx, args[0])
	})
	m.DefineSingleton("split", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return splitCall(ctx, args[0])
	})
	m.DefineSingleton("join", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return join(ctx, args)
	})
	m.DefineSingleton("for", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 {
			return object.Value{}, stdlib.ArgError(ctx, 0, "1+")
		}
		c := make([]*string, 9)
		for i := 0; i < 9 && i < len(args); i++ {
			if s, ok := args[i].AsString(); ok {
				c[i] = str(s)
			} else if n, ok := args[i].AsInt(); ok {
				c[i] = str(strconv.FormatInt(n, 10))
			}
		}
		cls := genericClass
		if kw != nil {
			if d, ok := kw.Get(object.Symbol("default")); ok {
				if dc, isClass := d.AsClass(); isClass {
					cls = dc
				}
			}
		}
		if c[0] != nil {
			if sc, ok := schemes[strings.ToUpper(*c[0])]; ok {
				cls = sc
			}
		}
		return construct(ctx, cls, c, false)
	})
	m.DefineSingleton("scheme_list", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		b := object.NewMapBuilder()
		for _, name := range []string{"FILE", "FTP", "HTTP", "HTTPS", "LDAP", "LDAPS", "MAILTO", "WS", "WSS"} {
			b.Set(object.String(name), object.ClassValue(schemes[name]))
		}
		return object.MapValue(b.Map()), nil
	})
	m.DefineSingleton("extract", -1, extract)
	m.DefineSingleton("regexp", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Value{}, ctx.Raise(object.NotImplementedErr, "URI.regexp is obsolete in Ruby and not in cameo; use URI.extract")
	})

	// Kernel#URI: a URI as it is, a String parsed.
	object.ObjectClass.Define("URI", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		v := args[0]
		if isURI(v) {
			return v, nil
		}
		if v.IsString() {
			return parse(ctx, v)
		}
		return object.Value{}, ctx.Raise(object.ArgumentError, "bad argument (expected URI object or URI string)")
	})
}

// splitCall is URI.split: the nine components, with the port as a String.
func splitCall(ctx object.Context, v object.Value) (object.Value, error) {
	s, ok := v.AsString()
	if !ok {
		shown, err := ctx.Inspect(v)
		if err != nil {
			return object.Value{}, err
		}
		return object.Value{}, ctx.Raise(invalidURI, "bad URI (is not URI?): %s", shown)
	}
	c, err := split(ctx, s)
	if err != nil {
		return object.Value{}, err
	}
	return strings9(c), nil
}

func strings9(c []*string) object.Value {
	out := make([]object.Value, 9)
	for i, s := range c {
		if s == nil {
			out[i] = object.Nil
		} else {
			out[i] = object.String(*s)
		}
	}
	return object.NewList(out...)
}

// join is URI.join: the first argument as a URI, with each following one
// merged into it.
func join(ctx object.Context, args []object.Value) (object.Value, error) {
	if len(args) == 0 {
		return object.Value{}, stdlib.ArgError(ctx, 0, "1+")
	}
	out, err := toURI(ctx, args[0])
	if err != nil {
		return object.Value{}, err
	}
	for _, a := range args[1:] {
		if out, err = merge(ctx, out, a); err != nil {
			return object.Value{}, err
		}
	}
	return out, nil
}

// extractRe is RFC 2396's absolute URI reference, what URI.extract finds
// in a text: a scheme, a colon, and the characters a URI may hold.
var extractRe = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.\-]*:(?:%[0-9A-Fa-f]{2}|[;/?:@&=+$,\[\]A-Za-z0-9\-_.!~*'()])+(?:#(?:%[0-9A-Fa-f]{2}|[;/?:@&=+$,\[\]A-Za-z0-9\-_.!~*'()])*)?`)

// extract is URI.extract(str, schemes = nil): every absolute URI in the
// text, of the schemes named when any are, to the block or as an Array.
func extract(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
	}
	s, err := stdlib.WantString(ctx, args[0])
	if err != nil {
		return object.Value{}, err
	}
	var want map[string]bool
	if len(args) == 2 && !args[1].IsNil() {
		names, err := stdlib.ToList(ctx, args[1])
		if err != nil {
			return object.Value{}, err
		}
		want = map[string]bool{}
		for _, n := range names {
			ns, err := stdlib.WantString(ctx, n)
			if err != nil {
				return object.Value{}, err
			}
			want[strings.ToLower(ns)] = true
		}
	}
	var found []object.Value
	for _, m := range extractRe.FindAllString(s, -1) {
		sch, _, _ := strings.Cut(m, ":")
		if want != nil && !want[strings.ToLower(sch)] {
			continue
		}
		if !block.IsZero() {
			if _, err := ctx.Yield(block, object.String(m)); err != nil {
				return object.Value{}, err
			}
			continue
		}
		found = append(found, object.String(m))
	}
	if !block.IsZero() {
		return object.Nil, nil
	}
	return object.NewList(found...), nil
}

// installGeneric defines URI::Generic: new and build, the readers, the
// operations, and the refusal of with.
func installGeneric() {
	c := genericClass
	c.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		cls := self.Cls()
		if cls.IsA(mailtoClass) {
			return newMailTo(ctx, cls, args)
		}
		if len(args) < 9 || len(args) > 11 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "9..11")
		}
		comps, err := componentStrings(ctx, args[:9])
		if err != nil {
			return object.Value{}, err
		}
		argCheck := len(args) == 11 && args[10].Truthy()
		return construct(ctx, cls, comps, argCheck)
	})
	// build takes the components as a Hash, which a script writes as
	// keywords, or as an Array in the class's component order.
	buildFn := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		arg, err := oneOrKeywords(ctx, args, kw)
		if err != nil {
			return object.Value{}, err
		}
		return buildCall(ctx, self.Cls(), arg)
	}
	c.DefineSingleton("build", -1, buildFn)
	c.DefineSingleton("build2", -1, buildFn)
	c.DefineSingleton("component", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return symbols(componentsOf(self.Cls())), nil
	})
	c.DefineSingleton("default_port", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if dp := defaultPort(self.Cls()); dp != nil {
			return object.Int(*dp), nil
		}
		return object.Nil, nil
	})
	c.DefineSingleton("use_registry", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.False, nil
	})

	// The readers.
	reader := func(name string, i int) {
		c.Define(name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.OrNil(self.Instance().Fields[i]), nil
		})
	}
	for i, name := range fieldNames {
		reader(name, i)
	}
	c.Define("registry", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Nil, nil
	})
	c.Define("user", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		u, _ := userPassword(read(self))
		return strOrNil(u), nil
	})
	c.Define("password", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		_, pw := userPassword(read(self))
		return strOrNil(pw), nil
	})
	c.Define("hostname", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		p := read(self)
		if p.host == nil {
			return object.Nil, nil
		}
		h := *p.host
		if strings.HasPrefix(h, "[") && strings.HasSuffix(h, "]") {
			h = h[1 : len(h)-1]
		}
		return object.String(h), nil
	})
	c.Define("default_port", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if dp := defaultPort(self.Instance().Cls); dp != nil {
			return object.Int(*dp), nil
		}
		return object.Nil, nil
	})
	c.Define("component", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return symbols(componentsOf(self.Instance().Cls)), nil
	})
	c.Define("component_ary", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		vs, err := componentValues(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(vs...), nil
	})
	c.Define("select", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		cls := self.Instance().Cls
		names := componentsOf(cls)
		out := make([]object.Value, len(args))
		for i, a := range args {
			n, ok := a.AsFieldName()
			if !ok || indexOf(names, n) < 0 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "expected of components of %s (%s)", cls.QualifiedName(), strings.Join(names, ", "))
			}
			v, err := ctx.Call(self, n)
			if err != nil {
				return object.Value{}, err
			}
			out[i] = v
		}
		return object.NewList(out...), nil
	})
	predicate := func(name string, f func(p parts) bool) {
		c.Define(name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Bool(f(read(self))), nil
		})
	}
	predicate("absolute?", func(p parts) bool { return p.scheme != nil })
	predicate("absolute", func(p parts) bool { return p.scheme != nil })
	predicate("relative?", func(p parts) bool { return p.scheme == nil })
	predicate("hierarchical?", func(p parts) bool { return p.path != nil })
	c.Define("to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(toS(self)), nil
	})
	c.Define("inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String("#<" + self.Instance().Cls.QualifiedName() + " " + toS(self) + ">"), nil
	})
	c.Define("==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		eq, err := equal(ctx, self, args[0])
		return object.Bool(eq), err
	})
	c.Define("normalize", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return normalize(self), nil
	})
	mergeFn := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return merge(ctx, self, args[0])
	}
	c.Define("merge", 1, mergeFn)
	c.Define("+", 1, mergeFn)
	routeFromFn := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return routeFrom(ctx, self, args[0])
	}
	c.Define("route_from", 1, routeFromFn)
	c.Define("-", 1, routeFromFn)
	c.Define("route_to", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		oth, err := toURI(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return routeFrom(ctx, oth, self)
	})
	c.Define("coerce", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		oth, err := toURI(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(oth, self), nil
	})
	// find_proxy reads the proxy environment variables in Ruby; cameo has
	// no ENV, so a script sees no proxy.
	c.Define("find_proxy", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Nil, nil
	})

	// A URI has no setters. u.host = "x", which cameo reads as with, and
	// with itself are refused: a different URI is parsed, joined, merged or
	// built.
	c.Define("with", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name := "with"
		if kw != nil && kw.Len() > 0 {
			if sym, ok := kw.Keys()[0].AsSymbol(); ok {
				name = string(sym) + "="
			}
		}
		return object.Value{}, ctx.Raise(object.NoMethodError, "undefined method '%s' for an instance of %s: a URI is parsed, joined, merged or built, not changed", name, self.Instance().Cls.QualifiedName())
	})
}

// userPassword splits the userinfo at its first colon, as Ruby stores
// user and password apart.
func userPassword(p parts) (user, password *string) {
	if p.userinfo == nil {
		return nil, nil
	}
	u, pw, found := strings.Cut(*p.userinfo, ":")
	if !found {
		return str(u), nil
	}
	return str(u), str(pw)
}

func joinUserinfo(user, password *string) *string {
	if user == nil {
		return nil
	}
	if password == nil {
		return user
	}
	return str(*user + ":" + *password)
}

func strOrNil(s *string) object.Value {
	if s == nil {
		return object.Nil
	}
	return object.String(*s)
}

// componentStrings reads new's nine positional components: a String, an
// Integer for the port, or nil.
func componentStrings(ctx object.Context, args []object.Value) ([]*string, error) {
	out := make([]*string, 9)
	for i, a := range args {
		switch {
		case a.IsNil():
		case a.IsString() || a.Kind() == object.KindSymbol:
			out[i] = str(a.Str())
		case i == 3:
			n, err := stdlib.WantInt(ctx, a, "")
			if err != nil {
				return nil, err
			}
			out[i] = str(strconv.FormatInt(n, 10))
		default:
			return nil, ctx.Raise(object.TypeError, "no implicit conversion of %s into String", stdlib.KindName(a))
		}
	}
	return out, nil
}

// buildCall is Generic.build and its subclasses': the components as a Hash
// by name or an Array in the class's component order, checked as new
// checks them.
func buildCall(ctx object.Context, cls *object.Class, arg object.Value) (object.Value, error) {
	names := componentsOf(cls)
	byName := map[string]object.Value{}
	if l, ok := arg.AsList(); ok && cls.IsA(mailtoClass) && len(l.Elems) >= 1 && len(l.Elems) <= 2 {
		// MailTo.build takes [to] or [to, headers].
		byName["to"] = l.Elems[0]
		if len(l.Elems) == 2 {
			byName["headers"] = l.Elems[1]
		}
		return buildMailTo(ctx, cls, byName)
	}
	if m, ok := arg.AsMap(); ok {
		for i, k := range m.Keys() {
			n, ok := k.AsFieldName()
			if !ok {
				return object.Value{}, ctx.Raise(object.ArgumentError, "expected Array of or Hash of components of %s (%s)", cls.QualifiedName(), strings.Join(names, ", "))
			}
			byName[n] = m.Values()[i]
		}
	} else if l, ok := arg.AsList(); ok && len(l.Elems) == len(names)-1 {
		// The Array form takes the components but the scheme, which is
		// the class's.
		for i, n := range names[1:] {
			byName[n] = l.Elems[i]
		}
	} else {
		return object.Value{}, ctx.Raise(object.ArgumentError, "expected Array of or Hash of components of %s (%s)", cls.QualifiedName(), strings.Join(names[1:], ", "))
	}
	if cls.IsA(mailtoClass) {
		return buildMailTo(ctx, cls, byName)
	}
	comps := make([]*string, 9)
	for i, n := range components[genericClass] {
		v, ok := byName[n]
		if !ok || v.IsNil() {
			continue
		}
		if i == 3 {
			if num, isInt := v.AsInt(); isInt {
				comps[i] = str(strconv.FormatInt(num, 10))
				continue
			}
		}
		s, err := stdlib.WantString(ctx, v)
		if err != nil {
			return object.Value{}, err
		}
		comps[i] = str(s)
	}
	// The scheme is the class's, whatever the components say, as Ruby's
	// build makes it.
	if s := schemeOf(cls); s != nil || comps[0] == nil {
		comps[0] = s
	}
	return construct(ctx, cls, comps, true)
}

// schemeOf is the scheme a class is registered for, nil for Generic.
func schemeOf(cls *object.Class) *string {
	for c := cls; c != nil; c = c.Super {
		for name, sc := range schemes {
			if sc == c {
				return str(strings.ToLower(name))
			}
		}
	}
	return nil
}

// installSubclasses defines what HTTP, FTP, File, LDAP and MailTo add.
func installSubclasses() {
	h := httpClass
	h.Define("request_uri", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		p := read(self)
		if p.path == nil {
			return object.Nil, nil
		}
		url := *p.path
		if p.query != nil {
			url += "?" + *p.query
		}
		if !strings.HasPrefix(url, "/") {
			url = "/" + url
		}
		return object.String(url), nil
	})
	h.Define("authority", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(authority(self)), nil
	})
	h.Define("origin", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		p := read(self)
		sch := ""
		if p.scheme != nil {
			sch = *p.scheme
		}
		return object.String(sch + "://" + authority(self)), nil
	})

	f := ftpClass
	f.Define("typecode", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		_, tc := ftpSplit(read(self))
		return strOrNil(tc), nil
	})
	f.Define("path", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		path, _ := ftpSplit(read(self))
		return strOrNil(path), nil
	})

	l := ldapClass
	ldapPart := func(name string, i int) {
		l.Define(name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return strOrNil(ldapSplit(read(self))[i]), nil
		})
	}
	for i, name := range []string{"dn", "attributes", "scope", "filter", "extensions"} {
		ldapPart(name, i)
	}

	mt := mailtoClass
	mt.Define("to", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		to, _ := mailto(self)
		return object.String(to), nil
	})
	mt.Define("headers", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		_, headers := mailto(self)
		return headersValue(headers), nil
	})
	mailtext := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		to, headers := mailto(self)
		toLine := decodeComponent(to, true)
		var head, body strings.Builder
		for _, h := range headers {
			switch h[0] {
			case "body":
				body.Reset()
				body.WriteString(decodeComponent(h[1], true))
			case "to":
				toLine += ", " + decodeComponent(h[1], true)
			default:
				head.WriteString(capitalize(decodeComponent(h[0], true)) + ": " + decodeComponent(h[1], true) + "\n")
			}
		}
		return object.String("To: " + toLine + "\n" + head.String() + "\n" + body.String() + "\n"), nil
	}
	mt.Define("to_mailtext", 0, mailtext)
	mt.Define("to_rfc822text", 0, mailtext)
}

func authority(v object.Value) string {
	p := read(v)
	h := ""
	if p.host != nil {
		h = *p.host
	}
	if p.port != nil && (defaultPort(v.Instance().Cls) == nil || *p.port != *defaultPort(v.Instance().Cls)) {
		h += ":" + strconv.FormatInt(*p.port, 10)
	}
	return h
}

// ftpSplit reads an FTP path's ;type= suffix off it.
func ftpSplit(p parts) (path, typecode *string) {
	if p.path == nil {
		return nil, nil
	}
	path = p.path
	if i := strings.Index(*p.path, ";type="); i >= 0 {
		path, typecode = str((*p.path)[:i]), str((*p.path)[i+len(";type="):])
	}
	// An absolute path is spelled %2F at the start, as RFC 1738 has it.
	if len(*path) >= 3 && strings.EqualFold((*path)[:3], "%2F") {
		path = str("/" + (*path)[3:])
	}
	return path, typecode
}

// ldapSplit reads an LDAP URL's dn from its path and attributes, scope,
// filter and extensions from its query, each nil when absent.
func ldapSplit(p parts) [5]*string {
	var out [5]*string
	if p.path != nil && *p.path != "" {
		out[0] = str(strings.TrimPrefix(*p.path, "/"))
	}
	if p.query != nil {
		for i, part := range strings.SplitN(*p.query, "?", 4) {
			if part != "" {
				out[i+1] = str(part)
			}
		}
	}
	return out
}

var mailtoToRe = regexp.MustCompile(`^(?:%[0-9A-Fa-f]{2}|[!$&-.0-9:;=@-Z_a-z~])*$`)

// newMailTo is MailTo.new with the nine components: the addresses and the
// headers come from the opaque part.
func newMailTo(ctx object.Context, cls *object.Class, args []object.Value) (object.Value, error) {
	if len(args) < 9 || len(args) > 11 {
		return object.Value{}, stdlib.ArgError(ctx, len(args), "9..11")
	}
	comps, err := componentStrings(ctx, args[:9])
	if err != nil {
		return object.Value{}, err
	}
	comps[0] = str("mailto")
	opaque := ""
	if comps[6] != nil {
		opaque = *comps[6]
	}
	if comps[7] != nil {
		opaque += "?" + *comps[7]
	}
	comps[6], comps[7] = str(opaque), nil
	v, err := construct(ctx, cls, comps, false)
	if err != nil {
		return object.Value{}, err
	}
	to, _ := mailto(v)
	if !mailtoToRe.MatchString(to) {
		return object.Value{}, ctx.Raise(invalidComp, "unrecognised opaque part for mailtoURL: %s", opaque)
	}
	return v, nil
}

// buildMailTo is MailTo.build: the addresses and the headers, the latter
// as a String of the query or as pairs.
func buildMailTo(ctx object.Context, cls *object.Class, byName map[string]object.Value) (object.Value, error) {
	to := ""
	if v, ok := byName["to"]; ok && !v.IsNil() {
		s, err := stdlib.WantString(ctx, v)
		if err != nil {
			return object.Value{}, err
		}
		to = s
	}
	var headers [][2]string
	if v, ok := byName["headers"]; ok && !v.IsNil() {
		var err error
		if headers, err = headersArg(ctx, v); err != nil {
			return object.Value{}, err
		}
	}
	if !mailtoToRe.MatchString(to) {
		return object.Value{}, ctx.Raise(invalidComp, "bad component(expected opaque component): %s", to)
	}
	return mailtoValue(cls, parts{}, to, headers), nil
}

// headersArg reads MailTo headers: a query String, or an Array of pairs.
func headersArg(ctx object.Context, v object.Value) ([][2]string, error) {
	if s, ok := v.AsString(); ok {
		var out [][2]string
		for _, h := range strings.Split(s, "&") {
			if h == "" {
				continue
			}
			k, val, _ := strings.Cut(h, "=")
			out = append(out, [2]string{k, val})
		}
		return out, nil
	}
	pairs, err := stdlib.ToList(ctx, v)
	if err != nil {
		return nil, err
	}
	var out [][2]string
	for _, pair := range pairs {
		kv, err := stdlib.ToList(ctx, pair)
		if err != nil {
			return nil, err
		}
		if len(kv) != 2 {
			return nil, ctx.Raise(object.ArgumentError, "a mailto header is a pair of a name and a value")
		}
		k, err := stdlib.WantString(ctx, kv[0])
		if err != nil {
			return nil, err
		}
		val, err := stdlib.WantString(ctx, kv[1])
		if err != nil {
			return nil, err
		}
		out = append(out, [2]string{k, val})
	}
	return out, nil
}

// mailtoValue is a MailTo of the addresses and headers, with the opaque
// part they make.
func mailtoValue(cls *object.Class, p parts, to string, headers [][2]string) object.Value {
	opaque := to
	if len(headers) > 0 {
		hs := make([]string, len(headers))
		for i, h := range headers {
			hs[i] = h[0] + "=" + h[1]
		}
		opaque += "?" + strings.Join(hs, "&")
	}
	p.scheme = str("mailto")
	p.opaque = str(opaque)
	p.path, p.query, p.host, p.port, p.userinfo = nil, nil, nil, nil, nil
	return build(cls, p)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

// installForms defines the www-form and URI-component encoders.
func installForms() {
	m := uriModule
	m.DefineSingleton("encode_www_form_component", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		s, err := ctx.ToS(args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.String(encodeComponent(s, true)), nil
	})
	m.DefineSingleton("encode_uri_component", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		s, err := ctx.ToS(args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.String(encodeComponent(s, false)), nil
	})
	decode := func(plus bool) fn {
		return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) < 1 || len(args) > 2 {
				return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
			}
			s, err := stdlib.WantString(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			if badEscapeRe.MatchString(s) {
				return object.Value{}, ctx.Raise(object.ArgumentError, "invalid %%-encoding (%s)", s)
			}
			return object.String(decodeComponent(s, plus)), nil
		}
	}
	m.DefineSingleton("decode_www_form_component", -1, decode(true))
	m.DefineSingleton("decode_uri_component", -1, decode(false))
	m.DefineSingleton("encode_www_form", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		arg, err := oneOrKeywords(ctx, args, kw)
		if err != nil {
			return object.Value{}, err
		}
		pairs, err := formPairs(ctx, arg)
		if err != nil {
			return object.Value{}, err
		}
		var out []string
		for _, kv := range pairs {
			k, err := ctx.ToS(kv[0])
			if err != nil {
				return object.Value{}, err
			}
			key := encodeComponent(k, true)
			switch {
			case kv[1].IsNil():
				out = append(out, key)
			case kv[1].Kind() == object.KindList:
				for _, w := range kv[1].List().Elems {
					// Ruby's block returns nil for a nil element, which
					// join writes as nothing.
					if w.IsNil() {
						out = append(out, "")
						continue
					}
					ws, err := ctx.ToS(w)
					if err != nil {
						return object.Value{}, err
					}
					out = append(out, key+"="+encodeComponent(ws, true))
				}
			default:
				vs, err := ctx.ToS(kv[1])
				if err != nil {
					return object.Value{}, err
				}
				out = append(out, key+"="+encodeComponent(vs, true))
			}
		}
		return object.String(strings.Join(out, "&")), nil
	})
	m.DefineSingleton("decode_www_form", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if !object.ASCIIOnly(s) {
			return object.Value{}, ctx.Raise(object.ArgumentError, "the input of URI.decode_www_form must be ASCII only string")
		}
		sep := "&"
		isindex := false
		if kw != nil {
			if v, ok := kw.Get(object.Symbol("separator")); ok {
				if sep, err = stdlib.WantString(ctx, v); err != nil {
					return object.Value{}, err
				}
			}
			if v, ok := kw.Get(object.Symbol("isindex")); ok {
				isindex = v.Truthy()
			}
		}
		var out []object.Value
		if s == "" {
			return object.NewList(), nil
		}
		for _, line := range eachLine(s, sep) {
			line = strings.TrimSuffix(line, sep)
			key, val, found := strings.Cut(line, "=")
			if isindex {
				if !found {
					key, val = "", key
				}
				isindex = false
			}
			out = append(out, object.NewList(object.String(decodeComponent(key, true)), object.String(decodeComponent(val, true))))
		}
		return object.NewList(out...), nil
	})
}

// oneOrKeywords is the one positional argument of a call, or its keywords
// as a Hash when it has none: what a method taking a Hash sees when a
// script writes the Hash as keywords.
func oneOrKeywords(ctx object.Context, args []object.Value, kw *object.Map) (object.Value, error) {
	if len(args) == 0 && kw != nil {
		return object.MapValue(kw), nil
	}
	if len(args) == 0 {
		return object.Value{}, stdlib.ArgError(ctx, 0, "1")
	}
	return args[0], nil
}

// eachLine is String#each_line(sep): the pieces of s, each ending in sep
// but perhaps the last.
func eachLine(s, sep string) []string {
	if sep == "" {
		return []string{s}
	}
	var out []string
	for s != "" {
		i := strings.Index(s, sep)
		if i < 0 {
			out = append(out, s)
			break
		}
		out = append(out, s[:i+len(sep)])
		s = s[i+len(sep):]
	}
	return out
}

// formPairs reads encode_www_form's argument: a Hash, or an Array of
// pairs.
func formPairs(ctx object.Context, v object.Value) ([][2]object.Value, error) {
	if m, ok := v.AsMap(); ok {
		out := make([][2]object.Value, m.Len())
		for i, k := range m.Keys() {
			out[i] = [2]object.Value{k, m.Values()[i]}
		}
		return out, nil
	}
	elems, err := stdlib.ToList(ctx, v)
	if err != nil {
		return nil, err
	}
	out := make([][2]object.Value, len(elems))
	for i, e := range elems {
		kv, err := stdlib.ToList(ctx, e)
		if err != nil {
			return nil, err
		}
		pair := [2]object.Value{object.Nil, object.Nil}
		copy(pair[:], kv)
		out[i] = pair
	}
	return out, nil
}

// encodeComponent is Go's url.QueryEscape: a space is + for a form and
// %20 for a URI component. Go keeps ~ and escapes *, where Ruby does the
// opposite.
func encodeComponent(s string, form bool) string {
	e := url.QueryEscape(s)
	if !form {
		e = strings.ReplaceAll(e, "+", "%20")
	}
	return e
}

// decodeComponent decodes every %XX escape, and a + as a space in a
// form, and leaves anything else as it is, as Ruby's decode_www_form
// does: one bad escape does not keep the good ones from decoding.
func decodeComponent(s string, form bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '+' && form:
			b.WriteByte(' ')
		case c == '%' && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]):
			b.WriteByte(unhex(s[i+1])<<4 | unhex(s[i+2]))
			i += 2
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// installParser gives URI::DEFAULT_PARSER the methods a script calls on
// it: parse, split, join and the component regexps' checks.
func installParser() {
	c := parserClass
	c.Define("parse", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return parse(ctx, args[0])
	})
	c.Define("split", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return splitCall(ctx, args[0])
	})
	c.Define("join", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return join(ctx, args)
	})
	c.Define("inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String("#<URI::RFC3986_Parser>"), nil
	})
}

func unhex(c byte) byte {
	switch {
	case c >= 'a':
		return c - 'a' + 10
	case c >= 'A':
		return c - 'A' + 10
	}
	return c - '0'
}
