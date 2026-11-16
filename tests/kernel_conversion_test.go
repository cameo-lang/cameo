package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// Integer() and Float() with exception: false return nil for a value that
// does not convert, as Ruby does: a string that is no number, nil, a value
// of a type with no conversion, a float that is no integer, and a value
// whose own to_int raises. A value that converts converts as without the
// keyword.
func TestIntegerAndFloatReturnNilWithExceptionFalse(t *testing.T) {
	out := runScript(t, `
class Broken
  def to_int = raise(ArgumentError, "boom")
end
p Integer("12", exception: false), Integer("x", exception: false), Float("1.5", exception: false), Float("x", exception: false)
p Integer(" 0x1A ", exception: false), Integer(2.7, exception: false), Integer(nil, exception: false), Integer([1], exception: false)
p Integer(Float::NAN, exception: false), Integer(Broken.new, exception: false)
p Float(3, exception: false), Float(nil, exception: false), Float(:a, exception: false), Float([1], exception: false)
`)
	assert.Equals(t, out, `12
nil
1.5
nil
26
2
nil
nil
nil
nil
3.0
nil
nil
nil
`)
}

// Integer() and Float() with exception: true raise as without the keyword,
// and refuse an exception: that is neither true nor false and any other
// keyword, with Ruby's messages.
func TestIntegerAndFloatCheckTheExceptionKeyword(t *testing.T) {
	out := runScript(t, `
[
  -> { Integer("x", exception: true) },
  -> { Float("x", exception: true) },
  -> { Integer(nil) },
  -> { Float(nil) },
  -> { Integer("x", exception: nil) },
  -> { Float("x", exception: 1) },
  -> { Integer("1", foo: 1) },
  -> { Float("1", foo: 1) },
].each do |f|
  begin
    f.call
  rescue ArgumentError, TypeError => e
    puts "#{e.class}: #{e.message}"
  end
end
`)
	assert.Equals(t, out, `ArgumentError: invalid value for Integer(): "x"
ArgumentError: invalid value for Float(): "x"
TypeError: can't convert nil into Integer
TypeError: can't convert nil into Float
ArgumentError: expected true or false as exception: nil
ArgumentError: expected true or false as exception: 1
ArgumentError: unknown keyword: :foo
ArgumentError: unknown keyword: :foo
`)
}
