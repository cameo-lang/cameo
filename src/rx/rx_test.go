package rx

import (
	"errors"
	"regexp"
	"testing"
)

func TestTranslate(t *testing.T) {
	for _, c := range []struct {
		ruby    string
		options int
		match   string
		want    bool
	}{
		{`^b`, 0, "a\nb", true},                // ^ is a line start
		{`a.c`, Multiline, "a\nc", true},       // m is Go's s
		{`a.c`, 0, "a\nc", false},              //
		{`ABC`, IgnoreCase, "abc", true},       //
		{`a b # c`, Extended, "ab", true},      // x drops spaces and comments
		{`\h+\z`, 0, "c0ffee", true},           //
		{`a{,2}b`, 0, "aab", true},             // Ruby's {,n}
		{`\e\[`, 0, "\x1b[", true},             //
		{`\xE3\x81\x82`, 0, "あ", true},         // bytes of a UTF-8 character
		{`\u{1F600}`, 0, "😀", true},            //
		{`\p{Alpha}+`, 0, "été", true},         // Ruby's property names
		{`(?-mix:a b)`, Extended, "a b", true}, // to_s turns x off
		{`[\h\s]+`, 0, "a f", true},            //
	} {
		pattern, _, err := Translate(c.ruby, c.options)
		if err != nil {
			t.Fatalf("%s: %v", c.ruby, err)
		}
		if got := regexp.MustCompile(pattern).MatchString(c.match); got != c.want {
			t.Errorf("/%s/ (as %s) on %q: %v, want %v", c.ruby, pattern, c.match, got, c.want)
		}
	}
}

func TestNamedGroupsAloneCapture(t *testing.T) {
	// With a named group, a plain group does not capture, as in Ruby.
	pattern, names, err := Translate(`(?<y>\d+)-(\d+)`, 0)
	if err != nil || len(names) != 1 || names[0] != "y" || regexp.MustCompile(pattern).NumSubexp() != 1 {
		t.Fatalf("%s %v %v", pattern, names, err)
	}
}

func TestBacktrackingIsRefused(t *testing.T) {
	for ruby, feature := range map[string]string{
		`(a)\1`: "a backreference", `(?<x>a)\k<x>`: "a named backreference", `(?=a)`: "lookahead",
		`(?<!a)b`: "lookbehind", `(?>a)`: "an atomic group", `a*+`: "a possessive quantifier",
		`a\Z`: `\Z, which needs lookahead`, `[a-z&&[^x]]`: "a character class intersection",
		`(?<n>a)\g<n>`: "a subexpression call",
	} {
		_, _, err := Translate(ruby, 0)
		var u *Unsupported
		if !errors.As(err, &u) || u.Feature != feature {
			t.Errorf("/%s/: %v, want %s", ruby, err, feature)
		}
	}
	_, _, err := Translate(`(a)\1`, 0)
	if want := `This regexp is not supported in Amber: /(a)\1/ uses a backreference`; err.Error() != want {
		t.Fatalf("%q", err)
	}
}
