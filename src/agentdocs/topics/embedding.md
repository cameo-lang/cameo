# Embedding cameo in a Go program

The package `github.com/cameo-lang/cameo/src/cameo` loads a script, runs
it, calls its methods and converts values both ways.

```go
prog, err := cameo.Load("point.cameo", cameoio.DefaultEnv()) // read, parse and check
if err != nil { return err }
if err := prog.Run(); err != nil { return err }  // RunContext(ctx) stops at ctx's end
hello, err := prog.Call(nil, "greet", "Go")      // a top-level method; "Point" for a class method
```

The last argument is a `cameo.Env`, all of the host the program reaches:
the streams its `puts` and `warn` write to, the script files `LoadFile`
reads, and the files a script may read and write. Package `cameo` touches
no file or stream of the host itself. The package
`github.com/cameo-lang/cameo/src/cameoio` has the Envs:

- `cameoio.DefaultEnv()`: the machine's files, from the working
  directory, and the process's standard streams;
- `cameoio.Streams(stdout, stderr)`: output alone, for a host that hands
  cameo the source, and for tests;
- `cameoio.New(fsys, dir, stdin, stdout, stderr)`: the afero file system
  and streams the host chooses.

A program that does not import `cameoio` uses none of cameo's default
I/O. `cameo.LoadFromString(file, src, env)` loads a script the program holds
as a string.

A host gives a script what it may reach as a module of Go functions,
defined after `Load` and before `Run`:

```go
web := prog.Module("Web")
web.Def("fetch", func(args []any, kw map[string]any, block *cameo.Block) (any, error) {
	p := prog.Promise()             // the script awaits it: page = await Web.fetch(url)
	go func() {
		body, err := get(args[0].(string))
		if err != nil {
			p.Fail(err)
			return
		}
		p.Resolve(body)
	}()
	return p, nil
})
```

- A function gets plain Go values: `int64`, `float64`, `string`, `bool`,
  `[]any`, `map[string]any` for a Hash of String or Symbol keys, a
  `*cameo.Block` for a Proc. What it returns goes back the same way; an
  error raises a `RuntimeError` with its text, a `*cameo.Exception` its
  own class.
- Workers that `async` started call host functions on goroutines of
  their own, so a function may run on several at once.
- Return a promise for anything slow: the script can start several and
  await them together, and a run that waits on one can be paused (topic
  `pause`).
- A script does no I/O unless the program allows it (topic `io`).
- `cameo.Run(path, env)` is `Load` and `Run` in one call, as the `cameo`
  command does, and `cameo.RunFromString(file, src, env)`
  `LoadFromString` and `Run`.

More, with the mapping of every value and protobuf messages both ways:
https://cameo-lang.github.io/cameo/go.html.
