# 21. Argument forwarding, `for`, `alias` and other sugar

The second round of decisions (step 20) began with the cheapest part of
`unsupported/`: constructs the front end rejected although they are sugar
over what amber already has. Each is a rewrite in the converter, so the
evaluator does not change.

| Ruby | becomes |
|---|---|
| `def fw(...) = target(...)` | the parameters and arguments it stands for: `*`, `**` and `&` |
| `def f(*, **, &)` and `target(*, **, &)` | anonymous parameters, forwarded by position |
| a top-level `def self.x` | a top-level method, since the top-level `self` is the script |
| `for x in xs … end` | `xs.each { |x| … }`, with `x` bound in the enclosing scope, as Ruby's `for` binds it |
| `alias new old` | a second name for a method, for a name not yet taken |
| `class A::B` inside an existing `A` | `class B` inside `module A` |

`alias` was left out in step 1 because a method is defined once. A new
name for an existing method defines nothing twice, so the rule holds, and
`alias` of a name already taken stays an error.

Three methods that only matter for mutable objects become harmless:
`freeze` returns its receiver and `frozen?` is true, since every value is
frozen; `compare_by_identity` returns the hash or set as it is, since amber
values have no identity to compare by. `:sym.to_proc` returns the proc that
`&:sym` makes.

[examples/sugar.arb](../../examples/sugar.arb) has each form, with ruby's
output as its expected output. 47 benchmark cases move to `works/`.
