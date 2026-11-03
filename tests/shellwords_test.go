package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected strings are Ruby 3.4's, with shellwords 0.2.2.

func TestShellwordsSplitsEscapesAndJoinsAsRubyDoes(t *testing.T) {
	out := runScript(t, `require "shellwords"
p Shellwords.split("ruby -e 'puts 1' \"two words\""), Shellwords.split("a  b\tc\nd"), Shellwords.split(""), Shellwords.split("a'b c'd"), Shellwords.split("a\"b c\"d"), Shellwords.split("\"a\\\"b\" 'c\\d'")
p Shellwords.split("a\\ b c"), Shellwords.split("a\\"), Shellwords.split("\\"), Shellwords.split("\"\\$HOME \\`+"`"+`x\\`+"`"+` \\\\ \\n \\q\""), Shellwords.split("a '' b"), Shellwords.split("a\\\nb"), Shellwords.split("\"a\nb\""), Shellwords.split("日本 語"), Shellwords.split("\"a\"'b'c\\ d")
["'unclosed", "\"unclosed", "x 'y", "x  'y", "x ' y"].each do |s|
  Shellwords.split(s)
rescue ArgumentError => e
  puts e.message
end
p ["", "a", "a b", "it's", "a\nb", "日本", "x;y|z&w", "$HOME `+"`cmd`"+` \"q\" \\", "-f", "a=b", "~/x", "*.rb", "é"].map { |s| Shellwords.escape(s) }
p Shellwords.join(["ls", "-l", "a b", "it's", ""]), ["echo", 1, nil, :sym].shelljoin, "a 'b c'".shellsplit, "a b".shellescape, Shellwords.shellwords("x y"), Shellwords.shellescape(1), Shellwords::VERSION
`)
	assert.Equals(t, out, `["ruby", "-e", "puts 1", "two words"]
["a", "b", "c", "d"]
[]
["ab cd"]
["ab cd"]
["a\"b", "c\\d"]
["a b", "c"]
["a\\"]
["\\"]
["$HOME `+"`x`"+` \\ \\n \\q"]
["a", "", "b"]
["a\\\nb"]
["a\nb"]
["日本", "語"]
["abc d"]
Unmatched quote at 0: '
Unmatched quote at 0: "
Unmatched quote at 2: ...'
Unmatched quote at 2: ... '
Unmatched quote at 2: ...' 
["''", "a", "a\\ b", "it\\'s", "a'\n'b", "\\日\\本", "x\\;y\\|z\\&w", "\\$HOME\\ \\`+"`"+`cmd\\`+"`"+`\\ \\\"q\\\"\\ \\\\", "-f", "a\\=b", "\\~/x", "\\*.rb", "\\é"]
"ls -l a\\ b it\\'s ''"
"echo 1 '' sym"
["a", "b c"]
"a\\ b"
["x", "y"]
"1"
"0.2.2"
`)
}
