# Task: the IR's messages, its instruction set and the listing

**Started:** 2026-11-11 16:00:00
**Ended:** 2026-11-11 16:40:00
**Strategy:** Feature (TDD)
**Status:** Completed
**Complexity:** Simple
**Used Models:** Fable 5.2
**Token usage (Estimated):** 250k input, 30k output

## Objective
Step 6 of `doc/design/ir.md`: write the IR as protobuf messages with its
operations, and print a unit as an assembler listing, before any compiler
or evaluator exists.

## Progress
- [x] Read the rest of the tree walker (classes, fields, patterns, async)
      for what each language form needs from an instruction
- [x] `src/cameopb/cameo/ir.proto`: Unit, Proc, Params, Code, Seq, Arg,
      Pattern, FieldDecl, Flag and Op, each operation with its stack effect
- [x] `src/ir`: `Listing`, `Version`
- [x] Tests: two listings of units built by hand, a round trip through the
      wire form

## Obstacles
- **Issue:** A nested message `Pattern.Bind` and the oneof field `bind`
  both generate the Go type `Pattern_Bind`; protoc-gen-go then names the
  oneof's wrapper `Pattern_Bind_`.
  **Resolution:** The nested messages have names of their own: Local, As,
  Array, Hash, Either, Search.
- **Issue:** The envelope turns a symbol into a string, so a symbol literal
  cannot be a `CONST`.
  **Resolution:** `SYM(name)`.
- **Issue:** A call with a splat needs its arguments counted at run time,
  and a marker on the stack would not be a cameo value.
  **Resolution:** Such arguments are built as one list with `LIST`,
  `APPEND` and `SPLAT`, and the call has the flag `argv`.
- **Issue:** `&.` must not evaluate the arguments when the receiver is nil.
  **Resolution:** `SAFE{rest}` runs the argument code and the `SEND` only
  for a receiver that is not nil.

## Outcome
The IR is defined and printable. The instruction set is what the compiler
of the next steps emits; it may still change with them, and the IR version
stays 1 until a dump depends on it.

## Insights
Reading every function of the tree walker that takes an AST node gave the
instruction set: each is one instruction, and its parameters are the
instruction's arguments, flags and lists.
