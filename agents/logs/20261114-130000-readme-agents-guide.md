# Task: a README that leads with the docs, and a guide for models that write cameo

**Started:** 2026-11-14 11:00:00
**Ended:** 2026-11-14 14:00:00
**Strategy:** Feature
**Status:** Completed
**Complexity:** Medium
**Used Models:** Fable 5.2
**Token usage (Estimated):** 300k input, 25k output

## Objective
The README is to start with the link to the site, say the four things
cameo is, show how to build and play with it, the differences from Ruby
in short, and numbers. `agents.md` is to tell a model how to write cameo.

## Progress
- [x] README: the docs first; Ruby but immutable, I/O separate, easy to
      embed, pause and resume; build with examples/ex1; irb and ^C;
      differences; speed by category and compatibility
- [x] `agents.md`: write Ruby and leave out what changes a value in
      place, with a table of what to write instead, classes, collections,
      async and await, and a checklist
- [x] A test runs every Ruby block of `agents.md`
- [x] `agents/CLAUDE.md` says which document a change updates

## Outcomes
- The README lost its status section, the package list in prose and the
  irb and debugger paragraphs; the site and the repository table have them.
- The count of passing ruby/spec examples is that of 2026-11-05, with
  its date; a later run passes fewer and is not recorded yet.
