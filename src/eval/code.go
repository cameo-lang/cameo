package eval

import (
	"sync"

	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/object"
)

// code is what the interpreters of one program share, the script's and
// every async worker's: its units, and the values of its literals that
// are made once.
type code struct {
	// programs holds the units of the programs that ran: the script, the
	// libraries written in cameo it required, the lines of a console.
	programs sync.Map // *syntax.Program -> *program
	// literals holds the value of an Integer literal beyond 64 bits and
	// of a Regexp literal without interpolation, by instruction.
	literals sync.Map // *cameopb.Code -> object.Value
	// units holds the units of the program by digest: the ones it
	// compiled and the ones a closure it decoded carried. A closure names
	// its code by the digest.
	units sync.Map // string -> *cameopb.Unit
	// params holds the parameter lists bindParams reads, by proc.
	params sync.Map // *cameopb.Proc -> *params
	// flat holds the flat code of each list that was lowered, by its
	// first instruction; nil for a list that runs from cursors.
	flat sync.Map // *cameopb.Code -> *flatCode
	// syms holds the names of each unit as Syms (symsOf).
	syms sync.Map // *cameopb.Unit -> *unitNames
	// uses holds what each proc needs of its binding to travel; see
	// closure.go.
	uses sync.Map // *cameopb.Proc -> *uses
}

// literal returns the value of the literal n, which make makes on first
// use. A literal make fails for is made again on its next evaluation.
func (c *code) literal(n *cameopb.Code, make func() (object.Value, error)) (object.Value, error) {
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
