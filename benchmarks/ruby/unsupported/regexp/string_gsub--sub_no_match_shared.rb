# frozen_string_literal: true
STR = ((("a" * 31) + "<") * 1000).freeze
STR_UNICODE = ((("a" * 30) + "\u2028") * 1000).freeze
ESCAPED_CHARS_BINARY = {
  "\u2028".b => '\u2028'.b,
  "\u2029".b => '\u2029'.b,
  ">".b => '\u003e'.b.freeze,
  "<".b => '\u003c'.b.freeze,
  "&".b => '\u0026'.b.freeze,
}
BINARY_PATTERN = Regexp.union(ESCAPED_CHARS_BINARY.keys)

ESCAPED_CHARS = {
  "\u2028" => '\u2028',
  "\u2029" => '\u2029',
  ">" => '\u003e',
  "<" => '\u003c',
  "&" => '\u0026',
}
ESCAPE_PATTERN = Regexp.union(ESCAPED_CHARS.keys)

NO_MATCH_SHARED_STRING = ("a" * 100_000).freeze

str = NO_MATCH_SHARED_STRING.dup
str.sub!("z", "x")
str

