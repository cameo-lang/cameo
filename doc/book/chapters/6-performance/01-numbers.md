## Numbers {#ch-numbers}

::: source
from `doc/gh_pages/numbers.html`
:::

How fast cameo is, and how much of Ruby it runs. Every number here comes from
a run that is in the repository, with the script that made it.

1.  [Speed: CRuby, CRuby with YJIT, cameo](#numbers-speed)
2.  [Compatibility](#numbers-compatibility)
3.  [Against other Rubies inside a Go program](#numbers-embedded)

### Speed: CRuby, CRuby with YJIT, cameo {#numbers-speed}

Time relative to CRuby, so lower is faster and 2.00× is twice CRuby's time.
Each cell is the geometric mean over the cases of a category.

| Category                              | Cases | CRuby | CRuby + YJIT | cameo |
|---------------------------------------|-------|-------|--------------|-------|
| Method calls, blocks and the VM       | 192   | 1.00× | 0.94×        | 3.28× |
| Small programs                        | 8     | 1.00× | 0.41×        | 2.04× |
| Arrays and Enumerable                 | 161   | 1.00× | 0.98×        | 1.30× |
| Hashes                                | 26    | 1.00× | 0.84×        | 4.62× |
| Sets                                  | 161   | 1.00× | 0.68×        | 0.97× |
| Strings and regexps                   | 105   | 1.00× | 1.00×        | 0.83× |
| Numbers                               | 34    | 1.00× | 0.93×        | 1.82× |
| Ranges                                | 36    | 1.00× | 0.91×        | 1.91× |
| Time                                  | 28    | 1.00× | 0.90×        | 0.85× |
| Libraries (File paths, Pathname, CGI) | 21    | 1.00× | 1.00×        | 0.51× |
| **All**                               | 772   | 1.00× | 0.88×        | 1.51× |

How it was measured:

-   **The cases** are those of CRuby's own `benchmark/` directory that use
    only what cameo supports, [781
    files](https://github.com/cameo-lang/cameo/tree/main/benchmarks/ruby/works).
    A case counts when all three ran it; cameo's output is checked against
    CRuby's.
-   **The machine** is a laptop, an Apple M4 with 10 cores, three cases at a
    time, on 2026-11-13. CRuby is 3.4.1, and "CRuby + YJIT" is the same binary
    with `--yjit`. A laptop's times vary more than a server's; read the table
    for its proportions, not its last digit.
-   **A time** is the mean of the timed runs of a case after one run to warm
    up, read from the Ruby's own clock, so no interpreter's start is in it.
    cameo starts in about 9 ms and CRuby in about 25 ms on this machine.
-   **To run it again:**
    `ruby benchmarks/compare/compare.rb --rubies mri,yjit,cameo --out benchmarks/compare/yjit.tsv`,
    then `ruby benchmarks/compare/categories.rb`.
    [yjit.tsv](https://github.com/cameo-lang/cameo/blob/main/benchmarks/compare/yjit.tsv)
    has every case.

cameo is an interpreter without a JIT. Where it spends its time is in
[benchmarks/compare](#ch-benchmarks-compare).

### Compatibility {#numbers-compatibility}

| Measured by                 | What counts                                                   | cameo        |
|-----------------------------|---------------------------------------------------------------|--------------|
| CRuby's benchmark suite     | cases that run with Ruby's output                             | 781 of 1318  |
| ruby/spec, all of it        | spec files cameo's checker accepts                            | 1855 of 3858 |
| ruby/spec, all of it        | examples that pass, of those CRuby passes in the same harness | 1558 of 2897 |
| Ruby 3.4's standard library | libraries built in                                            | 32 of 87     |

-   **The benchmark suite.** The 537 cases that do not run are sorted by the
    first thing that stops each: mutation (119), instance variables (61),
    `attr_*` accessors (59), metaprogramming (52) and smaller groups.
    [doc/cameo-vs-ruby.md](#ch-reference-cameo-vs-ruby) has the table.
-   **ruby/spec, the files.** Sorted without running a spec, on 2026-11-13.
    Most of the rest use an instance variable (1021 files), mostly in mspec's
    own `before` blocks, or require a library cameo does not build in (375).
-   **ruby/spec, the examples.** Run on 2026-11-15 against CRuby 3.3.6, every
    directory: 946 of 2017 pass in `language/` and `core/`, 610 of 877 in
    `library/`. An example counts only when CRuby passes it in the same
    harness, so the examples ruby/spec guards for Ruby 3.4 do not count.
    [Compatibility](#ch-compatibility) says, for each part of ruby/spec, what
    stops each example that does not pass, and which of those are bugs.
-   **The standard library.** Ruby 3.4.1 ships 58 default gems and 29 bundled
    ones. A library goes in when its work is computation on values, and stays
    out when it is the file system, processes or threads.
    [doc/stdlib.md](#ch-reference-stdlib) lists every one with its status.

### Against other Rubies inside a Go program {#numbers-embedded}

An older run, of 2026-10-19 on a 16-core server, compared cameo with two
Rubies that also run inside a Go program: CRuby compiled to WebAssembly, and
go-embedded-ruby. Over the cases all four ran, cameo took 2.9 times CRuby's
time, the WebAssembly build 7.2 times and go-embedded-ruby 9.9 times.
[benchmarks/compare](#ch-benchmarks-compare) has that run.
