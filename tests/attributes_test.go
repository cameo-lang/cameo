package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/cameoio"
)

// Class attributes are cameo's own; these outputs are their
// specification. A class is built in its body as an instance is in
// initialize: self.name = v sets an attribute while the body runs, from
// the body or from a class method it calls, and the class is sealed when
// the body ends.

func TestAttributeErrors(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"class A\n  attribute :x\n  def m\n    self.x = 1\n  end\nend\n", "self.x = …: a field is set in initialize, and a class attribute in a class body or a class method; elsewhere use with"},
		{"class A\n  attribute :x, :y, default: 1, other: 2\nend\n", "attribute: the one option is default:"},
		{"class A\n  attribute \"x\"\nend\n", "attribute: an attribute is named by a symbol"},
		{"module M\n  attribute :x\nend\n", "attribute :x: an attribute belongs to a class body"},
	} {
		var stdout, stderr bytes.Buffer
		err := cameo.RunFromString("t.cameo", c.src, cameoio.Streams(&stdout, &stderr))
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), c.want)
	}
}
