## Pause and resume {#ch-agents-pause}

::: source
from `src/agentdocs/topics/pause.md`
:::

A running script can be stopped where it is, inside a method or a loop, and
its state written out as bytes: its code, the calls it is inside, its
variables, its classes and its pending promises. Another process goes on from
those bytes, later or on another machine. That works because every value is
immutable and every object is a protobuf message.

On the command line, `^C` pauses a script and names the file;
`cameo resume <file>` goes on, and `cameo inspect <file>` shows where it
stopped and its variables. `cameo --no-pause` makes `^C` end the script
instead.

From Go:

``` go
go func() { done <- prog.Run() }()
prog.Pause()                        // from any goroutine; Run returns cameo.ErrPaused
dump, err := prog.Dump()            // dump.Data: the state as bytes
cont, err := cameo.Resume(dump.Data, cameoio.DefaultEnv()) // the host, as Load takes it
// give cont the host's modules again, then:
err = cont.Run()                    // goes on where the first run stopped
```

-   A dump holds no host module: the resuming program defines them again
    before `Run`.
-   A host object the run needs, such as the deck a module deals from, is a
    `cameo.Stateful` (`Dehydrate() ([]byte, error)`,
    `Rehydrate([]byte) error`) the host keeps with `prog.Keep(name, obj)`. The
    dump stores its bytes; on the resumed program, `cont.Keep(name, fresh)`
    hands them back before it returns. `Run` refuses while the dump holds
    state no `Keep` claimed. Order: `Resume`, `Module` and `Keep`, settle
    promises, `Run`.
-   A run waiting on a host promise pauses at its `await`; the resumed run
    finds the promise pending, `cont.PendingPromise(id)`, for the host to
    resolve.
-   `cameo.Resume(data, env, cameo.OnlyCode(file, src))` refuses a dump whose
    code is not the script given, so a dump cannot be edited to run other
    code.

More: <https://cameo-lang.github.io/cameo/pause.html>.
