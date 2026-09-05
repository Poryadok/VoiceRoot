package main

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestMemoryWsTicketStore_IssueConsumeOnce(t *testing.T) {
	t.Parallel()

	store := newMemoryWsTicketStore()
	record := wsTicketRecord{
		UserID:        "acc-1",
		ProfileID:     "prof-1",
		UpstreamToken: "jwt-token",
	}
	ticket, err := store.Issue(context.Background(), record, time.Minute)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if ticket == "" {
		t.Fatal("ticket is empty")
	}

	got, ok, err := store.Consume(context.Background(), ticket)
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if !ok {
		t.Fatal("Consume() ok = false, want true")
	}
	if got.ProfileID != "prof-1" || got.UpstreamToken != "jwt-token" {
		t.Fatalf("Consume() = %+v", got)
	}

	_, ok, err = store.Consume(context.Background(), ticket)
	if err != nil {
		t.Fatalf("second Consume() error = %v", err)
	}
	if ok {
		t.Fatal("ticket must be single-use")
	}
}

func TestRedisWsTicketStore_IssueConsume(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	store := newRedisWsTicketStore(mr.Addr(), "", "")
	record := wsTicketRecord{UserID: "acc-1", ProfileID: "prof-1", UpstreamToken: "jwt"}
	ticket, err := store.Issue(context.Background(), record, time.Minute)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	got, ok, err := store.Consume(context.Background(), ticket)
	if err != nil || !ok || got.ProfileID != "prof-1" {
		t.Fatalf("Consume() = %+v, ok=%v, err=%v", got, ok, err)
	}
}

func TestRealtimeWsTicketEndpoint(t *testing.T) {
	t.Parallel()

	store := newMemoryWsTicketStore()
	h := newGatewayForContract(t, gatewayTestOptions{
		wsTicketStore: store,
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
	})

	rec := performRequest(h, http.MethodPost, "/api/v1/realtime/ws-ticket", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body map[string]any
	decodeJSON(t, rec.Body, &body)
	ticket, _ := body["ticket"].(string)
	if ticket == "" {
		t.Fatalf("ticket missing in %v", body)
	}
	if expires, _ := body["expires_in_seconds"].(float64); expires <= 0 {
		t.Fatalf("expires_in_seconds = %v", body["expires_in_seconds"])
	}
}

func TestWebSocketTicketUpgrade(t *testing.T) {
	t.Parallel()

	store := newMemoryWsTicketStore()
	record := wsTicketRecord{
		UserID:        "account-1",
		ProfileID:     "profile-1",
		UpstreamToken: "valid-user-token",
	}
	ticket, err := store.Issue(context.Background(), record, time.Minute)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	var downstream http.Header
	h := newGatewayForContract(t, gatewayTestOptions{
		wsTicketStore: store,
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		realtimeUpstream: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			downstream = r.Header.Clone()
			w.WriteHeader(http.StatusSwitchingProtocols)
		}),
	})

	rec := performRequest(h, http.MethodGet, "/ws?ticket="+ticket, "", map[string]string{
		"Connection":            "Upgrade",
		"Upgrade":               "websocket",
		"Sec-WebSocket-Key":     "dGhlIHNhbXBsZSBub25jZQ==",
		"Sec-WebSocket-Version": "13",
	})
	if rec.Code != http.StatusSwitchingProtocols {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusSwitchingProtocols, rec.Body.String())
	}
	if got := downstream.Get("Authorization"); got != "Bearer valid-user-token" {
		t.Fatalf("Authorization = %q, want Bearer valid-user-token", got)
	}
	if got := downstream.Get("X-Voice-Profile-Id"); got != "profile-1" {
		t.Fatalf("X-Voice-Profile-Id = %q, want profile-1", got)
	}
}

func TestWebSocketTicketSingleUse(t *testing.T) {
	t.Parallel()

	store := newMemoryWsTicketStore()
	record := wsTicketRecord{UserID: "account-1", ProfileID: "profile-1", UpstreamToken: "valid-user-token"}
	ticket, err := store.Issue(context.Background(), record, time.Minute)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	h := newGatewayForContract(t, gatewayTestOptions{
		wsTicketStore: store,
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		realtimeUpstream: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusSwitchingProtocols)
		}),
	})
	headers := map[string]string{
		"Connection":            "Upgrade",
		"Upgrade":               "websocket",
		"Sec-WebSocket-Key":     "dGhlIHNhbXBsZSBub25jZQ==",
		"Sec-WebSocket-Version": "13",
	}

	first := performRequest(h, http.MethodGet, "/ws?ticket="+ticket, "", headers)
	if first.Code != http.StatusSwitchingProtocols {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusSwitchingProtocols)
	}
	second := performRequest(h, http.MethodGet, "/ws?ticket="+ticket, "", headers)
	if second.Code != http.StatusUnauthorized {
		t.Fatalf("second status = %d, want %d; body=%q", second.Code, http.StatusUnauthorized, second.Body.String())
	}
}
