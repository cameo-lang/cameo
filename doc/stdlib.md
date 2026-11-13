# Ruby's standard library in cameo

Which of Ruby 3.4's libraries cameo builds in, which it will, and which it
leaves out and why. This page is the plan and the record of its progress:
each library has a status, and the pull request that moves it updates the
status here.

| Status | Means |
|---|---|
| **available** | built in; `require` returns true, and the library follows the Ruby 3.4.1 version named |
| **in progress** | a pull request is open |
| **planned** | decided in, in the order of the table |
| **unsupported** | left out, for the reason given; a `require` raises `LoadError` |

Ruby 3.4.1 ships 58 default gems and 29 bundled gems. Every one is in a
table below. "Ruby" below is CRuby 3.4.1; the version named for a library
is the one it ships.

## What decides

A library goes in when its work is computation on values: parsing,
formatting, arithmetic, encoding. It stays out when what it does is one of
the things cameo leaves out on purpose
([cameo-vs-ruby.md](cameo-vs-ruby.md)): the file system and processes,
threads, metaprogramming, object identity, mutable state inside a value.

A library whose purpose is a cursor, a buffer or a sink (`StringIO`,
`StringScanner`, `Logger`) is a **handle**, as `Random` is: a value whose
calls advance the thing it names, which every holder of the handle sees,
and which cannot be copied, passed to `async` or stored in a field. That
is cameo's one kind of mutable value, and these take it rather than
being left out: a script that tokenises a string or collects output needs
them. The decision is recorded here so it is not re-opened by accident;
each handle's pull request states what its calls change.

Network I/O is the one exception to "no I/O". `net/http` is in, because a
script that fetches data is what cameo is for; it reaches the network only
through a client the Go host gives the program, so an embedding program
decides whether a script may make requests at all (see *net/http* below).

Files are the second: `File.read`, `File.open` and `File.write` are the methods
of `Cameo::IO::FileIO`, in `Cameo::IO`, the module cameo's I/O is to live
in, and `File` extends it. They need two things from the program, both:
file I/O allowed, and a file system, an `afero.Fs`
(`github.com/spf13/afero`) given with `Program.FileSystem(fsys, dir)`, which
may be the disk, a directory of it (`afero.NewBasePathFs`), a read-only
view (`afero.NewReadOnlyFs`) or memory; without either, `File` has no
`read`, `open` or `write`, and `respond_to?`, `methods` and Tab in irb say so. `cameo --io`
and `--io=file` give the machine's files, from the working directory,
and so does `cameo --io=file irb`. `File.write` writes a whole file;
`File.open` only reads. A path is only a path: `"|cmd"` runs nothing. See
[cameo-vs-ruby.md](cameo-vs-ruby.md) for what differs from Ruby.

What asks for a library is measured, not guessed:

- **ruby/spec** ([specs/](../specs)): of the 375 spec files that
  `require` a library cameo lacks, 131 want `net/http`, 52 `win32ole`, 22
  `etc`, 20 `syslog`, 13 each `coverage`, `erb`, `io/nonblock`, `ostruct`
  and `tempfile`, 11 `open3`, 8 `yaml`, and fewer each for the rest. It
  was 1006 files before `date`, `uri`, `matrix`, `stringio`, `bigdecimal`,
  `strscan`, `csv`, `zlib` and the other libraries of this document were
  built in.
- **CRuby's benchmarks** ([benchmarks/ruby](../benchmarks/ruby)): 22 cases
  stop on a `require` cameo refuses.
- What the scripts cameo runs ask for: a cameo script runs in SpicyLab's
  `code-ruby` tool, where a model writes it, and a model reaches for
  `json`, `uri`, `net/http`, `date` and `time` first.

## How a library is built in

- **One Go package per library**, under `src/lib/<require path>`:
  `src/lib/json` for `json`, `src/lib/net/http` for `net/http`, with
  the package named after the last element. `src/lib` itself registers
  them: `lib.Libraries` names what `require` accepts, with the libraries
  each loads along with it (`cgi` loads `cgi/escape` and `cgi/util`), and
  `lib.Install` installs them all when the interpreter starts. The
  exceptions are the core's own: `set` and `pp`, which `src/stdlib`
  installs and `lib.Libraries` registers; the reading of a time by a
  string, which `Time.new` needs and so stays in `src/stdlib`; and
  `debug` and `cameo/runner`, the evaluator's, which it accepts itself.
- **Installed at start, not at `require`.** `JSON` exists before `require
  "json"`, which Ruby's does not. `require` returns true the first time and
  false after, as Ruby's does.
- **Every class and module the Go binary binds at the top level is
  `Cameo::Native::Name` too**: once the core and the libraries installed,
  the evaluator mirrors every top-level class there (`object.MirrorNative`),
  and a class a Go host registers goes there as it is bound. The class
  keeps its name, so a script sees no difference. A library written in
  cameo defines its classes after that and is not there; it may take a
  top-level name and keep the Go part of it reachable, bound under
  `Cameo::Native` alone with `object.BindUnderNative`.
- **A library may be written in cameo.** Its Go package embeds the `.cameo`
  source and registers it in `lib.Sources` under its require path; `require`
  parses it and runs it at the top level of the script the first time, so
  its classes and methods exist from the `require` on, not from the start.
  A library may be part Go and part cameo: its source may open a module
  its Go package defined, `module M` or `class M::C`, which
  adds to the requiring program's copy of the module and leaves the one
  every program shares as it was; a script may not, nor a library open a
  class Go defined.
  A source that does not parse is a `SyntaxError`, as in Ruby; one that
  raises is a `LoadError` naming the library's file and line, and is not
  loaded, so a later `require` runs it again. A worker `async` started
  cannot load one, since its top level is a snapshot every worker shares:
  the script requires it first, and the workers see it loaded.
- **Ruby's behaviour, measured against Ruby.** A library follows the
  version Ruby 3.4.1 ships, to its output and the messages of its errors.
  `JSON::VERSION` and `Benchmark::VERSION` carry that version; `set`,
  `cgi`, `pathname` and `pp` have no `VERSION` yet. A library that lands
  from here on comes with an example under [examples/](../examples) whose
  expected output Ruby 3.4 generated ([benchmarks/warmruby](../benchmarks/warmruby)
  runs CRuby 3.4.1 where the `ruby` on PATH is older), Go tests in
  [tests/](../tests), and an integration test against the binary where
  it meets the command line or the outside, as `ARGV` and `File.read` do;
  the examples' directory and `tests/integration/test_cameo.sh` are the
  record of which library has which. Where cameo differs, its row says so.
- **Shared helpers come from `src/stdlib`**, which exports what a library
  needs of the core: argument checks, the `Strftime` of Time, the
  formatting of floats. A library imports `stdlib` and `object`; `stdlib`
  imports no library.
- **Pure Go.** No cgo, and a dependency only where Go's standard library
  has nothing: `golang.org/x/text` for `unicode_normalize`.

## Available

| Library | Ruby's version | What is in |
|---|---|---|
| `set` | set 1.1.1 | `Set`, as in Ruby 3.4 without a `require`; the mutators raise as every mutator does |
| `time` | time 0.4.1 | `Time.parse`, `strptime`, `iso8601`/`xmlschema`, `Time#iso8601`; `Time.new` reads a string; `rfc2822`, `httpdate` and their writers are **planned**. The reading is Go's time package: `Time.parse` tries the forms of a fixed list (ISO 8601, RFC 2822, HTTP dates, dates with month names, slashed and dashed dates, times of day), `strptime`'s format becomes a Go layout, and a zone is read with Go's zone layouts; a directive Go has no layout for (`%j` with a year, `%U`, `%V`, `%G`, `%C`, a `%s` beside other directives) is an ArgumentError, and text around a date, an ordinal suffix or an era word is not read. A zone name is one Ruby's Time knows (UTC, GMT, EST, PST and kin, the military letters) or the local zone. A day past the month's end, `2024-02-30`, is refused where Ruby rolls it over to the next month |
| `pp` | pp 0.6.2 | `pp` prints as `p` does: `inspect`, one value per line; no `pretty_inspect` and no line breaking at 80 columns |
| `pathname` | pathname 0.4.0 | path arithmetic without I/O: `join`, `+`, `basename`, `dirname`, `extname`, `cleanpath`, `parent`, `sub_ext`, `relative_path_from`, `each_filename`, `<=>`; `File.join`, `basename`, `dirname`, `extname`, `expand_path`; everything that reads the file system is **unsupported** |
| `cgi` (`cgi/escape`, `cgi/util`) | cgi 0.4.1 | the escapes, `escapeElement`, `rfc1123_date`, `pretty`; `CGI.new`, cookies and sessions read a request and are **unsupported** |
| `json` | json 2.9.1 | `parse`, `parse!`, `load`, `generate`, `pretty_generate`, `dump`, `JSON()`, `to_json`, the error classes; `json/add/*` is **unsupported** (it is Marshal in JSON) |
| `benchmark` | benchmark 0.4.0 | `measure`, `realtime`, `bm`, `bmbm`, `benchmark`, `Tms`; `Tms#add!` is a mutator |
| `uri` | uri 1.0.2 | `URI.parse`, `URI()`, `join`, `split`, `for`, `extract`, `scheme_list`, `URI::Generic` and `HTTP`, `HTTPS`, `FTP`, `File`, `WS`, `WSS`, `LDAP`, `LDAPS`, `MailTo` with their components, `merge`, `+`, `route_from`, `route_to`, `normalize`, `select`, `build`, `==`, `encode_www_form`, `decode_www_form`, the `_component` pair, `encode_uri_component`, `InvalidURIError`, `InvalidComponentError`, `BadURIError`. A URI has no setters: `u.host = x` and `u.with(host: x)` raise `NoMethodError`, since a URI is parsed, joined, merged or built, not changed; `merge!` and `normalize!` are bang methods; `URI.regexp` is obsolete in Ruby and raises. The parsing, the resolution of a reference and the escaping are Go's `net/url`, so `encode_www_form_component` keeps `~` and escapes `*`, the other way round from Ruby |
| `date` | date 3.4.1 | `Date` and `DateTime`: `new`/`civil`, `jd`, `ordinal`, `commercial`, `today`, `now`, `parse` and `_parse` with Ruby's forms, `strptime`, `_strptime`, `iso8601`, `rfc3339`, `xmlschema`, `rfc2822`, `httpdate`, `jisx0301` and their `_` readers, `valid_*?`, `leap?`, the readers, `+`, `-`, `>>`, `<<`, `next_*`, `prev_*`, `step`, `upto`, `downto`, Ranges, `<=>`, `===`, `strftime` and the writers, `new_offset`, `deconstruct_keys`, `to_date`, `to_datetime`, `to_time`, `Time#to_date`, `to_datetime`, `Date::ITALY` and the other starts, the name constants, `Date::Error`; a Rational of Ruby's is an Integer when whole and a Float otherwise; `Date::Infinity` and `marshal_dump` are left out |
| time zones | — | an IANA zone name wherever Ruby takes a UTC offset: `Time.new(…, in:)` and its seventh argument, `Time.at(…, in:)`, `Time.now(in:)`, `localtime`, `getlocal`, `DateTime.new`'s offset and `new_offset`; `Time#zone` is the abbreviation in force, `utc_offset` and `dst?` follow the date; Go's `time/tzdata` is compiled in. Ruby takes a zone object there, so this is cameo's extension; an abbreviation alone is refused as Ruby refuses it |
| `digest` (`digest/md5`, `sha1`, `sha2`) | digest 3.2.0 | `Digest::MD5`, `SHA1`, `SHA256`, `SHA384`, `SHA512`, `SHA2`: `hexdigest`, `digest`, `base64digest` on the class and on an object, `update`, `<<`, `reset`, `digest_length`, `block_length`, `==`, `Digest.hexencode`, `Digest()`; `require "openssl"` is accepted for the OpenSSL names; `OpenSSL::HMAC.hexdigest`, `digest`, `base64digest`, `OpenSSL::Digest.new`, `hexdigest`, `OpenSSL::Digest::SHA256` and kin, each taking the algorithm by name, class or digest object; `name` on every digest object, as OpenSSL's has. An object is a value: `update` returns the copy. `RMD160` (no RIPEMD-160 in Go) and `file` are left out |
| `base64` | base64 0.2.0 | `encode64`, `decode64`, `strict_encode64`, `strict_decode64`, `urlsafe_encode64` (`padding:`), `urlsafe_decode64`, with Ruby's line breaks, lenient decoder and errors |
| `securerandom` (`random/formatter`) | securerandom 0.4.1 | `SecureRandom.hex`, `uuid`, `uuid_v4`, `uuid_v7`, `random_bytes`, `bytes`, `gen_random`, `random_number`, `rand`, `alphanumeric` (`chars:`), `base64`, `urlsafe_base64`, drawing from Go's `crypto/rand`; `Random::Formatter` gives the same methods to a `Random` and to `Random` itself, and a seeded `Random` draws CRuby's values |
| `csv` | csv 3.3.2 | `CSV.parse` (with a block too), `parse_line`, `generate`, `generate_line`, `generate_lines`, `String#parse_csv`, `Array#to_csv`; the options `col_sep:`, `row_sep:` (`:auto`), `quote_char:` (`nil` too), `headers:` (true, an Array, a String), `return_headers:`, `write_headers:`, `converters:` and `header_converters:` (Ruby's by name, or a Proc of one or two arguments with `CSV::FieldInfo`), `skip_blanks:`, `skip_lines:`, `liberal_parsing:`, `strip:`, `nil_value:`, `empty_value:`, `write_nil_value:`, `write_empty_value:`, `force_quotes:`, `quote_empty:`; `CSV::Row` and `CSV::Table` with their readers, `by_col`, `by_row`, `each`, `to_a`, `to_csv`, `==`, `inspect`; `CSV.new(string)` as a handle with `shift`, `read`, `each`, `lineno`, `line`, `headers`, `eof?`, `rewind`, `<<`, `string`; `CSV::MalformedCSVError` with Ruby's messages and `line_number`. A Row and a Table are values: `[]=`, `<<`, `push`, `delete` and `delete_if` return the changed copy, and `delete` returns the Row or Table without what it removed, not the removed part. `CSV.read`, `foreach`, `open`, `table` and the other file methods, `by_col!` and `by_row!` (bang methods), `CSV::Converters` and `HeaderConverters` as Hashes of Procs are left out. The fields are read and written by Go's `encoding/csv`, its errors given Ruby's messages; the column separator is one character, as Go's reader takes |
| `marshal` | — (cameo's wire form) | `Marshal.dump`, `load`, `restore`, `load(…, freeze:)`, with `dump`'s depth limit accepted; the bytes are the protobuf `cameo.Value` envelope every value crosses to a host in, what `to_value_proto` gives, so cameo reads them back and any protobuf reader with `cameo/value.proto` reads them too. An instance of a cameo class is found again by its class's name and digest. A Proc, a class, a Random and an Enumerator raise `TypeError`, as a Proc does in Ruby; so does an instance of a library's class, a Date or a URI, which the envelope does not carry; a Symbol comes back as a String, as it crosses to a host. Ruby's format, an IO to write to or read from, the `proc` argument of `load`, `MAJOR_VERSION` and `MINOR_VERSION` are left out |
| `shellwords` | shellwords 0.2.2 | `Shellwords.split` (`shellsplit`, `shellwords`), `escape` (`shellescape`), `join` (`shelljoin`), `String#shellsplit`, `shellescape`, `Array#shelljoin`, with Ruby's quoting rules, its `ArgumentError` for an unmatched quote or a NUL, and its `NoMethodError` for what is not a String; written in cameo. The String and Array methods exist before the `require`, so `respond_to?` says so, and they say to require the library when it is not loaded; a class that includes the module gets the methods public, since every method in cameo is |
| `debug` | debug 1.10.0 | `debugger` and `binding.break` stop in cameo's debugger, which takes rdbg's commands ([debugger.md](debugger.md)) |
| `cameo/runner` | — | cameo's own: the library that gives CRuby `Cameo::Runner` is built in |
| `minispec` | — (cameo's own) | `describe`, `it`, `specify` and `skip`, minitest 5.25's `assert`, `assert_equal`, `assert_nil`, `assert_empty`, `assert_includes`, `assert_instance_of`, `assert_kind_of`, `assert_match`, `assert_respond_to`, `assert_predicate`, `assert_operator`, `assert_in_delta`, `assert_raises`, their `refute_*` negations, `flunk` and `pass`, with minitest's messages, and `Minispec::Assertion` and `Minispec::Skip`. An example runs when `it` is called and prints its mark; the report at the end of the script, through `at_exit`, lists every failure, error and skip with its line and counts them, and a failure or an error ends the script with status 1. No `before`, `after` or `let`: a local the describe block assigns is what its examples share. Ruby's minitest finds tests by method name and keeps state in instance variables, so it is left out (below) The functions are the methods of `Minispec::DSL` (`describe`, `it`, `specify`) and `Minispec::Assertions` (the assertions, `skip`, `flunk`, `pass`), which the `require` includes at the top level of the script alone, so they are no methods of every object and `File.methods` does not list them; a class that calls them in its methods, as one an async call runs examples in, includes the modules, as a Ruby class includes `Minitest::Assertions`. Called before the `require`, they say to require the library |
| `unicode_normalize` | — (core) | `String#unicode_normalize` and `unicode_normalized?` with `:nfc`, `:nfd`, `:nfkc`, `:nfkd`, on `golang.org/x/text/unicode/norm`, there without a `require` as in Ruby, and `require "unicode_normalize"` is accepted where Ruby knows only `unicode_normalize/normalize`; a binary String is `Encoding::CompatibilityError`, a UTF-8 String with bytes that are not UTF-8 is `ArgumentError`, and a US-ASCII String is as it is; `unicode_normalize!` is a bang method |
| `ipaddr` | ipaddr 1.2.7 | `IPAddr.new` from a string with a prefix length or a mask, or an Integer with its family, `to_s` compressed as Ruby compresses, `to_string`, `inspect`, `to_i`, `prefix`, `netmask`, `family`, `include?`, `===`, `==`, `eql?`, `<=>` and Comparable, `mask`, `to_range`, `succ` (so a Range of addresses iterates), `private?`, `loopback?`, `link_local?`, `ipv4?`, `ipv6?`, `ipv4_mapped?`, `ipv4_compat?`, `ipv4_mapped`, `ipv4_compat`, `native`, `\|`, `&`, `<<`, `>>`, `~`, `reverse`, `ip6_arpa`, `ip6_int`, `zone_id`, `hton`, `IPAddr.ntop`, the errors `IPAddr::Error`, `InvalidAddressError`, `InvalidPrefixError`, `AddressFamilyError`; `Socket::AF_INET`, `AF_INET6` and `AF_UNSPEC` are defined for the family argument. An IPAddr is a value with four fields, so it is a Hash key and a Set member; `mask!` and `set` are mutators, `mask` returns the copy; an error message names the address where ipaddr 1.2.6's left it out |
| `zlib` | zlib 3.2.1 | `Zlib.crc32`, `adler32`, `crc_table`, `Zlib::Deflate.deflate` and `Zlib.deflate` with a level, `Zlib::Inflate.inflate` and `Zlib.inflate`, `Zlib.gzip` (`level:`; `strategy:` is accepted and ignored, Go's deflate having none) and `gunzip`, the constants, `Zlib::Error` and its kinds, `Zlib::NeedDict` for a stream with a dictionary, `Zlib::GzipFile::Error`, `CRCError`, `LengthError`, `NoFooter`, with zlib's messages, but for a corrupt deflate body, whose `DataError` says where Go's reader stopped. On Go's `compress/zlib`, `compress/gzip`, `compress/flate` and `hash`: what deflates here inflates in Ruby and the other way round, the headers, checksums and errors are zlib's, but the compressed bytes are Go's deflate's, not byte for byte zlib's. `Zlib::Deflate.new` and `Inflate.new` raise `NotImplementedError`: the streaming methods, `GzipReader` and `GzipWriter` on an IO are left out |
| `prime` | prime 0.1.3 | `Prime.each` with a bound or endless, with a block or as an Enumerator, `prime?`, `include?`, `prime_division`, `int_from_prime_division`, `instance`, and the Enumerable methods on `Prime` itself (`first`, `take`, `lazy`, `select`…); `Integer#prime?`, `prime_division`, `Integer.from_prime_division`, `each_prime`. `prime?` is a Miller-Rabin test and `prime_division` divides out small factors then runs Pollard's rho, so a large number answers where Ruby's trial division does not finish; `Prime.each` returns an Enumerator, not `Prime::EratosthenesGenerator`. `prime_division` wants an Integer, where Ruby's divides a Float too; `int_from_prime_division` refuses a negative exponent, whose result is a Rational; a blockless `each_slice` or `each_cons` on the endless `each` runs until stopped, since the core's collect before they return ([cameo-lang/cameo#277](https://github.com/cameo-lang/cameo/issues/277)) |
| `tsort` | tsort 0.2.0 | `TSort.tsort`, `tsort_each`, `strongly_connected_components`, `each_strongly_connected_component`, `each_strongly_connected_component_from` over two callables, with a block or returning the Array, and the same as the methods of a class that includes `TSort` and defines `tsort_each_node` and `tsort_each_child`; `TSort::Cyclic` with Ruby's message, `tsort_each` yielding every node ahead of the cycle first; `tsort_each_node` and `tsort_each_child` raise `NotImplementedError` until the class defines them. The `id_map` and `stack` arguments of `each_strongly_connected_component_from` are refused when given, since a search is a value and cannot be carried between calls; written in cameo. Without a block, `tsort_each` and the `each_*` functions return the Array of what they would have yielded, not an Enumerator, and a `break` with a value out of their block returns nil, not the value ([cameo-lang/cameo#280](https://github.com/cameo-lang/cameo/issues/280)) |
| `abbrev` | abbrev 0.1.2 | `Abbrev.abbrev(words, pattern = nil)` and `Array#abbrev`, with a String prefix or a Regexp as the pattern, and Ruby's `NoMethodError` for any other pattern when it is first used; written in cameo. `Array#abbrev` exists before the `require`, so `respond_to?` says so, and says to require the library until it is loaded; a class that includes the module gets `abbrev` public, since every method in cameo is |
| `singleton` | singleton 0.3.0 | `include Singleton`: `instance` is the one instance, made by `initialize` the first time, once however the first calls race, and kept on the class, which a reopening shares: `instance` after the reopening is the same instance as a value of the class as it now is, with the new methods, though not `==` to the one from before, as no value made before a reopening is; a subclass has one of its own. `new` and `allocate` are refused with Ruby's NoMethodError, though `respond_to?(:new)` says true, since cameo has no private methods; `dup` and `clone` with its TypeError; a module including it and a class extending it are refused as in Ruby. The module uses two new hooks, `Class.Included` and `Extended`, which the evaluator runs when a class includes or extends a module |
| `forwardable`, `delegate` | forwardable 1.3.3, delegate 0.4.0 | `extend Forwardable` with `def_delegator`, `def_delegators`, `def_instance_delegator(s)`, `delegate … => :accessor` and `instance_delegate`, the accessor a method or a field (`:@items` names the field), and `SingleForwardable` with `def_single_delegator(s)` and `single_delegate` on a class or module; `SimpleDelegator`, `Delegator` and `DelegateClass(klass)` forwarding what they lack to the wrapped object through `method_missing`, with `__getobj__`, `respond_to?` (asking the wrapped object's own, so a target that answers through `method_missing` is forwarded to), `==`, `to_s`, `inspect` and the other Object methods forwarded, the rest reaching the object through `method_missing`. A delegator is a value: `__setobj__` returns the copy with the new object, `freeze` returns the delegator, and the wrapped object counts in its identity as a key. An accessor given as an expression String is refused, since cameo evaluates no strings. The evaluator now dispatches to a library's native `method_missing` too |
| `logger` | logger 1.6.4 | `Logger.new(STDOUT`, `STDERR` or `nil, level:, progname:, formatter:, datetime_format:)`, the levels as Integers, Symbols or Strings, `debug` to `unknown` with a message, a progname and a block, `add` and `log`, `<<`, the predicates, `level=`, `progname=`, `formatter=`, `datetime_format=`, `close`; `Logger::Formatter` with Ruby's line, `Logger::SEV_LABEL`, `Logger::Severity`, `Logger::Error`. A Logger is a handle: the setters change it for every holder, as in Ruby. A file name, log rotation and `Logger::LogDevice` are left out, since cameo writes no files |
| `stringio` | stringio 3.1.2 | `StringIO.new(string, mode)` and `open`, `read`, `gets`, `readline`, `readlines`, `each_line`, `each_char`, `each_byte`, `getc`, `readchar`, `getbyte`, `readbyte`, `ungetc`, `ungetbyte`, `readpartial`, `pread` with Ruby's separators, limits and `chomp:`; `write`, `<<`, `print`, `puts`, `printf`, `truncate`, writing over the position and padding past the end; `string`, `string=`, `reopen`, `pos`, `pos=`, `seek` with `IO::SEEK_SET`, `SEEK_CUR` and `SEEK_END`, `rewind`, `eof?`, `size`, `lineno`, `close`, `close_read`, `close_write` and the predicates, the modes `r`, `w`, `a` and `+`, with Ruby's `IOError`, `EOFError` and `Errno::EINVAL`. A StringIO is a handle; `string` returns the String as it is then, where Ruby returns the buffer itself |
| `strscan` | strscan 3.1.2 | `StringScanner.new(string, fixed_anchor:)`, `scan`, `scan_until`, `skip`, `skip_until`, `check`, `check_until`, `match?`, `exist?`, `scan_full`, `search_full` with a Regexp or a String, the match record with `matched`, `matched?`, `matched_size`, `pre_match`, `post_match`, `[]` by index or name, `size`, `captures`, `values_at`, `named_captures`, `getch`, `get_byte`, `scan_byte`, `peek`, `peek_byte`, `unscan`, `reset`, `terminate`, `pos`, `pos=`, `charpos`, `eos?`, `rest`, `rest_size`, `string`, `string=`, `concat`, `<<`, `beginning_of_line?`, `inspect` in Ruby's form, `StringScanner::Error`, `Version`. A StringScanner is a handle. With `fixed_anchor: true` the match at the position is found by searching the whole string, so `\A` holds at the start, as in Ruby |
| `optparse`, `getoptlong`, `ARGV` | optparse 0.6.0, getoptlong 0.2.1 | `ARGV` holds the script's arguments, `cameo script.cameo a -b c` (a host sets them with `Program.Args`); `OptionParser.new(banner, width, indent)` with a block, `on`, `on_tail`, `on_head`, `separator`, `banner`, `program_name`, `version`, `release`, `summary_width`, `summary_indent`, `help`, `to_s`, `to_a`, `summarize`, `ver`, `parse`, `parse!`, `order`, `order!`, `permute`, `permute!` on an Array or on `ARGV`, `getopts`; the switch forms (`-v`, `-nNAME`, `-n NAME`, `--name NAME`, `--name=NAME`, `[ARG]` optional, `--[no-]name`, `--no-name`), the types `Integer`, `Float`, `Numeric`, `String`, `Array`, `TrueClass`, an Array or Hash of permitted values completed by a unique prefix, long options completed by a unique prefix, the built-in `--help` and `--version` that print and raise `SystemExit`, the help text with Ruby's layout, `OptionParser::ParseError` with `args` and `reason` and its kinds `InvalidOption`, `MissingArgument`, `InvalidArgument`, `NeedlessArgument`, `AmbiguousOption`, `AmbiguousArgument`, with Ruby's messages; `GetoptLong.new` with Ruby's checks of the names and flags, `each`, `get`, `terminate`, `terminated?`, `ordering`, `quiet`, `error`, `error_message`, the constants, `GetoptLong::Error` and its kinds, an error printed to standard error as `file: message` unless quiet. An Array is a value, so `parse!` returns the operands and leaves its argument as it is; the options go through the blocks, or `getopts` returns them as a Hash; `into:` is refused with that advice. `GetoptLong` reads a copy of ARGV and gives the operands in `rest`, which Ruby lacks, since it leaves them in ARGV. A `Numeric` fraction is a Float, a `Symbol`, `Regexp` or other acceptor and `OptionParser.accept` are left out, as is `$0` (the help names the script file) |
| `matrix` | matrix 0.4.2 | `Matrix[]`, `rows`, `columns`, `build`, `identity`, `unit`, `I`, `zero`, `diagonal`, `scalar`, `row_vector`, `column_vector`, `empty`, `vstack`, `hstack`, `combine`; `[]`, `row`, `column`, `row_count`, `column_count`, `row_vectors`, `column_vectors`, `to_a`, `each`, `each_with_index`, `map`, `collect` and `find_index` with the `:diagonal`, `:lower`, `:upper` selectors, `+`, `-`, `*`, `/`, `**`, `-@`, `hadamard_product`, `determinant`, `inverse`, `transpose`, `rank`, `trace`, `minor`, `first_minor`, `cofactor`, `adjugate`, `laplace_expansion`, `round`, `rotate_entries`, the predicates (`square?`, `symmetric?`, `antisymmetric?`, `diagonal?`, `lower_triangular?`, `upper_triangular?`, `regular?`, `singular?`, `zero?`, `orthogonal?`, `permutation?`, `normal?`, `unitary?`, `hermitian?`, `real?`), `==`, `eql?`, `hash`, `to_s`, `inspect`, and Enumerable; `Vector[]`, `elements`, `zero`, `basis`, `independent?`, `[]`, `size`, `each`, `each2`, `map`, `map2`, `collect2`, `+`, `-`, `*`, `/`, `inner_product`, `dot`, `cross_product`, `norm`, `magnitude`, `normalize`, `angle_with`, `zero?`, `to_matrix`, `covector`, `round`; `ExceptionForMatrix::ErrDimensionMismatch`, `ErrNotRegular`, `ErrOperationNotDefined`, `ErrOperationNotImplemented`, `Vector::ZeroVectorError` with Ruby's messages. The determinant and the rank are Ruby's algorithms over the elements' own arithmetic, so Integer, Float and big Integer elements behave as in Ruby. A Matrix and a Vector are values: Hash keys and Set members, compared by their elements, `v[0] = x` returns the copy and `m[0, 0] = x` is refused. cameo has no Rational, so an Integer matrix's inverse, and what follows from it (`/` by a matrix, `**` with a negative exponent), has Floats computed exactly first, where Ruby has Rationals; `Matrix ** Float`, `eigensystem`, `lup`, `coerce` (so `2 * m` is a TypeError: write `m * 2`), Complex elements and `elements_to_*` are left out |
| `bigdecimal` (`bigdecimal/util`, `bigdecimal/math`) | bigdecimal 3.1.8 | `BigDecimal(value, prec, exception:)` from a String with Ruby's syntax, an Integer, a Float with a precision, or a BigDecimal, with Ruby's errors; `+`, `-`, `*` exact, `/` and `quo` to the digits bigdecimal computes with its rounding of the last digit, `add`, `sub`, `mult`, `div` with a precision, `div` to an Integer, `%`, `modulo`, `remainder`, `divmod`, `fdiv`, `**` and `power` with an Integer, a BigDecimal or a Float exponent, `-@`, `abs`, `sqrt`; `<=>`, `==`, `eql?`, `hash`, `<`, `>`, `coerce` and Comparable; `round`, `floor`, `ceil`, `truncate` with a place and a mode, `fix`, `frac`; `to_s` (`"F"`, `"+F"`, `"3F"`, `"E"`, an Integer group), `inspect`, `to_digits`, `to_f`, `to_i`, `to_d`, `_dump`, `_load`; `precision`, `scale`, `precision_scale`, `n_significant_digits`, `exponent`, `sign`, `split`, `zero?`, `nonzero?`, `finite?`, `infinite?`, `nan?`, `positive?`, `negative?`; `BigDecimal.limit`, `mode` (the rounding mode and the exception flags), `save_rounding_mode`, `save_exception_mode`, `save_limit`, `double_fig`, `interpret_loosely`, the constants, `INFINITY`, `NAN`; `String#to_d`, `Integer#to_d`, `Float#to_d(prec)`, `nil.to_d`; `BigMath.PI`, `E`, `sqrt`, `sin`, `cos`, `atan`, `log`, `exp`; `FloatDomainError` and `Math::DomainError` with Ruby's messages. A BigDecimal is a value with its sign, digits and exponent as fields: a Hash key and a Set member, equal to another with the same value. It is a Numeric, and Integer and Float now follow Ruby's `coerce` protocol, so `1 + big`, `2 * big`, `1 < big`, `[1, big].max` and `1.coerce(big)` work. The division is bigdecimal's: the digit count follows the operands' precision in nine-digit words, and the last digit is rounded from bigdecimal's own rough estimate of the next, so quotients agree with Ruby to the last digit. `sqrt(n)` and `BigMath`'s functions give the digits asked for, correctly rounded, where Ruby pads them with its own extra, partly wrong, digits; `limit` and `mode` are process-wide; `to_r` and `precs` are left out, since cameo has no Rational and `precs` is deprecated. A sum or difference across a gap of exponents carries every digit between, as bigdecimal does, and a gap of millions of digits takes seconds here, where the digits are binary; a comparison scales nothing |

## Planned, in this order

Each is one pull request, which also moves its row to *Available*.
The libraries that do I/O come last, each on top of the one before:
`resolv` ([cameo-lang/cameo#207](https://github.com/cameo-lang/cameo/pull/207)),
`net/http` ([cameo-lang/cameo#181](https://github.com/cameo-lang/cameo/pull/181))
and `open-uri` ([cameo-lang/cameo#193](https://github.com/cameo-lang/cameo/pull/193)).
`yaml` waits for a Go package that writes YAML as Psych does; until there
is one, cameo has no `yaml`.

`rexml` was a row of the plan, was built, and was withdrawn before it
landed: XML belongs behind a proper XML parser, chosen on its own terms,
and not in the standard library yet. It is neither planned nor refused;
`require "rexml/document"` raises `LoadError` for now.

| Library | Ruby's version | What goes in | Why this order |
|---|---|---|---|
| `resolv` | resolv 0.6.0 | `Resolv.getaddress`, `getaddresses`, `getname`, `Resolv::DNS.new.getresources` for A, AAAA, MX, TXT, CNAME, `Resolv::ResolvError`; through the host's resolver, as `net/http` goes through its client | the first of the libraries that do I/O, which come last; the host decides whether a script may look names up |
| `net/http` (`net/https`, `net/protocol`) | net-http 0.6.0 | `Net::HTTP.get`, `get_response`, `post`, `post_form`, `start`, `new` with `use_ssl`, `open_timeout`, `read_timeout`, `request`, the request classes (`Get`, `Post`, `Put`, `Patch`, `Delete`, `Head`, `Options`) with headers, `body`, `set_form_data`, `basic_auth`, `Net::HTTPResponse` with `code`, `message`, `body`, `[]`, `each_header`, `value`, `content_type`, and the response classes (`HTTPSuccess`, `HTTPNotFound`, …) for `case`; `Net::HTTPError` and the `Net::HTTPExceptions` hierarchy, `Net::OpenTimeout`, `ReadTimeout`, `SocketError` | after `resolv`; the I/O a data script needs, 131 spec files |
| `open-uri` | open-uri 0.5.0 | `URI.open(url)` and `URI::HTTP#open` with a block or `read`, over `net/http`, returning the body as a String with `base_uri`, `content_type`, `charset`, `status` on it; no `File` for a `file:` URL | over `net/http`, so after it: the one-liner that fetches a page |
| `yaml` | psych 5.2.2 | `YAML.load`, `safe_load`, `load_stream`, `dump`, `to_yaml`, `YAML.load_file` **unsupported** (a file); plain data only: scalars, arrays, hashes, symbols (`:sym`), times and dates; a `!ruby/object` tag is a `Psych::DisallowedClass`; an instance of a cameo class dumps as a mapping of its fields, as JSON does | last, and not before a Go package writes what Psych writes: Go's standard library has no YAML, gopkg.in/yaml.v3 is archived, and its successor go.yaml.in/yaml/v3 indents, quotes, folds and reports errors in another shape than libyaml's. A library is one Go package on Go's packages, not a parser carried along |

## Unsupported

A `require` raises `LoadError`. Grouped by the reason, which is one of
the decisions in [cameo-vs-ruby.md](cameo-vs-ruby.md) or
[design/cameo.md](design/cameo.md).

**The file system and the process.** cameo does no I/O beyond the standard
streams, the network and reading the files the program gives: a script is
one file, and what it reads it is given. `fileutils` 1.7.3, `tempfile` 0.3.1, `tmpdir` 0.3.1, `find` 0.2.0,
`pathname`'s I/O half (`exist?`, `read`, `children`, `glob`), `pstore`
0.1.4, `yaml/store`, `io/console` 0.8.0, `io/nonblock` 0.3.1, `io/wait`
0.3.1, `fcntl` 1.2.0, `etc` 1.4.5, `open3` 0.2.1, `pty`, `expect`,
`syslog` 0.2.0, `un` 0.3.0, `readline` 0.0.4 and `reline` 0.6.0 (in their
place, `Cameo::Kernel::Readline.readline(prompt, add_hist)` reads a line
from the terminal the host gives the program, and returns nil without
one), `socket`,
`win32ole` 1.9.1, `fiddle` 1.1.6, `nkf` 0.2.0 (encodings beyond UTF-8,
binary and US-ASCII do not exist), `English` 0.8.0 (global variables).

**The build of a Ruby.** `rbconfig` describes how a C Ruby was built and
where it was installed; a cameo script has no use for either, and the
version and platform it might want are `RUBY_VERSION`, `RUBY_ENGINE` and
`RUBY_PLATFORM`.

**Threads and processes.** cameo has `async` and `await` in their place.
`thread` (`Queue`, `Mutex`, `ConditionVariable`), `monitor`, `mutex_m`
0.3.0, `timeout` 0.4.3 (`Timeout.timeout(s) { }` stops a running block
from another thread; cameo has no thread to stop it from, and a deadline
is the host's, on the context it runs the program with), `drb` 2.2.1, `rinda` 0.2.0,
`continuation`.

**Metaprogramming, `eval` and `binding`.** cameo refuses `eval`,
`define_method`, `instance_variable_set` and `binding`, other than
`binding.break`, `binding.b` and `binding.irb`, which stop in the
debugger, before the script runs. `erb` 4.0.4 (a template is Ruby code evaluated when the template
is rendered, which is `eval` under another name; string interpolation,
`format` and heredocs are cameo's templating, and a `yaml` or `json`
document the data), `ostruct` 0.6.1 (an OpenStruct grows attributes at run time; a
class with typed fields or a Hash is cameo's form), `observer` 0.1.2 (a
mutable list of observers on the observed value), `minitest` 5.25.4,
`test-unit` 3.6.7 and `power_assert` 2.0.5 (test discovery by method
name, assertions on instance variables; `minispec`, under *Available*, is
cameo's own test library in their place), `rake` 13.2.1, `ruby2_keywords` 0.0.5 (cameo has no
legacy keyword passing to be compatible with).

**Object identity and the VM.** Values have no identity and no address.
`weakref` 0.1.3, `objspace`, `GC`, `coverage`, `ripper`, `prism` 1.2.0,
`rbs` 3.8.0, `typeprof` 0.30.1, `repl_type_completor` 0.1.9,
`did_you_mean` 2.0.0, `error_highlight` 0.7.0, `syntax_suggest` 2.0.2
(CRuby's own error machinery; cameo's checker reports its own errors),
`racc` 1.8.1, `rdoc` 6.10.0, `irb` 1.14.3 (cameo has `cameo irb`, and
`binding.irb` stops in the debugger), `json/add/*` (serialises objects by
class name and instance variables).

**Pretty printing.** `prettyprint` 0.2.0 is the line breaker under `pp`;
cameo's `pp` prints as `p` does, so nothing needs it.

**Gems.** cameo builds its libraries in and loads no code at run time.
`rubygems`, `bundler` 2.6.2.

**Network protocols nobody asked for.** `net/ftp` 0.3.8, `net/imap` 0.5.4,
`net/pop` 0.1.2, `net/smtp` 0.5.0, `rss` 0.3.1, `resolv-replace` 0.1.1,
`webrick` (not in 3.4). They would go through the host's connection as
`net/http` does, and one becomes *planned* when a script needs it.

**OpenSSL.** `openssl` 3.3.0 is a binding to a C library. TLS is Go's and
`net/http` uses it; what scripts take from OpenSSL beyond that is
`OpenSSL::Digest` and `OpenSSL::HMAC`, which `digest` carries under their
OpenSSL names. Certificates, keys, ciphers and `OpenSSL::SSL` stay out.

**Core classes left out by design**, which are not libraries but are
asked for as if they were: `Struct` and `Data` (classes declare typed
fields), `Rational` and `Complex`, `ObjectSpace`, `ENV`, `Marshal` in
Ruby's format (cameo's own is under *Available*). The reasons are in
[cameo-vs-ruby.md](cameo-vs-ruby.md).
