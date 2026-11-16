# frozen_string_literal: true

# Tests of runner/classify.rb: the status and the reason it gives the
# messages cameo prints.
#
#   ruby specs/test/classify_test.rb

require "minitest/autorun"
require_relative "../runner/classify"

class ClassifyTest < Minitest::Test
  # The messages are cameo's own, from src/syntax/convert.go,
  # src/object/value.go and src/stdlib.
  INVALID = [
    'a string literal with bytes that are not UTF-8 is a binary string and is written b"..."',
    "invalid byte sequence in UTF-8: keep the bytes in a binary string (Encoding::InvalidByteSequenceError)",
    "invalid byte sequence in US-ASCII: keep the bytes in a binary string (Encoding::InvalidByteSequenceError)",
    'invalid byte sequence in UTF-8: "\xFF" is not text; keep such bytes in a binary string (ArgumentError)',
    "invalid byte sequence in UTF-8: pack returns text, and these bytes are none (Encoding::InvalidByteSequenceError)",
  ].freeze

  BINARY = [
    "upcase reads characters, and a binary string has bytes alone: say what they are with force_encoding first (Encoding::CompatibilityError)",
    "an interpolation makes text, and a binary string has bytes alone: say what they are with force_encoding first, or join bytes with + (Encoding::CompatibilityError)",
    "getbyte reads a byte, and this UTF-8 string has characters beyond ASCII: call b first for its bytes, or read a character with [] (Encoding::CompatibilityError)",
    "setbyte sets a byte, and only a binary string has bytes alone: call b first (Encoding::CompatibilityError)",
  ].freeze

  def test_bytes_that_are_not_utf8_are_expected
    INVALID.each { assert_equal ["expected", "invalid-strings"], Classify.status(_1), _1 }
  end

  def test_a_binary_string_without_characters_is_expected
    BINARY.each { assert_equal ["expected", "binary-strings"], Classify.status(_1), _1 }
  end

  # Parts of the libraries that doc/stdlib.md leaves out, in the messages
  # cameo prints for them.
  LIBRARY = [
    "Zlib::Deflate.new is not supported: a stream fed piece by piece; use Zlib::Deflate.deflate on the whole string (NotImplementedError)",
    "uninitialized constant Zlib::GzipReader (NameError)",
    "undefined method 'eigensystem' for an instance of Matrix (NoMethodError)",
    "uninitialized constant Matrix::LUPDecomposition (NameError)",
    "Sorry, Operation(**) not implemented: Matrix op Float (ExceptionForMatrix::ErrOperationNotImplemented)",
    "URI.regexp is obsolete in Ruby and not in cameo; use URI.extract (NotImplementedError)",
    "uninitialized constant Date::Infinity (NameError)",
    "strptime: Go's time has no layout for %U (ArgumentError)",
    "uninitialized constant CGI::Cookie (NameError)",
    "uninitialized constant OpenSSL::X509 (NameError)",
    "undefined method 'secure_compare' for module OpenSSL (NoMethodError)",
    "undefined method 'pwd' for class Pathname (NoMethodError)",
  ].freeze

  def test_a_part_a_library_leaves_out_is_expected
    LIBRARY.each { assert_equal ["expected", "library-left-out"], Classify.status(_1), _1 }
  end

  def test_what_a_library_leaves_out_on_purpose_is_expected
    assert_equal ["expected", "mutation"], Classify.status("into: fills a Hash in place, which a value cannot be; read options after the parse, or use getopts, which returns the Hash (ArgumentError)")
    assert_equal ["expected", "enumerator-next"], Classify.status("undefined method 'next' for an instance of Enumerator (NoMethodError)")
    assert_equal ["expected", "c-api"], Classify.status("undefined method 'load_extension' for main (NoMethodError)")
  end

  def test_a_missing_method_of_a_library_is_a_bug
    assert_equal ["bug", ""], Classify.status("undefined method 'ascend' for an instance of Pathname (NoMethodError)")
    assert_equal ["bug", ""], Classify.status("uninitialized constant OpenSSL::Digest (NameError)")
  end

  # cameo ends the message of what it leaves out with the link to the page
  # of its code; the rules read the message without it.
  def test_reads_a_message_without_the_link_of_its_code
    assert_equal ["expected", "literal-only"], Classify.status("undefined method 'new' for class Range: use a literal (see https://15b.it/lit) (NoMethodError)")
    assert_equal ["expected", "threads"], Classify.status("uninitialized constant Thread (see https://15b.it/thr) (NameError)")
    assert_equal ["expected", "mutation"], Classify.status("cameo does not support mutating calls without rebinding; use a = a << 1, +, or append (see https://15b.it/mut) (CameoError)")
  end

  def test_an_unknown_message_is_a_bug
    assert_equal ["bug", ""], Classify.status("Expected 1 == 2 to be truthy but was false")
  end
end
