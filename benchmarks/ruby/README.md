# CRuby's benchmarks, grouped by what amber lacks

CRuby's own benchmark suite, one runnable script per case, sorted by whether
amber runs it today and, when it does not, by the first thing that stops it.
It shows what to build next and holds amber to Ruby's behaviour where the two
overlap.

| Directory | Holds |
|---|---|
| `works/` | runs under amber with Ruby's result |
| `bugs/` | should run, and fails on a defect in amber |
| `stdlib/<what>/` | stops on a builtin class or method amber is to have and does not yet; `<what>` names it |
| `unsupported/<what>/` | uses a feature or builtin amber leaves out by design; `<what>` names it |

A builtin decided out, such as `Struct`, `Rational` and `Complex`, or
`RUBY_VERSION`, sits under `unsupported/`; the design document's "Decisions
taken" says why.

[MANIFEST.tsv](MANIFEST.tsv) gives every file its directory, the benchmark it
came from and the reason it is where it is.

## Where the files come from

The `benchmark/` directory of [ruby/ruby](https://github.com/ruby/ruby) at
commit `25fb329ec9ae07b804dfdb5e34f7c05dcc92186e` (2026-10-16). A `.rb`
benchmark there is one file here under its own name, and a file of `gc/`
takes the prefix `gc_`. Each entry of a `.yml` benchmark definition becomes
`<definition>--<entry>.rb`: the definition's prelude followed by the entry's
body, run once instead of in benchmark-driver's loop.

The 24 `so_*` benchmarks, and the YAML entries of `so_count_words`,
`so_k_nucleotide` and `so_reverse_complement`, are left out. They are old
copies of the Computer Language Shootout programs, and CRuby's `LEGAL` file
says their legal situation is unclear. Everything imported is under Ruby's
license, [COPYING](COPYING), or at your choice the 2-clause BSD license,
[BSDL](BSDL).

## How each file was classified

Every case ran under `amber check`, under CRuby 3.3.6 and under amber. A case
works when amber exits cleanly with Ruby's output. Most benchmarks print
nothing, so a clean exit proves little: every YAML case that ran cleanly was
run again printing its value, and the `.rb` benchmarks that ran cleanly were
checked by hand with their result printed. A case CRuby 3.3 cannot run
(master's benchmarks use Ruby 3.4 and later, such as `it` and the core `Set`)
was classified by what amber does with it.

A case that draws random numbers cannot match Ruby's values, since amber's
generator is not Ruby's: it works when it runs cleanly and its result holds
what holds for any draw (a sorted result is sorted, a sample has distinct
elements of the array). A case that reads the clock works when its result
has Ruby's form. `Time#subsec` returns a Float where Ruby returns a
Rational, which amber leaves out; the case that calls it never prints it.

A failing case is filed under the first thing that stops it. Fixing that may
uncover a second blocker, so a file can move from `stdlib/` to another
directory rather than to `works/`.

`bugs/` is empty: every case filed there has been fixed. A bare `Object`
prints as `#<Object>`, without the address Ruby shows, since amber values
have no identity; the cases that print one count as working.

Integers grow past 64 bits as Ruby's do, so `unsupported/bignum/` is gone;
its rational and imaginary literal cases are under `unsupported/complex/`.

## Running them

```sh
ruby run.rb                 # every directory
ruby run.rb works           # what must keep working; exits 1 when one fails
ruby run.rb --amber /path/to/amber --jobs 8 --timeout 60 stdlib   # default: 4 jobs, 300 s
```

[run.rb](run.rb) builds amber from this component unless `--amber` names a
binary, runs the files in parallel, prints the outcome and time of each, and
lists the files outside `works/` that now run cleanly. Such a file is a
candidate for `works/` once its result matches Ruby's; a clean exit alone is
not enough: a wrong result exits cleanly too.
