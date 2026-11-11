# Task: plan the IR and pausing at any instruction; remove the closure compiler

**Started:** 2026-11-11 10:51:27
**Ended:** in progress
**Strategy:** Refactoring
**Status:** In Progress
**Complexity:** Medium
**Used Models:** Fable 5.2
**Token usage (Estimated):** 600k input, 60k output

## Objective
A running cameo program is to be persistable: its bindings, its procs, its
methods, and in the end its call stacks. The plan is an IR that protobuf
carries and an evaluator that keeps no state on the Go stack. This task
records the plan and the decisions taken with the human in
`doc/design/ir.md`, and does the first step that depends on none of them:
the closure compiler goes, so that one evaluator is ported to the IR.

## Progress
- [x] Read the evaluator, the scopes, procs, async, the dump and the envelope
- [x] Open questions put to the human, fourteen, all answered
- [x] `doc/design/ir.md`: the decisions, the IR, the stack of pull requests
- [x] eval: the tree walker evaluates every node but calls
- [ ] eval: the tree walker evaluates calls; `compile.go` goes

## Obstacles
- **Issue:** The request named the binding as what cannot be serialised.
  `eval.Env` is already names and values; what cannot travel is the code a
  Proc points at, the frame, the Go functions builtins hold, and the Go
  stack.
  **Resolution:** The design document lists the four, and the plan addresses
  each.
- **Issue:** Builtins that call a block cannot be rewritten in cameo to get
  their Go frames off the stack: a cameo block cannot rebind a variable of
  the scope around it, so `each { |x| out = out + [x] }` accumulates nothing.
  **Resolution:** The human chose resumable step functions in Go, whose
  state between two calls of the block is a cameo value.

## Outcome
In progress.
