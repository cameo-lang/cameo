# amber against three other Rubies

[compare.rb](compare.rb) runs amber's supported subset of CRuby's
benchmarks, the 824 files of [../ruby/works](../ruby/works), on four Rubies:

| Ruby | What it is |
|---|---|
| `mri` | the `ruby` on `PATH`; CRuby 3.3.6 for the results below |
| `amber` | this component's interpreter |
| `warmruby` | agentic's code-ruby: CRuby 3.4 for WASI on wazero, in a VM started ahead |
| `rbgo` | go-embedded-ruby v0.6.0, a Ruby interpreter in Go ([../rbgo](../rbgo)) |

The warmruby command links agentic, a module of
[radiospiel/ark](https://github.com/radiospiel/ark) under the AGPL, so it
lives there and not here; the results below were measured in ark with all
four. Here `compare.rb` runs the other three and checks outputs against
MRI's.

Every output is checked against CRuby 3.4's, which is warmruby's, since
amber follows 3.4 where 3.3 differs. [results.tsv](results.tsv) has every
case; `ruby compare.rb --report results.tsv` shows it as the table, and
`--whole` there shows whole processes, start included.

## Results

On 2026-10-17, two cases at a time on four cores. A time is what a run of
the script costs without its Ruby's start: each Ruby first runs an empty
script, whose median time is its start (mri 0.107 s, amber 0.008 s,
warmruby 1.789 s, rbgo 0.131 s); each case then runs until its runs add up
to a second, and its time is the mean run less the start. warmruby and
rbgo report the script's own time, which is used for them instead.

| | mri | amber | warmruby | rbgo |
|---|---|---|---|---|
| ran cleanly | 783 | 824 | 789 | 785 |
| failed or timed out | 41 | 0 | 35 | 39 |
| output differs from CRuby 3.4 | 7 | 0 | 0 | 0 |
| total seconds | 303 | 671 | 1994 | 1459 |
| geometric mean of time vs mri | 100% | 104% | 780% | 660% |
| fastest | 458 | 384 | 6 | 0 |

Without the start, amber and MRI are close on the typical case, a geometric
mean of 104%, and amber is faster in 324 of the 783 both run; most of those
take MRI a few milliseconds. On the 71 cases MRI takes half a second or
more, amber takes 234% of MRI's time, warmruby 1739% and rbgo 498%, and
amber is faster than MRI in 7 of them (`vm_dstr_obj` 34%, `gc_hash2` 41%).
Of the cases MRI takes at least 0.1 s, amber is slowest on

- sorting with `<=>` dispatched per comparison (`array_sort_float` 17×,
  `array_sort_block` 8×),
- hashes of a few literal entries built in a loop (`hash_literal_small8`
  16×, `hash_small8` 13×), and `Hash#shift` in a loop (`hash_shift_u24`
  12×),
- method calls in deep recursion (`app_tak` 14×, `app_tarai` 13×,
  `app_fib` 11×) and blocks (`loop_each` 11×).

[hotspots.rb](hotspots.rb) runs those cases alone.

What the others fail on: MRI 3.3 lacks the core `Set` and a `Pathname`
without `require` that the benchmarks, from CRuby master, use; seven `set`
cases print 3.3's inspect format. warmruby fails `sort_by`, where its VM
reports "failed to allocate memory", and a few others; rbgo fails
`CGI.escape_html`, lazy `zip` and `uniq`, and times out on four.
