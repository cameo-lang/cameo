# 14. `Array.new`, `Array[]` and `Hash.new`

The first of the `stdlib/` decisions of step 13. Literals were the only
way to make an array or a hash: `Array.new` and `Hash.new` failed with
"use a literal". The benchmarks build arrays with `Array.new(n) { |i| … }`
everywhere, and 57 cases sat in `stdlib/array` and `stdlib/hash`.

## The constructors

`Array.new` builds an empty array, `n` nils, `n` copies of a value, `n`
values of a block by index, or a copy of another array; `Array[]` lists its
arguments ([src/stdlib/list.go:11](https://github.com/radiospiel/amber/blob/@@COMMIT@@/src/stdlib/list.go#L11-L48)).
`Hash.new` without arguments builds an empty hash; with a default value or
a block it still fails, until step 18 decides how a default works without
mutation. `Kernel#Array` converts, as Ruby's does: `Array(nil)` is `[]`,
`Array(1..3)` is `[1, 2, 3]`.

## Ruby's names

In ark, this was the step that renamed amber's `List` and `Map` to Ruby's
`Array` and `Hash`: until then `[].class` printed `List`, and every
`inspect`, every error message and every constant lookup differed from
Ruby's, which the benchmark outputs made plain. This repository has used
Ruby's names since step 1, so here the step adds only the constructors and
a comment where the names are bound
([src/object/class.go:295](https://github.com/radiospiel/amber/blob/@@COMMIT@@/src/object/class.go#L295)).
The Go identifiers keep their names (`object.List`, `object.Map`), and so
do the protobuf field types `list` and `map` and the envelope's `List` and
`Map` messages, which are protobuf's vocabulary.

## The benchmarks

Of the 57 cases under `stdlib/array` and `stdlib/hash`, 25 now run with
Ruby's result and move to `works/`. The others move to the blocker they
reach now; one of them, `array_values_at_range`, is a bug in
`Array#values_at` with a range, filed under `bugs/` and fixed in step 20.
[examples/collections.arb](../../examples/collections.arb) gains the
constructors, with ruby's output as the expected output.
