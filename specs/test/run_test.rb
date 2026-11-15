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
  # environment env and with the arguments args, and returns the rows of
  # its --status file, each as a Hash by column, without the file.
  def status(src, env: {}, args: [])
    Dir.mktmpdir("run-test") do |dir|
      FileUtils.mkdir_p(File.join(dir, "core"))
      spec = File.join(dir, "core", "t_spec.rb")
      File.write(spec, src)
      out = File.join(dir, "status.tsv")
      log, st = Open3.capture2e(env, "ruby", RUN, "--spec", dir, "--cameo", CAMEO, "--status", out, *args, spec)
      assert st.success?, "run.rb failed:\n#{log}"
      header, *lines = File.read(out, encoding: "UTF-8").lines.map { _1.chomp.split("\t", -1) }
      rows = lines.map { header.zip(_1).to_h.tap { |r| r.delete("file") } }
      refute_empty rows, "run.rb wrote no status rows:\n#{log}"
      rows
    end
  end

  def test_reads_the_checker_in_the_posix_locale
    # The checker's message for an instance variable has an ellipsis,
    # which is not US-ASCII.
    rows = status(<<~RUBY, env: { "LC_ALL" => "C", "LANG" => "C" })
      describe "a" do
        @x = 1
      end
    RUBY
    assert_equal [["*", "expected", "instance-variables", "check"]], rows.map { _1.values_at("example", "status", "reason", "stage") }
    assert_includes rows.first["message"], "…"
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
    assert_equal [["a b", "pass", ""]], rows.map { _1.values_at("example", "status", "reason") }
  end

  def test_puts_a_matcher_without_parentheses_in_parentheses
    rows = status(<<~RUBY)
      describe "a" do
        it "b" do
          1.0.should be_close 1.00001, 0.001
        end
      end
    RUBY
    assert_equal [["a b", "pass"]], rows.map { _1.values_at("example", "status") }
  end

  def test_counts_what_ruby_does_not_pass_as_the_harness
    rows = status(<<~RUBY, args: ["--ruby", "ruby"])
      describe "a" do
        it "passes" do
          1.should == 1
        end
        it "passes under cameo alone" do
          RUBY_ENGINE.should == "cameo"
        end
        it "fails under both" do
          1.should == 2
        end
        ruby_version_is "99" do
          it "runs under neither" do
            1.should == 1
          end
        end
        guard -> { RUBY_ENGINE == "cameo" } do
          it "runs under cameo alone" do
            1.should == 1
          end
        end
      end
    RUBY
    assert_equal [
      ["a passes", "pass", ""],
      ["a passes under cameo alone", "harness", "fails under Ruby"],
      ["a fails under both", "harness", "fails under Ruby too"],
      ["a runs under cameo alone", "harness", "not run under Ruby"],
    ], rows.map { _1.values_at("example", "status", "reason") }
  end

  def test_names_the_stage_of_a_file_that_did_not_run
    rows = status(<<~RUBY)
      require "no_such_library"
      describe "a" do
      end
    RUBY
    assert_equal [["*", "expected", "require", "crash"]], rows.map { _1.values_at("example", "status", "reason", "stage") }
  end
end
