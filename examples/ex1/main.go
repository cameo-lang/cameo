// Command ex1 is the smallest program that embeds cameo: it defines a
// class in Go, runs a script that uses the class, and calls a method of
// the script.
//
//	go run ./examples/ex1                # runs hello.cameo, which is built in
//	go run ./examples/ex1 mine.cameo     # runs a script of your own
//
// The class is Greeter. Its fields are those of the protobuf message in
// greet.proto, so Greeter.new(greeting: "Hello") in a script is a
// *greetpb.Greeter in Go, and its methods are the Go functions below.
package main

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cameo-lang/cameo/examples/ex1/greetpb"
	"github.com/cameo-lang/cameo/src/cameo"
)

//go:embed hello.cameo
var hello string

// A class is registered once, before the first script is loaded. A method
// gets a copy of the instance it is called on: what it returns is a new
// value, and the script's instance stays as it was.
func init() {
	cameo.Register("Greeter", &greetpb.Greeter{}).
		DefineMethod("greet", func(self *greetpb.Greeter, args []any, kwargs map[string]any, block *cameo.Block) (any, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("greet takes one argument, not %d", len(args))
			}
			name, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("greet takes a String, not %T", args[0])
			}
			return fmt.Sprintf("%s, %s!", self.Greeting, name), nil
		}).
		DefineMethod("louder", func(self *greetpb.Greeter, args []any, kwargs map[string]any, block *cameo.Block) (any, error) {
			return &greetpb.Greeter{Greeting: strings.ToUpper(self.Greeting)}, nil
		})
}

// run loads the script src, named file in its error messages, and runs
// it. For the script that is built in, which defines welcome, run then
// calls that method from Go and prints what it returns.
func run(file, src string, stdout, stderr io.Writer) error {
	prog, err := cameo.Load(file, src, stdout, stderr)
	if err != nil {
		return err
	}
	if err := prog.Run(); err != nil {
		return err
	}
	if src != hello {
		return nil
	}
	line, err := prog.Call(nil, "welcome", "Go")
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, "from Go:", line)
	return nil
}

func main() {
	file, src := "hello.cameo", hello
	if len(os.Args) > 1 {
		data, err := os.ReadFile(os.Args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		file, src = os.Args[1], string(data)
	}
	if err := run(file, src, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
