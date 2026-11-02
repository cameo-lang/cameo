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
| **later** | nothing speaks against it, and nothing asks for it yet |
| **unsupported** | left out, for the reason given; a `require` raises `LoadError` |

Ruby 3.4.1 ships 58 default gems and 31 bundled gems. Every one is in a
table below. "Ruby" below is CRuby 3.4.1; the version named for a library
is the one it ships.

## What decides

A library goes in when its work is computation on values: parsing,
formatting, arithmetic, encoding. It stays out when what it does is one of
the things cameo leaves out on purpose
([cameo-vs-ruby.md](cameo-vs-ruby.md)): the file system and processes,
threads, metaprogramming, object identity, mutable state inside a value. A
library whose purpose is a mutable cursor or buffer (`StringIO`,
`StringScanner`) could be a handle, as `Random` is: the one kind of value
whose calls change something. None is one yet; those are *later*, with the
question open.

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
  has nothing: a YAML parser for `yaml`, `golang.org/x/text` for
  `unicode_normalize`.

## Available

| Library | Ruby's version | What is in |
|---|---|---|
| `set` | set 1.1.1 | `Set`, as in Ruby 3.4 without a `require`; the mutators raise as every mutator does |
| `time` | time 0.4.1 | `Time.parse`, `strptime`, `iso8601`/`xmlschema`, `Time#iso8601`; `Time.new` reads a string; `rfc2822`, `httpdate` and their writers are **planned** with `date` |
| `pp` | pp 0.6.2 | `pp` prints as `p` does: `inspect`, one value per line; no `pretty_inspect` and no line breaking at 80 columns |
| `pathname` | pathname 0.4.0 | path arithmetic without I/O: `join`, `+`, `basename`, `dirname`, `extname`, `cleanpath`, `parent`, `sub_ext`, `relative_path_from`, `each_filename`, `<=>`; `File.join`, `basename`, `dirname`, `extname`, `expand_path`; everything that reads the file system is **unsupported** |
| `cgi` (`cgi/escape`, `cgi/util`) | cgi 0.4.1 | the escapes, `escapeElement`, `rfc1123_date`, `pretty`; `CGI.new`, cookies and sessions read a request and are **unsupported** |
| `json` | json 2.9.1 | `parse`, `parse!`, `load`, `generate`, `pretty_generate`, `dump`, `JSON()`, `to_json`, the error classes; `json/add/*` is **unsupported** (it is Marshal in JSON) |
| `benchmark` | benchmark 0.4.0 | `measure`, `realtime`, `bm`, `bmbm`, `benchmark`, `Tms`; `Tms#add!` is a mutator |
| `debug` | debug 1.10.0 | `debugger` and `binding.break` stop in cameo's debugger, which takes rdbg's commands ([debugger.md](debugger.md)) |
| `cameo/runner` | — | cameo's own: the library that gives CRuby `Cameo::Runner` is built in |

## Planned, in this order

Each is one pull request, which also moves its row to *Available*.

| Library | Ruby's version | What goes in | Why this order |
|---|---|---|---|
| `uri` | uri 1.0.2 | `URI.parse`, `join`, `split`, `URI()`, `URI::Generic` and `HTTP`, `HTTPS`, `FTP`, `File`, `MailTo`, `WS`, `WSS` with their components, `to_s`, `==`, `merge`, `+`, `route_from`, `route_to`, `normalize`, `select`, `query` and `fragment` writers as copies (`with` and the `x=` forms both return the copy), `encode_www_form`, `decode_www_form`, the `_component` pair, `URI.extract`, `URI::InvalidURIError` and the RFC 3986 parser's messages | `net/http` takes a URI; 103 spec files |
| `date` | date 3.4.1 | `Date` and `DateTime`: `new`/`civil`, `jd`, `ordinal`, `commercial`, `today`, `now`, `parse`, `iso8601`, `rfc3339`, `rfc2822`, `httpdate`, `strptime`, `_parse`, `valid_date?`, `leap?`, `strftime`, `to_s`, `inspect`, `iso8601` and the other writers, `+`, `-`, `>>`, `<<`, `next_day`, `prev_month`, `next_year` and kin, `step`, `upto`, `downto`, a Range of dates, `<=>`, `===`, `cwday`, `cweek`, `cwyear`, `yday`, `mjd`, `ajd`, `day_fraction`, `sunday?`…`saturday?`, `MONTHNAMES`, `DAYNAMES` and the abbreviations, `Date::Error`, `Time#to_date`, `to_datetime`, `Date#to_time`; `Time#rfc2822` and `httpdate` of `time` | 146 spec files; the second thing a data script needs after JSON |
| `net/http` (`net/https`, `net/protocol`) | net-http 0.6.0 | `Net::HTTP.get`, `get_response`, `post`, `post_form`, `start`, `new` with `use_ssl`, `open_timeout`, `read_timeout`, `request`, the request classes (`Get`, `Post`, `Put`, `Patch`, `Delete`, `Head`, `Options`) with headers, `body`, `set_form_data`, `basic_auth`, `Net::HTTPResponse` with `code`, `message`, `body`, `[]`, `each_header`, `value`, `content_type`, and the response classes (`HTTPSuccess`, `HTTPNotFound`, …) for `case`; `Net::HTTPError` and the `Net::HTTPExceptions` hierarchy, `Net::OpenTimeout`, `ReadTimeout`, `SocketError` | the I/O a data script needs; 131 spec files |
| time zones | — | an IANA zone name wherever Ruby takes a UTC offset: `Time.new(…, in: "Europe/Berlin")`, `Time.now(in:)`, `Time.at(…, in:)`, `localtime`, `getlocal`; `Time#zone` is the abbreviation in force ("CET"); `TZ` in the process's environment names the local zone, as Go reads it. Go's `time/tzdata` is compiled in, so a zone resolves on a machine without a zoneinfo directory. Ruby takes a zone *object* here, so this is cameo's extension; `date` gets the same `in:` | "complete date and timezone"; Ruby has no zone database of its own |
| `yaml` (`psych`) | psych 5.2.2 | `YAML.load`, `safe_load`, `load_stream`, `dump`, `to_yaml`, `YAML.load_file` **unsupported** (a file); plain data only: scalars, arrays, hashes, symbols (`:sym`), times and dates; a `!ruby/object` tag is a `Psych::DisallowedClass`; an instance of a cameo class dumps as a mapping of its fields, as JSON does | configuration and fixtures; the LoadError sample on the site today |
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

## Later

Nothing speaks against these; nothing asks for them yet. One becomes
*planned* when a script needs it.

| Library | Ruby's version | Note |
|---|---|---|
| `prime` | prime 0.1.3 | `Prime.each`, `prime?`, `prime_division`, `Integer#prime?`; pure |
| `matrix` | matrix 0.4.2 | `Matrix` and `Vector`; pure, 94 spec files, and no script of cameo's has asked |
| `bigdecimal` | bigdecimal 3.1.8 | a decimal Numeric of any precision: a new kind of value with Ruby's rounding modes, as large as `Integer` of any size was; pure, and worth doing once a script computes money |
| `tsort` | tsort 0.2.0 | `TSort.tsort`, `strongly_connected_components` as module functions on a Hash of children |
| `singleton` | singleton 0.3.0 | `include Singleton`; `instance` returns the one value, which, immutable, is what a Singleton is for |
| `forwardable`, `delegate` | forwardable 1.3.3, delegate 0.4.0 | `def_delegators :field, :size, :each` and `SimpleDelegator` on a field; a class-body call that defines methods, which the checker has to learn as it learnt bang methods |
| `logger` | logger 1.6.4 | `Logger.new($stderr)` with levels, `formatter`, `progname`; a logger is a handle on a stream, and `level=` changes it in place |
| `stringio` | stringio 3.1.2 | `StringIO` as a handle like `Random`: `read`, `gets`, `each_line`, `write`, `puts`, `string`, `rewind`; the question is whether a handle is right for it or a cameo script should pass Strings |
| `strscan` | strscan 3.1.2 | `StringScanner` as a handle: `scan`, `scan_until`, `skip`, `check`, `matched`, `pos`, `eos?`; the same question |
| `optparse`, `getoptlong` | optparse 0.6.0, getoptlong 0.2.1 | cameo gives a script no `ARGV` today; when `cameo script.cameo -- args` passes one, `OptionParser` follows |
| `rexml` | rexml 3.4.0 | `REXML::Document` from a String, `elements`, `attributes`, `text`, `XPath.match` for the common paths, and writing; on Go's `encoding/xml` |
| `erb` | erb 4.0.4 | `ERB.new(template).result_with_hash(h)` alone; a template's Ruby runs through cameo's own evaluator, not `eval`; `result(binding)` is **unsupported** |
| `rbconfig` | — | `RbConfig::CONFIG["host_os"]`, `"arch"`, `"ruby_version"`, for scripts that look before they leap; `RUBY_PLATFORM` covers most of it |
| `abbrev` | abbrev 0.1.2 | `Abbrev.abbrev`; pure, tiny |
| `resolv` | resolv 0.6.0 | `Resolv.getaddress`, `getname`, `Resolv::DNS` through the host's resolver, as `net/http` goes through its client |
| `timeout` | timeout 0.4.3 | `Timeout.timeout(s) { }` would need to stop a running block, which only the host's deadline does; a cameo form could `await` with a limit |

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
0.3.0, `timeout` 0.4.3 (see *Later*), `drb` 2.2.1, `rinda` 0.2.0,
`continuation`.

**Metaprogramming, `eval` and `binding`.** cameo refuses `eval`,
`define_method`, `instance_variable_set` and `binding` before the script
runs. `erb` 4.0.4's `result(binding)` (see *Later* for the form that
works), `ostruct` 0.6.1 (an OpenStruct grows attributes at run time; a
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
`net/http` does, and one becomes *later* when a script needs it.

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

- this page, the proposal.
