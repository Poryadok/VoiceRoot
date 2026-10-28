//go:build linux && live

package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
)

func logComposeSpaceLeaveDiagnostic(t *testing.T, resp *http.Response, requestID string) {
	t.Helper()
	d := composeSpaceLeavePublicDiagnostic(resp.Body, resp.StatusCode, requestID)
	raw, _ := json.Marshal(d) // All output fields are primitive and closed/locally validated.
	t.Logf("VOICE503_DIAGNOSTIC %s", raw)
}

func TestComposeSpaceLeaveDiagnosticPreservesStrictStatus(t *testing.T) {
	for _, mode := range []string{"204", "503"} {
		t.Run(mode, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestComposeSpaceLeaveDiagnosticProbe$", "-test.v", "-test.count=1")
			cmd.Env = append(os.Environ(), "VOICE_LEAVE_DIAGNOSTIC_PROBE="+mode)
			raw, err := cmd.CombinedOutput()
			output := string(raw)
			if !strings.Contains(output, "LEAVE_PROBE_REQUESTS=1") {
				t.Fatal("real leave helper did not perform exactly one request")
			}
			for _, canary := range []string{"request-secret-canary", "body-secret-canary", "header-secret-canary", "postgres://"} {
				if strings.Contains(output, canary) {
					t.Fatal("private fixture data escaped the real helper diagnostic")
				}
			}
			const marker = "VOICE503_DIAGNOSTIC "
			if mode == "204" {
				if err != nil || strings.Contains(output, marker) {
					t.Fatal("successful leave changed status or emitted a failure diagnostic")
				}
				return
			}
			if err == nil {
				t.Fatal("real leave helper no longer rejects non204")
			}
			at := strings.Index(output, marker)
			if at < 0 {
				t.Fatal("strict leave failure omitted its safe diagnostic")
			}
			line := strings.SplitN(output[at+len(marker):], "\n", 2)[0]
			var d composeSpaceLeaveDiagnostic
			if json.Unmarshal([]byte(line), &d) != nil || d.Status != 503 || d.PublicError != "auth_unavailable" || d.Message != "redacted" || d.BodyResult != "parsed" || !strings.HasPrefix(d.RequestID, "voice503-leave-") {
				t.Fatal("real leave failure lost status/classification/request correlation")
			}
			requestHash := fmt.Sprintf("%x", sha256.Sum256([]byte(d.RequestID)))
			if !strings.Contains(output, "LEAVE_PROBE_REQUEST_ID_SHA256="+requestHash) {
				t.Fatal("logged correlation differs from the actual public request")
			}
		})
	}
}

// A separate process observes the actual testing.T failure from the unchanged
// strict204 helper without converting that failure into a passing assertion.
func TestComposeSpaceLeaveDiagnosticProbe(t *testing.T) {
	mode := os.Getenv("VOICE_LEAVE_DIAGNOSTIC_PROBE")
	if mode == "" {
		t.Skip("subprocess-only real public leave helper probe")
	}
	var requests atomic.Int32
	var requestID atomic.Value
	requestID.Store("")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		requestID.Store(r.Header.Get("X-Request-Id"))
		if !strings.HasPrefix(r.Header.Get("X-Request-Id"), "voice503-leave-") {
			t.Error("real leave request correlation missing")
		}
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/voice/rooms/fixture-room/leave" || r.Header.Get("Authorization") != "Bearer request-secret-canary" {
			t.Error("real leave request shape changed")
		}
		w.Header().Set("X-Request-Id", "header-secret-canary")
		if mode == "204" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"auth_unavailable","message":"body-secret-canary","extra":"postgres://body-secret-canary"}`))
	}))
	defer server.Close()
	t.Cleanup(func() {
		t.Logf("LEAVE_PROBE_REQUESTS=%d", requests.Load())
		t.Logf("LEAVE_PROBE_REQUEST_ID_SHA256=%x", sha256.Sum256([]byte(requestID.Load().(string))))
	})
	leaveComposeSpaceVoiceRoom(t, server.Client(), server.URL, "request-secret-canary", "fixture-room")
}
