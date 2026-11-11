# Task: the design of the activation stack

**Started:** 2026-11-11 16:25:00
**Ended:** 2026-11-11 16:45:00
**Strategy:** Feature (TDD), its design step
**Status:** Completed
**Complexity:** Simple
**Used Models:** Fable 5.2
**Token usage (Estimated):** 200k input, 12k output

## Objective
Step 18 of `doc/design/ir.md`, the first of stack B: say how a run's
position becomes data, so that a run pauses inside a statement.

## Progress
- [x] Read `vm.go`, `vm_control.go`, `call.go` and `env.go` for what is on
      the Go stack today
- [x] `doc/design/ir.md`, "The activation stack": activations and
      cursors, calls and unwinding, where a run can pause, step
      functions, the dump
- [x] The list of stack B reordered and renumbered

## Obstacles
- **Issue:** The plan had every builtin that calls a block become a step
  function before a run could pause inside a statement. That is some
  hundreds of builtins before the first visible result.
  **Resolution:** A builtin that is no step function still works: its
  call back into cameo runs a nested loop. A run pauses before an
  instruction when no nested loop is running, so each group of builtins
  that becomes step functions widens where a run pauses, and the pause
  itself comes third in the stack, not seventh.
- **Issue:** The plan gave scopes identity in the dump for closures to
  share. cameo's blocks capture values, so a closure shares nothing with
  the scope it was written in; only a `for` body does.
  **Resolution:** Scopes and frames are tables of the dump for the
  activations that share them. A closure keeps its binding by value, and
  names a frame only while that frame's call runs.
- **Issue:** Step 18 of stack A, a class with a proc field as a message,
  needs to say where the closure's code travels: an instance's message is
  kept on the instance and has no envelope of its own.
  **Resolution:** Moved into stack B as step 28 with the answer the dump
  suggests: the field names its code, and the envelope or the dump
  carries the unit.

## Outcome
The design is in `doc/design/ir.md`. Next is step 19: the lists inside an
instruction run on cursors.
