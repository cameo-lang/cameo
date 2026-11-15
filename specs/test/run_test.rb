# frozen_string_literal: true

# Tests of run.rb: each writes a spec into a directory of its own, runs
# run.rb on it with the cameo binary named by CAMEO_BIN, and reads the
# status rows it writes.
#
#   CAMEO_BIN=bin/cameo ruby specs/test/run_test.rb

require "fileutils"
require "minitest/autorun"
require "open3"
require "tmpdir"

class RunTest < Minitest::Test
  RUN = File.expand_path("../run.rb", __dir__)
  CAMEO = File.expand_path(ENV.fetch("CAMEO_BIN", File.join(__dir__, "..", "..", "bin", "cameo")))

  # status runs run.rb on a spec file with the source src, in the
  # environment env, and returns the rows of its --status file, each as
  # [example, status, reason, message].
  def status(src, env = {})
    Dir.mktmpdir("run-test") do |dir|
      FileUtils.mkdir_p(File.join(dir, "core"))
      spec = File.join(dir, "core", "t_spec.rb")
      File.write(spec, src)
      out = File.join(dir, "status.tsv")
      log, st = Open3.capture2e(env, "ruby", RUN, "--spec", dir, "--cameo", CAMEO, "--status", out, spec)
      assert st.success?, "run.rb failed:\n#{log}"
      rows = File.read(out, encoding: "UTF-8").lines.drop(1).map { _1.chomp.split("\t", -1).drop(1) }
      refute_empty rows, "run.rb wrote no status rows:\n#{log}"
      rows
    end
  end

  def test_reads_the_checker_in_the_posix_locale
    # The checker's message for an instance variable has an ellipsis,
    # which is not US-ASCII.
    rows = status(<<~RUBY, "LC_ALL" => "C", "LANG" => "C")
      describe "a" do
        @x = 1
      end
    RUBY
    assert_equal [["*", "expected", "instance-variables"]], rows.map { _1.first(3) }
    assert_includes rows.first[3], "…"
  end

  def test_rewrites_a_receiver_that_ends_in_a_block_argument
    # Prism 0.19 ends twice(0, &three) before its closing parenthesis.
    rows = status(<<~RUBY)
      def twice(x, &block)
        block.call(x) * 2
      end

      describe "a" do
        it "b" do
          three = ->(x) { x + 3 }
          twice(0, &three).should == 6
          twice(1, &three).should_not.zero?
        end
      end
    RUBY
    assert_equal [["a b", "pass", ""]], rows.map { _1.first(3) }
  end
end
