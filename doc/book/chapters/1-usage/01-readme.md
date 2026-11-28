## cameo {#ch-readme}

::: source
BOT: a shortened version of this goes into `README.md`
:::

If your love for Ruby goes so far, that you want to embed it somewhere, *critic* might be for you. It looks like ruby – each cameo program is a valid ruby program after all! – but it is different in some important aspects.

### It is immutable

values – including objects – never change. Cameo guarantees that the object that you have passed into function has the same value even after the function returns. This has some interesting applications down the road:

- sharing data (for example between threads) only shares read access. If an object never changes, no conflicting updates can occur, and synchronization is never necessary. 

```ruby
myObj = Klass.new(foo: "bar")
some_func(myObj)
# in ruby: the obj might or might not have changed. 
# in cameo: no change to the obj.
```

- instance vars (`@ivar`) are irrelevant (and unsupported). Ruby needs these to allow mutating the state of an object; we replace these with read-only attributes. Also, these attributes are typed (more on that later.) If you need a changed object, you must create a new one. Setting attributes can only occur in the constructor:

```ruby
class UserName
    string :user_name

    def normalize() = UserName.new(user_name: user_name.trim)
    
    def initialize(user_name:)
        self.user_name = user_name    
    end
end

user_name = UserName.new(user_name: "hugo")

# shortcut to update a nested slice of an object 
normalized_user = user.with do |u| 
    # return a update hash from the block
    { name: u.name.normalized_user }
end 
```

### No I/O by default.

By default, a script doesn't have access to files, network, processes etc. The go program that . The Go program that runs the script can allow some kind of I/O, or it can give the script Go functions to  call.

All I/O that comes with cameo is kept in a separate `cameoio` package. Not importing that package guarantees that I/O features are not present in an interpreter instance.

### Embedding in a Go program uses protobuf.

Embedding is a fairly straightforward process:

- embed the cameo interpreter package;
- (optional) add the cameo I/O package to give I/O capabilities for files, network, etc.
- (optional) configure cameo to access classes that your host provides;
- load scripts into the interpreter.

cameo uses protobuf messages as its internal representation. That allows exchanging data between the golang host, and other (GRPC-related) services. This goes both ways:

- a protobuf's message definition is automatically made available to a hosted cameo script. 
- a cameo class can be exported into a .proto definition which can be compiled into the go host.

Since protobuf requires types on its field names, cameo attributes must also be typed. We support all protobuf-native types for cameo attributes.   


### Pausing and resuming. 

Pausing and resuming a program requires to capture the state of a running program, and later reload the state and continue the execution. When all objects in a running process are immutable, that becomes trivial. cameo is also able to export the code that it is running, and this combinations makes it possible to stop a running script at any point and to dump its state. That state can later be loaded and resumed.

This feature can be useful for apps on platforms that freely terminate applications running on it and don't support running in the background (looking at you, iOS).


### async support

We implement support for some parallelity via `async` and `await`. This can help you distribute a CPU-bound workload over multiple CPU cores. However, but high performance applications are not the main purpose for cameo.

async support, however, makes it trivial to use `cameo` as a basis for persistent workflows. A script like the following can be put, for example, into an iOS app, which finishes the download task even when the host gets terminated and restarted.

``` ruby
include Cameo                  # async, await and promises

urls = [
  "http://foo.bar/1.html",
  "http://foo.bar/2.html",
  "http://foo.bar/3.html"
]

downloader = Downloader.new()

downloads = urls.map { |url| async downloader, :fetch, url }
await :downloads
```

The site [cameo-lang.github.io/cameo](#ch-overview) has more:
[examples](#ch-examples), [embedding in Go](#ch-go), [gRPC](#ch-grpc),
[pause and resume](#ch-pause), [async](#ch-async), [I/O](#ch-io),
[Ruby vs cameo](#ch-ruby) and [numbers](#ch-numbers).


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

What does not run falls into three groups. ruby/spec shows them best: of the
2897 examples that CRuby passes in cameo's harness, cameo passes 1558.

-   **Left out on purpose: 1198 examples.** They use something cameo does not
    have by design: mutation, instance variables, metaprogramming, threads or
    I/O.
-   **Bugs: 141 examples.** cameo should pass them and does not.
    COMPATIBILITY.md lists where they are.
-   **Not done yet.** cameo builds in 32 of the 87 libraries of Ruby 3.4's
    standard library. A script that requires one of the others does not run.
 The site's [numbers
page](#ch-numbers) says how each was
measured, and [COMPATIBILITY.md](#ch-reference-compatibility),
[doc/coverage.md](#ch-reference-coverage) and [doc/stdlib.md](#ch-reference-stdlib) have the
detail.

### License

MIT; see [LICENSE](https://github.com/cameo-lang/cameo/blob/main/LICENSE). The CRuby benchmarks in `benchmarks/ruby` and
ruby/spec in `specs` keep their own licences, named there.
