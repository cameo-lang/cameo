# Task: a Proc in the envelope, and a return that has nowhere to go

**Started:** 2026-11-11 21:45:00
**Ended:** 2026-11-11 22:50:00
**Strategy:** Bug Fix, then Feature (TDD)
**Status:** Completed
**Complexity:** Simple
**Used Models:** Fable 5.2
**Token usage (Estimated):** 400k input, 45k output

## Objective
Steps 11 and 12 of `doc/design/ir.md`: a frame that knows when its call
ended, so a proc's `return` after that raises; and `cameo.Value` carrying
a Proc as a closure, the proc of a unit and its binding.

## Progress
- [x] `Frame.ended`, set by `invoke` and by a lambda's call; RETURN raises
      LocalJumpError for an ended frame (failing test first)
- [x] `Constant` in `ir.proto`, so the IR imports nothing and
      `value.proto` can import it
- [x] `Closure` and `Binding` in `value.proto`; `Value.closure`
- [x] `protogen.ProcCodec`, `Encoder.WithProcs`, `Decoder.WithProcs`
- [x] `eval/closure.go`: `EncodeProc`, `DecodeProc`, what a proc's code
      uses of its binding, the registry of units by digest
- [x] `to_value_proto` on a Proc; `cameo proto` writes `cameo/ir.proto`
- [x] Tests: procs travel to an interpreter that never saw their source

## Obstacles
- **Issue:** `value.proto` needs `Unit` and `ir.proto` used `Value` for
  literals: an import cycle.
  **Resolution:** A literal is a `Constant`, a message of `ir.proto` with
  the scalar kinds alone. Code no longer depends on any value of a run.
- **Issue:** A proc captures every local in sight. One Enumerator among
  the top-level variables would keep every proc of a script from
  travelling.
  **Resolution:** Only the locals the code reads travel, found by walking
  the proc and the blocks written in it.
- **Issue:** A symbol a proc captured reads back as a String, so
  `mode == :fast` is false after the proc travelled. The envelope has
  always turned symbols into strings, and the pause dump does the same to
  a resumed script's variables.
  **Resolution:** Unresolved: it is the language's rule for the envelope.
  A test states it, the design document lists it as open, and the human
  is asked.
- **Issue:** `examples/roundtrip.cameo` held `-> {}.to_value_proto` to
  raising.
  **Resolution:** The example now sends a proc through `to_value_proto`
  and back, and shows the refusal on an Enumerator.

## Outcome
A Proc serialises. `async`, the host's `Encode` and the pause dump still
use the encoder that refuses one; each is switched in a pull request of
its own.

## Insights
"Persisting a running program" asks more of the envelope than "handing a
value to a host" did: what the envelope may lose in the second case (the
difference between a symbol and a string) changes the program in the
first.
