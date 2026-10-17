# amber

A small language with Ruby's syntax, run by an interpreter written in Go.
Every value is immutable, every object is a protobuf message, and a script
hands work to goroutines with `async` and collects it with `await`.

```ruby
class Point
  int :x, :y

  def initialize(x, y)
    @x = x
    @y = y
  end

  def +(other) = Point.new(x + other.x, y + other.y)
end

a = Point.new(1, 2)
a.x = 10                                  # a is rebound to a copy with x: 10
b = a.tap { |p| { y: p.y * 5 } }          # a copy with y: 10; a is unchanged

job = async a, :+, b                      # a.+(b) on its own goroutine
puts await job                            # Point(x: 20, y: 12)
```

The language, its differences from Ruby, the object model, the async model
and the protobuf representation are in
[doc/design/amber.md](doc/design/amber.md), which is also the plan: the
interpreter is built in the milestones listed there.

## Status

Milestone 0. The component parses scripts and reports syntax errors;
running them is not implemented yet.

```sh
amber check script.arb      # parse, report syntax errors, exit 0 if it parses
amber script.arb            # fails until milestone 2
```

The scripts under [examples/](examples) are the specification: each
`<name>.arb` has the output it must produce in `<name>.expected`, and `go test`
holds the interpreter to them as milestones land. Today the test proves every
example parses.

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
