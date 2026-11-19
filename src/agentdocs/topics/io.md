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
- A Go program allows it with `prog.AllowIO("file")` and a file system
  with `prog.FileSystem(fsys, dir)`, both; `prog.AllowHostIO("file")`
  is the short form for the machine's own files.
- Anything else a script reaches, the network, a database, a model, is a
  module of Go functions the host gives it (topic `embedding`). Such a
  function usually returns a promise: `page = await Web.fetch(url)`.

So do not write `File`, `Dir` or `ENV` unless you were told the script
runs with I/O allowed; ask which functions the host gives instead.
