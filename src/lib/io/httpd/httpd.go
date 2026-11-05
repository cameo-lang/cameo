// Package httpd is cameo's HTTP server, require "io/httpd": the one
// library that listens on a socket. HTTPD.serve takes a block and calls it
// with an HTTPD::Request for each request, on the goroutine that called
// serve, one request at a time; what the block returns is the response,
// in Sinatra's forms. It serves until the host stops the run, and it
// serves only in a program that allows network I/O,
// cameo.Program.AllowIO("network") or AllowIO("io/httpd"): the cameo
// command and a program cameo build wrote do with --io or --io=network.
//
// A library that wants to serve, frankie, calls Serve with a Go handler
// and keeps no I/O of its own.
package httpd

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/cameo-lang/cameo/src/object"
)

var (
	httpdModule  = object.NewModule("HTTPD")
	requestClass = object.NewClass("Request", object.ObjectClass)
	// errorClass is HTTPD::Error: the socket could not be opened, or the
	// host allows no I/O.
	errorClass = object.NewClass("Error", object.StandardError)
)

// Request is one request as the server read it.
type Request struct {
	Method  string
	Path    string
	Query   string // the query string, without the "?"
	Headers map[string]string
	Body    string
	remote  string
}

// Response is what the server writes.
type Response struct {
	Status  int
	Headers map[string]string
	Body    string
}

// Handler answers one request; an error is a 500 with the error's text,
// which also goes to standard error.
type Handler func(*Request) (Response, error)

// Install defines HTTPD, HTTPD::Request and HTTPD::Error. lib.Install
// calls it once.
func Install() {
	m := httpdModule
	object.DefineNative("HTTPD", object.ClassValue(m))
	for _, c := range []*object.Class{requestClass, errorClass} {
		c.Lexical = m
		m.Consts[c.Name] = object.ClassValue(c)
	}
	m.DefineSingleton("serve", -1, serve)
	// allowed? reports whether this program lets a script serve.
	m.DefineSingleton("allowed?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(ctx.IOAllowed("io/httpd")), nil
	})
	reader := func(name string, read func(*Request) object.Value) {
		requestClass.Define(name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return read(self.Instance().Message.(*Request)), nil
		})
	}
	reader("method", func(r *Request) object.Value { return object.String(r.Method) })
	reader("path", func(r *Request) object.Value { return object.String(r.Path) })
	reader("query", func(r *Request) object.Value { return object.String(r.Query) })
	reader("body", func(r *Request) object.Value { return object.String(r.Body) })
	reader("headers", func(r *Request) object.Value { return StringMap(r.Headers) })
}

// RequestValue is the HTTPD::Request of r.
func RequestValue(r *Request) object.Value {
	return object.InstanceValue(&object.Instance{Cls: requestClass, Message: r})
}

// StringMap is a Hash of Strings.
func StringMap(m map[string]string) object.Value {
	b := object.NewMapBuilder()
	for k, v := range m {
		b.Set(object.String(k), object.String(v))
	}
	return object.MapValue(b.Map())
}

// serve is HTTPD.serve(port: 4567, bind: "127.0.0.1", quiet: false) { |request| … }.
func serve(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if len(args) != 0 {
		return object.Value{}, ctx.Raise(object.ArgumentError, "wrong number of arguments (given %d, expected 0)", len(args))
	}
	if block.IsZero() {
		return object.Value{}, ctx.Raise(object.ArgumentError, "HTTPD.serve takes a block that answers each request")
	}
	port, bind, quiet := int64(4567), "127.0.0.1", false
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
			default:
				return object.Value{}, ctx.Raise(object.ArgumentError, "unknown keyword: :%s", k.Str())
			}
		}
	}
	err := Serve(ctx, net.JoinHostPort(bind, fmt.Sprint(port)), quiet, func(r *Request) (Response, error) {
		v, err := ctx.Yield(block, RequestValue(r))
		if err != nil {
			return Response{}, err
		}
		return Respond(ctx, v)
	})
	if err != nil {
		return object.Value{}, err
	}
	return object.Nil, nil
}

// job is one request waiting for the serving goroutine, with the channel
// its response goes back on.
type job struct {
	req   *Request
	reply chan Response
}

// Serve listens on addr and answers each request with handle, on the
// goroutine that called it, one request at a time, until the run is
// stopped (Context.Done), when it returns the run's stop error, or the
// handler ends it with an error that is no StandardError. A Response
// with no Status is a 200. Each request is logged on standard error in
// Rack's common log form with the seconds it took, unless quiet. It
// raises HTTPD::Error when the host allows no I/O or the socket cannot be
// opened.
func Serve(ctx object.Context, addr string, quiet bool, handle Handler) error {
	if !ctx.IOAllowed("io/httpd") {
		return ctx.Raise(errorClass, "the host allows no network I/O: run the script with cameo --io=network, or let the program say AllowIO(\"network\") or AllowIO(\"io/httpd\")")
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return ctx.Raise(errorClass, "%v", err)
	}
	jobs := make(chan job)
	done := ctx.Done()
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// A repeated header's values are joined, as Rack joins them, and
		// Host is a header too, though Go keeps it apart.
		headers := make(map[string]string, len(r.Header)+1)
		for k, vs := range r.Header {
			headers[k] = strings.Join(vs, ", ")
		}
		if r.Host != "" {
			headers["Host"] = r.Host
		}
		req := &Request{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Headers: headers, Body: string(body), remote: r.RemoteAddr}
		j := job{req: req, reply: make(chan Response, 1)}
		start := time.Now()
		select {
		case jobs <- j:
		case <-done:
			http.Error(w, "the server is stopping", http.StatusServiceUnavailable)
			return
		}
		res := <-j.reply
		for k, v := range res.Headers {
			w.Header().Set(k, v)
		}
		if res.Status == 0 {
			res.Status = http.StatusOK // a Go handler's zero Response is a 200
		}
		w.WriteHeader(res.Status)
		io.WriteString(w, res.Body)
		if !quiet {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			ctx.Write(true, fmt.Sprintf("%s - - [%s] %q %d %d %.4f\n", host, time.Now().Format("02/Jan/2006:15:04:05 -0700"), r.Method+" "+r.RequestURI+" "+r.Proto, res.Status, len(res.Body), time.Since(start).Seconds()))
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
				if exc, ok := err.(*object.Exception); !ok || !exc.Cls.IsA(object.StandardError) {
					j.reply <- Response{Status: http.StatusServiceUnavailable, Headers: map[string]string{"Content-Type": "text/plain;charset=utf-8"}, Body: "the server is stopping\n"}
					return err
				}
				text := ctx.FormatError(err)
				ctx.Write(true, text+"\n")
				res = Response{Status: http.StatusInternalServerError, Headers: map[string]string{"Content-Type": "text/plain;charset=utf-8"}, Body: text + "\n"}
			}
			j.reply <- res
		case <-done:
			return ctx.Stopped()
		}
	}
}

// Respond reads what a handler returned, in Sinatra's forms: a String is
// the body with status 200, an Integer a status with an empty body, nil
// an empty body, and an Array is [status, body] or [status, headers,
// body]. The content type is text/html;charset=utf-8 unless the headers
// set it.
func Respond(ctx object.Context, v object.Value) (Response, error) {
	res := Response{Status: http.StatusOK, Headers: map[string]string{}}
	var err error
	switch {
	case v.IsNil():
	case v.Kind() == object.KindString:
		res.Body = v.Str()
	case v.Kind() == object.KindInt:
		res.Status, err = status(ctx, v)
	case v.Kind() == object.KindList:
		elems := v.List().Elems
		switch len(elems) {
		case 2:
			if res.Status, err = status(ctx, elems[0]); err == nil {
				res.Body, err = body(ctx, elems[1])
			}
		case 3:
			if res.Status, err = status(ctx, elems[0]); err == nil {
				if res.Headers, err = headers(ctx, elems[1]); err == nil {
					res.Body, err = body(ctx, elems[2])
				}
			}
		default:
			err = ctx.Raise(object.TypeError, "a handler returned an Array of %d, not [status, body] or [status, headers, body]", len(elems))
		}
	default:
		err = ctx.Raise(object.TypeError, "a handler returned %s, not a String, an Integer or an Array", v.Class().QualifiedName())
	}
	if err != nil {
		return res, err
	}
	if !hasContentType(res.Headers) {
		res.Headers["Content-Type"] = "text/html;charset=utf-8"
	}
	return res, nil
}

// hasContentType says whether the headers set the content type, under
// any spelling of the name, since net/http canonicalises them.
func hasContentType(headers map[string]string) bool {
	for k := range headers {
		if http.CanonicalHeaderKey(k) == "Content-Type" {
			return true
		}
	}
	return false
}

// status reads a status: 200 to 999. net/http panics on one outside
// 100..999, and treats one below 200 as interim, never finishing the
// response, so such a one is the handler's error.
func status(ctx object.Context, v object.Value) (int, error) {
	if v.Kind() == object.KindInt && v.Int() >= 200 && v.Int() <= 999 {
		return int(v.Int()), nil
	}
	s, _ := ctx.Inspect(v)
	return 0, ctx.Raise(object.TypeError, "a handler returned the status %s, not an Integer from 200 to 999", s)
}

func body(ctx object.Context, v object.Value) (string, error) {
	if v.IsNil() {
		return "", nil
	}
	return ctx.ToS(v)
}

// headers reads a Hash of header names to values, each a String or a
// Symbol.
func headers(ctx object.Context, v object.Value) (map[string]string, error) {
	if v.Kind() != object.KindMap {
		return nil, ctx.Raise(object.TypeError, "a handler returned headers of %s, not a Hash", v.Class().QualifiedName())
	}
	m := v.Map()
	out := make(map[string]string, m.Len())
	for i, k := range m.Keys() {
		name, err := ctx.ToS(k)
		if err != nil {
			return nil, err
		}
		val, err := ctx.ToS(m.Values()[i])
		if err != nil {
			return nil, err
		}
		out[strings.TrimSpace(name)] = val
	}
	return out, nil
}
