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
through a client the Go host gives the program, so an embedding program
decides whether a script may make requests at all (see *net/http* below).

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
  has nothing: `golang.org/x/text` for `unicode_normalize`.

## Available

| Library | Ruby's version | What is in |
|---|---|---|
| `set` | set 1.1.1 | `Set`, as in Ruby 3.4 without a `require`; the mutators raise as every mutator does |
| `time` | time 0.4.1 | `Time.parse`, `strptime`, `iso8601`/`xmlschema`, `Time#iso8601`; `Time.new` reads a string; `rfc2822`, `httpdate` and their writers are **planned**. The reading is Go's time package: `Time.parse` tries the forms of a fixed list (ISO 8601, RFC 2822, HTTP dates, dates with month names, slashed and dashed dates, times of day), `strptime`'s format becomes a Go layout, and a zone is read with Go's zone layouts; a directive Go has no layout for (`%j` with a year, `%U`, `%V`, `%G`, `%C`, a `%s` beside other directives) is an ArgumentError, and text around a date, an ordinal suffix or an era word is not read |
| `pp` | pp 0.6.2 | `pp` prints as `p` does: `inspect`, one value per line; no `pretty_inspect` and no line breaking at 80 columns |
| `pathname` | pathname 0.4.0 | path arithmetic without I/O: `join`, `+`, `basename`, `dirname`, `extname`, `cleanpath`, `parent`, `sub_ext`, `relative_path_from`, `each_filename`, `<=>`; `File.join`, `basename`, `dirname`, `extname`, `expand_path`; everything that reads the file system is **unsupported** |
| `cgi` (`cgi/escape`, `cgi/util`) | cgi 0.4.1 | the escapes, `escapeElement`, `rfc1123_date`, `pretty`; `CGI.new`, cookies and sessions read a request and are **unsupported** |
| `json` | json 2.9.1 | `parse`, `parse!`, `load`, `generate`, `pretty_generate`, `dump`, `JSON()`, `to_json`, the error classes; `json/add/*` is **unsupported** (it is Marshal in JSON) |
| `benchmark` | benchmark 0.4.0 | `measure`, `realtime`, `bm`, `bmbm`, `benchmark`, `Tms`; `Tms#add!` is a mutator |
| `uri` | uri 1.0.2 | `URI.parse`, `URI()`, `join`, `split`, `for`, `extract`, `scheme_list`, `URI::Generic` and `HTTP`, `HTTPS`, `FTP`, `File`, `WS`, `WSS`, `LDAP`, `LDAPS`, `MailTo` with their components, `merge`, `+`, `route_from`, `route_to`, `normalize`, `select`, `build`, `==`, `encode_www_form`, `decode_www_form`, the `_component` pair, `encode_uri_component`, `InvalidURIError`, `InvalidComponentError`, `BadURIError`. A URI has no setters: `u.host = x` and `u.with(host: x)` raise `NoMethodError`, since a URI is parsed, joined, merged or built, not changed; `merge!` and `normalize!` are bang methods; `URI.regexp` is obsolete in Ruby and raises. The parsing, the resolution of a reference and the escaping are Go's `net/url`, so `encode_www_form_component` keeps `~` and escapes `*`, the other way round from Ruby |
| `date` | date 3.4.1 | `Date` and `DateTime`: `new`/`civil`, `jd`, `ordinal`, `commercial`, `today`, `now`, `parse` and `_parse` with Ruby's forms, `strptime`, `_strptime`, `iso8601`, `rfc3339`, `xmlschema`, `rfc2822`, `httpdate`, `jisx0301` and their `_` readers, `valid_*?`, `leap?`, the readers, `+`, `-`, `>>`, `<<`, `next_*`, `prev_*`, `step`, `upto`, `downto`, Ranges, `<=>`, `===`, `strftime` and the writers, `new_offset`, `deconstruct_keys`, `to_date`, `to_datetime`, `to_time`, `Time#to_date`, `to_datetime`, `Date::ITALY` and the other starts, the name constants, `Date::Error`; a Rational of Ruby's is an Integer when whole and a Float otherwise; `Date::Infinity` and `marshal_dump` are left out |
| time zones | — | an IANA zone name wherever Ruby takes a UTC offset: `Time.new(…, in:)` and its seventh argument, `Time.at(…, in:)`, `Time.now(in:)`, `localtime`, `getlocal`, `DateTime.new`'s offset and `new_offset`; `Time#zone` is the abbreviation in force, `utc_offset` and `dst?` follow the date; Go's `time/tzdata` is compiled in. Ruby takes a zone object there, so this is cameo's extension; an abbreviation alone is refused as Ruby refuses it |
| `debug` | debug 1.10.0 | `debugger` and `binding.break` stop in cameo's debugger, which takes rdbg's commands ([debugger.md](debugger.md)) |
| `cameo/runner` | — | cameo's own: the library that gives CRuby `Cameo::Runner` is built in |

## Planned, in this order

Each is one pull request, which also moves its row to *Available*.

| Library | Ruby's version | What goes in | Why this order |
|---|---|---|---|
| `net/http` (`net/https`, `net/protocol`) | net-http 0.6.0 | `Net::HTTP.get`, `get_response`, `post`, `post_form`, `start`, `new` with `use_ssl`, `open_timeout`, `read_timeout`, `request`, the request classes (`Get`, `Post`, `Put`, `Patch`, `Delete`, `Head`, `Options`) with headers, `body`, `set_form_data`, `basic_auth`, `Net::HTTPResponse` with `code`, `message`, `body`, `[]`, `each_header`, `value`, `content_type`, and the response classes (`HTTPSuccess`, `HTTPNotFound`, …) for `case`; `Net::HTTPError` and the `Net::HTTPExceptions` hierarchy, `Net::OpenTimeout`, `ReadTimeout`, `SocketError` | the I/O a data script needs; 131 spec files |
| `digest` (`digest/md5`, `sha1`, `sha2`, `sha256`…) | digest 3.2.0 | `Digest::MD5`, `SHA1`, `SHA256`, `SHA384`, `SHA512`, `RMD160`, `digest`, `hexdigest`, `base64digest`, `file` **unsupported**; an instance is a finished digest (`Digest::SHA256.new.update(s)` returns the updated digest and rebinds nothing, as every cameo value) | signing requests to APIs |
| `base64` | base64 0.2.0 | `encode64`, `decode64`, `strict_*`, `urlsafe_*` | with `digest` |
| `securerandom` (`random/formatter`) | securerandom 0.4.1 | `hex`, `uuid`, `uuid_v4`, `uuid_v7`, `random_bytes`, `random_number`, `alphanumeric`, `base64`, `urlsafe_base64`, `bytes`, `Random::Formatter` on `Random` too, drawing from Go's `crypto/rand` | tokens and ids; cheap |
| `csv` | csv 3.3.2 | `CSV.parse`, `parse_line`, `generate`, `generate_line`, `headers: true`, `CSV::Row` and `Table`, `converters: :numeric`, quoting and `col_sep`; `CSV.read`, `foreach`, `open` **unsupported** (files) | 42 spec files; data scripts |
| `marshal` | — | `Marshal.dump` and `load` in **cameo's wire form**, not Ruby's: the bytes are the protobuf envelope every value crosses to a host in, so cameo reads them back and any protobuf reader with the `.proto` reads them too; a Proc, a Random and an Enumerator raise `TypeError`, as a Proc does in Ruby; a class is found by name and digest, as `from_proto` finds it | Ruby's format carries object identity and instance variables cameo has neither of; a round trip inside cameo is what a script wants |
| `shellwords` | shellwords 0.2.2 | `split`, `escape`, `join`, `String#shellsplit`, `shellescape`, `Array#shelljoin` | small; what builds a command line for a host to run |
| `unicode_normalize` | — (core) | `String#unicode_normalize`, `unicode_normalized?` with `:nfc`, `:nfd`, `:nfkc`, `:nfkd`, on `golang.org/x/text/unicode/norm` | the one core String method missing from text handling |
| `ipaddr` | ipaddr 1.2.7 | `IPAddr.new`, `include?`, `===`, `to_s`, `to_string`, `mask`, `to_range`, `ipv4?`, `ipv6?`, `private?`, `loopback?`, `succ`, `<=>` | small; pure |
| `zlib` | zlib 3.2.1 | `crc32`, `adler32`, `Zlib::Deflate.deflate`, `Zlib::Inflate.inflate`, `Zlib.gzip`, `Zlib.gunzip`; `GzipReader`/`GzipWriter` on an IO **unsupported** | 25 spec files; compressed payloads from `net/http` |
| `open-uri` | open-uri 0.5.0 | `URI.open(url)` and `URI::HTTP#open` with a block or `read`, over `net/http`, returning the body as a String with `base_uri`, `content_type`, `charset`, `status` on it; no `File` for a `file:` URL | the one-liner that fetches a page |
| `prime` | prime 0.1.3 | `Prime.each`, `prime?`, `first`, `take`, `prime_division`, `int_from_prime_division`, `Integer#prime?`, `prime_division`, `Prime.each(n)` lazily | pure, small |
| `tsort` | tsort 0.2.0 | `TSort.tsort`, `tsort_each`, `strongly_connected_components`, `each_strongly_connected_component` as module functions on a Hash of children, and `include TSort` with `tsort_each_node` and `tsort_each_child` | pure, small |
| `abbrev` | abbrev 0.1.2 | `Abbrev.abbrev`, `Array#abbrev` | pure, tiny |
| `singleton` | singleton 0.3.0 | `include Singleton`: `instance` returns the one value, `new` is undefined; the value is immutable, so there is one and it never changes | small |
| `forwardable`, `delegate` | forwardable 1.3.3, delegate 0.4.0 | `extend Forwardable` with `def_delegator` and `def_delegators` on a field or a method, `delegate … to:`; `SimpleDelegator` and `DelegateClass` forwarding to the wrapped value through `method_missing`, `__getobj__`; `__setobj__` is a mutator | a class-body call that defines methods, which the checker has to learn as it learnt bang methods |
| `logger` | logger 1.6.4 | `Logger.new($stderr)`, the levels and their predicates, `formatter`, `progname`, `datetime_format`, `Logger::Formatter`; a Logger is a handle on a stream, so `level=` changes it for every holder | the way a script reports progress |
| `stringio` | stringio 3.1.2 | `StringIO.new`, `read`, `gets`, `each_line`, `readlines`, `getc`, `write`, `<<`, `puts`, `print`, `printf`, `string`, `pos`, `rewind`, `eof?`, `truncate`, `closed?`; a handle | 58 spec files; builds output and feeds text to code that reads an IO |
| `strscan` | strscan 3.1.2 | `StringScanner.new`, `scan`, `scan_until`, `skip`, `skip_until`, `check`, `check_until`, `match?`, `matched`, `matched?`, `pre_match`, `post_match`, `[]`, `captures`, `pos`, `pos=`, `eos?`, `rest`, `peek`, `getch`, `unscan`, `reset`, `terminate`, `string`; a handle | 44 spec files; tokenisers |
| `ARGV`, `optparse`, `getoptlong` | optparse 0.6.0, getoptlong 0.2.1 | `ARGV` from `cameo script.cameo -- args`, and from the host through a Load option, then `OptionParser` with `on`, `parse!` returning the rest (`parse!` rebinds nothing: it returns the operands and the options go through the block), `banner`, `help`, `OptionParser::InvalidOption`; `GetoptLong` the same way | a script run from the command line takes arguments |
| `rexml` | rexml 3.4.0 | `REXML::Document.new(string)`, `root`, `elements`, `each_element`, `attributes`, `text`, `name`, `XPath.first`, `XPath.match` for the paths of `//a/b[@c='d']` and `text()`, `Element#to_s`, `Document#write`, `Formatters::Pretty`, `REXML::ParseException`; on Go's `encoding/xml` | XML responses from `net/http` |
| `matrix` | matrix 0.4.2 | `Matrix` and `Vector`: construction, `[]`, `row`, `column`, `+`, `-`, `*`, `**`, `/`, `transpose`, `determinant`, `inverse`, `rank`, `trace`, `each`, `map`, `minor`, `identity`, `zero`, `Vector#inner_product`, `cross_product`, `norm`, `Matrix::ErrDimensionMismatch` and kin; Ruby's are immutable but for `[]=`, which is a mutator | 94 spec files; pure |
| `bigdecimal` (`bigdecimal/util`) | bigdecimal 3.1.8 | `BigDecimal` as a new Numeric kind of value: `BigDecimal("1.23")`, arithmetic with Integer and Float coercion, `round`, `floor`, `ceil`, `truncate` with Ruby's modes, `to_s` in Ruby's forms (`"0.123e1"`, `to_s("F")`), `precision`, `scale`, `sqrt`, `to_d` on String, Integer and Float, `BigDecimal.limit`, `mode`; on `math/big` | money; 56 spec files; the largest, so last of the pure ones |
| `resolv` | resolv 0.6.0 | `Resolv.getaddress`, `getaddresses`, `getname`, `Resolv::DNS.new.getresources` for A, AAAA, MX, TXT, CNAME, `Resolv::ResolvError`; through the host's resolver, as `net/http` goes through its client | after `net/http`; the host decides whether a script may look names up |
| `yaml` | psych 5.2.2 | `YAML.load`, `safe_load`, `load_stream`, `dump`, `to_yaml`, `YAML.load_file` **unsupported** (a file); plain data only: scalars, arrays, hashes, symbols (`:sym`), times and dates; a `!ruby/object` tag is a `Psych::DisallowedClass`; an instance of a cameo class dumps as a mapping of its fields, as JSON does | last, and not before a Go package writes what Psych writes: Go's standard library has no YAML, gopkg.in/yaml.v3 is archived, and its successor go.yaml.in/yaml/v3 indents, quotes, folds and reports errors in another shape than libyaml's. A library is one Go package on Go's packages, not a parser carried along |

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
`OpenSSL::Digest` and `OpenSSL::HMAC`, which `digest` will carry under
their OpenSSL names too. Certificates, keys, ciphers and `OpenSSL::SSL`
stay out.

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
- time zones: available.
