# I/O

A script does no I/O of its own. By default it computes and writes to
the two streams its host gave it: `puts`, `print`, `p` and `warn`. It has
no `ENV`, no sockets, no `system` and no backticks. `File` handles paths
as strings, and its methods that touch a file do not exist; the error
says how to get them.

```ruby
p File.respond_to?(:read)                   # false unless the host allows file I/O
p File.basename("/home/ada/notes.txt")      # a path is only a string
```

- `cameo --io=file script.cameo` gives `File.read`, `File.open` for
  reading and `File.write` on the machine's files, from the working
  directory; `cameo --io script.cameo` allows every kind. The flag goes
  before the script.
- A Go program allows it with `prog.AllowIO("file")`, and the files are
  those of the `cameo.Env` it loaded the script with: both are needed.
  `cameoio.New(fsys, dir, stdin, stdout, stderr)` shares an afero file
  system, `cameoio.DefaultEnv()` the machine's own, and
  `cameoio.Streams(stdout, stderr)` none.
- Anything else a script reaches, the network, a database, a model, is a
  module of Go functions the host gives it (topic `embedding`). Such a
  function usually returns a promise: `page = await Web.fetch(url)`.

So do not write `File`, `Dir` or `ENV` unless you were told the script
runs with I/O allowed; ask which functions the host gives instead.
