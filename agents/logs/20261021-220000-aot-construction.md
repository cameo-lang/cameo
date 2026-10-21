# Task: construction in compiled code

**Started:** 2026-10-21 22:00:00
**Ended:** 2026-10-21 23:30:00
**Strategy:** Feature (TDD)
**Status:** Completed
**Complexity:** Simple
**Used Models:** Fable
**Token usage (Estimated):** 120k input, 40k output

## Objective
Class.new in compiled code, so that a method may build the instances it
works on and a loop over instances compiles whole.

## Progress
- [x] eval.ClassNamed and eval.Construct: the class by its top-level constant; an instance built as Class.new builds it, every value converted and checked, ErrDeopt when one does not fit
- [x] Checker: Class.new through an initialize of field assignments alone, with its parameters typed by the fields they are assigned to; keyword construction without an initialize; an Integer accepted for a Float field; methods may return instances
- [x] Generator: arguments first, then a block with initialize's parameters as locals, the fields into a slice by index, eval.Construct with the class cached in the state
- [x] Tests: eval, checker reasons, generator output, examples/aot/construction.rb against the interpreter
- [x] Conformance 68 same; make check; tests/performance/06-construction

## Obstacles
- **Issue:** The block for initialize's parameters declared `var err error`, which clashed with an `err` a field value's helper call had declared inside the same block.
  **Resolution:** The construction's result goes to a fresh temporary with `:=`, and is copied out of the block.
- **Issue:** `Class.new(n: n)` was refused before construction saw it: the keyword check for ordinary calls came first.
  **Resolution:** Construction is dispatched before that check.
- **Issue:** A literal argument became a local of Go type int, where int64 was wanted.
  **Resolution:** Literals that become locals are written `int64(...)`.
- **Issue:** bodies.rb passed a Float to a method's parameter, which the compiler does not type yet.
  **Resolution:** The step size is an Integer per time unit; call-site parameter typing is the next step.

## Outcome
vectors: 2.216 s interpreted to 0.231 s compiled, four million instances;
bodies: 3.462 s to 0.180 s. Construction is now what those loops cost.

## Insights
Reusing the interpreter's coerce through eval.Construct kept the field
semantics in one place; the price is a Value slice and a check per field,
which a typed build step can remove later.
