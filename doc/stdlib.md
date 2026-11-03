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

Ruby 3.4.1 ships 58 default gems and 31 bundled gems. Every one is in a
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
is cameo's one kind of mutable value, and these three take it rather than
being left out: a script that tokenises a string or collects output needs
them. The decision is recorded here so it is not re-opened by accident;
each handle's pull request states what its calls change.

Network I/O is the one exception to "no I/O". `net/http` is in, because a
script that fetches data is what cameo is for; it reaches the network only
through a client the Go host gives the program (`Program.HTTPClient`), so
an embedding program decides whether a script may make requests at all.
The `cameo` command gives its scripts one.

What asks for a library is measured, not guessed:

- **ruby/spec** ([specs/](../specs)): of the 1006 spec files that
  `require` a library cameo lacks, 146 want `date`, 131 `net/http`, 103
  `uri`, 94 `matrix`, 58 `stringio`, 56 `bigdecimal`, 52 `win32ole`, 44
  `strscan`, 42 `csv`, 25 `zlib`, 22 `etc`, 20 `syslog`, 15 `openssl`, 13
  `ostruct`, 13 `erb`, 12 `tempfile`, 11 `prime`, 11 `open3`, 10
  `getoptlong`, 10 `digest`, 8 `yaml`, and fewer each for the rest.
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
  `lib.Install` installs them all when the interpreter starts.
- **Installed at start, not at `require`.** `JSON` exists before `require
  "json"`, which Ruby's does not. `require` returns true the first time and
  false after, as Ruby's does.
- **Ruby's behaviour, measured against Ruby.** A library follows the
  version Ruby 3.4.1 ships, to its output and the messages of its errors,
  and carries that version in its `VERSION` constant where Ruby has one.
  Each comes with an example under [examples/](../examples) whose expected
  output Ruby 3.4 generated ([benchmarks/warmruby](../benchmarks/warmruby)
  runs CRuby 3.4.1 where the `ruby` on PATH is older), Go tests in
  [tests/](../tests), and an integration test against the binary. Where
  cameo differs, [cameo-vs-ruby.md](cameo-vs-ruby.md) says so.
- **Shared helpers come from `src/stdlib`**, which exports what a library
  needs of the core: argument checks, the `Strftime` of Time, the
  formatting of floats. A library imports `stdlib` and `object`; `stdlib`
  imports no library.
- **Pure Go.** No cgo, and a dependency only where Go's standard library
  has nothing: `yaml` carries gopkg.in/yaml.v3's port of libyaml as a
  package of its own, so its output is Ruby's, and `golang.org/x/text` for
  `unicode_normalize`.

## Available

| Library | Ruby's version | What is in |
|---|---|---|
| `set` | set 1.1.1 | `Set`, as in Ruby 3.4 without a `require`; the mutators raise as every mutator does |
| `time` | time 0.4.1 | `Time.parse`, `strptime`, `iso8601`/`xmlschema`, `Time#iso8601`; `Time.new` reads a string; `rfc2822`, `httpdate` and their writers are **planned** |
| `pp` | pp 0.6.2 | `pp` prints as `p` does: `inspect`, one value per line; no `pretty_inspect` and no line breaking at 80 columns |
| `pathname` | pathname 0.4.0 | path arithmetic without I/O: `join`, `+`, `basename`, `dirname`, `extname`, `cleanpath`, `parent`, `sub_ext`, `relative_path_from`, `each_filename`, `<=>`; `File.join`, `basename`, `dirname`, `extname`, `expand_path`; everything that reads the file system is **unsupported** |
| `cgi` (`cgi/escape`, `cgi/util`) | cgi 0.4.1 | the escapes, `escapeElement`, `rfc1123_date`, `pretty`; `CGI.new`, cookies and sessions read a request and are **unsupported** |
| `json` | json 2.9.1 | `parse`, `parse!`, `load`, `generate`, `pretty_generate`, `dump`, `JSON()`, `to_json`, the error classes; `json/add/*` is **unsupported** (it is Marshal in JSON) |
| `benchmark` | benchmark 0.4.0 | `measure`, `realtime`, `bm`, `bmbm`, `benchmark`, `Tms`; `Tms#add!` is a mutator |
| `uri` | uri 1.0.2 | `URI.parse`, `URI()`, `join`, `split`, `for`, `extract`, `scheme_list`, `URI::Generic` and `HTTP`, `HTTPS`, `FTP`, `File`, `WS`, `WSS`, `LDAP`, `LDAPS`, `MailTo` with their components, the setters (a checked copy), `merge`, `+`, `route_from`, `route_to`, `normalize`, `select`, `build`, `==`, `encode_www_form`, `decode_www_form`, the `_component` pair, `encode_uri_component`, `InvalidURIError`, `InvalidComponentError`, `BadURIError`; `merge!` and `normalize!` are bang methods; `URI.regexp` is obsolete in Ruby and raises |
| `date` | date 3.4.1 | `Date` and `DateTime`: `new`/`civil`, `jd`, `ordinal`, `commercial`, `today`, `now`, `parse` and `_parse` with Ruby's forms, `strptime`, `_strptime`, `iso8601`, `rfc3339`, `xmlschema`, `rfc2822`, `httpdate`, `jisx0301` and their `_` readers, `valid_*?`, `leap?`, the readers, `+`, `-`, `>>`, `<<`, `next_*`, `prev_*`, `step`, `upto`, `downto`, Ranges, `<=>`, `===`, `strftime` and the writers, `new_offset`, `deconstruct_keys`, `to_date`, `to_datetime`, `to_time`, `Time#to_date`, `to_datetime`, `Date::ITALY` and the other starts, the name constants, `Date::Error`; a Rational of Ruby's is an Integer when whole and a Float otherwise; `Date::Infinity` and `marshal_dump` are left out |
| `net/http` (`net/https`, `net/protocol`) | net-http 0.6.0 | `Net::HTTP.get`, `get_response`, `get_print`, `post`, `post_form`, `put`, `start`, `new`, `use_ssl=`, the timeouts, `request`, the verbs, `send_request`; the request classes with `[]`, `[]=`, `body=`, `set_form_data`, `basic_auth`, the header methods; `Net::HTTPResponse` and its classes by status with `code`, `message`, `body`, `value`, `error!`, `content_type`, `uri`; the error hierarchy, `Net::ReadTimeout`, `SocketError`, `Errno::ECONNREFUSED`, `OpenSSL::SSL::SSLError`. The network is reached through the host's client alone (`Program.HTTPClient`; the `cameo` command gives one); redirects are not followed, as in Ruby; a request blocks its goroutine. `Net::HTTP::Proxy`, streamed bodies and `verify_mode` (kept, not applied) are left out |
| time zones | — | an IANA zone name wherever Ruby takes a UTC offset: `Time.new(…, in:)` and its seventh argument, `Time.at(…, in:)`, `Time.now(in:)`, `localtime`, `getlocal`, `DateTime.new`'s offset and `new_offset`; `Time#zone` is the abbreviation in force, `utc_offset` and `dst?` follow the date; Go's `time/tzdata` is compiled in. Ruby takes a zone object there, so this is cameo's extension; an abbreviation alone is refused as Ruby refuses it |
| `digest` (`digest/md5`, `sha1`, `sha2`) | digest 3.2.0 | `Digest::MD5`, `SHA1`, `SHA256`, `SHA384`, `SHA512`, `SHA2`: `hexdigest`, `digest`, `base64digest` on the class and on an object, `update`, `<<`, `reset`, `digest_length`, `block_length`, `==`, `Digest.hexencode`, `Digest()`; `OpenSSL::HMAC.hexdigest`, `digest`, `base64digest`, `OpenSSL::Digest.new`, `hexdigest`, `OpenSSL::Digest::SHA256` and kin. An object is a value: `update` returns the copy. `RMD160` (no RIPEMD-160 in Go) and `file` are left out |
| `base64` | base64 0.2.0 | `encode64`, `decode64`, `strict_encode64`, `strict_decode64`, `urlsafe_encode64` (`padding:`), `urlsafe_decode64`, with Ruby's line breaks, lenient decoder and errors |
| `securerandom` (`random/formatter`) | securerandom 0.4.1 | `SecureRandom.hex`, `uuid`, `uuid_v4`, `uuid_v7`, `random_bytes`, `bytes`, `gen_random`, `random_number`, `rand`, `alphanumeric` (`chars:`), `base64`, `urlsafe_base64`, drawing from Go's `crypto/rand`; `Random::Formatter` gives the same methods to a `Random` and to `Random` itself, and a seeded `Random` draws CRuby's values |
| `yaml` (`psych`) | psych 5.2.2 on libyaml 0.2.5 | `YAML.load`, `safe_load`, `unsafe_load`, `load_stream` with `permitted_classes:`, `permitted_symbols:`, `aliases:`, `symbolize_names:`, `fallback:`, `filename:`, `strict_integer:`; `dump`, `safe_dump`, `dump_stream`, `to_yaml` with `indentation:`, `line_width:`, `header:`; Psych's scalar rules (YAML 1.1 booleans, `:sym`, `1:30`, dates and times when their class is permitted), the merge key `<<`, `!!binary`, `!ruby/range`, `!ruby/regexp`, `!ruby/object:Set`, `!ruby/exception`, and an instance of a cameo class by its fields under `!ruby/object:Name`; `Psych::SyntaxError` with libyaml's message, line and column, `DisallowedClass`, `AliasesNotEnabled`, `AnchorNotDefined`. The parser and emitter are gopkg.in/yaml.v3's port of libyaml, carried in `src/lib/yaml/libyaml` under its MIT license with libyaml 0.2.5's emitter rules restored, so documents, quoting, folding and errors are Ruby's. `load_file` and the other file methods, `parse` and the node tree, `Psych::Set` (a Hash in cameo) and `Struct` are left out |
| `csv` | csv 3.3.2 | `CSV.parse` (with a block too), `parse_line`, `generate`, `generate_line`, `generate_lines`, `String#parse_csv`, `Array#to_csv`; the options `col_sep:`, `row_sep:` (`:auto`), `quote_char:` (`nil` too), `headers:` (true, an Array, a String), `return_headers:`, `write_headers:`, `converters:` and `header_converters:` (Ruby's by name, or a Proc of one or two arguments with `CSV::FieldInfo`), `skip_blanks:`, `skip_lines:`, `liberal_parsing:`, `strip:`, `nil_value:`, `empty_value:`, `write_nil_value:`, `write_empty_value:`, `force_quotes:`, `quote_empty:`; `CSV::Row` and `CSV::Table` with their readers, `by_col`, `by_row`, `each`, `to_a`, `to_csv`, `==`, `inspect`; `CSV.new(string)` as a handle with `shift`, `read`, `each`, `lineno`, `line`, `headers`, `eof?`, `rewind`, `<<`, `string`; `CSV::MalformedCSVError` with Ruby's messages and `line_number`. A Row and a Table are values: `[]=`, `<<`, `push`, `delete` and `delete_if` return the changed copy, and `delete` returns the Row or Table without what it removed, not the removed part. `CSV.read`, `foreach`, `open`, `table` and the other file methods, `by_col!` and `by_row!` (bang methods), `CSV::Converters` and `HeaderConverters` as Hashes of Procs are left out |
| `marshal` | — (cameo's wire form) | `Marshal.dump`, `load`, `restore`, `load(…, freeze:)`, with `dump`'s depth limit accepted; the bytes are the protobuf `cameo.Value` envelope every value crosses to a host in, what `to_value_proto` gives, so cameo reads them back and any protobuf reader with `cameo/value.proto` reads them too. An instance of a cameo class is found again by its class's name and digest. A Proc, a class, a Random and an Enumerator raise `TypeError`, as a Proc does in Ruby; so does an instance of a library's class, a Date or a URI, which the envelope does not carry; a Symbol comes back as a String, as it crosses to a host. Ruby's format, an IO to write to or read from, the `proc` argument of `load`, `MAJOR_VERSION` and `MINOR_VERSION` are left out |
| `shellwords` | shellwords 0.2.2 | `Shellwords.split` (`shellsplit`, `shellwords`), `escape` (`shellescape`), `join` (`shelljoin`), `String#shellsplit`, `shellescape`, `Array#shelljoin`, with Ruby's quoting rules and its `ArgumentError` for an unmatched quote |
| `unicode_normalize` | — (core) | `String#unicode_normalize` and `unicode_normalized?` with `:nfc`, `:nfd`, `:nfkc`, `:nfkd`, on `golang.org/x/text/unicode/norm`, there without a `require` as in Ruby; a binary String is `Encoding::CompatibilityError`; `unicode_normalize!` is a bang method |
| `ipaddr` | ipaddr 1.2.7 | `IPAddr.new` from a string with a prefix length or a mask, or an Integer with its family, `to_s` compressed as Ruby compresses, `to_string`, `inspect`, `to_i`, `prefix`, `netmask`, `family`, `include?`, `===`, `==`, `eql?`, `<=>` and Comparable, `mask`, `to_range`, `succ` (so a Range of addresses iterates), `private?`, `loopback?`, `link_local?`, `ipv4?`, `ipv6?`, `ipv4_mapped?`, `ipv4_compat?`, `ipv4_mapped`, `ipv4_compat`, `native`, `\|`, `&`, `<<`, `>>`, `~`, `reverse`, `ip6_arpa`, `ip6_int`, `zone_id`, `hton`, `IPAddr.ntop`, the errors `IPAddr::Error`, `InvalidAddressError`, `InvalidPrefixError`, `AddressFamilyError`; `Socket::AF_INET`, `AF_INET6` and `AF_UNSPEC` are defined for the family argument. An IPAddr is a value with four fields, so it is a Hash key and a Set member; `mask!` and `set` are mutators, `mask` returns the copy; an error message names the address where ipaddr 1.2.6's left it out |
| `zlib` | zlib 3.2.1 | `Zlib.crc32`, `adler32`, `crc_table`, `Zlib::Deflate.deflate` and `Zlib.deflate` with a level, `Zlib::Inflate.inflate` and `Zlib.inflate`, `Zlib.gzip` (`level:`) and `gunzip`, the constants, `Zlib::Error` and its kinds, `Zlib::GzipFile::Error`, `CRCError`, `LengthError`, `NoFooter`, with zlib's messages. On Go's `compress/zlib`, `compress/flate` and `hash`: what deflates here inflates in Ruby and the other way round, the headers, checksums and errors are zlib's, but the compressed bytes are Go's deflate's, not byte for byte zlib's. `Zlib::Deflate.new`, `Inflate.new` and the streaming methods, `GzipReader` and `GzipWriter` on an IO are left out |
| `open-uri` | open-uri 0.5.0 | `URI.open(url, mode, headers, options)` with a block or not, `URI::HTTP#open` and `#read`, `OpenURI.open_uri`; the options `redirect:`, `max_redirects:`, `http_basic_authentication:`, `read_timeout:`, `encoding:` and the `r:encoding` mode, header fields as String keys; redirects followed as Ruby follows them, with its RuntimeErrors for a forbidden one and a loop; `OpenURI::HTTPError` with `io`, `HTTPRedirect` with `uri`, `TooManyRedirects`. What `open` returns is a value, `OpenURI::Document`, with `read`, `base_uri`, `status`, `meta`, `metas`, `content_type`, `charset`, `content_encoding`, `last_modified`, where Ruby returns a StringIO or Tempfile with those methods mixed in; `URI::HTTP#read` gives a plain String. The body's encoding follows the charset as in Ruby: binary without one. A file name, a `file:` or `ftp:` URL and the proxy options are refused; the network is the host's client's, as for `net/http` |
| `prime` | prime 0.1.3 | `Prime.each` with a bound or endless, with a block or as an Enumerator, `prime?`, `include?`, `prime_division`, `int_from_prime_division`, `instance`, and the Enumerable methods on `Prime` itself (`first`, `take`, `lazy`, `select`…); `Integer#prime?`, `prime_division`, `Integer.from_prime_division`, `each_prime`. `prime?` is a Miller-Rabin test and `prime_division` divides out small factors then runs Pollard's rho, so a large number answers where Ruby's trial division does not finish; `Prime.each` returns an Enumerator, not `Prime::EratosthenesGenerator` |
| `tsort` | tsort 0.2.0 | `TSort.tsort`, `tsort_each`, `strongly_connected_components`, `each_strongly_connected_component`, `each_strongly_connected_component_from` over two callables, with a block or as an Enumerator, and the same as the methods of a class that includes `TSort` and defines `tsort_each_node` and `tsort_each_child`; `TSort::Cyclic` with Ruby's message. The `id_map` and `stack` arguments start fresh each call, since a Hash is a value |
| `abbrev` | abbrev 0.1.2 | `Abbrev.abbrev(words, pattern = nil)` and `Array#abbrev`, with a String prefix or a Regexp as the pattern |
| `singleton` | singleton 0.2.0 | `include Singleton`: `instance` is the one instance, made by `initialize` the first time, a subclass with one of its own; `new` and `allocate` are refused with Ruby's NoMethodError, `dup` and `clone` with its TypeError. The module uses a new hook, `Class.Included`, which the evaluator runs when a class includes a module |
| `rbconfig` | — | `RbConfig::CONFIG` and `MAKEFILE_CONFIG` with the version parts (`MAJOR`, `MINOR`, `TEENY`, `ruby_version`, `RUBY_PROGRAM_VERSION`), the names (`ruby_install_name` is `"cameo"`, `RUBY_ENGINE`), the host (`host_os`, `host_cpu`, `host`, `arch` as `RUBY_PLATFORM`, `EXEEXT`, `DLEXT`, `PATH_SEPARATOR`) and the running binary's directories (`bindir`, `prefix`, `libdir`…), `RbConfig::TOPDIR`, `RbConfig.ruby`, `RbConfig.expand`. The build details of a C Ruby, the compiler and its flags, are left out |
| `forwardable`, `delegate` | forwardable 1.3.3, delegate 0.4.0 | `extend Forwardable` with `def_delegator`, `def_delegators`, `def_instance_delegator(s)`, `delegate … => :accessor` and `instance_delegate`, the accessor a method or a field (`:@items` names the field), and `SingleForwardable` with `def_single_delegator(s)` and `single_delegate` on a class or module; `SimpleDelegator`, `Delegator` and `DelegateClass(klass)` forwarding what they lack to the wrapped object through `method_missing`, with `__getobj__`, `respond_to?`, `==`, `to_s`, `inspect` and the other Object methods forwarded. A delegator is a value: `__setobj__` returns the copy with the new object. An accessor given as an expression String is refused, since cameo evaluates no strings. The evaluator now dispatches to a library's native `method_missing` too |
| `logger` | logger 1.6.4 | `Logger.new(STDOUT`, `STDERR` or `nil, level:, progname:, formatter:, datetime_format:)`, the levels as Integers, Symbols or Strings, `debug` to `unknown` with a message, a progname and a block, `add` and `log`, `<<`, the predicates, `level=`, `progname=`, `formatter=`, `datetime_format=`, `close`; `Logger::Formatter` with Ruby's line, `Logger::SEV_LABEL`, `Logger::Severity`, `Logger::Error`. A Logger is a handle: the setters change it for every holder, as in Ruby. A file name, log rotation and `Logger::LogDevice` are left out, since cameo writes no files |
| `stringio` | stringio 3.1.2 | `StringIO.new(string, mode)` and `open`, `read`, `gets`, `readline`, `readlines`, `each_line`, `each_char`, `each_byte`, `getc`, `readchar`, `getbyte`, `readbyte`, `ungetc`, `ungetbyte`, `readpartial`, `pread` with Ruby's separators, limits and `chomp:`; `write`, `<<`, `print`, `puts`, `printf`, `truncate`, writing over the position and padding past the end; `string`, `string=`, `reopen`, `pos`, `pos=`, `seek` with `IO::SEEK_SET`, `SEEK_CUR` and `SEEK_END`, `rewind`, `eof?`, `size`, `lineno`, `close`, `close_read`, `close_write` and the predicates, the modes `r`, `w`, `a` and `+`, with Ruby's `IOError`, `EOFError` and `Errno::EINVAL`. A StringIO is a handle; `string` returns the String as it is then, where Ruby returns the buffer itself |
| `strscan` | strscan 3.1.2 | `StringScanner.new(string, fixed_anchor:)`, `scan`, `scan_until`, `skip`, `skip_until`, `check`, `check_until`, `match?`, `exist?`, `scan_full`, `search_full` with a Regexp or a String, the match record with `matched`, `matched?`, `matched_size`, `pre_match`, `post_match`, `[]` by index or name, `size`, `captures`, `values_at`, `named_captures`, `getch`, `get_byte`, `scan_byte`, `peek`, `peek_byte`, `unscan`, `reset`, `terminate`, `pos`, `pos=`, `charpos`, `eos?`, `rest`, `rest_size`, `string`, `string=`, `concat`, `<<`, `beginning_of_line?`, `inspect` in Ruby's form, `StringScanner::Error`, `Version`. A StringScanner is a handle. With `fixed_anchor: true` the match at the position is found by searching the whole string, so `\A` holds at the start, as in Ruby |
| `optparse`, `getoptlong`, `ARGV` | optparse 0.6.0, getoptlong 0.2.1 | `ARGV` holds the script's arguments, `cameo script.cameo a -b c` (a host sets them with `Program.Args`); `OptionParser.new(banner, width, indent)` with a block, `on`, `on_tail`, `on_head`, `separator`, `banner`, `program_name`, `version`, `release`, `summary_width`, `summary_indent`, `help`, `to_s`, `to_a`, `summarize`, `ver`, `parse`, `parse!`, `order`, `order!`, `permute`, `permute!` on an Array or on `ARGV`, `getopts`; the switch forms (`-v`, `-nNAME`, `-n NAME`, `--name NAME`, `--name=NAME`, `[ARG]` optional, `--[no-]name`, `--no-name`), the types `Integer`, `Float`, `Numeric`, `String`, `Array`, `TrueClass`, an Array or Hash of permitted values completed by a unique prefix, long options completed by a unique prefix, the built-in `--help` and `--version` that print and raise `SystemExit`, the help text with Ruby's layout, `OptionParser::ParseError` with `args` and `reason` and its kinds `InvalidOption`, `MissingArgument`, `InvalidArgument`, `NeedlessArgument`, `AmbiguousOption`, `AmbiguousArgument`, with Ruby's messages; `GetoptLong.new` with Ruby's checks of the names and flags, `each`, `get`, `terminate`, `terminated?`, `ordering`, `quiet`, `error`, `error_message`, the constants, `GetoptLong::Error` and its kinds, an error printed to standard error as `file: message` unless quiet. An Array is a value, so `parse!` returns the operands and leaves its argument as it is; the options go through the blocks, or `getopts` returns them as a Hash; `into:` is refused with that advice. `GetoptLong` reads a copy of ARGV and gives the operands in `rest`, which Ruby lacks, since it leaves them in ARGV. A `Numeric` fraction is a Float, a `Symbol`, `Regexp` or other acceptor and `OptionParser.accept` are left out, as is `$0` (the help names the script file) |
| `rexml` (`rexml/document`) | rexml 3.4.0 | `REXML::Document.new(string, context)` with `raw:`, `compress_whitespace:` and `ignore_whitespace_nodes:`; `Element` with `name`, `expanded_name`, `prefix`, `namespace`, `namespaces`, `attributes`, `elements`, `children`, `text`, `get_text`, `texts`, `cdatas`, `comments`, `has_elements?`, `has_text?`, `add_element`, `add_text`, `text=`, `add_attribute`, `add_attributes`, `delete_element`, `delete_attribute`, `attribute`, `[]`, `each_element`, `each_element_with_attribute`, `each_element_with_text`, `get_elements`, `each_recursive`, `find_first_recursive`, `next_element`, `previous_element`, `insert_before`, `insert_after`, `replace_child`, `remove`, `parent`, `root`, `document`, `xpath`, `to_s`, `inspect`, `write`, `deep_clone`, `clone`; `Elements` and `Attributes` with `[]`, `each`, `each_attribute`, `to_a`, `size`, `add`, `delete`, `delete_all` and Enumerable; `Text` (`value`, `<<`, `Text.normalize`, `unnormalize`), `CData`, `Comment`, `Instruction`, `XMLDecl`, `DocType` with its entities, `Attribute`; `XPath.first`, `match` and `each` with XPath 1.0: every axis, the node tests, predicates, the operators, the core functions, `$variables` and namespace prefixes; `Formatters::Default` and `Pretty` with `compact` and `width`, `write(output, indent, ie_hack)` to `$stdout`, a StringIO or an IO; `ParseException` with REXML's messages, `line` and `position`. The parser is cameo's own, so an entity the DOCTYPE declares resolves and an unknown one stays as written. The nodes are handles, as in Ruby: the tree changes for every holder and `==` is identity. A node-set holds each node once in document order where REXML repeats some; `normalize-space` works in a predicate where REXML's does not; a bad path raises `ParseException` where REXML fails obscurely; a Text's `to_s` is a String, not Ruby's live buffer; `elements[1] = x` and `attributes["a"] = v` need a local receiver, so `replace_child` and `add_attribute` serve. Files and streams, the stream parsers (`Parsers`, `SAX2`, `Light`), `Entity`, `Transitive` and `Element#xpath` of a detached node are left out |
| `debug` | debug 1.10.0 | `debugger` and `binding.break` stop in cameo's debugger, which takes rdbg's commands ([debugger.md](debugger.md)) |
| `cameo/runner` | — | cameo's own: the library that gives CRuby `Cameo::Runner` is built in |

## Planned, in this order

Each is one pull request, which also moves its row to *Available*.

| Library | Ruby's version | What goes in | Why this order |
|---|---|---|---|
| `matrix` | matrix 0.4.2 | `Matrix` and `Vector`: construction, `[]`, `row`, `column`, `+`, `-`, `*`, `**`, `/`, `transpose`, `determinant`, `inverse`, `rank`, `trace`, `each`, `map`, `minor`, `identity`, `zero`, `Vector#inner_product`, `cross_product`, `norm`, `Matrix::ErrDimensionMismatch` and kin; Ruby's are immutable but for `[]=`, which is a mutator | 94 spec files; pure |
| `bigdecimal` (`bigdecimal/util`) | bigdecimal 3.1.8 | `BigDecimal` as a new Numeric kind of value: `BigDecimal("1.23")`, arithmetic with Integer and Float coercion, `round`, `floor`, `ceil`, `truncate` with Ruby's modes, `to_s` in Ruby's forms (`"0.123e1"`, `to_s("F")`), `precision`, `scale`, `sqrt`, `to_d` on String, Integer and Float, `BigDecimal.limit`, `mode`; on `math/big` | money; 56 spec files; the largest, so last of the pure ones |
| `resolv` | resolv 0.6.0 | `Resolv.getaddress`, `getaddresses`, `getname`, `Resolv::DNS.new.getresources` for A, AAAA, MX, TXT, CNAME, `Resolv::ResolvError`; through the host's resolver, as `net/http` goes through its client | after `net/http`; the host decides whether a script may look names up |

## Unsupported

A `require` raises `LoadError`. Grouped by the reason, which is one of
the decisions in [cameo-vs-ruby.md](cameo-vs-ruby.md) or
[design/cameo.md](design/cameo.md).

**The file system and the process.** cameo does no I/O beyond the standard
streams and the network: a script is one file, and what it reads it is
given. `fileutils` 1.7.3, `tempfile` 0.3.1, `tmpdir` 0.3.1, `find` 0.2.0,
`pathname`'s I/O half (`exist?`, `read`, `children`, `glob`), `pstore`
0.1.4, `yaml/store`, `io/console` 0.8.0, `io/nonblock` 0.3.1, `io/wait`
0.3.1, `fcntl` 1.2.0, `etc` 1.4.5, `open3` 0.2.1, `pty`, `expect`,
`syslog` 0.2.0, `un` 0.3.0, `readline` 0.0.4, `reline` 0.6.0, `socket`,
`win32ole` 1.9.1, `fiddle` 1.1.6, `nkf` 0.2.0 (encodings beyond UTF-8,
binary and US-ASCII do not exist), `english` 0.8.0 (global variables).

**Threads and processes.** cameo has `async` and `await` in their place.
`thread` (`Queue`, `Mutex`, `ConditionVariable`), `monitor`, `mutex_m`
0.3.0, `timeout` 0.4.3 (`Timeout.timeout(s) { }` stops a running block
from another thread; cameo has no thread to stop it from, and a deadline
is the host's, on the context it runs the program with), `drb` 2.2.1, `rinda` 0.2.0,
`continuation`.

**Metaprogramming, `eval` and `binding`.** cameo refuses `eval`,
`define_method`, `instance_variable_set` and `binding` before the script
runs. `erb` 4.0.4 (a template is Ruby code evaluated when the template
is rendered, which is `eval` under another name; string interpolation,
`format` and heredocs are cameo's templating, and a `yaml` or `json`
document the data), `ostruct` 0.6.1 (an OpenStruct grows attributes at run time; a
class with typed fields or a Hash is cameo's form), `observer` 0.1.2 (a
mutable list of observers on the observed value), `minitest` 5.25.4,
`test-unit` 3.6.7 and `power_assert` 2.0.5 (test discovery by method
name, assertions on instance variables; a test helper of cameo's own is the
answer, not these), `rake` 13.2.1, `ruby2_keywords` 0.0.5 (cameo has no
legacy keyword passing to be compatible with).

**Object identity and the VM.** Values have no identity and no address.
`weakref` 0.1.3, `objspace`, `GC`, `coverage`, `ripper`, `prism` 1.2.0,
`rbs` 3.8.0, `typeprof` 0.30.1, `repl_type_completor` 0.1.9,
`did_you_mean` 2.0.0, `error_highlight` 0.7.0, `syntax_suggest` 2.0.2
(CRuby's own error machinery; cameo's checker reports its own errors),
`racc` 1.8.1, `rdoc` 6.10.0, `irb` 1.14.3 (cameo has `cameo irb`;
`binding.irb` needs a binding), `json/add/*` (serialises objects by class
name and instance variables).

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
Ruby's format (see *Planned* for cameo's). The reasons are in
[cameo-vs-ruby.md](cameo-vs-ruby.md).

## Progress

Pull requests, as they open and land:

- [cameo-lang/cameo#173](https://github.com/cameo-lang/cameo/pull/173): this page, the proposal.
- [cameo-lang/cameo#174](https://github.com/cameo-lang/cameo/pull/174), [#175](https://github.com/cameo-lang/cameo/pull/175), [#176](https://github.com/cameo-lang/cameo/pull/176), [#177](https://github.com/cameo-lang/cameo/pull/177), [#178](https://github.com/cameo-lang/cameo/pull/178): `json`, `cgi`, `benchmark`, `pathname` and `time` move into packages of their own under `src/lib`.
- [cameo-lang/cameo#179](https://github.com/cameo-lang/cameo/pull/179): `uri`.
- [cameo-lang/cameo#180](https://github.com/cameo-lang/cameo/pull/180): `date`.
- [cameo-lang/cameo#181](https://github.com/cameo-lang/cameo/pull/181): `net/http`.
- [cameo-lang/cameo#182](https://github.com/cameo-lang/cameo/pull/182): time zones.
- [cameo-lang/cameo#183](https://github.com/cameo-lang/cameo/pull/183): `digest`.
- [cameo-lang/cameo#184](https://github.com/cameo-lang/cameo/pull/184): `base64`.
- [cameo-lang/cameo#185](https://github.com/cameo-lang/cameo/pull/185): `securerandom`.
- [cameo-lang/cameo#186](https://github.com/cameo-lang/cameo/pull/186): `yaml`.
- [cameo-lang/cameo#187](https://github.com/cameo-lang/cameo/pull/187): `csv`.
- [cameo-lang/cameo#188](https://github.com/cameo-lang/cameo/pull/188): `marshal`.
- [cameo-lang/cameo#189](https://github.com/cameo-lang/cameo/pull/189): `shellwords`.
- [cameo-lang/cameo#190](https://github.com/cameo-lang/cameo/pull/190): `unicode_normalize`.
- [cameo-lang/cameo#191](https://github.com/cameo-lang/cameo/pull/191): `ipaddr`.
- [cameo-lang/cameo#192](https://github.com/cameo-lang/cameo/pull/192): `zlib`.
- [cameo-lang/cameo#193](https://github.com/cameo-lang/cameo/pull/193): `open-uri`.
- [cameo-lang/cameo#194](https://github.com/cameo-lang/cameo/pull/194): `prime`.
- [cameo-lang/cameo#195](https://github.com/cameo-lang/cameo/pull/195): `tsort`.
- [cameo-lang/cameo#196](https://github.com/cameo-lang/cameo/pull/196): `abbrev`.
- [cameo-lang/cameo#197](https://github.com/cameo-lang/cameo/pull/197): `singleton`.
- [cameo-lang/cameo#198](https://github.com/cameo-lang/cameo/pull/198): `rbconfig`.
- [cameo-lang/cameo#199](https://github.com/cameo-lang/cameo/pull/199): `forwardable`, `delegate`.
- [cameo-lang/cameo#200](https://github.com/cameo-lang/cameo/pull/200): `logger`.
- [cameo-lang/cameo#201](https://github.com/cameo-lang/cameo/pull/201): `stringio`.
- [cameo-lang/cameo#202](https://github.com/cameo-lang/cameo/pull/202): `strscan`.
- [cameo-lang/cameo#203](https://github.com/cameo-lang/cameo/pull/203): `optparse`, `getoptlong`, `ARGV`.
- `rexml`: available.
