package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	voicejwt "voice/backend/pkg/jwt"
)

// These tests describe the complete strict-mode boundary accepted in
// tmp/fleet/plans/A1-daily-messaging.md.  They intentionally name the small
// handler policy seam that the next GREEN slice must add; this file must not
// grow an event consumer or make a Redis client assertion part of WS behavior.

func TestWSStrictSessionEpochUpgradeUsesVerifiedClaimsBeforeHello(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	validator := &epochEnforcementValidator{claims: epochEnforcementClaims(now)}
	floor := &epochEnforcementFloor{byAccount: map[string]epochEnforcementFloorResult{
		"account-from-jwt": {minimum: 7},
	}}
	srv := httptest.NewServer(newEpochEnforcementHandler(validator, floor, func() time.Time { return now }, true, newWSHub(), nil, nil))
	t.Cleanup(srv.Close)

	headers := epochEnforcementHeaders("verified-token", "profile-from-jwt")
	headers.Set("X-Voice-User-Id", "attacker-account")
	conn := epochEnforcementDial(t, srv, headers)
	t.Cleanup(func() { _ = conn.Close() })
	require.Equal(t, "hello", epochEnforcementRead(t, conn).Op)
	require.Equal(t, 1, validator.callCount())
	require.Equal(t, []string{"account-from-jwt", "account-from-jwt"}, floor.accounts())

	// Raw internal headers are neither an alternative credential nor an account
	// identity.  X-Profile-Id remains the existing selector and is cross-checked
	// against the verified JWT profile; a conflicting raw selector is rejected.
	badHeaders := epochEnforcementHeaders("", "")
	badHeaders.Set("X-Voice-User-Id", "account-from-jwt")
	badHeaders.Set("X-Voice-Profile-Id", "profile-from-jwt")
	_, response, err := websocket.DefaultDialer.Dial(epochEnforcementEndpoint(t, srv), badHeaders)
	require.Error(t, err)
	require.NotNil(t, response)
	require.NotEqual(t, http.StatusSwitchingProtocols, response.StatusCode)

	conflictingProfile := epochEnforcementHeaders("verified-token", "profile-from-jwt")
	conflictingProfile.Set("X-Voice-Profile-Id", "attacker-profile")
	_, response, err = websocket.DefaultDialer.Dial(epochEnforcementEndpoint(t, srv), conflictingProfile)
	require.Error(t, err)
	require.NotNil(t, response)
	require.NotEqual(t, http.StatusSwitchingProtocols, response.StatusCode)
}

func TestWSStrictSessionEpochRejectsUnsafeUpgradeBeforeHello(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	cases := map[string]epochEnforcementFloorResult{
		"stale epoch":       {minimum: 8},
		"missing floor":     {minimum: 0},
		"corrupt floor":     {err: errors.New("invalid epoch floor")},
		"redis unavailable": {err: errors.New("redis unavailable")},
	}
	for name, result := range cases {
		t.Run(name, func(t *testing.T) {
			validator := &epochEnforcementValidator{claims: epochEnforcementClaims(now)}
			floor := &epochEnforcementFloor{byAccount: map[string]epochEnforcementFloorResult{"account-from-jwt": result}}
			srv := httptest.NewServer(newEpochEnforcementHandler(validator, floor, func() time.Time { return now }, true, newWSHub(), nil, nil))
			t.Cleanup(srv.Close)

			conn, response, err := websocket.DefaultDialer.Dial(epochEnforcementEndpoint(t, srv), epochEnforcementHeaders("verified-token", "profile-from-jwt"))
			if conn != nil {
				_ = conn.Close()
			}
			require.Error(t, err)
			require.NotNil(t, response)
			require.NotEqual(t, http.StatusSwitchingProtocols, response.StatusCode, "unsafe floor must be rejected before hello")
			require.Equal(t, []string{"account-from-jwt"}, floor.accounts())
		})
	}

	t.Run("expired verified token", func(t *testing.T) {
		validator := &epochEnforcementValidator{claims: epochEnforcementClaims(now.Add(-2 * time.Hour))}
		floor := &epochEnforcementFloor{byAccount: map[string]epochEnforcementFloorResult{
			"account-from-jwt": {minimum: 7},
		}}
		srv := httptest.NewServer(newEpochEnforcementHandler(validator, floor, func() time.Time { return now }, true, newWSHub(), nil, nil))
		t.Cleanup(srv.Close)
		conn, response, err := websocket.DefaultDialer.Dial(epochEnforcementEndpoint(t, srv), epochEnforcementHeaders("verified-token", "profile-from-jwt"))
		if conn != nil {
			_ = conn.Close()
		}
		require.Error(t, err)
		require.NotNil(t, response)
		require.NotEqual(t, http.StatusSwitchingProtocols, response.StatusCode)
	})
}

func TestWSStrictSessionEpochGuardsEveryInboundOperationBeforeDispatch(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	chatID := "11111111-1111-4111-8111-111111111111"
	messageID := "22222222-2222-4222-8222-222222222222"
	cases := map[string]map[string]any{
		"heartbeat":       {"op": "heartbeat", "d": map[string]any{}},
		"resume":          {"op": "resume", "d": map[string]any{"last_s": 1}},
		"subscribe":       {"op": "subscribe", "d": map[string]any{"chat_id": chatID}},
		"unsubscribe":     {"op": "unsubscribe", "d": map[string]any{"chat_id": chatID}},
		"typing start":    {"op": "typing_start", "d": map[string]any{"chat_id": chatID}},
		"typing stop":     {"op": "typing_stop", "d": map[string]any{"chat_id": chatID}},
		"mark read":       {"op": "mark_read", "d": map[string]any{"chat_id": chatID, "message_id": messageID}},
		"delivery ack":    {"op": "delivery_ack", "d": map[string]any{"chat_id": chatID, "message_id": messageID, "sender_profile_id": "66666666-6666-4666-8666-666666666666"}},
		"presence update": {"op": "presence_update", "d": map[string]any{"status": "away", "custom_status": "busy"}},
	}
	requiresSubscription := map[string]bool{
		"typing start": true,
		"typing stop":  true,
		"mark read":    true,
		"delivery ack": true,
	}
	for name, inbound := range cases {
		t.Run(name, func(t *testing.T) {
			floor := &epochEnforcementFloor{byAccount: map[string]epochEnforcementFloorResult{"account-from-jwt": {minimum: 7}}}
			validator := &epochEnforcementValidator{claims: epochEnforcementClaims(now)}
			presence := &epochEnforcementPresenceSpy{}
			publisher := &epochEnforcementDeliveryAckSpy{}
			dispatch := &epochEnforcementDispatchSpy{}
			policy := wsSessionEpochPolicy{
				Strict:              true,
				Floor:               floor,
				Now:                 func() time.Time { return now },
				OnAuthorizedInbound: dispatch.record,
			}
			srv := httptest.NewServer(newWSHandlerWithSessionEpoch(validator, nil, permitAllTestSubscriptions(newWSHub()), nil, "epoch-test", presence, publisher, policy))
			t.Cleanup(srv.Close)
			conn := epochEnforcementDial(t, srv, epochEnforcementHeaders("verified-token", "profile-from-jwt"))
			t.Cleanup(func() { _ = conn.Close() })
			require.Equal(t, "hello", epochEnforcementRead(t, conn).Op)
			if requiresSubscription[name] {
				require.NoError(t, conn.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": chatID}}))
				require.Equal(t, "subscribe_ack", epochEnforcementRead(t, conn).Op)
			}
			operationsBeforeRevocation := dispatch.operations()

			floor.set("account-from-jwt", epochEnforcementFloorResult{minimum: 8})
			require.NoError(t, conn.WriteJSON(inbound))
			epochEnforcementRequireRevokedClose(t, conn)
			require.Equal(t, operationsBeforeRevocation, dispatch.operations(), "revoked frame must not reach operation dispatch")
			require.Equal(t, 1, validator.callCount(), "established frames must not revalidate JWT signatures")
			require.Equal(t, 1, presence.countStatus("online"), "stale heartbeat must not refresh presence")
			require.Zero(t, publisher.callCount(), "stale delivery acknowledgement must not publish")
		})
	}
}

func TestWSStrictSessionEpochDispatchesOnlyAuthorizedInboundOperation(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	floor := &epochEnforcementFloor{byAccount: map[string]epochEnforcementFloorResult{"account-from-jwt": {minimum: 7}}}
	dispatch := &epochEnforcementDispatchSpy{}
	policy := wsSessionEpochPolicy{
		Strict:              true,
		Floor:               floor,
		Now:                 func() time.Time { return now },
		OnAuthorizedInbound: dispatch.record,
	}
	srv := httptest.NewServer(newWSHandlerWithSessionEpoch(&epochEnforcementValidator{claims: epochEnforcementClaims(now)}, nil, newWSHub(), nil, "epoch-test", nil, nil, policy))
	t.Cleanup(srv.Close)
	conn := epochEnforcementDial(t, srv, epochEnforcementHeaders("verified-token", "profile-from-jwt"))
	t.Cleanup(func() { _ = conn.Close() })
	require.Equal(t, "hello", epochEnforcementRead(t, conn).Op)
	require.NoError(t, conn.WriteJSON(map[string]any{"op": "heartbeat", "d": map[string]any{}}))
	require.Equal(t, "heartbeat_ack", epochEnforcementRead(t, conn).Op)
	require.Equal(t, []string{"heartbeat"}, dispatch.operations())
}

func TestWSStrictSessionEpochClosesEstablishedConnectionForUnsafeFloor(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	for name, unsafe := range map[string]epochEnforcementFloorResult{
		"missing floor":     {minimum: 0},
		"corrupt floor":     {err: errors.New("invalid floor encoding")},
		"redis unavailable": {err: errors.New("redis unavailable")},
	} {
		t.Run(name, func(t *testing.T) {
			floor := &epochEnforcementFloor{byAccount: map[string]epochEnforcementFloorResult{"account-from-jwt": {minimum: 7}}}
			validator := &epochEnforcementValidator{claims: epochEnforcementClaims(now)}
			srv := httptest.NewServer(newEpochEnforcementHandler(validator, floor, func() time.Time { return now }, true, newWSHub(), nil, nil))
			t.Cleanup(srv.Close)
			conn := epochEnforcementDial(t, srv, epochEnforcementHeaders("verified-token", "profile-from-jwt"))
			t.Cleanup(func() { _ = conn.Close() })
			require.Equal(t, "hello", epochEnforcementRead(t, conn).Op)

			floor.set("account-from-jwt", unsafe)
			require.NoError(t, conn.WriteJSON(map[string]any{"op": "heartbeat", "d": map[string]any{}}))
			epochEnforcementRequireRevokedClose(t, conn)
			require.Equal(t, 1, validator.callCount())
		})
	}
}

func TestWSStrictSessionEpochBlocksInboundSideEffects(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	chatID := "33333333-3333-4333-8333-333333333333"
	floor := &epochEnforcementFloor{byAccount: map[string]epochEnforcementFloorResult{"account-from-jwt": {minimum: 7}}}
	presence := &epochEnforcementPresenceSpy{}
	publisher := &epochEnforcementDeliveryAckSpy{}
	hub := newWSHub()
	srv := httptest.NewServer(newEpochEnforcementHandler(&epochEnforcementValidator{claims: epochEnforcementClaims(now)}, floor, func() time.Time { return now }, true, hub, presence, publisher))
	t.Cleanup(srv.Close)
	conn := epochEnforcementDial(t, srv, epochEnforcementHeaders("verified-token", "profile-from-jwt"))
	t.Cleanup(func() { _ = conn.Close() })
	require.Equal(t, "hello", epochEnforcementRead(t, conn).Op)

	// Establish the normal subscription first; after revocation neither the
	// delivery publisher nor the presence updater may see a new frame.
	require.NoError(t, conn.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": chatID}}))
	require.Equal(t, "subscribe_ack", epochEnforcementRead(t, conn).Op)
	floor.set("account-from-jwt", epochEnforcementFloorResult{minimum: 8})
	require.NoError(t, conn.WriteJSON(map[string]any{"op": "presence_update", "d": map[string]any{"status": "away"}}))
	epochEnforcementRequireRevokedClose(t, conn)
	require.Zero(t, presence.countStatus("away"))
	require.Zero(t, publisher.callCount())
}

func TestWSStrictSessionEpochBlocksStaleTypingFanout(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	chatID := "55555555-5555-4555-8555-555555555555"
	floor := &epochEnforcementFloor{byAccount: map[string]epochEnforcementFloorResult{
		"account-from-jwt": {minimum: 7},
		"peer-account":     {minimum: 7},
	}}
	hub := newWSHub()
	validator := &epochEnforcementValidator{claims: epochEnforcementClaims(now)}
	srv := httptest.NewServer(newEpochEnforcementHandler(validator, floor, func() time.Time { return now }, true, hub, nil, nil))
	t.Cleanup(srv.Close)
	sender := epochEnforcementDial(t, srv, epochEnforcementHeaders("verified-token", "profile-from-jwt"))
	t.Cleanup(func() { _ = sender.Close() })
	require.Equal(t, "hello", epochEnforcementRead(t, sender).Op)

	peerClaims := epochEnforcementClaims(now)
	peerClaims.UserID = "peer-account"
	peerClaims.ProfileID = "peer-profile"
	validator.setClaims(peerClaims)
	peer := epochEnforcementDial(t, srv, epochEnforcementHeaders("verified-token", "peer-profile"))
	t.Cleanup(func() { _ = peer.Close() })
	require.Equal(t, "hello", epochEnforcementRead(t, peer).Op)

	for _, conn := range []*websocket.Conn{sender, peer} {
		require.NoError(t, conn.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": chatID}}))
		require.Equal(t, "subscribe_ack", epochEnforcementRead(t, conn).Op)
	}
	floor.set("account-from-jwt", epochEnforcementFloorResult{minimum: 8})
	require.NoError(t, sender.WriteJSON(map[string]any{"op": "typing_start", "d": map[string]any{"chat_id": chatID}}))
	epochEnforcementRequireRevokedClose(t, sender)

	_ = peer.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, _, err := peer.ReadMessage()
	require.Error(t, err, "stale sender must not fan out typing before it closes")
	var closeError *websocket.CloseError
	require.NotErrorAs(t, err, &closeError)
}

func TestWSStrictSessionEpochRechecksFloorBeforeEveryLocalWrite(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	chatID := "77777777-7777-4777-8777-777777777777"
	type writeCase struct {
		floorResults []epochEnforcementFloorResult
		lister       chatBootstrapLister
		action       func(*testing.T, *websocket.Conn)
	}
	cases := map[string]writeCase{
		"hello": {
			floorResults: []epochEnforcementFloorResult{{minimum: 7}, {minimum: 8}},
			action: func(t *testing.T, conn *websocket.Conn) {
				epochEnforcementRequireRevokedClose(t, conn)
			},
		},
		"subscription sync": {
			floorResults: []epochEnforcementFloorResult{{minimum: 7}},
			lister:       epochEnforcementBootstrapLister{chatIDs: []string{chatID}},
			action: func(t *testing.T, conn *websocket.Conn) {
				require.Equal(t, "hello", epochEnforcementRead(t, conn).Op)
				epochEnforcementRequireRevokedClose(t, conn)
			},
		},
		"heartbeat acknowledgement": {
			floorResults: []epochEnforcementFloorResult{{minimum: 7}},
			action: func(t *testing.T, conn *websocket.Conn) {
				require.Equal(t, "hello", epochEnforcementRead(t, conn).Op)
				require.NoError(t, conn.WriteJSON(map[string]any{"op": "heartbeat", "d": map[string]any{}}))
				epochEnforcementRequireRevokedClose(t, conn)
			},
		},
		"subscribe acknowledgement": {
			floorResults: []epochEnforcementFloorResult{{minimum: 7}},
			action: func(t *testing.T, conn *websocket.Conn) {
				require.Equal(t, "hello", epochEnforcementRead(t, conn).Op)
				require.NoError(t, conn.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": chatID}}))
				epochEnforcementRequireRevokedClose(t, conn)
			},
		},
		"unsubscribe acknowledgement": {
			floorResults: []epochEnforcementFloorResult{{minimum: 7}},
			action: func(t *testing.T, conn *websocket.Conn) {
				require.Equal(t, "hello", epochEnforcementRead(t, conn).Op)
				require.NoError(t, conn.WriteJSON(map[string]any{"op": "unsubscribe", "d": map[string]any{"chat_id": chatID}}))
				epochEnforcementRequireRevokedClose(t, conn)
			},
		},
		"protocol error": {
			floorResults: []epochEnforcementFloorResult{{minimum: 7}},
			action: func(t *testing.T, conn *websocket.Conn) {
				require.Equal(t, "hello", epochEnforcementRead(t, conn).Op)
				require.NoError(t, conn.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": "not-a-uuid"}}))
				epochEnforcementRequireRevokedClose(t, conn)
			},
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			floor := &epochEnforcementFloor{byAccount: map[string]epochEnforcementFloorResult{"account-from-jwt": testCase.floorResults[0]}}
			validator := &epochEnforcementValidator{claims: epochEnforcementClaims(now)}
			targetWrite := map[string]string{
				"hello":                       "hello",
				"subscription sync":           "subscription_sync",
				"heartbeat acknowledgement":   "heartbeat_ack",
				"subscribe acknowledgement":   "subscribe_ack",
				"unsubscribe acknowledgement": "unsubscribe_ack",
				"protocol error":              "error",
			}[name]
			policy := wsSessionEpochPolicy{
				Strict: true,
				Floor:  floor,
				Now:    func() time.Time { return now },
				BeforeWrite: func(op string) {
					if op == targetWrite {
						floor.set("account-from-jwt", epochEnforcementFloorResult{minimum: 8})
					}
				},
			}
			srv := httptest.NewServer(newWSHandlerWithSessionEpoch(validator, testCase.lister, permitAllTestSubscriptions(newWSHub()), nil, "epoch-test", nil, nil, policy))
			t.Cleanup(srv.Close)
			conn := epochEnforcementDial(t, srv, epochEnforcementHeaders("verified-token", "profile-from-jwt"))
			t.Cleanup(func() { _ = conn.Close() })
			testCase.action(t, conn)
		})
	}
}

func TestWSStrictSessionEpochGuardsEveryOutboundFanoutAndExpiry(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	for name, mutate := range map[string]func(*epochEnforcementFloor, *epochEnforcementClock){
		"stale floor": func(f *epochEnforcementFloor, _ *epochEnforcementClock) {
			f.set("account-from-jwt", epochEnforcementFloorResult{minimum: 8})
		},
		"expired token": func(_ *epochEnforcementFloor, clock *epochEnforcementClock) {
			clock.set(base.Add(2 * time.Hour))
		},
	} {
		for fanoutName, fanout := range map[string]func(*wsHub){
			"profile fanout": func(hub *wsHub) {
				hub.broadcastToProfile("profile-from-jwt", fanoutEnvelope{Op: "message_create", D: json.RawMessage(`{"id":"should-not-leak"}`)}, nil, "")
			},
			"chat fanout": func(hub *wsHub) {
				hub.broadcastToChat("44444444-4444-4444-8444-444444444444", fanoutEnvelope{Op: "message_create", D: json.RawMessage(`{"id":"should-not-leak"}`)}, nil, "")
			},
		} {
			t.Run(name+"/"+fanoutName, func(t *testing.T) {
				clock := &epochEnforcementClock{value: base}
				floor := &epochEnforcementFloor{byAccount: map[string]epochEnforcementFloorResult{"account-from-jwt": {minimum: 7}}}
				hub := newWSHub()
				validator := &epochEnforcementValidator{claims: epochEnforcementClaims(base)}
				srv := httptest.NewServer(newEpochEnforcementHandler(validator, floor, clock.now, true, hub, nil, nil))
				t.Cleanup(srv.Close)
				conn := epochEnforcementDial(t, srv, epochEnforcementHeaders("verified-token", "profile-from-jwt"))
				t.Cleanup(func() { _ = conn.Close() })
				require.Equal(t, "hello", epochEnforcementRead(t, conn).Op)
				if fanoutName == "chat fanout" {
					require.NoError(t, conn.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": "44444444-4444-4444-8444-444444444444"}}))
					require.Equal(t, "subscribe_ack", epochEnforcementRead(t, conn).Op)
				}

				mutate(floor, clock)
				fanout(hub)
				epochEnforcementRequireRevokedClose(t, conn)
				require.Equal(t, 1, validator.callCount(), "fanout never revalidates the signature")
				wantFloorCalls := 3 // upgrade, hello write, guarded fanout write
				if fanoutName == "chat fanout" {
					wantFloorCalls += 2 // guarded subscribe plus guarded subscribe_ack
				}
				if name == "expired token" {
					wantFloorCalls-- // expiry is checked before the authoritative floor
				}
				require.Equal(t, wantFloorCalls, floor.callCount(), "outbound write must check the authoritative floor")
			})
		}
	}
}

func TestWSStrictSessionEpochRechecksQueuedFanoutAtActualWrite(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	floor := &epochEnforcementFloor{byAccount: map[string]epochEnforcementFloorResult{
		"account-from-jwt": {minimum: 7},
	}}
	hub := newWSHub()
	policy := wsSessionEpochPolicy{
		Strict: true,
		Floor:  floor,
		Now:    func() time.Time { return now },
		BeforeWrite: func(op string) {
			if op == "message_create" {
				floor.set("account-from-jwt", epochEnforcementFloorResult{minimum: 8})
			}
		},
	}
	srv := httptest.NewServer(newWSHandlerWithSessionEpoch(&epochEnforcementValidator{claims: epochEnforcementClaims(now)}, nil, hub, nil, "epoch-test", nil, nil, policy))
	t.Cleanup(srv.Close)
	conn := epochEnforcementDial(t, srv, epochEnforcementHeaders("verified-token", "profile-from-jwt"))
	t.Cleanup(func() { _ = conn.Close() })
	require.Equal(t, "hello", epochEnforcementRead(t, conn).Op)

	// The hub gate admits this fanout while the floor is 7. The write gate must
	// still be authoritative when the queued frame reaches Gorilla.
	hub.broadcastToProfile("profile-from-jwt", fanoutEnvelope{Op: "message_create", D: json.RawMessage(`{"id":"must-not-leak"}`)}, nil, "")
	epochEnforcementRequireRevokedClose(t, conn)
	require.Equal(t, 4, floor.callCount(), "upgrade, hello, enqueue, and actual write must each check the floor")
}

func TestWSStrictSessionEpochSerializesConcurrentInboundAndFanoutRevocationClose(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	floor := newEpochEnforcementBarrierFloor()
	hub := newWSHub()
	var closeCalls int
	var closeMu sync.Mutex
	policy := wsSessionEpochPolicy{
		Strict: true,
		Floor:  floor,
		Now:    func() time.Time { return now },
		CloseWriter: func(conn *websocket.Conn, code int, reason string) error {
			closeMu.Lock()
			closeCalls++
			closeMu.Unlock()
			return conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(time.Second))
		},
	}
	srv := httptest.NewServer(newWSHandlerWithSessionEpoch(&epochEnforcementValidator{claims: epochEnforcementClaims(now)}, nil, hub, nil, "epoch-test", nil, nil, policy))
	t.Cleanup(srv.Close)
	conn := epochEnforcementDial(t, srv, epochEnforcementHeaders("verified-token", "profile-from-jwt"))
	t.Cleanup(func() { _ = conn.Close() })
	require.Equal(t, "hello", epochEnforcementRead(t, conn).Op)
	floor.failClosed()

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_ = conn.WriteJSON(map[string]any{"op": "heartbeat", "d": map[string]any{}})
	}()
	go func() {
		defer wg.Done()
		<-start
		hub.broadcastToProfile("profile-from-jwt", fanoutEnvelope{Op: "message_create", D: json.RawMessage(`{}`)}, nil, "")
	}()
	close(start)
	floor.awaitFailures(t, 2)
	floor.releaseFailures()
	wg.Wait()
	epochEnforcementRequireRevokedClose(t, conn)
	closeMu.Lock()
	require.Equal(t, 1, closeCalls)
	closeMu.Unlock()
}

func TestWSCompatibilitySessionEpochSkipsFloorForLegacyConnection(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	claims := epochEnforcementClaims(now)
	claims.SessionEpoch = 0 // compatibility mode must retain legacy access tokens.
	validator := &epochEnforcementValidator{claims: claims}
	floor := &epochEnforcementFloor{byAccount: map[string]epochEnforcementFloorResult{"account-from-jwt": {err: errors.New("must not be read")}}}
	hub := newWSHub()
	srv := httptest.NewServer(newEpochEnforcementHandler(validator, floor, func() time.Time { return now }, false, hub, nil, nil))
	t.Cleanup(srv.Close)
	conn := epochEnforcementDial(t, srv, epochEnforcementHeaders("verified-token", "profile-from-jwt"))
	t.Cleanup(func() { _ = conn.Close() })
	require.Equal(t, "hello", epochEnforcementRead(t, conn).Op)
	require.NoError(t, conn.WriteJSON(map[string]any{"op": "heartbeat", "d": map[string]any{}}))
	require.Equal(t, "heartbeat_ack", epochEnforcementRead(t, conn).Op)
	hub.broadcastToProfile("profile-from-jwt", fanoutEnvelope{Op: "message_create", D: json.RawMessage(`{}`)}, nil, "")
	require.Equal(t, "message_create", epochEnforcementRead(t, conn).Op)
	require.Zero(t, floor.callCount())
	require.Equal(t, 1, validator.callCount())
}

// epochEnforcementPolicy is deliberately the only test-facing wiring seam.
// GREEN may wire it through realtimeConfig rather than exposing it publicly.
func newEpochEnforcementHandler(tv tokenValidator, floor sessionEpochFloor, now func() time.Time, strict bool, hub *wsHub, presence presenceUpdater, dap deliveryAckPublisher) http.Handler {
	return newEpochEnforcementHandlerWithLister(tv, nil, floor, now, strict, permitAllTestSubscriptions(hub), presence, dap)
}

func newEpochEnforcementHandlerWithLister(tv tokenValidator, lister chatBootstrapLister, floor sessionEpochFloor, now func() time.Time, strict bool, hub *wsHub, presence presenceUpdater, dap deliveryAckPublisher) http.Handler {
	return newWSHandlerWithSessionEpoch(tv, lister, hub, nil, "epoch-test", presence, dap, wsSessionEpochPolicy{
		Strict: strict,
		Floor:  floor,
		Now:    now,
	})
}

func epochEnforcementClaims(now time.Time) voicejwt.Claims {
	return voicejwt.Claims{
		UserID:       "account-from-jwt",
		ProfileID:    "profile-from-jwt",
		SessionEpoch: 7,
		ExpiresAt:    now.Add(time.Hour),
	}
}

type epochEnforcementValidator struct {
	mu     sync.Mutex
	claims voicejwt.Claims
	calls  int
}

func (v *epochEnforcementValidator) Validate(r *http.Request) (voicejwt.Claims, string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.calls++
	if strings.TrimSpace(r.Header.Get("Authorization")) != "Bearer verified-token" {
		return voicejwt.Claims{}, "invalid_token"
	}
	return v.claims, ""
}

func (v *epochEnforcementValidator) callCount() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.calls
}

func (v *epochEnforcementValidator) setClaims(claims voicejwt.Claims) {
	v.mu.Lock()
	v.claims = claims
	v.mu.Unlock()
}

type epochEnforcementFloorResult struct {
	minimum int64
	err     error
}

type epochEnforcementFloor struct {
	mu        sync.Mutex
	byAccount map[string]epochEnforcementFloorResult
	lookups   []string
}

func (f *epochEnforcementFloor) Minimum(_ context.Context, accountID string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lookups = append(f.lookups, accountID)
	result, ok := f.byAccount[accountID]
	if !ok {
		return 0, errors.New("missing scripted floor")
	}
	return result.minimum, result.err
}

func (f *epochEnforcementFloor) set(accountID string, result epochEnforcementFloorResult) {
	f.mu.Lock()
	f.byAccount[accountID] = result
	f.mu.Unlock()
}

func (f *epochEnforcementFloor) accounts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.lookups...)
}

func (f *epochEnforcementFloor) callCount() int { return len(f.accounts()) }

// epochEnforcementBarrierFloor makes two independent authorization paths reach
// the same revoked result before either is allowed to try the connection close.
// The GREEN implementation must consult the connection guard both for inbound
// frames and while a hub fanout is being enqueued for that connection.
type epochEnforcementBarrierFloor struct {
	mu       sync.Mutex
	failing  bool
	entered  chan struct{}
	release  chan struct{}
	released sync.Once
}

func newEpochEnforcementBarrierFloor() *epochEnforcementBarrierFloor {
	return &epochEnforcementBarrierFloor{
		entered: make(chan struct{}, 2),
		release: make(chan struct{}),
	}
}

func (f *epochEnforcementBarrierFloor) Minimum(context.Context, string) (int64, error) {
	f.mu.Lock()
	failing := f.failing
	f.mu.Unlock()
	if !failing {
		return 7, nil
	}
	f.entered <- struct{}{}
	<-f.release
	return 8, nil
}

func (f *epochEnforcementBarrierFloor) failClosed() {
	f.mu.Lock()
	f.failing = true
	f.mu.Unlock()
}

func (f *epochEnforcementBarrierFloor) awaitFailures(t *testing.T, count int) {
	t.Helper()
	for range count {
		select {
		case <-f.entered:
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for %d concurrent authorization failures", count)
		}
	}
}

func (f *epochEnforcementBarrierFloor) releaseFailures() {
	f.released.Do(func() { close(f.release) })
}

type epochEnforcementDispatchSpy struct {
	mu  sync.Mutex
	ops []string
}

func (s *epochEnforcementDispatchSpy) record(op string) {
	s.mu.Lock()
	s.ops = append(s.ops, op)
	s.mu.Unlock()
}

func (s *epochEnforcementDispatchSpy) operations() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.ops...)
}

type epochEnforcementBootstrapLister struct{ chatIDs []string }

func (l epochEnforcementBootstrapLister) ListChatIDs(context.Context, string, string) ([]string, error) {
	return append([]string(nil), l.chatIDs...), nil
}

type epochEnforcementClock struct {
	mu    sync.Mutex
	value time.Time
}

func (c *epochEnforcementClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

func (c *epochEnforcementClock) set(value time.Time) {
	c.mu.Lock()
	c.value = value
	c.mu.Unlock()
}

type epochEnforcementPresenceSpy struct {
	mu       sync.Mutex
	statuses []string
}

func (p *epochEnforcementPresenceSpy) UpdatePresence(_ context.Context, _ string, _ string, status string, _ string) error {
	p.mu.Lock()
	p.statuses = append(p.statuses, status)
	p.mu.Unlock()
	return nil
}

func (p *epochEnforcementPresenceSpy) countStatus(status string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	count := 0
	for _, got := range p.statuses {
		if got == status {
			count++
		}
	}
	return count
}

type epochEnforcementDeliveryAckSpy struct {
	mu    sync.Mutex
	calls int
}

func (p *epochEnforcementDeliveryAckSpy) PublishDeliveryAck(context.Context, string, string, string) error {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
	return nil
}

func (p *epochEnforcementDeliveryAckSpy) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func epochEnforcementEndpoint(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	u.Scheme = "ws"
	u.Path = "/ws"
	return u.String()
}

func epochEnforcementHeaders(token, profileID string) http.Header {
	headers := http.Header{}
	if token != "" {
		headers.Set("Authorization", "Bearer "+token)
	}
	if profileID != "" {
		headers.Set("X-Profile-Id", profileID)
	}
	return headers
}

func epochEnforcementDial(t *testing.T, srv *httptest.Server, headers http.Header) *websocket.Conn {
	t.Helper()
	conn, response, err := websocket.DefaultDialer.Dial(epochEnforcementEndpoint(t, srv), headers)
	if response != nil {
		defer response.Body.Close()
	}
	require.NoError(t, err)
	return conn
}

func epochEnforcementRead(t *testing.T, conn *websocket.Conn) wsOutbound {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	_, body, err := conn.ReadMessage()
	require.NoError(t, err)
	var message wsOutbound
	require.NoError(t, json.Unmarshal(body, &message))
	return message
}

func epochEnforcementRequireRevokedClose(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	_, _, err := conn.ReadMessage()
	require.Error(t, err)
	var closeError *websocket.CloseError
	require.ErrorAs(t, err, &closeError)
	require.Equal(t, websocket.ClosePolicyViolation, closeError.Code)
	require.Equal(t, "session_revoked", closeError.Text)
}
