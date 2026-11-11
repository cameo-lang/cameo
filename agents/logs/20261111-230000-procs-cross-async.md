# Task: a Proc crosses async, to a worker and back

**Started:** 2026-11-11 23:00:00
**Ended:** 2026-11-11 23:40:00
**Strategy:** Feature (TDD)
**Status:** Completed
**Complexity:** Simple
**Used Models:** Fable 5.2
**Token usage (Estimated):** 250k input, 25k output

## Objective
Step 13 of `doc/design/ir.md`: with a Proc in the envelope, `async` takes
one as an argument and a worker returns one.

## Progress
- [x] `startCall` encodes with `ProcEncoder(false)`: a worker has the units
- [x] `handOver` replaces `travels`: a result that holds a Proc is read
      from its wire form by the interpreter that awaits it
- [x] `resolve` takes a Proc the same way
- [x] `ruby/lib/cameo/runner.rb` lets a Proc through, alone and inside an
      Array or a Hash
- [x] Examples, the site and the design brought to the rule

## Obstacles
- **Issue:** A worker's result was handed to the awaiting script as the
  value itself, after a check that it could be encoded. A Proc handed over
  so would keep the worker's top-level self and the worker's classes.
  **Resolution:** A value that holds a Proc is encoded on the worker and
  decoded by whoever awaits, through the promise's `later`, which existed
  for results a host settles.
- **Issue:** Two examples and the site showed `async` refusing a Proc, and
  `examples/async/07_copies.rb` runs on CRuby too, where
  `Cameo::Runner` copies with `Marshal`, which has no copy of a Proc.
  **Resolution:** The Ruby adapter passes a Proc as it is and says in its
  comment what that means on Ruby. The examples show a Proc crossing, and
  a class refused. Both runtimes print the same for 07.

## Outcome
`async w, :apply, ->(x) { x * factor }, list` works, and a method a
worker runs may return a proc. A proc a builtin made and a class are
still refused, with the messages they had.
