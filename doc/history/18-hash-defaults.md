# 18. `Hash.new` with a default value or block

`counts = Hash.new(0)` and `memo = Hash.new { |h, k| h[k] = compute(k) }`
are two of Ruby's most common idioms, and the second one mutates the hash
inside the block, which amber cannot allow.

## Defaults

`Hash.new(value)` and `Hash.new { |hash, key| … }` give a hash a default,
which `[]`, `dig` and `values_at` fall back to for a missing key, and which
`default` and `default_proc` read. The default travels with the copies
that `[]=`, `merge` and `compact` make, since those are the same hash with
a change; a hash built anew, as `select` or `map` build one, has none, as
in Ruby. [examples/hash_defaults.arb](../../examples/hash_defaults.arb) shows
each case.

With a default, `counts[w] = counts[w] + 1` is the counting idiom amber
writes, since `h[k] = v` on a local rebinds it (step 2).

## The memoising block

In Ruby, `Hash.new { |h, k| h[k] = v }` stores into the hash the block is
given. In amber that store could only rebind the block's parameter `h`,
and the hash the caller holds would not change, so the block would compute
the value again on every access while looking as if it cached it. The
checker rejects it ([src/syntax/convert.go:35](https://github.com/radiospiel/amber/blob/95054594acc616b33d0d64937c439428c2a55014/src/syntax/convert.go#L35-L37)),
as it rejects any mutation, and the block returns the default instead:
`Hash.new { |h, n| n * n }`.

The envelope carries a default value; a hash with a default block cannot
travel, since a block cannot. The two `hash_defaults` benchmark cases move
to `works/`.
