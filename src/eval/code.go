package eval

import (
	"sync"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/syntax"
)

// code is what the interpreters of one program share, the script's and
// every async worker's: the values of its literals that are made once, and
// its compiled methods.
type code struct {
	// literals holds the value of a BigInt literal and of a Regexp
	// literal without interpolation, by node.
	literals sync.Map // syntax.Node -> object.Value
	// compiled is the Go implementation `cameo build` made of the
	// program's top-level methods, nil when it made none; see
	// RegisterCompiled.
	compiled Compiled
}

// literal returns the value of the literal n, which make makes on first
// use. A literal make fails for is made again on its next evaluation.
func (c *code) literal(n syntax.Node, make func() (object.Value, error)) (object.Value, error) {
	if v, ok := c.literals.Load(n); ok {
		return v.(object.Value), nil
	}
	v, err := make()
	if err != nil {
		return object.Value{}, err
	}
	actual, _ := c.literals.LoadOrStore(n, v)
	return actual.(object.Value), nil
}
