# 32. User classes that include Enumerable

Ruby's most common way to make a collection class is two lines: `include
Enumerable` and a `def each`. Every Enumerable method then works on the
class. In amber, Enumerable was one Go implementation over the builtin
collections (step 4), and a user class that included it got methods that
did not know how to read it.

## Enumerable over `each`

A user class that includes Enumerable and defines `each` now gets
Enumerable's methods. They run on an enumerator (step 26) over the class's
`each`: the enumerator calls `each` with a block that hands each element to
the method ([src/stdlib/enumerable.go:860](https://github.com/radiospiel/amber/blob/30b5ca7c897b598ea0d66f75e65aba48d733e30d/src/stdlib/enumerable.go#L860-L885)).
For that, a builtin passes a Go function as a block, which the evaluator
calls like any other block: `Context.Block` turns a Go function into a
Proc.

Because the enumerator streams, `first`, `find`, `take_while`, `include?`
and `lazy` stop as soon as they have their answer, and an endless `each`
works with them:

```ruby
class Naturals
  include Enumerable
  def each
    n = 0
    loop { yield n += 1 }
  end
end
Naturals.new.lazy.select(&:even?).first(3)   # [2, 4, 6]
```

The other methods read everything `each` yields, as Ruby's do.
`to_enum` and `enum_for` make the same enumerator explicitly, and
`each_with_index` without a block returns one.

[examples/user_enumerable.arb](../../examples/user_enumerable.arb) has a
finite and an endless class, with ruby's output as the expected output.
