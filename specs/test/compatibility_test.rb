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

  # written returns COMPATIBILITY.md and the site's page that
  # compatibility.rb writes from ROWS.
  def written
    Dir.mktmpdir("compatibility-test") do |dir|
      status = File.join(dir, "status.tsv")
      File.write(status, ROWS.map { _1.join("\t") }.join("\n") + "\n")
      out = File.join(dir, "COMPATIBILITY.md")
      html = File.join(dir, "compatibility.html")
      log, st = Open3.capture2e("ruby", SCRIPT, "--status", status, "--out", out, "--html", html,
                                "--commit", "abc", "--date", "2026-01-02")
      assert st.success?, "compatibility.rb failed:\n#{log}"
      [File.read(out, encoding: "UTF-8"), File.read(html, encoding: "UTF-8")]
    end
  end

  def compatibility = written.first
  def page = written.last

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

  # The site's page is the site's index.html around the same content.
  def test_writes_the_sites_page_in_the_sites_frame
    html = page
    assert_includes html, "<title>cameo — compatibility</title>"
    assert_includes html, %(<a href="compatibility.html" aria-current="page">Compatibility</a>)
    assert_equal 1, html.scan('aria-current="page"').size
    assert_includes html, %(<script type="module" src="site.js"></script>)
    assert_includes html, %(<h1 id="compatibility">Compatibility</h1>\n\n<p class="lead">How much of)
    assert_includes html, %(<footer class="site">)
    refute_includes html, "<!--"
  end

  def test_the_sites_page_has_the_same_tables
    html = page
    assert_includes html, %(<tr><td><a href="#core">core</a></td><td class="num">4</td><td class="num">3</td>) +
                          %(<td class="num">1 (33%)</td>)
    assert_includes html, %(<tr><td><strong>all</strong></td><td class="num">5</td>)
    assert_includes html, %(<td>core/string</td><td class="num">1</td><td class="num">0</td><td><code>Expected 1 == 2</code></td>)
    assert_includes html, %(<h3 id="core">core</h3>)
  end

  # A code links its page on the site: from COMPATIBILITY.md by its
  # address, from the site's own page relative to it.
  def test_links_each_code_to_its_page
    md, html = written
    assert_includes md, "| [mut](https://cameo-lang.github.io/cameo/compatibility/mut.html) | [mutation]("
    assert_includes html, %(<tr><td><a href="compatibility/mut.html">mut</a></td>)
  end

  # A file of the repository is a link to it on GitHub; an anchor stays.
  def test_the_sites_page_links_files_on_github
    html = page
    assert_includes html, %(<a href="https://github.com/cameo-lang/cameo/blob/main/specs/run.rb">specs/run.rb</a>)
    assert_includes html, %(<a href="https://github.com/cameo-lang/cameo/blob/main/doc/cameo-vs-ruby.md#different-in-cameo">mutation</a>)
    assert_includes html, %(<a href="#not-expected-to-work">reasons</a>)
  end
end
