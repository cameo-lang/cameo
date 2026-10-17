# 29. Path functions and `Pathname`

53 benchmark cases sat in `unsupported/filesystem`, and step 1 had left out
everything that touches the file system. Many of them did not touch it:
`File.join`, `File.basename` and `Pathname#cleanpath` are arithmetic on
strings that name files, not file access.

## Arithmetic, not I/O

`File` gains `join`, `basename`, `dirname`, `extname`, `split`,
`expand_path` and `absolute_path` ([src/stdlib/path.go:236](https://github.com/radiospiel/amber/blob/bc9317ad4f17bd4546a89a84bf7f28ac2e06924b/src/stdlib/path.go#L236-L347)).
None of them opens a file. Only `expand_path` reads anything, the working
directory and `$HOME`, to resolve a relative path or `~`, and that is read,
not written.

`Pathname` is an immutable path whose `+`, `join`, `cleanpath`, `parent`
and `relative_path_from` follow Ruby's
[pathname.rb](https://github.com/ruby/ruby/blob/master/ext/pathname/lib/pathname.rb),
the pure-Ruby half of the library, line by line in their edge cases:
`Pathname("a") + "/b"` is `/b`, and `Pathname("a/../..").cleanpath` is
`..`. `__FILE__` is the script's path as it was given and `__dir__` its
directory, which `File`'s functions take apart.

[examples/paths.arb](../../examples/paths.arb) has 63 lines of expected
output, ruby's. 22 filesystem cases move to `works/`, which holds 781; the
31 left read or write files and stay out.

## Out of order

In ark, this step came after the first half of encodings, the step that
added a `Bytes` class for binary strings. This repository brings
encodings in as one step, the next, and keeps paths before it, so this
step's diff is the path functions alone.
