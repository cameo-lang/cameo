# Task: Cut the allocation of a call and of a block's capture

**Started:** 2026-11-11 22:51:46
**Ended:** 2026-11-11 23:45:00
**Strategy:** Performance
**Status:** Completed
**Complexity:** Simple
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

Follow-up: the cost per instruction of the loop in run, step and exec.
A scope of a block's call that nothing kept could be reused as the
machine is.

## Insights
- The evaluator of the ir/ stack costs more per instruction than
  main's: a loop of `i += 1` takes 3.6 times main's time. Allocation
  was the first half of the gap; the rest is in the loop itself.
- With the garbage collector off the times stay the same on this
  machine: the collector runs on other cores. What a call costs is the
  allocation itself, and the clearing of what it allocates.
