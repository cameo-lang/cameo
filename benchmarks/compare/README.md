# cameo against three other Rubies

[compare.rb](compare.rb) runs cameo's supported subset of CRuby's
benchmarks, the 781 files of [../ruby/works](../ruby/works), on four Rubies:

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

The run is older than two of cameo's rules: a block cannot assign to a
variable of the scope around it, and a bang method of Ruby's core is
refused. It timed the 852 cases `works/` held then, and
[results.tsv](results.tsv) still has a row for each, and the summary below
counts all of them. Four groups of those rows are stale:

- 2 rows are of the `string_valid_encoding_p--` cases, now under
  [../ruby/unsupported/invalid-strings](../ruby/unsupported/invalid-strings):
  their prelude writes bytes that are not UTF-8 in a string literal.
- 61 rows are of cases now under
  [../ruby/unsupported/mutation](../ruby/unsupported/mutation), whose timed
  script mutates: `bighash`, `gc_hash2`, `hash_long`, the four
  `hash_shift` cases, `hash_small2`, `hash_small4`, `hash_small8`,
  `kernel_tap--kernel_tap`, the six `string_gsub--` and six
  `string_scrub--` cases, and 38 `set--` cases (`add`, `add_exist`,
  `addq`, `addq_exist`, `clear`, `delete`, `delete_not_exist`, `deleteq`,
  `deleteq_not_exist`, `flattenb`, and each size of `collect`,
  `delete_if`, `keep_if`, `reject`, `replace`, `reset` and `select`).
- 12 rows are of `.rb` cases that now build their data without mutation.
  A `.rb` case is timed whole, so its time includes that build:
  `app_lc_fizzbuzz`, `gc_hash1`, `hash_aref_array`, `hash_aref_dsym`,
  `hash_aref_fix`, `hash_flatten`, `hash_ident_num`, `hash_ident_str`,
  `hash_ident_sym`, `hash_keys`, `hash_to_proc` and `hash_values`.
- 8 rows are of the `string_upcase--upcase-ascii8bit-` and
  `string_downcase--downcase-ascii8bit-` cases, now under
  [../ruby/unsupported/binary-strings](../ruby/unsupported/binary-strings):
  they change the case of a binary string, which cameo refuses.

The other 161 `set--` cases, the two `hash_first--` cases and the eight
`vm_call_kw_and_kw_splat--` cases changed only in their prelude, which the
harness does not time, so their rows still hold.

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

`hash_small8` and `hash_small4` are under `unsupported/mutation/` now, and
`hash_aref_fix`, `hash_ident_str` and `hash_ident_sym` build their hash
differently (see above).

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

## CRuby, CRuby with YJIT and cameo, by category

`compare.rb --rubies mri,yjit,cameo` times those three alone: `mri` is the
`ruby` on `PATH` with `--disable-yjit`, whatever `RUBYOPT` says, and `yjit`
the same binary with `--yjit`. Without warmruby among the Rubies the
reference output is MRI's. [categories.rb](categories.rb) prints a results
file as a table of categories, each cell the geometric mean of a Ruby's
time as a multiple of MRI's over the cases every Ruby ran:

```sh
ruby benchmarks/compare/compare.rb --rubies mri,yjit,cameo --jobs 3 --out benchmarks/compare/yjit.tsv
ruby benchmarks/compare/categories.rb          # yjit.tsv, as Markdown
```

[yjit.tsv](yjit.tsv) is the run of 2026-11-14 on a laptop, an Apple M4
with 10 cores, three cases at a time, CRuby being 3.4.1:

| Category | Cases | CRuby | CRuby + YJIT | cameo |
|---|---:|---:|---:|---:|
| Method calls, blocks and the VM | 192 | 1.00× | 0.94× | 3.28× |
| Small programs | 8 | 1.00× | 0.41× | 2.04× |
| Arrays and Enumerable | 161 | 1.00× | 0.98× | 1.30× |
| Hashes | 26 | 1.00× | 0.84× | 4.62× |
| Sets | 161 | 1.00× | 0.68× | 0.97× |
| Strings and regexps | 105 | 1.00× | 1.00× | 0.83× |
| Numbers | 34 | 1.00× | 0.93× | 1.82× |
| Ranges | 36 | 1.00× | 0.91× | 1.91× |
| Time | 28 | 1.00× | 0.90× | 0.85× |
| Libraries (File paths, Pathname, CGI) | 21 | 1.00× | 1.00× | 0.51× |
| **All** | 772 | 1.00× | 0.88× | 1.51× |

The nine cases MRI fails are the eight `Pathname()` cases and
`CGI.escape_html`, as above. Time all three in one run: a column timed
alone has a quieter machine than the others had, and its times are not
comparable with theirs. The README and the site's numbers page show this
table.
