# cameo

A small language with Ruby's syntax, run by an interpreter written in Go.
Every value is immutable, every object is a protobuf message, and a script
hands work to goroutines with `async` and collects it with `await`.

```ruby
class Point
  int :x, :y

  def initialize(x, y)
    self.x = x
    self.y = y
  end

  def +(other) = Point.new(x + other.x, y + other.y)
end

a = Point.new(1, 2)
a.x = 10                                  # a is rebound to a copy with x: 10
b = a.with { |p| { y: p.y * 5 } }         # a copy with y: 10; a is unchanged

include Cameo                             # async, await and promises
job = async a, :+, b                      # a.+(b) on another goroutine; job is its promise
puts await job                            # Point(x: 20, y: 12)
```

The documentation is on the site, [cameo-lang.github.io/cameo](https://cameo-lang.github.io/cameo/):
the language for someone who knows Ruby, the async model, a Go program
embedding the interpreter, and native methods. In the repository, the
language, its differences from Ruby, the object model, the async model
and the protobuf representation are in
[doc/design/cameo.md](doc/design/cameo.md), which is also the plan: the
interpreter is built in the milestones listed there.
[doc/cameo-vs-ruby.md](doc/cameo-vs-ruby.md) lists, for someone who knows
Ruby, what is the same, what differs, and what cameo does not support.
A script can `require` the libraries cameo builds in, which
[doc/stdlib.md](doc/stdlib.md) lists, with every other library of Ruby's
standard library and its status in cameo: built in, planned, or left out
and why. [doc/coverage.md](doc/coverage.md) measures the first: which of
ruby/spec's examples pass, what stops the others, and which of those are
bugs.

## Status

The interpreter is complete to the plan in the design: the front end, the
evaluator, the library, classes with typed fields, protobuf and async.
A script that includes Cameo has `async recv, :method, *args`, which
starts the call on a worker of its own and returns its promise, an
Integer; `await` collects a promise's result or re-raises its exception,
`await :all` waits for everything the current scope started, and
`promise`, `resolve` and `fail` make and settle a promise by hand. A Go host
returns a promise from a method (`prog.Promise()`) and settles it when its
I/O is done; [examples/webfetch](examples/webfetch) fetches web pages so.
[examples/async](examples/async) walks through the model, and
[ruby/lib/cameo/runner.rb](ruby/lib/cameo/runner.rb) gives CRuby the same
`Cameo::Runner` module, so a script whose classes include it runs on both:
`ruby -I ruby/lib examples/async/02_fan_out.rb`. The site's
[async guide](https://cameo-lang.github.io/cameo/async.html) explains it all. The Ruby adapter
(milestone 7) is not built.

```sh
cameo script.cameo            # run; an uncaught exception prints script:line: message (Class)
cameo check script.cameo      # parse and check only, report every error with its line
cameo proto script.cameo      # the .proto files of the script's classes, one per package
cameo proto script.cameo out/ # the same, written into out/
cameo ir script.cameo         # the instructions the script compiles to; see doc/design/ir.md
cameo build script.cameo out/ # compile the top-level Integer methods to Go: out/main.go runs the script
cameo run script.cameo        # the same, built with go and run; needs Go and the cameo source cameo was built from
cameo resume path/to/dump     # continue a script that ^C paused
cameo inspect path/to/dump    # show where it paused and what its top level held
cameo irb [script.cameo]      # evaluate lines typed in, at the top level of the script when one is given
cameo debug script.cameo      # run the script under the debugger, stopped before its first statement
```

`^C` pauses a running script before its next top-level statement and writes
its state, a protobuf message, to a temporary file; `cameo resume` continues
it, `cameo inspect` shows it, and `cameo --no-pause script.cameo` lets `^C`
end the script instead. A statement that never ends is never paused: a
second `^C` ends the script. What the dump holds, and what it leaves out,
is in [doc/design/pause.md](doc/design/pause.md).

`cameo irb` is a read-eval-print loop: each line typed in runs at the top
level, its value is printed, and a line that is not a whole expression yet
waits for the next. With a script given, the script runs first and the
lines see what it defined. On a terminal it edits the line being typed:
arrow keys, a history on the up and down arrows, which keeps the last 1000
lines in `~/.cameo_history`, Tab to complete a name, and Ctrl-C to drop a
line. A program that embeds cameo gets the same loop from the package
[src/irb](src/irb), on a byte stream of its own, and the line editor alone
from [src/readline](src/readline), with the history file and its limit of
its choosing. A script run on a terminal reads lines with the same editor:
`Cameo::Kernel::Readline.readline("name? ", true)`.

`cameo debug` runs a script under a debugger with the commands of Ruby's
debug library: `step`, `next`, `finish`, `continue`, `break` on a line or
a method, `backtrace`, `info`, `list`, `up`, `down`, and any expression,
evaluated in the frame it stopped in. A script that requires `"debug"`
stops at `debugger` and `binding.break` the same way.
[doc/debugger.md](doc/debugger.md) has the commands.

`cameo build` is the first step of ahead-of-time compilation; see
[doc/design/aot.md](doc/design/aot.md).

From Go:

```go
import "github.com/cameo-lang/cameo/src/cameo"

prog, err := cameo.Load("script.cameo", src, os.Stdout, os.Stderr)
err = prog.Run()                                       // defines the classes
out, err := prog.Call("Point", "new", map[string]any{"x": 1, "y": 2})
msg := out.(proto.Message)                             // a dynamicpb message of cameo.Point
sum, err := prog.Call(msg, "+", msg)                   // call a method on it
text, err := prog.Proto("Point")                       // the .proto text
```

The scripts under [examples/](examples) are the specification: each
`<name>.cameo` has the output it must produce in `<name>.expected`, and `go test`
runs every script and holds it to that output. The examples a later milestone
brings are listed as pending in the test and must keep failing until it lands.
An example that is also plain Ruby has its expected output generated with
`ruby`, so cameo is held to Ruby's behaviour where the two overlap.

[benchmarks/ruby/](benchmarks/ruby) holds CRuby's own benchmark suite, one
script per case, grouped into what works, what fails on a cameo bug, what
needs a builtin cameo does not have yet, and what uses a feature cameo leaves
out; its `run.rb` checks that the working ones keep working.

The packages under `src/`: `syntax` is the front end (`syntax.Parse`),
`object` the value model, `eval` the evaluator and the async scheduler,
`stdlib` the builtin methods of the core classes, `lib` the libraries a
script requires, one package each (`lib/json` is `json`), `protogen` the
descriptors, `.proto` text and the envelope, `cameo` the host API (`cameo.Load`, `cameo.Run`, `cameo.Register`), and
`cmd/cameo` the binary.

## Building

```sh
make build      # the cameo binary and the packages
make tests      # go test ./..., in UTC
make install    # puts `cameo` into GOBIN
```

The parser is `github.com/go-ruby-parser/parser`, a Ruby parser in pure Go.
No cgo and no Ruby installation. The Go code of cameo's own protobuf
messages is generated and committed ([src/cameopb](src/cameopb)), so a build
needs Go alone; `make proto` regenerates it after a `.proto` file changed,
with the `protoc` release it fetches.

## Site

[cameo-lang.github.io/cameo](https://cameo-lang.github.io/cameo/) is the
static site in [doc/gh_pages](doc/gh_pages), served by this repository's
GitHub Pages. The Pages workflow
([.github/workflows/pages.yml](.github/workflows/pages.yml)) deploys it on
every change to the directory. `go test ./tests` runs every sample on the
site against its output.

## Releasing

A release is a tag `vMAJOR.MINOR.PATCH` on a commit of main. The Tag
workflow ([.github/workflows/tag.yml](.github/workflows/tag.yml)) creates
it: start it from the Actions tab with the version and, optionally, the
commit. It refuses a version that exists, a commit that is not on main, and
a commit whose `go.mod` names another module path than the repository's,
and it tags only after `make check` passes on that commit.

## Repository

| Path | Holds |
|---|---|
| `src/` | all Go code: the packages above and `src/cmd/cameo`, the command |
| `src/assert/` | the assertions the tests use, vendored from simple-go |
| `tests/` | Go tests against the packages, as a caller uses them |
| `tests/integration/` | shell tests that drive the built binary, on `testsh.inc` |
| `doc/history/` | how cameo came to be, one document per step; up to v0.1.1 cameo was called amber, so the early documents and logs say `src/amber` for `src/cameo` and `.arb` for `.cameo` |
| `agents/` | guidance for coding agents; start at [CLAUDE.md](CLAUDE.md) |
| `.github/` | the CI workflow and the pull request template |

## License

MIT; see [LICENSE](LICENSE). The CRuby benchmarks in `benchmarks/ruby` and
ruby/spec in `specs` keep their own licences, named there.
