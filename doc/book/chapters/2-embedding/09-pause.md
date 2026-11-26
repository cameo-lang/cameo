## Pause and resume {#ch-pause}

::: source
from `doc/gh_pages/pause.html`
:::

A running script can be stopped where it is, inside a method or a loop, and
its state written out as bytes. Another process continues it from those bytes,
later or on another machine.

1.  [By hand: \^C and cameo resume](#pause-hand)
2.  [From a Go program: Pause, Dump and Resume](#pause-go)
3.  [What the state holds](#pause-holds)

The state is a protobuf message: the script's code, the calls it is inside,
its variables and constants, its classes and its pending promises. That works
because every value is immutable and every object is a protobuf message
already.

### By hand: \^C and cameo resume {#pause-hand}

Take a script that runs for a while, `count.cameo`:

``` {.ruby data-name="count.cameo"}
def slow_square(n)
  i = 0
  while i < 3_000_000
    i += 1
  end
  n * n
end

total = 0
n = 1
while n <= 40
  total += slow_square(n)
  puts "#{n}: #{total}"
  n += 1
end
puts "done: #{total}"
```

Run it and press `^C`. cameo stops the script before its next instruction,
writes the state to a temporary file and says how to go on. Its exit status is
3.

``` sh
cameo count.cameo
```

``` output
1: 1
2: 5
...
10: 385
11: 506
^C
cameo: pausing; press ^C again to end the script
cameo: paused before line 3 of count.cameo
cameo: resume with: cameo resume /tmp/cameo-count-676897826.dump
```

`cameo inspect` shows what the file holds: where the script stopped, the calls
it was inside, and its variables.

``` sh
cameo inspect /tmp/cameo-count-676897826.dump
```

``` output
script: count.cameo
paused: before line 3
inside:
  slow_square, line 3
variables:
  total = 506
  n = 12
constants:
  ARGV = []
classes: none
promises: none
```

`cameo resume` continues the script there, in a new process. The file is not
used up: resuming it again starts from the same point.

``` sh
cameo resume /tmp/cameo-count-676897826.dump
```

``` output
12: 650
13: 819
...
40: 22140
done: 22140
```

A second `^C` while cameo is pausing ends the script, and
`cameo --no-pause count.cameo` makes the first one end it, as it ends any
program.

### From a Go program: Pause, Dump and Resume {#pause-go}

A Go program that embeds cameo does the same with three calls. `prog.Pause()`
asks the run to stop and may be called from any goroutine; `Run` then returns
`cameo.ErrPaused`. `prog.Dump()` returns the state as bytes, to store wherever
the program stores things. `cameo.Resume(data, env)` makes a new program of
them, with `env` as its host as `cameo.Load` takes it, and its `Run` goes on
where the first one stopped.

The example
[examples/pause](https://github.com/cameo-lang/cameo/tree/main/examples/pause)
runs this script, which tells the Go program after each step how far it is:

``` {.cameo .hosted file="examples/pause/steps.cameo" data-title="steps.cameo"}
# Five steps of work. After each, the script tells the Go program how
# far it is; the Go program may pause the run there.
total = 0
n = 1
while n <= 5
  total += n * n
  puts "step #{n}: total #{total}"
  Host.reached(n)
  n += 1
end
puts "done: #{total}"
```

The Go program pauses the run on step 3, takes the state, and continues it in
a second program:

``` {.go file="examples/pause/main.go" data-title="main.go"}
// Command pause stops a running script, keeps its state as bytes, and
// continues it later in another interpreter, as a program would that
// stores the bytes in a database and goes on after a restart.
//
//  go run ./examples/pause
//
// The script counts through five steps and calls Host.reached after
// each. On step 3 the Go program asks the run to pause: Run returns
// ErrPaused, Dump gives the state, and Resume makes a new program of it
// that goes on with step 4.
package main

import (
    _ "embed"
    "errors"
    "fmt"
    "io"
    "os"

    "github.com/cameo-lang/cameo/src/cameo"
    "github.com/cameo-lang/cameo/src/cameoio"
)

//go:embed steps.cameo
var script string

// host gives prog the module Host, whose reached calls stop with the
// step the script is at.
func host(prog *cameo.Program, stop func(step int64)) {
    prog.Module("Host").Def("reached", func(args []any, kwargs map[string]any, block *cameo.Block) (any, error) {
        if len(args) != 1 {
            return nil, fmt.Errorf("reached takes one argument, not %d", len(args))
        }
        step, ok := args[0].(int64)
        if !ok {
            return nil, fmt.Errorf("reached takes an Integer, not %T", args[0])
        }
        stop(step)
        return nil, nil
    })
}

// firstHalf runs the script until it has reached step, pauses it there
// and returns its state.
func firstHalf(step int64, stdout, stderr io.Writer) ([]byte, error) {
    prog, err := cameo.LoadFromString("steps.cameo", script, cameoio.Streams(stdout, stderr))
    if err != nil {
        return nil, err
    }
    host(prog, func(at int64) {
        if at == step {
            // Pause may be called from any goroutine: a timer, a
            // signal handler, a request to shut down.
            prog.Pause()
        }
    })
    if err := prog.Run(); !errors.Is(err, cameo.ErrPaused) {
        return nil, fmt.Errorf("the script did not pause: %v", err)
    }
    dump, err := prog.Dump()
    if err != nil {
        return nil, err
    }
    fmt.Fprintf(stdout, "paused before line %d\n", dump.Line)
    return dump.Data, nil
}

// secondHalf continues the run whose state is data, to its end.
func secondHalf(data []byte, stdout, stderr io.Writer) error {
    prog, err := cameo.Resume(data, cameoio.Streams(stdout, stderr))
    if err != nil {
        return err
    }
    // The state holds the script and its values, and nothing of the
    // host: the module is given again.
    host(prog, func(int64) {})
    return prog.Run()
}

func main() {
    data, err := firstHalf(3, os.Stdout, os.Stderr)
    if err == nil {
        fmt.Println("the state is bytes now; a program would store them")
        err = secondHalf(data, os.Stdout, os.Stderr)
    }
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

``` sh
go run ./examples/pause
```

``` output
step 1: total 1
step 2: total 5
step 3: total 14
paused before line 9
the state is bytes now; a program would store them
step 4: total 30
step 5: total 55
done: 55
```

The same `Pause` stops a script on a timer, on a signal, or when the service
that runs it shuts down. A paused program that is not dumped can also go on:
calling `Run` again continues it.

### What the state holds {#pause-holds}

-   **The script and its values**: the code, the stack of calls, local
    variables, constants, classes and methods, blocks and procs, and promises
    that are still pending.
-   **Nothing of the host**, but what it keeps. A module of Go functions is
    given to the resumed program again, as the example does, and a promise the
    host had not settled is reached with `prog.PendingPromise(id)`. An object
    of the host that the run needs, such as the deck a module deals from, is
    kept with `prog.Keep(name, obj)`: the dump stores the bytes its
    `Dehydrate` returns, and `Keep` of the same name on the resumed program
    hands them to its `Rehydrate`. `Run` refuses to go on while the dump holds
    state no `Keep` claimed.
-   **No handles.** A value that names something outside the script, such as a
    `Random`, is left out, and `dump.Missing` lists what was.
-   **Inside a block** of `each`, `map`, `times`, `loop` and the other common
    iterators a run pauses at once. Inside the block of a builtin that cannot
    stop halfway yet, it pauses when that builtin returns.

The design document [doc/design/pause.md](#ch-design-pause) has the format,
the list of those builtins, and the limits.
