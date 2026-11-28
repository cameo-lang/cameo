package tests

import (
	"bytes"
	"errors"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/cameoio"
)

// proc fields are cameo's own; these outputs are their specification.

func TestProcFieldRefusesWhatIsNoBlockAndADefault(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cameo.RunFromString("t.cameo", "class Route\n  proc :handler, default: nil\nend\n", cameoio.Streams(&stdout, &stderr))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "proc :handler takes no default")
	err = cameo.RunFromString("t.cameo", "proc :handler\n", cameoio.Streams(&stdout, &stderr))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "proc :handler declares a field, which only a class body can do")
	err = cameo.RunFromString("t.cameo", "class A\n  def m\n    proc :handler\n  end\nend\n", cameoio.Streams(&stdout, &stderr))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "proc :handler declares a field, which only a class body can do")
}

func TestAnInstanceWithAProcFieldGoesThroughADump(t *testing.T) {
	// A paused script's dump holds the instance, and the resumed run
	// calls its proc.
	const src = "class Route\n  string :path\n  proc :handler\nend\nk = 3\nr = Route.new(path: \"/a\", handler: proc { |x| x * k })\nTest.pause\nputs r.path, r.handler.call(5)\n"
	prog, err := cameo.LoadFromString("t.cameo", src, cameoio.Streams(&bytes.Buffer{}, &bytes.Buffer{}))
	assert.NoError(t, err)
	prog.Module("Test").Def("pause", func([]any, map[string]any, *cameo.Block) (any, error) {
		prog.Pause()
		return nil, nil
	})
	err = prog.Run()
	assert.True(t, errors.Is(err, cameo.ErrPaused), "Run returned %v, want ErrPaused", err)
	dump, err := prog.Dump()
	assert.NoError(t, err)
	assert.Equals(t, len(dump.Missing), 0)
	var out bytes.Buffer
	cont, err := cameo.Resume(dump.Data, cameoio.Streams(&out, &out))
	assert.NoError(t, err)
	cont.Module("Test").Def("pause", func([]any, map[string]any, *cameo.Block) (any, error) { return nil, nil })
	assert.NoError(t, cont.Run())
	assert.Equals(t, out.String(), "/a\n15\n")
}
