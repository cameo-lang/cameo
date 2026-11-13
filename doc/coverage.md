# Coverage of Ruby, measured

What of Ruby cameo runs, measured on 2026-11-13 against CRuby 3.4.1 on a
Mac: the language and its core classes, by
[ruby/spec](https://github.com/ruby/spec). The standard library and the
speed have their own pages, named at the end. [cameo-vs-ruby.md](cameo-vs-ruby.md) lists the differences
these numbers come from; the run that made them is `make spec-status`.

## Core Ruby: ruby/spec's language and core

[specs/run.rb](../specs/run.rb) runs ruby/spec's `language/` and `core/`
under cameo and under CRuby, through [a prelude](../specs/runner/prelude.cameo)
that stands in for mspec. Its log counts the files: of the 2237 in the
two directories, cameo's checker refused 1536 once mspec's idioms were
rewritten into calls of the prelude, one could not be rewritten, 700
ran, and 10 of those crashed.
[specs/status.tsv](../specs/status.tsv) has one row per example that
printed a result, and one row, with the example `*`, per file that was
refused or crashed; a file that ran and printed no example, because a guard skipped
it or its `before` failed, has none. [specs/runner/classify.rb](../specs/runner/classify.rb)
gives each row its status from the message that stopped it. The rows:

| | `language/` | `core/` | Both |
|---|---|---|---|
| files refused or crashed | 72 | 1475 | 1547 |
| files with examples | 8 | 624 | 632 |
| examples | 110 | 2205 | 2315 |
| of those, examples CRuby passes in this harness | 92 | 1981 | 2073 |
| of those, examples cameo passes | 74 | 916 | 990 |

run.rb's own summary says 990 of 2129: it sums CRuby's passes over a
whole file, examples cameo stopped before reaching them included.

The run of 2026-11-05 passed 1045 of 2134. Since then a String with
bytes that are not UTF-8 has to be a binary String, written `b"..."`,
and a method that reads characters refuses other bytes. All 57 examples
that passed then and do not pass now stop at that rule: 30 are in files
refused whole for such a literal, and 27 raise when they run. Two
examples pass that did not.

Most files the checker accepts do not run to an example: their `before`
blocks, their `mock` and `should_receive` need a state or a part of mspec
the prelude has not, and a refused file's one row says what stopped it.
The rows by status:

| Status | Rows | Means |
|---|---|---|
| `pass` | 990 | passes under cameo, as under CRuby |
| `expected` | 2558 | stopped by something cameo leaves out on purpose; the reason is below |
| `harness` | 247 | 242 fail under CRuby in this harness too, 5 need `before` or `after` |
| `bug` | 67 | cameo should pass it; the list is below |

### What is left out on purpose

Each reason is a decision in [cameo-vs-ruby.md](cameo-vs-ruby.md) or
[design/cameo.md](design/cameo.md). A file stopped before it ran counts
once; an example that ran counts once.

| Rows | Reason |
|---|---|
| 870 | instance variables: mspec's `before` sets `@object`; a cameo class declares fields |
| 294 | metaprogramming: `eval`, `define_method`, `instance_eval`, `singleton_class`, `class << self`, `send(:initialize)` |
| 244 | mutation: `<<`, `[]=`, `push`, setters; every value is immutable, a call returns a copy |
| 206 | `Rational`, `Complex`, rational and imaginary literals, `to_r`, `to_c`, `Numeric#i` |
| 182 | visibility: `private`, `protected`, `module_function`, and the method lists by visibility; every method is public |
| 167 | the process: `Process`, `Signal`, `ENV`, `GC`, `ObjectSpace`, `fork`, `system`, backticks |
| 97 | I/O: `IO`, `File`, `Dir`, `ARGF`, the standard streams as objects |
| 95 | encodings beyond UTF-8, binary and US-ASCII, and `force_encoding` |
| 88 | `attr_accessor`, `attr_writer`, `attr`: a field declaration defines its reader, and there are no setters |
| 66 | binary Strings: bytes that are not UTF-8 are written `b"..."`, and a method that reads characters refuses them |
| 56 | threads: `Thread`, `Mutex`, `Queue`, `Fiber`, `Ractor`; cameo has `async` and `await` |
| 40 | object identity and freezing: `equal?`, `object_id`, `frozen?`, `FrozenError` |
| 34 | `Array.new`, `Hash.new`, `String.new` with arguments: use a literal |
| 31 | a block captures the values its variables have: it cannot rebind an outer variable |
| 27 | global variables, `$~` and `Regexp.last_match` |
| 20 | `Marshal`: cameo's wire form, not Ruby's bytes |
| 16 | `Struct` |
| 10 | `Data` |
| 5 | a regexp Go's RE2 cannot run: look-behind, back-references |
| 4 | a constant scoped by `::` on the left of an assignment |
| 3 | `ruby2_keywords` |
| 2 | class variables |
| 1 | `require_relative` and `load` |

### What is not done yet

The 67 rows with status `bug`, grouped. Seven are files the checker or
the parser refused; the rest are examples that ran.

- **The parser**: `core/kernel/backtick_spec.rb` and `raise_spec.rb` stop
  at an `end` and a `rescue` the parser does not expect, and
  `core/exception/syntax_error_spec.rb` at a fixture's assignment target.
- **The checker**: `def self.prepended` in a module body, `include` with
  two modules, a `def` inside a top-level method, `for` with a nested
  target.
- **Methods that do not exist**: `Enumerator.produce`,
  `Symbol.all_symbols`, `Integer#quo` and `Float#quo`, `Random#state`, `const_defined?`,
  `Encoding#ascii_compatible?`, `Encoding#dummy?`, `IO#path`,
  `Set#compare_by_identity?`, `singleton_class` on `main`.
- **Wrong results**: `Hash#hash` of a recursive Hash; `Integer#upto` and
  `downto` with a String stop raise the wrong error; `Range#step` over
  Strings; `Time#localtime` given a UTC offset string; `Time#utc_offset`
  as a Rational; `Time#dup` of a subclass; `Range#dup` and a Range
  subclass's `frozen?`; `Regexp#source` keeps an escape Ruby drops;
  `Proc#to_s` names the file in full;
  `File.extname` returns a frozen String; `Set#initialize_clone` freezes;
  `BasicObject#!=` through `send`; `IO` does not include `Enumerable`;
  `MatchData` given a String subclass.

### What the harness cannot run

247 rows: 242 examples CRuby fails in this harness too, which run under
mspec alone, and 5 that need `before`, `after` or a shared example the
prelude does not have. They say nothing about cameo.

## The standard library

[stdlib.md](stdlib.md) is the measure, not this page: its *Available*
table lists the 32 libraries cameo builds in, each with the Ruby version it follows and,
in its row, what of it works, what is left out and why, and what its
review found. *Planned* lists the 4 next in order, and *Unsupported*
every other library of Ruby's standard library with the decision that
leaves it out.

## Performance

[benchmarks/compare](../benchmarks/compare) times the interpreter on
CRuby's own benchmarks against CRuby, warmruby and rbgo, and holds the
table.
