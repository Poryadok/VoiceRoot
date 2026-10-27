package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Only closed public classifications and a locally generated ID may reach CI logs.
type composeSpaceLeaveDiagnostic struct {
	Phase       string `json:"phase"`
	RequestID   string `json:"request_id"`
	Status      int    `json:"status"`
	BodyResult  string `json:"body_result"`
	PublicError string `json:"public_error"`
	Message     string `json:"message"`
}

func composeSpaceLeavePublicDiagnostic(body io.Reader, status int, requestID string) composeSpaceLeaveDiagnostic {
	d := composeSpaceLeaveDiagnostic{Phase: "deferred_public_leave", RequestID: "redacted", Status: status,
		BodyResult: "read_failed", PublicError: "redacted", Message: "redacted"}
	const prefix = "voice503-leave-"
	if strings.HasPrefix(requestID, prefix) && len(requestID) > len(prefix) && len(requestID) <= 40 {
		valid := true
		for _, ch := range strings.TrimPrefix(requestID, prefix) {
			if ch < '0' || ch > '9' {
				valid = false
			}
		}
		if valid {
			d.RequestID = requestID
		}
	}
	if body == nil {
		return d
	}
	raw, err := io.ReadAll(io.LimitReader(body, 4097))
	if err != nil {
		return d
	}
	if len(raw) > 4096 {
		d.BodyResult = "over_limit"
		return d
	}
	var public struct {
		Error   string `json:"error"`
		Code    string `json:"error_code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &public) != nil {
		d.BodyResult = "invalid_json"
		return d
	}
	d.BodyResult = "parsed"
	code := public.Code
	if code == "" {
		code = public.Error
	}
	switch code {
	case "auth_unavailable", "invalid_token", "token_revoked", "unavailable", "unauthenticated",
		"permission_denied", "internal", "not_found", "invalid_argument", "failed_precondition",
		"deadline_exceeded", "aborted", "resource_exhausted":
		d.PublicError = code
	case "":
		d.PublicError = "absent"
	}
	switch public.Message {
	case "Space media admission is not confirmed", "Space media revocation is not ready",
		"Space media participant removal is pending", "Space media admission fence is unavailable",
		"Space media admission fence is invalid", "Space media admission fence release failed",
		"voice persistence not configured", "Voice user-principal transport unavailable",
		"voice credential unavailable", "Space principal verifier unavailable", "invalid principal",
		"authenticated Voice user method unavailable on ordinary listener",
		"method unavailable on Voice user listener", "verified voice actor required",
		"internal error", "voice_room_id is required", "invalid voice request":
		d.Message = public.Message
	case "":
		d.Message = "absent"
	}
	return d
}

func logComposeSpaceLeaveDiagnostic(t *testing.T, resp *http.Response, requestID string) {
	t.Helper()
	d := composeSpaceLeavePublicDiagnostic(resp.Body, resp.StatusCode, requestID)
	raw, _ := json.Marshal(d) // All output fields are primitive and closed/locally validated.
	t.Logf("VOICE503_DIAGNOSTIC %s", raw)
}

func TestComposeSpaceLeaveDiagnosticAllowsOnlyPublicInventory(t *testing.T) {
	for _, tc := range []struct{ body, code, message string }{
		{`{"error":"auth_unavailable"}`, "auth_unavailable", "absent"},
		{`{"error_code":"unavailable","message":"Space media participant removal is pending"}`, "unavailable", "Space media participant removal is pending"},
	} {
		d := composeSpaceLeavePublicDiagnostic(strings.NewReader(tc.body), 503, "voice503-leave-123")
		if d.Phase != "deferred_public_leave" || d.Status != 503 || d.BodyResult != "parsed" || d.PublicError != tc.code || d.Message != tc.message || d.RequestID != "voice503-leave-123" {
			t.Fatalf("closed public inventory missing: %+v", d)
		}
	}
}

type composeLeaveFailingReader struct{}

func (composeLeaveFailingReader) Read([]byte) (int, error) {
	return 0, errors.New("secret-reader-canary")
}

func TestComposeSpaceLeaveDiagnosticRejectsUnsafeInput(t *testing.T) {
	const canary = "Bearer secret-jwt-canary postgres://secret-dsn-canary"
	for _, tc := range []struct {
		name string
		body io.Reader
		want string
	}{
		{"unknown", strings.NewReader(`{"error_code":"` + canary + `","message":"` + canary + `","extra":"` + canary + `"}`), "parsed"},
		{"malformed", strings.NewReader(canary), "invalid_json"},
		{"array", strings.NewReader(`[]`), "invalid_json"},
		{"trailing", strings.NewReader(`{} {}`), "invalid_json"},
		{"oversized", strings.NewReader(strings.Repeat("x", 4097) + canary), "over_limit"},
		{"reader_error", composeLeaveFailingReader{}, "read_failed"},
		{"nil", nil, "read_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := composeSpaceLeavePublicDiagnostic(tc.body, 503, canary)
			raw, err := json.Marshal(d)
			if err != nil || d.BodyResult != tc.want || d.PublicError != "redacted" || d.Message != "redacted" || d.RequestID != "redacted" || strings.Contains(string(raw), "canary") || len(raw) > 800 {
				t.Fatalf("unsafe diagnostic shape: result=%s bytes=%d", d.BodyResult, len(raw))
			}
		})
	}
	for _, requestID := range []string{"voice503-leave-", "voice503-leave-12\n", "voice503-leave-123secret", strings.Repeat("1", 41)} {
		d := composeSpaceLeavePublicDiagnostic(strings.NewReader(`{}`), 503, requestID)
		if d.RequestID != "redacted" {
			t.Fatal("untrusted correlation ID escaped validation")
		}
	}
}

type composeLeaveCountingReader struct{ bytes int }

func (r *composeLeaveCountingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	r.bytes += len(p)
	return len(p), nil
}

func TestComposeSpaceLeaveDiagnosticBoundsBodyRead(t *testing.T) {
	r := &composeLeaveCountingReader{}
	d := composeSpaceLeavePublicDiagnostic(r, 503, "voice503-leave-123")
	if r.bytes != 4097 || d.BodyResult != "over_limit" {
		t.Fatalf("diagnostic read was not bounded: bytes=%d result=%s", r.bytes, d.BodyResult)
	}
}
