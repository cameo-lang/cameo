# frozen_string_literal: true

# Tests of compatibility.rb: it writes COMPATIBILITY.md from a status file
# made up here.
#
#   ruby specs/test/compatibility_test.rb

require "minitest/autorun"
require "open3"
require "tmpdir"

class CompatibilityTest < Minitest::Test
  SCRIPT = File.expand_path("../compatibility.rb", __dir__)

  ROWS = [
    %w[file example status reason stage message],
    ["core/string/a_spec.rb", "String#a works", "pass", "", "", ""],
    ["core/string/a_spec.rb", "String#a refuses", "expected", "mutation", "", "can't modify"],
    ["core/string/b/c_spec.rb", "String#c", "bug", "", "", "Expected 1 == 2"],
    ["core/string/d_spec.rb", "*", "expected", "instance-variables", "check", "@x = …"],
    ["core/array/a_spec.rb", "Array#a", "harness", "fails under Ruby too", "", "mock"],
    ["language/if_spec.rb", "if works", "pass", "", "", ""],
    ["library/csv/a_spec.rb", "*", "expected", "require", "crash", "cannot load such file -- x (LoadError)"],
  ].freeze

  def compatibility
    Dir.mktmpdir("compatibility-test") do |dir|
      status = File.join(dir, "status.tsv")
      File.write(status, ROWS.map { _1.join("\t") }.join("\n") + "\n")
      out = File.join(dir, "COMPATIBILITY.md")
      log, st = Open3.capture2e("ruby", SCRIPT, "--status", status, "--out", out, "--commit", "abc", "--date", "2026-01-02")
      assert st.success?, "compatibility.rb failed:\n#{log}"
      File.read(out, encoding: "UTF-8")
    end
  end

  def test_writes_the_header
    md = compatibility
    assert_includes md, "ruby/spec at commit `abc`"
    assert_includes md, "run on 2026-01-02"
    assert_includes md, "CRuby #{RUBY_VERSION}"
  end

  def test_sums_each_directory
    md = compatibility
    # examples, should match, pass, not expected, bug, harness, files not run
    assert_includes md, "| [core](#core) | 4 | 3 | 1 (33%) | 1 | 1 | 1 | 1 |"
    assert_includes md, "| [library](#library) | 0 | 0 | 0 (–) | 0 | 0 | 0 | 1 |"
    assert_includes md, "| **all** | 5 | 4 | 2 (50%) | 1 | 1 | 1 | 2 |"
  end

  def test_has_one_row_per_category
    md = compatibility
    assert_includes md, "| core/string | 3 | 1 | 1 | mut 1 | 1 | 0 | 1 | check: ivar 1 |"
    assert_includes md, "| core/array | 1 | 0 | 0 |  | 0 | 1 | 0 |  |"
    assert_includes md, "| language | 1 | 1 | 0 |  | 0 | 0 | 0 |  |"
    assert_includes md, "| library/csv | 0 | 0 | 0 |  | 0 | 0 | 1 | crash: req 1 |"
  end

  def test_lists_the_bugs_by_category
    assert_includes compatibility, "| core/string | 1 | 0 | `Expected 1 == 2` |"
  end
end
