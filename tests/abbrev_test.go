package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected values are Ruby's, with abbrev 0.1.2.

func TestAbbrevFollowsRubys(t *testing.T) {
	out := runScript(t, `require "abbrev"
p Abbrev.abbrev(%w[ruby rules]), %w[car cone].abbrev, Abbrev.abbrev(%w[car cone], "ca"), Abbrev.abbrev(%w[car cone], /^co/), Abbrev.abbrev([]), Abbrev.abbrev(["", "a"]), Abbrev.abbrev(%w[a a]), Abbrev.abbrev(%w[abc ab a]), %w[日本 日光].abbrev, Abbrev.abbrev(%w[summer winter], "w"), Abbrev::VERSION
`)
	assert.Equals(t, out, `{"ruby" => "ruby", "rub" => "ruby", "rules" => "rules", "rule" => "rules", "rul" => "rules"}
{"car" => "car", "ca" => "car", "cone" => "cone", "con" => "cone", "co" => "cone"}
{"car" => "car", "ca" => "car"}
{"cone" => "cone", "con" => "cone", "co" => "cone"}
{}
{"a" => "a", "" => ""}
{"a" => "a"}
{"abc" => "abc", "ab" => "ab", "a" => "a"}
{"日本" => "日本", "日光" => "日光"}
{"winter" => "winter", "winte" => "winter", "wint" => "winter", "win" => "winter", "wi" => "winter", "w" => "winter"}
"0.1.2"
`)
}

// A pattern that is neither a String nor a Regexp raises Ruby's
// NoMethodError when first used, so an empty list gives {}; keywords to
// Array#abbrev are the pattern, a Hash, as Ruby passes them; a surplus
// argument gets Array#abbrev's count; a class that includes the module has
// the method; three thousand words are abbreviated in one pass.
func TestAbbrevRefusesAsRubyDoesAndScales(t *testing.T) {
	out := runScript(t, `require "abbrev"
p Abbrev.abbrev([], 1)
begin; Abbrev.abbrev(%w[a], 1); rescue NoMethodError => e; puts e.message; end
begin; p %w[a].abbrev(x: 1); rescue NoMethodError => e; puts e.message; end
begin; %w[a].abbrev("x", "y"); rescue ArgumentError => e; puts e.message; end
class K
  include Abbrev
end
p K.new.abbrev(%w[xy])
words = (1..3000).map { |i| "w#{i.to_s.rjust(9, "0")}" }
p words.abbrev.size
`)
	assert.Equals(t, out, `{}
undefined method '=~' for an instance of Integer
undefined method '=~' for an instance of Hash
wrong number of arguments (given 2, expected 0..1)
{"xy" => "xy", "x" => "xy"}
3003
`)
}

func TestArrayAbbrevSaysToRequireTheLibrary(t *testing.T) {
	out := runScript(t, `begin; [].abbrev; rescue NoMethodError => e; puts e.message; end`)
	assert.Equals(t, out, "undefined method 'abbrev' for an instance of Array: require \"abbrev\" first\n")
}
