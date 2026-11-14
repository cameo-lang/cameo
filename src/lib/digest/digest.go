// Package digest is Ruby's digest library, built into cameo: Digest::MD5,
// SHA1, SHA256, SHA384, SHA512 and SHA2 with hexdigest, digest and
// base64digest, and OpenSSL::HMAC and OpenSSL::Digest under their names.
//
// It follows digest 3.2.0, the version Ruby 3.4 ships. A digest object is
// an instance that holds the state of its hash after what it was fed:
// update and << return the copy with more fed, for the script to bind
// (d = d << "x"), where Ruby feeds the object in place. RMD160 is left
// out: Go's standard library has no RIPEMD-160. Digest::SHA256.file reads
// a file, which cameo does not.
package digest

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding"
	"encoding/base64"
	"encoding/hex"
	"hash"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	digestModule  = object.NewModule("Digest")
	instanceMod   = object.NewModule("Instance")
	classClass    = object.NewClass("Class", object.ObjectClass)
	baseClass     = object.NewClass("Base", classClass)
	sha2Class     = object.NewClass("SHA2", classClass)
	hmacClass     = object.NewClass("HMAC", object.ObjectClass)
	opensslDigest = object.NewClass("Digest", object.ObjectClass)
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

// sha2Bits is the SHA2 algorithm of a bit length: Digest::SHA2.new(384).
func sha2Bits(bits int) (algorithm, bool) {
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

// encoders are the three forms a sum is given in, by the method names
// digest, OpenSSL::Digest and OpenSSL::HMAC share.
var encoders = []struct {
	name   string
	encode func([]byte) object.Value
}{
	{"hexdigest", func(b []byte) object.Value { return object.NewString(hex.EncodeToString(b), object.ASCII) }},
	{"digest", func(b []byte) object.Value { return object.Bytes(b) }},
	{"base64digest", func(b []byte) object.Value {
		return object.NewString(base64.StdEncoding.EncodeToString(b), object.ASCII)
	}},
}

// The fields of a digest object: the state of its hash after what it was
// fed, as Go marshals it, empty before anything was, and the bit length
// of a Digest::SHA2.
const (
	fState = iota
	fBits
)

// Install defines Digest and its classes, and OpenSSL::HMAC and
// OpenSSL::Digest. lib.Install calls it once.
func Install() {
	m := digestModule
	object.DefineNative("Digest", object.ClassValue(m))
	for _, c := range []*object.Class{instanceMod, classClass, baseClass, sha2Class} {
		c.Lexical = m
		m.Consts[c.Name] = object.ClassValue(c)
	}
	classClass.Include(instanceMod)
	stdlib.DeclareFields(classClass, stdlib.FieldDecl{Name: "state", Type: "bytes"}, stdlib.FieldDecl{Name: "bits", Type: "int"})
	stdlib.InheritFields(baseClass)
	stdlib.InheritFields(sha2Class)
	for _, a := range known {
		c := object.NewClass(a.name, baseClass)
		c.Lexical = m
		m.Consts[a.name] = object.ClassValue(c)
		stdlib.InheritFields(c)
		algorithms[c] = a
	}
	installClasses()
	installOpenSSL()
}

// algorithmOf is the algorithm of an instance: its class's, or a
// superclass's, or a SHA2's by its bits. Every known length is one
// algorithm's.
func algorithmOf(v object.Value) algorithm {
	inst := v.Instance()
	for c := inst.Cls; c != nil; c = c.Super {
		if a, ok := algorithms[c]; ok {
			return a
		}
	}
	bits, _ := inst.Fields[fBits].AsInt()
	for _, a := range known {
		if a.bits == int(bits) {
			return a
		}
	}
	return algorithm{}
}

// hashOf is the hash of a digest object, fed what the object was.
func hashOf(v object.Value) hash.Hash {
	h := algorithmOf(v).fn()
	if f := v.Instance().Fields[fState]; !f.IsNil() && !f.IsZero() {
		// Every hash of Go's crypto marshals its state; unmarshalling
		// what the same kind marshalled cannot fail.
		_ = h.(encoding.BinaryUnmarshaler).UnmarshalBinary([]byte(f.Str()))
	}
	return h
}

// build is a digest object of a class, holding a hash's state; nil is the
// state before anything was fed.
func build(cls *object.Class, bits int, h hash.Hash) object.Value {
	fields := make([]object.Value, len(cls.Fields))
	var state []byte
	if h != nil {
		state, _ = h.(encoding.BinaryMarshaler).MarshalBinary()
	}
	fields[fState] = object.Bytes(state)
	fields[fBits] = object.Int(int64(bits))
	return object.InstanceValue(&object.Instance{Cls: cls, Fields: fields})
}

func sum(a algorithm, d []byte) []byte {
	h := a.fn()
	h.Write(d)
	return h.Sum(nil)
}

// algorithmFor is the algorithm of a class a class method is called on:
// SHA2's is SHA256, as Ruby's is, the bit length being new's argument
// alone.
func algorithmFor(ctx object.Context, cls *object.Class) (algorithm, error) {
	if cls.IsA(sha2Class) {
		a, _ := sha2Bits(256)
		return a, nil
	}
	for c := cls; c != nil; c = c.Super {
		if a, ok := algorithms[c]; ok {
			return a, nil
		}
	}
	return algorithm{}, ctx.Raise(object.NotImplementedErr, "%s is an abstract class", cls.QualifiedName())
}

func installClasses() {
	c := classClass
	c.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		cls := self.Cls()
		a, err := algorithmFor(ctx, cls)
		if err != nil {
			return object.Value{}, err
		}
		if !cls.IsA(sha2Class) {
			if len(args) > 0 {
				return object.Value{}, stdlib.ArgError(ctx, len(args), "0")
			}
			return build(cls, a.bits, nil), nil
		}
		// SHA2.new(384): the bit length.
		if len(args) > 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "0..1")
		}
		if len(args) == 1 {
			bits, err := stdlib.WantInt(ctx, args[0], "Integer")
			if err != nil {
				return object.Value{}, err
			}
			ok := false
			if a, ok = sha2Bits(int(bits)); !ok {
				return object.Value{}, ctx.Raise(object.ArgumentError, "unsupported bit length: %d", bits)
			}
		}
		return build(cls, a.bits, nil), nil
	})
	// hexdigest(str), digest(str) and base64digest(str) on the class.
	for _, e := range encoders {
		c.DefineSingleton(e.name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			a, err := algorithmFor(ctx, self.Cls())
			if err != nil {
				return object.Value{}, err
			}
			s, err := stdlib.WantString(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			return e.encode(sum(a, []byte(s))), nil
		})
	}
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
			if len(args) == 1 {
				s, err := stdlib.WantString(ctx, args[0])
				if err != nil {
					return object.Value{}, err
				}
				return encode(sum(algorithmOf(self), []byte(s))), nil
			}
			return encode(hashOf(self).Sum(nil)), nil
		})
	}
	for _, e := range encoders {
		instFn(e.name, e.encode)
	}
	instFn("to_s", encoders[0].encode)
	feed := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		inst := self.Instance()
		bits, _ := inst.Fields[fBits].AsInt()
		h := hashOf(self)
		h.Write([]byte(s))
		return build(inst.Cls, int(bits), h), nil
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
	// name is OpenSSL::Digest's: the algorithm's name, "SHA256".
	c.Define("name", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewString(algorithmOf(self).name, object.UTF8), nil
	})
	c.Define("inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewString("#<"+self.Instance().Cls.QualifiedName()+": "+hex.EncodeToString(hashOf(self).Sum(nil))+">", object.UTF8), nil
	})
	// == takes another digest object, or the hex string of a digest.
	c.Define("==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		mine := hex.EncodeToString(hashOf(self).Sum(nil))
		if other, ok := args[0].AsInstance(); ok && other.Cls.IsA(classClass) {
			return object.Bool(mine == hex.EncodeToString(hashOf(args[0]).Sum(nil))), nil
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
		return object.NewString(hex.EncodeToString([]byte(s)), object.ASCII), nil
	})
	// Digest(:SHA256) is the class.
	object.DefineFunction("Digest", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
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

// named is the algorithm an OpenSSL method is given: its name, "sha256",
// a digest class, or a digest object, as scripts pass
// OpenSSL::Digest.new("sha256") to OpenSSL::HMAC.
func named(ctx object.Context, v object.Value) (*object.Class, algorithm, error) {
	if cls, ok := v.AsClass(); ok && cls.IsA(classClass) {
		a, err := algorithmFor(ctx, cls)
		return cls, a, err
	}
	if inst, ok := v.AsInstance(); ok && inst.Cls.IsA(classClass) {
		return inst.Cls, algorithmOf(v), nil
	}
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

// installOpenSSL defines OpenSSL::HMAC and OpenSSL::Digest, the names
// scripts written for openssl use for the same algorithms.
func installOpenSSL() {
	openssl := stdlib.Module("OpenSSL")
	for _, c := range []*object.Class{opensslDigest, hmacClass} {
		c.Lexical = openssl
		openssl.Consts[c.Name] = object.ClassValue(c)
	}
	for _, a := range known {
		opensslDigest.Consts[a.name] = digestModule.Consts[a.name]
	}
	// OpenSSL::Digest.new("SHA256") is a digest object of that algorithm,
	// fed the second argument when there is one.
	opensslDigest.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		cls, a, err := named(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		var h hash.Hash
		if len(args) == 2 {
			s, err := stdlib.WantString(ctx, args[1])
			if err != nil {
				return object.Value{}, err
			}
			h = a.fn()
			h.Write([]byte(s))
		}
		return build(cls, a.bits, h), nil
	})
	for _, e := range encoders {
		// OpenSSL::Digest.hexdigest(algorithm, data) and kin.
		opensslDigest.DefineSingleton(e.name, 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			_, a, err := named(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			s, err := stdlib.WantString(ctx, args[1])
			if err != nil {
				return object.Value{}, err
			}
			return e.encode(sum(a, []byte(s))), nil
		})
		// OpenSSL::HMAC.hexdigest(algorithm, key, data) and kin.
		hmacClass.DefineSingleton(e.name, 3, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
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
			return e.encode(mac.Sum(nil)), nil
		})
	}
}
