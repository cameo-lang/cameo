// Package http is Ruby's net/http library, built into cameo: Net::HTTP
// with its class methods, the request classes, Net::HTTPResponse and its
// subclasses by status, and the errors.
//
// It follows net-http 0.6.0, the version Ruby 3.4 ships, in what a script
// calls and what it gets back. A program reaches the network only through
// the *http.Client its Go host gave it (Program.HTTPClient); without one,
// require "net/http" works and every request raises SocketError. A
// request blocks the goroutine that makes it, so a script fans requests
// out with async. Redirects are not followed, as Ruby does not follow
// them. A Net::HTTP, a request and a response are instances with typed
// fields: http.use_ssl = true and req["Accept"] = "x" rebind the variable
// to the changed copy, as every cameo assignment does, and
// req.set_form_data(h) and req.basic_auth(u, p) return the changed copy
// for the script to bind.
package http

import (
	"strconv"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
	"github.com/cameo-lang/cameo/src/stdlib"
)

// netModule and the classes. Net::HTTP's fields are the connection's
// settings; a request's and a response's are in request.go and
// response.go.
var (
	netModule     = object.NewModule("Net")
	protocolClass = object.NewClass("Protocol", object.ObjectClass)
	httpClass     = object.NewClass("HTTP", protocolClass)
	// Timeout and OpenSSL are shared with resolv and digest: whichever
	// library comes first makes them, the others find them.
	timeoutModule = stdlib.Module("Timeout")
	timeoutError  = timeoutErrorClass()
	socketError   = object.NewClass("SocketError", object.StandardError)
	opensslModule = stdlib.Module("OpenSSL")
	sslModule     = object.NewModule("SSL")
	sslError      = object.NewClass("SSLError", object.StandardError)

	protocolError      = object.NewClass("ProtocolError", object.StandardError)
	protoSyntaxError   = object.NewClass("ProtoSyntaxError", protocolError)
	protoFatalError    = object.NewClass("ProtoFatalError", protocolError)
	protoUnknownError  = object.NewClass("ProtoUnknownError", protocolError)
	protoServerError   = object.NewClass("ProtoServerError", protocolError)
	protoAuthError     = object.NewClass("ProtoAuthError", protocolError)
	protoCommandError  = object.NewClass("ProtoCommandError", protocolError)
	protoRetryError    = object.NewClass("ProtoRetriableError", protocolError)
	httpBadResponse    = object.NewClass("HTTPBadResponse", object.StandardError)
	httpHeaderSyntax   = object.NewClass("HTTPHeaderSyntaxError", object.StandardError)
	httpExceptions     = object.NewModule("HTTPExceptions")
	httpError          = object.NewClass("HTTPError", protocolError)
	httpRetriableError = object.NewClass("HTTPRetriableError", protoRetryError)
	httpClientExc      = object.NewClass("HTTPClientException", protoServerError)
	httpFatalError     = object.NewClass("HTTPFatalError", protoFatalError)
	openTimeout        = object.NewClass("OpenTimeout", timeoutError)
	readTimeout        = object.NewClass("ReadTimeout", timeoutError)
	writeTimeout       = object.NewClass("WriteTimeout", timeoutError)
	econnrefused       = object.NewClass("ECONNREFUSED", object.SystemCallError)
	econnreset         = object.NewClass("ECONNRESET", object.SystemCallError)
	ehostunreach       = object.NewClass("EHOSTUNREACH", object.SystemCallError)
	enetunreach        = object.NewClass("ENETUNREACH", object.SystemCallError)
	etimedout          = object.NewClass("ETIMEDOUT", object.SystemCallError)
)

// timeoutErrorClass is Timeout::Error, found when another library defined
// it and made otherwise.
func timeoutErrorClass() *object.Class {
	if v, ok := timeoutModule.Consts["Error"]; ok {
		if c, isClass := v.AsClass(); isClass {
			return c
		}
	}
	c := object.NewClass("Error", object.RuntimeError)
	c.Lexical = timeoutModule
	timeoutModule.Consts["Error"] = object.ClassValue(c)
	return c
}

// The fields of a Net::HTTP.
const (
	hAddress = iota
	hPort
	hUseSSL
	hOpenTimeout
	hReadTimeout
	hWriteTimeout
	hStarted
	hVerifyMode
	hMaxRetries
	hKeepAlive
	hFieldCount
)

var httpFields = [hFieldCount]struct{ name, typ string }{
	{"address", "string"}, {"port", "int"}, {"use_ssl", "bool"}, {"open_timeout", "float"}, {"read_timeout", "float"},
	{"write_timeout", "float"}, {"started", "bool"}, {"verify_mode", "int"}, {"max_retries", "int"}, {"keep_alive_timeout", "float"},
}

// declare gives a class fields of the names and scalar types given.
func declare(c *object.Class, fields []struct{ name, typ string }) {
	taken := func(number int64) bool {
		for _, f := range c.Fields {
			if f.Number == number {
				return true
			}
		}
		return false
	}
	for _, f := range fields {
		field := &object.Field{Name: f.name, Number: protogen.FieldNumber(f.name, taken), Type: object.ScalarField(f.typ), Owner: c}
		c.FieldIndex[f.name] = len(c.Fields)
		c.Fields = append(c.Fields, field)
	}
}

// inherit copies the superclass's fields into c.
func inherit(c *object.Class) {
	for _, f := range c.Super.Fields {
		c.FieldIndex[f.Name] = len(c.Fields)
		c.Fields = append(c.Fields, f)
	}
}

type fn = object.NativeFn

// Install defines Net::HTTP and its classes. lib.Install calls it once.
func Install() {
	object.ObjectClass.Consts["Net"] = object.ClassValue(netModule)
	object.ObjectClass.Consts["SocketError"] = object.ClassValue(socketError)
	sslModule.Lexical = opensslModule
	opensslModule.Consts["SSL"] = object.ClassValue(sslModule)
	sslError.Lexical = sslModule
	sslModule.Consts["SSLError"] = object.ClassValue(sslError)
	sslModule.Consts["VERIFY_NONE"] = object.Int(0)
	sslModule.Consts["VERIFY_PEER"] = object.Int(1)
	for _, c := range []*object.Class{econnrefused, econnreset, ehostunreach, enetunreach, etimedout} {
		c.Lexical = object.ErrnoModule
		object.ErrnoModule.Consts[c.Name] = object.ClassValue(c)
	}
	for _, c := range []*object.Class{protocolClass, httpClass, protocolError, protoSyntaxError, protoFatalError, protoUnknownError, protoServerError,
		protoAuthError, protoCommandError, protoRetryError, httpBadResponse, httpHeaderSyntax, httpExceptions, httpError, httpRetriableError,
		httpClientExc, httpFatalError, openTimeout, readTimeout, writeTimeout} {
		c.Lexical = netModule
		netModule.Consts[c.Name] = object.ClassValue(c)
	}
	netModule.Consts["ProtocRetryError"] = object.ClassValue(protoRetryError)
	netModule.Consts["HTTPServerException"] = object.ClassValue(httpClientExc)
	netModule.Consts["HTTPSession"] = object.ClassValue(httpClass)
	for _, c := range []*object.Class{httpError, httpRetriableError, httpClientExc, httpFatalError} {
		c.Include(httpExceptions)
	}
	// An HTTPExceptions error carries the response that raised it.
	httpExceptions.Define("response", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if exc, ok := self.AsException(); ok {
			if r, ok := exc.Attr("response"); ok {
				return r, nil
			}
		}
		return object.Nil, nil
	})
	httpExceptions.Define("data", 0, httpExceptions.Methods["response"].Native)

	h := httpClass
	h.Consts["VERSION"] = object.String("0.6.0")
	h.Consts["HAVE_ZLIB"] = object.True
	declare(h, httpFields[:])
	installRequests()
	installResponses()
	installHTTP()
}

// conn is a Net::HTTP as Go reads it.
type conn struct {
	address      string
	port         int64
	useSSL       bool
	openTimeout  float64
	readTimeout  float64
	writeTimeout float64
	started      bool
	verifyMode   *int64
	maxRetries   int64
	keepAlive    float64
}

func readConn(v object.Value) conn {
	f := v.Instance().Fields
	num := func(i int) int64 { n, _ := f[i].AsInt(); return n }
	flt := func(i int) float64 { x, _ := f[i].AsNumber(); return x }
	c := conn{address: f[hAddress].Str(), port: num(hPort), openTimeout: flt(hOpenTimeout), readTimeout: flt(hReadTimeout), writeTimeout: flt(hWriteTimeout), maxRetries: num(hMaxRetries), keepAlive: flt(hKeepAlive)}
	c.useSSL, _ = f[hUseSSL].AsBool()
	c.started, _ = f[hStarted].AsBool()
	if n, ok := f[hVerifyMode].AsInt(); ok {
		c.verifyMode = &n
	}
	return c
}

func buildConn(cls *object.Class, c conn) object.Value {
	fields := make([]object.Value, len(cls.Fields))
	fields[hAddress] = object.String(c.address)
	fields[hPort] = object.Int(c.port)
	fields[hUseSSL] = object.Bool(c.useSSL)
	fields[hOpenTimeout] = object.Float(c.openTimeout)
	fields[hReadTimeout] = object.Float(c.readTimeout)
	fields[hWriteTimeout] = object.Float(c.writeTimeout)
	fields[hStarted] = object.Bool(c.started)
	fields[hVerifyMode] = object.Nil
	if c.verifyMode != nil {
		fields[hVerifyMode] = object.Int(*c.verifyMode)
	}
	fields[hMaxRetries] = object.Int(c.maxRetries)
	fields[hKeepAlive] = object.Float(c.keepAlive)
	return object.InstanceValue(&object.Instance{Cls: cls, Fields: fields})
}

// newConn is Net::HTTP.new(address, port = nil): port nil is 80, and the
// timeouts are Ruby's defaults.
func newConn(address string, port *int64, useSSL bool) conn {
	c := conn{address: address, port: 80, useSSL: useSSL, openTimeout: 60, readTimeout: 60, writeTimeout: 60, maxRetries: 1, keepAlive: 2}
	if useSSL {
		c.port = 443
	}
	if port != nil {
		c.port = *port
	}
	return c
}

// seconds is a Ruby Rational or Integer as cameo gives it: an Integer
// when whole, a Float otherwise.
func seconds(f float64) object.Value {
	if f == float64(int64(f)) {
		return object.Int(int64(f))
	}
	return object.Float(f)
}

// addressArgs reads Net::HTTP.get's and get_response's arguments: a URI,
// or a host, a path and a port.
func addressArgs(ctx object.Context, a []object.Value) (c conn, path string, err error) {
	if len(a) == 0 || len(a) > 3 {
		return c, "", stdlib.ArgError(ctx, len(a), "1..3")
	}
	if len(a) == 1 {
		u, ok := uriOf(a[0])
		if !ok {
			return c, "", ctx.Raise(object.ArgumentError, "bad argument (expected URI object or URI string)")
		}
		return connOf(u), u.RequestURI, nil
	}
	host, err := stdlib.WantString(ctx, a[0])
	if err != nil {
		return c, "", err
	}
	if path, err = stdlib.WantString(ctx, a[1]); err != nil {
		return c, "", err
	}
	var port *int64
	if len(a) == 3 && !a[2].IsNil() {
		n, err := stdlib.WantInt(ctx, a[2], "")
		if err != nil {
			return c, "", err
		}
		port = &n
	}
	return newConn(host, port, false), path, nil
}

// installHTTP defines Net::HTTP's class methods and the connection object.
func installHTTP() {
	h := httpClass
	h.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(a) < 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(a), "1+")
		}
		address, err := stdlib.WantString(ctx, a[0])
		if err != nil {
			return object.Value{}, err
		}
		var port *int64
		if len(a) >= 2 && !a[1].IsNil() {
			n, err := stdlib.WantInt(ctx, a[1], "")
			if err != nil {
				return object.Value{}, err
			}
			port = &n
		}
		// The proxy arguments after the port are accepted and ignored:
		// cameo reaches the network through the host's client alone.
		return buildConn(self.Cls(), newConn(address, port, false)), nil
	})
	h.DefineSingleton("start", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(a) < 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(a), "1+")
		}
		address, err := stdlib.WantString(ctx, a[0])
		if err != nil {
			return object.Value{}, err
		}
		var port *int64
		if len(a) >= 2 && !a[1].IsNil() {
			if m, isMap := a[1].AsMap(); isMap {
				kw = m
			} else {
				n, err := stdlib.WantInt(ctx, a[1], "")
				if err != nil {
					return object.Value{}, err
				}
				port = &n
			}
		}
		// An options Hash after the proxy arguments, as Ruby takes it.
		for _, x := range a[2:] {
			if m, ok := x.AsMap(); ok {
				kw = m
			}
		}
		c := newConn(address, port, false)
		if c, err = applyOptions(ctx, c, kw); err != nil {
			return object.Value{}, err
		}
		if port == nil && c.useSSL {
			c.port = 443
		}
		return startConn(ctx, self.Cls(), c, block)
	})
	h.DefineSingleton("get", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		res, err := classGet(ctx, a)
		if err != nil {
			return object.Value{}, err
		}
		return readResponse(res).body, nil
	})
	h.DefineSingleton("get_response", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		res, err := classGet(ctx, a)
		if err != nil {
			return object.Value{}, err
		}
		// The block sees the response; the response is returned, as
		// Ruby's request does.
		if !block.IsZero() {
			if _, err := ctx.Yield(block, res); err != nil {
				return object.Value{}, err
			}
		}
		return res, nil
	})
	h.DefineSingleton("get_print", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		res, err := classGet(ctx, a)
		if err != nil {
			return object.Value{}, err
		}
		if body := readResponse(res).body; !body.IsNil() {
			if err := ctx.Write(false, body.Str()); err != nil {
				return object.Value{}, err
			}
		}
		return object.Nil, nil
	})
	simple := func(name, method string, form bool) {
		h.DefineSingleton(name, -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			a = withKeywords(a, kw)
			if len(a) < 2 || len(a) > 3 {
				return object.Value{}, stdlib.ArgError(ctx, len(a), "2..3")
			}
			u, ok := uriOf(a[0])
			if !ok {
				return object.Value{}, ctx.Raise(object.ArgumentError, "bad argument (expected URI object or URI string)")
			}
			req := newRequest(requestClasses[method], method, u.RequestURI, u)
			if form {
				body, err := formData(ctx, a[1], "&")
				if err != nil {
					return object.Value{}, err
				}
				req.body = &body
				req.headers.Set("content-type", "application/x-www-form-urlencoded")
			} else {
				body, err := stdlib.WantString(ctx, a[1])
				if err != nil {
					return object.Value{}, err
				}
				req.body = &body
				if len(a) == 3 && !a[2].IsNil() {
					if err := req.headers.Add(ctx, a[2]); err != nil {
						return object.Value{}, err
					}
				}
			}
			return perform(ctx, connOf(u), req)
		})
	}
	simple("post", "POST", false)
	simple("post_form", "POST", true)
	simple("put", "PUT", false)
	for name, v := range map[string]object.Value{"version_1_2": object.True, "version_1_2?": object.True, "version_1_1?": object.False, "proxy_class?": object.False, "default_port": object.Int(80), "http_default_port": object.Int(80), "https_default_port": object.Int(443)} {
		v := v
		h.DefineSingleton(name, 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return v, nil
		})
	}
	h.DefineSingleton("newobj", -1, h.SingletonMethods["new"].Native)

	// The connection's readers and setters.
	reader := func(name string, f func(c conn) object.Value) {
		h.Define(name, 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return f(readConn(self)), nil
		})
	}
	reader("address", func(c conn) object.Value { return object.String(c.address) })
	reader("port", func(c conn) object.Value { return object.Int(c.port) })
	reader("use_ssl?", func(c conn) object.Value { return object.Bool(c.useSSL) })
	reader("open_timeout", func(c conn) object.Value { return seconds(c.openTimeout) })
	reader("read_timeout", func(c conn) object.Value { return seconds(c.readTimeout) })
	reader("write_timeout", func(c conn) object.Value { return seconds(c.writeTimeout) })
	reader("keep_alive_timeout", func(c conn) object.Value { return seconds(c.keepAlive) })
	reader("max_retries", func(c conn) object.Value { return object.Int(c.maxRetries) })
	reader("started?", func(c conn) object.Value { return object.Bool(c.started) })
	reader("active?", func(c conn) object.Value { return object.Bool(c.started) })
	reader("proxy?", func(c conn) object.Value { return object.False })
	reader("proxy_address", func(c conn) object.Value { return object.Nil })
	reader("proxy_port", func(c conn) object.Value { return object.Nil })
	reader("continue_timeout", func(c conn) object.Value { return object.Nil })
	reader("verify_mode", func(c conn) object.Value {
		if c.verifyMode == nil {
			return object.Nil
		}
		return object.Int(*c.verifyMode)
	})
	reader("inspect", func(c conn) object.Value {
		open := "false"
		if c.started {
			open = "true"
		}
		return object.String("#<Net::HTTP " + c.address + ":" + strconv.FormatInt(c.port, 10) + " open=" + open + ">")
	})
	setter := func(name string, set func(ctx object.Context, c *conn, v object.Value) error) {
		h.Define(name+"=", 1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			c := readConn(self)
			if err := set(ctx, &c, a[0]); err != nil {
				return object.Value{}, err
			}
			return buildConn(self.Instance().Cls, c), nil
		})
	}
	// use_ssl= leaves the port as it is, as Ruby's does.
	setter("use_ssl", func(ctx object.Context, c *conn, v object.Value) error {
		c.useSSL = v.Truthy()
		return nil
	})
	timeoutSetter := func(name string, field func(c *conn) *float64) {
		setter(name, func(ctx object.Context, c *conn, v object.Value) error {
			if v.IsNil() {
				*field(c) = 0
				return nil
			}
			f, ok := v.AsNumber()
			if !ok {
				return ctx.Raise(object.TypeError, "no implicit conversion of %s into Integer", stdlib.KindName(v))
			}
			*field(c) = f
			return nil
		})
	}
	timeoutSetter("open_timeout", func(c *conn) *float64 { return &c.openTimeout })
	timeoutSetter("read_timeout", func(c *conn) *float64 { return &c.readTimeout })
	timeoutSetter("write_timeout", func(c *conn) *float64 { return &c.writeTimeout })
	timeoutSetter("keep_alive_timeout", func(c *conn) *float64 { return &c.keepAlive })
	setter("max_retries", func(ctx object.Context, c *conn, v object.Value) error {
		n, err := stdlib.WantInt(ctx, v, "")
		c.maxRetries = n
		return err
	})
	// verify_mode is kept for the script to read back; how certificates
	// are verified is the host's client's business.
	setter("verify_mode", func(ctx object.Context, c *conn, v object.Value) error {
		if v.IsNil() {
			c.verifyMode = nil
			return nil
		}
		n, err := stdlib.WantInt(ctx, v, "")
		c.verifyMode = &n
		return err
	})
	for _, name := range []string{"ca_file", "ca_path", "cert", "cert_store", "key", "ssl_version", "min_version", "max_version", "ciphers", "ssl_timeout", "continue_timeout", "local_host", "local_port", "ignore_eof", "response_body_encoding", "proxy_from_env"} {
		setter(name, func(ctx object.Context, c *conn, v object.Value) error { return nil })
	}
	h.Define("set_debug_output", 1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Nil, nil
	})

	h.Define("start", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		c := readConn(self)
		if c.started {
			return object.Value{}, ctx.Raise(object.IOError, "HTTP session already opened")
		}
		return startConn(ctx, self.Instance().Cls, c, block)
	})
	h.Define("finish", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		c := readConn(self)
		if !c.started {
			return object.Value{}, ctx.Raise(object.IOError, "HTTP session not yet started")
		}
		c.started = false
		return buildConn(self.Instance().Cls, c), nil
	})

	// request sends a request the script built; the verbs build one.
	h.Define("request", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(a) < 1 || len(a) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(a), "1..2")
		}
		req, ok := requestOf(a[0])
		if !ok {
			return object.Value{}, ctx.Raise(object.ArgumentError, "expected a Net::HTTPRequest, not %s", stdlib.KindName(a[0]))
		}
		if len(a) == 2 && !a[1].IsNil() {
			body, err := stdlib.WantString(ctx, a[1])
			if err != nil {
				return object.Value{}, err
			}
			req.body = &body
		}
		res, err := perform(ctx, readConn(self), req)
		if err != nil {
			return object.Value{}, err
		}
		if !block.IsZero() {
			if _, err := ctx.Yield(block, res); err != nil {
				return object.Value{}, err
			}
		}
		return res, nil
	})
	verb := func(name, method string, hasData bool) {
		h.Define(name, -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			a = withKeywords(a, kw)
			min, max := 1, 2
			if hasData {
				min, max = 2, 3
			}
			if len(a) < min || len(a) > max {
				return object.Value{}, stdlib.ArgError(ctx, len(a), strconv.FormatInt(int64(min), 10)+".."+strconv.FormatInt(int64(max), 10))
			}
			path, err := stdlib.WantString(ctx, a[0])
			if err != nil {
				return object.Value{}, err
			}
			c := readConn(self)
			req := newRequest(requestClasses[method], method, path, nil)
			req.headers.Set("host", hostHeader(c))
			if method == "DELETE" && !hasData {
				// Ruby's delete sends a WebDAV Depth header by default.
				req.headers.Set("depth", "Infinity")
			}
			i := 1
			if hasData {
				body, err := stdlib.WantString(ctx, a[1])
				if err != nil {
					return object.Value{}, err
				}
				req.body = &body
				i = 2
			}
			if len(a) > i && !a[i].IsNil() {
				if err := req.headers.Add(ctx, a[i]); err != nil {
					return object.Value{}, err
				}
			}
			if hasData && !req.headers.Has("content-type") {
				req.headers.Set("content-type", "application/x-www-form-urlencoded")
			}
			res, err := perform(ctx, c, req)
			if err != nil {
				return object.Value{}, err
			}
			// get { |segment| } yields the body, in one piece.
			if !block.IsZero() {
				if name == "get" || name == "request_get" || name == "get2" {
					if body := readResponse(res).body; !body.IsNil() {
						if _, err := ctx.Yield(block, body); err != nil {
							return object.Value{}, err
						}
					}
					return res, nil
				}
				if _, err := ctx.Yield(block, res); err != nil {
					return object.Value{}, err
				}
			}
			return res, nil
		})
	}
	verb("get", "GET", false)
	verb("request_get", "GET", false)
	verb("get2", "GET", false)
	verb("head", "HEAD", false)
	verb("request_head", "HEAD", false)
	verb("head2", "HEAD", false)
	verb("post", "POST", true)
	verb("request_post", "POST", true)
	verb("post2", "POST", true)
	verb("put", "PUT", true)
	verb("request_put", "PUT", true)
	verb("put2", "PUT", true)
	verb("patch", "PATCH", true)
	verb("delete", "DELETE", false)
	verb("options", "OPTIONS", false)
	verb("trace", "TRACE", false)
	verb("move", "MOVE", false)
	verb("copy", "COPY", false)
	verb("mkcol", "MKCOL", true)
	verb("lock", "LOCK", true)
	verb("unlock", "UNLOCK", true)
	verb("propfind", "PROPFIND", true)
	verb("proppatch", "PROPPATCH", true)
	h.Define("send_request", -1, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		a = withKeywords(a, kw)
		if len(a) < 2 || len(a) > 4 {
			return object.Value{}, stdlib.ArgError(ctx, len(a), "2..4")
		}
		method, err := stdlib.WantString(ctx, a[0])
		if err != nil {
			return object.Value{}, err
		}
		path, err := stdlib.WantString(ctx, a[1])
		if err != nil {
			return object.Value{}, err
		}
		c := readConn(self)
		method = strings.ToUpper(method)
		cls := requestClasses[method]
		if cls == nil {
			cls = genericRequestClass
		}
		req := newRequest(cls, method, path, nil)
		req.headers.Set("host", hostHeader(c))
		if len(a) >= 3 && !a[2].IsNil() {
			body, err := stdlib.WantString(ctx, a[2])
			if err != nil {
				return object.Value{}, err
			}
			req.body = &body
		}
		if len(a) == 4 && !a[3].IsNil() {
			if err := req.headers.Add(ctx, a[3]); err != nil {
				return object.Value{}, err
			}
		}
		return perform(ctx, c, req)
	})
}

// withKeywords is the arguments with the keywords as a trailing Hash: a
// header Hash a script writes as keywords, "Content-Type" => "text/plain".
func withKeywords(a []object.Value, kw *object.Map) []object.Value {
	if kw == nil || kw.Len() == 0 {
		return a
	}
	return append(append([]object.Value(nil), a...), object.MapValue(kw))
}

// applyOptions reads Net::HTTP.start's keyword options into a connection.
func applyOptions(ctx object.Context, c conn, kw *object.Map) (conn, error) {
	if kw == nil {
		return c, nil
	}
	for i, k := range kw.Keys() {
		name, _ := k.AsFieldName()
		v := kw.Values()[i]
		switch name {
		case "use_ssl":
			c.useSSL = v.Truthy()
		case "open_timeout", "read_timeout", "write_timeout", "keep_alive_timeout":
			f, ok := v.AsNumber()
			if !ok && !v.IsNil() {
				return c, ctx.Raise(object.TypeError, "no implicit conversion of %s into Integer", stdlib.KindName(v))
			}
			switch name {
			case "open_timeout":
				c.openTimeout = f
			case "read_timeout":
				c.readTimeout = f
			case "write_timeout":
				c.writeTimeout = f
			default:
				c.keepAlive = f
			}
		case "verify_mode":
			if !v.IsNil() {
				n, err := stdlib.WantInt(ctx, v, "")
				if err != nil {
					return c, err
				}
				c.verifyMode = &n
			}
		case "max_retries":
			n, err := stdlib.WantInt(ctx, v, "")
			if err != nil {
				return c, err
			}
			c.maxRetries = n
		}
	}
	return c, nil
}

// startConn is Net::HTTP.start and Net::HTTP#start: with a block, the
// started connection goes to the block and its value comes back; without
// one the started connection is returned.
func startConn(ctx object.Context, cls *object.Class, c conn, block object.Value) (object.Value, error) {
	c.started = true
	v := buildConn(cls, c)
	if block.IsZero() {
		return v, nil
	}
	return ctx.Yield(block, v)
}

// classGet is Net::HTTP.get and get_response's request.
func classGet(ctx object.Context, a []object.Value) (object.Value, error) {
	c, path, err := addressArgs(ctx, a)
	if err != nil {
		return object.Value{}, err
	}
	req := newRequest(requestClasses["GET"], "GET", path, nil)
	if len(a) == 1 {
		u, _ := uriOf(a[0])
		req.uri = u
	}
	req.headers.Set("host", hostHeader(c))
	return perform(ctx, c, req)
}

// hostHeader is the Host header of a connection: the address, with the
// port when it is not the scheme's default.
func hostHeader(c conn) string {
	if c.useSSL && c.port == 443 || !c.useSSL && c.port == 80 {
		return c.address
	}
	return c.address + ":" + strconv.FormatInt(c.port, 10)
}
