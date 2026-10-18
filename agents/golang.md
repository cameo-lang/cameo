# Go rules

cameo is one Go module, `github.com/cameo-lang/cameo`, with `go.mod` at the
root.

## Layout

**All Go source lives under `src/`.** The root holds what describes the
module — `go.mod`, `Makefile`, `README.md`, `doc/` — and nothing else.
Packages import as `github.com/cameo-lang/cameo/src/<name>`, and the last
element of the path is the package name:

```
src/cameo/        the host API a Go program imports
src/cmd/cameo/    the cameo command
src/assert/       the assertions the tests use, vendored
tests/            Go tests against the packages, as a caller uses them
tests/integration/  shell tests that drive the built binary
```

A package's own unit tests sit beside it in `src/` (`foo_test.go`). Tests
that use cameo the way a program embedding it does — through exported API
only — go in `tests/`.

## Building and testing

```sh
make deps build        # resolve modules, build bin/cameo
make tests             # gofmt check, go mod tidy check, go test ./...
make integration-tests # build, then run tests/integration/test_*.sh
make lint              # go vet ./...
make check             # lint, tests and integration tests: what CI runs
```

Integration tests are bash scripts built on
[tests/integration/testsh.inc](../tests/integration/testsh.inc): define
`test_*` functions, assert with `assert_eq`, `assert_neq`, `assert_match` or
`assert_contains`, and end with `run_tests`. The binary under test arrives in
`CAMEO_BIN`; `make integration-tests` sets it. When you add or change
behaviour of the command, extend them in the same commit.

## Session setup: verify Go LSP

At the start of a session, check that the Go language server, `gopls`, is
available: try an `LSP` tool call (`workspaceSymbol` with a real query)
before concluding it is not. Without it, navigation and cross-file edits rely
on text search; flag that and avoid non-trivial cross-file refactors until it
works.

In a Claude Code cloud sandbox (`CLAUDE_CODE_REMOTE_ENVIRONMENT_TYPE` is
`cloud_default`), install it without asking:

```sh
go install golang.org/x/tools/gopls@latest
PLUGIN_BIN="$(ls -d ~/.claude/plugins/cache/claude-plugins-official/gopls-lsp/*/bin 2>/dev/null | head -1)"
[ -n "$PLUGIN_BIN" ] && mkdir -p "$PLUGIN_BIN" && ln -sf "$(go env GOPATH)/bin/gopls" "$PLUGIN_BIN/gopls"
```

Elsewhere, ask before installing anything.

## Conventions

- **Arguments go through [cobra](https://github.com/spf13/cobra):** one
  `cobra.Command` per command, `RunE` returning an error, and a constructor
  (`newCommand(stdout)`) so a test can run it with `SetArgs` and read its
  output. Do not parse `os.Args` by hand.
- **Errors, not panics, across the API.** Exported functions return `error`.
  A panic helper is for the command and for test setup.
- **Assertions come from [src/assert](../src/assert):**
  `assert.Equals(t, got, want)`, `assert.NoError(t, err)`,
  `assert.Contains(t, list, x, "details %v", list)`. It is vendored from
  simple-go and keeps that licence; add a missing assertion there rather
  than hand-rolling a check.
- **Test names say the behaviour:** `TestParseDiscardsBlankRecords`, not
  `TestParse2`. Prefer explicit cases to a table unless the table earns its
  keep.
- **`gofmt` and `go vet` are not optional.** `make tests` refuses unformatted
  files and an untidy `go.mod`; `make lint` runs `go vet`.
