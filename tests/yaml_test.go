package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected strings are Ruby 3.4's, with psych 5.2.2 on libyaml 0.2.5.

func TestYAMLLoadsScalarsAsPsychDoes(t *testing.T) {
	out := runScript(t, `require "yaml"
p YAML.load("- 1\n- -1\n- 1_000\n- 0x1f\n- 0o17\n- 017\n- 0b101\n- 08\n- 1.5\n- 1e3\n- .5\n- 1.\n- -.inf\n- .NaN\n- ~\n- null\n- ''\n- yes\n- Off\n- n\n- :a\n- :\"a b\"\n- '1'\n- !!str 1\n- !!int '1'\n- !!float 1\n- 1:30\n- 1:30.5\n- 1,000\n- 1.2.3\n- 1e+3\n- 190:20:30\n- 1:60")
begin; YAML.load("a: b: c"); rescue YAML::SyntaxError => e; p e; end
begin; YAML.load("x: 2020-01-01"); rescue YAML::DisallowedClass => e; p e; end
p YAML.load("2020-13-01"), YAML.load("1-1-1"), YAML.load("!!binary aGk="), YAML.load("!foo 1"), YAML.load("!!bool x")
p YAML.load("x: 1\n<<: {y: 2}\n<<: [{z: 3}, {w: 4}]"), YAML.load("<<: 1\na: 2"), YAML.load("'<<': {a: 1}")
p YAML.load("a: {b: 1, c: [{d: 2}]}", symbolize_names: true), YAML.load("1: a\n'1': b\n1.0: c")
p YAML.load("x: &a [1]\ny: *a", aliases: true), YAML.load(""), YAML.load("", fallback: 7), YAML.load("---\n"), YAML.load("# c", fallback: 7)
p YAML.unsafe_load(""), YAML.load_stream("--- 1\n--- a\n---\nb: 2\n"), YAML.load_stream(""), YAML.load_stream("--- 1\n---\n")
p YAML.load("2020-01-31", permitted_classes: [Date]), YAML.load("2020-01-31 10:00:00 Z", permitted_classes: [Time]), YAML.load("2001-12-14t21:59:43.10-05:00", permitted_classes: ["Time"])
p YAML.safe_load("- :a\n- :b", permitted_classes: [Symbol], permitted_symbols: [:a, :b]), YAML.load("!ruby/range 1...3", permitted_classes: [Range]), YAML.load("!ruby/regexp /a.b/mix", permitted_classes: [Regexp])
p YAML.load("!ruby/object:Set\nhash:\n  1: true\n  2: true\n", permitted_classes: [Set]), YAML.load("!ruby/exception:RuntimeError\nmessage: boom\n", permitted_classes: [RuntimeError])
p YAML.load("a: 1", strict_integer: true), YAML.load("1,000", strict_integer: true), YAML.load("\xEF\xBB\xBFa: 1\r\nb: 2\r\n"), YAML.load("a: \"\\x41\\u263A\\e\\N\\_\\/\\t\"")
`)
	assert.Equals(t, out, `[1, -1, 1000, 31, "0o17", 15, 5, "08", 1.5, "1e3", 0.5, 1.0, -Infinity, NaN, nil, nil, "", true, false, "n", :a, :"a b", "1", "1", 1, 1.0, 5400, 5430.0, 1000, "1.2.3", "1e+3", 685230, "1:60"]
#<YAML::SyntaxError: (<unknown>): mapping values are not allowed in this context at line 1 column 5>
#<YAML::DisallowedClass: Tried to load unspecified class: Date>
"2020-13-01"
"1-1-1"
"hi"
1
"x"
{"x" => 1, "y" => 2, "w" => 4, "z" => 3}
{"<<" => 1, "a" => 2}
{"a" => 1}
{a: {b: 1, c: [{d: 2}]}}
{1 => "a", "1" => "b", 1.0 => "c"}
{"x" => [1], "y" => [1]}
nil
7
nil
7
false
[1, "a", {"b" => 2}]
[]
[1, nil]
#<Date: 2020-01-31 ((2458880j,0s,0n),+0s,-Infj)>
2020-01-31 10:00:00 UTC
2001-12-14 21:59:43.1 -0500
[:a, :b]
1...3
/a.b/mix
#<Set: {1, 2}>
#<RuntimeError: boom>
{"a" => 1}
"1,000"
{"a" => 1, "b" => 2}
{"a" => "A☺\e\u0085 /\t"}
`)
}

func TestYAMLRefusesWhatPsychRefuses(t *testing.T) {
	out := runScript(t, `require "yaml"
["a: [", "a: b: c", "\tx", "- a\nb: 1", "'a", "a: @b", "%YAML 1.3\n--- a", "...", "a: 1\n b: 2"].each do |doc|
  YAML.load(doc, filename: "f.yml")
rescue YAML::SyntaxError => e
  p [e.message, e.file, e.line, e.column, e.problem, e.context]
end
[":a", "2020-01-01", "2020-01-01 10:00:00", "!ruby/object:Pt\nx: 1\n", "!ruby/range 1..3", "!!set\n? a", "!ruby/class 'String'"].each do |doc|
  YAML.safe_load(doc)
rescue YAML::DisallowedClass => e
  puts e.message
end
begin; YAML.load("a: &x 1\nb: *x"); rescue => e; p e.class, e.message; end
begin; YAML.unsafe_load("b: *x"); rescue => e; p e.class, e.message; end
begin; YAML.load("a: 1", nope: 1); rescue ArgumentError => e; p e.message; end
begin; YAML.load(nil); rescue TypeError => e; p e.message; end
begin; YAML.safe_load(":a", permitted_classes: [Symbol], permitted_symbols: [:b]); rescue => e; p e.message; end
begin; YAML.load("!ruby/object:Nope\nx: 1\n", permitted_classes: ["Nope"]); rescue ArgumentError => e; p e.message; end
begin; YAML.load("!!float x"); rescue ArgumentError => e; p e.message; end
p YAML::SyntaxError.ancestors.take(4), YAML::AliasesNotEnabled.ancestors.take(4), YAML::VERSION, YAML::LIBYAML_VERSION
`)
	assert.Equals(t, out, `["(f.yml): did not find expected node content while parsing a flow node at line 2 column 1", "f.yml", 2, 1, "did not find expected node content", "while parsing a flow node"]
["(f.yml): mapping values are not allowed in this context at line 1 column 5", "f.yml", 1, 5, "mapping values are not allowed in this context", nil]
["(f.yml): found character that cannot start any token while scanning for the next token at line 1 column 1", "f.yml", 1, 1, "found character that cannot start any token", "while scanning for the next token"]
["(f.yml): did not find expected '-' indicator while parsing a block collection at line 1 column 1", "f.yml", 1, 1, "did not find expected '-' indicator", "while parsing a block collection"]
["(f.yml): found unexpected end of stream while scanning a quoted scalar at line 1 column 1", "f.yml", 1, 1, "found unexpected end of stream", "while scanning a quoted scalar"]
["(f.yml): found character that cannot start any token while scanning for the next token at line 1 column 4", "f.yml", 1, 4, "found character that cannot start any token", "while scanning for the next token"]
["(f.yml): found incompatible YAML document at line 1 column 1", "f.yml", 1, 1, "found incompatible YAML document", nil]
["(f.yml): did not find expected node content while parsing a block node at line 1 column 1", "f.yml", 1, 1, "did not find expected node content", "while parsing a block node"]
["(f.yml): mapping values are not allowed in this context at line 2 column 3", "f.yml", 2, 3, "mapping values are not allowed in this context", nil]
Tried to load unspecified class: Symbol
Tried to load unspecified class: Date
Tried to load unspecified class: Time
Tried to load unspecified class: Pt
Tried to load unspecified class: Range
Tried to load unspecified class: Set
Tried to load unspecified class: String
YAML::AliasesNotEnabled
"Alias parsing was not enabled. To enable it, pass `+"`aliases: true`"+` to `+"`YAML::load`"+` or `+"`YAML::safe_load`"+`."
YAML::AnchorNotDefined
"An alias referenced an unknown anchor: x"
"unknown keyword: :nope"
"no implicit conversion of nil into String"
"Tried to load unspecified class: Symbol"
"undefined class/module Nope"
"invalid value for Float(): \"x\""
[YAML::SyntaxError, YAML::Exception, RuntimeError, StandardError]
[YAML::AliasesNotEnabled, YAML::BadAlias, YAML::Exception, RuntimeError]
"5.2.2"
"0.2.5"
`)
}

func TestYAMLDumpsWhatPsychDumps(t *testing.T) {
	out := runScript(t, `require "yaml"
class Pt
  int :x
  ref :y, Pt
  def initialize(x, y)
    self.x = x
    self.y = y
  end
end
p YAML.dump({a: [1, "x"], "b" => {c: nil}, "s" => "yes", "t" => Time.utc(2020, 1, 2, 3, 4, 5), "f" => 1.5, "n" => nil, "ml" => "a\nb\n"})
p YAML.dump([[1, 2], [], {}, [nil, true, 1.0 / 0, -1.0 / 0, 0.0 / 0, 10**20, 1e-7, 1e20, 0.1 + 0.2]])
p ["", " lead", "trail ", "1", "true", "~", ":sym", "a: b", "a #b", "#c", "- x", "@x", "a\tb", "日本", "y", "N", "<<", "0755", "08", "1_000", ".inf", "1:30", "2020-01-01", "a\nb", "a\n\n", "  indented\nx", "\e", "a\\b", "true\n", "\xff".b, "hi".b, "x" * 90, "word " * 20].map { |s| YAML.dump(s) }
p YAML.dump({"k" => "a\n\tb\nc", "l" => "\na\nb", "m" => "a\n b\nc", "n" => "x\n\n\n"})
p YAML.dump([:a, :"a b", :"", :"a\nb"]), YAML.dump({1 => "i", nil => "n", [1, 2] => "arr", {a: 1} => "h", "" => "e"})
p YAML.dump([1..3, "a"...("b"), /a.b/i, Set[1, 2], Date.new(2020, 1, 31), DateTime.new(2020, 1, 2, 3, 4, 5), DateTime.new(2020, 1, 2, 3, 4, 5, "+02:00"), Time.new(2020, 1, 2, 3, 4, 5, "-05:30"), Time.at(0).utc, String, Encoding::UTF_8, RuntimeError.new("boom")])
p YAML.dump(Pt.new(1, Pt.new(3, nil))), YAML.dump({"pt" => Pt.new(5, nil)})
p [1, nil, "", [], {}, [nil], {"a" => nil}, [[]], {"a" => {}}, [""]].map { |v| YAML.dump(v) }
p YAML.dump({"long" => "word " * 20}, line_width: -1), YAML.dump({"long" => "word " * 20}, line_width: 20), YAML.dump({"a" => {"b" => [1, {"c" => 2}]}}, indentation: 4), YAML.dump({"a" => [1]}, indentation: 1)
p YAML.dump("a " * 10, line_width: 5), YAML.dump("a" * 100, line_width: 0), YAML.dump([1, "a"], header: true), YAML.dump(1, nope: 1)
p YAML.dump_stream(1, "x\ny\n\n", {"b" => 2}), YAML.dump("x\ny\n\n"), [1, "a", nil, {"k" => :v}].map(&:to_yaml)
p YAML.safe_dump({"a" => [1, "x", nil, true, 1.5]}), YAML.safe_dump({"a" => :b}, permitted_symbols: [:b]), YAML.safe_dump(Date.new(2020, 1, 1), permitted_classes: [Date])
[[{a: 1}, {}], [Time.utc(2020), {}], [Pt.new(1, nil), {}], [1..2, {}], [{"a" => :b}, {permitted_symbols: ["b"]}]].each do |v, o|
  YAML.safe_dump(v, **o)
rescue YAML::DisallowedClass => e
  puts e.message
end
begin; YAML.dump(1, line_width: -2); rescue ArgumentError => e; puts e.message; end
`)
	assert.Equals(t, out, `"---\n:a:\n- 1\n- x\nb:\n  :c:\ns: 'yes'\nt: 2020-01-02 03:04:05.000000000 Z\nf: 1.5\n\"n\":\nml: |\n  a\n  b\n"
"---\n- - 1\n  - 2\n- []\n- {}\n- -\n  - true\n  - .inf\n  - -.inf\n  - .nan\n  - 100000000000000000000\n  - 1.0e-07\n  - 1.0e+20\n  - 0.30000000000000004\n"
["--- ''\n", "--- \" lead\"\n", "--- 'trail '\n", "--- '1'\n", "--- 'true'\n", "--- \"~\"\n", "--- \":sym\"\n", "--- 'a: b'\n", "--- 'a #b'\n", "--- \"#c\"\n", "--- \"- x\"\n", "--- \"@x\"\n", "--- \"a\\tb\"\n", "--- 日本\n", "--- \"y\"\n", "--- \"N\"\n", "--- !!str '<<'\n", "--- '0755'\n", "--- '08'\n", "--- '1_000'\n", "--- \".inf\"\n", "--- '1:30'\n", "--- '2020-01-01'\n", "--- |-\n  a\n  b\n", "--- \"a\\n\\n\"\n", "--- |2-\n    indented\n  x\n", "--- \"\\e\"\n", "--- a\\b\n", "--- 'true\n\n  '\n", "--- !binary |-\n  /w==\n", "--- hi\n", "--- xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n", "--- 'word word word word word word word word word word word word word word word word\n  word word word word '\n"]
"---\nk: \"a\\n\\tb\\nc\"\nl: |2-\n\n  a\n  b\nm: |-\n  a\n   b\n  c\n\"n\": |+\n  x\n\n\n...\n"
"---\n- :a\n- :a b\n- !ruby/symbol\n- ! ':a\n\n  b'\n"
"---\n1: i\n! '': \"n\"\n? - 1\n  - 2\n: arr\n? :a: 1\n: h\n'': e\n"
"---\n- !ruby/range\n  begin: 1\n  end: 3\n  excl: false\n- !ruby/range\n  begin: a\n  end: b\n  excl: true\n- !ruby/regexp /a.b/i\n- !ruby/object:Set\n  hash:\n    1: true\n    2: true\n- 2020-01-31\n- !ruby/object:DateTime 2020-01-02 03:04:05.000000000 Z\n- !ruby/object:DateTime 2020-01-02 03:04:05.000000000 +02:00\n- 2020-01-02 03:04:05.000000000 -05:30\n- 1970-01-01 00:00:00.000000000 Z\n- !ruby/class 'String'\n- !ruby/encoding UTF-8\n- !ruby/exception:RuntimeError\n  message: boom\n  backtrace:\n"
"--- !ruby/object:Pt\nx: 1\ny: !ruby/object:Pt\n  x: 3\n  y:\n"
"---\npt: !ruby/object:Pt\n  x: 5\n  y:\n"
["--- 1\n", "---\n", "--- ''\n", "--- []\n", "--- {}\n", "---\n-\n", "---\na:\n", "---\n- []\n", "---\na: {}\n", "---\n- ''\n"]
"---\nlong: 'word word word word word word word word word word word word word word word word word word word word '\n"
"---\nlong: \"word word word\n  word word word word\n  word word word word\n  word word word word\n  word word word word\n  word \"\n"
"---\na:\n    b:\n    - 1\n    -   c: 2\n"
"---\na:\n- 1\n"
"--- \"a\n  a a a\n  a a a\n  a a a \"\n"
"--- >-\n  aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n"
"%YAML 1.1\n---\n- 1\n- a\n"
"--- 1\n"
"--- 1\n--- |+\n  x\n  y\n\n---\nb: 2\n"
"--- |+\n  x\n  y\n\n...\n"
["--- 1\n", "--- a\n", "---\n", "---\nk: :v\n"]
"---\na:\n- 1\n- x\n-\n- true\n- 1.5\n"
"---\na: :b\n"
"--- 2020-01-01\n"
Tried to dump unspecified class: Symbol(:a)
Tried to dump unspecified class: Time
Tried to dump unspecified class: Pt
Tried to dump unspecified class: Range
Tried to dump unspecified class: Symbol(:b)
Invalid line_width -2, must be non-negative or -1 for unlimited.
`)
}
