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
case; `ruby compare.rb --report results.tsv` shows it as the table, and
`--whole` there shows whole processes, start included. `--only NAME` times
one Ruby again and keeps the others' timings from the table.

mri, amber and rbgo were measured in
[radiospiel/ark](https://github.com/radiospiel/ark); warmruby was then
measured again with [../warmruby](../warmruby), which replaced the command
used there.

## Results

On 2026-10-17, two cases at a time on four cores. A time is what a run of
the script costs without its Ruby's start: each Ruby first runs an empty
script, whose median time is its start (mri 0.107 s, amber 0.008 s,
warmruby 1.987 s, rbgo 0.131 s); each case then runs until its runs add up
to a second, and its time is the mean run less the start. warmruby and
rbgo report the script's own time, which is used for them instead.
warmruby's is taken inside the guest around `load`, so on a case of a few
milliseconds it carries none of the noise of subtracting a start; that is
most of the 314 cases it is fastest on.

| | mri | amber | warmruby | rbgo |
|---|---|---|---|---|
| ran cleanly | 783 | 824 | 813 | 785 |
| failed or timed out | 41 | 0 | 11 | 39 |
| output differs from CRuby 3.4 | 7 | 0 | 0 | 0 |
| total seconds | 303 | 671 | 1305 | 1459 |
| geometric mean of time vs mri | 100% | 104% | 124% | 660% |
| fastest | 441 | 187 | 314 | 0 |

Without the start, amber and MRI are close on the typical case, a geometric
mean of 104%, and amber is faster in 324 of the 783 both run; most of those
take MRI a few milliseconds. On the 71 cases MRI takes half a second or
more, amber takes 234% of MRI's time, warmruby 746% and rbgo 498%, and
amber is faster than MRI in 7 of them (`vm_dstr_obj` 34%, `gc_hash2` 41%).
Of the cases MRI takes at least 0.1 s, amber is slowest on

- sorting with `<=>` dispatched per comparison (`array_sort_float` 17×,
  `array_sort_block` 8×); floats and strings have since been sorted without
  dispatch, as integers were, and `array_sort_float` takes 141%,
- hashes of a few literal entries built in a loop (`hash_literal_small8`
  16×, `hash_small8` 13×), and `Hash#shift` in a loop (`hash_shift_u24`
  12×),
- method calls in deep recursion (`app_tak` 14×, `app_tarai` 13×,
  `app_fib` 11×) and blocks (`loop_each` 11×).

[hotspots.rb](hotspots.rb) runs those cases alone, and one of amber's own,
[cases/array_sort_string.rb](cases/array_sort_string.rb): the suite sorts no
strings by `<=>`, so it sorts 1000 random strings of up to eight characters
10000 times, as `array_sort_float` does floats. Measured with the others on
2026-10-17, amber takes 520% of MRI's time on it, against 1527% on
`array_sort_float` and 38% on `array_sort_int--ary1000`, the one sort that
needs no dispatch; warmruby takes 294% and rbgo 1519%. Sorting arrays of
floats or of strings without dispatch, with typed swaps, brought amber to
141% on `array_sort_float` and 171% on `array_sort_string`.

What the others fail on: MRI 3.3 lacks the core `Set` and a `Pathname`
without `require` that the benchmarks, from CRuby master, use; seven `set`
cases print 3.3's inspect format. warmruby fails the eight `Pathname()`
cases, which 3.4 has only after `require "pathname"`, `CGI.escape_html`,
which 3.4 lacks, and `app_raise` and `set--divide_2arity_1000`, which
crash its VM; rbgo fails
`CGI.escape_html`, lazy `zip` and `uniq`, and times out on four.
