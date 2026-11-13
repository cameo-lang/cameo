# pause: stop a run, keep its state, continue it later

```sh
go run ./examples/pause
```

```
step 1: total 1
step 2: total 5
step 3: total 14
paused before line 9
the state is bytes now; a program would store them
step 4: total 30
step 5: total 55
done: 55
```

[steps.cameo](steps.cameo) counts through five steps and tells the Go
program after each how far it is. [main.go](main.go) pauses the run on
step 3 with `prog.Pause()`, takes its state with `prog.Dump()`, and
continues it in a second program with `cameo.Resume`. The state holds the
script and its values and nothing of the host, so the second program is
given the module `Host` again.

The site's [Pause and resume](https://cameo-lang.github.io/cameo/pause.html)
page has the same by hand, with `^C` and `cameo resume`.
