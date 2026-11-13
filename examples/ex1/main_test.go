package main

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The script that is built in uses the class Go defines, and Go calls the
// method the script defines.
func TestTheBuiltInScriptGreets(t *testing.T) {
	var out bytes.Buffer
	assert.NoError(t, run("hello.cameo", hello, &out, &out))
	assert.Equals(t, out.String(), "Hello, world!\nHELLO, cameo!\nHello, again!\nfrom Go: Welcome, Go!\n")
}

// A script of the user's own has the class too, and need not define
// welcome.
func TestAScriptOfItsOwnUsesTheClass(t *testing.T) {
	var out bytes.Buffer
	assert.NoError(t, run("mine.cameo", `puts Greeter.new(greeting: "Hi").louder.greet("there")`+"\n", &out, &out))
	assert.Equals(t, out.String(), "HI, there!\n")
}

// A wrong argument is an error of the script, with its line.
func TestAWrongArgumentIsAnErrorOfTheScript(t *testing.T) {
	var out bytes.Buffer
	err := run("mine.cameo", "Greeter.new(greeting: \"Hi\").greet(1)\n", &out, &out)
	assert.True(t, err != nil, "greet(1) ran")
	assert.Equals(t, err.Error(), "mine.cameo:1: greet takes a String, not int64 (RuntimeError)")
}
