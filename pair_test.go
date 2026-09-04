package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Regression guard for a bug actually hit in real use — payload.Host was emitted with the port
// included ("IP:port"), but since Android's PcSyncClient itself appends a fixed port
// (PC_SYNC_PORT) on top of that, the final address doubled up as "IP:port:port" and threw a
// MalformedURLException (see .docs/SYNC_MULTIUSER_PLAN.md stage 6). Verifies that the host value
// actually shown in the /pair response has no colon (port separator).
func TestHandlePair_HostFieldHasNoPort(t *testing.T) {
	state := newAppState(Config{FolderPath: t.TempDir(), Secret: "test-secret"})
	req := httptest.NewRequest(http.MethodGet, "/pair", nil)
	rec := httptest.NewRecorder()

	handlePair(rec, req, state, "AA:BB:CC:DD")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	const marker = "Address: <code>"
	start := strings.Index(body, marker)
	if start == -1 {
		t.Fatalf("could not find the address in the response HTML:\n%s", body)
	}
	start += len(marker)
	end := strings.Index(body[start:], "</code>")
	if end == -1 {
		t.Fatal("address code block is not closed")
	}
	host := body[start : start+end]

	if host == "" {
		t.Error("host is empty")
	}
	if strings.Contains(host, ":") {
		t.Errorf("host must not include a port (PcSyncClient appends its own fixed port): %q", host)
	}
}
