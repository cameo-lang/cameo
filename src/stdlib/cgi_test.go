package stdlib

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected strings are what Ruby 3.4's CGI returns for the same input.

func TestUnescapeHTMLKeepsAnAmpersandThatStartsNoEntity(t *testing.T) {
	assert.Equals(t, unescapeHTML("&&amp;&lt&lt;"), "&&&lt<")
}

func TestUnescapeHTMLDecodesNumericReferencesBelowTheLastCodepoint(t *testing.T) {
	assert.Equals(t, unescapeHTML("&#065;&#x41;&#X41;&#0;"), "AAA\x00")
	assert.Equals(t, unescapeHTML("&#1114111;&#x10FFFF;&#99999999999999999999;"), "&#1114111;&#x10FFFF;&#99999999999999999999;")
}

func TestUnescapeHTMLWritesASurrogateAsItsThreeBytes(t *testing.T) {
	assert.Equals(t, unescapeHTML("&#xD800;"), "\xed\xa0\x80")
}

func TestEscapeElementNeedsTheWholeNameAndIgnoresCase(t *testing.T) {
	assert.Equals(t, escapeElement("<b><br><B x><bx><bé>", []string{"b"}), "&lt;b&gt;<br>&lt;B x&gt;<bx>&lt;bé&gt;")
	assert.Equals(t, escapeElement("<a", []string{"a"}), "<a")
}

func TestUnescapeElementMatchesTheEscapedTagsWithoutRegardToCase(t *testing.T) {
	assert.Equals(t, unescapeElement("&LT;a&GT;&lt;a &amp;&gt;", []string{"a"}), "&LT;a&GT;<a &>")
}

func TestPrettyHTMLIndentsEachLevel(t *testing.T) {
	assert.Equals(t, prettyHTML("<a><b>x</b></a>", "  "), "<a>\n  <b>\n    x\n  </b>\n</a>\n")
}

func TestPrettyHTMLWithAShiftOfUnderscoresDropsOnlyTheFence(t *testing.T) {
	assert.Equals(t, prettyHTML("<x>__<y>q</y></x>", "__"), "<x>\n____\n__<y>\n____q\n__</y>\n</x>\n")
}
