package cameo

import (
	"bytes"
	"errors"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// A value that refers to a module the host gave the program, a variable
// and a method's argument, restores in a resumed run: the module is there
// from Resume on, and the host gives it its methods again.
func TestAResumedRunKeepsWhatRefersToAHostModule(t *testing.T) {
	const src = "def ask(model, n)\n  Host.pause\n  model.answer(n)\nend\nmodel = Model\nputs ask(model, 1)\nputs ask(Model, 2)\n"
	host := func(prog *Program, pause bool, factor int64) {
		prog.Module("Host").Def("pause", func([]any, map[string]any, *Block) (any, error) {
			if pause {
				prog.Pause()
			}
			return nil, nil
		})
		prog.Module("Model").Def("answer", func(args []any, _ map[string]any, _ *Block) (any, error) {
			return args[0].(int64) * factor, nil
		})
	}
	var out bytes.Buffer
	prog, err := Load("t.cameo", src, &out, &out)
	assert.NoError(t, err)
	host(prog, true, 10)
	assert.True(t, errors.Is(prog.Run(), ErrPaused), "the script did not pause")
	dump, err := prog.Dump()
	assert.NoError(t, err)
	assert.Equals(t, len(dump.Missing), 0)

	text, err := InspectDump(dump.Data)
	assert.NoError(t, err)
	assert.Contains(t, text, "  model = Model\n")

	var rest bytes.Buffer
	cont, err := Resume(dump.Data, &rest, &rest)
	assert.NoError(t, err)
	host(cont, false, 100)
	assert.NoError(t, cont.Run())
	assert.Equals(t, rest.String(), "100\n200\n")
}
