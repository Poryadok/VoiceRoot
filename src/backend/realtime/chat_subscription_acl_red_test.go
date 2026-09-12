package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// T-057 intentionally exercises the current public WS handler, not a test-only
// authorizer. Thus UUID-only acceptance is an executable RED failure rather
// than an undefined future seam that would make the package fail to compile.
func newACLTestRealtimeHandler(tv tokenValidator) (*wsHub, http.Handler) {
	return newACLTestRealtimeHandlerWithLister(tv, nil)
}

func newACLTestRealtimeHandlerWithLister(tv tokenValidator, lister chatBootstrapLister) (*wsHub, http.Handler) {
	hub := newWSHub()
	if lister != nil {
		hub.subscriptionChecker = subscriptionCheckerFunc(func(ctx context.Context, accountID, profileID, chatID string) error {
			ids, err := lister.ListChatIDs(ctx, accountID, profileID)
			if err != nil {
				return err
			}
			for _, id := range ids {
				if canonicalChatID(id) == canonicalChatID(chatID) {
					return nil
				}
			}
			return status.Error(codes.PermissionDenied, "not a chat member")
		})
	}
	return hub, newServiceHandler(serviceName, tv, lister, hub, nil, "acl-test-instance", readinessDeps{})
}

func dialACLTestConn(t *testing.T, srv *httptest.Server, token, profileID string) *websocket.Conn {
	t.Helper()
	h := wsUpgradeHeaders(token)
	h.Set("X-Profile-Id", profileID)
	c, _, err := websocket.DefaultDialer.Dial(wsEndpoint(t, srv), h)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	if got := readACLEnvelope(t, c); got.Op != "hello" || got.S != 1 {
		t.Fatalf("first outbound = %+v, want hello s=1", got)
	}
	return c
}

func readACLEnvelope(t *testing.T, c *websocket.Conn) wsEnvelope {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket message: %v", err)
	}
	var env wsEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("decode websocket envelope: %v", err)
	}
	return env
}

func assertSubscriptionDenied(t *testing.T, env wsEnvelope, sequence int64) {
	t.Helper()
	if env.Op != "error" || env.S != sequence {
		t.Fatalf("denial envelope = %+v, want error s=%d", env, sequence)
	}
	var body map[string]any
	if err := json.Unmarshal(env.D, &body); err != nil {
		t.Fatalf("decode subscription denial: %v", err)
	}
	if body["code"] != "permission_denied" {
		t.Fatalf("denial code = %#v, want permission_denied; body=%v", body["code"], body)
	}
	for _, forbidden := range []string{"grpc_code", "grpc_status", "internal_code", "cause", "detail"} {
		if _, ok := body[forbidden]; ok {
			t.Fatalf("subscription denial leaked internal field %q: %v", forbidden, body)
		}
	}
}

func TestWSSubscribeACLValidUUIDFailsClosedWithoutChatAuthorization(t *testing.T) {
	accountID, profileID, chatID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	hub, h := newACLTestRealtimeHandler(staticTokenValidator{
		"member": {UserID: accountID, ProfileID: profileID},
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := dialACLTestConn(t, srv, "member", profileID)

	if err := c.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": chatID}}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	assertSubscriptionDenied(t, readACLEnvelope(t, c), 2)
	hub.mu.RLock()
	_, registered := hub.byChat[chatID]
	hub.mu.RUnlock()
	if registered {
		t.Fatal("unverified subscribe registered hub state")
	}

	// A generic denial is a protocol response, not a connection close.
	if err := c.WriteJSON(map[string]any{"op": "heartbeat"}); err != nil {
		t.Fatalf("heartbeat after denied subscribe: %v", err)
	}
	if got := readACLEnvelope(t, c); got.Op != "heartbeat_ack" || got.S != 3 {
		t.Fatalf("connection did not remain open after denial: %+v", got)
	}
}

func TestWSSubscribeACLMalformedUUIDKeepsLegacyInvalidSubscribe(t *testing.T) {
	profileID := uuid.NewString()
	_, h := newACLTestRealtimeHandler(staticTokenValidator{
		"member": {UserID: uuid.NewString(), ProfileID: profileID},
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := dialACLTestConn(t, srv, "member", profileID)
	if err := c.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": "not-a-uuid"}}); err != nil {
		t.Fatalf("malformed subscribe: %v", err)
	}
	got := readACLEnvelope(t, c)
	if got.Op != "error" || got.S != 2 {
		t.Fatalf("malformed subscribe response = %+v", got)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(got.D, &body); err != nil || body.Code != "invalid_subscribe" {
		t.Fatalf("malformed subscribe body = %s, err=%v", got.D, err)
	}
}

type perProfileBootstrapLister map[string][]string

func (l perProfileBootstrapLister) ListChatIDs(_ context.Context, _, profileID string) ([]string, error) {
	return append([]string(nil), l[profileID]...), nil
}

func TestWSSubscribeACLSeparatesProfilesOfSameAccount(t *testing.T) {
	accountID, memberProfileID, otherProfileID, chatID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	hub, h := newACLTestRealtimeHandlerWithLister(staticTokenValidator{
		"member": {UserID: accountID, ProfileID: memberProfileID},
		"other":  {UserID: accountID, ProfileID: otherProfileID},
	}, perProfileBootstrapLister{memberProfileID: {chatID}})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	member := dialACLTestConn(t, srv, "member", memberProfileID)
	if got := readACLEnvelope(t, member); got.Op != "subscription_sync" || got.S != 2 {
		t.Fatalf("member bootstrap = %+v", got)
	}
	other := dialACLTestConn(t, srv, "other", otherProfileID)
	if got := readACLEnvelope(t, other); got.Op != "subscription_sync" || got.S != 2 {
		t.Fatalf("nonmember bootstrap = %+v", got)
	}

	// A fresh connection for another profile of the same account must not inherit
	// the first profile's bootstrap membership or gain it through lazy subscribe.
	if err := other.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": chatID}}); err != nil {
		t.Fatalf("nonmember subscribe: %v", err)
	}
	assertSubscriptionDenied(t, readACLEnvelope(t, other), 3)
	hub.mu.RLock()
	for reg := range hub.byProfile[otherProfileID] {
		if _, inherited := reg.chats[chatID]; inherited {
			hub.mu.RUnlock()
			t.Fatal("fresh profile inherited another profile's chat subscription")
		}
	}
	hub.mu.RUnlock()
}

func TestWSBootstrapAppliesSubscriptionPolicyFailClosed(t *testing.T) {
	accountID, profileID := uuid.NewString(), uuid.NewString()
	blockedChatID, unavailableChatID, allowedChatID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	checker := subscriptionCheckerFunc(func(_ context.Context, account, profile, chat string) error {
		if account != accountID || profile != profileID {
			return errors.New("unexpected caller")
		}
		switch canonicalChatID(chat) {
		case blockedChatID:
			return status.Error(codes.PermissionDenied, "account pair blocked")
		case unavailableChatID:
			return status.Error(codes.Unavailable, "social unavailable")
		case allowedChatID:
			return nil
		default:
			return errors.New("unexpected chat")
		}
	})
	hub := newWSHub()
	hub.subscriptionChecker = checker
	h := newServiceHandler(serviceName, staticTokenValidator{
		"member": {UserID: accountID, ProfileID: profileID},
	}, perProfileBootstrapLister{profileID: {blockedChatID, unavailableChatID, allowedChatID}}, hub, nil, "bootstrap-policy-test", readinessDeps{})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := dialACLTestConn(t, srv, "member", profileID)

	sync := readACLEnvelope(t, c)
	if sync.Op != "subscription_sync" || sync.S != 2 {
		t.Fatalf("bootstrap response = %+v", sync)
	}
	var body struct {
		ChatIDs  []string `json:"chat_ids"`
		Degraded bool     `json:"degraded"`
	}
	if err := json.Unmarshal(sync.D, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.ChatIDs) != 1 || body.ChatIDs[0] != allowedChatID {
		t.Fatalf("bootstrap chat_ids = %v, want only allowed chat %s", body.ChatIDs, allowedChatID)
	}
	if !body.Degraded {
		t.Fatal("dependency failure must mark bootstrap degraded while still omitting the chat")
	}
	hub.mu.RLock()
	_, blockedRegistered := hub.byChat[blockedChatID]
	_, unavailableRegistered := hub.byChat[unavailableChatID]
	_, allowedRegistered := hub.byChat[allowedChatID]
	hub.mu.RUnlock()
	if blockedRegistered || unavailableRegistered || !allowedRegistered {
		t.Fatalf("hub registrations blocked=%v unavailable=%v allowed=%v", blockedRegistered, unavailableRegistered, allowedRegistered)
	}
}
