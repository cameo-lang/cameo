# 23. `Class.new` builds a class at run time

26 benchmark cases build classes with `Class.new(Base) { … }`, most of them
to make many small classes in a loop. In step 1 `Class.new` was out, since
a class is identified by its body, and a block is not a class body to the
front end.

It fits once the block is treated as the body. `Class.new(Base) { … }`
builds a class as a class literal does: its block is the class body, with
fields and `def`s, and it is identified by the block's digest, so evaluating
the same `Class.new` twice, in a loop for instance, gives the same class
([src/eval/classes.go:105](https://github.com/radiospiel/amber/blob/@@COMMIT@@/src/eval/classes.go#L105-L144)). It takes the name
of the first constant it is bound to, `Point = Class.new { … }`, as Ruby's
does; an anonymous one prints as `#<Class>`, without the address Ruby
shows, since an amber value has none.

`Module.new` stays out. Its use is to `include` or `extend` at run time,
which is exactly what amber leaves out: a class's ancestry is fixed when
its body has run.

[examples/class_new.arb](../../examples/class_new.arb) has the forms, and
the benchmark cases under `unsupported/anonymous-classes` that only needed
this move on.
