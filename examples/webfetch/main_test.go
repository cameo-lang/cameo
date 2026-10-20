package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchAllFetchesAtOnce(t *testing.T) {
	// Each page waits until all three requests have arrived, so the run
	// finishes only if every workflow's fetch is in flight at once.
	var arrived atomic.Int32
	all := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if arrived.Add(1) == 3 {
			close(all)
		}
		select {
		case <-all:
		case <-time.After(5 * time.Second):
			http.Error(w, "the requests came one at a time", http.StatusGatewayTimeout)
			return
		}
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(strings.Repeat("x", len(r.URL.Path))))
	}))
	defer srv.Close()

	lines, err := fetchAll(srv.Client(), []string{srv.URL + "/a", srv.URL + "/bcd", srv.URL + "/missing"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{srv.URL + "/a: 2 bytes, 1 lines", srv.URL + "/bcd: 4 bytes, 1 lines", srv.URL + "/missing: 404 Not Found"}
	if len(lines) != len(want) {
		t.Fatalf("lines %q", lines)
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line %d: %q, want %q", i, lines[i], w)
		}
	}
}
