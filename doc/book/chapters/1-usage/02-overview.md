## Ruby, but immutable. {#ch-overview}

::: source
from `doc/gh_pages/index.html`
:::

cameo is a language with Ruby's syntax, run by an interpreter written in Go.
It is made for embedding in Go programs.

Every value is immutable. A call that would change a value in Ruby rebinds the
local variable it is called on to a changed copy; every other reference keeps
what it held. Classes declare typed fields, and every instance is a protobuf
message.

``` {.ruby .cameo}
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

p a, b, c
job = async a, :+, c           # a.+(c) on a goroutine; job is its promise
p await job
```

``` output
Point(x: 10, y: 2)
Point(x: 1, y: 2)
Point(x: 10, y: 10)
Point(x: 20, y: 12)
```

<figure>
<img src="../gh_pages/img/overview.svg"
alt="A Go program with the cameo interpreter inside it. The script and the program&#39;s Go functions and classes exchange values and protobuf messages; the program loads, runs, calls, pauses and resumes the script. Files, the network, databases and gRPC services are reached through the Go program alone, and a paused run leaves the program as bytes that are stored and resumed later." />
<figcaption>A script computes. Everything else, I/O included, goes through the
Go program that runs it.</figcaption>
</figure>

### What it is for: scripting a Go program

A Go program loads a cameo script, runs it, calls its methods, and passes
values and protobuf messages both ways. A script's classes are protobuf
messages, so the host reads an instance as a `proto.Message`, and a message
the host builds becomes an instance in the script.

-   **It starts in microseconds.** Loading, checking and running a five-line
    script with a class takes about 80 µs in a Go process (`cameo.Run` in a
    loop of 2000, on a four-core Linux machine).
-   **It needs no cgo.** The interpreter and its parser,
    [go-ruby-parser](https://github.com/go-ruby-parser/parser), are pure Go:
    no Ruby installation and no code generation.
-   **It builds wherever Go does.** The `cameo` package compiles with
    `CGO_ENABLED=0` for ios/arm64, android/arm64, darwin/arm64, windows/amd64,
    js/wasm and wasip1/wasm.
-   **A script does no I/O of its own.** It has no `ENV`, sockets or `system`,
    and writes only to the two streams the host gave it. The host allows more
    [kind by kind](#ch-io), such as files in a file system of its choosing, or
    gives the script a module of Go functions.
-   **A run can stop and go on later.** The host [pauses a script](#ch-pause)
    where it is, keeps its state as bytes, and resumes it in another process.

A script runs in its own interpreter, and `RunContext` stops it, with every
worker it started, when the host's context ends.

### How fast it is

cameo runs the 781 cases of CRuby's own benchmark suite that use only what
cameo supports, every one with Ruby's output. Over the 772 that CRuby 3.4 runs
too, on a laptop:

> cameo takes 1.51 times CRuby's time on the typical case, and CRuby with YJIT
> 0.88 times. It is faster than CRuby on strings, times and paths, and slowest
> on hashes and on calls of small methods.

[Numbers](#ch-numbers) has the table by category, how it was measured, and how
much of Ruby cameo runs.

### Read on

::: cards
[**Examples** cameo in seven short programs, each with what it
prints.](#ch-examples) [**Embedding in Go** Load, run and call a script;
convert values; pass protobuf messages; handle errors; run workers.](#ch-go)
[**gRPC** A script calls a public gRPC service, its classes being the
service's messages.](#ch-grpc) [**Pause and resume** Stop a run where it is
and continue it later, by hand and from Go.](#ch-pause) [**Async** The
concurrency model: async, await and promises, on one core and on many, with
I/O done by the host.](#ch-async) [**I/O** What a script may do by default,
and how the command line and a Go program allow more.](#ch-io) [**Ruby vs
cameo** What works as in Ruby, and each difference beside its Ruby:
immutability, typed fields, what the checker rejects.](#ch-ruby) [**Numbers**
Speed against CRuby and YJIT by category, and how much of Ruby
runs.](#ch-numbers) [**Native code** Give a script modules of Go methods and
classes backed by a protobuf message, or add a builtin class.](#ch-native)
[**For agents** The guide for models that write or embed cameo, as markdown
topics; `cameo agent-docs` prints the
same.](https://cameo-lang.github.io/cameo/llms.txt)
:::

The interpreter is complete to its [design](#ch-design-language). The current
release is v0.5.1, under the MIT licence.
