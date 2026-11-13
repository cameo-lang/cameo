// Command io lets a script read and write files, in a file system the Go
// program chooses. A script does no I/O unless the program that runs it
// allows a kind of it: here the kind "file", with a file system in
// memory that holds one file.
//
//	go run ./examples/io
package main

import (
	_ "embed"
	"fmt"
	"io"
	"os"

	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/spf13/afero"
)

//go:embed notes.cameo
var script string

// run runs the script with the files of fsys, relative paths read from
// dir.
func run(fsys afero.Fs, dir string, stdout, stderr io.Writer) error {
	prog, err := cameo.Load("notes.cameo", script, stdout, stderr)
	if err != nil {
		return err
	}
	// Both are needed: the kind allowed, and a file system to read. The
	// file system may be the disk (afero.NewOsFs), one directory of it
	// (afero.NewBasePathFs), a read-only view (afero.NewReadOnlyFs) or
	// memory, as here.
	prog.AllowIO("file")
	prog.FileSystem(fsys, dir)
	return prog.Run()
}

func main() {
	fsys := afero.NewMemMapFs()
	if err := afero.WriteFile(fsys, "/work/notes.txt", []byte("three\nlines\nhere\n"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := run(fsys, "/work", os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	shout, _ := afero.ReadFile(fsys, "/work/shout.txt")
	fmt.Printf("the script wrote %d bytes into the file system\n", len(shout))
}
