package http

import (
	"encoding/base64"
	"sort"
	"strconv"
	"strings"

	"github.com/cameo-lang/cameo/src/lib/uri"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

// The request classes: Net::HTTPGenericRequest, Net::HTTPRequest and
// Net::HTTP::Get and kin, each with its METHOD and whether a request and a
// response of it have a body. A request's fields are its method, path, the
// URI it was made from, its headers (a map of lower-case names to values,
// several joined by ", " as Ruby's [] returns them) and its body.

var (
	genericRequestClass = object.NewClass("HTTPGenericRequest", object.ObjectClass)
	requestClass        = object.NewClass("HTTPRequest", genericRequestClass)
	headerModule        = object.NewModule("HTTPHeader")
	requestClasses      = map[string]*object.Class{}
)

// verbs are Ruby's request classes: the method, and whether a request and
// a response of it carry a body.
var verbs = []struct {
	name, method     string
	reqBody, resBody bool
}{
	{"Get", "GET", false, true}, {"Head", "HEAD", false, false}, {"Post", "POST", true, true}, {"Put", "PUT", true, true},
	{"Delete", "DELETE", false, true}, {"Options", "OPTIONS", false, true}, {"Trace", "TRACE", false, true},
	{"Patch", "PATCH", true, true}, {"Propfind", "PROPFIND", true, true}, {"Proppatch", "PROPPATCH", true, true},
	{"Mkcol", "MKCOL", true, true}, {"Copy", "COPY", false, true}, {"Move", "MOVE", false, true}, {"Lock", "LOCK", true, true},
	{"Unlock", "UNLOCK", true, true},
}

const (
	rMethod = iota
	rPath
	rURI
	rHeaders
	rBody
	rFieldCount
)

// headers are a request's or response's header fields by lower-case name.
type headers struct {
	names  []string
	values map[string]string
}

func newHeaders() *headers { return &headers{values: map[string]string{}} }

func (h *headers) Set(name, value string) {
	key := strings.ToLower(name)
	if _, ok := h.values[key]; !ok {
		h.names = append(h.names, key)
	}
	h.values[key] = value
}

func (h *headers) Get(name string) (string, bool) {
	v, ok := h.values[strings.ToLower(name)]
	return v, ok
}

func (h *headers) Has(name string) bool { _, ok := h.Get(name); return ok }

func (h *headers) Delete(name string) {
	key := strings.ToLower(name)
	if _, ok := h.values[key]; !ok {
		return
	}
	delete(h.values, key)
	for i, n := range h.names {
		if n == key {
			h.names = append(h.names[:i], h.names[i+1:]...)
			break
		}
	}
}

// Add reads a Hash of header fields, as initheader and the verbs take it.
func (h *headers) Add(ctx object.Context, v object.Value) error {
	m, ok := v.AsMap()
	if !ok {
		return ctx.Raise(object.TypeError, "no implicit conversion of %s into Hash", stdlib.KindName(v))
	}
	for i, k := range m.Keys() {
		name, err := stdlib.WantString(ctx, k)
		if err != nil {
			return err
		}
		val := m.Values()[i]
		if val.IsNil() {
			h.Delete(name)
			continue
		}
		s, err := headerValue(ctx, val)
		if err != nil {
			return err
		}
		h.Set(name, s)
	}
	return nil
}

// headerValue is a header value as a String: a String as it is, an Array
// joined with ", ", anything else through to_s.
func headerValue(ctx object.Context, v object.Value) (string, error) {
	if l, ok := v.AsList(); ok {
		parts := make([]string, len(l.Elems))
		for i, e := range l.Elems {
			s, err := ctx.ToS(e)
			if err != nil {
				return "", err
			}
			parts[i] = s
		}
		return strings.Join(parts, ", "), nil
	}
	return ctx.ToS(v)
}

func (h *headers) toMap() object.Value {
	b := object.NewMapBuilder()
	for _, n := range h.names {
		b.Set(object.String(n), object.String(h.values[n]))
	}
	return object.MapValue(b.Map())
}

func headersOf(v object.Value) *headers {
	h := newHeaders()
	m, ok := v.AsMap()
	if !ok {
		return h
	}
	for i, k := range m.Keys() {
		h.Set(k.Str(), m.Values()[i].Str())
	}
	return h
}

// request is a request as Go reads it.
type request struct {
	cls     *object.Class
	method  string
	path    string
	uri     *uri.Components
	headers *headers
	body    *string
}

// newRequest is Net::HTTP::Get.new and kin: the default headers Ruby
// sends, and the Host of a URI.
func newRequest(cls *object.Class, method, path string, u *uri.Components) *request {
	r := &request{cls: cls, method: method, path: path, uri: u, headers: newHeaders()}
	r.headers.Set("accept-encoding", "gzip;q=1.0,deflate;q=0.6,identity;q=0.3")
	r.headers.Set("accept", "*/*")
	r.headers.Set("user-agent", "Ruby")
	if u != nil && u.Host != "" {
		host := u.Host
		if u.HasPort && !(u.Scheme == "https" && u.Port == 443 || u.Scheme == "http" && u.Port == 80) {
			host += ":" + strconv.FormatInt(u.Port, 10)
		}
		r.headers.Set("host", host)
	}
	return r
}

func (r *request) value() object.Value {
	fields := make([]object.Value, len(r.cls.Fields))
	fields[rMethod] = object.String(r.method)
	fields[rPath] = object.String(r.path)
	fields[rURI] = object.Nil
	if r.uri != nil {
		fields[rURI] = object.String(uriString(*r.uri))
	}
	fields[rHeaders] = r.headers.toMap()
	fields[rBody] = object.Nil
	if r.body != nil {
		fields[rBody] = object.String(*r.body)
	}
	return object.InstanceValue(&object.Instance{Cls: r.cls, Fields: fields})
}

// uriString is the URI a request was made from, as a String.
func uriString(u uri.Components) string {
	var b strings.Builder
	b.WriteString(u.Scheme + "://")
	if u.Userinfo != "" {
		b.WriteString(u.Userinfo + "@")
	}
	b.WriteString(u.Host)
	if u.HasPort && !(u.Scheme == "https" && u.Port == 443 || u.Scheme == "http" && u.Port == 80) {
		b.WriteString(":" + strconv.FormatInt(u.Port, 10))
	}
	b.WriteString(u.Path)
	if u.Query != "" {
		b.WriteString("?" + u.Query)
	}
	if u.Fragment != "" {
		b.WriteString("#" + u.Fragment)
	}
	return b.String()
}

func requestOf(v object.Value) (*request, bool) {
	inst, ok := v.AsInstance()
	if !ok || !inst.Cls.IsA(genericRequestClass) {
		return nil, false
	}
	f := inst.Fields
	r := &request{cls: inst.Cls, method: f[rMethod].Str(), path: f[rPath].Str(), headers: headersOf(f[rHeaders])}
	if !f[rURI].IsNil() {
		if c, ok := parseURI(f[rURI].Str()); ok {
			r.uri = &c
		}
	}
	if !f[rBody].IsNil() {
		body := f[rBody].Str()
		r.body = &body
	}
	return r, true
}

// parseURI reads a URI string without a Context: a request keeps the URI
// it was made from as text.
func parseURI(s string) (uri.Components, bool) {
	v, err := uri.Parse(quietContext{}, s)
	if err != nil {
		return uri.Components{}, false
	}
	return uri.Of(v)
}

// uriOf reads a URI argument: a URI::HTTP, or a String parsed as one.
func uriOf(v object.Value) (*uri.Components, bool) {
	if c, ok := uri.Of(v); ok {
		return &c, c.Host != ""
	}
	if s, ok := v.AsString(); ok {
		if c, ok := parseURI(s); ok && c.Host != "" {
			return &c, true
		}
	}
	return nil, false
}

// connOf is the connection a URI names.
func connOf(u *uri.Components) conn {
	ssl := u.Scheme == "https"
	var port *int64
	if u.HasPort {
		p := u.Port
		port = &p
	}
	return newConn(u.Host, port, ssl)
}

// formData is URI.encode_www_form of a Hash or an Array of pairs, with sep
// between the pairs.
func formData(ctx object.Context, v object.Value, sep string) (string, error) {
	var pairs [][2]object.Value
	if m, ok := v.AsMap(); ok {
		for i, k := range m.Keys() {
			pairs = append(pairs, [2]object.Value{k, m.Values()[i]})
		}
	} else {
		elems, err := stdlib.ToList(ctx, v)
		if err != nil {
			return "", err
		}
		for _, e := range elems {
			kv, err := stdlib.ToList(ctx, e)
			if err != nil {
				return "", err
			}
			pair := [2]object.Value{object.Nil, object.Nil}
			copy(pair[:], kv)
			pairs = append(pairs, pair)
		}
	}
	var out []string
	for _, kv := range pairs {
		k, err := ctx.ToS(kv[0])
		if err != nil {
			return "", err
		}
		if l, ok := kv[1].AsList(); ok {
			for _, w := range l.Elems {
				ws, err := ctx.ToS(w)
				if err != nil {
					return "", err
				}
				out = append(out, uri.Encode(k)+"="+uri.Encode(ws))
			}
			continue
		}
		if kv[1].IsNil() {
			out = append(out, uri.Encode(k))
			continue
		}
		vs, err := ctx.ToS(kv[1])
		if err != nil {
			return "", err
		}
		out = append(out, uri.Encode(k)+"="+uri.Encode(vs))
	}
	return strings.Join(out, sep), nil
}

// installRequests defines the request classes and the header methods
// requests and responses share.
func installRequests() {
	for _, c := range []*object.Class{genericRequestClass, requestClass, headerModule} {
		c.Lexical = netModule
		netModule.Consts[c.Name] = object.ClassValue(c)
	}
	declare(genericRequestClass, []struct{ name, typ string }{{"method", "string"}, {"path", "string"}, {"uri", "string"}})
	genericRequestClass.FieldIndex["headers"] = len(genericRequestClass.Fields)
	genericRequestClass.Fields = append(genericRequestClass.Fields, mapField(genericRequestClass, "headers"))
	declare(genericRequestClass, []struct{ name, typ string }{{"body", "string"}})
	inherit(requestClass)
	genericRequestClass.Include(headerModule)
	for _, v := range verbs {
		c := object.NewClass(v.name, requestClass)
		c.Lexical = httpClass
		httpClass.Consts[v.name] = object.ClassValue(c)
		inherit(c)
		c.Consts["METHOD"] = object.String(v.method)
		c.Consts["REQUEST_HAS_BODY"] = object.Bool(v.reqBody)
		c.Consts["RESPONSE_HAS_BODY"] = object.Bool(v.resBody)
		requestClasses[v.method] = c
	}

	// Get.new(uri_or_path, initheader = nil).
	requestClass.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		a = withKeywords(a, kw)
		if len(a) < 1 || len(a) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(a), "1..2")
		}
		cls := self.Cls()
		method := "GET"
		if m, ok := cls.Consts["METHOD"]; ok {
			method = m.Str()
		}
		var r *request
		if u, ok := uri.Of(a[0]); ok {
			if u.Host == "" {
				return object.Value{}, ctx.Raise(object.ArgumentError, "not an HTTP URI")
			}
			r = newRequest(cls, method, u.RequestURI, &u)
		} else {
			path, err := stdlib.WantString(ctx, a[0])
			if err != nil {
				return object.Value{}, err
			}
			if path == "" || !strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "*") {
				return object.Value{}, ctx.Raise(object.ArgumentError, "HTTP request path is empty")
			}
			r = newRequest(cls, method, path, nil)
		}
		if len(a) == 2 && !a[1].IsNil() {
			if err := r.headers.Add(ctx, a[1]); err != nil {
				return object.Value{}, err
			}
		}
		return r.value(), nil
	})

	g := genericRequestClass
	reader := func(name string, f func(r *request) object.Value) {
		g.Define(name, 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			r, _ := requestOf(self)
			return f(r), nil
		})
	}
	reader("method", func(r *request) object.Value { return object.String(r.method) })
	reader("path", func(r *request) object.Value { return object.String(r.path) })
	reader("uri", func(r *request) object.Value {
		if r.uri == nil {
			return object.Nil
		}
		v, err := uri.Parse(quietContext{}, uriString(*r.uri))
		if err != nil {
			return object.Nil
		}
		return v
	})
	reader("body", func(r *request) object.Value {
		if r.body == nil {
			return object.Nil
		}
		return object.String(*r.body)
	})
	reader("body_stream", func(r *request) object.Value { return object.Nil })
	reader("request_body_permitted?", func(r *request) object.Value {
		v, ok := r.cls.Consts["REQUEST_HAS_BODY"]
		return object.Bool(ok && v.Truthy())
	})
	reader("response_body_permitted?", func(r *request) object.Value {
		v, ok := r.cls.Consts["RESPONSE_HAS_BODY"]
		return object.Bool(!ok || v.Truthy())
	})
	reader("inspect", func(r *request) object.Value {
		return object.String("#<" + r.cls.QualifiedName() + " " + r.method + ">")
	})
	// A changed request is a copy; the setters return it.
	update := func(name string, arity int, f func(ctx object.Context, r *request, a []object.Value) error) {
		g.Define(name, arity, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			a = withKeywords(a, kw)
			if len(a) == 0 {
				return object.Value{}, stdlib.ArgError(ctx, 0, "1")
			}
			r, _ := requestOf(self)
			if err := f(ctx, r, a); err != nil {
				return object.Value{}, err
			}
			return r.value(), nil
		})
	}
	update("body=", 1, func(ctx object.Context, r *request, a []object.Value) error {
		if a[0].IsNil() {
			r.body = nil
			return nil
		}
		s, err := stdlib.WantString(ctx, a[0])
		r.body = &s
		return err
	})
	setForm := func(ctx object.Context, r *request, a []object.Value) error {
		sep := "&"
		if len(a) == 2 {
			s, err := stdlib.WantString(ctx, a[1])
			if err != nil {
				return err
			}
			sep = s
		}
		body, err := formData(ctx, a[0], sep)
		if err != nil {
			return err
		}
		r.body = &body
		r.headers.Set("content-type", "application/x-www-form-urlencoded")
		return nil
	}
	update("set_form_data", -1, setForm)
	update("form_data=", 1, setForm)
	update("basic_auth", 2, func(ctx object.Context, r *request, a []object.Value) error {
		return auth(ctx, r, "authorization", a)
	})
	update("proxy_basic_auth", 2, func(ctx object.Context, r *request, a []object.Value) error {
		return auth(ctx, r, "proxy-authorization", a)
	})
	installHeaderMethods(headerModule, func(v object.Value) *headers {
		r, _ := requestOf(v)
		return r.headers
	}, func(v object.Value, h *headers) object.Value {
		r, _ := requestOf(v)
		r.headers = h
		return r.value()
	})
}

// auth sets a Basic authorization header.
func auth(ctx object.Context, r *request, field string, a []object.Value) error {
	user, err := stdlib.WantString(ctx, a[0])
	if err != nil {
		return err
	}
	pass, err := stdlib.WantString(ctx, a[1])
	if err != nil {
		return err
	}
	r.headers.Set(field, "Basic "+base64.StdEncoding.EncodeToString([]byte(user+":"+pass)))
	return nil
}

// installHeaderMethods defines Net::HTTPHeader's methods on a module a
// request class and the response class include: read reads the headers
// of an instance, write returns the instance with headers changed.
func installHeaderMethods(m *object.Class, read func(v object.Value) *headers, write func(v object.Value, h *headers) object.Value) {
	m.Define("[]", 1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := stdlib.WantString(ctx, a[0])
		if err != nil {
			return object.Value{}, err
		}
		if v, ok := read(self).Get(name); ok {
			return object.String(v), nil
		}
		return object.Nil, nil
	})
	m.Define("[]=", 2, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := stdlib.WantString(ctx, a[0])
		if err != nil {
			return object.Value{}, err
		}
		h := read(self)
		if a[1].IsNil() {
			h.Delete(name)
		} else {
			v, err := headerValue(ctx, a[1])
			if err != nil {
				return object.Value{}, err
			}
			h.Set(name, v)
		}
		return write(self, h), nil
	})
	m.Define("add_field", 2, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := stdlib.WantString(ctx, a[0])
		if err != nil {
			return object.Value{}, err
		}
		v, err := headerValue(ctx, a[1])
		if err != nil {
			return object.Value{}, err
		}
		h := read(self)
		if old, ok := h.Get(name); ok {
			v = old + ", " + v
		}
		h.Set(name, v)
		return write(self, h), nil
	})
	m.Define("delete", 1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := stdlib.WantString(ctx, a[0])
		if err != nil {
			return object.Value{}, err
		}
		h := read(self)
		h.Delete(name)
		return write(self, h), nil
	})
	m.Define("key?", 1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := stdlib.WantString(ctx, a[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.Bool(read(self).Has(name)), nil
	})
	m.Define("fetch", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(a) < 1 || len(a) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(a), "1..2")
		}
		name, err := stdlib.WantString(ctx, a[0])
		if err != nil {
			return object.Value{}, err
		}
		if v, ok := read(self).Get(name); ok {
			return object.String(v), nil
		}
		if len(a) == 2 {
			return a[1], nil
		}
		if !block.IsZero() {
			return ctx.Yield(block, a[0])
		}
		return object.Value{}, ctx.Raise(object.KeyError, "key not found: %s", object.QuoteString(name))
	})
	m.Define("get_fields", 1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := stdlib.WantString(ctx, a[0])
		if err != nil {
			return object.Value{}, err
		}
		v, ok := read(self).Get(name)
		if !ok {
			return object.Nil, nil
		}
		return stdlib.StringList(strings.Split(v, ", ")), nil
	})
	// The each methods yield to a block, and without one return an
	// Enumerator over what they would yield, as Ruby's do.
	eachOf := func(name string, items func(h *headers) [][]object.Value) {
		m.Define(name, 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			h := read(self)
			if block.IsZero() {
				return object.EnumeratorValue(&object.Enumerator{Recv: self, Meth: name, Each: func(ctx object.Context, yield func(object.Value) error) error {
					for _, item := range items(h) {
						if err := yield(object.NewList(item...)); err != nil {
							return err
						}
					}
					return nil
				}}), nil
			}
			for _, item := range items(h) {
				if _, err := ctx.Yield(block, item...); err != nil {
					return object.Value{}, err
				}
			}
			return self, nil
		})
	}
	pairs := func(h *headers) [][]object.Value {
		var out [][]object.Value
		for _, n := range h.names {
			out = append(out, []object.Value{object.String(n), object.String(h.values[n])})
		}
		return out
	}
	eachOf("each_header", pairs)
	eachOf("each", pairs)
	eachOf("each_capitalized", func(h *headers) [][]object.Value {
		var out [][]object.Value
		for _, n := range h.names {
			out = append(out, []object.Value{object.String(capitalize(n)), object.String(h.values[n])})
		}
		return out
	})
	names := func(h *headers) [][]object.Value {
		var out [][]object.Value
		for _, n := range h.names {
			out = append(out, []object.Value{object.String(n)})
		}
		return out
	}
	eachOf("each_name", names)
	eachOf("each_key", names)
	eachOf("each_value", func(h *headers) [][]object.Value {
		var out [][]object.Value
		for _, n := range h.names {
			out = append(out, []object.Value{object.String(h.values[n])})
		}
		return out
	})
	m.Define("to_hash", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		h := read(self)
		b := object.NewMapBuilder()
		for _, n := range h.names {
			b.Set(object.String(n), stdlib.StringList(strings.Split(h.values[n], ", ")))
		}
		return object.MapValue(b.Map()), nil
	})
	m.Define("length", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(int64(len(read(self).names))), nil
	})
	m.Define("size", 0, m.Methods["length"].Native)
	m.Define("content_type", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		main, sub, _ := contentType(read(self))
		if main == "" {
			return object.Nil, nil
		}
		return object.String(main + "/" + sub), nil
	})
	m.Define("main_type", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		main, _, _ := contentType(read(self))
		if main == "" {
			return object.Nil, nil
		}
		return object.String(main), nil
	})
	m.Define("sub_type", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		main, sub, _ := contentType(read(self))
		if main == "" {
			return object.Nil, nil
		}
		return object.String(sub), nil
	})
	m.Define("type_params", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		_, _, params := contentType(read(self))
		b := object.NewMapBuilder()
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.Set(object.String(k), object.String(params[k]))
		}
		return object.MapValue(b.Map()), nil
	})
	m.Define("charset", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		_, _, params := contentType(read(self))
		if c, ok := params["charset"]; ok {
			return object.String(c), nil
		}
		return object.Nil, nil
	})
	m.Define("content_length", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		v, ok := read(self).Get("content-length")
		if !ok {
			return object.Nil, nil
		}
		digits := strings.TrimSpace(v)
		if !allDigits(digits) {
			return object.Value{}, ctx.Raise(httpHeaderSyntax, "wrong Content-Length format")
		}
		n, _ := strconv.ParseInt(digits, 10, 64)
		return object.Int(n), nil
	})
	m.Define("chunked?", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		v, ok := read(self).Get("transfer-encoding")
		return object.Bool(ok && strings.Contains(strings.ToLower(v), "chunked")), nil
	})
	m.Define("content_type=", 1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := stdlib.WantString(ctx, a[0])
		if err != nil {
			return object.Value{}, err
		}
		h := read(self)
		h.Set("content-type", s)
		return write(self, h), nil
	})
	m.Define("set_content_type", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(a) < 1 || len(a) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(a), "1..2")
		}
		s, err := stdlib.WantString(ctx, a[0])
		if err != nil {
			return object.Value{}, err
		}
		if len(a) == 2 {
			m, ok := a[1].AsMap()
			if !ok {
				return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into Hash", stdlib.KindName(a[1]))
			}
			for i, k := range m.Keys() {
				ks, err := ctx.ToS(k)
				if err != nil {
					return object.Value{}, err
				}
				vs, err := ctx.ToS(m.Values()[i])
				if err != nil {
					return object.Value{}, err
				}
				s += "; " + ks + "=" + vs
			}
		}
		h := read(self)
		h.Set("content-type", s)
		return write(self, h), nil
	})
	m.Define("content_length=", 1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		h := read(self)
		if a[0].IsNil() {
			h.Delete("content-length")
		} else {
			n, err := stdlib.WantInt(ctx, a[0], "")
			if err != nil {
				return object.Value{}, err
			}
			h.Set("content-length", strconv.FormatInt(n, 10))
		}
		return write(self, h), nil
	})
	m.Define("range", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		v, ok := read(self).Get("range")
		if !ok {
			return object.Nil, nil
		}
		return object.String(v), nil
	})
	m.Define("connection_close?", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		v, _ := read(self).Get("connection")
		return object.Bool(strings.EqualFold(strings.TrimSpace(v), "close")), nil
	})
	m.Define("connection_keep_alive?", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		v, _ := read(self).Get("connection")
		return object.Bool(strings.EqualFold(strings.TrimSpace(v), "keep-alive")), nil
	})
}

// contentType splits a Content-Type header into its type, subtype and
// parameters.
func contentType(h *headers) (main, sub string, params map[string]string) {
	v, ok := h.Get("content-type")
	params = map[string]string{}
	if !ok {
		return "", "", params
	}
	parts := strings.Split(v, ";")
	mt := strings.TrimSpace(parts[0])
	main, sub, found := strings.Cut(mt, "/")
	if !found {
		sub = ""
	}
	for _, p := range parts[1:] {
		k, val, found := strings.Cut(strings.TrimSpace(p), "=")
		if found {
			params[strings.ToLower(k)] = strings.Trim(val, `"`)
		}
	}
	return strings.ToLower(main), strings.ToLower(sub), params
}

func capitalize(name string) string {
	parts := strings.Split(name, "-")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "-")
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// mapField is a map<string, string> field.
func mapField(c *object.Class, name string) *object.Field {
	taken := func(number int64) bool {
		for _, f := range c.Fields {
			if f.Number == number {
				return true
			}
		}
		return false
	}
	return &object.Field{Name: name, Number: protogenNumber(name, taken), Type: &object.FieldType{Kind: "map", Key: "string", Elem: object.ScalarField("string")}, Owner: c}
}
