# How we work with pull request stacks

Work goes out as a stack: a row of pull requests, each based on the branch
of the one below it, the lowest on `main`. This file says how a stack is
shaped, how CI runs on it, what to do when the run fails, and how to put a
stack that grew out of shape back in order.
[merge-stack](../.claude/skills/merge-stack/SKILL.md) lands a stack on
`main`.

## One stack, no branches

Keep all work in flight in one stack that does not branch: a new pull
request goes on top of the last one, not on `main` and not on a pull request
lower down, whether it depends on the others or not. Every change in flight
is then tested together, in the run of the top, and merge conflicts are less
likely.

## What a pull request of a stack is

**One change, as small as it can be.** A pull request that does five similar
things is five pull requests. A title with "and" in it names two.

**One commit.** The commit's message is the pull request's title and
description. A fix to a pull request amends its commit; it is not a second
commit and not a pull request further up.

**Next to its relatives.** Pull requests of one concern are neighbours: the
changes to the evaluator's calls together, the changes to strings together.
A reader who follows the stack bottom-up meets one subject at a time.

**In its final form.** A stack does not show how the work went. When a later
change reworks or drops what an earlier pull request of the same stack did,
both are one pull request, in the form that stays:

- a pull request and the fix of a bug it brought in;
- a value that was set, and changed again two pull requests later;
- a feature that was added and then removed: neither is in the stack.

A work log belongs to the pull request it logs, not to one of its own.

Each pull request leaves `main` working: it builds, and its tests pass with
what is below it.

## CI runs on the top alone

Only the run on the top of a stack tests everything together. A run on each
pull request below it costs time and says nothing the top's run does not.

Every commit of a stack but the top one carries `[skip ci]` as a paragraph
of its own, before the trailers. GitHub starts no workflow for a push or a
pull request whose head commit has it, so pushing a stack of twenty starts
one run.

```
eval: looks the method of a call up by its interned name

A SEND read its name as a string ...

[skip ci]

Co-Authored-By: ...
```

- A new pull request on top is committed without the marker. The one below
  it keeps its green run; give it the marker the next time its commit is
  rewritten.
- When you rewrite the stack, set the marker on every commit but the top
  before you push.
- After a push of many branches the top's run can take a few minutes to
  appear in the list of runs. Wait and read the list again before you
  conclude that none started; do not push again to start one.
- The marker is never in a pull request's title or description. merge-stack
  writes each commit's message from those when it lands the stack, so no
  commit on `main` carries the marker. A pull request merged by itself in
  GitHub must not take its commit's message as the merge's: `main` would
  skip its run.

Before a push, the local evidence is what it was: the change's tests, and
`make check` on the top.

## When the top's run fails

1. **Read the log and name the failing tests.** Not "CI is red": which test,
   which message.
2. **Run them on the top locally.** A failure that does not reproduce is a
   different problem: a test that depends on timing or on the machine.
3. **Find where in the stack they start to fail.** The likely place is the
   pull request that introduced what the test exercises, so start there:
   check out its commit and the one below it and run the test on both. When
   that does not name it, bisect, each pull request being one commit:

   ```sh
   git bisect start <top> <a commit where the test passes>
   git bisect run go test ./src/eval/ -run 'TestName'
   ```

4. **Fix it in the pull request that introduced it**, with a test that fails
   without the fix: amend that commit, not the top and not a new pull
   request.
5. **Rebase everything above it** onto the amended commit, bottom-up, and
   push each branch with a lease on the commit it had on origin:

   ```sh
   git rebase --onto <amended commit> <old commit> <branch above>
   git push --force-with-lease=<branch>:<sha on origin> origin <branch>
   ```

   Then read the top's new run to its conclusion.

A failure that is on `main` too is not the stack's. Say so, with the test
and `main`'s run, and fix it at the bottom of the stack or in a pull request
of its own.

## Putting a stack back in order

A stack that several sessions added to holds unrelated groups in the order
the work happened, and changes that later ones took back. Reorder it before
it is reviewed or merged.

1. **Survey.** List the commits bottom-up with their titles and the files
   each touches. For every commit, count the lines it deletes that an
   earlier commit of the stack added (`git blame` on the deleted ranges):
   that names the changes that were reworked or dropped.
2. **Plan the units**, bottom-up: which commits each new pull request is
   made of. Changes that stand alone on `main` go to the bottom, where they
   can merge first. Then one group per concern. A commit moves only past
   commits it does not depend on.
3. **Replay in a worktree of its own**, never in the checkout a human or
   another session uses. Cherry-pick the commits of a unit without
   committing, resolve, build, commit. When the units so far hold exactly
   the commits up to some old commit, the tree must be that old commit's
   tree: take it instead of resolving a conflict by hand.
4. **Compare the tops.** `git diff <old top> <new top>` shows only what the
   reorder changed on purpose. List those changes in the report.
5. **Build and test every unit** that is a state the old stack never had.
6. **Keep the pull requests that survive.** A unit made of one old pull
   request keeps it: push the new commit to its branch and set its base.
   A unit made of several keeps the first and closes the others. A unit
   split off an old pull request gets a new branch and a new pull request.
   Before the push, set the base of every pull request whose present base
   will lie above it to `main`: a pull request whose head is inside its
   base has no commits, and GitHub closes it.
   Keep the old top on an `archive/` branch until the new stack is merged.
7. **Fix the references.** A work log, a document or a description that
   names a pull request by number must name the one that holds the change
   now.

## After a pull request below was merged by itself

A squash merge leaves the branch above with the original commits of the one
merged below it. Move it onto `main` by hand:

```sh
git fetch origin
git rebase --onto origin/main <last commit of the merged branch> <your branch>
git push --force-with-lease
```

After a fetch, compare the local branch with `origin/<branch>` before you
trust it (`git log --oneline origin/<branch>..<branch>` and the reverse): a
rebase on a stale local ref drops the commits that only origin has. Give
each agent its own worktree (`git worktree add`); never share one.
