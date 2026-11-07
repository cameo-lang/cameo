// Package rack is Ruby's Rack, require "rack": the interface between a web
// server and a web application. Go defines the Rack module, the part of
// Rack::Request that holds the env, as Rack's Rack::Request::Env does, and
// Rack::Handler, the servers registered in the program; rack.cameo, which
// this package embeds and lib.Sources registers, opens the module and the
// class to add the rest: the request's readers, Response, Builder with
// middleware, URLMap, MockRequest and Utils.
package rack

import (
	_ "embed"
	"sync"

	"github.com/cameo-lang/cameo/src/object"
)

// Source is the library's cameo part, which require runs at the top level.
//
//go:embed rack.cameo
var Source string

var (
	module  = object.NewModule("Rack")
	request = object.NewClass("Request", object.ObjectClass)
	handler = object.NewModule("Handler")
)

// Install defines Rack, Rack::Request with what holds the env: new(env),
// env, get_header and has_header?, and Rack::Handler.
func Install() {
	object.DefineNative("Rack", object.ClassValue(module))
	request.Lexical = module
	module.Consts[request.Name] = object.ClassValue(request)
	installHandler()

	// new(env) keeps env, a Hash, on the request, of the class new was
	// called on, so a subclass's requests are the subclass's.
	request.DefineSingleton("new", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if _, ok := args[0].AsMap(); !ok {
			cls, err := ctx.Call(args[0], "class")
			if err != nil {
				return object.Value{}, err
			}
			name, err := ctx.ToS(cls)
			if err != nil {
				return object.Value{}, err
			}
			return object.Value{}, ctx.Raise(object.TypeError, "a Rack env is a Hash, not %s", name)
		}
		return object.InstanceValue(&object.Instance{Cls: self.Cls(), Message: env{hash: args[0]}}), nil
	})
	request.Define("env", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self.Instance().Message.(env).hash, nil
	})
	request.Define("get_header", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return ctx.Call(self.Instance().Message.(env).hash, "[]", args[0])
	})
	request.Define("has_header?", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(self.Instance().Message.(env).hash.Map().Has(args[0])), nil
	})
}

// env is what a request holds: the Hash it was made with, which, as every
// Hash, never changes.
type env struct{ hash object.Value }

// installHandler defines Rack::Handler, the registry of the servers that
// run a Rack app, as rackup's Rack::Handler: a server registers itself
// under a name when a script requires it, and a framework runs an app on
// Handler.default without naming the server. A server is a value that
// answers run(app, **options), the options rackup's, :Host and :Port among
// them; one that answers allowed? false cannot serve in this program, and
// default passes it over. The registry is the program's: cameo values do
// not change, so it is kept in Go, in the run's state.
func installHandler() {
	handler.Lexical = module
	module.Consts[handler.Name] = object.ClassValue(handler)
	// register(name, server) registers server under name, a String or a
	// Symbol, after those registered before it, or in its place, and
	// returns server.
	handler.DefineSingleton("register", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := handlerName(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if !ctx.RespondsTo(args[1], "run") {
			s, _ := ctx.Inspect(args[1])
			return object.Value{}, ctx.Raise(object.ArgumentError, "a Rack handler responds to run; %s does not", s)
		}
		registry(ctx).register(name, args[1])
		return args[1], nil
	})
	// get(name) returns the server registered under name, or nil.
	handler.DefineSingleton("get", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := handlerName(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		server, ok := registry(ctx).get(name)
		if !ok {
			return object.Nil, nil
		}
		return server, nil
	})
	// default returns the first server registered that can serve in this
	// program, or nil when there is none.
	handler.DefineSingleton("default", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for _, server := range registry(ctx).servers() {
			if ctx.RespondsTo(server, "allowed?") {
				ok, err := ctx.Call(server, "allowed?")
				if err != nil {
					return object.Value{}, err
				}
				if !ok.Truthy() {
					continue
				}
			}
			return server, nil
		}
		return object.Nil, nil
	})
	// names returns the names registered, in order.
	handler.DefineSingleton("names", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		r := registry(ctx)
		r.mu.Lock()
		defer r.mu.Unlock()
		out := make([]object.Value, len(r.names))
		for i, n := range r.names {
			out[i] = object.String(n)
		}
		return object.NewList(out...), nil
	})
}

func handlerName(ctx object.Context, v object.Value) (string, error) {
	if v.Kind() == object.KindSymbol {
		return v.Str(), nil
	}
	s, ok := v.AsString()
	if !ok {
		return "", ctx.Raise(object.TypeError, "a Rack handler's name is a String or a Symbol, not %s", v.Class().QualifiedName())
	}
	return s, nil
}

// handlers are the servers registered in one program, by name, in the
// order they were first registered.
type handlers struct {
	mu     sync.Mutex
	names  []string
	byName map[string]object.Value
}

// registry returns the program's handlers.
func registry(ctx object.Context) *handlers {
	return ctx.State("rack/handlers", func() any { return &handlers{byName: map[string]object.Value{}} }).(*handlers)
}

func (h *handlers) register(name string, server object.Value) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.byName[name]; !ok {
		h.names = append(h.names, name)
	}
	h.byName[name] = server
}

func (h *handlers) get(name string) (object.Value, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	v, ok := h.byName[name]
	return v, ok
}

func (h *handlers) servers() []object.Value {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]object.Value, len(h.names))
	for i, n := range h.names {
		out[i] = h.byName[n]
	}
	return out
}
