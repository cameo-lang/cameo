# 16. `String#succ` and ranges of strings

`"a".."e"`, `"aa".succ` and `"Az".upto("Bc")` are common in Ruby and in 70
cases of the benchmark suite. `String#succ` looks simple and is not:
`"az".succ` is `"ba"`, `"zz".succ` is `"aaa"`, `"a9".succ` is `"b0"`,
`"1.9.9".succ` is `"2.0.0"`, and a string without letters or digits steps
by codepoint.

## A port of CRuby's rules

`succ` ports CRuby's `str_succ` from
[string.c](https://github.com/ruby/ruby/blob/master/string.c)
([src/stdlib/succ.go:26](https://github.com/radiospiel/amber/blob/@@COMMIT@@/src/stdlib/succ.go#L26)): it walks from the
right over the alphanumeric characters; a run that wraps carries into the
character to its left, with a new character inserted at the leftmost
wrapped one; a step skips one character of another kind; and a letter
after a digit stops the carry. The rules are Ruby's, not a simplification
of them, because a script that iterates `"a".."zz"` must see the same 702
strings in both languages.

## Ranges of strings

`String#upto` and ranges of strings and symbols share one iterator, after
CRuby's `rb_str_upto_each`, used by `each`, `to_a`, splats, Enumerable and
`include?`. `include?` walks the range; `cover?` and `===` compare its
ends, as in Ruby, which gives different answers for `("a".."z").include?("bb")`
and `cover?("bb")`.

Along the way `Range#min` and `#max` compare the ends of any range,
`String#inspect` escapes as Ruby does, and `Enumerable#zip` with a block
yields each tuple and returns `nil`.

[examples/succ.arb](../../examples/succ.arb) holds the cases, with ruby's
output as the expected output. Of the 70 benchmark cases under
`stdlib/string-range`, 28 move to `works/`; the rest stop on
`Array#pack`, `String#rpartition`, GC introspection or lazy enumerators.
