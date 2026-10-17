# 28. `method_missing`

Step 1 listed `method_missing` with the metaprogramming amber leaves out,
next to `define_method` and `send`. 17 benchmark cases use it, and on a
closer look it does not belong in that list: it adds no method and changes
no class, it only answers a call the class has no method for. That fits a
language whose classes cannot change.

A call to a method a class lacks goes to the class's own `method_missing`,
with the name as a symbol, then the arguments, keywords and block
([src/eval/call.go:265](https://github.com/radiospiel/amber/blob/a3bbeb128be0d5103e91080f997b109cab9a5dec/src/eval/call.go#L265-L286)). `super`
from it reaches Object's, which raises the `NoMethodError` the call would
have raised. `respond_to?` asks `respond_to_missing?` for a name it does
not find, as Ruby's does, so a class that answers some names through
`method_missing` can say so.

[examples/method_missing.arb](../../examples/method_missing.arb) has a
proxy and a class answering by pattern. `String#chomp` takes a suffix in
the same step. The 17 cases move to `works/`, which holds 759.
