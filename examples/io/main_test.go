package main

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/spf13/afero"
)

// The script reads the file the program put into the file system and
// writes another into it; the disk is not touched.
func TestTheScriptReadsAndWritesTheProgramsFiles(t *testing.T) {
	fsys := afero.NewMemMapFs()
	assert.NoError(t, afero.WriteFile(fsys, "/work/notes.txt", []byte("three\nlines\nhere\n"), 0o644))
	var out bytes.Buffer
	assert.NoError(t, run(fsys, "/work", &out, &out))
	assert.Equals(t, out.String(), "3 lines\nTHREE\n")
	shout, err := afero.ReadFile(fsys, "/work/shout.txt")
	assert.NoError(t, err)
	assert.Equals(t, string(shout), "THREE\nLINES\nHERE\n")
}

// A read-only file system refuses the script's write.
func TestAReadOnlyFileSystemRefusesTheWrite(t *testing.T) {
	fsys := afero.NewMemMapFs()
	assert.NoError(t, afero.WriteFile(fsys, "/work/notes.txt", []byte("one\n"), 0o644))
	var out bytes.Buffer
	err := run(afero.NewReadOnlyFs(fsys), "/work", &out, &out)
	assert.True(t, err != nil, "the write went through")
	assert.Equals(t, out.String(), "1 lines\n")
}
