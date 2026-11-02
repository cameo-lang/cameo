package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestScriptsRunByTheCommandReachTheNetwork proves a script the command
// runs gets a client for net/http, where a program a host loads gets none.
func TestScriptsRunByTheCommandReachTheNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "from the server")
	}))
	t.Cleanup(srv.Close)
	script := filepath.Join(t.TempDir(), "fetch.cameo")
	src := "require \"net/http\"\nputs Net::HTTP.get(URI(\"" + srv.URL + "/x\"))\n"
	if err := os.WriteFile(script, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if status := runScript(script, false, false, &stdout, &stderr); status != 0 {
		t.Fatalf("exit status %d, stderr:\n%s", status, stderr.String())
	}
	if stdout.String() != "from the server\n" {
		t.Fatalf("output %q", stdout.String())
	}
}
