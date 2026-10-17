# Agent guidance

How to work in amber: picking a strategy, shaping an interface, logging what
you did, and finishing on a pull request.

| File | Covers |
|---|---|
| this file | how an agent works |
| [WRITING.md](WRITING.md) | how an agent writes: prose, comments, commits, pull requests |
| [golang.md](golang.md) | the Go rules: layout, conventions, tests |
| [teamwork.md](teamwork.md) | working with the human: what to decide, what to ask, how to report |
| [strategy-guide.md](strategy-guide.md) | the workflow for a bug fix, a feature, a refactoring, a performance task |
| [interface-design.md](interface-design.md) | how to shape a public API, a command or a flag |

## Before you start

**Pick a strategy.** Before starting a task, say which strategy from
[strategy-guide.md](strategy-guide.md) applies and why — Bug Fix, Feature
(TDD), Refactoring or Performance — and follow its workflow. For a mixed task,
split it and apply each strategy to its part. If it is unclear which applies,
ask.

**Design interfaces for the caller.** Whenever you add to or reshape amber's
public surface — the `amber` Go package, a command, a flag, a language
construct — follow [interface-design.md](interface-design.md): lean interfaces
named in the caller's terms, deep modules behind them.

## While you work

**Scripts are Ruby.** A build helper, a test runner or a one-off migration is
written in Ruby, using the standard library alone (`yaml`, `open3`,
`fileutils`, `optparse` cover most needs), so it runs on any machine with a
Ruby and no Gemfile. Reach for a gem only when the standard library cannot do
the job, and say why in the script.

**Secrets never go into the repository** — not in code, a Makefile, a
workflow `env:` block or a config file. A workflow reads a credential from a
GitHub Actions secret.

**Design documents.** When you carry out a plan that was built in plan mode,
summarise it in `doc/design/<name>.md` as part of the change: the context,
the decisions and their reasons, the final shape of the code. Not for ad-hoc
tasks.

## Writing it down

How to write — prose, comments, commit messages, pull request descriptions — is
in [WRITING.md](WRITING.md). Read it before you write anything a human will
read. Above all: **no person names in pull request titles and descriptions.** It
is not important who asked for a change, but why someone asked for it.

### Work logs

Keep a log in [agents/logs/](logs/) for each significant task, from
[logs-template.md](logs-template.md), named `YYYYMMDD-HHMMSS-short-description.md`.
Create it when you start, update it as steps finish, record every obstacle
(also those you resolved quickly), and set the outcome and the "Ended" time
when you commit.

Complexity grades the session, not the change: *Simple* when the task needed
no critical feedback from the human, *Medium* when a planning stage needed
important feedback and later feedback was cosmetic, *Complex* when the plan
was not enough and the human had to step in repeatedly.

## Finishing

### Branch, rebase, and stage

**Branch from a fresh remote:** `git fetch origin` and branch from
`origin/main`. A stale base puts other people's work into your diff.

**Rebase onto main; never merge main in.** Fetch and rebase before you touch a
branch you are picking back up. Force-push a rebased branch you own with
`--force-with-lease`.

**Stage explicit paths, not `git add -A`.** Build output and generated files
end up committed otherwise, above all after switching branches. Read
`git diff --cached --stat` before you commit and check it matches the change
you made. Format the files you edited, not every file in a directory.

**Push review fixes as new commits.** Pull requests are squash-merged, so the
merge makes the history tidy, not amending.

### Do not wait for CI needlessly

Every push starts a CI run, but a pull request does not wait for its run to
finish, or to start, before it is merged. What it needs before it is pushed is
local evidence: unit tests cover the change, and `make check` passed. Then
merge it while its run is still going or only scheduled.

Work goes out as a stack, so ten pull requests start ten runs, and only the
run on the top of the stack tests everything together. Waiting for each of the
nine below it costs time and tells you nothing the top run does not. Read the
top run to its conclusion; when it fails, fix the failure with a new pull
request on top of the stack.

### One stack, no branches

Keep all work in flight in one stack that does not branch: each new pull
request goes on top of the last one, not on `main` or on a pull request lower
down. Every change in flight is then tested together, in the top run, and
merge conflicts are less likely.

### Every task ends on a pull request

Always, without being asked: a pushed branch is not a deliverable. Open it to
the template in [WRITING.md](WRITING.md) (GitHub pre-fills it from
`.github/pull_request_template.md`), then **follow CI to a conclusion** on the
top of the stack (see above): read a failure, fix it, push again. Handing back
a red pull request is handing back the task. An empty list of checks means the run has not started, not that it
passed.

### Stacked pull requests

Split a task that is not one concern into small pull requests that each leave
`main` green, and stack them, whether they depend on each other or not: each
is based on the branch of the one below it. Merge bottom-up; a pull request
above waits for the one below it.

Because merges are squashes, the branch above still holds the original
commits of the one merged below it. Move it onto `main` by hand:

```sh
git fetch origin
git rebase --onto origin/main <last-commit-of-the-merged-branch> <your-branch>
git push --force-with-lease
```

After a fetch, compare the local branch with `origin/<branch>` before you
trust it (`git log --oneline origin/<branch>..<branch>` and the reverse): a
rebase on a stale local ref drops the commits that only origin has. Give each
agent its own worktree (`git worktree add`); never share one.

### Verify before you claim

Run `make check` — lint, the Go tests and the integration tests — before
reporting a task complete, and say what you ran and what you did not. If a
test fails, say so with its output.

**A green run is only evidence if it could have been red.**

- Build the filesystem a test depends on in `t.TempDir()`, never the
  repository you are standing in.
- Assert that a test compared something: a suite that found no cases passes
  too.
- Never hide stderr in a command you use as evidence.
- Verify the committed tree, not the working tree: `git show --stat HEAD`.
- "It compiles" means little when part of the build is skipped. Build the
  whole module.
