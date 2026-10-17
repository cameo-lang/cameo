// Command amber runs amber scripts. The language and the plan are in
// doc/design/amber.md.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/radiospiel/amber/src/amber"
	"github.com/radiospiel/amber/src/syntax"
)

const usage = `usage: amber <script.arb>          run the script
       amber check <script.arb>    parse and check the script, report every error`

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
	case len(args) == 1 && args[0] != "check":
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
