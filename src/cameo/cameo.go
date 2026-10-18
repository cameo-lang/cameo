// Package cameo is the host API: load and run cameo scripts from Go, call
// their methods, and move values across the boundary. The language is in
// doc/design/cameo.md.
package cameo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"reflect"
	"sort"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/cameo-lang/cameo/src/eval"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
	"github.com/cameo-lang/cameo/src/syntax"
)

// Run parses, checks and runs a script, writing its output to stdout and
// stderr. It returns a syntax.ErrorList when the script does not check, or
// a *RunError for the exception that ended it.
func Run(file, src string, stdout, stderr io.Writer) error {
	prog, err := Load(file, src, stdout, stderr)
	if err != nil {
		return err
	}
	return prog.Run()
}

// RunFile runs the script at path.
func RunFile(path string, stdout, stderr io.Writer) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return Run(path, string(src), stdout, stderr)
}

// RunError is an exception that ended a script, with its message formatted
// for the terminal: "script.cameo:12: message (Class)".
type RunError struct {
	Err     error
	Message string
}

func (e *RunError) Error() string { return e.Message }
func (e *RunError) Unwrap() error { return e.Err }

// Program is a loaded script: parsed and checked, with an interpreter that
// runs it and keeps its classes afterwards.
type Program struct {
	file string
	prog *syntax.Program
	in   *eval.Interp
	ran  bool
}

// Load parses and checks a script. The program's output goes to stdout and
// stderr when it runs.
func Load(file, src string, stdout, stderr io.Writer) (*Program, error) {
	loaded.Store(true)
	prog, err := syntax.Parse(file, src)
	if err != nil {
		return nil, err
	}
	return &Program{file: file, prog: prog, in: eval.New(stdout, stderr)}, nil
}

// Run evaluates the script once; its classes stay loaded for Call.
func (p *Program) Run() error {
	p.ran = true
	if err := p.in.Run(p.file, p.prog); err != nil {
		return &RunError{Err: err, Message: p.in.FormatError(err)}
	}
	return nil
}

// LastValue returns the script's last top-level expression as p shows it,
// once Run returned without an error: "3" for 1 + 2, "\"x\"" for "x". It
// returns "" for nil, and before the script ran.
func (p *Program) LastValue() (string, error) {
	v := p.in.Last()
	if v.IsNil() {
		return "", nil
	}
	return p.in.Inspect(v)
}

// RunContext is Run until ctx ends. Then the script, and every worker it
// started, stops at its next loop pass, method call or block call, no
// rescue catches it, and RunContext returns ctx's error.
func (p *Program) RunContext(ctx context.Context) error {
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			p.in.Stop()
		case <-done:
		}
	}()
	err := p.Run()
	if errors.Is(err, eval.ErrStopped) && ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// Call invokes method on recv with args, converting Go values in and the
// result out. recv may be a Go value, an object.Value, a class name
// (string, for a class method), a proto.Message of a loaded class, or nil
// for the script's top level and its own methods.
func (p *Program) Call(recv any, method string, args ...any) (any, error) {
	if !p.ran {
		if err := p.Run(); err != nil {
			return nil, err
		}
	}
	values := make([]object.Value, len(args))
	for i, a := range args {
		v, err := p.ToValue(a)
		if err != nil {
			return nil, err
		}
		values[i] = v
	}
	var out object.Value
	var err error
	if recv == nil {
		// A nil receiver is the script's top level, where its own methods
		// are called without a receiver.
		out, err = p.in.CallTop(method, values, nil, object.Value{})
	} else {
		r, convErr := p.ToValue(recv)
		if convErr != nil {
			return nil, convErr
		}
		if s, ok := recv.(string); ok {
			if cls := p.Class(s); cls != nil {
				r = object.ClassValue(cls)
			}
		}
		out, err = p.in.Call(r, method, values...)
	}
	if err != nil {
		return nil, &RunError{Err: err, Message: p.in.FormatError(err)}
	}
	return p.ToGo(out)
}

// Class finds a loaded or registered class by its qualified name
// (Geo::Point), nil when there is none.
func (p *Program) Class(name string) *object.Class {
	for _, cls := range p.in.Classes() {
		if cls.QualifiedName() == name {
			return cls
		}
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	return registered[name]
}

// Classes lists the loaded classes.
func (p *Program) Classes() []*object.Class { return p.in.Classes() }

// Proto is the .proto text of a loaded class.
func (p *Program) Proto(name string) (string, error) {
	cls := p.Class(name)
	if cls == nil {
		return "", fmt.Errorf("no class %s", name)
	}
	if _, err := p.in.Proto().Message(cls); err != nil {
		return "", err
	}
	return protogen.Text(cls), nil
}

// ProtoFiles renders every loaded class as .proto files, one per package,
// keyed by file name, plus the envelope in cameo/value.proto.
func (p *Program) ProtoFiles() map[string]string {
	files := protogen.Files(p.Classes())
	files["cameo/value.proto"] = "syntax = \"proto3\";\n\npackage cameo;\n\n" + protogen.EnvelopeText
	return files
}

// Descriptor is the message descriptor of a loaded class.
func (p *Program) Descriptor(name string) (protoreflect.MessageDescriptor, error) {
	cls := p.Class(name)
	if cls == nil {
		return nil, fmt.Errorf("no class %s", name)
	}
	return p.in.Proto().Message(cls)
}

// Encode is the envelope wire form of a value (an object.Value or a Go
// value), for storing or sending it.
func (p *Program) Encode(v any) ([]byte, error) {
	val, err := p.ToValue(v)
	if err != nil {
		return nil, err
	}
	return p.in.Encoder().Encode(val)
}

// Decode reads an envelope back into a Go value.
func (p *Program) Decode(data []byte) (any, error) {
	v, err := p.in.Decoder().Decode(data)
	if err != nil {
		return nil, err
	}
	return p.ToGo(v)
}

// ToValue converts a Go value to a cameo value: nil, bool, integers (a
// *big.Int among them),
// floats, string, []byte, time.Time, []any, map[string]any or
// map[any]any, an object.Value as it is, or a proto.Message whose descriptor full name is a
// loaded class's.
func (p *Program) ToValue(v any) (object.Value, error) { return conv{p.in}.toValue(v) }

// ToGo converts a cameo value to a Go value: nil, bool, int64 (a *big.Int
// beyond 64 bits), float64,
// string, []byte, time.Time, []any, map[any]any (with a Symbol key as a
// string), a proto.Message for an instance (a value of the registered Go
// type for a native class, a dynamic message otherwise, either a copy),
// and the object.Value itself for everything else (symbols, ranges,
// classes). A Hash with a key that is not comparable in Go, or with two
// keys that are one in Go, such as :a and "a", is an error.
func (p *Program) ToGo(v object.Value) (any, error) { return conv{p.in}.toGo(v) }

// conv converts values for one interpreter: a program's, or the one a
// host method runs in, which may be an async worker's.
type conv struct{ in *eval.Interp }

func (c conv) toValue(v any) (object.Value, error) {
	switch v := v.(type) {
	case nil:
		return object.Nil, nil
	case object.Value:
		return v, nil
	case bool:
		return object.Bool(v), nil
	case int:
		return object.Int(v), nil
	case int8:
		return object.Int(v), nil
	case int16:
		return object.Int(v), nil
	case int32:
		return object.Int(v), nil
	case int64:
		return object.Int(v), nil
	case uint:
		return object.BigInt(new(big.Int).SetUint64(uint64(v))), nil
	case uint8:
		return object.Int(v), nil
	case uint16:
		return object.Int(v), nil
	case uint32:
		return object.Int(v), nil
	case uint64:
		return object.BigInt(new(big.Int).SetUint64(v)), nil
	case *big.Int:
		return object.BigInt(new(big.Int).Set(v)), nil
	case float32:
		return object.Float(float64(v)), nil
	case float64:
		return object.Float(v), nil
	case string:
		return object.String(v), nil
	case []byte:
		return object.Bytes(v), nil
	case time.Time:
		return object.TimeValue(v.Round(0)), nil
	case []any:
		elems := make([]object.Value, len(v))
		for i, e := range v {
			val, err := c.toValue(e)
			if err != nil {
				return object.Value{}, err
			}
			elems[i] = val
		}
		return object.NewList(elems...), nil
	case []string:
		elems := make([]object.Value, len(v))
		for i, e := range v {
			elems[i] = object.String(e)
		}
		return object.NewList(elems...), nil
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b := object.NewMapBuilder()
		for _, k := range keys {
			val, err := c.toValue(v[k])
			if err != nil {
				return object.Value{}, err
			}
			b.Set(object.String(k), val)
		}
		return object.MapValue(b.Map()), nil
	case map[any]any:
		b := object.NewMapBuilder()
		for k, e := range v {
			key, err := c.toValue(k)
			if err != nil {
				return object.Value{}, err
			}
			val, err := c.toValue(e)
			if err != nil {
				return object.Value{}, err
			}
			b.Set(key, val)
		}
		return object.MapValue(b.Map()), nil
	case proto.Message:
		if cls := protogen.NativeClass(v.ProtoReflect().Descriptor().FullName()); cls != nil {
			inst, err := c.in.Proto().FromMessage(cls, v.ProtoReflect())
			if err != nil {
				return object.Value{}, err
			}
			return object.InstanceValue(inst), nil
		}
		full := string(v.ProtoReflect().Descriptor().FullName())
		for _, cls := range c.in.Classes() {
			if protogen.Package(cls)+"."+cls.Name != full {
				continue
			}
			data, err := proto.Marshal(v)
			if err != nil {
				return object.Value{}, err
			}
			inst, err := c.in.Proto().Unmarshal(cls, data)
			if err != nil {
				return object.Value{}, err
			}
			return object.InstanceValue(inst), nil
		}
		return object.Value{}, fmt.Errorf("no loaded class has the message %s", full)
	}
	return object.Value{}, fmt.Errorf("cannot convert %T to a cameo value", v)
}

func (c conv) toGo(v object.Value) (any, error) {
	switch v.Kind() {
	case object.KindBool:
		v := v.Bool()
		return bool(v), nil
	case object.KindInt:
		v := v.Int()
		return int64(v), nil
	case object.KindBig:
		return new(big.Int).Set(v.Big()), nil
	case object.KindFloat:
		v := v.Float()
		return float64(v), nil
	case object.KindString, object.KindASCII:
		v := v.Str()
		return string(v), nil
	case object.KindBytes:
		v := v.Bytes()
		return []byte(v), nil
	case object.KindList:
		v := v.List()
		out := make([]any, len(v.Elems))
		for i, e := range v.Elems {
			g, err := c.toGo(e)
			if err != nil {
				return nil, err
			}
			out[i] = g
		}
		return out, nil
	case object.KindSet:
		v := v.Set()
		out := make([]any, v.Len())
		for i, e := range v.Elems() {
			g, err := c.toGo(e)
			if err != nil {
				return nil, err
			}
			out[i] = g
		}
		return out, nil
	case object.KindMap:
		v := v.Map()
		out := make(map[any]any, v.Len())
		for i, k := range v.Keys() {
			// A Symbol key is a Go string, as a Symbol becomes a String wherever a
			// value leaves cameo.
			var key any
			if name, sym := k.AsSymbol(); sym {
				key = name
			} else {
				var err error
				if key, err = c.toGo(k); err != nil {
					return nil, err
				}
			}
			if key != nil && !reflect.TypeOf(key).Comparable() {
				return nil, fmt.Errorf("a Hash key of class %s is a %T in Go, which a Go map cannot hold as a key", k.Class().QualifiedName(), key)
			}
			if _, taken := out[key]; taken {
				return nil, fmt.Errorf("two keys of a Hash are both %#v in Go", key)
			}
			val, err := c.toGo(v.Values()[i])
			if err != nil {
				return nil, err
			}
			out[key] = val
		}
		return out, nil
	case object.KindTime:
		return v.Time(), nil
	case object.KindInstance:
		v := v.Instance()
		if v.Cls.Digest == "" {
			return v, nil
		}
		return c.in.Proto().GoMessage(v)
	}
	if v.IsNil() {
		return nil, nil
	}
	return v, nil
}
