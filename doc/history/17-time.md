# 17. `Time`, an immutable instant with its offset

Ruby's `Time` is mutable in two places: `utc` and `localtime` change the
receiver's zone. Everything else about it is a value, an instant and an
offset, which is what amber's `Time` is.

## What a time is

`Time.now`, `Time.at` and `Time.new` (with `in:` and a trailing zone in
Ruby's forms: `"+09:00"`, `"UTC"`, `"Z"`, a military letter) make a time in
UTC, the local zone or a fixed offset
([src/stdlib/time.go:17](https://github.com/radiospiel/amber/blob/@@COMMIT@@/src/stdlib/time.go#L17-L282)). A time compares, hashes
and is equal as an instant, so two times in different zones for the same
moment are `==`, as in Ruby. `strftime` follows Ruby's directives and
flags, and `to_s` and `inspect` print what Ruby prints.

Where Ruby's method mutates, amber's returns a new time: `utc` and
`localtime` behave as `getutc` and `getlocal`. A script that wrote
`t.utc; puts t` sees the old zone, which is the one difference from Ruby a
script can observe, and the design records it.

## Fractions of a second

`subsec` is a Float, since amber has no `Rational` (step 13 left it out).
A float of seconds given to `Time.at` becomes nanoseconds exactly, rounded
down as Ruby shows its exact rational, so `Time.at(1.1).nsec` is
`100000000` in both.

## Across the wire

The envelope carries a time, as seconds, nanoseconds, offset and zone name,
so a time crosses `async`, and the host API of step 6 maps it to Go's
`time.Time`. A field type for times does not exist yet.

All 14 benchmark cases under `stdlib/time` move to `works/`.
