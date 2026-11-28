# abbrev: testcase to demonstrate that abbrev follows Ruby's
#
# The expected values are Ruby's, with abbrev 0.1.2.
#
# (Introduced in PR #281. expected output follows the __END__ separator.)
require "abbrev"
p Abbrev.abbrev(%w[ruby rules]), %w[car cone].abbrev, Abbrev.abbrev(%w[car cone], "ca"), Abbrev.abbrev(%w[car cone], /^co/), Abbrev.abbrev([]), Abbrev.abbrev(["", "a"]), Abbrev.abbrev(%w[a a]), Abbrev.abbrev(%w[abc ab a]), %w[日本 日光].abbrev, Abbrev.abbrev(%w[summer winter], "w"), Abbrev::VERSION

__END__
{"ruby" => "ruby", "rub" => "ruby", "rules" => "rules", "rule" => "rules", "rul" => "rules"}
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
