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
- [14. `Array.new`, `Array[]` and `Hash.new`](14-array-hash-new.md)
- [15. Random numbers from a seedable generator](15-random.md)
- [16. `String#succ` and ranges of strings](16-string-succ.md)
- [17. `Time`, an immutable instant with its offset](17-time.md)
- [18. `Hash.new` with a default value or block](18-hash-defaults.md)
- [19. `Math`, `Range#bsearch`, `Float#step` and other builtins](19-small-builtins.md)
- [20. Linear maps, and the benchmark bugs fixed](20-linear-maps.md)
- [21. Argument forwarding, `for`, `alias` and other sugar](21-sugar.md)
- [22. A mutating call on a local rebinds it](22-mutating-calls.md)
- [23. `Class.new` builds a class at run time](23-class-new.md)
- [24. `Regexp` and `MatchData` on Go's regexp](24-regexp.md)
- [25. Integers of arbitrary precision](25-bignum.md)
- [26. Lazy enumerators](26-lazy.md)
- [27. `require` of built-in libraries, CGI escaping and Time parsing](27-require.md)
