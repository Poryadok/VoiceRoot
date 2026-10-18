package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type appNotificationAuthorityStub struct {
	gameScoped, allowed bool
	calls               int
	profileID, chatID   uuid.UUID
	category, channel   string
}

func (s *appNotificationAuthorityStub) AllowsAppNotificationPush(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (bool, error) {
	return false, nil
}

func (s *appNotificationAuthorityStub) ResolveAppNotificationChat(_ context.Context, profileID, chatID uuid.UUID, category, channel string) (bool, bool, error) {
	s.calls++
	s.profileID, s.chatID, s.category, s.channel = profileID, chatID, category, channel
	return s.gameScoped, s.allowed, nil
}

func TestInternalAppNotificationChatLookupSignsScopedAndOrdinaryResults(t *testing.T) {
	now := time.Unix(1790435000, 0)
	key := []byte("0123456789abcdef0123456789abcdef")
	profileID, chatID := uuid.New(), uuid.New()
	path := "/internal/v1/notification-consents/chat/" + profileID.String() + "/" + chatID.String() + "/game_activity/push"
	authority := &appNotificationAuthorityStub{}
	h := &InternalAppNotificationConsentHandler{
		PushVerifier: WorkloadVerifier{Key: key, Principal: "notification", Now: func() time.Time { return now }, Nonces: &nonceMemory{}}, Authority: authority,
	}
	request := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		signAppNotificationRequest(r, key, now, uuid.NewString(), "notification")
		return r
	}

	// Unmapped Voice chats return the explicit non-game result used by Notification's normal push path.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, request())
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"game_scoped":false,"allowed":false}`, w.Body.String())
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	r := request()
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, r.Header.Get("X-Voice-Timestamp"), w.Header().Get("X-Voice-Response-Timestamp"))
	require.Equal(t, r.Header.Get("X-Voice-Nonce"), w.Header().Get("X-Voice-Response-Nonce"))
	require.Equal(t, responseSignature(key, http.StatusOK, r.URL.EscapedPath(), r.Header.Get("X-Voice-Timestamp"),
		r.Header.Get("X-Voice-Nonce"), w.Body.Bytes()), w.Header().Get("X-Voice-Response-Signature"))

	authority.gameScoped, authority.allowed = true, true
	w = httptest.NewRecorder()
	h.ServeHTTP(w, request())
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"game_scoped":true,"allowed":true}`, w.Body.String())
	require.Equal(t, profileID, authority.profileID)
	require.Equal(t, chatID, authority.chatID)
	require.Equal(t, "game_activity", authority.category)
	require.Equal(t, "push", authority.channel)
}

func TestInternalAppNotificationChatLookupRejectsReplayAndMalformedScope(t *testing.T) {
	now := time.Unix(1790435000, 0)
	key := []byte("0123456789abcdef0123456789abcdef")
	authority := &appNotificationAuthorityStub{gameScoped: true, allowed: true}
	h := &InternalAppNotificationConsentHandler{
		PushVerifier: WorkloadVerifier{Key: key, Principal: "notification", Now: func() time.Time { return now }, Nonces: &nonceMemory{}}, Authority: authority,
	}
	path := "/internal/v1/notification-consents/chat/" + uuid.NewString() + "/" + uuid.NewString() + "/game_activity/push"
	r := httptest.NewRequest(http.MethodGet, path, nil)
	signAppNotificationRequest(r, key, now, uuid.NewString(), "notification")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Equal(t, 1, authority.calls)

	for _, malformed := range []string{
		"/internal/v1/notification-consents/chat/not-a-uuid/" + uuid.NewString() + "/game_activity/push",
		"/internal/v1/notification-consents/chat/" + uuid.NewString() + "/" + uuid.NewString() + "/unknown/push",
	} {
		bad := httptest.NewRequest(http.MethodGet, malformed, nil)
		SignWorkloadRequest(bad, key, now, uuid.NewString())
		w = httptest.NewRecorder()
		h.ServeHTTP(w, bad)
		require.Equal(t, http.StatusNotFound, w.Code)
	}
	require.Equal(t, 1, authority.calls)
}

func TestInternalAppNotificationChatLookupUsesChannelSpecificWorkloadKey(t *testing.T) {
	now := time.Unix(1790435000, 0)
	pushKey := []byte("0123456789abcdef0123456789abcdef")
	inAppKey := []byte("abcdef0123456789abcdef0123456789")
	authority := &appNotificationAuthorityStub{gameScoped: true, allowed: true}
	h := &InternalAppNotificationConsentHandler{
		PushVerifier:  WorkloadVerifier{Key: pushKey, Principal: "notification", Now: func() time.Time { return now }, Nonces: &nonceMemory{}},
		InAppVerifier: WorkloadVerifier{Key: inAppKey, Principal: "realtime", Now: func() time.Time { return now }, Nonces: &nonceMemory{}},
		Authority:     authority,
	}
	path := "/internal/v1/notification-consents/chat/" + uuid.NewString() + "/" + uuid.NewString() + "/game_social/in_app"
	r := httptest.NewRequest(http.MethodGet, path, nil)
	signAppNotificationRequest(r, inAppKey, now, uuid.NewString(), "realtime")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, responseSignature(inAppKey, http.StatusOK, r.URL.EscapedPath(), r.Header.Get("X-Voice-Timestamp"),
		r.Header.Get("X-Voice-Nonce"), w.Body.Bytes()), w.Header().Get("X-Voice-Response-Signature"))
	require.Equal(t, 1, authority.calls)

	wrongIdentity := httptest.NewRequest(http.MethodGet, path, nil)
	signAppNotificationRequest(wrongIdentity, inAppKey, now, uuid.NewString(), "notification")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, wrongIdentity)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Equal(t, 1, authority.calls)
}

func signAppNotificationRequest(r *http.Request, key []byte, now time.Time, nonce, principal string) {
	timestamp := strconv.FormatInt(now.Unix(), 10)
	r.Header.Set("X-Voice-Workload", principal)
	r.Header.Set("X-Voice-Timestamp", timestamp)
	r.Header.Set("X-Voice-Nonce", nonce)
	r.Header.Set("X-Voice-Signature", workloadSignature(key, r.Method, r.URL.EscapedPath(), timestamp, nonce))
}
