package tests

import (
	"os"
	"testing"
	"time"
)

// TestMain runs the tests in UTC, as make does for every target: a test
// that holds cameo to what Ruby prints for a local time must not depend on
// the machine's zone, and Go's test cache does not key on TZ, so a direct
// `go test ./tests` pins the zone itself.
func TestMain(m *testing.M) {
	os.Setenv("TZ", "UTC")
	time.Local = time.UTC
	code := m.Run()
	if builtCameo.dir != "" {
		os.RemoveAll(builtCameo.dir)
	}
	os.Exit(code)
}
