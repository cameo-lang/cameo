package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/cameoio"
)

// A block captures the values of its variables when it is made, not the
// variables: these outputs are the specification. A rebinding after the
// block was made is invisible to it, a rebinding inside it is refused by
// the checker (src/syntax/syntax_test.go), its own variables are its own,
// and a lambda may call itself by the name it is assigned to.

func TestTheCheckerRefusesARebindingInABlock(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cameo.RunFromString("t.cameo", "total = 0\n[1, 2].each { |x| total += x }\n", cameoio.Streams(&stdout, &stderr))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "t.cameo:2: a block cannot rebind total, a variable of the scope around it: a block captures the values its variables have when it is made; bind a new name, or use reduce, each_with_object or while")
}
