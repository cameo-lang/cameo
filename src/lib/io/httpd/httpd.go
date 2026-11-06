// Package httpd is cameo's HTTP server, require "io/httpd": the one
// library that listens on a socket. It speaks Rack 3: HTTPD.serve(app)
// calls app.call(env) for each request, env the request as a Rack env
// Hash, and writes the [status, headers, body] it returns; a block is the
// app too, HTTPD.serve { |env| ... }. It calls the app on the goroutine
// that called serve, one request at a time, and serves until the host
// stops the run, and only in a program that allows network I/O,
// cameo.Program.AllowIO("network") or AllowIO("io/httpd"): the cameo
// command and a program cameo build wrote do with --io or --io=network.
//
// The rack library reads env and builds what the app returns
// (Rack::Request, Rack::Builder); frankie is an app.
package httpd

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/cameo-lang/cameo/src/lib/stringio"
	"github.com/cameo-lang/cameo/src/object"
)

var (
	httpdModule = object.NewModule("HTTPD")
	// errorClass is HTTPD::Error: the socket could not be opened, or the
	// host allows no I/O.
	errorClass = object.NewClass("Error", object.StandardError)
)

// request is one request as the server read it, with what env needs.
type request struct {
	r    *http.Request
	body []byte
	addr string // the address the server listens on, host:port
}

// response is what the server writes.
type response struct {
	status int
	header http.Header
	body   []byte
}

// handler answers one request; an error is a 500 with the error's text,
// which also goes to standard error.
type handler func(*request) (response, error)

// Install defines HTTPD and HTTPD::Error. lib.Install calls it once.
func Install() {
	m := httpdModule
	object.DefineNative("HTTPD", object.ClassValue(m))
	errorClass.Lexical = m
	m.Consts[errorClass.Name] = object.ClassValue(errorClass)
	m.DefineSingleton("serve", -1, serve)
	// allowed? reports whether this program lets a script serve.
	m.DefineSingleton("allowed?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(ctx.IOAllowed("io/httpd")), nil
	})
}

// serve is HTTPD.serve(app = nil, port: 4567, bind: "127.0.0.1", quiet:
// false, listening: nil) { |env| … }: app, or the block, answers each
// request; listening: is a callable given the URL once the socket is open.
func serve(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if len(args) > 1 {
		return object.Value{}, ctx.Raise(object.ArgumentError, "wrong number of arguments (given %d, expected 0..1)", len(args))
	}
	var call func(env object.Value) (object.Value, error)
	switch {
	case len(args) == 1 && !block.IsZero():
		return object.Value{}, ctx.Raise(object.ArgumentError, "HTTPD.serve takes an app or a block, not both")
	case len(args) == 1:
		app := args[0]
		if !ctx.RespondsTo(app, "call") {
			s, _ := ctx.Inspect(app)
			return object.Value{}, ctx.Raise(object.ArgumentError, "a Rack app responds to call; %s does not", s)
		}
		call = func(env object.Value) (object.Value, error) { return ctx.Call(app, "call", env) }
	case !block.IsZero():
		call = func(env object.Value) (object.Value, error) { return ctx.Yield(block, env) }
	default:
		return object.Value{}, ctx.Raise(object.ArgumentError, "HTTPD.serve takes a Rack app, or a block that answers each request's env")
	}
	port, bind, quiet := int64(4567), "127.0.0.1", false
	listening := object.Nil
	if kw != nil {
		for i, k := range kw.Keys() {
			v := kw.Values()[i]
			switch k.Str() {
			case "port":
				if v.Kind() != object.KindInt {
					return object.Value{}, ctx.Raise(object.TypeError, "port must be an Integer, not %s", v.Class().QualifiedName())
				}
				port = v.Int()
			case "bind":
				s, ok := v.AsString()
				if !ok {
					return object.Value{}, ctx.Raise(object.TypeError, "bind must be a String, not %s", v.Class().QualifiedName())
				}
				bind = s
			case "quiet":
				quiet = v.Truthy()
			case "listening":
				listening = v
			default:
				return object.Value{}, ctx.Raise(object.ArgumentError, "unknown keyword: :%s", k.Str())
			}
		}
	}
	var onListen func(url string) error
	if !listening.IsNil() {
		onListen = func(url string) error {
			_, err := ctx.Call(listening, "call", object.String(url))
			return err
		}
	}
	err := listen(ctx, net.JoinHostPort(bind, fmt.Sprint(port)), quiet, onListen, func(req *request) (response, error) {
		v, err := call(env(req))
		if err != nil {
			return response{}, err
		}
		return rackResponse(ctx, v)
	})
	if err != nil {
		return object.Value{}, err
	}
	return object.Nil, nil
}

// env is the Rack env of req: the CGI names, the headers as HTTP_*, and
// rack.input, a StringIO of the body. PATH_INFO is the path as it came,
// percent-encoded, as Rack servers give it.
func env(req *request) object.Value {
	r := req.r
	b := object.NewMapBuilder()
	set := func(k string, v object.Value) { b.Set(object.String(k), v) }
	str := func(k, v string) { set(k, object.String(v)) }
	str("REQUEST_METHOD", r.Method)
	str("SCRIPT_NAME", "")
	str("PATH_INFO", r.URL.EscapedPath())
	str("QUERY_STRING", r.URL.RawQuery)
	str("REQUEST_URI", r.RequestURI)
	str("SERVER_PROTOCOL", r.Proto)
	host, port := r.Host, ""
	if h, p, err := net.SplitHostPort(r.Host); err == nil {
		host, port = h, p
	}
	if port == "" {
		_, port, _ = net.SplitHostPort(req.addr)
	}
	if host == "" {
		host, _, _ = net.SplitHostPort(req.addr)
	}
	str("SERVER_NAME", host)
	str("SERVER_PORT", port)
	remote, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remote = r.RemoteAddr
	}
	str("REMOTE_ADDR", remote)
	if r.Host != "" {
		str("HTTP_HOST", r.Host)
	}
	// A repeated header's values are joined with commas, as Rack servers
	// join them, cookies with semicolons.
	for name, values := range r.Header {
		key := "HTTP_" + strings.ReplaceAll(strings.ToUpper(name), "-", "_")
		sep := ", "
		switch name {
		case "Content-Type":
			key = "CONTENT_TYPE"
		case "Content-Length":
			key = "CONTENT_LENGTH"
		case "Cookie":
			sep = "; "
		}
		str(key, strings.Join(values, sep))
	}
	str("rack.url_scheme", "http")
	set("rack.input", stringio.NewInput(req.body))
	if stderr, ok := object.ObjectClass.Consts["STDERR"]; ok {
		set("rack.errors", stderr)
	}
	set("rack.multithread", object.False)
	set("rack.multiprocess", object.False)
	set("rack.run_once", object.False)
	set("rack.hijack?", object.False)
	return object.MapValue(b.Map())
}

// rackResponse reads what an app returned: [status, headers, body], the
// status an Integer, headers a Hash of names to a String, whose lines are
// values each, or an Array of them, body an Array of Strings or anything
// whose each yields them, closed after when it responds to close.
func rackResponse(ctx object.Context, v object.Value) (response, error) {
	res := response{header: http.Header{}}
	if v.Kind() != object.KindList || len(v.List().Elems) != 3 {
		s, _ := ctx.Inspect(v)
		return res, ctx.Raise(object.TypeError, "a Rack app returns [status, headers, body], not %s", s)
	}
	elems := v.List().Elems
	var err error
	if res.status, err = status(ctx, elems[0]); err != nil {
		return res, err
	}
	if res.header, err = headers(ctx, elems[1]); err != nil {
		return res, err
	}
	res.body, err = body(ctx, elems[2])
	return res, err
}

// status reads a status: 200 to 999. net/http panics on one outside
// 100..999, and treats one below 200 as interim, never finishing the
// response, so such a one is the app's error.
func status(ctx object.Context, v object.Value) (int, error) {
	if v.Kind() == object.KindInt && v.Int() >= 200 && v.Int() <= 999 {
		return int(v.Int()), nil
	}
	s, _ := ctx.Inspect(v)
	return 0, ctx.Raise(object.TypeError, "a Rack app returned the status %s, not an Integer from 200 to 999", s)
}

// headers reads a Hash of header names to values: a String, each line of
// which is a value, or an Array of Strings.
func headers(ctx object.Context, v object.Value) (http.Header, error) {
	if v.Kind() != object.KindMap {
		return nil, ctx.Raise(object.TypeError, "a Rack app returned headers of %s, not a Hash", v.Class().QualifiedName())
	}
	m := v.Map()
	out := http.Header{}
	for i, k := range m.Keys() {
		name, err := ctx.ToS(k)
		if err != nil {
			return nil, err
		}
		name = strings.TrimSpace(name)
		val := m.Values()[i]
		var values []object.Value
		if val.Kind() == object.KindList {
			values = val.List().Elems
		} else {
			values = []object.Value{val}
		}
		for _, one := range values {
			s, err := ctx.ToS(one)
			if err != nil {
				return nil, err
			}
			for _, line := range strings.Split(s, "\n") {
				out.Add(name, line)
			}
		}
	}
	return out, nil
}

// body reads a Rack body: an Array of Strings, or what its each yields;
// a String is none, in Rack 3.
func body(ctx object.Context, v object.Value) ([]byte, error) {
	chunks := v
	if v.Kind() != object.KindList {
		if v.IsString() || !ctx.RespondsTo(v, "each") {
			return nil, ctx.Raise(object.TypeError, "a Rack body responds to each; %s does not, so wrap a String in an Array", v.Class().QualifiedName())
		}
		e, err := ctx.Call(v, "to_enum", object.Symbol("each"))
		if err != nil {
			return nil, err
		}
		if chunks, err = ctx.Call(e, "to_a"); err != nil {
			return nil, err
		}
	}
	var out []byte
	for _, c := range chunks.List().Elems {
		s, err := ctx.ToS(c)
		if err != nil {
			return nil, err
		}
		out = append(out, s...)
	}
	if v.Kind() != object.KindList && ctx.RespondsTo(v, "close") {
		if _, err := ctx.Call(v, "close"); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// job is one request waiting for the serving goroutine, with the channel
// its response goes back on.
type job struct {
	req   *request
	reply chan response
}

// listen listens on addr and answers each request with handle, on the
// goroutine that called it, one request at a time, until the run is
// stopped (Context.Done), when it returns the run's stop error, or the
// handler ends it with an error that is no StandardError. listening,
// when given, is called with the URL once the socket is open, before any
// request. Each request is logged on standard error in Rack's common log
// form with the seconds it took, unless quiet. It raises HTTPD::Error
// when the host allows no I/O or the socket cannot be opened.
func listen(ctx object.Context, addr string, quiet bool, listening func(url string) error, handle handler) error {
	if !ctx.IOAllowed("io/httpd") {
		return ctx.Raise(errorClass, "the host allows no network I/O: run the script with cameo --io=network, or let the program say AllowIO(\"network\") or AllowIO(\"io/httpd\")")
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return ctx.Raise(errorClass, "%v", err)
	}
	if listening != nil {
		// The socket is open: the URL it serves at, an IPv6 host in brackets.
		if err := listening("http://" + ln.Addr().String()); err != nil {
			ln.Close()
			return err
		}
	}
	jobs := make(chan job)
	done := ctx.Done()
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		j := job{req: &request{r: r, body: body, addr: ln.Addr().String()}, reply: make(chan response, 1)}
		start := time.Now()
		select {
		case jobs <- j:
		case <-done:
			http.Error(w, "the server is stopping", http.StatusServiceUnavailable)
			return
		}
		res := <-j.reply
		for k, vs := range res.header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(res.status)
		w.Write(res.body)
		if !quiet {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			ctx.Write(true, fmt.Sprintf("%s - - [%s] %q %d %d %.4f\n", host, time.Now().Format("02/Jan/2006:15:04:05 -0700"), r.Method+" "+r.RequestURI+" "+r.Proto, res.status, len(res.body), time.Since(start).Seconds()))
		}
	})}
	go server.Serve(ln)
	// The listener closes at once; a reply on its way, the 503 of the
	// request that ended serve, gets a moment to be written.
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()
	for {
		select {
		case j := <-jobs:
			res, err := handle(j.req)
			if err != nil {
				// A StandardError is the request's 500; anything else, a
				// break, an exit or the run's stop, ends serve as it ends
				// any method yielding to the block.
				plain := http.Header{"Content-Type": {"text/plain;charset=utf-8"}}
				if exc, ok := err.(*object.Exception); !ok || !exc.Cls.IsA(object.StandardError) {
					j.reply <- response{status: http.StatusServiceUnavailable, header: plain, body: []byte("the server is stopping\n")}
					return err
				}
				text := ctx.FormatError(err)
				ctx.Write(true, text+"\n")
				res = response{status: http.StatusInternalServerError, header: plain, body: []byte(text + "\n")}
			}
			j.reply <- res
		case <-done:
			return ctx.Stopped()
		}
	}
}
