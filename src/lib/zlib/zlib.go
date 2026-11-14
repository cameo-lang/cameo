// Package zlib is Ruby's zlib library, built into cameo: the checksums,
// Zlib::Deflate.deflate and Zlib::Inflate.inflate, Zlib.gzip and gunzip,
// the constants and the error classes, on Go's compress and hash
// packages.
//
// It follows zlib 3.2.1, the version Ruby 3.4 ships. What deflates here
// inflates in Ruby and the other way round, and the checksums, headers
// and error classes are zlib's; the compressed bytes themselves are Go's
// deflate's, not zlib's, so they are not identical to Ruby's byte for
// byte, and the message of a DataError for a corrupt body is Go's
// reader's, not zlib's. The streaming objects, Zlib::Deflate.new and
// Inflate.new, raise NotImplementedError; GzipReader and GzipWriter on
// an IO are left out, and gzip's strategy: is accepted and ignored,
// since Go's deflate has none.
package zlib

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/cameo-lang/cameo/src/str"
	"hash/adler32"
	"hash/crc32"
	"io"
	"strings"
	"time"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	module      = object.NewModule("Zlib")
	errorClass  = object.NewClass("Error", object.StandardError)
	dataError   = object.NewClass("DataError", errorClass)
	bufError    = object.NewClass("BufError", errorClass)
	streamError = object.NewClass("StreamError", errorClass)
	memError    = object.NewClass("MemError", errorClass)
	needDict    = object.NewClass("NeedDict", errorClass)
	versionErr  = object.NewClass("VersionError", errorClass)
	gzipFile    = object.NewClass("GzipFile", object.ObjectClass)
	gzipError   = object.NewClass("Error", errorClass)
	crcError    = object.NewClass("CRCError", gzipError)
	lengthError = object.NewClass("LengthError", gzipError)
	noFooter    = object.NewClass("NoFooter", gzipError)
	deflateCls  = object.NewClass("Deflate", object.ObjectClass)
	inflateCls  = object.NewClass("Inflate", object.ObjectClass)
)

func raise(cls *object.Class, msg string) error { return &object.Exception{Cls: cls, Message: msg} }

// Install defines Zlib. lib.Install calls it once.
func Install() {
	m := module
	object.DefineNative("Zlib", object.ClassValue(m))
	m.Consts["VERSION"] = object.NewString("3.2.1", str.ASCII)
	m.Consts["ZLIB_VERSION"] = object.NewString("1.3.1", str.ASCII)
	for name, v := range map[string]int64{
		"NO_COMPRESSION": 0, "BEST_SPEED": 1, "BEST_COMPRESSION": 9, "DEFAULT_COMPRESSION": -1,
		"FILTERED": 1, "HUFFMAN_ONLY": 2, "RLE": 3, "FIXED": 4, "DEFAULT_STRATEGY": 0,
		"NO_FLUSH": 0, "PARTIAL_FLUSH": 1, "SYNC_FLUSH": 2, "FULL_FLUSH": 3, "FINISH": 4,
		"MAX_WBITS": 15, "MAX_MEM_LEVEL": 9, "DEF_MEM_LEVEL": 8,
		"BINARY": 0, "ASCII": 1, "TEXT": 1, "UNKNOWN": 2,
		"OS_CODE": 3, "OS_UNIX": 3, "OS_MSDOS": 0, "OS_WIN32": 11, "OS_MACOS": 7, "OS_UNKNOWN": 255,
	} {
		m.Consts[name] = object.Int(v)
	}
	for _, c := range []*object.Class{errorClass, dataError, bufError, streamError, memError, needDict, versionErr, gzipFile, deflateCls, inflateCls} {
		c.Lexical = m
		m.Consts[c.Name] = object.ClassValue(c)
	}
	for _, c := range []*object.Class{gzipError, crcError, lengthError, noFooter} {
		c.Lexical = gzipFile
		gzipFile.Consts[c.Name] = object.ClassValue(c)
	}
	// The streaming objects are left out: new says so, where Object's new
	// would make an object without their methods.
	for _, c := range []*object.Class{deflateCls, inflateCls} {
		name := c.Name
		c.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Value{}, ctx.Raise(object.NotImplementedErr, "Zlib::%s.new is not supported: a stream fed piece by piece; use Zlib::%s.%s on the whole string", name, name, strings.ToLower(name))
		})
	}
	m.DefineSingleton("zlib_version", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewString("1.3.1", str.ASCII), nil
	})

	// crc32(string = "", crc = 0) and adler32(string = "", adler = 1).
	checksum := func(name string, initial uint32, update func(sum uint32, data []byte) uint32) {
		m.DefineSingleton(name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) > 2 {
				return object.Value{}, stdlib.ArgError(ctx, len(args), "0..2")
			}
			sum := initial
			if len(args) == 2 && !args[1].IsNil() {
				n, err := stdlib.WantInt(ctx, args[1], "")
				if err != nil {
					return object.Value{}, err
				}
				sum = uint32(n)
			}
			if len(args) == 0 || args[0].IsNil() {
				return object.Int(int64(sum)), nil
			}
			s, err := stdlib.WantString(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			return object.Int(int64(update(sum, []byte(s)))), nil
		})
	}
	checksum("crc32", 0, func(sum uint32, data []byte) uint32 { return crc32.Update(sum, crc32.IEEETable, data) })
	checksum("adler32", 1, adler32Update)
	m.DefineSingleton("crc_table", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		out := make([]object.Value, 256)
		for i, v := range crc32.IEEETable {
			out[i] = object.Int(int64(v))
		}
		return object.NewList(out...), nil
	})

	// level reads a compression level: -1 and 0 to 9, or Ruby's
	// StreamError.
	level := func(ctx object.Context, v object.Value) (int, error) {
		if v.IsZero() || v.IsNil() {
			return flate.DefaultCompression, nil
		}
		n, err := stdlib.WantInt(ctx, v, "")
		if err != nil {
			return 0, err
		}
		if n < -1 || n > 9 {
			return 0, raise(streamError, "stream error")
		}
		return int(n), nil
	}
	deflate := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		lvl := object.Value{}
		if len(args) == 2 {
			lvl = args[1]
		}
		n, err := level(ctx, lvl)
		if err != nil {
			return object.Value{}, err
		}
		var b bytes.Buffer
		w, _ := zlib.NewWriterLevel(&b, n)
		w.Write([]byte(s))
		w.Close()
		return object.Bytes(b.Bytes()), nil
	}
	inflate := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		out, err := inflateZlib([]byte(s))
		if err != nil {
			return object.Value{}, err
		}
		return object.Bytes(out), nil
	}
	m.DefineSingleton("deflate", -1, deflate)
	m.DefineSingleton("inflate", 1, inflate)
	deflateCls.DefineSingleton("deflate", -1, deflate)
	inflateCls.DefineSingleton("inflate", 1, inflate)

	m.DefineSingleton("gzip", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
		}
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		lvl := object.Value{}
		if kw != nil {
			for _, k := range kw.Keys() {
				if name, _ := k.AsSymbol(); name != "level" && name != "strategy" {
					shown, _ := ctx.Inspect(k)
					return object.Value{}, ctx.Raise(object.ArgumentError, "unknown keyword: %s", shown)
				}
			}
			if v, ok := kw.Get(object.Symbol("level")); ok {
				lvl = v
			}
		}
		n, err := level(ctx, lvl)
		if err != nil {
			return object.Value{}, err
		}
		return object.Bytes(gzipBytes([]byte(s), n)), nil
	})
	m.DefineSingleton("gunzip", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		out, err := gunzipBytes([]byte(s))
		if err != nil {
			return object.Value{}, err
		}
		return object.Bytes(out), nil
	})
}

// adler32Update continues an Adler-32 sum over data: hash/adler32's for
// a fresh sum, and the same loop, reduced every 5552 bytes as zlib
// reduces it, for a continued one.
func adler32Update(sum uint32, data []byte) uint32 {
	if sum == 1 {
		return adler32.Checksum(data)
	}
	const mod, nmax = 65521, 5552
	a, b := sum&0xffff, sum>>16
	for len(data) > 0 {
		chunk := data[:min(len(data), nmax)]
		data = data[len(chunk):]
		for _, c := range chunk {
			a += uint32(c)
			b += a
		}
		a, b = a%mod, b%mod
	}
	return b<<16 | a
}

// inflateZlib is Zlib::Inflate.inflate: the zlib stream's data, with
// zlib's errors; what follows the stream is ignored.
func inflateZlib(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, raise(bufError, "buffer error")
	}
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, zlibError(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, zlibError(err)
	}
	return out, nil
}

// zlibError is the Zlib error for what Go's reader refuses.
func zlibError(err error) error {
	switch {
	case errors.Is(err, zlib.ErrDictionary):
		return raise(needDict, "need dictionary")
	case errors.Is(err, zlib.ErrHeader):
		return raise(dataError, "incorrect header check")
	case errors.Is(err, zlib.ErrChecksum):
		return raise(dataError, "incorrect data check")
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return raise(bufError, "buffer error")
	}
	// A corrupt body: Go's reader says where, zlib would say what.
	return raise(dataError, strings.TrimPrefix(err.Error(), "flate: "))
}

// gzipBytes is Zlib.gzip: Go's gzip writer, with the header Ruby writes:
// the time, no name, and Unix as the system.
func gzipBytes(data []byte, lvl int) []byte {
	var b bytes.Buffer
	w, _ := gzip.NewWriterLevel(&b, lvl)
	w.ModTime = time.Now()
	w.OS = 3
	w.Write(data)
	w.Close()
	return b.Bytes()
}

// gunzipBytes is Zlib.gunzip, with Ruby's errors for what is not a gzip
// member, is cut short, or fails its checks: a fixed read the string
// cannot fill is "unexpected end of string", a deflate body cut short
// "unexpected end of file", and a member with no footer at all NoFooter.
func gunzipBytes(data []byte) ([]byte, error) {
	short := raise(gzipError, "unexpected end of string")
	if len(data) < 10 {
		return nil, short
	}
	if data[0] != 0x1f || data[1] != 0x8b {
		return nil, raise(gzipError, "not in gzip format")
	}
	if data[2] != 8 {
		return nil, raise(gzipError, fmt.Sprintf("unsupported compression method %d", data[2]))
	}
	flags := data[3]
	switch {
	case flags&0x02 != 0:
		return nil, raise(gzipError, "multi-part gzip file is not supported")
	case flags&0x20 != 0:
		return nil, raise(gzipError, "encrypted gzip file is not supported")
	case flags&0xc0 != 0:
		// Ruby prints the flags as a signed byte, so 0x80 is 0xffffff80.
		return nil, raise(gzipError, fmt.Sprintf("unknown flags 0x%x", uint32(int32(int8(flags)))))
	}
	pos := 10
	if flags&4 != 0 { // FEXTRA
		if pos+2 > len(data) {
			return nil, short
		}
		pos += 2 + int(binary.LittleEndian.Uint16(data[pos:]))
		if pos > len(data) {
			return nil, short
		}
	}
	for _, flag := range []byte{8, 16} { // FNAME, FCOMMENT
		if flags&flag != 0 {
			end := bytes.IndexByte(data[pos:], 0)
			if end < 0 {
				return nil, short
			}
			pos += end + 1
		}
	}
	body := bytes.NewReader(data[pos:])
	r := flate.NewReader(body)
	out, err := io.ReadAll(r)
	if err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, raise(gzipError, "unexpected end of file")
		}
		return nil, zlibError(err)
	}
	rest := data[len(data)-body.Len():]
	switch {
	case len(rest) == 0:
		return nil, raise(noFooter, "footer is not found")
	case len(rest) < 8:
		return nil, short
	}
	if binary.LittleEndian.Uint32(rest[0:4]) != crc32.ChecksumIEEE(out) {
		return nil, raise(crcError, "invalid compressed data -- crc error")
	}
	if binary.LittleEndian.Uint32(rest[4:8]) != uint32(len(out)) {
		return nil, raise(lengthError, "invalid compressed data -- length error")
	}
	return out, nil
}
