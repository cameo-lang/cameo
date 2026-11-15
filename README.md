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

cameo is a small language with Ruby's syntax, run by an interpreter
written in Go.

- **Ruby, but immutable.** An assignment makes a changed copy and rebinds
  the variable. Classes declare typed fields, and every instance is a
  protobuf message.
- **I/O is separate.** A script reads no file, opens no socket and starts
  no process unless the program that runs it allows that, kind by kind,
  or gives it Go functions to call.
- **Easy to embed in Go.** The interpreter is pure Go, with no cgo. A Go
  program loads a script, runs it, calls its methods, and passes values
  and protobuf messages both ways.
- **Pause, dump, resume later.** A running script stops where it is,
  inside a method or a loop. Its state is bytes: store them, and continue
  the run in another process.

```ruby
include Cameo                  # async, await and promises

class Point
  int :x, :y

  def +(other) = Point.new(x: x + other.x, y: y + other.y)
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

cameo needs Go and nothing else.

```sh
make build      # the cameo binary, in bin/
make install    # or: put cameo into GOBIN
make check      # lint, the Go tests and the integration tests: what CI runs
```

In a Go module of your own, `go get github.com/cameo-lang/cameo` adds the
package `github.com/cameo-lang/cameo/src/cameo`.

## Embedding

[examples/ex1](examples/ex1) is the smallest program that embeds cameo: it
defines a class, `Greeter`, in Go, runs a script that uses it, and calls a
method the script defines.

```go
cameo.Register("Greeter", &greetpb.Greeter{}).DefineMethod("greet", greet)

prog, err := cameo.Load(file, src, stdout, stderr)     // checks the script
err = prog.Run()                                       // runs it
line, err := prog.Call(nil, "welcome", "Go")           // calls a method of the script
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

## Commands

```sh
cameo script.cameo            # run; an uncaught exception prints script:line: message (Class)
cameo --io=file script.cameo  # the same, with File.read and File.write allowed
cameo check script.cameo      # parse and check only, report every error with its line
cameo proto script.cameo      # the .proto files of the script's classes
cameo ir script.cameo         # the instructions the script compiles to
cameo irb                     # read, evaluate and print, line by line
cameo debug script.cameo      # run under a debugger with the commands of Ruby's debug
cameo resume <dump>           # continue a paused script
cameo inspect <dump>          # show where it paused and its variables
```

`^C` pauses a running script before its next statement and writes its
state to a dump file, named on standard error. `cameo resume` continues
it. [Pause and resume](https://cameo-lang.github.io/cameo/pause.html)
walks through an example.

## Differences from Ruby

Most Ruby runs as it is: methods, blocks, classes, modules, pattern
matching, exceptions, and the core classes with their Enumerable. Three
things differ.

**Values are immutable.**

| Works | Does not |
|---|---|
| `a += [1]`, `a = a << 1`, `h[k] = v`, `point.x = 3`: an assignment rebinds the variable to a copy | `a << 1`, `a.push(1)`, `h.delete(k)` as statements: they raise, and name the form to write |
| `a = a.sort`, `s = s.gsub(…)` | `a.sort!`, `s.gsub!(…)`: every bang method of Ruby's core is refused before the script runs |
| `list.sum`, `reduce`, `map`, `each_with_object`, a `while` loop | `list.each { \|x\| total += x }`: a block cannot assign to a variable outside itself |
| typed fields, `int :x`, and `with(x: 1)` for a changed copy | instance variables, `attr_accessor`, setters, `Struct` |
| `==` on contents | `equal?`, `object_id`: there is no identity |

**Every object is a protobuf message.** An instance crosses to Go, to a
file or to a gRPC service as it is, `cameo proto` prints the `.proto`
text, and a run can be dumped and resumed.

**No threads, no I/O, little metaprogramming.** `async` and `await`
stand in for threads. I/O is the host's to give. `eval`, `define_method`,
globals and `private` are left out.

[doc/cameo-vs-ruby.md](doc/cameo-vs-ruby.md) is the full list. A model
that writes cameo reads [agents.md](agents.md).

## Numbers

Measured on 2026-11-13 on a laptop (Apple M4), against CRuby 3.4.1.

**Speed**, time relative to CRuby, lower is faster: over the 772 cases of
CRuby's benchmark suite that cameo runs, the geometric mean is 1.51×
(CRuby with YJIT: 0.88×). By category it goes from 0.51× (libraries) to
4.62× (hashes).

**Compatibility.**

| Measured by | What counts | cameo |
|---|---|---|
| CRuby's benchmark suite | cases that run with Ruby's output | 781 of 1318 |
| ruby/spec, all of it | spec files the checker accepts | 1855 of 3858 |
| ruby/spec, `language/` and `core/` | examples that pass, of those CRuby passes in the same harness | 990 of 2073 |
| Ruby 3.4's standard library | libraries built in | 32 of 87 |

Most of what does not run uses something cameo leaves out on purpose.
The [numbers page](https://cameo-lang.github.io/cameo/numbers.html) has
the categories and how each figure was measured;
[doc/coverage.md](doc/coverage.md) and [doc/stdlib.md](doc/stdlib.md) have
the detail.

## Repository

| Path | Holds |
|---|---|
| `src/` | all Go code: `syntax` the front end, `ir` the instructions a script compiles to, `eval` the evaluator and the async scheduler, `object` the value model, `str` strings, `stdlib` the core classes, `lib` the libraries a script requires, `protogen` the descriptors and `.proto` text, `cameo` the host API, `cmd/cameo` the binary |
| `examples/` | the specification: each `<name>.cameo` has the output it must print in `<name>.expected`, and `go test` holds it to that; the directories are Go programs that embed cameo |
| `tests/`, `tests/integration/` | Go tests against the packages, and shell tests that drive the built binary |
| `benchmarks/`, `specs/` | CRuby's benchmark suite and ruby/spec, sorted by what cameo runs |
| `doc/gh_pages/` | the site, deployed by the Pages workflow on every change; `go test ./tests` runs its samples |
| `doc/design/`, `doc/history/` | the design, and how cameo came to be, one document per step; up to v0.1.1 cameo was called amber |
| `agents/`, [agents.md](agents.md) | guidance for coding agents that work on cameo ([CLAUDE.md](CLAUDE.md)), and for models that write cameo |

A release is a tag `vMAJOR.MINOR.PATCH` on a commit of main, made by the
Tag workflow ([.github/workflows/tag.yml](.github/workflows/tag.yml))
after `make check` passes on that commit.

## License

MIT; see [LICENSE](LICENSE). The CRuby benchmarks in `benchmarks/ruby` and
ruby/spec in `specs` keep their own licences, named there.
