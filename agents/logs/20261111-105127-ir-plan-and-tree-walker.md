# Task: plan the IR and pausing at any instruction; remove the closure compiler

**Started:** 2026-11-11 10:51:27
**Ended:** 2026-11-11 12:10:00
**Strategy:** Refactoring
**Status:** Completed
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
- [x] eval: the tree walker evaluates calls; `compile.go` goes
- [x] `make check` on each step; the conformance run on the last, compared with `main`

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

- **Issue:** `ruby tests/conformance/run.rb` stops with "invalid byte
  sequence in US-ASCII" where the locale is not UTF-8, and with a UTF-8
  locale exits 1 on `main`: 241 scripts under `benchmarks/ruby/works` are
  refused by the checker ("a block cannot rebind …").
  **Resolution:** Ran it with `LC_ALL=C.UTF-8` on `main` and on the branch
  and compared the output: equal. The stale scripts are unresolved and not
  part of this task.

## Outcome
Three pull requests, stacked on `main`: the design document (#317), the
tree walker for every node but calls (#318), and calls in the tree walker
with `compile.go` deleted (#319). The next step of the plan is `make
proto` and the envelope as a `.proto` file.

## Insights
The two evaluators shared one cache, so the closure compiler could go one
kind of node at a time with the suite green in between. The IR evaluator
can come in the same way, a method or block at a time.
