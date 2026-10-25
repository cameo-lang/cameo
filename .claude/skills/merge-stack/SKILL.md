---
name: merge-stack
description: Merge a stack of pull requests into main as one merge pull request - squash each pull request to one commit, restack them on a fresh origin/main, run CI on the top only and fix a failure in the pull request that introduced it, review the whole stack and fix findings where they belong, then cherry-pick every commit onto an mq/<topic> branch, fast-forward main to it once CI is green, and close the source pull requests. Use when asked to merge a stack, or to land the work in flight.
---

# Merge a stack

Work goes out as a stack of pull requests, each based on the branch of the one
below it ([agents/CLAUDE.md](../../../agents/CLAUDE.md)). Merging them one by one
squash-merges each, and every merge forces a restack of what is above it. This
skill lands the whole stack at once: one commit per pull request, in one merge
pull request that `main` is fast-forwarded to, so `main` keeps one commit per
source pull request.

Asking for this skill is the authorization to force-push the stack's branches,
to push the merge pull request to `main` and to close the source pull requests. Nothing
else is merged or closed.

## Before you start

- Work in a clean checkout. Ask the human once whether to switch this checkout
  or to create a worktree; do not create one unasked.
- Work from `origin/<branch>` refs, never from local branches: a local branch
  may be stale.
- Keep two lists in the scratchpad, bottom to top. The stack: number, head
  branch, base branch, the head sha before you touched it, and `k`, the number
  of the pull request's own commits. The commits: each pull request's current
  commit, which you rewrite as you go. Every later step reads them.
- Keep each pull request's title and description in a file
  (`gh pr view <n> --json title,body`): step 2 commits with it, and you edit it
  when a fix changes what it says.

### gh without read:org

`gh pr edit` fails when the token has no `read:org` scope. Use the REST API,
which needs only `repo`:

```sh
gh api -X PATCH repos/<owner>/<repo>/pulls/<n> -f base=main          # retarget
gh api -X PATCH repos/<owner>/<repo>/pulls/<n> -F body=@<file>       # description
gh api -X PATCH repos/<owner>/<repo>/pulls/<n> -f state=closed       # close
gh api -X POST  repos/<owner>/<repo>/issues/<n>/comments -f body=... # comment
gh api repos/<owner>/<repo>/pulls/<n> --jq .commits                  # k
```

`gh pr list --json commits` over many pull requests exceeds GitHub's GraphQL
node limit; read `k` per pull request with the last command.

## 1. Identify the stack

Start from the pull request the human named, or from the one whose head is the
current branch.

```sh
git fetch origin --prune
gh pr list --state open --limit 100 --json number,title,headRefName,baseRefName,headRefOid
```

- **Down:** follow `baseRefName` until it is `main`, or a branch whose pull
  request is merged (`gh pr list --head <base> --state merged`). That merged
  branch is not part of the stack; its commits are already on `main` as a
  squash.
- **Up:** the open pull request whose base is this head. One is the next; none
  is the top.

If two open pull requests share a base, the stack branches. Stop and ask which
line to merge.

Check that `origin/<head>~k` is the head of the pull request below, and that
no range holds a merge commit (`git rev-list --merges`). A pull request whose
`k` does not reach its base has a history the next step cannot squash; ask.

## 2. Squash each pull request and restack it on origin/main

Do this bottom-up, in one pass. The bottom's new parent is a freshly fetched
`origin/main`; every other pull request's new parent is the new commit of the
one below it.

A pull request's own commits are the last `k` commits of its head. Use that
range, not `<base>..<head>`: when the base was squash-merged, `<base>..<head>`
holds `main`'s commits again, or its base branch is gone. Squash on the old
base first, so a conflict comes up once per pull request rather than once per
commit:

```sh
squash=$(git commit-tree "<old head>^{tree}" -p "<old head>~<k>" -F <message file>)
git switch --detach <new parent>
git cherry-pick "$squash"     # resolve a conflict, git add, git cherry-pick --continue
```

Record `HEAD` as the pull request's commit and go on with the next. A script
that loops over the stack and stops at the first conflict saves retyping.

**Resolving a conflict.** A conflict with `main` usually means two changes to
the same feature: keep both behaviours, not one side. Build and run the tests
of the packages you touched before `--continue`. When the resolution changes
behaviour the pull request describes, add it to its description.

**Generated files.** A file built from the source conflicts or goes stale on
every restack. Do not merge it by hand: take either side and regenerate it.
In cameo, `examples/aot/*/*.go` embed a digest of the parsed script, so any
change to `syntax` changes them:

```sh
make build
for rb in examples/aot/*.rb; do
  ex=$(basename "$rb" .rb)
  pkg=$ex; [ -f "examples/aot/$ex/main.go" ] && pkg=main
  bin/cameo build "$rb" "examples/aot/$ex" "$pkg"
done
```

Run `make check` on the top before you push. Then push each branch with a
lease on the sha it had on origin, which is the last one you pushed, not a
commit you amended since:

```sh
git push --force-with-lease=<head>:<sha on origin> origin <commit>:refs/heads/<head>
```

Retarget the bottom pull request to `main` if its base was a merged branch.
`git diff <old top> <new top>` shows only what `main` gained since the stack
was based on it, and the conflicts you resolved; read it.

## 3. Run CI on the top only

Every push starts a run. Cancel the runs of every pull request but the top,
and check again for half a minute or so: a run is created a little after its
push.

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

   Fix the bug in the pull request whose commit bisect names, with a test that
   fails without the fix (see *Fixing a lower pull request*), push, and run CI
   again.

## 4. Review the stack

Review the range, not the top branch: `/code-review` given a branch or a pull
request reviews that pull request alone. Pass the range and the rules:

```
/code-review origin/main..<new top> — review this whole range, one commit per
pull request (#a, #b, … bottom to top). For every finding, name the commit it
is in. Do not check out, switch or modify any checkout or worktree; read with
git show and git diff, and build from `git archive <top> | tar -x -C <new
directory in the scratchpad>`.
```

For each finding:

- **Inside the stack's concerns:** fix it in the pull request whose commit it
  names (see *Fixing a lower pull request*), with a test that fails without the
  fix. Update the pull request's description if the fix changes what it says.
- **Outside the stack's concerns** — code the stack does not change or whose
  behaviour it does not touch: do not fix it. File an issue for it, after
  checking `gh issue list --search` for one that exists.
- **Not fixable here**, such as a limitation of a dependency: file an issue
  and link it from the pull request's notes.
- **Not a bug:** a behaviour the repository documents as a decision (in cameo,
  `doc/cameo-vs-ruby.md`), or a report that cannot happen. Leave it, and say
  why in the report to the human.

Write an issue to [agents/WRITING.md](../../../agents/WRITING.md): the problem
and its impact. Claim in it only what you ran.

After the fixes, push, and go back to step 3 until the top's run is green.

### Fixing a lower pull request

Fix a pull request in its own commit, so the merge keeps one commit per pull
request, then carry everything above it along:

```sh
git switch --detach <its commit>
# edit, test, regenerate generated files
git commit --amend -F <message file>   # its message, updated if the fix changed it
# for each pull request above it, bottom-up:
git cherry-pick <its commit>           # resolve, regenerate, test, --continue
```

Record each new commit as you go. Fix several pull requests bottom-up in one
pass: amend the lowest, pick up to the next one to fix, amend it, and so on.
The descriptions on GitHub are what step 5 copies into the commit messages, so
update them (`gh api -X PATCH …/pulls/<n> -F body=@<file>`) when a fix changes
what one says.

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
  bottom to top, with their titles:

  ```markdown
  Merging stack: <short summary>

  - #135 specs: runs ruby/spec's language and core under cameo, with CRuby as baseline
  - #136 syntax: accepts bang methods a script defines, as plain calls
  ```

## 6. Wait for CI and fast-forward main

```sh
gh pr checks <merge pr> --watch
git fetch origin
git push origin mq/<topic>:main
```

The push fast-forwards `main` to the merge pull request's head: `main` gets the
stack's commits as they are, with the SHAs CI tested, and no merge commit.
GitHub marks the pull request merged when its head reaches `main`. A push that
is not a fast-forward is refused, without `--force`, when `main` moved since
step 5. Then rebase `mq/<topic>` onto `origin/main`, push it, wait for its CI
again, and push to `main`.

Never `--squash`: it leaves one commit for the whole stack. When `main` is
protected against pushes, merge in GitHub instead:
`gh pr merge <merge pr> --merge --subject "Merging stack: <short summary> (#<merge pr>)"`
keeps the commits under a merge commit; `--rebase` keeps them in a line but
rewrites them, so `main` holds SHAs CI never ran on.

Check the result: `git log --oneline origin/main` shows one commit per pull
request on top of the old `main`, and `gh pr view <merge pr> --json state` is
`MERGED`. A failure here is debugged as in step 3; the tree is the top's, so
it is most likely a change on `main` since step 2.

## 7. Close the source pull requests

Close them top-down, so no open pull request loses its base. Comment first
with the commit each became on `main`
(`git log --format=%h --grep='(#<n>)$' <old main>..origin/main`):

```sh
gh api -X POST repos/<owner>/<repo>/issues/<n>/comments -f body="Merged in #<merge pr> as <commit>."
gh api -X PATCH repos/<owner>/<repo>/pulls/<n> -f state=closed
```

Report to the human: the merge pull request as a link, the commit `main` is at,
what the conflicts with `main` changed, the review findings fixed and left
alone and why, the issues you filed, what you could not test, and any failure
you left alone because it was already on `main`. Offer to remove a worktree
you created.
