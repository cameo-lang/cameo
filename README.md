# amber

amber is a small language with Ruby's syntax, run by an interpreter written in
Go. Every value is immutable, every object can be a protobuf message, and a
script hands work to goroutines with `async` and collects it with `await`. A
Go program embeds it through the `amber` package.

This repository holds the module's skeleton: the `amber` package, the `amber`
command, the tests and the build. The interpreter itself is developed in
[radiospiel/ark](https://github.com/radiospiel/ark), under
`parts/golang/amber`, and moves here next.

## Build and test

amber needs Go, at the version `go.mod` names.

```sh
make deps build         # resolve modules, build bin/amber
make tests              # gofmt and go.mod checks, then go test ./...
make integration-tests  # build, then run the shell tests against bin/amber
make check              # lint, tests and integration tests: what CI runs
bin/amber version
```

## Layout

| Path | Holds |
|---|---|
| `src/amber/` | the `amber` package: the API a Go program uses |
| `src/cmd/amber/` | the `amber` command |
| `src/assert/` | the assertions the tests use, vendored from simple-go |
| `tests/` | Go tests against the packages, as a caller uses them |
| `tests/integration/` | shell tests that drive the built binary, on `testsh.inc` |
| `agents/` | guidance for coding agents; start at [CLAUDE.md](CLAUDE.md) |
| `.github/` | the CI workflow and the pull request template |

## License

Proprietary; see [LICENSE](LICENSE). Not open source.
