# Task: integer map keys, builtin calls on the stack, append into spare room

**Started:** 2026-11-12 20:30:00
**Ended:** 2026-11-12 23:30:00
**Strategy:** Performance
**Status:** Completed
**Complexity:** Medium
**Used Models:** Fable 5.2
**Token usage (Estimated):** 2M input, 60k output

## Objective
Take what two experiments pointed at into the runtime: integer keys of
maps and sets found by their number, builtins called with their
arguments where they are on the stack. Then make building a list
element by element linear, and let `map` fill a list of its full length.

## Progress
- [x] Integer keys of a map or a set found by their number (#385)
- [x] A builtin that keeps no slice of its arguments is called with
      them on the caller's stack (#386), and 28 more builtins marked so
      (#387)
- [x] `<<`, `push` and `append` write into the room behind the list's
      last element; `map` returns its state as the list (#388)

## Measurements
Apple M4, best of five, wall time.

| case | before | after |
|---|---|---|
| `out = out << x` to 1000 elements, 10,000 times | 5.809 s | 1.614 s |
| `a.map { }` over 1000 elements, 10,000 times | 0.754 s | 0.739 s |

The benchmark groups do not move with #388 (array 99.9%, enum 102.5%,
hash 95.4%, app 102.0%, vm 100.7% of the time before, within the noise
of one run each); no case of them appends in a loop. #387 was within
noise too.

Measured on the way, for the question of pausing at `await` only:

| case | time |
|---|---|
| a 30M-pass loop in a method | 0.888 s |
| the same with `await p if p` after the loop | 3.726 s |
| `each`, a step function, 20M block calls | 1.232 s |
| `reverse_each`, a Go loop | 1.514 s |
| `each` written in cameo with `yield` | 1.859 s |

## Findings
- A method with an `await` anywhere in it runs from cursors and takes
  four times as long. Pausing at `await` and `async` only would let it
  run as flat code.
- Step functions are the fastest way a builtin calls a block now, so
  making the rest of the block-calling builtins step functions costs
  nothing.
- Two experiments were not landed: strings of up to 8 bytes inline in
  the Value (branch `ir/x-inline-short-strings`, neutral overall) and a
  prototype with a heap of its own (branch `ir/x-bump-heap`, 3 to 4.6
  times faster on allocation scripts, of which Go's allocator is about
  a quarter).

## Left open
- `prepend`, `insert` and index assignment still copy the list.
- `vm_freezeobj` and `vm_call_send_iseq` are slower than on main.
