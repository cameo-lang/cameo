// Command amber runs amber scripts. The language and the plan are in
// doc/design/amber.md.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/radiospiel/amber/src/amber"
	"github.com/radiospiel/amber/src/syntax"
)

const usage = `usage: amber <script.arb>              run the script
       amber check <script.arb>        parse and check the script, report every error
       amber proto <script.arb> [dir]  print the .proto files of the script's classes, or write them into dir`

func main() {
	os.Exit(run(os.Args[1:]))
}

// run executes the command line and returns the process exit status: 0 when
// the script ran or checked, 1 when it failed, 2 when the command line is
// wrong.
func run(args []string) int {
	switch {
	case len(args) == 1 && args[0] == "version":
		fmt.Println(amber.Version())
		return 0
	case len(args) == 1 && args[0] == "--version":
		fmt.Printf("amber version %s\n", amber.Version())
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
	case len(args) == 1 && args[0] != "check" && args[0] != "proto":
		if err := amber.RunFile(args[0], os.Stdout, os.Stderr); err != nil {
			var errs syntax.ErrorList
			if errors.As(err, &errs) {
				fmt.Fprintln(os.Stderr, errs)
				return 1
			}
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
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
		return fmt.Errorf("amber: %w", err)
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
		return fmt.Errorf("amber: %w", err)
	}
	prog, err := amber.Load(path, string(src), io.Discard, io.Discard)
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
