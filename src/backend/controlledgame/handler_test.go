package controlledgame

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestCallbackHandlerRejectsNoncanonicalRouteAndMethod(t *testing.T) {
	handler := NewHandler(HandlerConfig{
		Credentials: map[string]SigningCredential{vectorKeyID: testCredential(testKey())},
		Clock:       func() time.Time { return time.Unix(1790500000, 0).UTC() },
	})
	for _, test := range []struct {
		name   string
		method string
		path   string
		status int
	}{
		{name: "percent-encoded path", method: http.MethodPost, path: "/callbacks/game-commands/%76%31", status: http.StatusNotFound},
		{name: "query", method: http.MethodPost, path: vectorPath + "?x=1", status: http.StatusNotFound},
		{name: "wrong method", method: http.MethodGet, path: vectorPath, status: http.StatusMethodNotAllowed},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, bytes.NewReader([]byte(canonicalCommand)))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			require.Equal(t, test.status, recorder.Code)
		})
	}
}

func TestCallbackHandlerRejectsDuplicateAuthenticationAndContentTypeHeaders(t *testing.T) {
	key := testKey()
	handler := NewHandler(HandlerConfig{
		Credentials: map[string]SigningCredential{vectorKeyID: testCredential(key)},
		Clock:       func() time.Time { return time.Unix(1790500000, 0).UTC() },
	})
	for _, header := range []string{"X-Voice-Signature", "X-Voice-Key-Id", "X-Voice-Timestamp", "Content-Type"} {
		t.Run(header, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, vectorPath, bytes.NewReader([]byte(canonicalCommand)))
			request.Header.Set("Content-Type", "application/vnd.voice.game-command+json;version=1")
			request.Header.Set("X-Voice-Key-Id", vectorKeyID)
			request.Header.Set("X-Voice-Timestamp", vectorTimestamp)
			request.Header.Set("X-Voice-Signature", testSignature(key, http.MethodPost, vectorPath,
				vectorTimestamp, vectorKeyID, []byte(canonicalCommand)))
			request.Header.Add(header, request.Header.Get(header))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}

func TestCallbackHandlerRejectsBodyOver64KiBAndAcceptsExactLimit(t *testing.T) {
	key := testKey()
	now := time.Unix(1790500000, 0).UTC()
	handler := NewHandler(HandlerConfig{Credentials: map[string]SigningCredential{vectorKeyID: testCredential(key)}, Clock: func() time.Time { return now }, Store: acceptingStore{clock: func() time.Time { return now }}, Apply: func(context.Context, pgx.Tx, []byte) ([]byte, error) { return []byte(canonicalResult), nil }})
	for _, test := range []struct {
		name   string
		body   []byte
		signed bool
		want   int
	}{
		{name: "one byte over", body: append([]byte(canonicalCommand), bytes.Repeat([]byte(" "), 64*1024-len(canonicalCommand)+1)...), signed: false, want: http.StatusRequestEntityTooLarge},
		{name: "exactly 64 KiB", body: []byte(strings.Replace(canonicalCommand, `"encounter-42"`, `"`+strings.Repeat("x", 64*1024-len(canonicalCommand)+len("encounter-42"))+`"`, 1)), signed: true, want: http.StatusAccepted},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, vectorPath, bytes.NewReader(test.body))
			request.Header.Set("Content-Type", callbackContentType)
			request.Header.Set("X-Voice-Key-Id", vectorKeyID)
			request.Header.Set("X-Voice-Timestamp", vectorTimestamp)
			if test.signed {
				request.Header.Set("X-Voice-Signature", testSignature(key, http.MethodPost, vectorPath, vectorTimestamp, vectorKeyID, test.body))
			} else {
				request.Header.Set("X-Voice-Signature", "invalid")
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			require.Equal(t, test.want, recorder.Code)
			if test.name == "exactly 64 KiB" {
				require.Equal(t, canonicalResult, recorder.Body.String())
			}
		})
	}
}

type acceptingStore struct {
	clock func() time.Time
}

func (store acceptingStore) accept(ctx context.Context, _, _ string, _, _ []byte, expiresAt int64, apply EffectApplier) ([]byte, bool, error) {
	if store.clock != nil && store.clock().UTC().Unix() >= expiresAt {
		return nil, false, errCommandExpired
	}
	receipt, err := apply(ctx, nil, []byte(canonicalCommand))
	return receipt, false, err
}

type replayingStore struct {
	calls   int
	receipt []byte
}

func (store *replayingStore) accept(context.Context, string, string, []byte, []byte, int64, EffectApplier) ([]byte, bool, error) {
	store.calls++
	return append([]byte(nil), store.receipt...), true, nil
}

func TestCallbackHandlerReturnsSavedReceiptForExpiredReplay(t *testing.T) {
	key := testKey()
	now := time.Unix(1790500120, 0).UTC()
	store := &replayingStore{receipt: []byte(canonicalResult)}
	applyCalls := 0
	handler := NewHandler(HandlerConfig{
		Store:       store,
		Credentials: map[string]SigningCredential{vectorKeyID: testCredential(key)},
		Clock:       func() time.Time { return now },
		Apply: func(context.Context, pgx.Tx, []byte) ([]byte, error) {
			applyCalls++
			return nil, nil
		},
	})
	request := httptest.NewRequest(http.MethodPost, vectorPath, bytes.NewReader([]byte(canonicalCommand)))
	request.Header.Set("Content-Type", callbackContentType)
	request.Header.Set("X-Voice-Key-Id", vectorKeyID)
	request.Header.Set("X-Voice-Timestamp", vectorTimestamp)
	request.Header.Set("X-Voice-Signature", testSignature(key, http.MethodPost, vectorPath, vectorTimestamp, vectorKeyID, []byte(canonicalCommand)))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusAccepted, recorder.Code, "an expired command may recover its committed receipt")
	require.Equal(t, canonicalResult, recorder.Body.String())
	require.Equal(t, 1, store.calls, "the durable store must decide replay before new admission expiry")
	require.Zero(t, applyCalls, "replay must not apply the effect again")
}

func TestCallbackHandlerRejectsCommandAtExpiryBoundary(t *testing.T) {
	key := testKey()
	now := time.Unix(1790500120, 0).UTC()
	command := []byte(canonicalCommand)
	handler := NewHandler(HandlerConfig{
		Store:       acceptingStore{clock: func() time.Time { return now }},
		Credentials: map[string]SigningCredential{vectorKeyID: testCredential(key)},
		Clock:       func() time.Time { return now },
		Apply:       func(context.Context, pgx.Tx, []byte) ([]byte, error) { return []byte(canonicalResult), nil },
	})
	request := httptest.NewRequest(http.MethodPost, vectorPath, bytes.NewReader(command))
	request.Header.Set("Content-Type", callbackContentType)
	request.Header.Set("X-Voice-Key-Id", vectorKeyID)
	request.Header.Set("X-Voice-Timestamp", vectorTimestamp)
	request.Header.Set("X-Voice-Signature", testSignature(key, http.MethodPost, vectorPath, vectorTimestamp, vectorKeyID, command))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusGone, recorder.Code, "now >= expires_at is expired")
}

func testCredential(key []byte) SigningCredential {
	return SigningCredential{KeyID: vectorKeyID, AppID: "00000000-0000-4000-8000-000000000005", EnvironmentID: "00000000-0000-4000-8000-000000000006", InstallationID: "00000000-0000-4000-8000-000000000007", Secret: key, Status: CredentialCurrent}
}

func TestCallbackHandlerScopesAndChecksProvisionedCredentials(t *testing.T) {
	key := testKey()
	now := time.Unix(1790500000, 0).UTC()
	valid := SigningCredential{KeyID: vectorKeyID, AppID: "00000000-0000-4000-8000-000000000005", EnvironmentID: "00000000-0000-4000-8000-000000000006", InstallationID: "00000000-0000-4000-8000-000000000007", Secret: key, Status: CredentialCurrent}
	for _, test := range []struct {
		name       string
		credential SigningCredential
		want       int
	}{
		{name: "app mismatch", credential: func() SigningCredential { c := valid; c.AppID = "00000000-0000-4000-8000-00000000000c"; return c }(), want: http.StatusUnauthorized},
		{name: "environment mismatch", credential: func() SigningCredential {
			c := valid
			c.EnvironmentID = "00000000-0000-4000-8000-00000000000c"
			return c
		}(), want: http.StatusUnauthorized},
		{name: "installation mismatch", credential: func() SigningCredential {
			c := valid
			c.InstallationID = "00000000-0000-4000-8000-00000000000c"
			return c
		}(), want: http.StatusUnauthorized},
		{name: "revoked", credential: func() SigningCredential { c := valid; c.Status = CredentialRevoked; return c }(), want: http.StatusUnauthorized},
		{name: "overlap expired", credential: func() SigningCredential { c := valid; c.Status = CredentialOverlap; c.NotAfter = now; return c }(), want: http.StatusUnauthorized},
		{name: "current", credential: valid, want: http.StatusInternalServerError},
		{name: "overlap active", credential: func() SigningCredential {
			c := valid
			c.Status = CredentialOverlap
			c.NotAfter = now.Add(time.Minute)
			return c
		}(), want: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := NewHandler(HandlerConfig{Credentials: map[string]SigningCredential{vectorKeyID: test.credential}, Clock: func() time.Time { return now }})
			request := httptest.NewRequest(http.MethodPost, vectorPath, bytes.NewReader([]byte(canonicalCommand)))
			request.Header.Set("Content-Type", callbackContentType)
			request.Header.Set("X-Voice-Key-Id", vectorKeyID)
			request.Header.Set("X-Voice-Timestamp", vectorTimestamp)
			request.Header.Set("X-Voice-Signature", testSignature(key, http.MethodPost, vectorPath, vectorTimestamp, vectorKeyID, []byte(canonicalCommand)))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			require.Equal(t, test.want, recorder.Code)
		})
	}
	t.Run("unknown key ID", func(t *testing.T) {
		handler := NewHandler(HandlerConfig{Credentials: map[string]SigningCredential{vectorKeyID: valid}, Clock: func() time.Time { return now }})
		request := httptest.NewRequest(http.MethodPost, vectorPath, bytes.NewReader([]byte(canonicalCommand)))
		request.Header.Set("Content-Type", callbackContentType)
		request.Header.Set("X-Voice-Key-Id", "00000000-0000-4000-8000-00000000000c")
		request.Header.Set("X-Voice-Timestamp", vectorTimestamp)
		request.Header.Set("X-Voice-Signature", testSignature(key, http.MethodPost, vectorPath, vectorTimestamp, vectorKeyID, []byte(canonicalCommand)))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusUnauthorized, recorder.Code)
	})
}
