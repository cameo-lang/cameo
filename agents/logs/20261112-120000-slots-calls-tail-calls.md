# Task: BEGIN as jumps, slot locals, a call that allocates nothing, tail calls

**Started:** 2026-11-12 12:00:00
**Ended:** 2026-11-12 20:00:00
**Strategy:** Performance
**Status:** Completed
**Complexity:** High
**Used Models:** Fable 5.2
**Token usage (Estimated):** 4M input, 120k output

## Objective
Carry on from the comparison that left the top of the ir/ stack at 95%
of main's time: run a begin as jumps in flat code, keep locals in slots,
make a call allocate nothing. Then two requests made on the way: inline
short strings and symbols in the 16-byte Value and measure it, and make
tail calls a feature of the language, with a mode that keeps up to 256
frames for debugging.

## Progress
- [x] BEGIN as jumps: a static tree of scopes per flat code, which a
      completion walks outwards; a record per BEGIN on the activation
      (#379)
- [x] Slot locals: a layout per proc, scopes that hold slots, the top
      level's own layout, a block that captures what it reads (#380)
- [x] A call without allocation: contained procs use the frame and the
      scope of their machine, arguments stay on the caller's stack,
      release sets fields one by one (#381)
- [x] Tail calls: FLAG_TAIL from the compiler, the callee in place of
      the caller, a loop in place for contained methods, kept frames
      for the debugger and --keep-tail-frames (#382)
- [x] Inline short strings and symbols: built behind a switch and
      measured on the whole suite (see Outcome)

## Obstacles
- **A whole-struct reset cost more than the allocation it replaced.**
  With a frame and a scope embedded, `*m = machine{...}` in release was
  the top entry of the profile of a call (a typed copy of several
  hundred bytes with write barriers).
  **Resolution:** release sets each field by itself and clears the
  frame and the scope only when a call used them; a test compares a
  released machine with a new one by reflection, and fails on a field
  the test does not set.
- **By-name reads through a layout were slower at the top level.**
  loop_whileloop lost 20% once a scope looked a name up in a layout.
  **Resolution:** lowering a top-level statement reserves a slot for
  each variable in the top scope's own layout; the loop is now faster
  than main's.
- **A test hung.** A method that called itself in tail position to
  provoke SystemStackError became an endless loop with tail calls.
  **Resolution:** the test recurses outside tail position; the change
  of behaviour is in the pull request's notes and the language doc.
- **Kept tail frames were trimmed per call.** Counting the row and
  moving 256 pointers on every tail call made the mode 8 times slower.
  **Resolution:** each kept activation knows the length of its row, and
  a row is cut back in batches of a quarter.
- **An existing test file was overwritten.** env_test.go existed; a new
  file of the same name replaced it.
  **Resolution:** restored from git and the new tests appended; the
  three original tests pass unchanged against the new Env.

## Outcome
#379 to #382 on top of the stack. All 791 cases of benchmarks/ruby/works,
timed inside each Ruby, three at once, Apple M4; CRuby 3.4.1 and main
(7a0bcb8b) as timed for the comparison before, the stack at 850f6246:

- geometric mean against CRuby without YJIT: 179%, where the stack was
  220% and main is 231%;
- 78% of main's time in the geometric mean, faster than main in 623 of
  the 780 cases both ran, and 91% on the 48 cases CRuby takes 0.1 s or
  more for, where it was 109%;
- 99.1 s for one run of each case, where main takes 152.7 s;
- no output differs from CRuby's, and all 791 cases run.

app_fib 1.30 to 0.84 s (main 1.22 s), app_tarai 1.60 to 0.87 s,
vm_ensure 1.42 to 0.65 s, vm_yield 2.23 to 1.58 s, loop_whileloop 0.59
to 0.48 s. Still behind main: vm_freezeobj (201%), the send cases of
vm_call_send_iseq (142%), app_strconcat (135%).

Inline short strings and symbols (up to 8 bytes in the payload word,
the length in the mark the pointer word holds; the pointer word must
stay a pointer for Go's collector) were measured on the whole suite and
not landed: 100% in the geometric mean, 108% on the 48 large cases,
array_* 128%, vm_dstr* 124%, array_max_str 400%, string_* 91%. Reading
an inline string as a Go string copies its bytes to the heap, and a
string that points at its bytes costs nothing to make or to read. The
experiment is on the local branch ir/x-inline-short-strings.

Follow-up: a symbol as its interned number (needs a name table read
without a lock, and maps that hash a symbol by its number); a send
specialised in flat code, with the method cached at the instruction;
the block calls of a for loop, which still go by name.

## Insights
- A static table beats a runtime stack for handlers: which BEGIN and
  which loop an instruction is in is known when it is lowered, so
  entering a begin costs one instruction and an exception finds its
  handler by walking parents.
- "Cannot leak" is cheap to decide and worth a lot: a proc that names no
  other proc and starts no async call cannot have its frame or scope
  kept, and that one bit lets a call use storage that is reused.
- A Value that is already a pointer and a length has little to gain
  from holding short strings inline in Go: reading the bytes as a Go
  string needs them somewhere a pointer can reach.
- Measure the reset as well as the allocation. Pooling moved the cost
  of a call from the allocator to the code that clears the pooled
  object.
