# Task: every exception is a protobuf message

**Started:** 2026-10-21 23:30:00
**Ended:** 2026-10-22 00:40:00
**Strategy:** Feature (TDD)
**Status:** Completed
**Complexity:** Simple
**Used Models:** Opus
**Token usage (Estimated):** 350k input, 60k output

## Objective
Make every exception a protobuf message: Ruby's exception classes become
messages of their message, line, cause and what they carry, and a script's
exception class declares typed fields, sets them in an `initialize` that
runs on `new` and `raise`, and copies with `with`. Exceptions cross to
`await` and to the Go host as messages; `cameo proto` writes an exception
class. Raise and rescue in a loop must not slow down much.

## Progress
- [x] Baseline: specs of core/exception, language and core/kernel; a raise/rescue loop timed
- [x] object.Exception with Cause, Fields, Building; Key includes the fields
- [x] protogen: the parts of an exception's message, its descriptor and .proto text, the envelope's Object for exceptions
- [x] stdlib: initialize of Exception, NameError, KeyError and SystemExit in place of their new
- [x] eval: Exception.new runs initialize; fields set and read; with; to_proto, from_proto, proto
- [x] eval: the cause from the rescued exception, `cause:`, a bare raise in a rescue clause
- [x] async: a worker's exception and `fail` cross as the message
- [x] host: RunError.Exception, exceptions in ToGo and ToValue
- [x] Tests, examples/exception_fields, doc/design/exceptions.md, cameo.md, cameo-vs-ruby.md
- [x] Specs after: the same per file and per failing example

## Obstacles
- **Issue:** a script's exception class could declare fields, but reading one panicked ("exception read as instance") and its initialize never ran.
  **Resolution:** Exception.new runs initialize, and readers read an exception's fields.
- **Issue:** looking initialize up on every `raise SomeError` cost a raise loop about a tenth of its time.
  **Resolution:** raise reads the classes' own method tables up to Exception.
- **Issue:** a NameError's receiver is often the top-level main, which the envelope encodes but no other interpreter can read back.
  **Resolution:** a part the reading side cannot decode is left out, as one the envelope cannot carry is.
- **Issue:** stashing the change to benchmark HEAD left the new files in place, which did not compile against the old tree.
  **Resolution:** benchmarked a `git archive` copy of HEAD in the scratch directory.
- **Issue:** timings on the shared machine (load 8 to 9) vary by 20% between runs.
  **Resolution:** interleaved runs of both binaries and compared the minimum.

## Outcome
Exceptions are messages; script exception classes have fields and run
their initialize. A raise/rescue loop takes about 8% more CPU (minimum of
9 runs). Open: `raise E, status: 404` still drops the keywords (Ruby makes
them the message); a method that calls `initialize` on an existing
instance changes it, which this change refuses for exceptions only.

## Insights
Building the message only when an exception crosses keeps raise and
rescue free of protobuf, the same way an instance builds its message on
first use.
