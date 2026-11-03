# cameo and Ruby

cameo runs a subset of Ruby, with Ruby's syntax and, where cameo accepts a
construct, Ruby's behaviour, CRuby 3.4's where 3.3 differs. Everything cameo
accepts parses as Ruby. Where cameo differs it does so in one of three ways:

- **a different meaning**, almost all of it because every value is immutable;
- **rejected before the script runs**: the checker names the construct and,
  where there is one, the alternative;
- **a missing builtin**, which raises `NameError` or `NoMethodError` when the
  script reaches it (`Proc#curry` raises `NotImplementedError`, and
  `Integer()` with a base `ArgumentError`).

This page lists each, as of v0.1. The [design document](design/cameo.md)
says why; its *Decisions taken* records the choices made explicitly.

## How compatible, measured

**CRuby's benchmark suite.** [benchmarks/ruby](../benchmarks/ruby) holds the
1318 cases of CRuby's `benchmark/` that cameo may legally carry. 852 of them
run under cameo with Ruby's output. The others, by the first thing that stops
each:

| Cases | Reason |
|---|---|
| 52 | metaprogramming: `eval`, `define_method`, `class_eval`, `instance_variable_set` |
| 61 | instance variables |
| 59 | `attr_*` accessors |
| 58 | mutation of something other than a local variable |
| 33 | GC and VM introspection |
| 32 | `Module.new` |
| 31 | the file system |
| 24 | threads and fibers |
| 24 | `Rational` and `Complex` |
| 22 | `require` of a library cameo does not build in |
| 18 | reopening a builtin class |
| 10 | `Struct` |
| 9 | singleton methods outside a class |
| 9 | global variables |
| 8 | `Marshal` |
| 6 | `ruby2_keywords` |
| 5 | object identity |
| 3 | `ENV` |
| 1 each | class variables, `String#tr` with a hash |

**ruby/spec.** [specs](../specs) sorts all 3841 spec files of
[ruby/spec](https://github.com/ruby/spec) by what cameo's checker accepts:
1403 are accepted. None has been run, so this counts the constructs a spec
uses, not whether its examples pass. Most of the rest require a library cameo
does not build in (1006), or use an instance variable (852), mostly in
mspec's own `before` blocks.

## The same as in Ruby

- **Literals**: `nil`, `true`, `false`, integers of any size, floats,
  strings with interpolation and heredocs, symbols, arrays, hashes, ranges,
  regular expressions, `%w[]` and `%i[]`.
- **Control flow**: `if`/`unless`/`elsif`, modifiers, `case`/`when`,
  pattern matching with `case`/`in`, `while`/`until` and `begin … end
  while`, `loop` and `break` with a value, `next`, `redo`, `retry`, `catch` and
  `throw`, safe navigation `&.`, `BEGIN`.
- **Methods**: positional, optional, `*rest`, keyword, `**kwrest` and
  `&block` parameters; endless `def`; argument forwarding (`...`, `*`, `**`,
  `&`); `yield` and `block_given?`; a second `def` of a name replaces the
  first.
- **Blocks and procs**: `{ }` and `do … end`, numbered parameters (`_1`),
  `->` lambdas, `proc`, `&:sym`, `then` and `tap`.
- **Classes and modules**: single inheritance, overriding and `super`,
  under Ruby's hierarchy: BasicObject above Object, and Numeric above
  Integer and Float, so `Integer.ancestors` is `[Integer, Numeric,
  Comparable, Object, Kernel, BasicObject]` and `case x when Numeric`
  matches both;
  `module` as a namespace and a mixin through `include` and `extend`;
  `Comparable`, and `Enumerable` for a class that defines `each`;
  bang methods a class defines, `def save!`, which return their result
  and rebind nothing;
  `method_missing` and `respond_to_missing?`; `send`, `public_send` and
  `__send__`, which refuse a mutating method as a call
  does; `Class.new(Base) { … }`;
  `def self.x` in a class body.
- **Exceptions**: `begin`/`rescue`/`else`/`ensure`, `rescue => e`,
  `raise`, a bare `raise` in a rescue clause, `retry`, `cause` and `raise
  ..., cause:`, an exception class's own `initialize` with `super(msg)`,
  `exit` and `abort`, which raise SystemExit and end the command with its
  status, and the exception classes of Ruby's core.
- **Core classes**: BasicObject, Numeric, Integer, Float, String, Symbol, Array, Hash, Set (as in
  3.4, without `require`), Range, Regexp and MatchData, Time, Math,
  Enumerator and `lazy`, Comparable and Enumerable, with most of their
  methods. `format` and `String#%`, `pack` and `unpack`, `Random`,
  `rand` and `srand` on Ruby's Mersenne Twister, so a seeded script draws
  Ruby's numbers, `Integer()`, `Float()`, `String()` and `Array()`,
  `Process.clock_gettime`.
- **Libraries**: `require` of `set`, `time`, `pp`, `pathname`, `cgi`,
  `json`, `benchmark`, `uri`, `date`, `net/http`, `digest`, `base64` and
  `securerandom`, `yaml`, `csv`, `marshal`, `shellwords`, `ipaddr`, `zlib`,
  `open-uri`, `prime`, `tsort`, `abbrev` and `singleton`, which cameo builds
  in; [stdlib.md](stdlib.md)
  has every library of Ruby's standard library with its status, the ones
  to come and the ones left out. `cgi` has what Ruby's `cgi/escape` and `cgi/util`
  do without I/O: `escape` and `unescape`, the URI-component pair,
  `escapeHTML` and `unescapeHTML`, `escapeElement` and `unescapeElement`,
  `rfc1123_date` and `pretty`, with their snake_case names. A Symbol
  passes where these take a String; Ruby raises a TypeError. `json` is
  Ruby 3.4's json 2.9.1: `JSON.parse` (with `symbolize_names:`,
  `allow_nan:` and `max_nesting:`), `parse!`, `load`, `generate`, `dump`,
  `pretty_generate` and `JSON()`, with the generator's options, `to_json`
  on every value, and `JSON::ParserError`, `NestingError` and
  `GeneratorError`. Output, parsed values and error messages are Ruby's.
  `benchmark` is Ruby 3.4's benchmark 0.4.0: `Benchmark.measure`,
  `realtime`, `bm`, `bmbm` and `benchmark`, and `Benchmark::Tms` with its
  readers, `format`, `to_a`, `to_h`, arithmetic and `add`. The times are
  read inside the script, so the interpreter's start is not in them; the
  CPU times are the whole process's, the work of `async` calls included.
  `uri` is Ruby 3.4's uri 1.0.2 on Go's `net/url`, which reads a URI,
  resolves a reference and escapes a component: `URI.parse`,
  `URI()`, `join`, `split`, `for`, `extract`, `scheme_list`, the
  www-form and URI-component encoders, and `URI::Generic` with `HTTP`,
  `HTTPS`, `FTP`, `File`, `WS`, `WSS`, `LDAP`, `LDAPS` and `MailTo`:
  the components and their setters, `merge` and `+`, `route_from`,
  `route_to`, `normalize`, `select`, `build`, `==`, and the error
  classes, with Ruby's messages. A URI is an instance whose components
  are typed fields, so `u.with(host: "x")` is a copy; `u.host = "x"` is
  the same copy, checked as Ruby's setter checks it, bound to `u` where
  Ruby changes the URI in place (another variable holding the URI keeps
  it as it was). `merge!` and `normalize!` are bang methods, refused as
  every bang method of Ruby's core is.
  `date` is Ruby 3.4's date 3.4.1: `Date` and `DateTime` with Ruby's
  calendar (the Julian Day Number, the start of the Gregorian calendar,
  Dates are read through Go's time package: `Date.parse` tries a fixed
  list of forms, `strptime` goes through a Go layout, and what the layouts
  cannot say, a week date, an ordinal suffix, an era word or a directive
  such as `%V`, is not read.
  `Date::ITALY`, `ENGLAND`, `JULIAN` and `GREGORIAN`), `new`/`civil`,
  `jd`, `ordinal`, `commercial`, `today`, `now`, `valid_date?` and kin,
  the readers, `+`, `-`, `>>`, `<<`, `next_*` and `prev_*`, `step`,
  `upto`, `downto`, a Range of dates, `<=>`, `==`, `===`, `strftime`,
  `to_s`, `inspect`, `iso8601`, `rfc3339`, `xmlschema`, `rfc2822`,
  `httpdate`, `jisx0301`, `asctime`, `deconstruct_keys`, `Date.parse`
  and `_parse` with the forms Ruby reads, `strptime` and `_strptime`,
  the fixed formats and their underscored readers, `new_offset`,
  `to_date`, `to_datetime`, `to_time`, and `Time#to_date` and
  `to_datetime`. Where Ruby returns a Rational, cameo returns an Integer
  when the value is whole and a Float otherwise: `Date - Date` is `59`,
  not `(59/1)`; `ajd`, `day_fraction`, `sec_fraction` and `offset` are
  Floats. `Date::Infinity` and `marshal_dump` are left out, and a day
  number past 64 bits raises `RangeError`.
  `net/http` (with `net/https` and `net/protocol`) is Ruby 3.4's
  net-http 0.6.0 in what a script calls: `Net::HTTP.get`,
  `get_response`, `get_print`, `post`, `post_form`, `put`, `start` and
  `new` with `use_ssl`, `open_timeout`, `read_timeout`, `request`, the
  verbs (`get`, `head`, `post`, `put`, `patch`, `delete`, …),
  `send_request`; `Net::HTTP::Get` and the other request classes with
  `[]`, `[]=`, `body=`, `set_form_data`, `basic_auth`, `content_type`,
  `each_header`, `to_hash`; `Net::HTTPResponse` and its classes by status
  (`Net::HTTPOK`, `HTTPNotFound`, `HTTPSuccess`, …) with `code`,
  `message`, `body`, `[]`, `value`, `error!`, `content_type`,
  `content_length`, `uri`; `Net::HTTPError` and its hierarchy,
  `Net::ReadTimeout`, `SocketError`, `Errno::ECONNREFUSED`,
  `OpenSSL::SSL::SSLError`, with Ruby's messages. A program reaches the
  network only through the HTTP client its Go host gives it
  (`Program.HTTPClient`); the `cameo` command gives one, an embedding
  program decides. Without one, `require "net/http"` works and a
  request raises `SocketError`. Redirects are not followed, as in Ruby.
  A request blocks the goroutine it runs on; a script fans requests out
  with `async`. A connection, a request and a response are instances:
  `http.use_ssl = true` and `req["Accept"] = "x"` rebind the variable to
  the changed copy, and `req.set_form_data(h)` and `req.basic_auth(u, p)`
  return the copy for the script to bind, `req = req.basic_auth(u, p)`,
  where Ruby changes the request in place. `verify_mode=` is kept but
  not applied: how certificates are verified is the host's client's
  business. `Net::HTTP::Proxy`, `body_stream`, a request body streamed
  from an IO and a response body read in pieces are left out.
  `digest` is Ruby 3.4's digest 3.2.0: `Digest::MD5`, `SHA1`, `SHA256`,
  `SHA384`, `SHA512` and `SHA2` with `hexdigest`, `digest` and
  `base64digest` on the class and on an object, `update` and `<<`,
  `reset`, `digest_length`, `block_length`, `==`, `Digest.hexencode`,
  `Digest(:SHA256)`; and, under the names openssl gives them,
  `OpenSSL::HMAC.hexdigest`, `digest` and `base64digest`,
  `OpenSSL::Digest.new("SHA256")`, `OpenSSL::Digest.hexdigest` and
  `OpenSSL::Digest::SHA256`. A digest object is a value: `update` and
  `<<` return the object with more fed, for the variable to take,
  `d = d << "x"`, where Ruby feeds it in place. `RMD160` is left out
  (Go has no RIPEMD-160), and `file` reads a file, which cameo does not.
  `base64` is Ruby 3.4's base64 0.2.0: `encode64`, `decode64`,
  `strict_encode64`, `strict_decode64`, `urlsafe_encode64` with
  `padding:` and `urlsafe_decode64`, with Ruby's line breaks, its lenient
  decoder and its `ArgumentError`.
  `securerandom` is Ruby 3.4's securerandom 0.4.1: `SecureRandom.hex`,
  `uuid`, `uuid_v4`, `uuid_v7`, `random_bytes`, `random_number`,
  `alphanumeric` with `chars:`, `base64` and `urlsafe_base64`, drawing
  from the operating system through Go's `crypto/rand`; and
  `Random::Formatter`, which gives a `Random` the same methods, so a
  seeded `Random` draws the hex, the tokens and the numbers CRuby's
  draws.
  `yaml` is Ruby 3.4's psych 5.2.2 on the libyaml it is built on: `load`,
  `safe_load`, `unsafe_load` and `load_stream` with Psych's keywords,
  `dump`, `safe_dump`, `dump_stream` and `to_yaml` with `indentation:`,
  `line_width:` and `header:`, Psych's scalar rules, the merge key, and
  Psych's tags for a Range, a Regexp, a Set, an exception and an object,
  which is an instance of a cameo class by its fields. The parser and
  emitter are gopkg.in/yaml.v3's port of libyaml, so a document loads,
  dumps, quotes, folds and fails as it does in Ruby, with Ruby's error
  message, line and column. `load_file` and the other file methods are
  left out, as is `parse` with its node tree; a `!set` loads as a
  Hash; the module is `YAML` alone, with no `Psych` name, so the errors
  are `YAML::SyntaxError` and kin; a Proc, a Random or a MatchData dumps as the empty object Psych
  writes.
  `csv` is Ruby 3.4's csv 3.3.2: `CSV.parse`, `parse_line`, `generate`,
  `generate_line`, `String#parse_csv` and `Array#to_csv` with csv's
  options, headers and converters, `CSV::Row`, `CSV::Table`, and
  `CSV.new(string)` as a handle read with `shift` or `each`, written with
  `<<`. A Row and a Table are values: `row["c"] = 3` rebinds `row` to the
  copy, and `tb = tb << fields` is the form, where Ruby changes them in
  place; `delete` returns the copy without the part, not the part. The
  parser and the writer are Go's `encoding/csv`, so `quote_char:` is
  always `"`, `col_sep:` is one character, `row_sep:` is a line feed with
  or without a carriage return, `force_quotes:` is refused, an empty
  String is written bare and a field with a leading space is quoted. The
  file methods, `read`, `foreach`, `open` and `table`, are left out.
  `marshal` is `Marshal.dump`, `load` and `restore` in cameo's own wire
  form, not Ruby's: the bytes are the protobuf `cameo.Value` envelope a
  value crosses to a host in, so cameo reads them back and any protobuf
  reader with `cameo/value.proto` reads them too. An instance of a cameo
  class comes back by its class's name and digest. A Proc, a class, a
  Random, an Enumerator and an instance of a library's class such as a
  Date raise `TypeError`; a Symbol comes back as a String, as it crosses
  to a host. An IO to write to or read from is left out.
  `shellwords` is Ruby 3.4's shellwords 0.2.2: `Shellwords.split`,
  `escape` and `join`, `String#shellsplit` and `shellescape`,
  `Array#shelljoin`, with Ruby's quoting rules and its message for an
  unmatched quote.
  `String#unicode_normalize` and `unicode_normalized?` take the four
  forms, on Go's `golang.org/x/text`, and are there without a `require`,
  as Ruby's are.
  `ipaddr` is Ruby 3.4's ipaddr 1.2.7: `IPAddr.new` with a prefix or a
  mask, Ruby's compressed `to_s` and full `to_string`, `include?`,
  `mask`, `to_range`, `succ`, the predicates, the operators and the
  reverse names. An IPAddr is a value: `mask` returns the masked copy
  where Ruby's `mask!` changes the address in place.
  `zlib` is Ruby 3.4's zlib 3.2.1 as module functions: `crc32`,
  `adler32`, `Zlib::Deflate.deflate`, `Zlib::Inflate.inflate`, `gzip`
  and `gunzip`, with zlib's constants, errors and messages. The
  compressed bytes are Go's deflate's: they inflate in Ruby and Ruby's
  inflate here, but are not identical to zlib's byte for byte. The
  streaming objects and the gzip IO classes are left out.
  `open-uri` is Ruby 3.4's open-uri 0.5.0 over `net/http`: `URI.open`
  and `URI::HTTP#open` and `#read` with the header fields, redirect,
  basic authentication, timeout and encoding options. What `open`
  returns is an `OpenURI::Document` value with Ruby's `read`,
  `base_uri`, `status`, `meta`, `content_type`, `charset` and
  `last_modified`, where Ruby returns an IO with those methods mixed in.
  A file name, a `file:` URL and a proxy are left out.
  `prime` is Ruby 3.4's prime 0.1.3: `Prime.each`, `prime?`,
  `prime_division`, `int_from_prime_division` and the Enumerable methods
  on `Prime`, with `Integer#prime?` and `prime_division`. The test is
  Miller-Rabin and the factorisation Pollard's rho, so a large number
  answers where Ruby's trial division does not finish.
  `tsort` is Ruby 3.4's tsort 0.2.0: the module functions over two
  callables, `->(&b) { graph.each_key(&b) }` and
  `->(n, &b) { graph[n].each(&b) }`, and the methods of a class that
  includes `TSort`, component for component in Ruby's order.
  `abbrev` is Ruby 3.4's abbrev 0.1.2: `Abbrev.abbrev` and
  `Array#abbrev`, with a prefix or a Regexp as the pattern.
  `singleton` is Ruby 3.4's singleton 0.2.0: `include Singleton` gives
  the class `instance`, the one value, and takes `new` away.
- **Output**: `puts`, `print`, `p`, `pp`, `warn`.

## Different in cameo

**Values are immutable.** Nothing changes in place, and no call changes or
rebinds its receiver:

- A call of one of Ruby's mutators on a value of the kind it changes
  raises a `CameoError` that names the non-mutating form: `a << 1` and
  `a.push(1)` on an array ("use +, or append"), `s << "x"` and
  `s.setbyte(0, 65)` on a string, `h.store(k, v)`, `h.delete(k)` and
  `h.update(o)` on a hash, `st.add(x)` and `st.merge(o)` on a set. The
  test `src/eval/mutation_test.go` lists every one. With `cameo
  --ignore-mutation-errors` (or `Program.IgnoreMutationErrors` from Go)
  the call instead returns what Ruby's returns, computed on a copy, and
  the receiver keeps its value: `a.pop` is the last element, and `a` is
  as it was.
- A mutator whose value is assigned back to its receiver rebinds it:
  `a = a << 1`, `a <<= 1` and `self.xs = self.xs << x` in `initialize`
  return what Ruby's returns, computed on a copy, and the assignment binds
  it. Another variable that held the old value keeps it.
- A bang method of Ruby's core (`compact!`, `sort!`, `gsub!`, `merge!`)
  is refused before the script runs: "This is not supported, use x =
  x.compact instead". A bang method a class defines is an ordinary call.
- `force_encoding` returns the string relabelled and leaves the receiver,
  since nothing else reinterprets bytes. A call whose value the script
  drops, a statement on its own, raises, since it would do nothing.
- The same names on a value they do not change are ordinary calls:
  `Integer#<<` shifts, and a class's own `push` is its own method.
- Assignment makes the copy and rebinds the local: `a += [1]`, `a.x = 1`,
  `a.x.y = 1` and `h[k] = v` rebind `a` or `h` to a copy with the change.
  The chain must start at a local variable, or at `self` inside
  `initialize`.
- `frozen?` is true for everything; `freeze` and `dup` change nothing.
- Equality is structural: two arrays, hashes or instances with equal
  contents are `==` and hash alike. There is no object identity, so
  `equal?`, `object_id` and `__id__` do not exist, and `compare_by_identity` changes
  nothing.

**A Random is a handle on a generator**, as a file handle is on a file:
`r.rand`, `r.bytes` and `r.random_number` advance the generator `r` names,
so each call returns the next number, and every holder of `r` (another
variable, an array, a hash, a block, a method's argument) draws from the
same generator. It is the one value whose calls change something. A copy
would draw the numbers its original draws next, so a Random is not copied:
`dup` and `clone` raise a `TypeError` ("a Random cannot be copied"), and so
do passing one to `async`, returning one from an `async` call and storing
one in a typed field, as for a Proc. `r == other` compares seed and
position, as in Ruby, and `p r` prints `#<Random>`, without an address.

**Classes declare typed fields**, and every instance is a protobuf message:

```ruby
class Point
  int32 :x, :y
  string :label, default: ""

  def initialize(x, y)
    self.x = x
    self.y = y
  end
end

p = Point.new(1, 2)
q = p.with(x: 3)            # a copy; p is unchanged
```

- A field's type is one of proto3's scalar types (`int32`, `uint64`,
  `double`, `bool`, `string`, `bytes` and the rest), `int` and `float` for
  Ruby's Integer and Float, `bignum` for an Integer of any size, or `list`,
  `set`, `map`, `ref` and `enum`. A value outside its type raises a
  `TypeError` or a `RangeError`.
- `initialize` sets a field with `self.x = v`; a method reads it with `x`
  or `self.x`. There are no instance variables and no setters: `with` makes
  the changed copy.
- A class without `initialize` takes its fields as keywords:
  `Point.new(x: 1, y: 2)`.
- An exception class is a class: it declares fields and sets them in
  `initialize`, where Ruby sets instance variables. Without an
  `initialize` of its own it takes the message as its one positional
  argument and its fields as keywords, `HTTPError.new("down", status:
  503)`, and `e.with(status: 502, message: "bad gateway")` makes the copy.
  An exception raised in a rescue clause gets its cause on a copy: a
  local that holds the exception keeps it without the cause, where Ruby
  sets it on the object.
- A class is a value bound to its name. Evaluating the same `class X … end`
  again gives the same class; a different body reopens a copy of the
  class and rebinds `X` to it. Instances made before, and classes that
  inherit from it, keep the class as it was and do not see what the
  reopening adds, as they do in Ruby. A builtin class cannot be reopened.
- Numeric and BasicObject cannot be subclassed. A Numeric of a script's
  own would need Integer's and Float's arithmetic to `coerce` it, which it
  does not; a BasicObject subclass is a blank-slate object without
  Object's methods, which cameo does not make. `method_missing` in an
  ordinary class does what a BasicObject proxy does in Ruby.

`Data` is left out: a class with typed fields is cameo's Data, and `with`
is Data's `with`. Ruby's

```ruby
Point = Data.define(:x, :y)
a = Point.new(x: 1, y: 2)
b = a.with(y: 3)
```

is, in cameo,

```ruby
class Point
  int :x, :y
end

a = Point.new(x: 1, y: 2)
b = a.with(y: 3)
```

`Point.new(1, 2)` with positional arguments needs an `initialize` that
sets the fields, and `p a` prints `Point(x: 1, y: 2)` where Ruby prints
`#<data Point x=1, y=2>`.

**Smaller differences:**

- Hash and array literals, and errors, print as CRuby 3.4 prints them:
  `{x: 1}`, not 3.3's `{:x=>1}`.
- A bare `Object` prints as `#<Object>`, without an address, and a Proc
  as `#<Proc:script.rb:3 (lambda)>`, where it was written.
- `__LINE__` is the line its statement starts on, which differs from
  Ruby's inside a statement that spans lines.
- A `throw` ends the `catch` whose tag is equal to its own, where Ruby
  wants the same object: `catch("a") { throw "a" }` returns nil. The
  Object a `catch` without a tag makes is still a tag of its own.
- `Time` takes an IANA zone name, `"Europe/Berlin"`, wherever Ruby
  takes a UTC offset; see *Only in cameo*.
- `Time` is immutable: `utc` and `localtime` return a new time, and
  `subsec` is a Float, since `Rational` is left out.
- A String is UTF-8, ASCII-8BIT (binary) or US-ASCII; other encodings do not
  exist.
- Regular expressions use Ruby's syntax on cameo's own engine; lookbehind
  and backreferences are rejected with "This regexp is not supported in
  Cameo".
- `JSON.generate` writes an instance of a cameo class as an object of its
  fields, by name in declaration order, where Ruby writes the string of
  `to_s`. A class that defines `to_json` writes itself, as in Ruby, and
  receives the generator's options as a Hash with `depth:` instead of a
  `JSON::State`; passing it on to `to_json` nests the same way.
- `Benchmark::Tms#add!` does not exist: it changes its receiver. Use
  `t = t.add { ... }`. `p` of a `Benchmark::Tms` shows no address.
- An iterating method called without a block returns an Enumerator, as
  Ruby's does: `map.with_index(1)`, `each_slice(2).to_a` and `3.times.size`
  work as in Ruby. An Enumerator has no `next`, `peek` or `rewind`, which
  advance it in place. `with_object` passes the block's value on as the
  next memo, as `each_with_object` does. `chunk` with a block,
  `chunk_while`, `slice_when`, `slice_before` and `slice_after` return an
  Array of the slices, not an Enumerator.
- A Symbol becomes a String where a value leaves cameo, in a protobuf
  message or an `async` call, unless an `enum` field declares it.
- Ruby's version constants describe cameo. A script tells cameo from CRuby
  by `RUBY_ENGINE`, and cannot assign any of them at the top level:

  | Constant | In cameo |
  |---|---|
  | `RUBY_ENGINE` | `"cameo"` |
  | `RUBY_ENGINE_VERSION` | cameo's version, as `cameo version` prints it |
  | `RUBY_VERSION` | `"3.4.0"`: the Ruby cameo follows, 3.4 as released |
  | `RUBY_PATCHLEVEL` | `0`, Ruby 3.4.0's; cameo takes no CRuby patch release |
  | `RUBY_PLATFORM` | Ruby's name for the platform, as `x86_64-linux`, `aarch64-linux`, `arm64-darwin` or `x64-mingw-ucrt`, without the OS version CRuby appends on macOS |
  | `RUBY_RELEASE_DATE` | the date of the commit cameo was built from, or `"unknown"` |
  | `RUBY_REVISION` | the hash of that commit, or `"unknown"` |
  | `RUBY_DESCRIPTION` | the build in CRuby's form: `cameo 0.2.3 (2026-10-20 revision 0123456789) +ruby 3.4.0 [x86_64-linux]` |
  | `RUBY_COPYRIGHT` | `"cameo - Copyright (C) 2026 the cameo authors (MIT License)"` |

  The commit is known when the binary was built with `go build` in a git
  checkout; a binary from `go install ...@v0.2.3` knows its version but no
  commit.

## Rejected before the script runs

The checker rejects these by name, with the line:

| Construct | Instead |
|---|---|
| instance variables, `@x` | `self.x = v` in `initialize`; read `x` or `self.x` |
| class variables, `@@x`; global variables, `$x`, including `$!` and `$~` | a constant, a field or a local; `rescue => e`; the MatchData of `match`. `$stdout` and `$stderr` exist: they are `STDOUT` and `STDERR`, with `puts`, `print`, `write`, `flush` and `sync` |
| setters, `def x=(v)`; `attr_reader`, `attr_writer`, `attr_accessor` | field declarations, which define readers, and `with` |
| `private`, `protected`, `public`, `module_function` | none: every method is public |
| `eval`, `instance_eval`, `class_eval`, `define_method`, `instance_variable_get` and `_set`, `undef` | a direct call or a `def` |
| `def obj.x`, `class << self` | `def self.x` in a class body |
| a subclass of Numeric or BasicObject, `class X < Numeric` | none for Numeric; `method_missing` in an ordinary class for BasicObject. Inside a module, and with `Class.new(Numeric)`, the refusal is a `CameoError` when the definition runs |
| a bang method of Ruby's core, such as `a.sort!` | the copying form, assigned: `a = a.sort` |
| nested multiple assignment, `a, (b, c) = …` | separate assignments |
| `require_relative`, `load` | one file per script |
| backticks, `fork` | none; `fork`'s message names `async` |
| rational and imaginary literals, `1r` and `2i` | none |
| assignment to a scoped constant, `A::B = 1` | assignment inside `A` |

## Missing builtins

These are absent, and a script that uses one fails when it reaches it:

- **Left out by design**: `Struct` and `Data` (classes declare typed
  fields; see above for Data), `Rational` and `Complex`, `Marshal` (instances are protobuf
  messages), `ObjectSpace`, `GC`, `ENV`, `Module.new`,
  refinements, `binding`, `method` and `Proc#curry`.
- **No I/O beyond standard output and error, and the network through
  `net/http`**: `File`, `IO`, `Dir`, `gets`, `STDIN`, `Socket`.
- **No threads or processes**: `Thread`, `Fiber`, `Mutex`, `sleep`,
  `system`, `exec`, `spawn`; cameo has `async` and `await` instead.
- **Not there yet**: `at_exit`, `exit!`,
  `printf` (`format` exists), `__method__`, `Enumerator.new`,
  `Integer()` with a base, and `autoload`.
- **Libraries**: `require` of any library but the twenty-four above, such as
  `matrix`, raises a `LoadError`. [stdlib.md](stdlib.md) says which are
  planned and which are left out, and why.

## Only in cameo

- Typed fields, with every class a protobuf message: `to_proto`,
  `from_proto`, and `Point.proto` for its `.proto` text. Every exception is
  a message too, of its message, line, cause and fields, and crosses to
  `await` and to a Go host with them.
- `with`, which returns a copy with some fields changed.
- Time zones by name. An IANA name such as `"Europe/Berlin"` or
  `"America/New_York"` is accepted wherever Ruby takes a UTC offset:
  `Time.new(…, in:)` and its seventh argument, `Time.at(…, in:)`,
  `Time.now(in:)`, `localtime`, `getlocal`, `DateTime.new`'s offset and
  `new_offset`. The time then carries the zone: `zone` is the
  abbreviation in force, `"CEST"`, `utc_offset` and `dst?` follow the
  date, and adding an hour across a change of clocks lands where the
  clocks did. Ruby takes a zone object there and has no zone database;
  cameo compiles Go's in (`time/tzdata`), so a name resolves on any
  machine. A name it does not know is an `ArgumentError`; an
  abbreviation alone, `"CET"`, is refused as Ruby refuses it.
- The Cameo module, which a script includes: `async receiver, :method,
  args…` runs a method concurrently and returns its promise, an
  Integer; `await` collects a promise's result, or a list's, and `await
  :all` every one the scope started; `promise`, `resolve` and `fail` make
  and settle a promise by hand, and a Go host settles the promises its
  methods return when their I/O is done. `Cameo::Runner` is the same
  module under the name that Ruby has too: `ruby/lib/cameo/runner.rb`
  gives CRuby `async`, `await` and promises on threads, so a script that
  starts with `require "cameo/runner"` and whose classes include
  `Cameo::Runner` runs on both. The top level refuses `Cameo::Runner` on
  both; `include Cameo` at the top level is cameo's alone.
- The Go host API: a Go program loads a script, calls into it, and passes
  protobuf messages both ways. A host can give the script a module of its
  own, whose methods are Go functions, and native classes: classes whose
  fields are a generated protobuf message and whose methods are Go
  functions (`cameo.Register`).
