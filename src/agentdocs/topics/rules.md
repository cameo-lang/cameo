# What to leave out of Ruby

| Do not write | Write instead |
|---|---|
| `@x`, `attr_accessor`, `attr_reader`, `def x=(v)` | a typed field: `int :x` in the class body; read it as `x` |
| `a << 1`, `a.push(1)`, `h.delete(k)`, `s << "x"` as statements | an assignment: `a += [1]`, `a = a << 1`, `h = h.except(k)`, `s += "x"` |
| `a.sort!`, `s.gsub!(…)`, `h.merge!(o)`: every bang method of Ruby's core | the copying form, assigned: `a = a.sort` |
| `list.each { \|x\| total += x }`: a block that assigns to a variable outside it | `sum`, `reduce`, `map`, `count`, `each_with_object`, or a `while` loop |
| `@@x`, `$x` | a constant, a field or a local |
| `Struct`, `Data`, `OpenStruct` | a class with typed fields |
| `private`, `protected`, `module_function` | nothing: every method a script defines is public |
| `define_method`, `eval`, `instance_variable_get`, `class << self`, `def obj.x` | a plain `def`; `def self.x` for a class method |
| `Thread`, `Mutex`, `Fiber`, `sleep`, `system`, backticks, `fork` | `async` and `await` |
| `File.read`, `File.write` unless you were told I/O is allowed; `Dir`, `ENV`, `gets`, `STDIN`, sockets | the functions the host gives the script; ask what they are |
| `require` of a library cameo does not build in, `require_relative`, `load` | one file per script; the libraries are in the topic `stdlib` |
| `Rational`, `Complex`, `1r`, `2i`, `ObjectSpace`, `GC` | none |
| `equal?`, `object_id` | `==`, which compares contents |
| `"\xFF"`, a literal with bytes that are not UTF-8 | a binary literal, `b"\xFF"` |
| `"v=#{bytes}"`, `format("%s", bytes)` with a binary String | `bytes.force_encoding("UTF-8")` first when the bytes are text; `+` to join bytes |
| `s.getbyte(i)` on text with characters beyond ASCII, `s.setbyte` on text | `s.b.getbyte(i)`; a binary String for `setbyte` |

`h[k] = v` and `a.x = 1` are assignments: they rebind `h` and `a` to a
changed copy. Any other variable that held the old value keeps it.
