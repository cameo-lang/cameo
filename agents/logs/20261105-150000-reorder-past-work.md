# Task: Replay the work since v0.4.2 as a reordered, reviewed stack

**Started:** 2026-11-05 15:00:00
**Ended:** 2026-11-05 17:45:31
**Strategy:** Refactoring
**Status:** In progress
**Complexity:** Complex
**Used Models:** Fable 5.1
**Token usage (Estimated):** 2.5M input, 400k output

## Objective
Forty-seven changes had landed on main since v0.4.2 in the order they
were made: libraries, then a language change (blocks capture values)
that rewrote their tests, then rewrites of three libraries in cameo. Sort
them into categories (build prerequisite, core language, core stdlib,
minispec, other libraries, documentation), squash a change and its later
revisions into one unit, reset main to v0.4.2, and replay each unit as
one small pull request on the one below it, reviewed with /code-review
and its findings fixed without a human in the loop. Then measure: ruby/spec
coverage of core Ruby, the library coverage, and the performance on this
Mac against Ruby 3.4.1; and write the procedure down as a skill.

## Progress
- [x] Merged the stack in flight (#231 to #243) and kept main as
      archive/main-before-reorder; reset main to v0.4.2
- [x] Sorted the 47 changes into 43 units; the plan in the scratchpad
- [x] Units 01 to 07: UTC tests, AOT folds, blocks capture values, proc
      fields, class attributes, host blocks, at_exit
- [x] Units 09 to 16: the library packages, the plan page, libraries
      written in cameo, Cameo::Native
- [x] Units 17 to 43: minispec, every library in its final form, io/httpd
      with the I/O policy, frankie, the site
- [x] Every pull request reviewed; each review's findings fixed in its
      unit and the units above rebased, 41 reviews in all
- [x] Issues filed for what a unit could not fix: #250, #266, #277, #279,
      #280, #285
- [x] The ruby/spec run, the plan page's tables and the performance table
      on this Mac (doc/coverage.md, doc/stdlib.md, tests/performance)
- [x] The skill, .claude/skills/reorder-past-work
- [x] The merge procedure's review of the whole range found nine things a
      unit's own review could not see: a named zone lost through the wire
      form, Matrix without coerce, send(:[]=) refused where assignment was
      not, require marking a cameo library loaded too late, Pathname
      rendered by the evaluator, a comment promising that Go-defined
      classes cross (#295), and three stale sentences and rows; eight
      fixed in their units, the bang exemption by name left and explained
- [ ] The stack merged with /merge-stack: the step after this log is
      committed, so its outcome is not in it. The coverage and performance
      runs predate the range review's fixes, which touched no measured
      path but Pathname's inspect

## Obstacles
- **Issue:** the replay script's registry merge had a stray colon in a git
  path and silently registered no library; the digest unit did not build.
  **Resolution:** fixed, and the script's rules for the plan page's rows,
  aliased imports and packages without Install grew with each unit.
- **Issue:** the descriptions of the first twenty units ignored the
  template in agents/WRITING.md until a review quoted it.
  **Resolution:** a body generator normalises every description.
- **Issue:** the ruby/spec harness's prelude rebound a captured variable,
  so every spec failed its check under the replayed binary.
  **Resolution:** the prelude reads the options with all? (unit 03) and the
  forty units above were rebased.
- **Issue:** an accumulation into an Array is quadratic (#279), a break
  through &block ends the inner call (#280), an assignment to a handle's
  setter keeps the with rewrite's rules (#285).
  **Resolution:** filed; the libraries work around the first two.

## Outcome
Forty-four pull requests on v0.4.2, one commit each, reviewed, ready
for the merge as one stack; the reports in doc/coverage.md, doc/stdlib.md
and tests/performance; the skill for the next time. The merge itself
comes after this log.

## Insights
Review as you go: a finding in a low unit costs a rebase of everything
above it, and the cost grows with the stack. A reviewer that verifies
against the Ruby on the machine finds real divergences in every library;
ten findings a unit was the norm, and all but a handful were bugs.
