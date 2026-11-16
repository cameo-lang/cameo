# Weekly — what to rerun once a week

Some of what the repository states is measured, not written: it goes
stale while the code moves on, and no test fails when it does. Once a week
an agent reruns each measurement below, and opens a pull request with
what changed. A week without changes still ends on a sentence to the
human: what ran, and that nothing moved.

## Compatibility with ruby/spec

[COMPATIBILITY.md](../COMPATIBILITY.md) and the site's
[compatibility page](../doc/gh_pages/compatibility.html) say how much of
ruby/spec cameo passes, and why it does not pass the rest. Both are
written by [specs/compatibility.rb](../specs/compatibility.rb) from
[specs/status.tsv](../specs/status.tsv), which
[specs/run.rb](../specs/run.rb) writes from a run of every spec file.

### Run it

It needs a Ruby with Prism (3.3 or later), and takes about a minute on
four cores.

```sh
git fetch origin && git switch -c claude/compatibility-$(date +%Y%m%d) origin/main
make ruby-spec        # clones ruby/spec, or fetches the commit COUNTS.md names
make spec-tests       # the harness's own tests: stop here if one fails
make spec-status      # runs every spec, rewrites status.tsv, COMPATIBILITY.md and compatibility.html
git diff --stat
```

The run keeps to the ruby/spec commit [specs/COUNTS.md](../specs/COUNTS.md)
names, so a change in the numbers is a change in cameo or in the harness,
never in ruby/spec. Moving to a newer ruby/spec is a change of its own,
with the sorting `specs/README.md` describes.

### Read the diff

`git diff specs/status.tsv` lists every example whose status changed.
For each:

- **bug to pass**: a fix landed. Nothing to do.
- **pass to bug**: a regression. Find the commit that made it (`git log`
  since the last refresh, or `git bisect` running
  `ruby specs/run.rb --ruby ruby --out /tmp/one core/string/x_spec.rb` on
  the one file), and fix it in its own pull request,
  or open an issue that names the commit, the spec and the message.
- **bug to expected, or expected to bug**: classify.rb's rules met a new
  message. When cameo leaves the construct out on purpose, as
  `doc/cameo-vs-ruby.md` says, add the message to its rule in
  [specs/runner/classify.rb](../specs/runner/classify.rb), with a test in
  `specs/test/classify_test.rb`. When cameo should run it, it is a bug:
  leave it so.
- **a new reason**: `compatibility.rb` warns about a reason without a
  description. Give it a code, a name, what it means and the document
  that states the rule in `REASONS`, and the site a page for the code
  under `doc/gh_pages/compatibility/`.
- **anything to harness**: the prelude lost an idiom, or CRuby changed.
  Read the message before you accept it.

Run `make spec-status` a second time when you changed classify.rb: the
second run must not change status.tsv again.

### Open the pull request

One pull request, `specs: refreshes COMPATIBILITY.md for <date>`, with
`specs/status.tsv`, `COMPATIBILITY.md`, `doc/gh_pages/compatibility.html`
and any classify.rb change. Its description gives the summary row before
and after (*all*: examples, pass, not expected, bug), names each
regression and what was done about it, and says which reasons changed.
When the pass count moved, update the figures the README and the site's
`numbers.html` quote from it.

When only the date in the header changed, there is nothing to merge:
close the branch, and tell the human the run found no change.
