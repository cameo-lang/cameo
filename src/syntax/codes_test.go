package syntax

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// refusals is a script for each code of Codes that the checker refuses
// with it.
var refusals = map[string]string{
	"attr":  "class A\n  attr_reader :x\nend",
	"blk":   "t = 0\n[1].each { |n| t += n }",
	"bobj":  "class P < BasicObject\nend",
	"cvar":  "class A\n  @@n = 0\nend",
	"glob":  "$n = 0",
	"inv":   `s = "\xFF"`,
	"ivar":  "class A\n  def initialize\n    @x = 1\n  end\nend",
	"meta":  `eval("1")`,
	"mut":   "a = [2, 1]\na.sort!",
	"proc":  "puts `date`",
	"rat":   "x = 1r",
	"re":    "p 'ab' =~ /(?<=a)b/",
	"req":   `require_relative "x"`,
	"scope": "module M\nend\nM::X = 1",
	"vis":   "class A\n  private\nend",
}

// TestRefusalsLinkThePageOfTheirCode proves each code of Codes is the
// code of a refusal, and that the refusal's message ends with the page
// that explains it.
func TestRefusalsLinkThePageOfTheirCode(t *testing.T) {
	for _, code := range Codes {
		src, ok := refusals[code]
		if !ok {
			t.Errorf("no script is refused with %s", code)
			continue
		}
		_, err := Parse("test.cameo", src)
		var list ErrorList
		if !errors.As(err, &list) || len(list) == 0 {
			t.Errorf("%s: %q was not refused: %v", code, src, err)
			continue
		}
		if list[0].Code != code {
			t.Errorf("%s: %q was refused with the code %q: %v", code, src, list[0].Code, list[0])
		}
		want := " (see https://15b.it/" + code + ")"
		if !strings.HasSuffix(list[0].Error(), want) {
			t.Errorf("%s: the message does not end with the page:\n  %v", code, list[0])
		}
	}
	for code := range refusals {
		if !slices.Contains(Codes, code) {
			t.Errorf("%s is not one of Codes", code)
		}
	}
}

// TestAnErrorThatIsNoRefusalLinksNothing proves a syntax error, which
// cameo does not refuse on purpose, carries no code.
func TestAnErrorThatIsNoRefusalLinksNothing(t *testing.T) {
	_, err := Parse("test.cameo", "def f(\n")
	if err == nil || strings.Contains(err.Error(), "(see ") {
		t.Fatalf("error %v", err)
	}
	_, err = Parse("test.cameo", "def f\n  super_\nend\nyield")
	if err == nil || strings.Contains(err.Error(), "(see ") {
		t.Fatalf("error %v", err)
	}
}
