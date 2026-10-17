# frozen_string_literal: true

require 'cgi/escape'

STR = ((("a" * 31) + "<") * 1000).freeze

ESCAPED_CHARS = {
  "\\" => "&#39",
  "\"" => "&quot;",
  ">" => '&gt;',
  "<" => '&lt;',
  "&" => '&amp;',
}.freeze
ESCAPE_PATTERN = Regexp.union(ESCAPED_CHARS.keys)

#--- benchmark: the prelude above runs once, the script below in the loop ---
STR.gsub(ESCAPE_PATTERN, ESCAPED_CHARS)

