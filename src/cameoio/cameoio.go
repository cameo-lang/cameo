// Package cameoio has the Envs a host gives cameo (cameo.Env): the
// machine's own files and standard streams, DefaultEnv, and an Env of the
// files and streams a host chooses, New. Package cameo touches no file or
// stream of the host itself, and this package does not import it: a
// program that does not link cameoio uses none of cameo's default I/O.
//
//	prog, err := cameo.Load("script.cameo", cameoio.DefaultEnv())
//	prog, err := cameo.LoadFromString("t.cameo", src, cameoio.Streams(&out, &out))
package cameoio

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

// Env is a cameo.Env of the files and streams it was made with.
type Env struct {
	fsys   afero.Fs
	dir    string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

// DefaultEnv is the machine's own: the files of the disk, a relative path
// starting at the working directory, and the process's standard streams.
// It is what the cameo command runs scripts with; a script still reads
// and writes files only once the program allows file I/O (AllowIO).
func DefaultEnv() *Env {
	dir, err := os.Getwd()
	if err != nil {
		dir = "/"
	}
	return New(afero.NewOsFs(), dir, os.Stdin, os.Stdout, os.Stderr)
}

// New is an Env of fsys, a relative path starting at dir, and the streams
// given. A nil fsys shares no files: LoadFile fails and a script has no
// File.read; a nil stdin reads nothing, and a nil stdout or stderr
// discards what is written to it.
func New(fsys afero.Fs, dir string, stdin io.Reader, stdout, stderr io.Writer) *Env {
	if stdin == nil {
		stdin = eof{}
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	return &Env{fsys: fsys, dir: dir, stdin: stdin, stdout: stdout, stderr: stderr}
}

// Streams is an Env of output alone: no files and nothing to read, its
// output to stdout and stderr. A host that hands cameo the source of its
// scripts, and a test, need no more.
func Streams(stdout, stderr io.Writer) *Env { return New(nil, "", nil, stdout, stderr) }

// ErrNoFiles is what LoadFile returns for an Env without files.
var ErrNoFiles = errors.New("cameoio: the Env shares no files")

// LoadFile returns the source of the script at path: a relative path
// starts at the Env's directory.
func (e *Env) LoadFile(path string) (string, error) {
	if e.fsys == nil {
		return "", ErrNoFiles
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(e.dir, path)
	}
	data, err := afero.ReadFile(e.fsys, path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Stdin, Stdout and Stderr are the Env's streams.
func (e *Env) Stdin() io.Reader  { return e.stdin }
func (e *Env) Stdout() io.Writer { return e.stdout }
func (e *Env) Stderr() io.Writer { return e.stderr }

// Files is the Env's file system and the directory a relative path
// starts at; a nil file system for an Env without files.
func (e *Env) Files() (afero.Fs, string) { return e.fsys, e.dir }

type eof struct{}

func (eof) Read([]byte) (int, error) { return 0, io.EOF }
