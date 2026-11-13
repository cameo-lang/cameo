# cameo's documentation

The documentation a user reads is the site,
[cameo-lang.github.io/cameo](https://cameo-lang.github.io/cameo/), whose
pages are in [gh_pages](gh_pages). It is ordered by what a reader wants
to do, and shows each thing as a program with what it prints.

| To | Read |
|---|---|
| see the language | [Examples](https://cameo-lang.github.io/cameo/examples.html) |
| run a script from a Go program | [Embedding in Go](https://cameo-lang.github.io/cameo/go.html), and [examples/ex1](../examples/ex1) |
| call a gRPC service from a script | [gRPC](https://cameo-lang.github.io/cameo/grpc.html) |
| stop a run and continue it later | [Pause and resume](https://cameo-lang.github.io/cameo/pause.html) |
| run work on several cores | [Async](https://cameo-lang.github.io/cameo/async.html) |
| let a script read and write | [I/O](https://cameo-lang.github.io/cameo/io.html) |
| know what differs from Ruby | [Ruby vs cameo](https://cameo-lang.github.io/cameo/ruby.html) |
| know how fast it is and how much of Ruby runs | [Numbers](https://cameo-lang.github.io/cameo/numbers.html) |
| give a script Go functions and classes | [Native code](https://cameo-lang.github.io/cameo/native.html) |
| have a model write cameo | [agents.md](../agents.md) |

The documents in this directory are the reference behind the site:

| File | Holds |
|---|---|
| [cameo-vs-ruby.md](cameo-vs-ruby.md) | every difference from Ruby: what is the same, what differs, what the checker rejects, what is missing |
| [stdlib.md](stdlib.md) | every library of Ruby's standard library and its status in cameo |
| [coverage.md](coverage.md) | ruby/spec, measured: which examples pass and what stops the others |
| [debugger.md](debugger.md) | the commands of `cameo debug` |
| [design/](design) | the design: the language ([cameo.md](design/cameo.md)), the IR ([ir.md](design/ir.md)), exceptions ([exceptions.md](design/exceptions.md)), pause and resume ([pause.md](design/pause.md)) |
| [history/](history) | how cameo came to be, one document per step |

## Changing the site

A page is one HTML file with the same header as the others. Three rules
keep it true, and `go test ./tests` holds the site to them:

- A `<pre class="cameo">` block is run, and the `<pre class="output">`
  after it must be what it prints. A block cameo must refuse is
  `<pre class="cameo rejected">`, followed by its error.
- A block that shows a file of the repository names it,
  `<pre data-file="examples/pause/main.go" class="go">`, and must be the
  file as it is. The file's own test runs it.
- A script that needs a Go program around it is
  `<pre class="cameo hosted">`, which is not run on its own.
