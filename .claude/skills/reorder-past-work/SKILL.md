---
name: reorder-past-work
description: Rebuild the work that landed on main since a tag as a stack of small, reviewed pull requests in a sensible order - sort the changes into categories, squash a change and its later revisions into one unit, reset main to the tag, replay each unit on the one below it, review every pull request and fix its findings, then measure coverage and performance and merge the stack. Use when asked to reorder, replay or re-land past work, or to turn a tangle of merged changes into a clean history.
---

# Reorder past work into a stack

A stretch of work lands as it was done: a library, a fix to it, a rewrite,
a language change that made half the earlier tests obsolete. This skill
rebuilds that stretch as the history it should have been: one pull request
per change in its final form, in an order a reader can follow, each
reviewed on its own. It needs a tag to start from, the branch that holds
everything since, and the authority to force-push `main`.

Asking for this skill is the authorization to reset `main` to the tag,
force-push the replay's branches, open a pull request per unit without
asking, review each with `/code-review` and fix what it finds without a
human in the loop, and merge the stack at the end with
[merge-stack](../merge-stack/SKILL.md). Before `main` moves, keep what it
was: `git push origin main:archive/main-before-reorder`.

## 1. Survey

```sh
git log --oneline <tag>..origin/main          # every change since the tag
gh pr list --state merged --limit 200 --json number,title,mergedAt
```

For each merged pull request, read its title and description, and note:

- its **category**: a build prerequisite (a test that must run in UTC, a
  helper every later unit needs), a core language change, a core stdlib
  change (basic types), the test library, another library, documentation
  or the build;
- what **later changes revised it**: a library rewritten in cameo, a test
  rewritten when blocks started to capture values, a hunk a later
  refactoring moved. Those are squashed into the unit in its final form:
  the unit takes the archive's final files for what it owns, and the
  original's for what it does not;
- its **prerequisites** that sit in a later category: a library a core
  change needs comes before the change, the plan page before the
  libraries it lists.

Write the plan down (`replay-plan.md` in the scratchpad): a table of
units, bottom-up, each with its category and the commits it squashes.
Ask the human about the categories and the order before building
anything; a wrong order costs a rebuild of every unit above it.

## 2. Reset main

```sh
git push origin main:archive/main-before-reorder
git push --force origin <tag>:main
```

Every later unit is picked from `origin/archive/main-before-reorder`.

## 3. Replay a unit

Each unit is a branch off the one below it, `reorder/NN-<name>`, with one
commit, a pull request on the branch below, a review, and its findings
fixed. A unit is small: a library, one language change, one plan page.

**Pick.** Cherry-pick the unit's commits with `--no-commit` and resolve the
shapes that recur. A script pays for itself by the third unit; write it
in Ruby, as [agents/CLAUDE.md](../../../agents/CLAUDE.md) asks, and keep
it in the scratchpad: the first reorder's helper was session scratch and
is not in the repository, so the rules are here in prose:

- files only the unit owns (its package, its tests, its example): take
  the archive's final version, which carries every later revision;
- the library registry (`src/lib/lib.go`): keep ours, then add the unit's
  import, its `Install` call when its package has one, its `Libraries`
  entries, and its `Sources` entry when it is written in cameo; a package
  imported under another name is installed under that name;
- the plan page (`doc/stdlib.md`): keep ours, move only the rows of the
  libraries the unit adds from *Planned* to *Available*, and apply the
  unit's prose changes by hand; never let a conflict bring the archive's
  neighbouring rows in, and never leave a blank line under a table's
  header separator;
- README, `doc/cameo-vs-ruby.md`: keep ours, where the lists of libraries
  point at the plan page; what a library's paragraph there said about its
  differences from Ruby goes into its row of the plan page;
- work logs and a wrapper file a later unit dropped: drop;
- anything else: resolve by hand, keeping the replay's own names (one
  `userMethod` lookup, `DefineNative` for every top-level name, the
  helpers in `src/stdlib` instead of a wrapper).

A helper the archive added late but an early unit needs (`Context.Const`,
`stdlib.Module`, `stdlib.DeclareFields`) comes with its first user in the
replay, and the description says so.

**Write.** Methods written in cameo use the multi-line `def` form, not
`def x = …`. A block captures values, so a loop that accumulates is a
fold, a `while`, or a Hash built with `merge`; an Array built by `<<` is
quadratic.

**Check.** `make check` must be green; read its exit status, not its
last line.

**Describe.** The pull request's description follows the template in
[agents/WRITING.md](../../../agents/WRITING.md): a summary line, `## What`,
`## Why`, `## Test`, `## Notes`. The original's description is the
source, with its stacking and generator lines removed, its *Checked* as
*Test*, its *Where cameo differs* folded into *What*, and a note
"Replays #N in the reordered history on <tag>" plus what the replay
changed. The title is a sentence in the third person: "lib: adds
digest", not "lib: add digest". The commit's message is the title and the
description.

**Review.** Run `/code-review <pr>` and wait for it; fix every finding in
the unit (a finding in a lower unit is fixed there, and every unit above
rebased), add a test for each fix, verify against the Ruby on the
machine, and say in the description what the review found. A finding the
unit cannot fix, a limit of the language or a core behaviour, gets an
issue and a sentence in the library's row of the plan page. A finding
the repository documents as a decision is left, and the report says why.

**Rebase.** Fixing a lower unit means `git rebase --onto` every unit
above it, bottom-up, and `git push --force-with-lease` each; the lease is
the branch's commit on origin, so fetch first.

Review as you go: one unit at a time, its findings fixed before the next
is built, so a lower fix costs one rebase, not ten.

## 4. Measure

On the top of the stack, with the binary copied out of the tree so a
reviewer switching branches cannot disturb the run:

```sh
make build && cp bin/cameo /tmp/cameo-final
ruby specs/run.rb --cameo /tmp/cameo-final --ruby ruby --status /tmp/status.tsv --out /tmp/specout
ruby tests/performance/run.rb
```

The first is ruby/spec's coverage of core Ruby: what passes, what fails
by design (`specs/runner/classify.rb` names the decisions), what is not
done yet; it goes into `doc/coverage.md`, with `specs/status.tsv` updated.
The library coverage is the plan page's tables. The second is the
performance table for this machine, which goes into
`tests/performance/README.md`. Run them one after the other: the
performance run wants the CPU to itself.

## 5. Finish

A last unit holds the documentation: the work logs of the archive with
their outcome set, a log of the reorder itself, the coverage and
performance reports, and this skill. Then merge the stack with
[merge-stack](../merge-stack/SKILL.md) from the bottom pull request, and
report: the pull requests, what each review found and what was left with
its reason, the issues filed, and the reports' numbers.

## What went wrong the first time

- The registry merge in the replay script had a stray colon in a git path
  and silently skipped every library's registration; the first unit that
  failed to build revealed it. Read a script's output, and test a helper
  on one unit before trusting it on ten.
- A pull request's description template was ignored for twenty units
  until a reviewer read the rules; read [agents/WRITING.md](../../../agents/WRITING.md)
  before the first description.
- A commit made with `reset --soft` holds the index alone: stage the
  unit's paths before it, or the unstaged fixes stay behind and the pull
  request does not build.
- A pick of a late commit brings the archive's neighbouring table rows in
  through a conflict's "theirs" side; move rows by name, never by hunk.
- Staging everything with `git add -A` committed the cloned ruby/spec
  checkout as an embedded repository; stage explicit paths, as
  agents/CLAUDE.md says, and exclude the clone in `.git/info/exclude`.
