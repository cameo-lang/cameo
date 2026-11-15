# ruby/spec, sorted by what cameo's checker accepts

All of [ruby/spec](https://github.com/ruby/spec), the specification of the
Ruby language and its core and standard libraries, sorted by `cameo check`
alone. A file in `supported/` is one the checker accepts; whether its
examples pass is what [run.rb](run.rb) finds out, and
[status.tsv](status.tsv) records.

| Directory | Holds |
|---|---|
| `supported/` | specs `cameo check` accepts |
| `bug/` | specs cameo's parser rejects although MRI parses them: a defect in cameo |
| `unsupported/<what>/` | specs that use a construct cameo leaves out, or require a library cameo does not build in; `<what>` names it |

[COUNTS.md](COUNTS.md) has the number in each directory, and
[MANIFEST.tsv](MANIFEST.tsv) lists every spec file with its directory and
the message that put it there.

## How a file is sorted

[sort.rb](sort.rb) runs `cameo check` on each `*_spec.rb` and files it under
the first thing that stops it:

- A `require` of a library cameo does not build in files it under
  `unsupported/require`. sort.rb asks cameo: it runs a script that
  requires the library, once per name, and a LoadError says no.
- Otherwise the checker's first message names the directory, through the
  table in `sort.rb`'s `reason`: instance variables, which cameo leaves
  out, global variables, mutating methods, singleton methods and so
  on. A message the table does not know goes to `unsupported/other`.
- A parse error goes to `bug/` when `ruby -c` accepts the file, and to
  `unsupported/newer-syntax` when MRI rejects it too.

The `require_relative` lines that load mspec's `spec_helper` and a spec's
fixtures are the harness's business, so they do not count against a file.
mspec's own idioms do count: a `before` block that sets `@object` uses an
instance variable, and that alone files most of the 1021 specs under
`unsupported/instance-variables`. A harness that runs the specs on cameo
will have to give those blocks somewhere to keep their state.

## Layout

Each directory mirrors ruby/spec's tree, so a spec keeps its path, and holds
the support files its specs load: `spec_helper.rb`, the top-level
`fixtures/` and `shared/`, the `fixtures/` and `shared/` next to each spec,
and what a spec loads with `require_relative`, followed through. A support
file is copied into every directory that needs it.

## Running the specs

[run.rb](run.rb) runs all of ruby/spec under cameo, `language/`, `core/`,
`library/`, `command_line/`, `security/` and `optional/`, or the
directories and files it is given:

```sh
ruby run.rb --ruby ruby --out /tmp/spec-run
```

cameo cannot reopen `Object`, so mspec does not load. In its place,
[runner/prelude.cameo](runner/prelude.cameo) has `describe`, `it`, the
guards, the numeric helpers and the matchers, as functions, and run.rb
rewrites each spec with Prism so that `a.should == b`,
`a.should.raise(E)`, `a.should.pred?(x)` and `a.should matcher` call them.
The fixtures a spec loads with `require_relative` are put in its place.
The rewritten program goes through `cameo check`, which accepts or rejects
the whole file, and then runs; each example prints one line, which run.rb
counts.

`before`, `after`, shared examples, `mock`, `should_receive`, `ScratchPad`,
`fixture` and `ruby_exe` need state or I/O an example cannot keep in cameo,
and are not in the prelude. A spec's `require` of a library stays: under
cameo it loads the library cameo builds in, or stops the file with a
LoadError. `--ruby` runs each program under CRuby as well,
which shows what the harness can run: the summary counts how many of the
examples CRuby passes also pass under cameo. The full run takes under a
minute on four cores, the spec files running on as many threads as
there are cores.

[test/](test) holds the tests of run.rb itself; `make spec-tests` runs
them with the built cameo. Like run.rb, they need a Ruby with Prism.

## Which examples pass, and which should

[status.tsv](status.tsv) has one row per example of all of ruby/spec,
and one row, with the example `*`, per file that did not run; its
`stage` column says what stopped that file: `check` (cameo's checker
refused it), `crash` (it stopped before its first example), `timeout`,
or `rewrite` (Prism could not parse it). Its status is one of:

| Status | Means |
|---|---|
| `pass` | passes under cameo, as under CRuby |
| `expected` | stopped by something cameo leaves out on purpose: mutation, instance variables, object identity, threads, I/O, `Rational`, a part of a library that [stdlib.md](../doc/stdlib.md) leaves out; the `reason` column names it |
| `harness` | CRuby does not pass it in this harness: it fails there too (`fails under Ruby too`, or `fails under Ruby` where cameo passes it), or CRuby does not run it (`not run under Ruby`), mostly an example ruby/spec guards for a newer Ruby; or it needs `before`, `after` or a shared example |
| `bug` | anything else: cameo should pass it |

[runner/classify.rb](runner/classify.rb) maps the message that stopped
an example to its status; a message it does not know is a bug.
[compatibility.rb](compatibility.rb) writes
[COMPATIBILITY.md](../COMPATIBILITY.md) from status.tsv: the counts for
each category of ruby/spec, the reasons, and where the bugs are.
Regenerate both after a change, and the diff of status.tsv shows what
the change fixed and broke:

```sh
make spec-status
```

It runs the copy of ruby/spec at the commit [COUNTS.md](COUNTS.md)
names, the one the files here were sorted at.

## Where the files come from

ruby/spec at the commit named in [COUNTS.md](COUNTS.md). [checkout.rb](checkout.rb)
clones it into `ruby-spec/` at the root of the main repository, which every
worktree shares, or pulls into the copy that is there, and prints the path. Re-sort
that copy with:

```sh
ruby sort.rb --spec "$(ruby checkout.rb)"
```

`make ruby-spec` runs `checkout.rb` alone.

ruby/spec is under the MIT license; a copy of it, `LICENSE`, is at the root
of each directory.
