# Task: an instance of a reopened class resumes

**Started:** 2026-11-11 16:05:00
**Ended:** 2026-11-11 16:20:00
**Strategy:** Bug Fix
**Status:** Completed
**Complexity:** Simple
**Used Models:** Fable 5.2
**Token usage (Estimated):** 120k input, 8k output

## Objective
A dump that held an instance made before its class was reopened did not
resume: `no class A (digest …) is loaded`. The class of such an instance
is one no constant leads to, and the dump of step 16 carries a class only
when something of the dump refers to it.

## Progress
- [x] A test that fails: an instance and an exception made before their
      classes were reopened, through two pauses
- [x] `protogen.Encoder.WithClasses`: the encoder tells the dump the class
      of every instance and exception it encodes
- [x] `Interp.FindClass` finds a class of the dump by its name and digest
- [x] `doc/design/pause.md`

## Obstacles
- **Issue:** The test compared `{k: old}[:k]` after the resume, which is
  nil: a symbol key reads back as a String. That is the open question on
  symbols in `doc/design/ir.md`, not this change.
  **Resolution:** The test uses a String key.

## Outcome
An instance keeps the class it was made of across a pause, also when the
class was reopened since. A closure still names the classes of its
binding by name alone.
