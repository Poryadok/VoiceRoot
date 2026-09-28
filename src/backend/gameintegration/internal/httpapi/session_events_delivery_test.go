package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/registry"
)

type deliverySessionCredentialVerifier struct {
	principal registry.SessionPrincipal
	err       error
}

func (v deliverySessionCredentialVerifier) VerifyGameServer(*http.Request) (registry.SessionPrincipal, error) {
	return v.principal, v.err
}

// This structural fake is the HTTP layer's expected claim/ACK seam. It embeds
// the existing session stub so the test exercises the same session handler
// while consumer delivery routes are introduced.
type sessionEventDeliveryRouteFake struct {
	*sessionOrchestratorStub
	claim            *registry.SessionEventClaim
	claimErr         error
	ackErr           error
	ackCalls         int
	ackedEvent       uuid.UUID
	ackedLease       uuid.UUID
	ackedHash        []byte
	eventApplication uuid.UUID
	eventEnvironment uuid.UUID
}

func (f *sessionEventDeliveryRouteFake) ClaimSessionEvent(context.Context, registry.SessionPrincipal) (*registry.SessionEventClaim, error) {
	return f.claim, f.claimErr
}

func (f *sessionEventDeliveryRouteFake) AckSessionEvent(_ context.Context, principal registry.SessionPrincipal, eventID, leaseID uuid.UUID, digest []byte) error {
	f.ackCalls++
	f.ackedEvent, f.ackedLease, f.ackedHash = eventID, leaseID, append([]byte(nil), digest...)
	if f.eventApplication != uuid.Nil && (principal.ApplicationID != f.eventApplication || principal.EnvironmentID != f.eventEnvironment) {
		return registry.ErrSessionNotFound
	}
	return f.ackErr
}

func TestSessionEventClaimHTTPReturnsEmptyPollContractAndRequiresCurrentCredentialScope(t *testing.T) {
	validPrincipal := registry.SessionPrincipal{
		ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Scopes: []string{"game.sessions.manage"},
	}
	newHandler := func(verifier SessionCredentialVerifier) http.Handler {
		return NewSessionHandler(verifier, &sessionEventDeliveryRouteFake{sessionOrchestratorStub: &sessionOrchestratorStub{}})
	}
	request := func(handler http.Handler, authorization string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/session-events/claim", nil)
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	for _, tc := range []struct {
		name          string
		authorization string
		err           error
	}{
		{name: "missing credential", err: context.Canceled},
		{name: "expired credential", authorization: "Bearer expired-game-server", err: context.DeadlineExceeded},
		{name: "revoked credential", authorization: "Bearer revoked-game-server", err: context.Canceled},
	} {
		t.Run(tc.name+" is unauthorized", func(t *testing.T) {
			handler := newHandler(deliverySessionCredentialVerifier{err: tc.err})
			rec := request(handler, tc.authorization)
			require.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}

	t.Run("credential without session management scope is forbidden", func(t *testing.T) {
		principal := validPrincipal
		principal.Scopes = []string{"game.events.write"}
		handler := newHandler(deliverySessionCredentialVerifier{principal: principal})
		rec := request(handler, "Bearer valid-but-insufficient")
		require.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("empty scoped outbox returns no content and one second retry hint", func(t *testing.T) {
		handler := newHandler(deliverySessionCredentialVerifier{principal: validPrincipal})
		rec := request(handler, "Bearer current-game-server")
		require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
		require.Equal(t, "1", rec.Header().Get("Retry-After"))
		require.Empty(t, rec.Body.Bytes())
	})

	// A claimed response is checked in the registry integration test against the
	// persisted body bytes and digest. Here the route must remain an opaque body
	// transport; Gateway tests separately guard the bytes over the public proxy.
}

func TestSessionEventClaimHTTPReturnsExactStoredBytesAndDeliveryHeaders(t *testing.T) {
	principal := registry.SessionPrincipal{ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Scopes: []string{"game.sessions.manage"}}
	eventID := uuid.MustParse("00000000-0000-4000-8000-000000000002")
	leaseID := uuid.MustParse("00000000-0000-4000-8000-000000000003")
	expiresAt := time.Date(2026, time.September, 28, 12, 0, 30, 0, time.UTC)
	body := []byte(`{"event_id":"00000000-0000-4000-8000-000000000002","application_id":"` + principal.ApplicationID.String() +
		`","environment_id":"` + principal.EnvironmentID.String() + `","session_id":"00000000-0000-4000-8000-000000000004","operation_id":"00000000-0000-4000-8000-000000000005","kind":"party","active_at":"2026-09-28T12:00:00Z"}`)
	digest := sha256.Sum256(body)
	orchestrator := &sessionEventDeliveryRouteFake{
		sessionOrchestratorStub: &sessionOrchestratorStub{},
		claim: &registry.SessionEventClaim{
			EventID: eventID, SessionID: uuid.MustParse("00000000-0000-4000-8000-000000000004"),
			PayloadBytes: body, PayloadSHA256: digest[:], LeaseID: leaseID, LeaseExpiresAt: expiresAt,
		},
	}
	handler := NewSessionHandler(deliverySessionCredentialVerifier{principal: principal}, orchestrator)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/session-events/claim", nil)
	req.Header.Set("Authorization", "Bearer current-game-server")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, body, rec.Body.Bytes(), "claim response is the exact persisted seven-field payload, without an envelope")
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	require.Equal(t, "identity", rec.Header().Get("Content-Encoding"))
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	require.Equal(t, eventID.String(), rec.Header().Get("X-Voice-Event-Id"))
	require.Equal(t, hex.EncodeToString(digest[:]), rec.Header().Get("X-Voice-Payload-SHA256"))
	require.Equal(t, leaseID.String(), rec.Header().Get("X-Voice-Claim-Lease-Id"))
	require.Equal(t, expiresAt.Format(time.RFC3339), rec.Header().Get("X-Voice-Claim-Lease-Expires-At"))
}

func TestSessionEventAckHTTPUnknownOrForeignEventUsesNotFoundContract(t *testing.T) {
	principal := registry.SessionPrincipal{
		ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Scopes: []string{"game.sessions.manage"},
	}
	orchestrator := &sessionEventDeliveryRouteFake{sessionOrchestratorStub: &sessionOrchestratorStub{}, ackErr: registry.ErrSessionNotFound}
	handler := NewSessionHandler(deliverySessionCredentialVerifier{principal: principal}, orchestrator)
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/session-events/00000000-0000-4000-8000-000000000009/ack",
		bytes.NewBufferString(`{"lease_id":"00000000-0000-4000-8000-000000000001","payload_sha256":"`+strings.Repeat("a", 64)+`"}`))
	req.Header.Set("Authorization", "Bearer current-game-server")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "NOT_FOUND")
}

func TestSessionEventAckHTTPMapsDigestMismatchAndStaleLeaseToConflict(t *testing.T) {
	principal := registry.SessionPrincipal{ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Scopes: []string{"game.sessions.manage"}}
	for _, tc := range []struct{ name, digest string }{
		{name: "payload digest mismatch", digest: strings.Repeat("a", 64)},
		{name: "expired or superseded lease", digest: strings.Repeat("b", 64)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			orchestrator := &sessionEventDeliveryRouteFake{sessionOrchestratorStub: &sessionOrchestratorStub{}, ackErr: registry.ErrIdempotencyConflict}
			handler := NewSessionHandler(deliverySessionCredentialVerifier{principal: principal}, orchestrator)
			eventID := uuid.MustParse("00000000-0000-4000-8000-000000000009")
			leaseID := uuid.MustParse("00000000-0000-4000-8000-000000000001")
			req := httptest.NewRequest(http.MethodPost, "/api/v1/session-events/"+eventID.String()+"/ack",
				strings.NewReader(`{"lease_id":"`+leaseID.String()+`","payload_sha256":"`+tc.digest+`"}`))
			req.Header.Set("Authorization", "Bearer current-game-server")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
			require.Equal(t, 1, orchestrator.ackCalls)
			require.Equal(t, eventID, orchestrator.ackedEvent)
			require.Equal(t, leaseID, orchestrator.ackedLease)
			require.Equal(t, tc.digest, hex.EncodeToString(orchestrator.ackedHash))
		})
	}
}

func TestSessionEventAckHTTPForeignApplicationAndEnvironmentAreNotFound(t *testing.T) {
	ownerApplication, ownerEnvironment := uuid.New(), uuid.New()
	for _, tc := range []struct {
		name string
		app  uuid.UUID
		env  uuid.UUID
	}{
		{name: "foreign application", app: uuid.New(), env: ownerEnvironment},
		{name: "foreign environment", app: ownerApplication, env: uuid.New()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			principal := registry.SessionPrincipal{ApplicationID: tc.app, EnvironmentID: tc.env, Scopes: []string{"game.sessions.manage"}}
			orchestrator := &sessionEventDeliveryRouteFake{
				sessionOrchestratorStub: &sessionOrchestratorStub{},
				eventApplication:        ownerApplication, eventEnvironment: ownerEnvironment,
			}
			handler := NewSessionHandler(deliverySessionCredentialVerifier{principal: principal}, orchestrator)
			req := httptest.NewRequest(http.MethodPost,
				"/api/v1/session-events/00000000-0000-4000-8000-000000000009/ack",
				strings.NewReader(`{"lease_id":"00000000-0000-4000-8000-000000000001","payload_sha256":"`+strings.Repeat("a", 64)+`"}`))
			req.Header.Set("Authorization", "Bearer current-game-server")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			require.Equal(t, http.StatusNotFound, rec.Code)
			require.Contains(t, rec.Body.String(), "NOT_FOUND")
		})
	}
}
