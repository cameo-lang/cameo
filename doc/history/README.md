# How amber came to be

amber was first built inside [radiospiel/ark](https://github.com/radiospiel/ark),
a monorepo, under `parts/golang/amber`, as a stack of pull requests. This
repository replays that history one step at a time, and each step has a
document here: what the step did, why, and the code it is about. A code
reference in a document points at the commit the step merged as, so it shows
the code as it was then, not as it is now.

The replay keeps the order in which things were built, optimisations
included. It differs from the history in ark in three places, where a
decision ark made late was the right one from the start:

- Lists and hashes are Ruby's `Array` and `Hash` from the first step. ark
  called them `List` and `Map` until step 14.
- Regular expressions run on Go's `regexp` from their first step,
  step 24. ark first tried an engine that needed a newer Go.
- Strings carry their encoding from the step that brings encodings,
  step 30. ark first added a separate `Bytes` class and
  removed it again.

The steps:

- [1. The language design, and `amber check`](01-language-design.md)
- [2. The front end: amber's tree, the checker and the rewrite](02-front-end.md)
- [3. The evaluator core and the scalar library](03-evaluator.md)
- [4. The collection library](04-collections.md)
- [5. Classes with typed fields, copies through `tap`, and modules](05-classes.md)
- [6. Every class is a protobuf message; the host API](06-protobuf.md)
- [7. `async` and `await`](07-async.md)
- [8. A method cache per class](08-method-cache.md)
- [9. Two allocations per method call instead of seven](09-allocations.md)
- [10. Each node compiles to a closure on first use](10-closure-compiler.md)
- [11. Integer intermediates stay unboxed](11-unboxed-intermediates.md)
- [12. A Value is a tagged struct](12-tagged-values.md)
- [13. CRuby's benchmark suite, sorted by what amber lacks](13-ruby-benchmarks.md)
