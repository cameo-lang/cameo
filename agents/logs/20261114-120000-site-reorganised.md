# Task: reorganise the site around what a reader wants to do

**Started:** 2026-11-14 10:00:00
**Ended:** 2026-11-14 13:45:00
**Strategy:** Feature
**Status:** Completed
**Complexity:** Medium
**Used Models:** Fable 5.2
**Token usage (Estimated):** 500k input, 40k output

## Objective
The site had five pages: an overview, Ruby, async, Go and native code.
Add what was missing, in the order a reader needs it: language examples,
embedding, gRPC, pause and resume, async, I/O, Ruby vs cameo, numbers.

## Progress
- [x] `examples.html`: seven programs with their output, all run by the test
- [x] `pause.html`: by hand with ^C, and from Go with examples/pause
- [x] `io.html`: the default, `--io`, `AllowIO` and `FileSystem`
- [x] `grpc.html`: examples/grpc against grpcb.in
- [x] `numbers.html`: the table by category, and compatibility
- [x] The overview has a diagram, and a card and a nav entry per page
- [x] A test holds each block that names a file to that file
- [x] `doc/README.md`: what is where, and the rules of a page

## Obstacles
- **Issue:** The article the structure was to take after could not be
  fetched (HTTP 403). A search gave its points: start from a diagram,
  show examples, build the structure after the content.
  **Resolution:** Followed those: one diagram on the overview, a program
  with its output first in every section.
- **Issue:** A script that needs a Go program cannot run in the site's test.
  **Resolution:** Such a block is `cameo hosted`, shown from its file,
  and the example's own test runs it.

## Outcomes
- "Coming from Ruby" is "Ruby vs cameo".
- The commands shown on pause.html and io.html were run by hand; no test
  runs a `sh` block.
