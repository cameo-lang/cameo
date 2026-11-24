## pause: stop a run, keep its state, continue it later {#ch-example-pause}

::: source
from `examples/pause/README.md`
:::

``` sh
go run ./examples/pause
```

    step 1: total 1
    step 2: total 5
    step 3: total 14
    paused before line 9
    the state is bytes now; a program would store them
    step 4: total 30
    step 5: total 55
    done: 55

[steps.cameo](https://github.com/cameo-lang/cameo/blob/main/examples/pause/steps.cameo) counts through five steps and tells the Go program
after each how far it is. [main.go](https://github.com/cameo-lang/cameo/blob/main/examples/pause/main.go) pauses the run on step 3 with
`prog.Pause()`, takes its state with `prog.Dump()`, and continues it in a
second program with `cameo.Resume`. The state holds the script and its values
and nothing of the host, so the second program is given the module `Host`
again.

The site's [Pause and resume](#ch-pause)
page has the same by hand, with `^C` and `cameo resume`.
