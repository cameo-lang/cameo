# Task: adds the merge-stack skill

**Started:** 2026-10-25 12:00:00
**Ended:** 2026-10-25 12:20:00
**Strategy:** Feature (TDD) — documentation only, no code under test
**Status:** Completed
**Complexity:** Simple
**Used Models:** Opus
**Token usage (Estimated):** 80k input, 10k output

## Objective
A project skill that lands a whole stack of pull requests as one merge pull
request with one commit per source pull request.

## Progress
- [x] Read the stacking and branching rules in agents/CLAUDE.md
- [x] Checked the repository allows merge commits (`gh repo view`)
- [x] Wrote .claude/skills/merge-stack/SKILL.md
- [x] Opened the pull request

## Obstacles
- **Issue:** the bottom of an open stack can be based on a branch that was
  already squash-merged (#135 on claude/ruby-spec-checkout, merged as #134), so
  `<base>..<head>` is not the pull request's own commits.
  **Resolution:** the skill takes the last `k` commits of the head, with `k`
  from `gh pr view --json commits`.

## Outcome
The skill is written; it has not been run on a stack yet.
