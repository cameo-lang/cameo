package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// A lazy grep takes its pattern when it is called, as an eager one does.
// Without one the call is an ArgumentError; it was accepted, and the
// enumerator ended the process when it ran.
func TestALazyGrepTakesItsPattern(t *testing.T) {
	out := runScript(t, `
[-> { [1, 2].lazy.grep }, -> { [1, 2].lazy.grep_v(1, 2) }].each do |f|
  f.call
rescue ArgumentError => e
  puts e.message
end
p [1, "a", 2].lazy.grep(Integer).to_a, [1, "a", 2].lazy.grep_v(Integer).first(1)
`)
	assert.Equals(t, out, "wrong number of arguments (given 0, expected 1)\nwrong number of arguments (given 2, expected 1)\n[1, 2]\n[\"a\"]\n")
}
