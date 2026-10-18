# CLAUDE.md — cameo

cameo is a small language with Ruby's syntax, run by an interpreter written in
Go. This file is the entry point for coding agents. Read it first, then:

1. **[agents/WRITING.md](agents/WRITING.md)** — always. How to write pull
   request titles and descriptions, commit messages, code comments,
   documentation and messages to the human. **No person names in pull
   request titles and descriptions.** It is not important who asked for a
   change, but why someone asked for it.
2. **[agents/CLAUDE.md](agents/CLAUDE.md)** — always. Task strategies,
   interface design, work logs, branching, and finishing on a pull request.
3. **[agents/golang.md](agents/golang.md)** — the Go layout and conventions.
4. **[agents/teamwork.md](agents/teamwork.md)** — what to decide yourself and
   what to ask.
5. [README.md](README.md) — what cameo is and how to build it.

## Pull requests

**Open a pull request for every change without asking, and never ask whether
to open one.** A pushed branch without a pull request is unfinished work.
Follow its CI to green.

## Build and test

```sh
make check            # lint, Go tests, integration tests: what CI runs
make help             # every target
```

Go code lives in `src/`, tests in `tests/` (Go tests) and
`tests/integration/` (shell tests against the built binary).
