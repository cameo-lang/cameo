// Package rbconfig is Ruby's rbconfig library, built into cameo:
// RbConfig::CONFIG with what a script looks at before it leaps, the
// version parts, the names, the host and the directories of the running
// cameo, RbConfig.ruby and RbConfig.expand.
//
// The values describe cameo: ruby_install_name is "cameo", the version
// parts are the Ruby cameo follows, the host is Go's GOOS and GOARCH in
// Ruby's spelling, and the directories are the running binary's. The
// build details of a C Ruby, the compiler and its flags, are left out;
// CONFIG has what is true of cameo.
package rbconfig

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/cameo-lang/cameo/src/buildinfo"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var module = object.NewModule("RbConfig")

// config is RbConfig::CONFIG, in the order its keys are listed.
func config() ([]string, map[string]string) {
	info := buildinfo.Read()
	parts := strings.SplitN(buildinfo.RubyVersion, ".", 3)
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	hostOS, cpu, vendor := runtime.GOOS, runtime.GOARCH, "pc"
	exe, dlext := "", "so"
	switch runtime.GOOS {
	case "darwin":
		vendor, dlext = "apple", "bundle"
	case "windows":
		hostOS, exe, dlext = "mingw32", ".exe", "dll"
	}
	switch runtime.GOARCH {
	case "amd64":
		cpu = "x86_64"
	case "386":
		cpu = "i386"
	case "arm64":
		if runtime.GOOS != "darwin" {
			cpu = "aarch64"
		}
	}
	bindir, prefix := "", ""
	if path, err := os.Executable(); err == nil {
		bindir = filepath.Dir(path)
		prefix = filepath.Dir(bindir)
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
	}
	keys := []string{"MAJOR", "MINOR", "TEENY", "PATCHLEVEL", "ruby_version", "RUBY_PROGRAM_VERSION", "RUBY_API_VERSION",
		"ruby_install_name", "RUBY_INSTALL_NAME", "RUBY_BASE_NAME", "RUBY_SO_NAME", "RUBY_ENGINE", "RUBY_ENGINE_VERSION",
		"host_os", "target_os", "build_os", "host_cpu", "target_cpu", "build_cpu", "host_vendor", "host", "target", "build", "arch",
		"EXEEXT", "DLEXT", "LIBEXT", "OBJEXT", "SHELL", "PATH_SEPARATOR",
		"prefix", "exec_prefix", "bindir", "libdir", "rubylibprefix", "rubylibdir", "sitelibdir", "datadir", "mandir"}
	values := map[string]string{
		"MAJOR": parts[0], "MINOR": parts[1], "TEENY": parts[2], "PATCHLEVEL": "0",
		"ruby_version": parts[0] + "." + parts[1] + ".0", "RUBY_PROGRAM_VERSION": buildinfo.RubyVersion, "RUBY_API_VERSION": parts[0] + "." + parts[1],
		"ruby_install_name": "cameo", "RUBY_INSTALL_NAME": "cameo", "RUBY_BASE_NAME": "cameo", "RUBY_SO_NAME": "cameo",
		"RUBY_ENGINE": "cameo", "RUBY_ENGINE_VERSION": info.Version,
		"host_os": hostOS, "target_os": hostOS, "build_os": hostOS, "host_cpu": cpu, "target_cpu": cpu, "build_cpu": cpu,
		"host_vendor": vendor, "host": cpu + "-" + vendor + "-" + hostOS, "target": cpu + "-" + vendor + "-" + hostOS, "build": cpu + "-" + vendor + "-" + hostOS,
		"arch":   info.Platform,
		"EXEEXT": exe, "DLEXT": dlext, "LIBEXT": "a", "OBJEXT": "o", "SHELL": shell, "PATH_SEPARATOR": sep,
		"prefix": prefix, "exec_prefix": prefix, "bindir": bindir, "libdir": filepath.Join(prefix, "lib"),
		"rubylibprefix": filepath.Join(prefix, "lib", "cameo"), "rubylibdir": filepath.Join(prefix, "lib", "cameo", parts[0]+"."+parts[1]+".0"),
		"sitelibdir": filepath.Join(prefix, "lib", "cameo", "site_ruby", parts[0]+"."+parts[1]+".0"),
		"datadir":    filepath.Join(prefix, "share"), "mandir": filepath.Join(prefix, "share", "man"),
	}
	return keys, values
}

var variable = regexp.MustCompile(`\$\(([^()]+)\)|\$\{([^{}]+)\}`)

// expand is RbConfig.expand: $(name) and ${name} replaced from the
// config, repeatedly, an unknown name by nothing.
func expand(s string, values map[string]string) string {
	for i := 0; i < 16 && strings.Contains(s, "$"); i++ {
		next := variable.ReplaceAllStringFunc(s, func(m string) string {
			name := strings.Trim(m[1:], "(){}")
			return values[name]
		})
		if next == s {
			break
		}
		s = next
	}
	return s
}

// Install defines RbConfig. lib.Install calls it once.
func Install() {
	m := module
	object.ObjectClass.Consts["RbConfig"] = object.ClassValue(m)
	keys, values := config()
	hash := object.NewMap()
	for _, k := range keys {
		hash = hash.Put(object.String(k), object.String(values[k]))
	}
	m.Consts["CONFIG"] = object.MapValue(hash)
	m.Consts["MAKEFILE_CONFIG"] = object.MapValue(hash)
	m.Consts["TOPDIR"] = object.String(values["prefix"])
	m.Consts["DESTDIR"] = object.String("")
	m.DefineSingleton("ruby", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		path, err := os.Executable()
		if err != nil {
			path = filepath.Join(values["bindir"], "cameo"+values["EXEEXT"])
		}
		return object.String(path), nil
	})
	m.DefineSingleton("expand", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		table := values
		if len(args) == 2 {
			given, ok := args[1].AsMap()
			if !ok {
				return object.Value{}, ctx.Raise(object.TypeError, "expand takes a Hash of the configuration")
			}
			table = map[string]string{}
			for i, k := range given.Keys() {
				ks, _ := ctx.ToS(k)
				vs, _ := ctx.ToS(given.Values()[i])
				table[ks] = vs
			}
		}
		return object.String(expand(s, table)), nil
	})
}
