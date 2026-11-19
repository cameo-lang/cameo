# cameo for agents

cameo is a small language with Ruby's syntax, run by an interpreter
written in Go. A script is Ruby with one rule:

**Write Ruby, and leave out everything that changes a value in place.**

Every value is immutable, so a program is values computed from values:
`a += [1]`, not `a << 1`; `with(total: total + 1)`, not `@total += 1`.
Most of Ruby's core classes and many of its libraries are there. A script
does no I/O unless whoever runs it allows it, and a running script can be
paused, written out as bytes, and resumed in another process.

Check a script before you hand it over:

```sh
cameo check script.cameo     # every error, with its line, without running
cameo script.cameo           # run it
cameo ir script.cameo        # the instructions it compiles to
```

`cameo check` names the construct it refuses and what to write instead,
with a link to a page that explains it (`https://15b.it/<code>`). Fix what
it says and check again; do not guess around it.

The full documentation is at https://cameo-lang.github.io/cameo/.
