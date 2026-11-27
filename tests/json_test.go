package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/cameoio"
)

// runScript runs src and returns what it printed, failing the test when
// the script fails. Where a script is plain Ruby, the expected output is
// Ruby 3.4's.
func runScript(t *testing.T, src string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if err := cameo.RunFromString("t.cameo", src, cameoio.Streams(&stdout, &stderr)); err != nil {
		t.Fatalf("%s\nfailed: %v\nstderr: %s", src, err, stderr.String())
	}
	return stdout.String()
}
