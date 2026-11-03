// Package openuri is Ruby's open-uri library, built into cameo: URI.open
// and URI::HTTP#open and #read, which fetch a page over net/http and
// give its body with the response's details, as OpenURI::Meta gives
// them.
//
// It follows open-uri 0.5.0, the version Ruby 3.4 ships: redirects are
// followed up to max_redirects, or refused with HTTPRedirect when
// redirect: false, the header fields of the options are sent,
// http_basic_authentication and read_timeout apply, and a response other
// than a success is OpenURI::HTTPError with the body on its io. What
// open returns is a value, OpenURI::Document, with read, base_uri,
// status, meta, metas, content_type, charset, content_encoding and
// last_modified, where Ruby returns a StringIO or a Tempfile with those
// methods mixed in; read gives a plain String, not one with the details
// on it. A file name or a file: URL is refused, since cameo reads no
// files; a proxy is the host's client's business and the proxy options
// are refused.
package openuri

import (
	gohttp "net/http"
	"regexp"
	"strconv"
	"strings"

	nethttp "github.com/cameo-lang/cameo/src/lib/net/http"
	"github.com/cameo-lang/cameo/src/lib/uri"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	module           = object.NewModule("OpenURI")
	httpError        = object.NewClass("HTTPError", object.StandardError)
	httpRedirect     = object.NewClass("HTTPRedirect", httpError)
	tooManyRedirects = object.NewClass("TooManyRedirects", httpError)
	documentClass    = object.NewClass("Document", object.ObjectClass)
)

// document is the fetched page: the Go side of an OpenURI::Document.
type document struct {
	body    object.Value // a String in the charset's encoding
	baseURI object.Value
	code    string
	message string
	names   []string
	values  map[string]string
}

func docOf(v object.Value) *document { return v.Instance().Message.(*document) }

func (d *document) value() object.Value {
	return object.InstanceValue(&object.Instance{Cls: documentClass, Message: d})
}

// options are what open takes besides the header fields.
type options struct {
	header       map[string]string
	redirect     bool
	maxRedirects int
	user, pass   string
	readTimeout  float64
	encoding     string
}

// known are open-uri's option names; another is Ruby's ArgumentError.
var known = map[string]bool{
	"proxy": true, "proxy_http_basic_authentication": true, "progress_proc": true, "content_length_proc": true,
	"http_basic_authentication": true, "read_timeout": true, "open_timeout": true, "ssl_ca_cert": true,
	"ssl_verify_mode": true, "ssl_min_version": true, "ssl_max_version": true, "ftp_active_mode": true,
	"redirect": true, "encoding": true, "max_redirects": true,
}

// readOptions takes the positional mode and options Hash, and the
// keywords.
func readOptions(ctx object.Context, args []object.Value, kw *object.Map) (*options, error) {
	o := &options{header: map[string]string{}, redirect: true, maxRedirects: 64}
	var hashes []*object.Map
	for i, a := range args {
		if s, ok := a.AsString(); ok && i == 0 {
			// The mode: r, rb, or r:encoding.
			if m := modeRe.FindStringSubmatch(s); m != nil {
				o.encoding = m[1]
				continue
			}
			return nil, ctx.Raise(object.ArgumentError, "invalid access mode %s (URI::HTTP resource is read only.)", s)
		}
		if m, ok := a.AsMap(); ok {
			hashes = append(hashes, m)
			continue
		}
		if _, ok := a.AsInt(); ok && i < 2 {
			continue // the permission bits, which a URL has none of
		}
		return nil, ctx.Raise(object.ArgumentError, "extra arguments")
	}
	if kw != nil {
		hashes = append(hashes, kw)
	}
	for _, m := range hashes {
		for i, k := range m.Keys() {
			v := m.Values()[i]
			if name, ok := k.AsString(); ok {
				s, err := ctx.ToS(v)
				if err != nil {
					return nil, err
				}
				o.header[name] = s
				continue
			}
			name, ok := k.AsSymbol()
			if !ok || !known[name] {
				inspect, _ := ctx.Inspect(k)
				return nil, ctx.Raise(object.ArgumentError, "unrecognized option: %s", inspect)
			}
			switch name {
			case "redirect":
				o.redirect = v.Truthy()
			case "max_redirects":
				n, err := stdlib.WantInt(ctx, v, "")
				if err != nil {
					return nil, err
				}
				o.maxRedirects = int(n)
			case "http_basic_authentication":
				pair, err := stdlib.ToList(ctx, v)
				if err != nil {
					return nil, err
				}
				if len(pair) != 2 {
					return nil, ctx.Raise(object.ArgumentError, "http_basic_authentication takes [user, password]")
				}
				if o.user, err = ctx.ToS(pair[0]); err != nil {
					return nil, err
				}
				if o.pass, err = ctx.ToS(pair[1]); err != nil {
					return nil, err
				}
			case "read_timeout", "open_timeout":
				if v.IsNil() {
					continue
				}
				f, ok := v.AsNumber()
				if !ok {
					return nil, ctx.Raise(object.TypeError, "%s is not a number", name)
				}
				if name == "read_timeout" {
					o.readTimeout = f
				}
			case "encoding":
				s, err := ctx.ToS(v)
				if err != nil {
					return nil, err
				}
				if o.encoding != "" {
					return nil, ctx.Raise(object.ArgumentError, "encoding specified twice")
				}
				o.encoding = s
			case "proxy", "proxy_http_basic_authentication":
				if !v.IsNil() && v.Kind() != object.KindBool {
					return nil, ctx.Raise(object.ArgumentError, "a proxy is the host's HTTP client's business: the %s option is not supported", name)
				}
			}
		}
	}
	return o, nil
}

var (
	modeRe   = regexp.MustCompile(`\Arb?(?:\z|:([^:]+))\z`)
	typeRe   = regexp.MustCompile(`\A\s*([!#$%&'*+\-.0-9A-Z^_` + "`" + `a-z|~]+)\s*/([!#$%&'*+\-.0-9A-Z^_` + "`" + `a-z|~]+)\s*(.*?)(?:;\s*)?\z`)
	paramRe  = regexp.MustCompile(`;\s*([!#$%&'*+\-.0-9A-Z^_` + "`" + `a-z|~]+)\s*=\s*(?:([!#$%&'*+\-.0-9A-Z^_` + "`" + `a-z|~]+)|("(?:[^"\\]|\\.)*"))`)
	tokenRe  = regexp.MustCompile(`[!#$%&'*+\-.0-9A-Z^_` + "`" + `a-z|~]+`)
	schemeOK = regexp.MustCompile(`(?i)\Ahttps?\z`)
)

// contentType parses the Content-Type field: the type and its parameters,
// or nil when there is none or it does not parse.
func (d *document) contentType() (string, [][2]string, bool) {
	v, ok := d.values["content-type"]
	if !ok {
		return "", nil, false
	}
	m := typeRe.FindStringSubmatch(v)
	if m == nil {
		return "", nil, false
	}
	var params [][2]string
	for _, p := range paramRe.FindAllStringSubmatch(m[3], -1) {
		val := p[2]
		if p[3] != "" {
			val = unquote(p[3][1 : len(p[3])-1])
		}
		params = append(params, [2]string{strings.ToLower(p[1]), val})
	}
	return strings.ToLower(m[1]) + "/" + strings.ToLower(m[2]), params, true
}

func unquote(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// charset is the Content-Type's charset, utf-8 for text without one, and
// "" otherwise.
func (d *document) charset() string {
	typ, params, ok := d.contentType()
	if !ok {
		return ""
	}
	for _, p := range params {
		if p[0] == "charset" {
			return strings.ToLower(p[1])
		}
	}
	if strings.HasPrefix(typ, "text/") {
		return "utf-8"
	}
	return ""
}

// encoded is the body in its encoding: UTF-8 or US-ASCII when the charset
// says so, binary otherwise, as open-uri sets it.
func encoded(body string, charset string) object.Value {
	switch strings.ToLower(charset) {
	case "utf-8", "utf8":
		return object.String(body)
	case "us-ascii", "ascii", "ansi_x3.4-1968":
		return object.StringOf(object.KindASCII, body)
	}
	return object.Bytes([]byte(body))
}

// Install defines OpenURI, URI.open and URI::HTTP#open and #read.
// lib.Install calls it after uri's Install.
func Install() {
	m := module
	object.ObjectClass.Consts["OpenURI"] = object.ClassValue(m)
	m.Consts["VERSION"] = object.String("0.5.0")
	for _, c := range []*object.Class{httpError, httpRedirect, tooManyRedirects, documentClass} {
		c.Lexical = m
		m.Consts[c.Name] = object.ClassValue(c)
	}
	for _, name := range []string{"io", "uri"} {
		name := name
		httpError.Define(name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if v, ok := self.Exception().Attr(name); ok {
				return v, nil
			}
			return object.Nil, nil
		})
	}

	open := func(ctx object.Context, target object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, err := readOptions(ctx, args, kw)
		if err != nil {
			return object.Value{}, err
		}
		doc, err := fetch(ctx, target, o)
		if err != nil {
			return object.Value{}, err
		}
		if !block.IsZero() {
			return ctx.Yield(block, doc)
		}
		return doc, nil
	}
	m.DefineSingleton("open_uri", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 {
			return object.Value{}, stdlib.ArgError(ctx, 0, "1+")
		}
		return open(ctx, args[0], args[1:], kw, block)
	})
	uriValue, ok := object.ObjectClass.Consts["URI"]
	if !ok {
		return
	}
	uriModule, _ := uriValue.AsClass()
	uriModule.DefineSingleton("open", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 {
			return object.Value{}, stdlib.ArgError(ctx, 0, "1+")
		}
		return open(ctx, args[0], args[1:], kw, block)
	})
	httpValue, ok := uriModule.Consts["HTTP"]
	if !ok {
		return
	}
	httpClass, _ := httpValue.AsClass()
	httpClass.Define("open", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return open(ctx, self, args, kw, block)
	})
	httpClass.Define("read", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		doc, err := open(ctx, self, args, kw, object.Value{})
		if err != nil {
			return object.Value{}, err
		}
		return docOf(doc).body, nil
	})
	installDocument()
}

// fetch follows the redirects from target to the page, as open_loop does.
func fetch(ctx object.Context, target object.Value, o *options) (object.Value, error) {
	current := target
	if s, ok := target.AsString(); ok {
		// A name without a scheme is a file to Ruby, which cameo reads
		// none of.
		parsed, err := uri.Parse(ctx, s)
		if err != nil {
			return object.Value{}, err
		}
		if c, ok := uri.Of(parsed); !ok || c.Scheme == "" {
			return object.Value{}, ctx.Raise(object.ArgumentError, "%q is not an http or https URL: cameo reads no files", s)
		}
		current = parsed
	}
	seen := map[string]bool{}
	for {
		u, ok := uri.Of(current)
		if !ok {
			return object.Value{}, ctx.Raise(object.ArgumentError, "open-uri takes a URI, not %s", stdlib.KindName(current))
		}
		text, err := ctx.ToS(current)
		if err != nil {
			return object.Value{}, err
		}
		if !schemeOK.MatchString(u.Scheme) {
			return object.Value{}, ctx.Raise(object.ArgumentError, "%s is not an http or https URL: open-uri reaches the network alone", text)
		}
		if u.Userinfo != "" {
			return object.Value{}, ctx.Raise(object.ArgumentError, "userinfo not supported.  [RFC3986]")
		}
		user, pass := o.user, o.pass
		if len(seen) > 0 {
			user, pass = "", "" // credentials do not follow a redirect
		}
		f, err := nethttp.Fetch(ctx, &u, o.header, user, pass, o.readTimeout)
		if err != nil {
			return object.Value{}, err
		}
		doc := &document{code: strconv.Itoa(f.Code), message: f.Message, names: f.Names, values: f.Values, baseURI: current}
		charset := doc.charset()
		if o.encoding != "" {
			charset = o.encoding
		}
		doc.body = encoded(f.Body, charset)
		status := doc.code + " " + doc.message
		switch f.Code {
		case 200, 201, 202, 203, 204, 205, 206:
			return doc.value(), nil
		case 301, 302, 303, 307, 308:
			location, ok := f.Values["location"]
			if !ok {
				return object.Value{}, httpErr(httpError, status+" (Invalid Location URI)", doc.value(), object.Value{})
			}
			next, err := ctx.Call(current, "+", object.String(location))
			if err != nil {
				return object.Value{}, httpErr(httpError, status+" (Invalid Location URI)", doc.value(), object.Value{})
			}
			if !o.redirect {
				return object.Value{}, httpErr(httpRedirect, status, doc.value(), next)
			}
			nu, _ := uri.Of(next)
			nextText, err := ctx.ToS(next)
			if err != nil {
				return object.Value{}, err
			}
			if !redirectable(u.Scheme, nu.Scheme) {
				return object.Value{}, ctx.Raise(object.RuntimeError, "redirection forbidden: %s -> %s", text, nextText)
			}
			if seen[nextText] {
				return object.Value{}, ctx.Raise(object.RuntimeError, "HTTP redirection loop: %s", nextText)
			}
			seen[nextText] = true
			if o.maxRedirects > 0 && len(seen) > o.maxRedirects {
				return object.Value{}, httpErr(tooManyRedirects, "Too many redirects", doc.value(), object.Value{})
			}
			current = next
		default:
			return object.Value{}, httpErr(httpError, status, doc.value(), object.Value{})
		}
	}
}

func redirectable(from, to string) bool {
	from, to = strings.ToLower(from), strings.ToLower(to)
	return from == to || (from == "http" || from == "ftp") && (to == "http" || to == "https" || to == "ftp")
}

// httpErr makes an OpenURI::HTTPError and kin, with the document on io.
func httpErr(cls *object.Class, msg string, io, uriValue object.Value) error {
	attrs := map[string]object.Value{"io": io}
	if !uriValue.IsZero() {
		attrs["uri"] = uriValue
	}
	return &object.Exception{Cls: cls, Message: msg, Attrs: attrs}
}

// installDocument defines OpenURI::Document: the body and the details of
// the response, as OpenURI::Meta reads them.
func installDocument() {
	c := documentClass
	def := func(name string, arity int, f func(ctx object.Context, d *document, args []object.Value) (object.Value, error)) {
		c.Define(name, arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return f(ctx, docOf(self), args)
		})
	}
	body := func(ctx object.Context, d *document, args []object.Value) (object.Value, error) { return d.body, nil }
	def("read", -1, body)
	def("string", 0, body)
	def("to_s", 0, body)
	def("size", 0, func(ctx object.Context, d *document, args []object.Value) (object.Value, error) {
		return object.Int(len(d.body.Str())), nil
	})
	lines := func(ctx object.Context, d *document, args []object.Value) (object.Value, error) {
		return ctx.Call(d.body, "lines")
	}
	def("readlines", -1, lines)
	def("each_line", -1, func(ctx object.Context, d *document, args []object.Value) (object.Value, error) {
		return ctx.Call(d.body, "each_line")
	})
	def("base_uri", 0, func(ctx object.Context, d *document, args []object.Value) (object.Value, error) {
		return d.baseURI, nil
	})
	def("status", 0, func(ctx object.Context, d *document, args []object.Value) (object.Value, error) {
		return object.NewList(object.String(d.code), object.String(d.message)), nil
	})
	def("meta", 0, func(ctx object.Context, d *document, args []object.Value) (object.Value, error) {
		m := object.NewMap()
		for _, n := range d.names {
			m = m.Put(object.String(n), object.String(d.values[n]))
		}
		return object.MapValue(m), nil
	})
	def("metas", 0, func(ctx object.Context, d *document, args []object.Value) (object.Value, error) {
		m := object.NewMap()
		for _, n := range d.names {
			m = m.Put(object.String(n), object.NewList(object.String(d.values[n])))
		}
		return object.MapValue(m), nil
	})
	def("content_type", 0, func(ctx object.Context, d *document, args []object.Value) (object.Value, error) {
		typ, _, ok := d.contentType()
		if !ok {
			return object.String("application/octet-stream"), nil
		}
		return object.String(typ), nil
	})
	def("charset", 0, func(ctx object.Context, d *document, args []object.Value) (object.Value, error) {
		if cs := d.charset(); cs != "" {
			return object.String(cs), nil
		}
		return object.Nil, nil
	})
	def("content_encoding", 0, func(ctx object.Context, d *document, args []object.Value) (object.Value, error) {
		out := []object.Value{}
		if v, ok := d.values["content-encoding"]; ok {
			for _, t := range tokenRe.FindAllString(v, -1) {
				out = append(out, object.String(strings.ToLower(t)))
			}
		}
		return object.NewList(out...), nil
	})
	def("last_modified", 0, func(ctx object.Context, d *document, args []object.Value) (object.Value, error) {
		v, ok := d.values["last-modified"]
		if !ok {
			return object.Nil, nil
		}
		// Time.httpdate's three forms, which net/http reads.
		t, err := gohttp.ParseTime(v)
		if err != nil {
			return object.Value{}, ctx.Raise(object.ArgumentError, "not RFC 2616 compliant date: %q", v)
		}
		return object.TimeValue(t.UTC()), nil
	})
	for _, name := range []string{"close", "rewind"} {
		def(name, 0, func(ctx object.Context, d *document, args []object.Value) (object.Value, error) {
			return object.Nil, nil
		})
	}
	def("closed?", 0, func(ctx object.Context, d *document, args []object.Value) (object.Value, error) {
		return object.False, nil
	})
	def("eof?", 0, func(ctx object.Context, d *document, args []object.Value) (object.Value, error) {
		return object.False, nil
	})
	def("inspect", 0, func(ctx object.Context, d *document, args []object.Value) (object.Value, error) {
		s, err := ctx.ToS(d.baseURI)
		if err != nil {
			return object.Value{}, err
		}
		return object.String("#<OpenURI::Document " + s + " " + d.code + " " + d.message + ">"), nil
	})
}
