package cameoio_test

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/cameoio"
	"github.com/spf13/afero"
)

// Every Env of the package is a cameo.Env.
var (
	_ cameo.Env = cameoio.DefaultEnv()
	_ cameo.Env = cameoio.Streams(nil, nil)
)

// A script loads from the Env's files, a relative path from its
// directory, writes to its streams, and reads and writes its files once
// file I/O is allowed.
func TestAScriptLoadsAndReadsThroughItsEnv(t *testing.T) {
	fsys := afero.NewMemMapFs()
	assert.NoError(t, afero.WriteFile(fsys, "/work/hello.cameo", []byte(`puts File.read("notes.txt").upcase
File.write("out.txt", "done")
warn "warned"
`), 0o644))
	assert.NoError(t, afero.WriteFile(fsys, "/work/notes.txt", []byte("hi\n"), 0o644))
	var out, errs bytes.Buffer
	env := cameoio.New(fsys, "/work", nil, &out, &errs)
	prog, err := cameo.Load("hello.cameo", env)
	assert.NoError(t, err)
	prog.AllowIO("file")
	assert.NoError(t, prog.Run())
	assert.Equals(t, out.String(), "HI\n")
	assert.Equals(t, errs.String(), "warned\n")
	written, err := afero.ReadFile(fsys, "/work/out.txt")
	assert.NoError(t, err)
	assert.Equals(t, string(written), "done")
}

// An Env of streams alone shares no files: LoadFile fails, and a script
// has no File.read even with file I/O allowed.
func TestStreamsShareNoFiles(t *testing.T) {
	var out bytes.Buffer
	env := cameoio.Streams(&out, &out)
	_, err := cameo.Load("x.cameo", env)
	assert.True(t, errors.Is(err, cameoio.ErrNoFiles), "LoadFile: %v", err)
	prog, err := cameo.LoadFromString("t.cameo", "p File.respond_to?(:read)", env)
	assert.NoError(t, err)
	prog.AllowIO("file")
	assert.NoError(t, prog.Run())
	assert.Equals(t, out.String(), "false\n")
	data, _ := io.ReadAll(env.Stdin())
	assert.Equals(t, len(data), 0)
}

// cameo refuses to load without an Env.
func TestNoEnvIsRefused(t *testing.T) {
	_, err := cameo.LoadFromString("t.cameo", "1", nil)
	assert.True(t, errors.Is(err, cameo.ErrNoEnv), "Load: %v", err)
	_, err = cameo.Resume(nil, nil)
	assert.True(t, errors.Is(err, cameo.ErrNoEnv), "Resume: %v", err)
}

// Package cameo does not link this package: a host that does not import
// it uses none of cameo's default I/O.
func TestCameoDoesNotLinkTheDefaultEnv(t *testing.T) {
	deps, err := exec.Command("go", "list", "-deps", "github.com/cameo-lang/cameo/src/cameo").Output()
	assert.NoError(t, err)
	for _, dep := range strings.Fields(string(deps)) {
		assert.True(t, dep != "github.com/cameo-lang/cameo/src/cameoio", "package cameo depends on cameoio")
	}
}
