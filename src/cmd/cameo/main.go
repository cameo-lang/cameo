// Command cameo runs cameo scripts. The language and the plan are in
// doc/design/cameo.md.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"

	"github.com/cameo-lang/cameo/src/aot"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/syntax"
)

const usage = `usage: cameo [--no-pause] <script.cameo>  run the script; ^C pauses it before its next top-level statement
                                         and writes its state to a temporary file, or ends it with --no-pause
       cameo [--no-pause] resume <dump>  continue the script a dump holds
       cameo --ignore-mutation-errors <script.cameo>
                                         run it, a call of Ruby's mutators such as push returning its
                                         result on a copy instead of raising; resume takes it too
       cameo inspect <dump>              show what a dump holds: where the script paused, its variables,
                                         constants and promises
       cameo irb [script.cameo]          evaluate lines typed in, at the top level of the script when one is given
       cameo [--no-pause] debug <script.cameo>
                                         run the script under the debugger, stopped before its first statement
       cameo check <script.cameo>        parse and check the script, report every error
       cameo proto <script.cameo> [dir]  print the .proto files of the script's classes, or write them into dir
       cameo build <script.cameo> <dir> [package]
                                         compile the script's top-level methods that it can to Go: into
                                         dir/main.go, a program that runs the script, or dir/<package>.go,
                                         a package a host imports
       cameo run <script.cameo>          compile the script as cameo build does, build the program with go, and run it`

func main() {
	tuneGC()
	os.Exit(run(os.Args[1:]))
}

// tuneGC lets the heap grow to five times what survives a collection, where
// Go's default is twice, which spends a quarter less time on CRuby's
// benchmarks; past a soft limit of 1 GB the collector runs as often as it
// would by default, so a large heap does not grow fivefold. GOGC and
// GOMEMLIMIT in the environment override either. A program that embeds
// cameo keeps its own settings.
func tuneGC() {
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(400)
	}
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(1 << 30)
	}
}

// run executes the command line and returns the process exit status: 0 when
// the script ran or checked, 1 when it failed, 2 when the command line is
// wrong.
func run(args []string) int {
	pause, ignoreMutations := true, false
flags:
	for len(args) > 0 {
		switch args[0] {
		case "--no-pause":
			pause = false
		case "--ignore-mutation-errors":
			ignoreMutations = true
		default:
			break flags
		}
		args = args[1:]
	}
	switch {
	case len(args) == 1 && args[0] == "version":
		fmt.Println(cameo.Version())
		return 0
	case len(args) == 1 && args[0] == "--version":
		fmt.Printf("cameo version %s\n", cameo.Version())
		return 0
	case len(args) == 2 && args[0] == "check":
		if err := check(args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case (len(args) == 2 || len(args) == 3) && args[0] == "proto":
		dir := ""
		if len(args) == 3 {
			dir = args[2]
		}
		if err := protoFiles(args[1], dir, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case (len(args) == 3 || len(args) == 4) && args[0] == "build":
		pkg := "main"
		if len(args) == 4 {
			pkg = args[3]
		}
		if err := build(args[1], args[2], pkg, os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case len(args) == 2 && args[0] == "run":
		return runCompiled(args[1], os.Stdout, os.Stderr)
	case len(args) == 2 && args[0] == "resume":
		return resume(args[1], pause, ignoreMutations, os.Stdout, os.Stderr)
	case (len(args) == 1 || len(args) == 2) && args[0] == "irb":
		path := ""
		if len(args) == 2 {
			path = args[1]
		}
		return irb(path, os.Stdin, os.Stdout, os.Stderr)
	case len(args) == 2 && args[0] == "debug":
		return debugScript(args[1], pause, os.Stdin, os.Stdout, os.Stderr)
	case len(args) == 2 && args[0] == "inspect":
		if err := inspect(args[1], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case len(args) == 1 && args[0] != "check" && args[0] != "proto" && args[0] != "build" && args[0] != "run" && args[0] != "resume" && args[0] != "inspect" && args[0] != "irb" && args[0] != "debug":
		return runScript(args[0], pause, ignoreMutations, os.Stdout, os.Stderr)
	default:
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}
}

// check parses and checks the script at path and returns every error found,
// each naming its line.
func check(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cameo: %w", err)
	}
	_, err = syntax.Parse(path, string(src))
	return err
}

// protoFiles runs the script with its output discarded, since classes are
// defined by running, and prints or writes the .proto file of every
// package its classes belong to, plus the envelope.
func protoFiles(path, dir string, out io.Writer) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cameo: %w", err)
	}
	prog, err := cameo.Load(path, string(src), io.Discard, io.Discard)
	if err != nil {
		return err
	}
	if err := prog.Run(); err != nil {
		return err
	}
	files := prog.ProtoFiles()
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for i, n := range names {
		if dir == "" {
			if i > 0 {
				fmt.Fprintln(out)
			}
			fmt.Fprintf(out, "// %s\n%s", n, files[n])
			continue
		}
		target := filepath.Join(dir, n)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(files[n]), 0o644); err != nil {
			return err
		}
		fmt.Fprintln(out, target)
	}
	return nil
}

// build compiles the script at path into dir/main.go, or dir/<pkg>.go for
// a package other than main, and reports to out
// which methods run as Go, and why the others do not.
func build(path, dir, pkg string, out io.Writer) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cameo: %w", err)
	}
	res, err := aot.Build(path, string(src), aot.Options{Package: pkg})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	target := filepath.Join(dir, "main.go")
	if pkg != "main" {
		target = filepath.Join(dir, pkg+".go")
	}
	if err := os.WriteFile(target, res.Go, 0o644); err != nil {
		return err
	}
	for _, name := range res.Compiled {
		fmt.Fprintf(out, "compiled %s\n", name)
	}
	left := make([]string, 0, len(res.Left))
	for name := range res.Left {
		left = append(left, name)
	}
	sort.Strings(left)
	for _, name := range left {
		fmt.Fprintf(out, "interpreted %s: %s\n", name, res.Left[name])
	}
	fmt.Fprintln(out, target)
	return nil
}
