# amber against three other Rubies

[compare.rb](compare.rb) runs amber's supported subset of CRuby's
benchmarks, the 824 files of [../ruby/works](../ruby/works), on four Rubies:

| Ruby | What it is |
|---|---|
| `mri` | the `ruby` on `PATH`; CRuby 3.3.6 for the results below |
| `amber` | this component's interpreter |
| `warmruby` | CRuby 3.4 compiled to WASI, on wazero, from a module compiled ahead ([../warmruby](../warmruby)) |
| `rbgo` | go-embedded-ruby v0.6.0, a Ruby interpreter in Go ([../rbgo](../rbgo)) |

Every output is checked against CRuby 3.4's, which is warmruby's, since
amber follows 3.4 where 3.3 differs. [results.tsv](results.tsv) has every
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
benchmarks/compare/rerun.sh --only amber    # time amber again, keep the others
```

It needs Go and a Ruby of 3.1 or later on `PATH`; that Ruby is the one
measured as `mri`, so on macOS it is Homebrew's rather than the system's
2.6. The first run downloads warmruby's module. It prints the machine and
the versions first, which the results do not record.

## Results

On 2026-10-18, two cases at a time on four cores, mri being CRuby 3.3.6.
The starts the harness keeps out of every time were mri 0.068 s, amber
0.005 s, warmruby 1.384 s and rbgo 0.106 s.

| | mri | amber | warmruby | rbgo |
|---|---|---|---|---|
| ran cleanly | 783 | 824 | 811 | 787 |
| failed or timed out | 41 | 0 | 13 | 37 |
| output differs from CRuby 3.4 | 0 | 0 | 0 | 0 |
| total seconds, one run of each case | 224 | 464 | 722 | 1085 |
| geometric mean of time vs mri | 100% | 265% | 708% | 1019% |
| fastest | 591 | 190 | 9 | 25 |
| slowest | 9 | 58 | 388 | 360 |

amber takes about two and a half times MRI's time on the typical case, a
geometric mean of 265% over the 783 cases both run, and is faster in 185 of
them. That holds across sizes: 257% on the 677 cases MRI runs in under a
millisecond, 269% on the 43 it takes half a second or more, where warmruby
takes 539% and rbgo 519%. amber is faster than MRI in 5 of those 43
(`gc_hash2` 22%, `vm_dstr_obj` 43%, `string_fstring--fstring_random` 80%).

The earlier harness took a whole process per run and subtracted the Ruby's
start, which put amber at 104% of MRI: on a case of a few microseconds it
measured mostly the noise of the start. Timed inside the Ruby, that noise
is gone.

Of the cases MRI takes at least 0.1 s, amber is slowest on

- hashes of a few entries built in a loop (`hash_small8` 29×,
  `hash_literal_small8` 28×, `hash_small4` 10×), and `values_at` with a
  range (`array_values_at_range` 27×),
- method calls in deep recursion (`app_tarai` 14×, `app_tak` 12×, `app_fib`
  11×),
- hash lookups (`hash_ident_str` 11×, `hash_ident_sym` 9×) and
  `Hash#shift` in a loop (`hash_shift_u24` 10×),
- sorting with a block (`array_sort_block` 9×); sorting floats, which needs
  no dispatch, takes 155% (`array_sort_float`).

[hotspots.rb](hotspots.rb) runs the slow cases alone, and one of amber's
own, [cases/array_sort_string.rb](cases/array_sort_string.rb): the suite
sorts no strings by `<=>`, so it sorts 1000 random strings of up to eight
characters 10000 times, as `array_sort_float` does floats.

What the others fail on:

- MRI 3.3 fails 32 `set` cases that use `it`, the block parameter of 3.4;
  the eight `Pathname()` cases, which need `require "pathname"`; and
  `CGI.escape_html`, which needs `require "cgi"`.
- warmruby fails the eight `Pathname()` cases and `CGI.escape_html`, as 3.4
  does without the `require`. Its VM fails four more when the harness runs
  them many times: an out-of-bounds memory access on `app_raise` and
  `vm_raise1`, and an exception raised outside the script on
  `set--divide_2arity_100` and `_1000`. Each passes when run once.
- rbgo fails `CGI.escape_html`, `Pathname`, lazy `zip`, `Time.strptime`,
  reverse `each` over Bignum ranges and a few more, and times out on two
  endless-range `bsearch` cases.
