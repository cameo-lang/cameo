// Package digest is Ruby's digest library, built into cameo: Digest::MD5,
// SHA1, SHA256, SHA384, SHA512 and SHA2 with hexdigest, digest and
// base64digest, and OpenSSL::HMAC and OpenSSL::Digest under their names.
//
// It follows digest 3.2.0, the version Ruby 3.4 ships. A digest object is
// an instance that holds what it was fed: update and << return the copy
// with more fed, for the script to bind (d = d << "x"), where Ruby feeds
// the object in place. RMD160 is left out: Go's standard library has no
// RIPEMD-160. Digest::SHA256.file reads a file, which cameo does not.
package digest

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"hash"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	digestModule  = object.NewModule("Digest")
	instanceMod   = object.NewModule("Instance")
	classClass    = object.NewClass("Class", object.ObjectClass)
	baseClass     = object.NewClass("Base", classClass)
	sha2Class     = object.NewClass("SHA2", classClass)
	hmacClass     *object.Class
	opensslDigest *object.Class
	// algorithms by class, with their hash constructors and block sizes.
	algorithms = map[*object.Class]algorithm{}
)

type algorithm struct {
	name  string
	bits  int
	block int
	fn    func() hash.Hash
}

var known = []algorithm{
	{"MD5", 128, 64, md5.New}, {"SHA1", 160, 64, sha1.New}, {"SHA256", 256, 64, sha256.New},
	{"SHA384", 384, 128, sha512.New384}, {"SHA512", 512, 128, sha512.New},
}

func byBits(bits int) (algorithm, bool) {
	for _, a := range known[2:] {
		if a.bits == bits {
			return a, true
		}
	}
	return algorithm{}, false
}

func byName(name string) (algorithm, bool) {
	name = strings.ToUpper(strings.ReplaceAll(name, "-", ""))
	for _, a := range known {
		if a.name == name {
			return a, true
		}
	}
	return algorithm{}, false
}

// The fields of a digest object: the bytes fed so far, and the bit length
// of a Digest::SHA2.
const (
	fData = iota
	fBits
)

func declare(c *object.Class) {
	taken := func(number int64) bool {
		for _, f := range c.Fields {
			if f.Number == number {
				return true
			}
		}
		return false
	}
	for _, f := range []struct{ name, typ string }{{"data", "bytes"}, {"bits", "int"}} {
		field := &object.Field{Name: f.name, Number: protogen.FieldNumber(f.name, taken), Type: object.ScalarField(f.typ), Owner: c}
		c.FieldIndex[f.name] = len(c.Fields)
		c.Fields = append(c.Fields, field)
	}
}

func inherit(c *object.Class) {
	for _, f := range c.Super.Fields {
		c.FieldIndex[f.Name] = len(c.Fields)
		c.Fields = append(c.Fields, f)
	}
}

// Install defines Digest and its classes, and OpenSSL::HMAC and
// OpenSSL::Digest. lib.Install calls it once.
func Install() {
	m := digestModule
	object.ObjectClass.Consts["Digest"] = object.ClassValue(m)
	for _, c := range []*object.Class{instanceMod, classClass, baseClass, sha2Class} {
		c.Lexical = m
		m.Consts[c.Name] = object.ClassValue(c)
	}
	classClass.Include(instanceMod)
	declare(classClass)
	inherit(baseClass)
	inherit(sha2Class)
	for _, a := range known {
		c := object.NewClass(a.name, baseClass)
		c.Lexical = m
		m.Consts[a.name] = object.ClassValue(c)
		inherit(c)
		algorithms[c] = a
	}
	installClasses()
	installOpenSSL()
}

// algorithmOf is the algorithm of an instance: its class's, or a SHA2's by
// its bits.
func algorithmOf(v object.Value) algorithm {
	inst := v.Instance()
	if a, ok := algorithms[inst.Cls]; ok {
		return a
	}
	bits, _ := inst.Fields[fBits].AsInt()
	a, _ := byBits(int(bits))
	return a
}

func data(v object.Value) []byte {
	f := v.Instance().Fields[fData]
	if f.IsNil() || f.IsZero() {
		return nil
	}
	return []byte(f.Str())
}

func build(cls *object.Class, bits int, d []byte) object.Value {
	fields := make([]object.Value, len(cls.Fields))
	fields[fData] = object.Bytes(d)
	fields[fBits] = object.Int(int64(bits))
	return object.InstanceValue(&object.Instance{Cls: cls, Fields: fields})
}

func sum(a algorithm, d []byte) []byte {
	h := a.fn()
	h.Write(d)
	return h.Sum(nil)
}

// algorithmFor is the algorithm of a class a class method is called on.
func algorithmFor(ctx object.Context, cls *object.Class, args []object.Value) (algorithm, []object.Value, error) {
	if cls == sha2Class || cls.IsA(sha2Class) && cls != sha2Class {
		bits := 256
		// SHA2.new(384): the bit length is the first argument.
		if len(args) > 0 {
			if n, ok := args[0].AsInt(); ok {
				bits = int(n)
				args = args[1:]
			}
		}
		a, ok := byBits(bits)
		if !ok {
			return algorithm{}, nil, ctx.Raise(object.ArgumentError, "unsupported bit length: %d", bits)
		}
		return a, args, nil
	}
	for c := cls; c != nil; c = c.Super {
		if a, ok := algorithms[c]; ok {
			return a, args, nil
		}
	}
	return algorithm{}, nil, ctx.Raise(object.NotImplementedErr, "%s is an abstract class", cls.QualifiedName())
}

func installClasses() {
	c := classClass
	c.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		a, rest, err := algorithmFor(ctx, self.Cls(), args)
		if err != nil {
			return object.Value{}, err
		}
		if len(rest) > 0 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "0")
		}
		return build(self.Cls(), a.bits, nil), nil
	})
	// hexdigest(str), digest(str) and base64digest(str) on the class.
	classFn := func(name string, encode func([]byte) object.Value) {
		c.DefineSingleton(name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			a, rest, err := algorithmFor(ctx, self.Cls(), args)
			if err != nil {
				return object.Value{}, err
			}
			if len(rest) != 1 {
				return object.Value{}, stdlib.ArgError(ctx, len(rest), "1")
			}
			s, err := stdlib.WantString(ctx, rest[0])
			if err != nil {
				return object.Value{}, err
			}
			return encode(sum(a, []byte(s))), nil
		})
	}
	hexOf := func(b []byte) object.Value { return object.String(hex.EncodeToString(b)) }
	binOf := func(b []byte) object.Value { return object.Bytes(b) }
	b64Of := func(b []byte) object.Value { return object.String(base64.StdEncoding.EncodeToString(b)) }
	classFn("hexdigest", hexOf)
	classFn("digest", binOf)
	classFn("base64digest", b64Of)
	c.DefineSingleton("file", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Value{}, ctx.Raise(object.NotImplementedErr, "%s.file reads a file, which cameo does not; digest the string instead", self.Cls().QualifiedName())
	})

	// The instance methods: hexdigest and kin of what was fed, or of the
	// one argument alone, as Ruby's are.
	instFn := func(name string, encode func([]byte) object.Value) {
		c.Define(name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) > 1 {
				return object.Value{}, stdlib.ArgError(ctx, len(args), "0..1")
			}
			d := data(self)
			if len(args) == 1 {
				s, err := stdlib.WantString(ctx, args[0])
				if err != nil {
					return object.Value{}, err
				}
				d = []byte(s)
			}
			return encode(sum(algorithmOf(self), d)), nil
		})
	}
	instFn("hexdigest", hexOf)
	instFn("digest", binOf)
	instFn("base64digest", b64Of)
	instFn("to_s", hexOf)
	feed := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		inst := self.Instance()
		bits, _ := inst.Fields[fBits].AsInt()
		return build(inst.Cls, int(bits), append(append([]byte(nil), data(self)...), s...)), nil
	}
	c.Define("update", 1, feed)
	c.Define("<<", 1, feed)
	c.Define("reset", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		inst := self.Instance()
		bits, _ := inst.Fields[fBits].AsInt()
		return build(inst.Cls, int(bits), nil), nil
	})
	c.Define("file", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Value{}, ctx.Raise(object.NotImplementedErr, "%s#file reads a file, which cameo does not; digest the string instead", self.Instance().Cls.QualifiedName())
	})
	size := func(name string, f func(a algorithm) int) {
		c.Define(name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Int(int64(f(algorithmOf(self)))), nil
		})
	}
	size("digest_length", func(a algorithm) int { return a.bits / 8 })
	size("length", func(a algorithm) int { return a.bits / 8 })
	size("size", func(a algorithm) int { return a.bits / 8 })
	size("block_length", func(a algorithm) int { return a.block })
	c.Define("inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String("#<" + self.Instance().Cls.QualifiedName() + ": " + hex.EncodeToString(sum(algorithmOf(self), data(self))) + ">"), nil
	})
	// == takes another digest object, or the hex string of a digest.
	c.Define("==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		mine := hex.EncodeToString(sum(algorithmOf(self), data(self)))
		if other, ok := args[0].AsInstance(); ok && other.Cls.IsA(classClass) {
			return object.Bool(mine == hex.EncodeToString(sum(algorithmOf(args[0]), data(args[0])))), nil
		}
		if s, ok := args[0].AsString(); ok {
			return object.Bool(mine == s), nil
		}
		return object.False, nil
	})

	digestModule.DefineSingleton("hexencode", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.String(hex.EncodeToString([]byte(s))), nil
	})
	// Digest(:SHA256) is the class.
	object.ObjectClass.Define("Digest", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if v, ok := digestModule.Consts[name]; ok {
			return v, nil
		}
		return object.Value{}, ctx.Raise(object.LoadError, "library not found for class Digest::%s -- digest/%s", name, strings.ToLower(name))
	})
}

// installOpenSSL defines OpenSSL::HMAC and OpenSSL::Digest, the names
// scripts written for openssl use for the same algorithms.
func installOpenSSL() {
	openssl := stdlib.Module("OpenSSL")
	opensslDigest = object.NewClass("Digest", object.ObjectClass)
	opensslDigest.Lexical = openssl
	openssl.Consts["Digest"] = object.ClassValue(opensslDigest)
	for _, a := range known {
		opensslDigest.Consts[a.name] = digestModule.Consts[a.name]
	}
	// OpenSSL::Digest.new("SHA256") is a digest object of that algorithm;
	// OpenSSL::Digest.digest("SHA256", data) and hexdigest its sum.
	named := func(ctx object.Context, v object.Value) (*object.Class, algorithm, error) {
		name, err := stdlib.WantString(ctx, v)
		if err != nil {
			return nil, algorithm{}, err
		}
		a, ok := byName(name)
		if !ok {
			return nil, algorithm{}, ctx.Raise(object.RuntimeError, "Unsupported digest algorithm (%s).: unknown object name", name)
		}
		return digestModule.Consts[a.name].Cls(), a, nil
	}
	opensslDigest.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		cls, a, err := named(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		var d []byte
		if len(args) == 2 {
			s, err := stdlib.WantString(ctx, args[1])
			if err != nil {
				return object.Value{}, err
			}
			d = []byte(s)
		}
		return build(cls, a.bits, d), nil
	})
	for name, encode := range map[string]func([]byte) object.Value{
		"digest":       func(b []byte) object.Value { return object.Bytes(b) },
		"hexdigest":    func(b []byte) object.Value { return object.String(hex.EncodeToString(b)) },
		"base64digest": func(b []byte) object.Value { return object.String(base64.StdEncoding.EncodeToString(b)) },
	} {
		encode := encode
		opensslDigest.DefineSingleton(name, 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			_, a, err := named(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			s, err := stdlib.WantString(ctx, args[1])
			if err != nil {
				return object.Value{}, err
			}
			return encode(sum(a, []byte(s))), nil
		})
		hm := encode
		if hmacClass == nil {
			hmacClass = object.NewClass("HMAC", object.ObjectClass)
			hmacClass.Lexical = openssl
			openssl.Consts["HMAC"] = object.ClassValue(hmacClass)
		}
		// OpenSSL::HMAC.hexdigest(algorithm, key, data) and kin.
		hmacClass.DefineSingleton(name, 3, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			_, a, err := named(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			key, err := stdlib.WantString(ctx, args[1])
			if err != nil {
				return object.Value{}, err
			}
			msg, err := stdlib.WantString(ctx, args[2])
			if err != nil {
				return object.Value{}, err
			}
			mac := hmac.New(a.fn, []byte(key))
			mac.Write([]byte(msg))
			return hm(mac.Sum(nil)), nil
		})
	}
}
