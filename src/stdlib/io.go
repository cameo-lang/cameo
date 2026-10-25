package stdlib

import (
	"github.com/cameo-lang/cameo/src/object"
)

// IO is the class of STDOUT and STDERR, which $stdout and $stderr name:
// the script's standard output and error, with puts, print, write, flush
// and sync as Ruby's IO has them. There is no <<, which the checker
// refuses on a receiver it cannot rebind, and no other stream: a script
// has no files, and reads nothing.

// IOClass is IO.
var IOClass = object.NewClass("IO", object.ObjectClass)

var stdout, stderr object.Value

// installIO defines IO, STDOUT and STDERR.
func installIO() {
	c := IOClass
	object.ObjectClass.Consts["IO"] = object.ClassValue(c)
	stdout = object.InstanceValue(&object.Instance{Cls: c, Fields: []object.Value{object.String("stdout")}})
	stderr = object.InstanceValue(&object.Instance{Cls: c, Fields: []object.Value{object.String("stderr")}})
	object.ObjectClass.Consts["STDOUT"] = stdout
	object.ObjectClass.Consts["STDERR"] = stderr
	// isErr reports whether self is the error stream.
	isErr := func(self object.Value) bool { return self.Instance().Fields[0].Str() == "stderr" }
	def(c, "puts", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Nil, puts(ctx, isErr(self), args)
	})
	def(c, "print", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for _, a := range args {
			s, err := ctx.ToS(a)
			if err != nil {
				return object.Value{}, err
			}
			if err := ctx.Write(isErr(self), s); err != nil {
				return object.Value{}, err
			}
		}
		return object.Nil, nil
	})
	// write writes each argument's to_s and returns the bytes written.
	def(c, "write", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n := 0
		for _, a := range args {
			s, err := ctx.ToS(a)
			if err != nil {
				return object.Value{}, err
			}
			if err := ctx.Write(isErr(self), s); err != nil {
				return object.Value{}, err
			}
			n += len(s)
		}
		return object.Int(int64(n)), nil
	})
	// The streams are not buffered: flush does nothing, and sync is true.
	def(c, "flush", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def(c, "sync", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.True, nil
	})
	def(c, "fileno", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if isErr(self) {
			return object.Int(2), nil
		}
		return object.Int(1), nil
	})
	def(c, "tty?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.False, nil
	})
}
