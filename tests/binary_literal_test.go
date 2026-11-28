package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/cameoio"
)

// A string literal with bytes that are not UTF-8 is refused before the
// script runs, and the message names the literal to write.
func TestAStringLiteralOfOtherBytesIsRefused(t *testing.T) {
	for src, line := range map[string]string{`p "\xFF"`: "1", `p "a#{1}\xC3"`: "1", "puts 1\np \"\\xFF\"": "2"} {
		var stdout, stderr bytes.Buffer
		err := cameo.RunFromString("t.cameo", src+"\n", cameoio.Streams(&stdout, &stderr))
		assert.Error(t, err, "t.cameo:"+line+`: a string literal with bytes that are not UTF-8 is a binary string and is written b"..."`)
		assert.Equals(t, stdout.String(), "")
	}
}
