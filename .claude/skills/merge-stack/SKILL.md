---
name: merge-stack
description: Merge a stack of pull requests into main as one merge pull request - squash each pull request to one commit, restack them on a fresh origin/main, run CI on the top only and fix a failure in the pull request that introduced it, review the stack and fix findings where they belong, then cherry-pick every commit onto an mq/<topic> branch, merge it without squashing once CI is green, and close the source pull requests. Use when asked to merge a stack, or to land the work in flight.
---

# Merge a stack

Work goes out as a stack of pull requests, each based on the branch of the one
below it ([agents/CLAUDE.md](../../../agents/CLAUDE.md)). Merging them one by one
squash-merges each, and every merge forces a restack of what is above it. This
skill lands the whole stack at once: one commit per pull request, in one merge
pull request that is merged with a merge commit, so `main` keeps one commit per
source pull request.

Asking for this skill is the authorization to force-push the stack's branches,
to merge the merge pull request and to close the source pull requests. Nothing
else is merged or closed.

## Before you start

- Work in a clean checkout. Ask the human once whether to switch this checkout
  or to create a worktree; do not create one unasked.
- Work from `origin/<branch>` refs, never from local branches: a local branch
  may be stale.
- Keep a list in the scratchpad of every pull request in the stack, bottom to
  top: number, title, head branch, base branch, the head sha before you
  touched it, and its new commit. Every later step reads it.

## 1. Identify the stack

Start from the pull request the human named, or from the one whose head is the
current branch.

```sh
git fetch origin --prune
gh pr view <n> --json number,title,headRefName,baseRefName,headRefOid,commits
```

- **Down:** follow `baseRefName` until it is `main`, or a branch whose pull
  request is merged (`gh pr list --head <base> --state merged`). That merged
  branch is not part of the stack; its commits are already on `main` as a
  squash.
- **Up:** `gh pr list --base <head> --state open` for each head. One pull
  request above is the next; none is the top.

If two open pull requests share a base, the stack branches. Stop and ask which
line to merge.

## 2. Squash each pull request and restack it on origin/main

Do this bottom-up, in one pass. The bottom's new parent is a freshly fetched
`origin/main`; every other pull request's new parent is the new commit of the
one below it.

A pull request's own commits are the last `k` commits of its head, where `k`
is the length of `commits` from `gh pr view`. Use that range, not
`<base>..<head>`: when the base was squash-merged, `<base>..<head>` either
holds `main`'s commits again or its base branch is gone.

```sh
git switch --detach <new parent>
git cherry-pick <old head>~<k>..<old head>   # resolve conflicts as they come
git reset --soft <new parent>
git commit -F <message file>                  # "<title>\n\n<body>" of the PR
git branch -f <head> HEAD
```

A pull request with one commit skips the reset and the commit; it is still
cherry-picked onto its new parent.

Then, for each pull request:

```sh
git push --force-with-lease=<head>:<old head sha> origin <head>
```

and retarget the bottom pull request to `main` if its base was a merged
branch: `gh pr edit <bottom> --base main`. `git diff <old top> <new top>`
shows only what `main` gained since the stack was based on it, and the
conflicts you resolved; read it.

## 3. Run CI on the top only

Every push starts a run. Cancel the runs of every pull request but the top,
and check again once the top's run shows up: a run is created a little after
its push.

```sh
gh run list --branch <head> --json databaseId,status \
  --jq '.[] | select(.status != "completed") | .databaseId' | xargs -n1 gh run cancel
gh pr checks <top> --watch
```

When a check fails, read its log (`gh run view <id> --log-failed`) and decide
where the failure comes from:

1. **Already on main.** Look at `main`'s latest run of the same workflow
   (`gh run list --branch main --workflow <workflow> --limit 1`). If it fails
   the same test, the stack did not cause it. Find or file an issue for it,
   and ask the human whether to merge the stack anyway.
2. **From the stack.** Find the lowest pull request where the failure first
   appears. Each pull request is one commit now, so bisect the stack locally
   with the failing target:

   ```sh
   git bisect start <new top> origin/main
   git bisect run make <target>
   ```

   Fix the bug in the pull request whose commit bisect names, as an amend of
   that commit, with a test that fails without the fix. Restack everything
   above it (step 2, from that pull request up), push, and run CI again.

## 4. Review the stack

Run `/code-review` on the top branch; its diff against `main` is the whole
stack. For each finding:

- **Inside the stack's concerns:** find the pull request that introduced the
  code (`git blame` or `git log -L` against the stack's commits) and fix it
  there, as an amend of that pull request's commit. Update its description if
  the fix changes what it says.
- **Outside the stack's concerns** — code the stack does not change or whose
  behaviour it does not touch: do not fix it. File an issue for it, after
  checking `gh issue list --search` for one that exists. Write it to
  [agents/WRITING.md](../../../agents/WRITING.md): the problem and its impact.

After the fixes, restack from the lowest pull request you changed, push, and
go back to step 3 until the top's run is green.

## 5. Collapse the stack into a merge pull request

GitHub has no tool that merges a stack as one pull request, so build it by
hand. Name the branch `mq/<topic>`, with `<topic>` the stack's common concern
in a few words (`mq/ruby-spec`).

```sh
git fetch origin
git switch -c mq/<topic> origin/main
```

For each pull request, bottom-up, cherry-pick its commit and give it the pull
request's title with its number, and the pull request's description as body:

```sh
git cherry-pick <commit>
gh pr view <n> --json title,body --jq '.title + " (#<n>)\n\n" + .body' > <message file>
git commit --amend -F <message file>
```

`git diff <top> mq/<topic>` must be empty. Push the branch and open the merge
pull request against `main`:

- Title: `Merging stack: <short summary>`.
- Description: the same line, then the list of the stack's pull requests,
  bottom to top, as links with their titles:

  ```markdown
  Merging stack: <short summary>

  - #135 specs: runs ruby/spec's language and core under cameo, with CRuby as baseline
  - #136 syntax: accepts bang methods a script defines, as plain calls
  ```

## 6. Wait for CI and merge

```sh
gh pr checks <merge pr> --watch
gh pr merge <merge pr> --merge --subject "Merging stack: <short summary> (#<merge pr>)"
```

Merge with `--merge`, **never `--squash`**: the merge keeps one commit per
source pull request on `main`. A failure here is debugged as in step 3; the
tree is the top's, so it is most likely a change on `main` since step 2.

## 7. Close the source pull requests

Close them top-down, so no open pull request loses its base:

```sh
gh pr close <n> --comment "Merged in #<merge pr> as <commit on main>."
```

Report to the human: the merge pull request as a link, the commit it merged,
the issues you filed, and any failure you left alone because it was already on
`main`.
