# 31. `with` makes the copy with changes, `tap` is Ruby's

The one design decision of step 1 that did not last. The original request
gave `tap` a new meaning: `point.tap { |p| { x: p.x + 1 } }` returned a
copy of `point` with the hash the block returned applied (step 5). Ruby's
`tap` yields its receiver and returns it unchanged, for a side effect in
the middle of a chain:

```ruby
items.map(&:price).tap { |prices| puts prices.inspect }.sum
```

That idiom appears throughout Ruby code, including the benchmark suite's
`kernel_tap`, and amber's `tap` broke it: the block returned `nil`, not a
hash, and `tap` raised.

## The decision

Ruby already has a name for a copy with changes:
[`Data#with`](https://docs.ruby-lang.org/en/3.3/Data.html#method-i-with),
since Ruby 3.2, which takes the changes as keywords. amber's `with` takes
them as keywords, or from the hash its block returns, keywords first
([src/eval/fields.go:461](https://github.com/radiospiel/amber/blob/@@COMMIT@@/src/eval/fields.go#L461-L502)):

```ruby
moved = point.with(x: 10)
moved = point.with { |p| { x: p.x + 10 } }
```

and `tap` is Ruby's again: it yields the receiver and returns it. The
rewrite of `a.x.y = 3` (step 2) produced `With` nodes all along, so it
needed no change; only the method a script calls was renamed.

A `tap` whose block ends in a hash literal, the old spelling, is almost
certainly a script written for the old meaning, so the checker rejects it
with a pointer to the new one
([src/syntax/convert.go:506](https://github.com/radiospiel/amber/blob/@@COMMIT@@/src/syntax/convert.go#L506)):

```
tap returns its receiver unchanged; for a copy with changes use with
```

## What stays rejected

The same round decided a neighbouring question: a mutating call on a
temporary, `s.dup.merge!(x)`, `[1, 2].push(3)`, `foo.bar[0] = 1`, stays
rejected (step 22). There is no variable to rebind, and the non-mutating
form, `merge`, `+`, `with`, says what is meant.

Had `with` been the name from step 1, every example and history document
since step 5 would read as Ruby does. The design records the change, and
`kernel_tap` moves to `works/`.
