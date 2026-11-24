## cameo's documentation {#ch-reference-docs}

::: source
from `doc/README.md`
:::

The documentation a user reads is the site,
[cameo-lang.github.io/cameo](#ch-overview), whose pages
are in [gh_pages](https://github.com/cameo-lang/cameo/blob/main/doc/gh_pages). It is ordered by what a reader wants to do, and
shows each thing as a program with what it prints.

| To                                            | Read                                                                                               |
|-----------------------------------------------|----------------------------------------------------------------------------------------------------|
| see the language                              | [Examples](#ch-examples)                                       |
| run a script from a Go program                | [Embedding in Go](#ch-go), and [examples/ex1](https://github.com/cameo-lang/cameo/blob/main/examples/ex1) |
| call a gRPC service from a script             | [gRPC](#ch-grpc)                                               |
| stop a run and continue it later              | [Pause and resume](#ch-pause)                                  |
| run work on several cores                     | [Async](#ch-async)                                             |
| let a script read and write                   | [I/O](#ch-io)                                                  |
| know what differs from Ruby                   | [Ruby vs cameo](#ch-ruby)                                      |
| know how fast it is and how much of Ruby runs | [Numbers](#ch-numbers)                                         |
| give a script Go functions and classes        | [Native code](#ch-native)                                      |
| have a model write cameo                      | [agents.md](#ch-agents)                                                                          |

The documents in this directory are the reference behind the site:

| File                                 | Holds                                                                                                                                                                                      |
|--------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [cameo-vs-ruby.md](#ch-reference-cameo-vs-ruby) | every difference from Ruby: what is the same, what differs, what the checker rejects, what is missing                                                                                      |
| [stdlib.md](#ch-reference-stdlib)               | every library of Ruby's standard library and its status in cameo                                                                                                                           |
| [coverage.md](#ch-reference-coverage)           | ruby/spec, measured: which examples pass and what stops the others                                                                                                                         |
| [debugger.md](#ch-reference-debugger)           | the commands of `cameo debug`                                                                                                                                                              |
| [design/](https://github.com/cameo-lang/cameo/blob/main/doc/design)                    | the design: the language ([cameo.md](#ch-design-language)), the IR ([ir.md](#ch-design-ir)), exceptions ([exceptions.md](#ch-design-exceptions)), pause and resume ([pause.md](#ch-design-pause)) |
| [history/](https://github.com/cameo-lang/cameo/blob/main/doc/history)                  | how cameo came to be, one document per step                                                                                                                                                |

### Changing the site

A page is one HTML file with the same header as the others. Four rules keep it
true, and `go test ./tests` holds the site to them:

-   A `<pre class="cameo">` block is run, and the `<pre class="output">` after
    it must be what it prints. A block cameo must refuse is
    `<pre class="cameo rejected">`, followed by its error.
-   A block that shows a file of the repository names it,
    `<pre data-file="examples/pause/main.go" class="go">`, and must be the
    file as it is. The file's own test runs it.
-   A script that needs a Go program around it is
    `<pre class="cameo hosted">`, which is not run on its own.
-   A command of one line, `<pre class="sh">` with `cameo …` or
    `go run ./examples/…`, that has a `<pre class="output">` under it is run,
    and must print that. A file it needs is a block of the same page with its
    name, `<pre data-name="count.cameo" class="ruby">`. In the output, a line
    `...` stands for any lines, a line `^C` is an interrupt sent after the
    line before it, and the path of a dump may be any.

The README is held to the same: its irb session, its `^C` session and its Go
excerpt, which must be lines of `examples/ex1/main.go`.
