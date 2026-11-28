package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/cameoio"
)

// attr :name, type: X declares a typed field whose type is a class or the
// name of one. What it prints is specified by tests/scripts/typed_attr_*;
// this file holds what it refuses.

func TestAttrRefusesWhatIsNoType(t *testing.T) {
	for src, want := range map[string]string{
		"class A\n  attr :x\nend\n":                  "attr without type: is not supported",
		"class A\n  attr :x, type: 3\nend\n":         "attr: type: is a class or the name of a class",
		"class A\n  attr :x, type: \"nope\"\nend\n":  "attr: type: \"nope\" is not a class name",
		"class A\n  attr :x, type: A, of: 1\nend\n":  "attr: unknown option of:",
		"class A\n  attr \"x\", type: String\nend\n": "attr: a field is named by a symbol",
		"attr :x, type: String\n":                    "attr :x declares a field, which only a class body can do",
		"class A\n  attr :x, type: Nope\nend\n":      "Nope",
	} {
		var stdout, stderr bytes.Buffer
		err := cameo.RunFromString("t.cameo", src, cameoio.Streams(&stdout, &stderr))
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), want)
	}
}
