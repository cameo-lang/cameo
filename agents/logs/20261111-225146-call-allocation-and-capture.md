# Task: Cut the allocation of a call and of a block's capture

**Started:** 2026-11-11 22:51:46
**Ended:** 2026-11-12 04:00:00
**Strategy:** Performance
**Status:** Completed
**Complexity:** Medium
**Used Models:** Fable 5.2
**Token usage (Estimated):** 3M input, 60k output

## Objective
Continue the performance work on the ir/ stack from the four areas its
CPU profiles named. Two were done and unmeasured (#367 mayMutate, #368
map literals); two were open: the allocation of every method and block
call, and the copy of every visible variable a block makes when it is
written. Then compare main's interpreter, the top of this stack and
CRuby on the benchmarks.

## Progress
- [x] Measured #367 and #368 on an Apple M4, put the tables in their
      Test sections and marked them ready
- [x] Call allocation: machines are reused, an operator on two integers
      or two floats makes no argument list, a lambda call's frame and
      scope are one allocation, a block and its body are one allocation
- [x] Parameters: a list per interpreter by unit and index, and plain
      parameters bound without the general pass (#369)
- [x] Block capture: a snapshot shares the captured scope it sits in
      (#370)
- [x] The three-way benchmark: all 791 cases of benchmarks/ruby/works
      on CRuby 3.4.1 without and with YJIT, main (7a0bcb8b) and the top
      of the stack (e343c790)
- [x] Names as numbers (object.Sym): scopes and operators (#371), the
      method of a SEND (#372)
- [x] An instruction per arithmetic and comparison operator, the SEND
      form removed, IR version 2 (#373)
- [x] A Value of two words, 16 bytes where it was 24 (#374)
- [x] Flat code: a proc lowered to jumps when it is first called, CASE
      as jumps, BEGIN and pattern matches from cursors in place (#375)
- [x] Follow-ups the comparison found: reading a payload kind with one
      comparison (#376), a map lookup without its key's string (#377),
      to_s by its Sym (#378)

## Obstacles
- **Issue:** a CPU profile from runtime/pprof on macOS showed the
  interpreter's loop at 12% of app_fib and the Go scheduler and garbage
  collector at the rest, which GOGC=off disproved: the wall time did not
  change.
  **Resolution:** `sample <pid>` gives the functions on top of the main
  thread's stack; allocation was about half of them.
- **Issue:** the first run of a binary that was just built is slower by
  up to 0.3 s, which made the lambda case look 50% slower once.
  **Resolution:** every timing is the fastest of five alternating runs.
- **Issue:** app_lc_fizzbuzz was handed over as 130 s on this stack
  against 70 s on main, measured in a container.
  **Resolution:** on this machine it took 40.7 s before the two changes
  and 20.6 s after; main takes 63 s.
- **Issue:** the BINARY_ instructions first kept the SEND form running
  for dumps written before them.
  **Resolution:** dumps are not kept across versions; the form, its flag
  and its test were removed and the IR version raised.
- **Issue:** with the kind of a Value computed from its two words, three
  cases were 4 to 12% slower.
  **Resolution:** the kinds that live in the payload are tested by one
  comparison of the pointer word with the kind's mark.

## Outcome
#369 and #370 on top of the stack. From ir/48-map-put to the top:
app_fib 3.86 to 2.00 s, app_lc_fizzbuzz 40.7 to 20.6 s, most call-heavy
cases 20 to 30% faster.

The three-way run, timed inside each Ruby, three cases at once: the
geometric mean of a case's time against CRuby without YJIT is 231% for
main and 313% for the top of the stack. The stack is slower than main in
724 of the 780 cases all three ran, 136% of main's time in the
geometric mean, and 163% on the 49 cases CRuby takes 0.1 s or more for.
The largest gaps are loops and calls with no allocation left to remove:
loop_whileloop 365% of main, vm_ensure 423%, app_strconcat 268%.

After the benchmark: names as numbers and the method lookup by number
changed the times by 0 to 4%, the operator instructions by 2 to 5%. The
Value of 16 bytes took 37% off hash_literal_small8, 22% off
array_values_at_range, 10% off hash_aref_fix and 6% off app_fib.

With flat code and the follow-ups, the same comparison on the top of the
stack (6c797869): 220% of CRuby in the geometric mean where main has
231%; 95% of main's time, faster than main in 487 of 780 cases, and
109% on the 48 cases CRuby takes 0.1 s or more for. No output differs
from CRuby's.

Follow-up: BEGIN as jumps (vm_ensure takes twice main's time), a call
that does not allocate its frame, scope and argument list (app_fib is
6.7 times CRuby's time), and the block calls of times and for
(loop_times and loop_for take 1.5 times main's time).

## Insights
- The evaluator of the ir/ stack costs more per instruction than
  main's: a loop of `i += 1` takes 3.6 times main's time. Allocation
  was the first half of the gap; the rest is in the loop itself.
- With the garbage collector off the times stay the same on this
  machine: the collector runs on other cores. What a call costs is the
  allocation itself, and the clearing of what it allocates.
