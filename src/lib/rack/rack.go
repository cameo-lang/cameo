// Package rack is Ruby's Rack, require "rack": the interface io/httpd
// serves and frankie is built on. Go defines the Rack module and the part
// of Rack::Request that holds the env, as Rack's Rack::Request::Env does;
// rack.cameo, which this package embeds and lib.Sources registers, opens
// the module and the class to add the rest: the request's readers,
// Response, Builder with middleware, URLMap, MockRequest and Utils.
package rack

import (
	_ "embed"

	"github.com/cameo-lang/cameo/src/object"
)

// Source is the library's cameo part, which require runs at the top level.
//
//go:embed rack.cameo
var Source string

var (
	module  = object.NewModule("Rack")
	request = object.NewClass("Request", object.ObjectClass)
)

// Install defines Rack and Rack::Request with what holds the env: new(env),
// env, get_header and has_header?.
func Install() {
	object.DefineNative("Rack", object.ClassValue(module))
	request.Lexical = module
	module.Consts[request.Name] = object.ClassValue(request)

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
