// Command warmruby runs a Ruby script on CRuby 3.4 compiled to WebAssembly
// (WASI), on the wazero runtime: the Ruby a Go program can embed without
// cgo. It is the WASM Ruby cameo's benchmarks compare against.
//
//	warmruby [-time FILE] [-cache DIR] script.rb
//
// The module is the ruby+stdlib.wasm of the npm package @ruby/3.4-wasm-wasi,
// downloaded once into the cache directory and checked against its SHA-256.
// wazero compiles it to machine code once and keeps the result in the same
// directory, so a later run starts from the compiled module: that is what
// makes the Ruby warm. The script's directory is the working directory
// inside the sandbox; nothing else of the host is visible.
//
// With -time, the seconds the script itself ran are written to FILE: Ruby
// measures them around the script, so the VM's start is not in them.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// The pinned module.
const (
	packageURL  = "https://registry.npmjs.org/@ruby/3.4-wasm-wasi/-/3.4-wasm-wasi-2.10.1.tgz"
	packageFile = "package/dist/ruby+stdlib.wasm"
	moduleHash  = "4b814fc9d13505ea5b4c1f3bd3a7cc7ccca137f551196b9b21f0a7caf9cc5878"
	moduleName  = "ruby-3.4-wasi-2.10.1.wasm"
)

// program is the Ruby the command evaluates: it runs the script as Ruby's
// main script would run, in the script's directory, reports an exception
// as Ruby's top level does, without this program's own frames,
// writes the monotonic seconds the script took to /timing/seconds when that
// directory is mounted, and evaluates to the exit status.
const program = `
$0 = %q
Dir.chdir(File.dirname($0))
__warmruby_status = 0
__warmruby_start = Process.clock_gettime(Process::CLOCK_MONOTONIC)
begin
  load $0
rescue SystemExit => e
  __warmruby_status = e.status
rescue Exception => e
  e.set_backtrace(e.backtrace.reject { _1.start_with?("eval:") || _1.include?("Kernel.eval") })
  $stderr.puts e.full_message(highlight: false)
  __warmruby_status = 1
ensure
  __warmruby_seconds = Process.clock_gettime(Process::CLOCK_MONOTONIC) - __warmruby_start
  $stdout.flush
  $stderr.flush
  File.write("/timing/seconds", __warmruby_seconds.to_s) if File.directory?("/timing")
end
__warmruby_status.to_s
`

func main() {
	os.Exit(run())
}

func run() int {
	timeFile := flag.String("time", "", "write the seconds the script ran to this file")
	cacheDir := flag.String("cache", defaultCache(), "where the module and its compiled form are kept")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: warmruby [-time FILE] [-cache DIR] script.rb")
		return 2
	}
	script, err := filepath.Abs(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "warmruby:", err)
		return 1
	}
	code, err := runScript(context.Background(), *cacheDir, script, *timeFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "warmruby:", err)
		return 1
	}
	return code
}

func defaultCache() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "cameo-warmruby")
	}
	return filepath.Join(os.TempDir(), "cameo-warmruby")
}

// runScript runs script on the module and returns Ruby's exit status.
func runScript(ctx context.Context, cacheDir, script, timeFile string) (int, error) {
	wasm, err := module(ctx, cacheDir)
	if err != nil {
		return 0, err
	}
	compiled, err := wazero.NewCompilationCacheWithDir(filepath.Join(cacheDir, "compiled"))
	if err != nil {
		return 0, err
	}
	defer compiled.Close(ctx)
	// The largest memory WebAssembly's 32-bit addresses allow, 4 GiB.
	config := wazero.NewRuntimeConfig().WithCompilationCache(compiled).WithMemoryLimitPages(65536)
	rt := wazero.NewRuntimeWithConfig(ctx, config)
	defer rt.Close(ctx)
	wasi_snapshot_preview1.MustInstantiate(ctx, rt)
	mod, err := rt.CompileModule(ctx, wasm)
	if err != nil {
		return 0, fmt.Errorf("compiling the module: %w", err)
	}
	if err := stubHostModules(ctx, rt, mod); err != nil {
		return 0, err
	}

	dir := filepath.Dir(script)
	fs := wazero.NewFSConfig().WithReadOnlyDirMount(dir, "/work")
	var timing string
	if timeFile != "" {
		if timing, err = os.MkdirTemp("", "warmruby-timing-"); err != nil {
			return 0, err
		}
		defer os.RemoveAll(timing)
		fs = fs.WithDirMount(timing, "/timing")
	}
	modConfig := wazero.NewModuleConfig().
		WithName("ruby").
		WithStartFunctions("_initialize").
		WithArgs("ruby").
		WithEnv("HOME", "/work").
		WithFSConfig(fs).
		WithStdin(os.Stdin).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		WithSysWalltime().
		WithSysNanotime().
		WithRandSource(crand.Reader)
	inst, err := rt.InstantiateModule(ctx, mod, modConfig)
	if err != nil {
		return 0, fmt.Errorf("starting the module: %w", err)
	}
	vm := &rubyVM{ctx: ctx, mod: inst}
	if err := vm.init(); err != nil {
		return 0, err
	}
	status, err := vm.eval(strings.Replace(program, "%q", strconv.Quote("/work/"+filepath.Base(script)), 1))
	if err != nil {
		return 0, err
	}
	code, err := strconv.Atoi(status)
	if err != nil {
		return 0, fmt.Errorf("the script's exit status %q: %w", status, err)
	}
	if timing != "" {
		secs, err := os.ReadFile(filepath.Join(timing, "seconds"))
		if err == nil {
			err = os.WriteFile(timeFile, append(bytes.TrimSpace(secs), '\n'), 0o644)
		}
		if err != nil && code == 0 {
			return 0, fmt.Errorf("the script's time: %w", err)
		}
	}
	return code, nil
}

// module returns the pinned module from the cache directory, downloading it
// there first if it is missing.
func module(ctx context.Context, cacheDir string) ([]byte, error) {
	path := filepath.Join(cacheDir, moduleName)
	if wasm, err := os.ReadFile(path); err == nil && hashOf(wasm) == moduleHash {
		return wasm, nil
	}
	wasm, err := download(ctx)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	tmp := path + ".part"
	if err := os.WriteFile(tmp, wasm, 0o644); err != nil {
		return nil, err
	}
	return wasm, os.Rename(tmp, path)
}

// download fetches the npm package and returns the module inside it, which
// must have the pinned hash.
func download(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, packageURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", packageURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading %s: %s", packageURL, resp.Status)
	}
	unzipped, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, err
	}
	entries := tar.NewReader(unzipped)
	for {
		h, err := entries.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("%s holds no %s", packageURL, packageFile)
		}
		if err != nil {
			return nil, err
		}
		if strings.TrimPrefix(h.Name, "./") != packageFile {
			continue
		}
		wasm, err := io.ReadAll(entries)
		if err != nil {
			return nil, err
		}
		if got := hashOf(wasm); got != moduleHash {
			return nil, fmt.Errorf("%s in %s has SHA-256 %s, not %s", packageFile, packageURL, got, moduleHash)
		}
		return wasm, nil
	}
}

func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// stubHostModules satisfies the imports ruby.wasm has besides WASI. The
// module is built for browsers as well, and imports the host functions of
// its JS gem (rb-js-abi-host) and the resource table of the component model
// (canonical_abi). A script that does not require "js" never calls the
// former, so they trap; the resource table keeps a value per handle.
func stubHostModules(ctx context.Context, rt wazero.Runtime, mod wazero.CompiledModule) error {
	var handles []uint64
	builders := map[string]wazero.HostModuleBuilder{}
	for _, f := range mod.ImportedFunctions() {
		modName, name, _ := f.Import()
		if modName == wasi_snapshot_preview1.ModuleName {
			continue
		}
		b, ok := builders[modName]
		if !ok {
			b = rt.NewHostModuleBuilder(modName)
			builders[modName] = b
		}
		var fn api.GoModuleFunc
		switch {
		case strings.HasPrefix(name, "resource_new_"):
			fn = func(_ context.Context, _ api.Module, stack []uint64) {
				handles = append(handles, stack[0])
				stack[0] = uint64(len(handles) - 1)
			}
		case strings.HasPrefix(name, "resource_get_"):
			fn = func(_ context.Context, _ api.Module, stack []uint64) {
				stack[0] = handles[stack[0]]
			}
		case strings.HasPrefix(name, "resource_drop_"):
			fn = func(context.Context, api.Module, []uint64) {}
		default:
			name := name
			fn = func(context.Context, api.Module, []uint64) {
				panic(fmt.Sprintf("%s.%s: JavaScript is not available outside a browser", modName, name))
			}
		}
		b.NewFunctionBuilder().WithGoModuleFunction(fn, f.ParamTypes(), f.ResultTypes()).Export(name)
	}
	for name, b := range builders {
		if _, err := b.Instantiate(ctx); err != nil {
			return fmt.Errorf("the host module %s: %w", name, err)
		}
	}
	return nil
}

// rubyVM calls the functions ruby.wasm exports, in the component model's
// calling convention: a string is a pointer and a length in the module's
// memory, allocated with cabi_realloc, and a result that does not fit in a
// register comes back as a pointer to it.
type rubyVM struct {
	ctx context.Context
	mod api.Module
}

func (vm *rubyVM) call(name string, args ...uint64) ([]uint64, error) {
	f := vm.mod.ExportedFunction(name)
	if f == nil {
		return nil, fmt.Errorf("ruby.wasm has no function %s", name)
	}
	return f.Call(vm.ctx, args...)
}

// put copies b into the module's memory and returns its address. A NUL
// follows the bytes, outside the length passed with them, since ruby.wasm
// hands some strings to C functions that read up to it.
func (vm *rubyVM) put(b []byte) (uint32, error) {
	b = append(b[:len(b):len(b)], 0)
	r, err := vm.call("cabi_realloc", 0, 0, 1, uint64(len(b)))
	if err != nil {
		return 0, err
	}
	ptr := uint32(r[0])
	if !vm.mod.Memory().Write(ptr, b) {
		return 0, errors.New("writing into ruby.wasm's memory")
	}
	return ptr, nil
}

func (vm *rubyVM) u32(addr uint32) uint32 {
	v, _ := vm.mod.Memory().ReadUint32Le(addr)
	return v
}

// init starts Ruby with the arguments ruby.wasm's own JavaScript runtime
// gives it, and sets up its load path.
func (vm *rubyVM) init() error {
	args := []string{"ruby.wasm", "-EUTF-8", "-e_=0"}
	list := make([]byte, 8*len(args))
	for i, a := range args {
		ptr, err := vm.put([]byte(a))
		if err != nil {
			return err
		}
		binary.LittleEndian.PutUint32(list[8*i:], ptr)
		binary.LittleEndian.PutUint32(list[8*i+4:], uint32(len(a)))
	}
	ptr, err := vm.put(list)
	if err != nil {
		return err
	}
	if _, err := vm.call("ruby-init: func(args: list<string>) -> ()", uint64(ptr), uint64(len(args))); err != nil {
		return fmt.Errorf("starting Ruby: %w", err)
	}
	if _, err := vm.call("ruby-init-loadpath: func() -> ()"); err != nil {
		return fmt.Errorf("setting Ruby's load path: %w", err)
	}
	return nil
}

// eval evaluates code and returns the string it evaluates to.
func (vm *rubyVM) eval(code string) (string, error) {
	ptr, err := vm.put([]byte(code))
	if err != nil {
		return "", err
	}
	r, err := vm.call("rb-eval-string-protect: func(str: string) -> tuple<handle<rb-abi-value>, s32>", uint64(ptr), uint64(len(code)))
	if err != nil {
		return "", fmt.Errorf("evaluating the script: %w", err)
	}
	ret := uint32(r[0])
	value, state := vm.u32(ret), vm.u32(ret+4)
	if state != 0 {
		return "", fmt.Errorf("Ruby raised outside the script (state %d)", state)
	}
	r, err = vm.call("rstring-ptr: func(value: handle<rb-abi-value>) -> string", uint64(value))
	if err != nil {
		return "", err
	}
	ret = uint32(r[0])
	s, ok := vm.mod.Memory().Read(vm.u32(ret), vm.u32(ret+4))
	if !ok {
		return "", errors.New("reading Ruby's result")
	}
	return string(s), nil
}
