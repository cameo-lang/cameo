# Task: blocks capture values; class attributes; option parsing in that world

**Started:** 2026-11-05 12:00:00
**Ended:** in progress
**Strategy:** Feature (TDD)
**Status:** In Progress
**Complexity:** Complex
**Used Models:** Fable 5.1
**Token usage (Estimated):** 400k input, 60k output

## Objective
A test that a proc reads its surrounding binding (#237) made the human
ask for the opposite: in an immutable language a block binds values, not
references, so `a = 3; l = proc { a }; a = 4; l.call` must return 3. The
decisions, taken example by example with the human: every block copies
the values of its variables when it is written; a block rebinding a
captured variable is refused by the checker (the Ruby idiom
`list.each { |x| total += x }` hung `loop do i += 1 … end` forever when it
silently did nothing); a lambda may call itself by the name it is
assigned to; any other block reading a name bound after it is refused.

The option parsers were built on handler blocks writing into outer
variables, so they need a value-returning form. The human wants a class
body to declare its switches, `on("--verbose") { |o, _| o.with(verbose:
true) }`, which needs a place on the class to record them: class
attributes, set with `self.name = v` while the class body runs, the way
initialize sets fields. The human asked for the docs to be explicit that
this makes a class deliberately unlike an instance.

## Plan: a stack on top of #236
1. Class attributes: `attribute :name, default:` in a class body,
   `self.name = v` from the body or a class method it calls, sealed after.
2. GetoptLong includes Enumerable and gains `GetoptLong::Options`, written
   in cameo, with `on` and `parse`; OptionParser's `on` without a block
   records into `options`.
3. Blocks capture values, with the checker rule and the forward reference;
   every test and example rewritten.

## Progress
- [x] Measured on main: capturing in proc/lambda literals only breaks 4 tests; in every block, 12, plus the loop hang
- [x] Option libraries (#241): GetoptLong includes Enumerable and takes args:; Getopt::Options in cameo with on and parse (it cannot nest in GetoptLong: a builtin class is shared by every program in the process); OptionParser#options records what a parse saw
- [x] Class attributes (#240): converter (`attribute`, `self.x = v` in class bodies and class methods), object.Class.Attrs and Open, evalAttrDecl, writeAttr, inheritance, reopening, Class.new; tests; docs
