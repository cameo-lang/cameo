// Command amber runs amber scripts.
//
// Milestone 1: `amber check script.arb` parses and checks a script and reports
// what the parser rejects and what amber leaves out, with line numbers;
// `amber script.arb` fails, since the evaluator does not exist yet. The
// language and the plan are in doc/design/amber.md.
package main

import (
	"fmt"
	"os"

	"github.com/radiospiel/amber/src/amber"
	"github.com/radiospiel/amber/src/syntax"
)

const usage = `usage: amber check <script.arb>    parse and check the script, report every error
       amber <script.arb>          run the script (not implemented yet)`

func main() {
	os.Exit(run(os.Args[1:]))
}

// run executes the command line and returns the process exit status: 0 when
// the command succeeded, 1 when the script failed, 2 when the command line
// is wrong or the command is not implemented.
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
		fmt.Fprintf(os.Stderr, "amber: running scripts is not implemented yet; `amber check %s` checks it\n", args[0])
		return 2
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
