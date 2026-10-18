# ruby/spec, sorted by what cameo's checker accepts

All of [ruby/spec](https://github.com/ruby/spec), the specification of the
Ruby language and its core and standard libraries, sorted by `cameo check`
alone. **No spec has been run.** A file in `supported/` is one the checker
accepts; whether its examples pass is for a run to find out.

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

- A `require` of a library cameo does not build in (anything but `set`,
  `time`, `pp`, `pathname` and `cgi`) files it under `unsupported/require`.
- Otherwise the checker's first message names the directory, through the
  table in `sort.rb`'s `reason`: instance variables, which cameo leaves
  out, global variables, mutating methods, singleton methods and so
  on. A message the table does not know goes to `unsupported/other`.
- A parse error goes to `bug/` when `ruby -c` accepts the file, and to
  `unsupported/newer-syntax` when MRI rejects it too.

The `require_relative` lines that load mspec's `spec_helper` and a spec's
fixtures are the harness's business, so they do not count against a file.
mspec's own idioms do count: a `before` block that sets `@object` uses an
instance variable, and that alone files most of the 852 specs under
`unsupported/instance-variables`. A harness that runs the specs on cameo
will have to give those blocks somewhere to keep their state.

## Layout

Each directory mirrors ruby/spec's tree, so a spec keeps its path, and holds
the support files its specs load: `spec_helper.rb`, the top-level
`fixtures/` and `shared/`, the `fixtures/` and `shared/` next to each spec,
and what a spec loads with `require_relative`, followed through. A support
file is copied into every directory that needs it.

## Where the files come from

ruby/spec at the commit named in [COUNTS.md](COUNTS.md). Re-sort a checkout
with:

```sh
git clone --depth 1 https://github.com/ruby/spec.git /tmp/ruby-spec
ruby sort.rb --spec /tmp/ruby-spec
```

ruby/spec is under the MIT license; a copy of it, `LICENSE`, is at the root
of each directory.
