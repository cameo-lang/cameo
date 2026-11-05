package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/spf13/afero"
)

// testFiles returns the file system the tests give a program, in memory.
func testFiles(t *testing.T) afero.Fs {
	t.Helper()
	fsys := afero.NewMemMapFs()
	for name, data := range map[string]string{
		"/notes.txt":         "one\ntwo\n",
		"/docs/readme.md":    "# hi\n",
		"/docs/sub/deep.txt": "deep",
		"/|echo hi":          "a file, not a command",
	} {
		assert.NoError(t, afero.WriteFile(fsys, name, []byte(data), 0o644))
	}
	return fsys
}

// runWithFiles runs src in a program that allows io and has fsys, with
// relative paths from dir; fsys nil gives none.
func runWithFiles(t *testing.T, src string, fsys afero.Fs, dir string, io ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	prog, err := cameo.Load("t.cameo", src, &stdout, &stderr)
	assert.NoError(t, err)
	prog.AllowIO(io...)
	if fsys != nil {
		prog.FileSystem(fsys, dir)
	}
	if err := prog.Run(); err != nil {
		t.Fatalf("%s\nfailed: %v\nstderr: %s", src, err, stderr.String())
	}
	return stdout.String()
}

// TestFileReadNeedsFileIOAndAFileSystem proves File.read and File.open are
// there only where the program allows file I/O and gave a file system:
// without either, File answers neither and does not list them, and the
// error says how to allow them.
func TestFileReadNeedsFileIOAndAFileSystem(t *testing.T) {
	src := `p File.respond_to?(:read), File.respond_to?(:open), File.methods.include?(:read), File.singleton_methods.include?(:open)
begin
  File.read("notes.txt")
rescue NoMethodError => e
  puts e.message
end
`
	without := "false\nfalse\nfalse\nfalse\nundefined method 'read' for class File: the host allows no file I/O; run the script with cameo --io=file, or let the program say AllowIO(\"file\") and give it a file system with FileSystem\n"
	assert.Equals(t, runWithFiles(t, src, nil, "/"), without)
	assert.Equals(t, runWithFiles(t, src, nil, "/", "file"), without)
	assert.Equals(t, runWithFiles(t, src, testFiles(t), "/"), without)
	assert.Equals(t, runWithFiles(t, src, testFiles(t), "/", "network"), without)
	with := "true\ntrue\ntrue\ntrue\n"
	assert.Equals(t, runWithFiles(t, `p File.respond_to?(:read), File.respond_to?(:open), File.methods.include?(:read), File.singleton_methods.include?(:open)`, testFiles(t), "/", "file"), with)
	assert.Equals(t, runWithFiles(t, `p File.respond_to?(:read), File.respond_to?(:open), File.methods.include?(:read), File.singleton_methods.include?(:open)`, testFiles(t), "/", "all"), with)
}

// TestFileReadReadsTheHostsFiles proves File.read and File.open read the
// file system the host gave, as Ruby's do: relative paths from the
// directory, absolute ones from the root, ".." stopping at the root; and
// that a "|" names a file and runs nothing.
func TestFileReadReadsTheHostsFiles(t *testing.T) {
	src := `p File.read("readme.md"), File.read("sub/deep.txt", 2, 1), File.read("/notes.txt")
p File.read("../../../notes.txt"), File.read(Pathname("readme.md"))
p File.read("/|echo hi")
p File.open("/notes.txt") { |f| [f.gets, f.readlines, f.eof?] }
f = File.open("/notes.txt")
p f, f.path, f.each_line.map(&:chomp)
f.close
p f.closed?, f
`
	out := runWithFiles(t, "require \"pathname\"\n"+src, testFiles(t), "/docs", "file")
	assert.Equals(t, out, `"# hi\n"`+"\n"+`"ee"`+"\n"+`"one\ntwo\n"`+"\n"+
		`"one\ntwo\n"`+"\n"+`"# hi\n"`+"\n"+
		`"a file, not a command"`+"\n"+
		`["one\n", ["two\n"], true]`+"\n"+
		"#<File:/notes.txt>\n\"/notes.txt\"\n[\"one\", \"two\"]\n"+
		"true\n#<File:/notes.txt (closed)>\n")
}

// TestFileReadRaisesRubysErrors proves the errors are Ruby's: a missing
// file, a directory, a negative length or offset, a NUL; and that a mode
// that writes is Errno::EROFS, since the file system is read only, and
// writing to a handle IOError.
func TestFileReadRaisesRubysErrors(t *testing.T) {
	src := `[-> { File.read("missing") }, -> { File.read("docs") }, -> { File.read("notes.txt", -1) },
 -> { File.read("notes.txt", 1, -1) }, -> { File.read("a\0b") }, -> { File.open("notes.txt", "w") },
 -> { File.read("notes.txt", mode: "a") }, -> { File.open("notes.txt", "x") },
 -> { File.open("notes.txt") { |f| f.write("x") } }].each do |f|
  f.call
rescue => e
  p [e.class, e.message]
end
p Errno::ENOENT::Errno, Errno::ENOENT.ancestors.include?(SystemCallError)
`
	out := runWithFiles(t, src, testFiles(t), "/", "file")
	assert.Equals(t, out, `[Errno::ENOENT, "No such file or directory @ rb_sysopen - missing"]
[Errno::EISDIR, "Is a directory @ io_fread - docs"]
[ArgumentError, "negative length -1 given"]
[ArgumentError, "negative offset -1 given"]
[ArgumentError, "path name contains null byte"]
[Errno::EROFS, "Read-only file system @ rb_sysopen - notes.txt"]
[Errno::EROFS, "Read-only file system @ rb_sysopen - notes.txt"]
[ArgumentError, "invalid access mode x"]
[IOError, "not opened for writing"]
2
true
`)
}

// TestFileReadKeepsRubysEncodings proves a whole file is UTF-8, and a read
// with a length, a binary mode or encoding: "BINARY" is binary.
func TestFileReadKeepsRubysEncodings(t *testing.T) {
	src := `p File.read("notes.txt").encoding, File.read("notes.txt", 2).encoding, File.read("notes.txt", mode: "rb").encoding,
  File.read("notes.txt", encoding: "BINARY").encoding, File.open("notes.txt", "rb") { |f| f.read.encoding }, File.read("notes.txt", 0), File.read("notes.txt", 3, 100)
`
	assert.Equals(t, runWithFiles(t, src, testFiles(t), "/", "file"),
		"#<Encoding:UTF-8>\n#<Encoding:BINARY (ASCII-8BIT)>\n#<Encoding:BINARY (ASCII-8BIT)>\n#<Encoding:BINARY (ASCII-8BIT)>\n#<Encoding:BINARY (ASCII-8BIT)>\n\"\"\nnil\n")
}

// TestFileReadWorksInAnAsyncCall proves a worker reads the files its
// program may.
func TestFileReadWorksInAnAsyncCall(t *testing.T) {
	src := `include Cameo
class Reader
  def run(name) = File.read(name).size
end
p await(async(Reader.new, :run, "notes.txt"))
`
	assert.Equals(t, runWithFiles(t, src, testFiles(t), "/", "file"), "8\n")
}
