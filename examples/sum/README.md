# sum: add up numbers typed, pause with ^C, go on later

```sh
go run ./examples/sum
```

Type a number a line. `^C` pauses the script and writes its state to a
file; `^D` ends the input:

```
3
1: total 3
4
2: total 7
^C
paused; go on with: go run ./examples/sum /tmp/sum-1334916290.dump
```

The file holds the sum so far, so a later run goes on from it:

```
$ go run ./examples/sum /tmp/sum-1334916290.dump
5
3: total 12
ten
not a number: ten
^D
the sum of 3 numbers is 12
```

The same file can be continued again, as often as you like, and
`cameo inspect` shows what it holds: the variables `total` and `count`,
and the line the script waits for, a pending promise.

[sum.cameo](sum.cameo) adds the numbers. A script reads no keyboard, so
[main.go](main.go) gives it the module `Keyboard`, whose `line` returns a
promise of the next line. The script waits for it with `await`, and a
script that waits in `await` pauses at once. On `^C` the program calls
`prog.Pause()` and writes `prog.Dump()`.

To go on, the program makes a program of the file with `cameo.Resume`
and gives it `Keyboard` again: the state holds the script and its values
and nothing of the host. The line the script waited for is a pending
promise in the dump; the program finds it in the dump's `promises` and
settles it, through `prog.PendingPromise(id)`, with the next line typed.

`cameo resume` cannot continue the file: the `cameo` command has no
`Keyboard` to give the script.
