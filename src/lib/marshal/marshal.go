// Package marshal is Ruby's Marshal, built into cameo in cameo's own wire
// form: Marshal.dump writes a value as the protobuf cameo.Value envelope
// every value crosses to a host in, and Marshal.load reads it back.
//
// The bytes are not Ruby's marshal format, which carries object identity
// and instance variables that cameo has neither of; they are what
// to_value_proto and from_value_proto give, so cameo reads them back, any
// protobuf reader with cameo/value.proto reads them too, and a script
// round-trips its values as it would in Ruby, with two differences the
// envelope brings: a Symbol comes back as a String, as it crosses to a
// host, and an instance of a library's class, a Date or a URI, cannot be
// dumped, since the envelope carries the instances of classes with fields
// alone. A Proc, a class and a handle such as a Random cannot be dumped
// and raise TypeError, as a Proc does in Ruby; an instance of a cameo
// class is found again by its class's name and digest, as from_proto
// finds it. Marshal.dump to an IO and Marshal.load from one are left out,
// with Ruby's MAJOR_VERSION and MINOR_VERSION, which name Ruby's format.
package marshal

import (
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var module = object.NewModule("Marshal")

// envelope is what the interpreter gives a library to serialise with: the
// evaluator's context implements it.
type envelope interface {
	Encoder() *protogen.Encoder
	Decoder() *protogen.Decoder
}

// Install defines Marshal. lib.Install calls it once.
func Install() {
	m := module
	object.DefineNative("Marshal", object.ClassValue(m))
	m.DefineSingleton("dump", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		if len(args) == 2 {
			// Ruby's second argument is an IO, or the depth limit.
			if _, ok := args[1].AsInt(); !ok {
				return object.Value{}, ctx.Raise(object.TypeError, "Marshal.dump writes a String: an IO is not supported")
			}
		}
		env, ok := ctx.(envelope)
		if !ok {
			return object.Value{}, ctx.Raise(object.CameoError, "Marshal.dump needs the interpreter's encoder")
		}
		data, err := env.Encoder().Encode(args[0])
		if err != nil {
			if _, ok := err.(*protogen.NotSerialisable); ok {
				return object.Value{}, ctx.Raise(object.TypeError, "no _dump_data is defined for class %s", args[0].Class().QualifiedName())
			}
			return object.Value{}, ctx.Raise(object.TypeError, "no _dump_data is defined for class %s: %s", args[0].Class().QualifiedName(), err.Error())
		}
		return object.Bytes(data), nil
	})
	m.DefineSingleton("load", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		if len(args) == 2 && !args[1].IsNil() {
			return object.Value{}, ctx.Raise(object.TypeError, "Marshal.load takes no proc: every value loaded is a value")
		}
		if kw != nil {
			for _, k := range kw.Keys() {
				if name, _ := k.AsSymbol(); name != "freeze" {
					inspect, _ := ctx.Inspect(k)
					return object.Value{}, ctx.Raise(object.ArgumentError, "unknown keyword: %s", inspect)
				}
			}
		}
		var data []byte
		switch args[0].Kind() {
		case object.KindBytes:
			data = args[0].Bytes()
		case object.KindString, object.KindASCII:
			data = []byte(args[0].Str())
		default:
			return object.Value{}, ctx.Raise(object.TypeError, "instance of IO needed")
		}
		env, ok := ctx.(envelope)
		if !ok {
			return object.Value{}, ctx.Raise(object.CameoError, "Marshal.load needs the interpreter's decoder")
		}
		v, err := env.Decoder().Decode(data)
		if err != nil {
			return object.Value{}, ctx.Raise(object.ArgumentError, "incompatible marshal data: %s", err.Error())
		}
		return v, nil
	})
	m.DefineSingleton("restore", -1, m.SingletonMethods["load"].Native)
}
