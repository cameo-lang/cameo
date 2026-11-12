// Command bumpvm runs a script on the experimental evaluator of package
// bumpvm: bumpvm [-stats] [-heap megabytes] script.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	"github.com/cameo-lang/cameo/src/bumpvm"
	"github.com/cameo-lang/cameo/src/ir"
	"github.com/cameo-lang/cameo/src/syntax"
)

func main() {
	stats := flag.Bool("stats", false, "print what the heap did")
	heap := flag.Int("heap", 32, "megabytes of heap before a collection")
	flag.Parse()
	path := flag.Arg(0)
	src, err := os.ReadFile(path)
	if err == nil {
		var prog *syntax.Program
		if prog, err = syntax.Parse(path, string(src)); err == nil {
			unit, cerr := ir.Compile(path, prog)
			if err = cerr; err == nil {
				out := bufio.NewWriter(os.Stdout)
				vm := bumpvm.New(*heap<<20/8, out)
				vm.Load(unit)
				vm.Run()
				out.Flush()
				if *stats {
					fmt.Fprintf(os.Stderr, "allocated %d MB, %d collections, copied %d KB\n", vm.Allocated*8>>20, vm.Collections, vm.Copied*8>>10)
				}
				return
			}
		}
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
