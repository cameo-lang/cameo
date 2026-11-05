// Package stringio is Ruby's stringio library, built into cameo:
// StringIO, a handle that reads and writes a String as an IO does, for
// code that takes an IO.
//
// It follows stringio 3.1.2, the version Ruby 3.4 ships: the reading
// methods with a separator, a limit and chomp:, the writing methods that
// overwrite at the position and pad with NUL past the end, the position
// and seek, truncate, ungetc, the modes "r", "w", "a" and "r+", and the
// IOError, EOFError and Errno::EINVAL of Ruby. A StringIO is a handle,
// the one changing kind of value, as an IO is in Ruby.
package stringio

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	class  = object.NewClass("StringIO", object.ObjectClass)
	einval = object.NewClass("EINVAL", object.SystemCallError)
)

// sio is a StringIO's state.
type sio struct {
	buf      []byte
	pos      int
	lineno   int
	kind     object.Kind // the String kind of what is read and of string
	readable bool
	writable bool
	appends  bool
	closedR  bool
	closedW  bool
}

func sioOf(v object.Value) *sio { return v.Instance().Message.(*sio) }

func raise(cls *object.Class, msg string) error { return &object.Exception{Cls: cls, Message: msg} }

func (s *sio) checkRead() error {
	if !s.readable || s.closedR {
		return raise(object.IOError, "not opened for reading")
	}
	return nil
}

func (s *sio) checkWrite() error {
	if !s.writable || s.closedW {
		return raise(object.IOError, "not opened for writing")
	}
	return nil
}

// str makes a String of the kind the buffer was given as.
func (s *sio) str(b []byte) object.Value {
	if s.kind == object.KindBytes {
		return object.Bytes(append([]byte(nil), b...))
	}
	return object.StringOf(s.kind, string(b))
}

// write puts b at the position, over what is there and past the end,
// padding with NUL when the position is beyond it.
func (s *sio) write(b []byte) int {
	if s.appends {
		s.pos = len(s.buf)
	}
	for len(s.buf) < s.pos {
		s.buf = append(s.buf, 0)
	}
	end := s.pos + len(b)
	if end > len(s.buf) {
		s.buf = append(s.buf[:s.pos], b...)
	} else {
		copy(s.buf[s.pos:end], b)
	}
	s.pos = end
	return len(b)
}

// line reads the next line up to sep (nil for the rest, "" for a
// paragraph), at most limit bytes when limit is positive, and reports
// false at the end.
func (s *sio) line(sep *string, limit int, chomp bool) ([]byte, bool) {
	if s.pos >= len(s.buf) {
		return nil, false
	}
	rest := s.buf[s.pos:]
	var out []byte
	paragraph := sep != nil && *sep == ""
	if paragraph {
		for len(rest) > 0 && rest[0] == '\n' {
			rest = rest[1:]
			s.pos++
		}
		if len(rest) == 0 {
			return nil, false
		}
	}
	switch {
	case sep == nil:
		out = rest
	case paragraph:
		i := bytes.Index(rest, []byte("\n\n"))
		if i < 0 {
			out = rest
		} else {
			j := i + 2
			for j < len(rest) && rest[j] == '\n' {
				j++
			}
			out = rest[:j]
		}
	default:
		i := bytes.Index(rest, []byte(*sep))
		if i < 0 {
			out = rest
		} else {
			out = rest[:i+len(*sep)]
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
		// A limit does not cut a character in two.
		for len(out) < len(rest) && !utf8.RuneStart(rest[len(out)]) && s.kind != object.KindBytes {
			out = rest[:len(out)+1]
		}
	}
	s.pos += len(out)
	s.lineno++
	if chomp {
		if sep != nil && !paragraph {
			out = bytes.TrimSuffix(out, []byte(*sep))
		} else {
			out = bytes.TrimRight(out, "\n")
		}
	}
	return append([]byte(nil), out...), true
}

// lineArgs reads gets' arguments: a separator, a limit, or both, and
// chomp:.
func lineArgs(ctx object.Context, args []object.Value, kw *object.Map) (sep *string, limit int, chomp bool, err error) {
	nl := "\n"
	sep = &nl
	switch len(args) {
	case 0:
	case 1:
		if args[0].IsNil() {
			sep = nil
		} else if n, ok := args[0].AsInt(); ok {
			limit = int(n)
		} else {
			s, err := stdlib.WantString(ctx, args[0])
			if err != nil {
				return nil, 0, false, err
			}
			sep = &s
		}
	case 2:
		if args[0].IsNil() {
			sep = nil
		} else {
			s, err := stdlib.WantString(ctx, args[0])
			if err != nil {
				return nil, 0, false, err
			}
			sep = &s
		}
		n, err := stdlib.WantInt(ctx, args[1], "")
		if err != nil {
			return nil, 0, false, err
		}
		limit = int(n)
	default:
		return nil, 0, false, stdlib.ArgError(ctx, len(args), "0..2")
	}
	if kw != nil {
		if v, ok := kw.Get(object.Symbol("chomp")); ok {
			chomp = v.Truthy()
		}
	}
	return sep, limit, chomp, nil
}

// newSIO is StringIO.new(string = "", mode = "r+").
func newSIO(ctx object.Context, args []object.Value) (*sio, error) {
	s := &sio{readable: true, writable: true, kind: object.KindString}
	if len(args) > 2 {
		return nil, stdlib.ArgError(ctx, len(args), "0..2")
	}
	if len(args) >= 1 {
		if !args[0].IsString() {
			return nil, ctx.Raise(object.TypeError, "no implicit conversion of %s into String", stdlib.KindName(args[0]))
		}
		s.buf = []byte(args[0].Str())
		s.kind = args[0].Kind()
	}
	if len(args) == 2 && !args[1].IsNil() {
		mode, err := stdlib.WantString(ctx, args[1])
		if err != nil {
			return nil, err
		}
		base := strings.TrimRight(mode, "bt+")
		plus := strings.Contains(mode, "+")
		switch base {
		case "r":
			s.readable, s.writable = true, plus
		case "w":
			s.readable, s.writable = plus, true
			s.buf = nil
		case "a":
			s.readable, s.writable, s.appends = plus, true, true
		default:
			return nil, ctx.Raise(object.ArgumentError, "invalid access mode %s", mode)
		}
	}
	return s, nil
}

// Install defines StringIO. lib.Install calls it once.
func Install() {
	c := class
	object.ObjectClass.Consts["StringIO"] = object.ClassValue(c)
	c.Consts["VERSION"] = object.String("3.1.2")
	c.Consts["MAX_LENGTH"] = object.Int(int64(1<<62 - 1))
	c.Include(object.Enumerable)
	// IO's seek constants, for seek(offset, IO::SEEK_END).
	for i, name := range []string{"SEEK_SET", "SEEK_CUR", "SEEK_END"} {
		if _, ok := stdlib.IOClass.Consts[name]; !ok {
			stdlib.IOClass.Consts[name] = object.Int(i)
		}
		c.Consts[name] = object.Int(i)
	}
	if _, ok := object.ErrnoModule.Consts["EINVAL"]; !ok {
		einval.Lexical = object.ErrnoModule
		object.ErrnoModule.Consts["EINVAL"] = object.ClassValue(einval)
	} else {
		einval, _ = object.ErrnoModule.Consts["EINVAL"].AsClass()
	}
	wrap := func(s *sio) object.Value { return object.InstanceValue(&object.Instance{Cls: c, Message: s}) }
	c.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := newSIO(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		return wrap(s), nil
	})
	c.DefineSingleton("open", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := newSIO(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		v := wrap(s)
		if block.IsZero() {
			return v, nil
		}
		return ctx.Yield(block, v)
	})
	def := func(name string, arity int, f func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error)) {
		c.Define(name, arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return f(ctx, sioOf(self), self, args, kw, block)
		})
	}
	// Reading.
	def("read", -1, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := s.checkRead(); err != nil {
			return object.Value{}, err
		}
		if len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "0..2")
		}
		if len(args) == 0 || args[0].IsNil() {
			out := s.str(s.buf[min(s.pos, len(s.buf)):])
			s.pos = max(s.pos, len(s.buf))
			return out, nil
		}
		n, err := stdlib.WantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if n < 0 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "negative length %d given", n)
		}
		if n == 0 {
			return object.Bytes(nil), nil
		}
		if s.pos >= len(s.buf) {
			return object.Nil, nil
		}
		end := min(s.pos+int(n), len(s.buf))
		out := object.Bytes(append([]byte(nil), s.buf[s.pos:end]...))
		s.pos = end
		return out, nil
	})
	partial := func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := s.checkRead(); err != nil {
			return object.Value{}, err
		}
		if len(args) < 1 {
			return object.Value{}, stdlib.ArgError(ctx, 0, "1..2")
		}
		n, err := stdlib.WantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if s.pos >= len(s.buf) {
			return object.Value{}, raise(object.EOFError, "end of file reached")
		}
		end := min(s.pos+int(n), len(s.buf))
		out := object.Bytes(append([]byte(nil), s.buf[s.pos:end]...))
		s.pos = end
		return out, nil
	}
	def("readpartial", -1, partial)
	def("read_nonblock", -1, partial)
	def("pread", -1, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "2..3")
		}
		n, err := stdlib.WantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		off, err := stdlib.WantInt(ctx, args[1], "")
		if err != nil {
			return object.Value{}, err
		}
		if int(off) >= len(s.buf) {
			return object.Value{}, raise(object.EOFError, "end of file reached")
		}
		end := min(int(off+n), len(s.buf))
		return object.Bytes(append([]byte(nil), s.buf[off:end]...)), nil
	})
	gets := func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := s.checkRead(); err != nil {
			return object.Value{}, err
		}
		sep, limit, chomp, err := lineArgs(ctx, args, kw)
		if err != nil {
			return object.Value{}, err
		}
		line, ok := s.line(sep, limit, chomp)
		if !ok {
			return object.Nil, nil
		}
		return s.str(line), nil
	}
	def("gets", -1, gets)
	def("readline", -1, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		v, err := gets(ctx, s, self, args, kw, block)
		if err != nil {
			return object.Value{}, err
		}
		if v.IsNil() {
			return object.Value{}, raise(object.EOFError, "end of file reached")
		}
		return v, nil
	})
	eachLine := func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := s.checkRead(); err != nil {
			return object.Value{}, err
		}
		sep, limit, chomp, err := lineArgs(ctx, args, kw)
		if err != nil {
			return object.Value{}, err
		}
		if block.IsZero() {
			return object.EnumeratorValue(&object.Enumerator{Recv: self, Meth: "each_line", Each: func(ctx object.Context, yield func(object.Value) error) error {
				for {
					line, ok := s.line(sep, limit, chomp)
					if !ok {
						return nil
					}
					if err := yield(s.str(line)); err != nil {
						return err
					}
				}
			}}), nil
		}
		for {
			line, ok := s.line(sep, limit, chomp)
			if !ok {
				return self, nil
			}
			if _, err := ctx.Yield(block, s.str(line)); err != nil {
				return object.Value{}, err
			}
		}
	}
	def("each_line", -1, eachLine)
	def("each", -1, eachLine)
	def("readlines", -1, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := s.checkRead(); err != nil {
			return object.Value{}, err
		}
		sep, limit, chomp, err := lineArgs(ctx, args, kw)
		if err != nil {
			return object.Value{}, err
		}
		out := []object.Value{}
		for {
			line, ok := s.line(sep, limit, chomp)
			if !ok {
				return object.NewList(out...), nil
			}
			out = append(out, s.str(line))
		}
	})
	getc := func(s *sio) (object.Value, bool) {
		if s.pos >= len(s.buf) {
			return object.Nil, false
		}
		n := 1
		if s.kind != object.KindBytes {
			_, n = utf8.DecodeRune(s.buf[s.pos:])
		}
		out := s.str(s.buf[s.pos : s.pos+n])
		s.pos += n
		return out, true
	}
	def("getc", 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := s.checkRead(); err != nil {
			return object.Value{}, err
		}
		v, _ := getc(s)
		return v, nil
	})
	def("readchar", 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := s.checkRead(); err != nil {
			return object.Value{}, err
		}
		v, ok := getc(s)
		if !ok {
			return object.Value{}, raise(object.EOFError, "end of file reached")
		}
		return v, nil
	})
	getbyte := func(s *sio) (object.Value, bool) {
		if s.pos >= len(s.buf) {
			return object.Nil, false
		}
		b := s.buf[s.pos]
		s.pos++
		return object.Int(int64(b)), true
	}
	def("getbyte", 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := s.checkRead(); err != nil {
			return object.Value{}, err
		}
		v, _ := getbyte(s)
		return v, nil
	})
	def("readbyte", 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := s.checkRead(); err != nil {
			return object.Value{}, err
		}
		v, ok := getbyte(s)
		if !ok {
			return object.Value{}, raise(object.EOFError, "end of file reached")
		}
		return v, nil
	})
	eachOf := func(name string, next func(s *sio) (object.Value, bool)) {
		def(name, 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if err := s.checkRead(); err != nil {
				return object.Value{}, err
			}
			if block.IsZero() {
				return object.EnumeratorValue(&object.Enumerator{Recv: self, Meth: name, Each: func(ctx object.Context, yield func(object.Value) error) error {
					for {
						v, ok := next(s)
						if !ok {
							return nil
						}
						if err := yield(v); err != nil {
							return err
						}
					}
				}}), nil
			}
			for {
				v, ok := next(s)
				if !ok {
					return self, nil
				}
				if _, err := ctx.Yield(block, v); err != nil {
					return object.Value{}, err
				}
			}
		})
	}
	eachOf("each_char", getc)
	eachOf("each_byte", getbyte)
	unget := func(ctx object.Context, s *sio, b []byte) {
		if s.pos >= len(b) {
			s.pos -= len(b)
			copy(s.buf[s.pos:], b)
			return
		}
		rest := append([]byte(nil), s.buf[s.pos:]...)
		s.buf = append(append([]byte(nil), b...), rest...)
		s.pos = 0
	}
	def("ungetc", 1, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := s.checkRead(); err != nil {
			return object.Value{}, err
		}
		if args[0].IsNil() {
			return object.Nil, nil
		}
		if n, ok := args[0].AsInt(); ok {
			unget(ctx, s, []byte(string(rune(n))))
			return object.Nil, nil
		}
		str, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		unget(ctx, s, []byte(str))
		return object.Nil, nil
	})
	def("ungetbyte", 1, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := s.checkRead(); err != nil {
			return object.Value{}, err
		}
		if args[0].IsNil() {
			return object.Nil, nil
		}
		if n, ok := args[0].AsInt(); ok {
			unget(ctx, s, []byte{byte(n)})
			return object.Nil, nil
		}
		str, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		unget(ctx, s, []byte(str))
		return object.Nil, nil
	})
	// Writing.
	def("write", -1, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := s.checkWrite(); err != nil {
			return object.Value{}, err
		}
		n := 0
		for _, a := range args {
			str, err := ctx.ToS(a)
			if err != nil {
				return object.Value{}, err
			}
			n += s.write([]byte(str))
		}
		return object.Int(int64(n)), nil
	})
	def("<<", 1, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := s.checkWrite(); err != nil {
			return object.Value{}, err
		}
		str, err := ctx.ToS(args[0])
		if err != nil {
			return object.Value{}, err
		}
		s.write([]byte(str))
		return self, nil
	})
	def("print", -1, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := s.checkWrite(); err != nil {
			return object.Value{}, err
		}
		for _, a := range args {
			str, err := ctx.ToS(a)
			if err != nil {
				return object.Value{}, err
			}
			s.write([]byte(str))
		}
		return object.Nil, nil
	})
	def("puts", -1, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := s.checkWrite(); err != nil {
			return object.Value{}, err
		}
		if len(args) == 0 {
			s.write([]byte("\n"))
			return object.Nil, nil
		}
		var put func(v object.Value) error
		put = func(v object.Value) error {
			if list, ok := v.AsList(); ok {
				for _, e := range list.Elems {
					if err := put(e); err != nil {
						return err
					}
				}
				return nil
			}
			str := ""
			if !v.IsNil() {
				var err error
				if str, err = ctx.ToS(v); err != nil {
					return err
				}
			}
			s.write([]byte(str))
			if !strings.HasSuffix(str, "\n") {
				s.write([]byte("\n"))
			}
			return nil
		}
		for _, a := range args {
			if err := put(a); err != nil {
				return object.Value{}, err
			}
		}
		return object.Nil, nil
	})
	def("printf", -1, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := s.checkWrite(); err != nil {
			return object.Value{}, err
		}
		if len(args) == 0 {
			return object.Nil, nil
		}
		v, err := ctx.Call(object.ClassValue(object.ObjectClass), "format", args...)
		if err != nil {
			return object.Value{}, err
		}
		s.write([]byte(v.Str()))
		return object.Nil, nil
	})
	def("truncate", 1, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := s.checkWrite(); err != nil {
			return object.Value{}, err
		}
		n, err := stdlib.WantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if n < 0 {
			return object.Value{}, raise(einval, "Invalid argument - negative length")
		}
		for len(s.buf) < int(n) {
			s.buf = append(s.buf, 0)
		}
		s.buf = s.buf[:n]
		return object.Int(0), nil
	})
	// Position and state.
	def("string", 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return s.str(s.buf), nil
	})
	def("string=", 1, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		str, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		s.buf, s.pos, s.lineno, s.kind = []byte(str), 0, 0, args[0].Kind()
		return self, nil
	})
	def("reopen", -1, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, err := newSIO(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		*s = *n
		return self, nil
	})
	for _, name := range []string{"pos", "tell"} {
		def(name, 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Int(s.pos), nil
		})
	}
	def("pos=", 1, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, err := stdlib.WantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if n < 0 {
			return object.Value{}, raise(einval, "Invalid argument")
		}
		s.pos = int(n)
		return self, nil
	})
	def("seek", -1, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		off, err := stdlib.WantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		whence := int64(0)
		if len(args) == 2 {
			if name, ok := args[1].AsSymbol(); ok {
				switch name {
				case "SET":
					whence = 0
				case "CUR":
					whence = 1
				case "END":
					whence = 2
				default:
					return object.Value{}, raise(einval, "Invalid argument - invalid whence")
				}
			} else if whence, err = stdlib.WantInt(ctx, args[1], ""); err != nil {
				return object.Value{}, err
			}
		}
		var base int64
		switch whence {
		case 0:
		case 1:
			base = int64(s.pos)
		case 2:
			base = int64(len(s.buf))
		default:
			return object.Value{}, raise(einval, "Invalid argument - invalid whence")
		}
		if base+off < 0 {
			return object.Value{}, raise(einval, "Invalid argument")
		}
		s.pos = int(base + off)
		return object.Int(0), nil
	})
	def("rewind", 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s.pos, s.lineno = 0, 0
		return object.Int(0), nil
	})
	for _, name := range []string{"eof?", "eof"} {
		def(name, 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if err := s.checkRead(); err != nil {
				return object.Value{}, err
			}
			return object.Bool(s.pos >= len(s.buf)), nil
		})
	}
	for _, name := range []string{"size", "length"} {
		def(name, 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Int(len(s.buf)), nil
		})
	}
	def("lineno", 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(s.lineno), nil
	})
	def("lineno=", 1, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, err := stdlib.WantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		s.lineno = int(n)
		return self, nil
	})
	def("close", 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s.closedR, s.closedW = true, true
		return object.Nil, nil
	})
	def("close_read", 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if !s.readable {
			return object.Value{}, raise(object.IOError, "closing non-duplex IO for reading")
		}
		s.closedR = true
		return object.Nil, nil
	})
	def("close_write", 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if !s.writable {
			return object.Value{}, raise(object.IOError, "closing non-duplex IO for writing")
		}
		s.closedW = true
		return object.Nil, nil
	})
	def("closed?", 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool((s.closedR || !s.readable) && (s.closedW || !s.writable)), nil
	})
	def("closed_read?", 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(s.closedR || !s.readable), nil
	})
	def("closed_write?", 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(s.closedW || !s.writable), nil
	})
	// What an IO answers and a String needs none of.
	for _, name := range []string{"flush", "binmode", "set_encoding"} {
		def(name, -1, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return self, nil
		})
	}
	def("sync", 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.True, nil
	})
	def("sync=", 1, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def("fsync", 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(0), nil
	})
	for _, name := range []string{"tty?", "isatty"} {
		def(name, 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.False, nil
		})
	}
	for _, name := range []string{"fileno", "pid", "internal_encoding"} {
		def(name, 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Nil, nil
		})
	}
	def("external_encoding", 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return ctx.Call(s.str(nil), "encoding")
	})
	def("inspect", 0, func(ctx object.Context, s *sio, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(fmt.Sprintf("#<StringIO pos:%d size:%d>", s.pos, len(s.buf))), nil
	})
}
