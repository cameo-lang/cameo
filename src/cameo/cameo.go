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
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/afero"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/eval"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
	"github.com/cameo-lang/cameo/src/readline"
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

// ExitStatus returns the status a script ended with by exit or abort, or
// by raising SystemExit itself, and false when err is anything else.
func ExitStatus(err error) (int, bool) {
	var exc *object.Exception
	if !errors.As(err, &exc) || !exc.Cls.IsA(object.SystemExit) {
		return 0, false
	}
	status, ok := exc.Attr("status")
	if !ok || status.Kind() != object.KindInt {
		return 0, true
	}
	return int(status.Int()), true
}

// RunError is an exception that ended a script, with its message formatted
// for the terminal: "script.cameo:12: message (Class)".
type RunError struct {
	Err     error
	Message string
	proto   *protogen.Registry
}

func (e *RunError) Error() string { return e.Message }
func (e *RunError) Unwrap() error { return e.Err }

// Exception returns the exception that ended the script as the protobuf
// message its class is: its message, line and cause, what an exception of
// Ruby's core carries (a KeyError's key), and the fields the class
// declares, read by name with protoreflect. It returns nil when the script
// ended with an error that is not an exception.
func (e *RunError) Exception() proto.Message {
	var exc *object.Exception
	if e.proto == nil || !errors.As(e.Err, &exc) {
		return nil
	}
	msg, err := e.proto.ExceptionMessage(exc)
	if err != nil {
		return nil
	}
	return msg
}

// Program is a loaded script: parsed and checked, with an interpreter that
// runs it and keeps its classes afterwards.
type Program struct {
	file string
	src  string
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
	p := &Program{file: file, src: src, prog: prog, in: eval.New(stdout, stderr)}
	p.Args(nil)
	return p, nil
}

// Args sets ARGV, the script's arguments: what follows the script on the
// cameo command line, or what the host gives it. It is empty by default.
func (p *Program) Args(args []string) {
	elems := make([]object.Value, len(args))
	for i, a := range args {
		elems[i] = object.String(a)
	}
	p.in.SetConst("ARGV", object.NewList(elems...))
}

// IgnoreMutationErrors sets whether a call of one of Ruby's mutators,
// such as a.push(1), returns its result on a copy instead of raising. The
// receiver never changes either way.
func (p *Program) IgnoreMutationErrors(ignore bool) { p.in.IgnoreMutationErrors = ignore }

// Terminal gives the program e, which Cameo::Kernel::Readline.readline reads
// lines from; without one, readline returns nil. Call it before the program
// runs.
func (p *Program) Terminal(e *readline.Editor) {
	p.in.State(readline.StateName, func() any { return e })
}

// File is the script's path, as Load was given it.
func (p *Program) File() string { return p.file }

// Run evaluates the script once; its classes stay loaded for Call. It
// returns ErrPaused when Pause asked it to pause. Run on a program that
// paused goes on where it paused: a host that could not dump the script,
// or did not want to, loses nothing.
func (p *Program) Run() error {
	p.ran = true
	err := p.in.Run(p.file, p.prog)
	if err == nil || errors.Is(err, ErrPaused) {
		return err
	}
	return conv{p.in}.runError(err)
}

// runError wraps an error of the interpreter for a Go caller, with the
// message the cameo binary would print.
func (c conv) runError(err error) *RunError {
	return &RunError{Err: err, Message: c.in.FormatError(err), proto: c.in.Proto()}
}

// toValues converts the arguments of a call from Go.
func (c conv) toValues(args []any) ([]object.Value, error) {
	values := make([]object.Value, len(args))
	for i, a := range args {
		v, err := c.toValue(a)
		if err != nil {
			return nil, err
		}
		values[i] = v
	}
	return values, nil
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
	values, err := conv{p.in}.toValues(args)
	if err != nil {
		return nil, err
	}
	var out object.Value
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
		return nil, conv{p.in}.runError(err)
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
	files["cameo/value.proto"] = cameopb.ValueProto
	files["cameo/ir.proto"] = cameopb.IRProto
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
// value), for storing or sending it. A *Block is encoded as the Proc it
// came from, with the code it runs and the values it captured, so that
// another program can decode and call it; the classes it names must be
// loaded there.
func (p *Program) Encode(v any) ([]byte, error) {
	val, err := p.ToValue(v)
	if err != nil {
		return nil, err
	}
	return p.in.ProcEncoder(true).Encode(val)
}

// Decode reads an envelope back into a Go value: a Proc as a *Block.
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
// map[any]any, an object.Value as it is, a *Block as the Proc it came
// from, or a proto.Message whose descriptor full name is a
// loaded class's or an exception class of Ruby's, which becomes an
// instance or an exception.
func (p *Program) ToValue(v any) (object.Value, error) { return conv{p.in}.toValue(v) }

// ToGo converts a cameo value to a Go value: nil, bool, int64 (a *big.Int
// beyond 64 bits), float64,
// string, []byte, time.Time, []any, map[any]any (with a Symbol key as a
// string), a proto.Message for an instance (a value of the registered Go
// type for a native class, a dynamic message otherwise, either a copy) or
// an exception (a dynamic message, as RunError.Exception gives), a *Block
// for a Proc, which Block.Call runs,
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
	case *Block:
		if v == nil {
			// The block a host method got without one, given back.
			return object.Nil, nil
		}
		return v.v, nil
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
	case *Promise:
		return object.Int(v.ID()), nil
	case proto.Message:
		if cls := protogen.NativeClass(v.ProtoReflect().Descriptor().FullName()); cls != nil {
			inst, err := c.in.Decoder().InstanceOf(cls, v.ProtoReflect())
			if err != nil {
				return object.Value{}, err
			}
			return object.InstanceValue(inst), nil
		}
		full := string(v.ProtoReflect().Descriptor().FullName())
		for _, cls := range slices.Concat(c.in.Classes(), c.in.HostExceptions(), builtinExceptions()) {
			if protogen.Package(cls)+"."+cls.Name != full {
				continue
			}
			data, err := proto.Marshal(v)
			if err != nil {
				return object.Value{}, err
			}
			if cls.IsA(object.ExceptionClass) {
				exc, err := c.in.Decoder().UnmarshalException(cls, data)
				if err != nil {
					return object.Value{}, err
				}
				return object.ExceptionValue(exc), nil
			}
			inst, err := c.in.Decoder().Instance(cls, data)
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
	case object.KindProc:
		return &Block{c: c, v: v}, nil
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
	case object.KindException:
		return c.in.ProcEncoder(true).ExceptionMessage(v.Exception())
	case object.KindInstance:
		v := v.Instance()
		if v.Cls.Digest == "" {
			return v, nil
		}
		// The proc of a proc field goes to Go as a closure with its
		// code, which comes back as the proc.
		return c.in.ProcEncoder(true).GoMessage(v)
	}
	if v.IsNil() {
		return nil, nil
	}
	return v, nil
}

// builtinExceptions lists the exception classes of Ruby's core that cameo
// has, those inside a builtin module (JSON::ParserError) among them. They
// are fixed once the standard library is installed, which Load does
// before anything converts a value.
var builtinExceptions = sync.OnceValue(func() []*object.Class {
	registryMu.Lock()
	defer registryMu.Unlock()
	var out []*object.Class
	for _, v := range object.ObjectClass.Consts {
		cls, ok := v.AsClass()
		if !ok || cls.Digest != "" {
			continue
		}
		if cls.IsA(object.ExceptionClass) && !cls.IsModule {
			out = append(out, cls)
		}
		if cls == object.ObjectClass {
			// Object's constants are the top level this loop walks.
			continue
		}
		for _, inner := range cls.Consts {
			if c, ok := inner.AsClass(); ok && !c.IsModule && c.Digest == "" && c.IsA(object.ExceptionClass) {
				out = append(out, c)
			}
		}
	}
	return out
})

// AllowIO lets the script do I/O. Nothing is allowed unless the program
// says so: a program that embeds cameo is safe by default, and opts in
// to each kind of I/O apart. Each of what is a kind, "network", "file"
// or "popen", "all" for every kind, or a library's require path, which
// allows that one library: AllowIO("network") lets every library that
// reaches the network act, AllowIO("file") File.read and File.open, and
// AllowIO("all") any I/O. The
// cameo command allows what its --io flag names. Call it before Run.
func (p *Program) AllowIO(what ...string) { p.in.AllowIO(what...) }

// FileSystem gives the script fsys to read and write files in, through
// File.read, File.open and File.write, with a relative path starting at dir, an absolute path
// in fsys ("" for its root); an absolute path is fsys's own, and ".."
// stops at its root. The script reads and writes only where AllowIO allows
// file I/O too: without both, File has no read, open or write. A host that
// shares part of a disk confines the script there itself, with
// afero.NewBasePathFs, and keeps it from writing with afero.NewReadOnlyFs. Call it before
// Run.
func (p *Program) FileSystem(fsys afero.Fs, dir string) { p.in.SetFileSystem(fsys, dir) }

// AllowHostIO allows what AllowIO does, and gives file I/O, when what
// allows it, the machine's file system, with relative paths from the
// working directory: what the cameo command does with its --io flag. A
// program that embeds cameo calls
// AllowIO and FileSystem instead, with the files it means to share.
func (p *Program) AllowHostIO(what ...string) {
	p.AllowIO(what...)
	if !slices.Contains(what, "all") && !slices.Contains(what, "file") {
		return
	}
	dir, err := os.Getwd()
	if err != nil {
		dir = "/"
	}
	p.FileSystem(afero.NewOsFs(), dir)
}

// IOKinds are the kinds of I/O --io= and AllowIO name.
var IOKinds = []string{"network", "file", "popen"}

// ParseIOFlags reads the --io flags at the front of a command line, as the
// cameo command takes them: --io allows
// every kind of I/O, --io=network,file,popen the kinds named. It returns
// what AllowIO takes and the arguments left, or an error naming a kind it
// does not know.
func ParseIOFlags(args []string) (allow, rest []string, err error) {
	for len(args) > 0 {
		switch {
		case args[0] == "--io":
			allow = append(allow, "all")
		case strings.HasPrefix(args[0], "--io="):
			for _, kind := range strings.Split(strings.TrimPrefix(args[0], "--io="), ",") {
				if !slices.Contains(IOKinds, kind) {
					return nil, nil, fmt.Errorf("--io takes %s, not %q", strings.Join(IOKinds, ", "), kind)
				}
				allow = append(allow, kind)
			}
		default:
			return allow, args, nil
		}
		args = args[1:]
	}
	return allow, args, nil
}
