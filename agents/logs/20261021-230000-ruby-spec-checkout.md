# Task: a script that clones ruby/spec unless a local copy exists

**Started:** 2026-10-21 23:00:00
**Ended:** 2026-10-21 23:20:00
**Strategy:** Feature (TDD)
**Status:** Completed
**Complexity:** Simple
**Used Models:** Opus
**Token usage (Estimated):** 60k input, 5k output

## Objective
Add a script that clones https://github.com/ruby/spec into `ruby-spec/` at
the root of the main repository when it is not there yet, and pulls it when it is.

## Progress
- [x] `specs/checkout.rb`: clone, or pull an existing copy (`--ff-only`), print the path, `--dir`, `--commit`
- [x] `make ruby-spec`
- [x] `specs/README.md` re-sorts through `checkout.rb`
- [x] Ran it against the existing copy, a fresh directory (pinned commit), a rerun and a non-git directory

## Obstacles
- **Issue:** a worktree's root is not where the shared copy lives.
  **Resolution:** the default directory comes from `git rev-parse --git-common-dir`.

## Outcome
Script, Makefile target and README change, on a pull request stacked on the open one.
