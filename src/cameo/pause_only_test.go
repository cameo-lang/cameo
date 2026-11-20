package cameo

import (
	"github.com/cameo-lang/cameo/src/cameoio"

	"bytes"
	"errors"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/ir"
	"github.com/cameo-lang/cameo/src/syntax"
)

// boxScript pauses holding a Box whose proc field returns says, and
// requires abbrev, a library written in cameo, whose code the dump holds.
// Its method handler returns a proc that returns says.
func boxScript(says string) string {
	return `require "abbrev"
class Box
  proc :f
end
box = Box.new(f: proc { "` + says + `" })
def handler = proc { "` + says + `" }
Test.pause
puts box.f.call, Abbrev.abbrev(["car"]).size
`
}

// pausedDump runs src to its pause and returns the dump's message.
func pausedDump(t *testing.T, src string) *cameopb.Dump {
	t.Helper()
	_, d := pausedProgram(t, src)
	return d
}

// pausedProgram runs src to its pause and returns the program and its
// dump's message.
func pausedProgram(t *testing.T, src string) (*Program, *cameopb.Dump) {
	t.Helper()
	prog, _ := pausing(t, src)
	err := prog.Run()
	assert.True(t, errors.Is(err, ErrPaused), "Run returned %v, want ErrPaused", err)
	dump, err := prog.Dump()
	assert.NoError(t, err)
	d, err := readDump(dump.Data)
	assert.NoError(t, err)
	return prog, d
}

// resumeRun resumes the dump d with opts and runs it, and returns its
// output.
func resumeRun(t *testing.T, d *cameopb.Dump, opts ...ResumeOption) (string, error) {
	t.Helper()
	data, err := proto.Marshal(d)
	assert.NoError(t, err)
	var out bytes.Buffer
	prog, err := Resume(data, cameoio.Streams(&out, &out), opts...)
	if err != nil {
		return "", err
	}
	err = prog.Run()
	return out.String(), err
}

// A dump of the script OnlyCode names resumes, with the code of the
// library it required and the closure in an object's fields.
func TestOnlyCodeResumesTheScriptItNames(t *testing.T) {
	src := boxScript("own")
	out, err := resumeRun(t, pausedDump(t, src), OnlyCode("t.cameo", src))
	assert.NoError(t, err)
	assert.Equals(t, out, "own\n3\n")
}

// A script with the file name of a library it requires resumes: the
// library's unit is checked against the library, not the script.
func TestOnlyCodeResumesAScriptNamedAsALibrary(t *testing.T) {
	src := boxScript("own")
	prog, _ := pausingAs(t, "abbrev.cameo", src)
	err := prog.Run()
	assert.True(t, errors.Is(err, ErrPaused), "Run returned %v, want ErrPaused", err)
	dump, err := prog.Dump()
	assert.NoError(t, err)
	var resumed bytes.Buffer
	cont, err := Resume(dump.Data, cameoio.Streams(&resumed, &resumed), OnlyCode("abbrev.cameo", src))
	assert.NoError(t, err)
	assert.NoError(t, cont.Run())
	assert.Equals(t, resumed.String(), "own\n3\n")
}

// A dump of another script, or of the same script under another name, is
// refused before anything of it is read.
func TestOnlyCodeRefusesADumpOfAnotherScript(t *testing.T) {
	d := pausedDump(t, boxScript("other"))
	_, err := resumeRun(t, d, OnlyCode("t.cameo", boxScript("own")))
	assert.True(t, errors.Is(err, ErrOtherCode), "Resume returned %v, want ErrOtherCode", err)
	_, err = resumeRun(t, d, OnlyCode("u.cameo", boxScript("other")))
	assert.True(t, errors.Is(err, ErrOtherCode), "Resume returned %v, want ErrOtherCode", err)
}

// A unit of code in the dump's list that compiles from neither the script
// nor a library is refused, also when it names the file of either.
func TestOnlyCodeRefusesAUnitOfOtherCode(t *testing.T) {
	src := boxScript("own")
	for _, file := range []string{"t.cameo", "abbrev.cameo", "other.cameo"} {
		d := pausedDump(t, src)
		prog, err := syntax.Parse(file, `puts "other"`)
		assert.NoError(t, err)
		unit, err := ir.Compile(file, prog)
		assert.NoError(t, err)
		d.Units = append(d.Units, unit)
		_, err = resumeRun(t, d)
		assert.NoError(t, err)
		_, err = resumeRun(t, d, OnlyCode("t.cameo", src))
		assert.True(t, errors.Is(err, ErrOtherCode), "Resume of a unit of %s returned %v, want ErrOtherCode", file, err)
	}
}

// A dump crafted to carry other code in a closure inside an object's
// fields, which the dump keeps as the encoded message of the object's
// class, is refused: the unit is checked where the closure is decoded.
func TestOnlyCodeRefusesAClosureInAnObjectsFields(t *testing.T) {
	src := boxScript("own")
	d := pausedDump(t, src)
	prog, other := pausedProgram(t, boxScript("other"))
	var box *cameopb.Dump_Binding
	for _, b := range other.Variables {
		if b.Name == "box" {
			box = b
		}
	}
	assert.True(t, box != nil, "the other dump has no variable box")
	// The closure in the Box's field carries the other script's unit, so
	// the dump's list of units holds only the code the script ships.
	obj := box.Value.GetObject()
	md, err := prog.Descriptor("Box")
	assert.NoError(t, err)
	fields := dynamicpb.NewMessage(md)
	assert.NoError(t, proto.Unmarshal(obj.Message, fields))
	fd := md.Fields().ByName("f")
	raw, err := proto.Marshal(fields.Get(fd).Message().Interface())
	assert.NoError(t, err)
	closure := &cameopb.Closure{}
	assert.NoError(t, proto.Unmarshal(raw, closure))
	for _, unit := range other.Units {
		if unit.GetDigest() == closure.GetUnit() {
			closure.Code = unit
		}
	}
	assert.True(t, closure.Code != nil, "the other dump has no unit %s", closure.GetUnit())
	raw, err = proto.Marshal(closure)
	assert.NoError(t, err)
	carried := fields.Get(fd).Message().New()
	assert.NoError(t, proto.Unmarshal(raw, carried.Interface()))
	fields.Set(fd, protoreflect.ValueOfMessage(carried))
	obj.Message, err = proto.Marshal(fields)
	assert.NoError(t, err)
	for i, b := range d.Variables {
		if b.Name == "box" {
			d.Variables[i] = box
		}
	}

	// The crafted dump runs the other code where nothing checks it.
	out, err := resumeRun(t, d)
	assert.NoError(t, err)
	assert.Equals(t, out, "other\n3\n")

	_, err = resumeRun(t, d, OnlyCode("t.cameo", src))
	assert.True(t, errors.Is(err, ErrOtherCode), "Resume returned %v, want ErrOtherCode", err)
}

// A program resumed with OnlyCode refuses a proc of other code in a
// message the host decodes later, and takes one of its own script.
func TestOnlyCodeRefusesAProcOfOtherCodeInAMessage(t *testing.T) {
	src := boxScript("own")
	d := pausedDump(t, src)
	data, err := proto.Marshal(d)
	assert.NoError(t, err)
	var out bytes.Buffer
	cont, err := Resume(data, cameoio.Streams(&out, &out), OnlyCode("t.cameo", src))
	assert.NoError(t, err)

	encode := func(file, src string) []byte {
		writer, err := LoadFromString(file, src, cameoio.Streams(&out, &out))
		assert.NoError(t, err)
		writer.Module("Test").Def("pause", func([]any, map[string]any, *Block) (any, error) { return nil, nil })
		assert.NoError(t, writer.Run())
		block, err := writer.Call(nil, "handler")
		assert.NoError(t, err)
		msg, err := writer.Encode(block)
		assert.NoError(t, err)
		return msg
	}
	_, err = cont.Decode(encode("t.cameo", boxScript("other")))
	assert.True(t, errors.Is(err, ErrOtherCode), "Decode returned %v, want ErrOtherCode", err)
	_, err = cont.Decode(encode("t.cameo", src))
	assert.NoError(t, err)
}
