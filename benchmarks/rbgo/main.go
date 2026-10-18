// Command rbgo runs a Ruby script on go-embedded-ruby, a Ruby interpreter
// written in Go, through its embedding API. It is the Go-embedded Ruby
// cameo's benchmarks compare against.
//
//	rbgo [-time FILE] script.rb
//
// The script's output, standard error included, goes to standard output:
// go-embedded-ruby's API has one writer for both. With -time, the seconds
// the script ran, without the binary's start, are written to FILE.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-embedded-ruby/ruby"
)

func main() {
	timeFile := flag.String("time", "", "write the seconds the script ran to this file")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: rbgo [-time FILE] script.rb")
		os.Exit(2)
	}
	src, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	start := time.Now()
	err = ruby.Run(string(src), os.Stdout)
	elapsed := time.Since(start)
	if *timeFile != "" {
		secs := strconv.FormatFloat(elapsed.Seconds(), 'f', 6, 64)
		os.WriteFile(*timeFile, []byte(secs+"\n"), 0o644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
