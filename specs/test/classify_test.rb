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

  def test_an_unknown_message_is_a_bug
    assert_equal ["bug", ""], Classify.status("Expected 1 == 2 to be truthy but was false")
  end
end
