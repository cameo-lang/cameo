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
	"strings"

	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/ir"
	"github.com/cameo-lang/cameo/src/syntax"
	"golang.org/x/term"
)

// commands are the words cameo takes before a script's path, which a
// script cannot be named by; "--" too, so a mistyped command prints the
// usage instead of opening a file by that name.
var commands = map[string]bool{"check": true, "proto": true, "ir": true, "resume": true, "inspect": true, "irb": true, "debug": true, "version": true, "--version": true, "--": true}

const usage = `usage: cameo [--no-pause] [--io] <script.cameo> [args...]
                                         run the script, with args in ARGV; ^C pauses it where it is
                                         and writes its state to a temporary file,
                                         or ends it with --no-pause. A script does no I/O unless --io
                                         allows every kind, or --io=network,file,popen the kinds named
       cameo [--no-pause] resume <dump>  continue the script a dump holds
       cameo --ignore-mutation-errors <script.cameo>
                                         run it, a call of Ruby's mutators such as push returning its
                                         result on a copy instead of raising; resume takes it too
       cameo --keep-tail-frames <script.cameo>
                                         run it, a call that makes a tail call staying on the stack, the
                                         last 256 of a row, so that a dump shows the calls that led to
                                         where the script paused; resume takes it too
       cameo inspect <dump>              show what a dump holds: where the script paused, its variables,
                                         constants and promises
       cameo [--io] irb [script.cameo]   evaluate lines typed in, at the top level of the script when one is given
       cameo [--no-pause] debug <script.cameo>
                                         run the script under the debugger, stopped before its first statement
       cameo check <script.cameo>        parse and check the script, report every error
       cameo proto <script.cameo> [dir]  print the .proto files of the script's classes, or write them into dir
       cameo ir <script.cameo>           print the script's IR, the instructions it compiles to`

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
	pause, ignoreMutations, keepTail := true, false, false
	var io []string
flags:
	for len(args) > 0 {
		switch {
		case args[0] == "--no-pause":
			pause = false
		case args[0] == "--ignore-mutation-errors":
			ignoreMutations = true
		case args[0] == "--keep-tail-frames":
			keepTail = true
		case strings.HasPrefix(args[0], "--io"):
			allow, rest, err := cameo.ParseIOFlags(args)
			if err != nil {
				fmt.Fprintf(os.Stderr, "cameo: %v\n%s\n", err, usage)
				return 2
			}
			io, args = append(io, allow...), rest
			continue
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
	case len(args) == 2 && args[0] == "ir":
		if err := irListing(args[1], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case len(args) == 2 && args[0] == "resume":
		return resume(args[1], io, pause, ignoreMutations, keepTail, os.Stdout, os.Stderr)
	case (len(args) == 1 || len(args) == 2) && args[0] == "irb":
		path := ""
		if len(args) == 2 {
			path = args[1]
		}
		if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
			return irbOnTerminal(path, io, os.Stdin, os.Stdout)
		}
		return irb(path, io, os.Stdin, os.Stdout, os.Stderr)
	case len(args) == 2 && args[0] == "debug":
		return debugScript(args[1], io, pause, os.Stdin, os.Stdout, os.Stderr)
	case len(args) == 2 && args[0] == "inspect":
		if err := inspect(args[1], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case len(args) >= 1 && !commands[args[0]]:
		// What follows the script is its ARGV; a -- before it is dropped.
		argv := args[1:]
		if len(argv) > 0 && argv[0] == "--" {
			argv = argv[1:]
		}
		return runScript(args[0], argv, io, pause, ignoreMutations, keepTail, os.Stdout, os.Stderr)
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

// irListing prints the IR of the script at path: the unit it compiles to,
// one instruction per line. It fails on a script that uses a form the
// compiler does not compile yet, naming the form and its line.
func irListing(path string, out io.Writer) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cameo: %w", err)
	}
	prog, err := syntax.Parse(path, string(src))
	if err != nil {
		return err
	}
	unit, err := ir.Compile(path, prog)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	_, err = io.WriteString(out, ir.Listing(unit))
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
