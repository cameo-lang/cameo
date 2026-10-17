# amber

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

job = async a, :+, b                      # a.+(b) on its own goroutine
puts await job                            # Point(x: 20, y: 12)
```

The language, its differences from Ruby, the object model, the async model
and the protobuf representation are in
[doc/design/amber.md](doc/design/amber.md), which is also the plan: the
interpreter is built in the milestones listed there.
[doc/amber-vs-ruby.md](doc/amber-vs-ruby.md) lists, for someone who knows
Ruby, what is the same, what differs, and what amber does not support.

## Status

The interpreter is complete to the plan in the design: the front end, the
evaluator, the library, classes with typed fields, protobuf and async.
`async recv, :method, *args` starts the call on a process of its own,
`await` collects its result or re-raises its exception, and `await :all`
waits for everything the current scope started. The Ruby adapter
(milestone 7) is not built.

```sh
amber script.arb            # run; an uncaught exception prints script:line: message (Class)
amber check script.arb      # parse and check only, report every error with its line
amber proto script.arb      # the .proto files of the script's classes, one per package
amber proto script.arb out/ # the same, written into out/
```

From Go:

```go
import "github.com/radiospiel/amber/src/amber"

prog, err := amber.Load("script.arb", src, os.Stdout, os.Stderr)
err = prog.Run()                                       // defines the classes
out, err := prog.Call("Point", "new", map[string]any{"x": 1, "y": 2})
msg := out.(proto.Message)                             // a dynamicpb message of amber.Point
sum, err := prog.Call(msg, "+", msg)                   // call a method on it
text, err := prog.Proto("Point")                       // the .proto text
```

The scripts under [examples/](examples) are the specification: each
`<name>.arb` has the output it must produce in `<name>.expected`, and `go test`
runs every script and holds it to that output. The examples a later milestone
brings are listed as pending in the test and must keep failing until it lands.
An example that is also plain Ruby has its expected output generated with
`ruby`, so amber is held to Ruby's behaviour where the two overlap.

[benchmarks/ruby/](benchmarks/ruby) holds CRuby's own benchmark suite, one
script per case, grouped into what works, what fails on an amber bug, what
needs a builtin amber does not have yet, and what uses a feature amber leaves
out; its `run.rb` checks that the working ones keep working.

The packages under `src/`: `syntax` is the front end (`syntax.Parse`),
`object` the value model, `eval` the evaluator and the async scheduler,
`stdlib` the builtin methods, `protogen` the descriptors, `.proto` text and
the envelope, `amber` the host API (`amber.Load`, `amber.Run`), and
`cmd/amber` the binary.

## Building

```sh
make build      # the amber binary and the packages
make tests      # go test ./...
make install    # puts `amber` into GOBIN
```

The parser is `github.com/go-ruby-parser/parser`, a Ruby parser in pure Go.
No cgo, no Ruby installation, no code generation.

## Repository

| Path | Holds |
|---|---|
| `src/` | all Go code: the packages above and `src/cmd/amber`, the command |
| `src/assert/` | the assertions the tests use, vendored from simple-go |
| `tests/` | Go tests against the packages, as a caller uses them |
| `tests/integration/` | shell tests that drive the built binary, on `testsh.inc` |
| `doc/history/` | how amber came to be, one document per step |
| `agents/` | guidance for coding agents; start at [CLAUDE.md](CLAUDE.md) |
| `.github/` | the CI workflow and the pull request template |

## License

Proprietary; see [LICENSE](LICENSE). Not open source.
