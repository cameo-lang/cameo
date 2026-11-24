## cameo {#ch-readme}

::: source
from `README.md`
:::

cameo is a small programming language. It uses the syntax of Ruby. An
interpreter that is written in Go runs it.

-   **Values do not change.** When you assign a value, cameo makes a copy
    with the change and gives the variable the copy. Other references keep
    the old value. A class declares fields with types, and each instance of
    a class is a protobuf message.
-   **A script does no I/O.** A script only computes. It cannot read a file,
    open a socket or start a process. The Go program that runs the script
    can permit each kind of I/O, or it can give the script Go functions to
    call.
-   **You can embed cameo in a Go program.** The interpreter is pure Go and
    does not use cgo. A Go program loads a script, runs it and calls its
    methods. Values and protobuf messages go in the two directions.
-   **A script can stop and continue later.** A running script can stop at
    any point, also inside a method or a loop. Its state is a sequence of
    bytes. You can keep these bytes and continue the run in a different
    process.

The site [cameo-lang.github.io/cameo](#ch-overview) has more:
[examples](#ch-examples), [embedding in Go](#ch-go), [gRPC](#ch-grpc),
[pause and resume](#ch-pause), [async](#ch-async), [I/O](#ch-io),
[Ruby vs cameo](#ch-ruby) and [numbers](#ch-numbers).

``` ruby
include Cameo                  # async, await and promises

class Point
  int :x, :y

  def +(other)
    Point.new(x: x + other.x, y: y + other.y)
  end
end

a = Point.new(x: 1, y: 2)
b = a
a.x = 10                       # rebinds a to a copy; b keeps x: 1
c = a.with { |p| { y: p.y * 5 } }

p a, b, c                      # Point(x: 10, y: 2), Point(x: 1, y: 2), Point(x: 10, y: 10)
job = async a, :+, c           # a.+(c) on a goroutine; job is its promise
p await job                    # Point(x: 20, y: 12)
```

`include Cameo` gives the script the methods of the `Cameo` module:
`async`, `await`, `promise`, `resolve` and `fail`. Without the include, these
names are not known. Thus a script shows that it runs work concurrently.

`int :x, :y` declares two fields of type `int`. `int` is a method of every
class, and the script does not need `include Cameo` for it. The other field
types have methods of the same kind, for example `string`, `float` and
`bool`. [Classes: typed fields and with](#ch-agents-classes) lists all of
them.

### Build

To build cameo, you need Go. You do not need cgo or Ruby.

``` sh
git clone https://github.com/cameo-lang/cameo
cd cameo
make build      # the cameo binary, in bin/
make install    # or: put cameo into GOBIN
make check      # lint, the Go tests and the integration tests: what CI runs
```

To use cameo in a Go module, run `go get github.com/cameo-lang/cameo`. This
adds the package `github.com/cameo-lang/cameo/src/cameo`.

### Play with it (interactive)

`cameo irb` reads a line, evaluates it and prints its value:

    $ cameo irb
    cameo(main):001> x = 41
    => 41
    cameo(main):002> def inc(n)
    cameo(main):002*   n + 1
    cameo(main):002* end
    => :inc
    cameo(main):003> inc(x)
    => 42
    cameo(main):004> a = [1, 2]
    => [1, 2]
    cameo(main):005> a << 3
    (irb):1: cameo does not support mutating calls without rebinding; use a = a << 1, +, or append (see https://15b.it/mut) (CameoError)
    cameo(main):006> a += [3]
    => [1, 2, 3]

To stop a script, press `^C`. You can continue it later. `count.cameo` is the
script of [Pause and resume](#ch-pause):

    $ cameo count.cameo
    1: 1
    2: 5
    ^C
    cameo: pausing; press ^C again to end the script
    cameo: paused before line 3 of count.cameo
    cameo: resume with: cameo resume /tmp/cameo-count-676897826.dump
    $ cameo inspect /tmp/cameo-count-676897826.dump     # where it stopped, and its variables
    $ cameo resume /tmp/cameo-count-676897826.dump
    3: 14
    4: 30
    ...
    done: 22140

The other commands:

``` sh
cameo script.cameo            # run; an uncaught exception prints script:line: message (Class)
cameo --io=file script.cameo  # the same, with File.read and File.write allowed
cameo check script.cameo      # parse and check only, report every error with its line
cameo proto script.cameo      # the .proto files of the script's classes
cameo ir script.cameo         # the IR, the instructions the script compiles to
cameo debug script.cameo      # run under a debugger with the commands of Ruby's debug
cameo agent-docs [topic]      # the guide for models that write cameo, as markdown
```

### Embedding: examples/ex1

[examples/ex1](https://github.com/cameo-lang/cameo/blob/main/examples/ex1)
is the smallest Go program that embeds cameo. It has three files that you
write, and one file that a tool makes:

-   `greet.proto` declares the protobuf message `Greeter`.
-   `greetpb/greet.pb.go` is the Go code for this message. `protoc` makes it
    from `greet.proto`, when you run `make proto`. Do not edit it.
-   `main.go` registers `Greeter` as a cameo class, runs a script and calls
    a method of the script.
-   `hello.cameo` is the script. `main.go` embeds it.

`greet.proto`:

``` proto
// The message behind the Greeter class of this example: an instance of
// the class in a script is one of these messages in Go.
syntax = "proto3";

package ex1;

option go_package = "github.com/cameo-lang/cameo/examples/ex1/greetpb";

message Greeter {
  string greeting = 1;
}
```

`main.go`:

``` go
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
	"github.com/cameo-lang/cameo/src/cameoio"
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
	prog, err := cameo.LoadFromString(file, src, cameoio.Streams(stdout, stderr))
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
```

`cameo.Register` gives the class `Greeter` to every script. Its fields are
the fields of the message, so `Greeter.new(greeting: "Hello")` in a script is
a `*greetpb.Greeter` in Go. Each `DefineMethod` adds a method to the class.
The Go function of a method gets the instance (`self`), the arguments, the
keyword arguments and the block of the call. It returns the result and an
error. A Go function that returns an error makes the script raise it.

`hello.cameo`:

``` ruby
# Greeter is a class the Go program defines: its fields are those of a
# protobuf message, and its methods are Go functions.
greeter = Greeter.new(greeting: "Hello")
puts greeter.greet("world")

# A method returns a new value; nothing changes in place.
loud = greeter.louder
puts loud.greet("cameo")
puts greeter.greet("again")

# The Go program calls this method once the script has run.
def welcome(name)
  Greeter.new(greeting: "Welcome").greet(name)
end
```

``` sh
go run ./examples/ex1                # Hello, world! …
go run ./examples/ex1 mine.cameo     # the same class, in a script of your own
```

### File I/O and network

The other programs in `examples/` each show one thing:

-   [io](https://github.com/cameo-lang/cameo/blob/main/examples/io) gives a
    script a file system.
-   [webfetch](https://github.com/cameo-lang/cameo/blob/main/examples/webfetch)
    fetches web pages for a script.
-   [grpc](https://github.com/cameo-lang/cameo/blob/main/examples/grpc) calls
    a public gRPC service.
-   [pause](https://github.com/cameo-lang/cameo/blob/main/examples/pause)
    stops a run and continues it from its bytes.

### Differences from Ruby

Most Ruby runs as it is: methods, blocks, classes, modules, pattern matching,
exceptions, and the core classes with their Enumerable. Three things differ.

**Values are immutable.**

| Works                                                                                             | Does not                                                                                               |
|---------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------|
| `a += [1]`, `a = a << 1`, `h[k] = v`, `point.x = 3`: an assignment rebinds the variable to a copy | `a << 1`, `a.push(1)`, `h.delete(k)` as statements: they raise, and name the form to write             |
| `a = a.sort`, `s = s.gsub(…)`                                                                     | `a.sort!`, `s.gsub!(…)`: every bang method of Ruby's core is refused before the script runs            |
| `list.sum`, `reduce`, `map`, `each_with_object`, a `while` loop                                   | `list.each { |x| total += x }`: a block captures values and cannot assign to a variable outside itself |
| typed fields, `int :x`, and `with(x: 1)` for a changed copy                                       | instance variables, `attr_accessor`, setters, `Struct`                                                 |
| `==` on contents                                                                                  | `equal?`, `object_id`: there is no identity                                                            |

**Every object is a protobuf message.** A class's fields are a message's
fields, so an instance crosses to Go, to a file or to a gRPC service as it is,
and `cameo proto` prints the `.proto` text. This is also what lets a run be
dumped and resumed.

**No threads, no I/O, little metaprogramming.** `async` and `await` stand in
for threads. I/O is the host's to give. `eval`, `define_method`, globals and
`private` are left out.

[Ruby vs cameo](#ch-ruby) shows each
difference beside its Ruby, and [doc/cameo-vs-ruby.md](#ch-reference-cameo-vs-ruby)
is the full list. A model that writes cameo reads `cameo agent-docs`
([agents.md](#ch-agents) lists its topics); a Go program hands the same text to
its models with `agentdocs.Text`.

### Numbers

**Speed.** Time relative to CRuby, lower is faster: the geometric mean over
the cases of CRuby's own benchmark suite that cameo runs, by category.
Measured on 2026-11-13 on a laptop (Apple M4), against CRuby 3.4.1.

| Category                              | Cases | CRuby | CRuby + YJIT | cameo |
|---------------------------------------|------:|------:|-------------:|------:|
| Method calls, blocks and the VM       |   192 | 1.00× |        0.94× | 3.28× |
| Small programs                        |     8 | 1.00× |        0.41× | 2.04× |
| Arrays and Enumerable                 |   161 | 1.00× |        0.98× | 1.30× |
| Hashes                                |    26 | 1.00× |        0.84× | 4.62× |
| Sets                                  |   161 | 1.00× |        0.68× | 0.97× |
| Strings and regexps                   |   105 | 1.00× |        1.00× | 0.83× |
| Numbers                               |    34 | 1.00× |        0.93× | 1.82× |
| Ranges                                |    36 | 1.00× |        0.91× | 1.91× |
| Time                                  |    28 | 1.00× |        0.90× | 0.85× |
| Libraries (File paths, Pathname, CGI) |    21 | 1.00× |        1.00× | 0.51× |
| **All**                               |   772 | 1.00× |        0.88× | 1.51× |

**Compatibility.**

| Measured by                 | What counts                                                   | cameo        |
|-----------------------------|---------------------------------------------------------------|--------------|
| CRuby's benchmark suite     | cases that run with Ruby's output                             | 781 of 1318  |
| ruby/spec, all of it        | spec files the checker accepts                                | 1855 of 3858 |
| ruby/spec, all of it        | examples that pass, of those CRuby passes in the same harness | 1558 of 2897 |
| Ruby 3.4's standard library | libraries built in                                            | 32 of 87     |

Most of what does not run uses something cameo leaves out on purpose:
mutation, instance variables, metaprogramming, I/O. The site's [numbers
page](#ch-numbers) says how each was
measured, and [COMPATIBILITY.md](#ch-reference-compatibility),
[doc/coverage.md](#ch-reference-coverage) and [doc/stdlib.md](#ch-reference-stdlib) have the
detail.

### Repository

| Path                                           | Holds                                                                                                                                                                                                                                                                                                                                                                                                      |
|------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `src/`                                         | all Go code: `syntax` the front end, `ir` the instructions a script compiles to, `eval` the evaluator and the async scheduler, `object` the value model, `str` what the bytes of a string are (encodings, the scan, the index of characters), `stdlib` the core classes, `lib` the libraries a script requires, `protogen` the descriptors and `.proto` text, `cameo` the host API, `cmd/cameo` the binary |
| `examples/`                                    | the specification: each `<name>.cameo` has the output it must print in `<name>.expected`, and `go test` holds it to that; the directories are Go programs that embed cameo                                                                                                                                                                                                                                 |
| `tests/`, `tests/integration/`                 | Go tests against the packages, and shell tests that drive the built binary                                                                                                                                                                                                                                                                                                                                 |
| `benchmarks/`                                  | CRuby's benchmark suite sorted by what cameo runs, and the comparisons with other Rubies                                                                                                                                                                                                                                                                                                                   |
| `specs/`, [COMPATIBILITY.md](#ch-reference-compatibility) | ruby/spec sorted by what cameo's checker accepts, the harness that runs it, and what of it cameo passes                                                                                                                                                                                                                                                                                                    |
| `doc/gh_pages/`                                | the site, deployed by the Pages workflow on every change; `go test ./tests` runs its samples                                                                                                                                                                                                                                                                                                               |
| `doc/design/`                                  | the design: the language, the IR, exceptions, pause and resume                                                                                                                                                                                                                                                                                                                                             |
| `doc/history/`                                 | how cameo came to be, one document per step; up to v0.1.1 cameo was called amber                                                                                                                                                                                                                                                                                                                           |
| `agents/`, [agents.md](#ch-agents)              | guidance for coding agents that work on cameo ([CLAUDE.md](https://github.com/cameo-lang/cameo/blob/main/CLAUDE.md)), and for models that write cameo                                                                                                                                                                                                                                                                                                    |

A release is a tag `vMAJOR.MINOR.PATCH` on a commit of main, made by the Tag
workflow ([.github/workflows/tag.yml](https://github.com/cameo-lang/cameo/blob/main/.github/workflows/tag.yml)) after
`make check` passes on that commit.

### License

MIT; see [LICENSE](https://github.com/cameo-lang/cameo/blob/main/LICENSE). The CRuby benchmarks in `benchmarks/ruby` and
ruby/spec in `specs` keep their own licences, named there.
