package main

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"switcher/internal/fork"
	"switcher/internal/server"
)

// Paired Macs judge what a fork host can do by the upstream release it is
// built on, which it reports beside its own number.
func TestTheUpstreamBaseIsReported(t *testing.T) {
	body, err := os.ReadFile("VERSION")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(string(body))
	if fork.UpstreamVersion != want {
		t.Fatalf("fork.UpstreamVersion = %q, VERSION is %q", fork.UpstreamVersion, want)
	}
	if got := (&server.API{Version: "1.0.0"}).LocalStateFields(httptest.NewRequest("GET", "/api/state", nil))["upstream_version"]; got != want {
		t.Errorf("state reports upstream_version %v, want %s", got, want)
	}
}
