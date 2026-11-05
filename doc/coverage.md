# Coverage of Ruby, measured

What of Ruby cameo runs, measured on 2026-11-05 at the head of the stack
that replayed the work since v0.4.2 (#293), against CRuby 3.4.1 on this
Mac: the language and its core classes, by
[ruby/spec](https://github.com/ruby/spec). The standard library and the
speed have their own pages, named at the end. [cameo-vs-ruby.md](cameo-vs-ruby.md) lists the differences
these numbers come from; the run that made them is `make spec-status`.

## Core Ruby: ruby/spec's language and core

[specs/run.rb](../specs/run.rb) runs ruby/spec's `language/` and `core/`
under cameo and under CRuby, through [a prelude](../specs/runner/prelude.cameo)
that stands in for mspec. Its log counts the files: of the 2237 in the
two directories, cameo's checker accepted 1517 once mspec's idioms were
rewritten into calls of the prelude, 702 ran, and 17 of those crashed.
[specs/status.tsv](../specs/status.tsv) has one row per example that
printed a result, and one row, with the example `*`, per file the checker
refused; a file that ran and printed no example, because a guard skipped
it or its `before` failed, has none. [specs/runner/classify.rb](../specs/runner/classify.rb)
gives each row its status from the message that stopped it. The rows:

| | `language/` | `core/` | Both |
|---|---|---|---|
| files the checker refused | 71 | 1464 | 1535 |
| files with examples | 9 | 628 | 637 |
| examples | 116 | 2267 | 2383 |
| of those, examples CRuby passes in this harness | 92 | 2042 | 2134 |
| of those, examples cameo passes | 74 | 971 | 1045 |

run.rb's own summary says 1045 of 2302: it sums CRuby's passes over a
whole file, examples cameo stopped before reaching them included.

Most files the checker accepts do not run to an example: their `before`
blocks, their `mock` and `should_receive` need a state or a part of mspec
the prelude has not, and a refused file's one row says what stopped it.
The rows by status:

| Status | Rows | Means |
|---|---|---|
| `pass` | 1045 | passes under cameo, as under CRuby |
| `expected` | 2554 | stopped by something cameo leaves out on purpose; the reason is below |
| `harness` | 254 | 249 fail under CRuby in this harness too, 5 need `before` or `after` |
| `bug` | 65 | cameo should pass it; the list is below |

### What is left out on purpose

Each reason is a decision in [cameo-vs-ruby.md](cameo-vs-ruby.md) or
[design/cameo.md](design/cameo.md). A file stopped before it ran counts
once; an example that ran counts once.

| Rows | Reason |
|---|---|
| 878 | instance variables: mspec's `before` sets `@object`; a cameo class declares fields |
| 295 | metaprogramming: `eval`, `define_method`, `instance_eval`, `singleton_class`, `class << self`, `send(:initialize)` |
| 264 | mutation: `<<`, `[]=`, `push`, setters; every value is immutable, a call returns a copy |
| 197 | `Rational`, `Complex`, rational and imaginary literals, `to_r`, `to_c`, `Numeric#i` |
| 183 | visibility: `private`, `protected`, `module_function`, and the method lists by visibility; every method is public |
| 167 | the process: `Process`, `Signal`, `ENV`, `GC`, `ObjectSpace`, `fork`, `system`, backticks |
| 119 | encodings beyond UTF-8, binary and US-ASCII, and `force_encoding` |
| 97 | I/O: `IO`, `File`, `Dir`, `ARGF`, the standard streams as objects |
| 88 | `attr_accessor`, `attr_writer`, `attr`: a field declaration defines its reader, and there are no setters |
| 56 | threads: `Thread`, `Mutex`, `Queue`, `Fiber`, `Ractor`; cameo has `async` and `await` |
| 47 | object identity and freezing: `equal?`, `object_id`, `frozen?`, `FrozenError` |
| 34 | `Array.new`, `Hash.new`, `String.new` with arguments: use a literal |
| 32 | a block captures the values its variables have: it cannot rebind an outer variable |
| 27 | global variables, `$~` and `Regexp.last_match` |
| 22 | `Marshal`: cameo's wire form, not Ruby's bytes |
| 16 | `Struct` |
| 10 | `Data` |
| 7 | a subclass of `Numeric` |
| 5 | a regexp Go's RE2 cannot run: look-behind, back-references |
| 4 | a constant scoped by `::` on the left of an assignment |
| 3 | `ruby2_keywords` |
| 2 | class variables |
| 1 | `require_relative` and `load` |

### What is not done yet

The 65 rows with status `bug`, grouped. Seven are files the checker or
the parser refused; the rest are examples that ran.

- **The parser**: `core/kernel/backtick_spec.rb` and `raise_spec.rb` stop
  at an `end` and a `rescue` the parser does not expect, and
  `core/exception/syntax_error_spec.rb` at a fixture's assignment target.
- **The checker**: `def self.prepended` in a module body, `include` with
  two modules, a `def` inside a top-level method, `for` with a nested
  target.
- **Methods that do not exist**: `Enumerator.produce`,
  `Symbol.all_symbols`, `Time._load`, `Random#state`, `const_defined?`,
  `Encoding#ascii_compatible?`, `Encoding#dummy?`,
  `Encoding::InvalidByteSequenceError#incomplete_input?`, `IO#path`,
  `Set#compare_by_identity?`, `singleton_class` on `main`.
- **Wrong results**: `Hash#hash` of a recursive Hash; `Integer#upto` and
  `downto` with a String stop raise the wrong error; `Range#step` over
  Strings; `Time#localtime` given a UTC offset string; `Time#utc_offset`
  as a Rational; `Time#dup` of a subclass; `Range#dup` and a Range
  subclass's `frozen?`; `Regexp#source` keeps an escape Ruby drops;
  `String#to_sym` of a binary String; `Proc#to_s` names the file in full;
  `File.extname` returns a frozen String; `Set#initialize_clone` freezes;
  `BasicObject#!=` through `send`; `IO` does not include `Enumerable`;
  `MatchData` given a String subclass.

### What the harness cannot run

254 rows: 249 examples CRuby fails in this harness too, which run under
mspec alone, and 5 that need `before`, `after` or a shared example the
prelude does not have. They say nothing about cameo.

## The standard library

[stdlib.md](stdlib.md) is the measure, not this page: its *Available*
table lists the 34 libraries cameo builds in, each with the Ruby version it follows and,
in its row, what of it works, what is left out and why, and what its
review found. *Planned* lists the 4 next in order, and *Unsupported*
every other library of Ruby's standard library with the decision that
leaves it out.

## Performance

[tests/performance](../tests/performance) holds the scripts and this
Mac's table: the interpreter against CRuby 3.4.1 without YJIT, and
`cameo build`'s programs against CRuby with YJIT.
