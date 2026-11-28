# cameo

**Documentation: [cameo-lang.github.io/cameo](https://cameo-lang.github.io/cameo/)**:
[examples](https://cameo-lang.github.io/cameo/examples.html) ·
[embedding in Go](https://cameo-lang.github.io/cameo/go.html) ·
[gRPC](https://cameo-lang.github.io/cameo/grpc.html) ·
[pause and resume](https://cameo-lang.github.io/cameo/pause.html) ·
[async](https://cameo-lang.github.io/cameo/async.html) ·
[I/O](https://cameo-lang.github.io/cameo/io.html) ·
[Ruby vs cameo](https://cameo-lang.github.io/cameo/ruby.html) ·
[numbers](https://cameo-lang.github.io/cameo/numbers.html)

cameo is Ruby for embedding. A cameo program looks like Ruby and has
Ruby's syntax, but some important things work differently. An interpreter
written in pure Go runs it, inside your Go program.

- **It is immutable.** A value never changes, objects included. An object
  that you pass into a method has the same value after the method returns.
  Work that runs concurrently shares data only to read it, so it never
  needs a lock. A class
  declares typed fields instead of instance variables. To change an object,
  make a new one, or a changed copy with `with`.
- **No I/O by default.** A script cannot reach files, the network or
  processes. The Go program that runs it can allow some kinds of I/O, or give
  it Go functions to call. All of cameo's I/O is in the package `cameoio`: an
  interpreter without it has none.
- **Embedding in Go uses protobuf.** Add the interpreter package, optionally
  add `cameoio` and your own classes, and load scripts. Every instance is a
  protobuf message, so values cross between your Go program, the script and
  gRPC services as they are.
- **Pause and resume.** As nothing changes, the state of a running script is
  easy to capture. A script can stop at any point, and its state is bytes.
  Load them later, even in another process, and the script continues. This
  helps on platforms that end apps without notice, such as iOS.
- **async and await.** A script can spread work over the cores of the
  machine. Together with pause and resume, this makes cameo a base for
  workflows that survive a restart of their host.

```ruby
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

## Build

cameo needs Go and nothing else: no cgo, no Ruby.

```sh
git clone https://github.com/cameo-lang/cameo
cd cameo
make build      # the cameo binary, in bin/
make install    # or: put cameo into GOBIN
make check      # lint, the Go tests and the integration tests: what CI runs
```

In a Go module of your own, `go get github.com/cameo-lang/cameo` adds the
package `github.com/cameo-lang/cameo/src/cameo`.

### A first program: examples/ex1

[examples/ex1](examples/ex1) is the smallest program that embeds cameo.
It defines a class, `Greeter`, in Go, runs a script that uses it, and
calls a method the script defines:

```go
cameo.Register("Greeter", &greetpb.Greeter{}).
	DefineMethod("greet", func(self *greetpb.Greeter, args []any, kwargs map[string]any, block *cameo.Block) (any, error) {
		name, ok := args[0].(string)
		// … refuses what is not a String
		return fmt.Sprintf("%s, %s!", self.Greeting, name), nil
	}).

prog, err := cameo.LoadFromString(file, src, cameoio.Streams(stdout, stderr)) // checks the script
if err := prog.Run(); err != nil {                     // runs it
line, err := prog.Call(nil, "welcome", "Go")           // calls a method of the script
```

These are lines of [examples/ex1/main.go](examples/ex1/main.go), which
has the rest.

```ruby
greeter = Greeter.new(greeting: "Hello")
puts greeter.greet("world")
```

```sh
go run ./examples/ex1                # Hello, world! …
go run ./examples/ex1 mine.cameo     # the same class, in a script of your own
```

The other programs under `examples/` each show one thing:
[io](examples/io) gives a script a file system,
[pause](examples/pause) stops a run and continues it from its bytes,
[webfetch](examples/webfetch) fetches web pages for a script, and
[grpc](examples/grpc) calls a public gRPC service.

## Play with it

`cameo irb` reads a line, evaluates it and prints its value:

```
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
```

Pause a script with `^C` and continue it later. `count.cameo` is the
script of [Pause and resume](https://cameo-lang.github.io/cameo/pause.html):

```
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
```

The other commands:

```sh
cameo script.cameo            # run; an uncaught exception prints script:line: message (Class)
cameo --io=file script.cameo  # the same, with File.read and File.write allowed
cameo check script.cameo      # parse and check only, report every error with its line
cameo proto script.cameo      # the .proto files of the script's classes
cameo ir script.cameo         # the IR, the instructions the script compiles to
cameo debug script.cameo      # run under a debugger with the commands of Ruby's debug
cameo agent-docs [topic]      # the guide for models that write cameo, as markdown
```

## Differences from Ruby

Most Ruby runs as it is: methods, blocks, classes, modules, pattern
matching, exceptions, and the core classes with their Enumerable. Three
things differ.

**Values are immutable.**

| Works | Does not |
|---|---|
| `a += [1]`, `a = a << 1`, `h[k] = v`, `point.x = 3`: an assignment rebinds the variable to a copy | `a << 1`, `a.push(1)`, `h.delete(k)` as statements: they raise, and name the form to write |
| `a = a.sort`, `s = s.gsub(…)` | `a.sort!`, `s.gsub!(…)`: every bang method of Ruby's core is refused before the script runs |
| `list.sum`, `reduce`, `map`, `each_with_object`, a `while` loop | `list.each { \|x\| total += x }`: a block captures values and cannot assign to a variable outside itself |
| typed fields, `int :x`, and `with(x: 1)` for a changed copy | instance variables, `attr_accessor`, setters, `Struct` |
| `==` on contents | `equal?`, `object_id`: there is no identity |

**Every object is a protobuf message.** A class's fields are a message's
fields, so an instance crosses to Go, to a file or to a gRPC service as
it is, and `cameo proto` prints the `.proto` text. This is also what
lets a run be dumped and resumed.

**No threads, no I/O, little metaprogramming.** `async` and `await`
stand in for threads. I/O is the host's to give. `eval`, `define_method`,
globals and `private` are left out.

[Ruby vs cameo](https://cameo-lang.github.io/cameo/ruby.html) shows each
difference beside its Ruby, and [doc/cameo-vs-ruby.md](doc/cameo-vs-ruby.md)
is the full list. A model that writes cameo reads `cameo agent-docs`
([agents.md](agents.md) lists its topics); a Go program hands the same
text to its models with `agentdocs.Text`.

## Numbers

**Speed.** Time relative to CRuby, lower is faster: the geometric mean
over the cases of CRuby's own benchmark suite that cameo runs, by
category. Measured on 2026-11-13 on a laptop (Apple M4), against CRuby 3.4.1.

| Category | Cases | CRuby | CRuby + YJIT | cameo |
|---|---:|---:|---:|---:|
| Method calls, blocks and the VM | 192 | 1.00× | 0.94× | 3.28× |
| Small programs | 8 | 1.00× | 0.41× | 2.04× |
| Arrays and Enumerable | 161 | 1.00× | 0.98× | 1.30× |
| Hashes | 26 | 1.00× | 0.84× | 4.62× |
| Sets | 161 | 1.00× | 0.68× | 0.97× |
| Strings and regexps | 105 | 1.00× | 1.00× | 0.83× |
| Numbers | 34 | 1.00× | 0.93× | 1.82× |
| Ranges | 36 | 1.00× | 0.91× | 1.91× |
| Time | 28 | 1.00× | 0.90× | 0.85× |
| Libraries (File paths, Pathname, CGI) | 21 | 1.00× | 1.00× | 0.51× |
| **All** | 772 | 1.00× | 0.88× | 1.51× |

**Compatibility.**

| Measured by | What counts | cameo |
|---|---|---|
| CRuby's benchmark suite | cases that run with Ruby's output | 781 of 1318 |
| ruby/spec, all of it | spec files the checker accepts | 1855 of 3858 |
| ruby/spec, all of it | examples that pass, of those CRuby passes in the same harness | 1558 of 2897 |
| Ruby 3.4's standard library | libraries built in | 32 of 87 |

Most of what does not run uses something cameo leaves out on purpose:
mutation, instance variables, metaprogramming, I/O. The site's
[numbers page](https://cameo-lang.github.io/cameo/numbers.html) says how
each was measured, and [COMPATIBILITY.md](COMPATIBILITY.md),
[doc/coverage.md](doc/coverage.md) and [doc/stdlib.md](doc/stdlib.md)
have the detail.

## Repository

| Path | Holds |
|---|---|
| `src/` | all Go code: `syntax` the front end, `ir` the instructions a script compiles to, `eval` the evaluator and the async scheduler, `object` the value model, `str` what the bytes of a string are (encodings, the scan, the index of characters), `stdlib` the core classes, `lib` the libraries a script requires, `protogen` the descriptors and `.proto` text, `cameo` the host API, `cmd/cameo` the binary |
| `examples/` | the specification: each `<name>.cameo` has the output it must print in `<name>.expected`, and `go test` holds it to that; the directories are Go programs that embed cameo |
| `tests/`, `tests/integration/` | Go tests against the packages, and shell tests that drive the built binary |
| `benchmarks/` | CRuby's benchmark suite sorted by what cameo runs, and the comparisons with other Rubies |
| `specs/`, [COMPATIBILITY.md](COMPATIBILITY.md) | ruby/spec sorted by what cameo's checker accepts, the harness that runs it, and what of it cameo passes |
| `doc/gh_pages/` | the site, deployed by the Pages workflow on every change; `go test ./tests` runs its samples |
| `doc/design/` | the design: the language, the IR, exceptions, pause and resume |
| `doc/history/` | how cameo came to be, one document per step; up to v0.1.1 cameo was called amber |
| `agents/`, [agents.md](agents.md) | guidance for coding agents that work on cameo ([CLAUDE.md](CLAUDE.md)), and for models that write cameo |

A release is a tag `vMAJOR.MINOR.PATCH` on a commit of main, made by the
Tag workflow ([.github/workflows/tag.yml](.github/workflows/tag.yml))
after `make check` passes on that commit.

## License

MIT; see [LICENSE](LICENSE). The CRuby benchmarks in `benchmarks/ruby` and
ruby/spec in `specs` keep their own licences, named there.
