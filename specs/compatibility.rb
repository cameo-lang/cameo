#!/usr/bin/env ruby
# frozen_string_literal: true

# Writes COMPATIBILITY.md, how much of ruby/spec cameo passes, from
# status.tsv, which run.rb writes. It runs no spec itself.
#
#   ruby compatibility.rb [--status FILE] [--out FILE] [--ruby RUBY]
#                         [--commit SHA] [--date YYYY-MM-DD]
#
# The defaults are specs/status.tsv, COMPATIBILITY.md at the root of the
# repository, the Ruby named ruby, the ruby/spec commit COUNTS.md names and
# today: make spec-status runs run.rb and then this script, so the date is
# that of the run and the Ruby the one run.rb compared with.
#
# A category is a spec's directory two levels deep (core/string,
# library/csv), or the top-level directory for a spec that sits in it
# (language, command_line).

require "date"
require "open3"
require "optparse"

HERE = __dir__
ROOT = File.expand_path("..", HERE)

options = { status: File.join(HERE, "status.tsv"), out: File.join(ROOT, "COMPATIBILITY.md"), ruby: "ruby",
            date: Date.today.iso8601 }
OptionParser.new do |o|
  o.banner = "usage: ruby compatibility.rb [options]"
  o.on("--status FILE", "the status of every example, as run.rb writes it (default: specs/status.tsv)") { options[:status] = _1 }
  o.on("--out FILE", "where to write (default: COMPATIBILITY.md at the root)") { options[:out] = _1 }
  o.on("--ruby RUBY", "the Ruby run.rb compared with (default: ruby)") { options[:ruby] = _1 }
  o.on("--commit SHA", "the ruby/spec commit (default: the one COUNTS.md names)") { options[:commit] = _1 }
  o.on("--date DATE", "the date of the run (default: today)") { options[:date] = _1 }
end.parse!

options[:commit] ||= File.read(File.join(HERE, "COUNTS.md"))[/commit `(\h{40})`/, 1] or abort "compatibility.rb: COUNTS.md names no commit"
ruby_version, ruby_status = Open3.capture2(options[:ruby], "-e", 'print "#{RUBY_VERSION} (#{RUBY_PLATFORM})"')
abort "compatibility.rb: #{options[:ruby]} did not run" unless ruby_status.success?

VS = "doc/cameo-vs-ruby.md"
DIFFERENT = "#{VS}#different-in-cameo"
REJECTED = "#{VS}#rejected-before-the-script-runs"
MISSING = "#{VS}#missing-builtins"
STDLIB = "doc/stdlib.md"

# The reasons runner/classify.rb gives what cameo leaves out on purpose:
# the code the tables use, a name, what it means, and the document that
# states the rule.
REASONS = {
  "instance-variables" => ["ivar", "instance variables", "cameo has no instance variables; a class declares fields. Most are mspec's `before` blocks, which set `@object`", REJECTED],
  "metaprogramming" => ["meta", "metaprogramming", "`eval`, `instance_eval`, `define_method`, `instance_method`, `singleton_class`, `class << self`, `send(:initialize)`", REJECTED],
  "mutation" => ["mut", "mutation", "every value is immutable: `<<`, `[]=`, `push`, setters, bang methods and nested multiple assignment are refused or return a copy", DIFFERENT],
  "visibility" => ["vis", "visibility", "`private`, `protected`, `module_function` and the method lists by visibility: every method a script defines is public", REJECTED],
  "attr-accessor" => ["attr", "`attr_accessor`", "`attr_reader`, `attr_writer`, `attr_accessor`: a field declaration defines its reader, and there are no setters", REJECTED],
  "rational-complex" => ["rat", "`Rational`, `Complex`", "`Rational`, `Complex`, `1r`, `2i`, `to_r`, `to_c`", MISSING],
  "process" => ["proc", "the process", "`Process`, `Signal`, `ENV`, `GC`, `ObjectSpace`, `fork`, `system`, backticks", MISSING],
  "io" => ["io", "I/O", "`IO`, `File`, `Dir`, `ARGF` and the standard streams as objects, beyond writing to standard output and error", MISSING],
  "encodings" => ["enc", "encodings", "encodings other than UTF-8, binary and US-ASCII", DIFFERENT],
  "invalid-strings" => ["inv", "invalid strings", "a UTF-8 String holds characters alone: bytes that are not UTF-8 are a binary String, written `b\"...\"`, and what makes text refuses them", DIFFERENT],
  "binary-strings" => ["bin", "binary strings", "a binary String has bytes and no characters, and a UTF-8 String with characters beyond ASCII has no bytes alone (`getbyte`)", DIFFERENT],
  "threads" => ["thr", "threads", "`Thread`, `Mutex`, `Queue`, `Fiber`, `Ractor`: cameo has `async` and `await`", MISSING],
  "identity-and-freezing" => ["id", "identity, freezing", "`equal?`, `object_id`, `frozen?`, `FrozenError`: values are immutable and compared by what they hold", DIFFERENT],
  "literal-only" => ["lit", "no constructor", "`Range.new`, `Proc.new`, `Enumerator.new`, `Module.new`, `String.new`: cameo makes these values with a literal", MISSING],
  "block-captures-values" => ["blk", "blocks capture values", "a block captures the values its variables have, and cannot rebind a variable of the scope around it", REJECTED],
  "globals" => ["glob", "global variables", "`$x`, `$~`, `Regexp.last_match`", REJECTED],
  "class-variables" => ["cvar", "class variables", "`@@x`", REJECTED],
  "marshal" => ["mar", "`Marshal`", "`Marshal` writes cameo's wire form, not Ruby's bytes", STDLIB],
  "struct" => ["struct", "`Struct`", "a class declares typed fields instead", MISSING],
  "data" => ["data", "`Data`", "a class with typed fields is cameo's Data", MISSING],
  "regexp" => ["re", "regexps", "look-behind and back-references, which cameo's engine does not run", DIFFERENT],
  "scoped-constants" => ["scope", "scoped constants", "`A::B = 1`: a constant is assigned inside its module", REJECTED],
  "ruby2_keywords" => ["r2k", "`ruby2_keywords`", "`ruby2_keywords`", REJECTED],
  "numeric-subclass" => ["num", "Numeric subclasses", "a subclass of `Numeric`", DIFFERENT],
  "basic-object-subclass" => ["bobj", "BasicObject subclasses", "a subclass of `BasicObject`: `method_missing` in an ordinary class instead", REJECTED],
  "enumerator-next" => ["next", "external enumerators", "an Enumerator has no `next`, `peek` or `rewind`", DIFFERENT],
  "require" => ["req", "libraries", "a `require` of a library cameo does not build in, or `require_relative` and `load`", STDLIB],
  "library-left-out" => ["lib", "parts of libraries", "a part of a library cameo builds in that its row in stdlib.md leaves out: Zlib's streams, `Matrix#eigensystem`, `Date::Infinity`, OpenSSL beyond digests", STDLIB],
  "c-api" => ["capi", "the C API", "Ruby's C API: cameo builds its libraries in and loads no native code", STDLIB],
}.freeze

DIRECTORIES = %w[language core library command_line security optional].freeze

Row = Struct.new(:file, :example, :status, :reason, :stage, :message)

lines = File.read(options[:status], encoding: "UTF-8").lines.map(&:chomp)
header = lines.shift.split("\t")
abort "compatibility.rb: #{options[:status]} is not run.rb's status file" unless header == Row.members.map(&:to_s)
rows = lines.map { Row.new(*_1.split("\t", -1)) }

unknown = rows.select { _1.status == "expected" }.map(&:reason).uniq - REASONS.keys
warn "compatibility.rb: reasons without a description: #{unknown.join(", ")}" unless unknown.empty?

def category(file)
  parts = file.split("/")
  parts.size > 2 ? parts.first(2).join("/") : parts.first
end

def code(reason) = REASONS.dig(reason, 0) || reason

# Tally counts the examples of a part of ruby/spec by status, and those
# not expected to work by reason, and the files that did not run by the
# stage that stopped them and by why.
class Tally
  attr_reader :examples, :status, :reasons, :files, :why, :file_reasons, :file_bugs

  def initialize
    @examples = 0
    @status = Hash.new(0)
    @reasons = Hash.new(0)
    @files = 0
    @why = Hash.new { |h, k| h[k] = Hash.new(0) }
    @file_reasons = Hash.new(0)
    @file_bugs = 0
  end

  def add(row)
    if row.example == "*"
      @files += 1
      @why[row.stage][label(row)] += 1
      @file_reasons[row.reason] += 1 if row.status == "expected"
      @file_bugs += 1 if row.status == "bug"
    else
      @examples += 1
      @status[row.status] += 1
      @reasons[row.reason] += 1 if row.status == "expected"
    end
  end

  # label returns how a file that did not run shows in the table: the
  # code of its reason, or its status, bug or harness.
  def label(row) = row.status == "expected" ? code(row.reason) : row.status

  def should_match = examples - status["harness"]
  def expected = status["expected"]
end

def by_count(hash) = hash.sort_by { |k, n| [-n, k] }

def reason_cell(reasons) = by_count(reasons).map { |r, n| "#{code(r)} #{n}" }.join(", ")

def why_cell(why)
  %w[check crash timeout rewrite].filter_map do |stage|
    next unless why.key?(stage)
    "#{stage}: #{by_count(why[stage]).map { |l, n| "#{l} #{n}" }.join(", ")}"
  end.join("; ")
end

def percent(n, of) = of.zero? ? "–" : "#{(100.0 * n / of).round}%"

# code_span returns text as a Markdown code span that a table cell holds.
def code_span(text)
  text = text.gsub("|", "\\|")
  text.include?("`") ? "`` #{text} ``" : "`#{text}`"
end

by_dir = Hash.new { |h, k| h[k] = Tally.new }
by_cat = Hash.new { |h, k| h[k] = Tally.new }
total = Tally.new
rows.each do |row|
  by_dir[row.file.split("/").first].add(row)
  by_cat[category(row.file)].add(row)
  total.add(row)
end
dirs = DIRECTORIES.select { by_dir.key?(_1) } + (by_dir.keys - DIRECTORIES).sort

out = +""
out << <<~MD
  # Compatibility with ruby/spec

  <!-- Generated by specs/compatibility.rb from specs/status.tsv. Do not edit; run `make spec-status`. -->

  How much of [ruby/spec](https://github.com/ruby/spec), the specification
  of the Ruby language and its core and standard libraries, cameo passes,
  and why it does not pass the rest.

  - ruby/spec at commit `#{options[:commit]}`
  - CRuby #{ruby_version}, which runs every program as well
  - run on #{options[:date]}
  - regenerate with `make spec-status`, which needs a Ruby with Prism and
    the copy of ruby/spec that `make ruby-spec` makes: it runs
    [specs/run.rb](specs/run.rb), which writes
    [specs/status.tsv](specs/status.tsv), and then
    [specs/compatibility.rb](specs/compatibility.rb), which writes this page

  [specs/run.rb](specs/run.rb) rewrites each spec file's mspec idioms into
  calls of [a prelude](specs/runner/prelude.cameo) and runs it under cameo
  and under CRuby. Each example has one status:

  | Status | Means |
  |---|---|
  | pass | passes under cameo, as under CRuby |
  | not expected | stopped by something cameo leaves out on purpose; the [reasons](#not-expected-to-work) say what |
  | bug | CRuby passes it and cameo does not, for no reason cameo states: cameo should pass it |
  | harness | CRuby does not pass it in this harness either, or it needs `before`, `after` or a shared example, which the prelude does not run |

  Only an example CRuby passes in this harness counts as one cameo should
  match. CRuby's own failures, mostly examples that need a part of mspec
  the prelude lacks (`mock`, `should_receive`, `ruby_exe`), and the
  examples CRuby does not run, which ruby/spec guards for a newer Ruby
  than CRuby #{ruby_version.split.first}, count as harness.
  *Should match* below is pass, not expected and bug together.

  A spec file that did not run has no examples here. It counts once, under
  *files not run*, with what stopped it: `cameo check` refused it (check),
  it stopped before its first example (crash), it ran out of time
  (timeout), or Prism could not parse it for the rewrite (rewrite).
  [specs/runner/classify.rb](specs/runner/classify.rb) gives a
  status and a reason from the message that stopped an example or a file.

  ## Summary

  | Directory | Examples | Should match | Pass | Not expected | Bug | Harness | Files not run |
  |---|--:|--:|--:|--:|--:|--:|--:|
MD
(dirs.map { [_1, by_dir[_1]] } + [["all", total]]).each do |name, t|
  label = name == "all" ? "**all**" : "[#{name}](##{name})"
  out << "| #{label} | #{t.examples} | #{t.should_match} | #{t.status["pass"]} (#{percent(t.status["pass"], t.should_match)}) | " \
         "#{t.expected} | #{t.status["bug"]} | #{t.status["harness"]} | #{t.files} |\n"
end
harness = rows.select { _1.example != "*" && _1.status == "harness" }.map do |row|
  case row.reason
  when "fails under Ruby too", "fails under Ruby" then :fails
  when "not run under Ruby" then :not_run
  else :mspec
  end
end.tally
out << "\nOf the #{total.status["harness"]} harness examples, #{harness.fetch(:fails, 0)} fail under CRuby, " \
       "#{harness.fetch(:not_run, 0)} CRuby does not run, and #{harness.fetch(:mspec, 0)} need `before`, " \
       "`after` or a shared example.\n"
out << <<~MD

  ## Not expected to work

  What cameo leaves out on purpose, each with the document that states the
  rule. *Examples* counts examples that ran and stopped on it; *files*
  counts spec files that did not run because of it.

  | Code | Reason | Means | Examples | Files |
  |---|---|---|--:|--:|
MD
reasons = (total.reasons.keys | total.file_reasons.keys)
reasons.sort_by { |r| [-(total.reasons[r] + total.file_reasons[r]), r] }.each do |r|
  c, name, means, link = REASONS[r] || [r, r, "", nil]
  name = "[#{name}](#{link})" if link
  out << "| #{c} | #{name} | #{means} | #{total.reasons[r]} | #{total.file_reasons[r]} |\n"
end

out << <<~MD

  ## Where the bugs are

  The categories with the most bugs: examples CRuby passes and cameo does
  not, and files a bug stopped. [specs/status.tsv](specs/status.tsv) lists
  each, with its message.

  | Category | Bug examples | Bug files | The most common message |
  |---|--:|--:|---|
MD
bug_rows = rows.select { _1.status == "bug" }.group_by { category(_1.file) }
bug_rows.sort_by { |cat, rs| [-rs.size, cat] }.first(20).each do |cat, rs|
  t = by_cat[cat]
  message, = by_count(rs.map(&:message).tally).first
  message = message.length > 100 ? "#{message[0, 99]}…" : message
  out << "| #{cat} | #{t.status["bug"]} | #{t.file_bugs} | #{code_span(message)} |\n"
end

out << <<~MD

  ## By category

  One row per category. *Why* gives the count for each reason by its code
  from the [table above](#not-expected-to-work); for a file that did not
  run, bug or harness when the message was not one cameo states.
MD
dirs.each do |name|
  out << "\n### #{name}\n\n"
  out << "| Category | Examples | Pass | Not expected | Why | Bug | Harness | Files not run | Why not run |\n"
  out << "|---|--:|--:|--:|---|--:|--:|--:|---|\n"
  by_cat.keys.select { _1 == name || _1.start_with?("#{name}/") }.sort.each do |cat|
    t = by_cat[cat]
    out << "| #{cat} | #{t.examples} | #{t.status["pass"]} | #{t.expected} | #{reason_cell(t.reasons)} | " \
           "#{t.status["bug"]} | #{t.status["harness"]} | #{t.files} | #{why_cell(t.why)} |\n"
  end
end

File.write(options[:out], out)
