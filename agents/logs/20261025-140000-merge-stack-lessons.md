# Task: merge-stack skill, from its first run

**Started:** 2026-10-25 14:00:00
**Ended:** 2026-10-25 14:40:00
**Strategy:** Refactoring — documentation only
**Status:** Completed
**Complexity:** Simple
**Used Models:** Opus
**Token usage (Estimated):** 60k input, 10k output

## Objective
Fold what the first run of merge-stack (#135–#151, merged as #162) needed
into the skill, and fast-forward main instead of merging with a merge commit.

## Progress
- [x] Step 4 reviews the range `origin/main..<top>`, without touching a checkout
- [x] A procedure for fixing a lower pull request and carrying the stack along
- [x] Squash on the old base with commit-tree, one conflict per pull request
- [x] Generated files, push leases, gh without read:org
- [x] Step 6 fast-forwards main with a push

## Obstacles
- **Issue:** `/code-review` given the top branch reviewed only #151.
  **Resolution:** a second review over the range; the skill now passes the range.
- **Issue:** `gh pr edit` fails without the `read:org` scope.
  **Resolution:** `gh api -X PATCH repos/.../pulls/N`.
- **Issue:** a lease on a locally amended sha rejected the #151 push.
  **Resolution:** lease on the sha origin holds.

## Outcome
The skill is updated; the fast-forward in step 6 has not been run yet.
