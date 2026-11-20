package cameo

import (
	"errors"
	"io"

	"github.com/spf13/afero"
)

// Env is all of the host a program reaches: the script files Load and
// Run read, the three streams, and the files a script reads and
// writes once AllowIO allows file I/O. Package cameo touches no file or
// stream of the host itself; everything goes through the Env a host
// passes to Load, LoadFromString, Run, RunFromString or Resume.
//
// The package cameoio has the machine's own, cameoio.DefaultEnv: the
// working directory's files and the process's standard streams. A host
// that shares less passes an Env of its own, and one that never imports
// cameoio can show, with go list -deps, that no script reaches the host
// through cameo's defaults.
type Env interface {
	// LoadFile returns the source of the script at path.
	LoadFile(path string) (string, error)
	// Stdin is what the host reads on the program's behalf, such as the
	// lines cameo irb evaluates; a script has no stdin of its own.
	Stdin() io.Reader
	// Stdout and Stderr are where the script's puts, print and p, and
	// its warn, write.
	Stdout() io.Writer
	Stderr() io.Writer
	// Files is the file system a script reads and writes, through
	// File.read, File.open and File.write, once AllowIO allows file I/O:
	// a relative path starts at dir, an absolute path is fsys's own, and
	// ".." stops at its root. A nil fsys shares no files, and File then
	// has no read, open or write whatever AllowIO allows. A host that
	// shares part of a disk confines the script there with
	// afero.NewBasePathFs, and keeps it from writing with
	// afero.NewReadOnlyFs.
	Files() (fsys afero.Fs, dir string)
}

// noHost is an Env that shares nothing: no files, nothing to read, and
// output that goes nowhere; InspectDump resumes a dump on it.
type noHost struct{}

func (noHost) LoadFile(path string) (string, error) {
	return "", errors.New("cameo: the host shares no files: cannot load " + path)
}
func (noHost) Stdin() io.Reader          { return eofReader{} }
func (noHost) Stdout() io.Writer         { return io.Discard }
func (noHost) Stderr() io.Writer         { return io.Discard }
func (noHost) Files() (afero.Fs, string) { return nil, "" }

type eofReader struct{}

func (eofReader) Read([]byte) (int, error) { return 0, io.EOF }

// ErrNoEnv is what Load, LoadFromString, Run, RunFromString and Resume return for a
// nil Env.
var ErrNoEnv = errors.New("cameo: no Env: pass the host's, such as cameoio.DefaultEnv()")
