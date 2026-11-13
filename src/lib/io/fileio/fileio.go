// Package fileio is Cameo::IO::FileIO, the module File extends for
// File.open, File.read and File.write: reading and writing a file in the
// file system the host gave the program. Cameo::IO is the module cameo's I/O lives in.
//
// The module's methods are there only in a program that allows file I/O
// and has a file system: the cameo command with --io or --io=file, which
// gives the script the machine's files, or a program that embeds cameo
// and calls Program.AllowIO("file") and Program.FileSystem with an
// afero.Fs. Anywhere else
// File.read is an undefined method, respond_to?(:read) is false and
// File.methods leaves it out (object.Class.IOKind).
//
// File.write writes a whole file; File.open and File.read only read, and
// a mode that writes raises ArgumentError. A host that wants a script to
// read only gives it an afero.NewReadOnlyFs, where File.write raises
// Errno::EACCES. A relative path starts at the directory the host named,
// an absolute one at the root of the file system, and ".." stops at the
// root. A path is a path and nothing else: "|cmd" names a file called
// "|cmd", as in Ruby 3.4's File.read; no form runs a command, unlike the
// open and IO.read of older Rubies.
package fileio

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"

	"github.com/cameo-lang/cameo/src/lib/stringio"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	// ioModule is Cameo::IO, where cameo's I/O lives.
	ioModule = object.NewModule("IO")
	// fileIO is Cameo::IO::FileIO, which File extends.
	fileIO = object.NewModule("FileIO")
)

// errnos are the Errno classes reading a file raises, with their numbers.
var errnos = map[string]int64{"ENOENT": 2, "EACCES": 13, "ENOTDIR": 20, "EISDIR": 21, "EROFS": 30}

// errno returns the class Errno::name, defining it the first time.
func errno(name string) *object.Class {
	if v, ok := object.ErrnoModule.Consts[name]; ok {
		c, _ := v.AsClass()
		return c
	}
	c := object.NewClass(name, object.SystemCallError)
	c.Lexical = object.ErrnoModule
	c.Consts["Errno"] = object.Int(errnos[name])
	object.ErrnoModule.Consts[name] = object.ClassValue(c)
	return c
}

// Install defines Cameo::IO, Cameo::IO::FileIO and the methods of a File
// handle, and makes File extend FileIO. lib.Install calls it once, after
// stringio's.
func Install() {
	ioModule.Lexical = object.CameoModule
	object.CameoModule.Consts["IO"] = object.ClassValue(ioModule)
	fileIO.Lexical = ioModule
	fileIO.IOKind = "file"
	ioModule.Consts["FileIO"] = object.ClassValue(fileIO)
	for name := range errnos {
		errno(name)
	}
	file, _ := object.ObjectClass.Consts["File"].AsClass()
	file.Extend(fileIO)
	stringio.DefineReading(file)
	file.Define("path", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		p, _ := stringio.ReaderInfo(self)
		return object.String(p), nil
	})
	file.Define("to_path", 0, file.Methods["path"].Native)
	file.Define("inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		p, closed := stringio.ReaderInfo(self)
		if closed {
			return object.String("#<File:" + p + " (closed)>"), nil
		}
		return object.String("#<File:" + p + ">"), nil
	})

	// open(path, mode = "r", **opts) returns a handle reading the file,
	// or with a block yields it, closes it and returns the block's value.
	fileIO.Define("open", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 3 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..3")
		}
		name, err := pathArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		mode := object.Nil
		if len(args) > 1 {
			mode = args[1]
		}
		binary, err := readMode(ctx, name, mode, kw)
		if err != nil {
			return object.Value{}, err
		}
		data, err := readFile(ctx, name)
		if err != nil {
			return object.Value{}, err
		}
		h := stringio.NewReader(self.Cls(), name, data, stringKind(binary))
		if block.IsZero() {
			return h, nil
		}
		v, err := ctx.Yield(block, h)
		if _, cerr := ctx.Call(h, "close"); err == nil && cerr != nil {
			err = cerr
		}
		return v, err
	})

	// read(path, length = nil, offset = nil, **opts) returns the file's
	// contents, or length bytes from offset, as Ruby's File.read: a String
	// read with a length is binary, as Ruby's is, and nil past the end.
	fileIO.Define("read", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 3 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..3")
		}
		name, err := pathArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		length, offset := int64(-1), int64(0)
		if len(args) > 1 && !args[1].IsNil() {
			if length, err = stdlib.WantInt(ctx, args[1], ""); err != nil {
				return object.Value{}, err
			}
			if length < 0 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "negative length %d given", length)
			}
		}
		if len(args) > 2 && !args[2].IsNil() {
			if offset, err = stdlib.WantInt(ctx, args[2], ""); err != nil {
				return object.Value{}, err
			}
			if offset < 0 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "negative offset %d given", offset)
			}
		}
		binary, err := readMode(ctx, name, object.Nil, kw)
		if err != nil {
			return object.Value{}, err
		}
		data, err := readFile(ctx, name)
		if err != nil {
			return object.Value{}, err
		}
		if offset > int64(len(data)) {
			offset = int64(len(data))
		}
		data = data[offset:]
		if length >= 0 {
			if length > 0 && len(data) == 0 {
				return object.Nil, nil
			}
			data = data[:min(int64(len(data)), length)]
			binary = true
		}
		if binary {
			return object.Bytes(data), nil
		}
		// Counting is little beside the reading.
		return object.Counted(string(data)), nil
	})

	// write(path, data) and write(path) { data } write data, a String or
	// what its to_s returns, to the file, creating it or replacing what it
	// held, and return the String written, where Ruby's returns its length.
	fileIO.Define("write", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		if kw != nil && kw.Len() > 0 {
			k, _ := ctx.Inspect(kw.Keys()[0])
			return object.Value{}, ctx.Raise(object.ArgumentError, "unknown keyword: %s", k)
		}
		name, err := pathArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		var data object.Value
		switch {
		case len(args) == 2 && !block.IsZero():
			return object.Value{}, ctx.Raise(object.ArgumentError, "File.write takes the data or a block that returns it, not both")
		case len(args) == 2:
			data = args[1]
		case !block.IsZero():
			if data, err = ctx.Yield(block); err != nil {
				return object.Value{}, err
			}
		default:
			return object.Value{}, ctx.Raise(object.ArgumentError, "File.write takes the data to write, or a block that returns it")
		}
		if !data.IsString() {
			s, err := ctx.ToS(data)
			if err != nil {
				return object.Value{}, err
			}
			data = object.String(s)
		}
		if err := writeFile(ctx, name, []byte(data.Str())); err != nil {
			return object.Value{}, err
		}
		return data, nil
	})
}

// pathArg returns the path v names, a String or a Pathname, refusing a
// NUL as Ruby does.
func pathArg(ctx object.Context, v object.Value) (string, error) {
	name, ok := stdlib.PathnameOf(v)
	if !ok {
		var err error
		if name, err = stdlib.WantString(ctx, v); err != nil {
			return "", err
		}
	}
	if strings.ContainsRune(name, 0) {
		return "", ctx.Raise(object.ArgumentError, "path name contains null byte")
	}
	return name, nil
}

// readMode returns whether the mode, the positional one or mode:, or
// encoding: or binmode:, reads binary. A mode that writes raises
// ArgumentError, since File.open and File.read only read, as does one Ruby
// does not know. Other options are ignored, as Ruby's File.read
// ignores them.
func readMode(ctx object.Context, name string, mode object.Value, kw *object.Map) (binary bool, err error) {
	if kw != nil {
		if v, ok := kw.Get(object.Symbol("mode")); ok && mode.IsNil() {
			mode = v
		}
		if v, ok := kw.Get(object.Symbol("binmode")); ok && v.Truthy() {
			binary = true
		}
		if v, ok := kw.Get(object.Symbol("encoding")); ok && !v.IsNil() {
			enc, err := stdlib.WantString(ctx, v)
			if err != nil {
				return false, err
			}
			binary = binary || isBinary(enc)
		}
	}
	if mode.IsNil() {
		return binary, nil
	}
	m, err := stdlib.WantString(ctx, mode)
	if err != nil {
		return false, err
	}
	access, enc, _ := strings.Cut(m, ":")
	if isBinary(strings.SplitN(enc, ":", 2)[0]) {
		binary = true
	}
	switch strings.TrimRight(access, "bt") {
	case "r":
		return binary || strings.Contains(access, "b"), nil
	case "w", "a", "r+", "w+", "a+", "wx", "w+x":
		return false, ctx.Raise(object.ArgumentError, "File.open and File.read only read, not in mode %s; File.write writes a file", m)
	}
	return false, ctx.Raise(object.ArgumentError, "invalid access mode %s", m)
}

func isBinary(enc string) bool {
	switch strings.ToUpper(enc) {
	case "BINARY", "ASCII-8BIT":
		return true
	}
	return false
}

func stringKind(binary bool) object.Kind {
	if binary {
		return object.KindBytes
	}
	return object.KindString
}

// readFile reads the file name names from the host's file system, with
// Ruby's Errno for what fails.
func readFile(ctx object.Context, name string) ([]byte, error) {
	fsys, dir := ctx.FileSystem()
	if fsys == nil || !ctx.IOAllowed("file") {
		// File's methods are gone without both; a method kept from before
		// cannot reach here, but a library calling in can.
		return nil, ctx.Raise(object.IOError, "the host allows no file I/O")
	}
	p := Resolve(dir, name)
	info, err := fsys.Stat(p)
	if err == nil && info.IsDir() {
		return nil, ctx.Raise(errno("EISDIR"), "Is a directory @ io_fread - %s", name)
	}
	var data []byte
	if err == nil {
		data, err = afero.ReadFile(fsys, p)
	}
	switch {
	case err == nil:
		return data, nil
	case errors.Is(err, fs.ErrPermission):
		return nil, ctx.Raise(errno("EACCES"), "Permission denied @ rb_sysopen - %s", name)
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, fs.ErrInvalid):
		return nil, ctx.Raise(errno("ENOENT"), "No such file or directory @ rb_sysopen - %s", name)
	}
	if strings.Contains(err.Error(), "not a directory") {
		return nil, ctx.Raise(errno("ENOTDIR"), "Not a directory @ rb_sysopen - %s", name)
	}
	return nil, ctx.Raise(object.IOError, "%s: %v", name, err)
}

// writeFile writes data to the file name names in the host's file system,
// with Ruby's Errno for what fails.
func writeFile(ctx object.Context, name string, data []byte) error {
	fsys, dir := ctx.FileSystem()
	if fsys == nil || !ctx.IOAllowed("file") {
		return ctx.Raise(object.IOError, "the host allows no file I/O")
	}
	p := Resolve(dir, name)
	if info, err := fsys.Stat(p); err == nil && info.IsDir() {
		return ctx.Raise(errno("EISDIR"), "Is a directory @ rb_sysopen - %s", name)
	}
	err := afero.WriteFile(fsys, p, data, 0o644)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, fs.ErrPermission):
		return ctx.Raise(errno("EACCES"), "Permission denied @ rb_sysopen - %s", name)
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, fs.ErrInvalid):
		return ctx.Raise(errno("ENOENT"), "No such file or directory @ rb_sysopen - %s", name)
	}
	if strings.Contains(err.Error(), "not a directory") {
		return ctx.Raise(errno("ENOTDIR"), "Not a directory @ rb_sysopen - %s", name)
	}
	return ctx.Raise(object.IOError, "%s: %v", name, err)
}

// Resolve returns the absolute path in the file system that name names:
// a relative name from dir, an absolute one as it is, with ".." that
// would climb above the root stopping there.
func Resolve(dir, name string) string {
	if filepath.IsAbs(name) {
		return filepath.Clean(name)
	}
	return filepath.Join(string(filepath.Separator), dir, name)
}
