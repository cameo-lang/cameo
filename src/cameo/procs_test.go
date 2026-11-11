package cameo

import (
	"io"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

const greeterScript = `class Greeter
  string :greeting
  def to(name) = ->(punct) { "#{greeting} #{name}#{punct}" }
end
`

// A host stores a Proc one program made and calls it in another: the
// bytes carry the code and the values the proc captured, its self among
// them, and the other program has the class.
func TestAHostEncodesAProcForAnotherProgram(t *testing.T) {
	writer, err := Load("greeter.cameo", greeterScript, io.Discard, io.Discard)
	assert.NoError(t, err)
	assert.NoError(t, writer.Run())
	greeter, err := writer.Call("Greeter", "new", map[string]any{"greeting": "hello"})
	assert.NoError(t, err)
	block, err := writer.Call(greeter, "to", "ann")
	assert.NoError(t, err)
	_, isBlock := block.(*Block)
	assert.True(t, isBlock, "the method returned %T", block)
	data, err := writer.Encode(block)
	assert.NoError(t, err)

	reader, err := Load("greeter.cameo", greeterScript, io.Discard, io.Discard)
	assert.NoError(t, err)
	assert.NoError(t, reader.Run())
	decoded, err := reader.Decode(data)
	assert.NoError(t, err)
	out, err := decoded.(*Block).Call("!")
	assert.NoError(t, err)
	assert.Equals(t, out, "hello ann!")

	// A program without the class refuses the proc, and says which class.
	empty, err := Load("empty.cameo", "", io.Discard, io.Discard)
	assert.NoError(t, err)
	assert.NoError(t, empty.Run())
	_, err = empty.Decode(data)
	assert.True(t, err != nil, "a program without Greeter decoded the proc")
	assert.Contains(t, err.Error(), "Greeter")
}
