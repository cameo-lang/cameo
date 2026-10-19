# cameo against three other Rubies

[compare.rb](compare.rb) runs cameo's supported subset of CRuby's
benchmarks, the 852 files of [../ruby/works](../ruby/works), on four Rubies:

| Ruby | What it is |
|---|---|
| `mri` | the `ruby` on `PATH`; CRuby 3.4.11 for the results below |
| `cameo` | this component's interpreter |
| `warmruby` | CRuby 3.4 compiled to WASI, on wazero, from a module compiled ahead ([../warmruby](../warmruby)) |
| `rbgo` | go-embedded-ruby v0.6.0, a Ruby interpreter in Go ([../rbgo](../rbgo)) |

Every output is checked against CRuby 3.4's, which is warmruby's, since
cameo follows 3.4 where 3.3 differs. [results.tsv](results.tsv) has every
case; `ruby compare.rb --report results.tsv` shows it as the table.
`--only NAME` times one Ruby again and keeps the others' timings from the
table.

Each case is timed from inside the Ruby that runs it, by a harness
compare.rb writes around it: the case's prelude runs once, its script once
to warm up, and then the script in batches until the batches add up to a
second. Each batch reads the Ruby's own monotonic clock before and after,
and doubles until it takes a tenth of a second, so what a run costs beyond
the script is a counter's test and increment. A time is the mean of the
timed runs; no Ruby's start and no prelude is in it.

## Rerunning

[rerun.sh](rerun.sh) reruns every case on all four Rubies into
`results.tsv`, on Linux or macOS:

```sh
benchmarks/compare/rerun.sh                 # half the CPUs' worth of cases at once
JOBS=4 benchmarks/compare/rerun.sh          # or as many as JOBS says
benchmarks/compare/rerun.sh --only cameo    # time cameo again, keep the others
```

It needs Go and a Ruby of 3.1 or later on `PATH`; that Ruby is the one
measured as `mri`, so on macOS it is Homebrew's rather than the system's
2.6. The first run downloads warmruby's module. It prints the machine and
the versions first, which the results do not record.

## Results

On 2026-10-19, eight cases at a time on a Hetzner ccx43 (16 dedicated AMD
EPYC-Milan vCPUs), mri being CRuby 3.4.11 and cameo 65b0249 (see
[../hetzner](../hetzner)). The starts the harness keeps out
of every time were mri 0.038 s, cameo 0.006 s, warmruby 0.715 s and rbgo
0.064 s.

| | mri | cameo | warmruby | rbgo |
|---|---|---|---|---|
| ran cleanly | 843 | 852 | 838 | 815 |
| failed or timed out | 9 | 0 | 14 | 37 |
| output differs from CRuby 3.4 | 0 | 0 | 0 | 0 |
| total seconds, one run of each case | 81 | 262 | 408 | 653 |
| geometric mean of time vs mri | 100% | 281% | 732% | 994% |
| fastest | 666 | 168 | 1 | 8 |
| slowest | 5 | 59 | 388 | 391 |

cameo takes a little under three times MRI's time on the typical case, a
geometric mean of 281% over the 843 cases both run, and is faster in 174 of
them. That holds across sizes: 276% on the 744 cases MRI runs in under a
millisecond, 279% on the 31 it takes half a second or more, where warmruby
takes 474% and rbgo 460%. cameo is faster than MRI in 3 of those 31
(`vm_dstr_obj` 56%, `time_subsec` 69%, `string_fstring--fstring_random`
91%). The same suite on main (6ffe953) the same day gave cameo 283% and 256.6 s;
that run is not committed.

The earlier harness took a whole process per run and subtracted the Ruby's
start, which put cameo at 104% of MRI: on a case of a few microseconds it
measured mostly the noise of the start. Timed inside the Ruby, that noise
is gone.

Of the cases MRI takes at least 0.1 s, cameo is slowest on

- hashes of a few entries built in a loop (`hash_literal_small8` 56×,
  `hash_small8` 32×, `hash_literal_small4` 9×, `hash_small4` 8×), and
  `values_at` with a range (`array_values_at_range` 12×),
- method calls in deep recursion (`app_tarai`, `app_tak` and `app_fib`
  10×),
- hash lookups (`hash_ident_str` and `hash_ident_sym` 7×, `hash_aref_fix`
  6×),
- sorting with a block (`array_sort_block` 7×); sorting floats, which needs
  no dispatch, takes 109% (`array_sort_float`).

[hotspots.rb](hotspots.rb) runs the slow cases alone, and one of cameo's
own, [cases/array_sort_string.rb](cases/array_sort_string.rb): the suite
sorts no strings by `<=>`, so it sorts 1000 random strings of up to eight
characters 10000 times, as `array_sort_float` does floats.

What the others fail on:

- MRI 3.4 fails the eight `Pathname()` cases, which need
  `require "pathname"`, and `CGI.escape_html`, which needs `require "cgi"`.
- warmruby fails the same nine. Its VM fails five more when the harness
  runs them many times: `app_raise`, `vm_raise1` and `vm_raise2`, and
  `set--divide_2arity_100` and `_1000`. Each passes when run once.
- rbgo fails `CGI.escape_html`, `Pathname`, lazy `zip`, `Time.strptime`,
  reverse `each` over Bignum ranges and a few more, and times out on two
  endless-range `bsearch` cases.
