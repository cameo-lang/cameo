# Task: the repository's skeleton

**Started:** 2026-10-17 12:15:00
**Ended:** 2026-10-17 12:45:00
**Strategy:** Feature (TDD)
**Status:** Completed
**Complexity:** Simple
**Used Models:** Opus
**Token usage (Estimated):** 60k input, 20k output

## Objective
Give the empty repository what an agent needs to work in it: a README, the
agent guidance taken from ark, a CI workflow, a minimal `amber` package with
Go code under `src/`, and Go and integration tests under `tests/`.

## Progress
- [x] go.mod, `src/amber` (Version), `src/cmd/amber` (cobra, `version`).
- [x] Makefile with ark's targets; `make check` runs lint, tests and
      integration tests.
- [x] tests/: a Go test on the package; tests/integration/ with ark's
      testsh.inc and a test of the command.
- [x] agents/: WRITING.md (with the no-names rule), CLAUDE.md, golang.md,
      teamwork.md, strategy-guide.md, interface-design.md, the log template.
- [x] .github/workflows/ci.yml and the pull request template.

## Outcome
`make check` passes locally: vet, gofmt, tidy, one Go test, three
integration tests. The integration tests fail, as they should, against a
binary that prints the wrong thing.
