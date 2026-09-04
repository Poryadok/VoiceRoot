package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	voicejwt "voice/backend/pkg/jwt"
)

func TestSessionEpochWebSocketStrictFloorAllowsCurrentAndNewerTokens(t *testing.T) {
	for _, tc := range []struct {
		name    string
		epoch   int64
		minimum int64
	}{
		{name: "current equal floor", epoch: 7, minimum: 7},
		{name: "newer than floor", epoch: 8, minimum: 7},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			events := []string{}
			validator := newWSSequenceValidator(&events, map[string][]wsValidationResult{
				"current": {{claims: tokenClaims{UserID: "account-1", JTI: "jti-1", SessionEpoch: tc.epoch}}},
			})
			blacklist := &wsRecordingBlacklist{events: &events}
			floor := &wsSequenceFloor{events: &events, results: []wsFloorResult{{minimum: tc.minimum}}}
			upstreamCalls := 0
			h := newGatewayForContract(t, gatewayTestOptions{
				tokenValidator:     validator,
				tokenBlacklist:     blacklist,
				sessionEpochStrict: true,
				sessionEpochFloor:  floor,
				realtimeUpstream: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					upstreamCalls++
					w.WriteHeader(http.StatusSwitchingProtocols)
				}),
			})

			rec := performRequest(h, http.MethodGet, "/ws", "", wsUpgradeHeaders("current"))
			if rec.Code != http.StatusSwitchingProtocols {
				t.Fatalf("status/body = %d %q", rec.Code, rec.Body.String())
			}
			if upstreamCalls != 1 || validator.calls != 1 || blacklist.calls != 1 || floor.calls != 1 {
				t.Fatalf("upstream/validator/blacklist/floor calls = %d/%d/%d/%d, want 1/1/1/1", upstreamCalls, validator.calls, blacklist.calls, floor.calls)
			}
			assertWSEvents(t, events, "validate:current", "blacklist:jti-1", "floor:account-1")
		})
	}
}

func TestSessionEpochWebSocketStrictFloorDeniesBeforeUpstream(t *testing.T) {
	for _, tc := range []struct {
		name           string
		claims         tokenClaims
		validationCode string
		blacklist      wsBlacklistResult
		floor          wsFloorResult
		wantStatus     int
		wantError      string
		wantBlacklist  int
		wantFloor      int
	}{
		{name: "revoked JTI", claims: tokenClaims{UserID: "account-1", JTI: "jti-1", SessionEpoch: 7}, blacklist: wsBlacklistResult{revoked: true}, floor: wsFloorResult{minimum: 7}, wantStatus: http.StatusUnauthorized, wantError: "token_revoked", wantBlacklist: 1},
		{name: "stale epoch", claims: tokenClaims{UserID: "account-1", JTI: "jti-1", SessionEpoch: 6}, floor: wsFloorResult{minimum: 7}, wantStatus: http.StatusUnauthorized, wantError: "token_revoked", wantBlacklist: 1, wantFloor: 1},
		{name: "invalid epoch", claims: tokenClaims{UserID: "account-1", JTI: "jti-1", SessionEpoch: 0}, floor: wsFloorResult{minimum: 7}, wantStatus: http.StatusUnauthorized, wantError: "invalid_token", wantBlacklist: 1},
		{name: "blacklist unavailable", claims: tokenClaims{UserID: "account-1", JTI: "jti-1", SessionEpoch: 7}, blacklist: wsBlacklistResult{err: errors.New("redis down")}, floor: wsFloorResult{minimum: 7}, wantStatus: http.StatusServiceUnavailable, wantError: "auth_unavailable", wantBlacklist: 1},
		{name: "floor unavailable", claims: tokenClaims{UserID: "account-1", JTI: "jti-1", SessionEpoch: 7}, floor: wsFloorResult{err: errors.New("redis down")}, wantStatus: http.StatusServiceUnavailable, wantError: "auth_unavailable", wantBlacklist: 1, wantFloor: 1},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			events := []string{}
			validator := newWSSequenceValidator(&events, map[string][]wsValidationResult{
				"token": {{claims: tc.claims, code: tc.validationCode}},
			})
			blacklist := &wsRecordingBlacklist{events: &events, results: []wsBlacklistResult{tc.blacklist}}
			floor := &wsSequenceFloor{events: &events, results: []wsFloorResult{tc.floor}}
			upstreamCalls := 0
			h := newGatewayForContract(t, gatewayTestOptions{
				tokenValidator:     validator,
				tokenBlacklist:     blacklist,
				sessionEpochStrict: true,
				sessionEpochFloor:  floor,
				realtimeUpstream: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					upstreamCalls++
					w.WriteHeader(http.StatusSwitchingProtocols)
				}),
			})

			rec := performRequest(h, http.MethodGet, "/ws", "", wsUpgradeHeaders("token"))
			if rec.Code != tc.wantStatus || !strings.Contains(rec.Body.String(), tc.wantError) {
				t.Fatalf("status/body = %d %q", rec.Code, rec.Body.String())
			}
			if upstreamCalls != 0 || validator.calls != 1 || blacklist.calls != tc.wantBlacklist || floor.calls != tc.wantFloor {
				t.Fatalf("upstream/validator/blacklist/floor calls = %d/%d/%d/%d, want 0/1/%d/%d", upstreamCalls, validator.calls, blacklist.calls, floor.calls, tc.wantBlacklist, tc.wantFloor)
			}
		})
	}
}

func TestSessionEpochWsTicketIssueUsesStrictChecksAndDoesNotIssueOnDeny(t *testing.T) {
	for _, tc := range []struct {
		name      string
		claims    tokenClaims
		blacklist wsBlacklistResult
		floor     wsFloorResult
		wantCode  int
		wantError string
	}{
		{name: "current", claims: tokenClaims{UserID: "account-1", JTI: "jti-1", SessionEpoch: 7}, floor: wsFloorResult{minimum: 7}, wantCode: http.StatusOK},
		{name: "revoked", claims: tokenClaims{UserID: "account-1", JTI: "jti-1", SessionEpoch: 7}, blacklist: wsBlacklistResult{revoked: true}, floor: wsFloorResult{minimum: 7}, wantCode: http.StatusUnauthorized, wantError: "token_revoked"},
		{name: "stale", claims: tokenClaims{UserID: "account-1", JTI: "jti-1", SessionEpoch: 6}, floor: wsFloorResult{minimum: 7}, wantCode: http.StatusUnauthorized, wantError: "token_revoked"},
		{name: "invalid epoch", claims: tokenClaims{UserID: "account-1", JTI: "jti-1"}, floor: wsFloorResult{minimum: 7}, wantCode: http.StatusUnauthorized, wantError: "invalid_token"},
		{name: "blacklist unavailable", claims: tokenClaims{UserID: "account-1", JTI: "jti-1", SessionEpoch: 7}, blacklist: wsBlacklistResult{err: errors.New("redis down")}, floor: wsFloorResult{minimum: 7}, wantCode: http.StatusServiceUnavailable, wantError: "auth_unavailable"},
		{name: "floor unavailable", claims: tokenClaims{UserID: "account-1", JTI: "jti-1", SessionEpoch: 7}, floor: wsFloorResult{err: errors.New("redis down")}, wantCode: http.StatusServiceUnavailable, wantError: "auth_unavailable"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			events := []string{}
			validator := newWSSequenceValidator(&events, map[string][]wsValidationResult{
				"issue-token": {{claims: tc.claims}},
			})
			blacklist := &wsRecordingBlacklist{events: &events, results: []wsBlacklistResult{tc.blacklist}}
			floor := &wsSequenceFloor{events: &events, results: []wsFloorResult{tc.floor}}
			store := newWSRecordingTicketStore()
			h := newGatewayForContract(t, gatewayTestOptions{
				tokenValidator:     validator,
				tokenBlacklist:     blacklist,
				sessionEpochStrict: true,
				sessionEpochFloor:  floor,
				wsTicketStore:      store,
			})

			rec := performRequest(h, http.MethodPost, "/api/v1/realtime/ws-ticket", "", map[string]string{"Authorization": "Bearer issue-token"})
			if rec.Code != tc.wantCode {
				t.Fatalf("status/body = %d %q", rec.Code, rec.Body.String())
			}
			if tc.wantError != "" && !strings.Contains(rec.Body.String(), tc.wantError) {
				t.Fatalf("body = %q, want %q", rec.Body.String(), tc.wantError)
			}
			if tc.wantCode == http.StatusOK {
				if store.issueCalls != 1 || len(store.records) != 1 {
					t.Fatalf("ticket issue calls/records = %d/%d, want 1/1", store.issueCalls, len(store.records))
				}
				return
			}
			if store.issueCalls != 0 || len(store.records) != 0 {
				t.Fatalf("ticket issue calls/records = %d/%d, want 0/0", store.issueCalls, len(store.records))
			}
		})
	}
}

func TestSessionEpochWsTicketConsumeRevalidatesStoredTokenAndOverwritesClientAuthorization(t *testing.T) {
	events := []string{}
	freshClaims := tokenClaims{UserID: "account-1", ProfileID: "fresh-profile", JTI: "jti-a", SessionEpoch: 7}
	validator := newWSSequenceValidator(&events, map[string][]wsValidationResult{
		"token-a": {{claims: freshClaims}, {claims: freshClaims}},
		"token-b": {{claims: tokenClaims{UserID: "attacker", ProfileID: "attacker-profile", JTI: "jti-b", SessionEpoch: 99}}},
	})
	blacklist := &wsRecordingBlacklist{events: &events, results: []wsBlacklistResult{{}, {}}}
	floor := &wsSequenceFloor{events: &events, results: []wsFloorResult{{minimum: 7}, {minimum: 7}}}
	store := newWSRecordingTicketStore()
	var downstream http.Header
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenValidator:     validator,
		tokenBlacklist:     blacklist,
		sessionEpochStrict: true,
		sessionEpochFloor:  floor,
		wsTicketStore:      store,
		realtimeUpstream: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			downstream = r.Header.Clone()
			w.WriteHeader(http.StatusSwitchingProtocols)
		}),
	})

	ticket := issueWSTicket(t, h, "token-a")
	stored := store.records[ticket]
	stored.UserID = "snapshot-user"
	stored.ProfileID = "snapshot-profile"
	store.records[ticket] = stored

	rec := performRequest(h, http.MethodGet, "/ws?ticket="+ticket, "", wsUpgradeHeaders("token-b"))
	if rec.Code != http.StatusSwitchingProtocols {
		t.Fatalf("status/body = %d %q", rec.Code, rec.Body.String())
	}
	if got := downstream.Get("Authorization"); got != "Bearer token-a" {
		t.Fatalf("upstream Authorization = %q, want stored token-a", got)
	}
	if got := downstream.Get("X-Voice-User-Id"); got != "account-1" {
		t.Fatalf("upstream user = %q, want freshly validated account-1", got)
	}
	if got := downstream.Get("X-Voice-Profile-Id"); got != "fresh-profile" {
		t.Fatalf("upstream profile = %q, want freshly validated fresh-profile", got)
	}
	if validator.calls != 2 || blacklist.calls != 2 || floor.calls != 2 {
		t.Fatalf("validator/blacklist/floor calls = %d/%d/%d, want 2/2/2", validator.calls, blacklist.calls, floor.calls)
	}
	assertWSEvents(t, events,
		"validate:token-a", "blacklist:jti-a", "floor:account-1",
		"validate:token-a", "blacklist:jti-a", "floor:account-1",
	)
	second := performRequest(h, http.MethodGet, "/ws?ticket="+ticket, "", wsUpgradeHeaders("token-b"))
	if second.Code != http.StatusUnauthorized || !strings.Contains(second.Body.String(), "invalid_ticket") {
		t.Fatalf("second status/body = %d %q", second.Code, second.Body.String())
	}
}

func TestSessionEpochWsTicketConsumeFailsClosedAndSpendsTicket(t *testing.T) {
	for _, tc := range []struct {
		name             string
		validatorResults []wsValidationResult
		floorResults     []wsFloorResult
		revokeAfterIssue bool
		wantStatus       int
		wantError        string
	}{
		{name: "floor raised", validatorResults: []wsValidationResult{{claims: wsCurrentClaims()}, {claims: wsCurrentClaims()}}, floorResults: []wsFloorResult{{minimum: 7}, {minimum: 8}}, wantStatus: http.StatusUnauthorized, wantError: "token_revoked"},
		{name: "JTI revoked", validatorResults: []wsValidationResult{{claims: wsCurrentClaims()}, {claims: wsCurrentClaims()}}, floorResults: []wsFloorResult{{minimum: 7}, {minimum: 7}}, revokeAfterIssue: true, wantStatus: http.StatusUnauthorized, wantError: "token_revoked"},
		{name: "JWT expired or invalid", validatorResults: []wsValidationResult{{claims: wsCurrentClaims()}, {code: "invalid_token"}}, floorResults: []wsFloorResult{{minimum: 7}}, wantStatus: http.StatusUnauthorized, wantError: "invalid_token"},
		{name: "floor error", validatorResults: []wsValidationResult{{claims: wsCurrentClaims()}, {claims: wsCurrentClaims()}}, floorResults: []wsFloorResult{{minimum: 7}, {err: errors.New("redis down")}}, wantStatus: http.StatusServiceUnavailable, wantError: "auth_unavailable"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			events := []string{}
			validator := newWSSequenceValidator(&events, map[string][]wsValidationResult{"token-a": tc.validatorResults})
			blacklist := &wsRecordingBlacklist{events: &events, results: []wsBlacklistResult{{}, {}}}
			floor := &wsSequenceFloor{events: &events, results: tc.floorResults}
			store := newWSRecordingTicketStore()
			upstreamCalls := 0
			h := newGatewayForContract(t, gatewayTestOptions{
				tokenValidator:     validator,
				tokenBlacklist:     blacklist,
				sessionEpochStrict: true,
				sessionEpochFloor:  floor,
				wsTicketStore:      store,
				realtimeUpstream: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					upstreamCalls++
					w.WriteHeader(http.StatusSwitchingProtocols)
				}),
			})

			ticket := issueWSTicket(t, h, "token-a")
			if tc.revokeAfterIssue {
				blacklist.results[1].revoked = true
			}
			rec := performRequest(h, http.MethodGet, "/ws?ticket="+ticket, "", wsUpgradeHeaders("attacker-token"))
			if rec.Code != tc.wantStatus || !strings.Contains(rec.Body.String(), tc.wantError) {
				t.Fatalf("status/body = %d %q", rec.Code, rec.Body.String())
			}
			if upstreamCalls != 0 || validator.calls != 2 {
				t.Fatalf("upstream/validator calls = %d/%d, want 0/2", upstreamCalls, validator.calls)
			}
			repeat := performRequest(h, http.MethodGet, "/ws?ticket="+ticket, "", wsUpgradeHeaders("attacker-token"))
			if repeat.Code != http.StatusUnauthorized || !strings.Contains(repeat.Body.String(), "invalid_ticket") {
				t.Fatalf("repeat status/body = %d %q", repeat.Code, repeat.Body.String())
			}
		})
	}
}

func TestSessionEpochWsTicketWithoutUpgradeRemainsUnconsumed(t *testing.T) {
	events := []string{}
	claims := wsCurrentClaims()
	validator := newWSSequenceValidator(&events, map[string][]wsValidationResult{"token-a": {{claims: claims}, {claims: claims}}})
	blacklist := &wsRecordingBlacklist{events: &events, results: []wsBlacklistResult{{}, {}}}
	floor := &wsSequenceFloor{events: &events, results: []wsFloorResult{{minimum: 7}, {minimum: 7}}}
	store := newWSRecordingTicketStore()
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenValidator:     validator,
		tokenBlacklist:     blacklist,
		sessionEpochStrict: true,
		sessionEpochFloor:  floor,
		wsTicketStore:      store,
		realtimeUpstream:   http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusSwitchingProtocols) }),
	})

	ticket := issueWSTicket(t, h, "token-a")
	plain := performRequest(h, http.MethodGet, "/ws?ticket="+ticket, "", nil)
	if plain.Code != http.StatusBadRequest || !strings.Contains(plain.Body.String(), "websocket_upgrade_required") {
		t.Fatalf("plain status/body = %d %q", plain.Code, plain.Body.String())
	}
	if store.consumeCalls != 0 {
		t.Fatalf("ticket consume calls = %d, want 0 without Upgrade", store.consumeCalls)
	}
	upgrade := performRequest(h, http.MethodGet, "/ws?ticket="+ticket, "", wsUpgradeHeaders("attacker-token"))
	if upgrade.Code != http.StatusSwitchingProtocols {
		t.Fatalf("upgrade status/body = %d %q", upgrade.Code, upgrade.Body.String())
	}
}

func wsUpgradeHeaders(token string) map[string]string {
	return map[string]string{
		"Authorization":         "Bearer " + token,
		"Connection":            "Upgrade",
		"Upgrade":               "websocket",
		"Sec-WebSocket-Key":     "dGhlIHNhbXBsZSBub25jZQ==",
		"Sec-WebSocket-Version": "13",
	}
}

func issueWSTicket(t *testing.T, h http.Handler, token string) string {
	t.Helper()
	rec := performRequest(h, http.MethodPost, "/api/v1/realtime/ws-ticket", "", map[string]string{"Authorization": "Bearer " + token})
	if rec.Code != http.StatusOK {
		t.Fatalf("ticket issue status/body = %d %q", rec.Code, rec.Body.String())
	}
	var body map[string]any
	decodeJSON(t, rec.Body, &body)
	ticket, _ := body["ticket"].(string)
	if ticket == "" {
		t.Fatalf("missing ticket in %v", body)
	}
	return ticket
}

func wsCurrentClaims() tokenClaims {
	return tokenClaims{UserID: "account-1", ProfileID: "profile-1", JTI: "jti-a", SessionEpoch: 7}
}

type wsValidationResult struct {
	claims tokenClaims
	code   string
}

type wsSequenceValidator struct {
	events    *[]string
	results   map[string][]wsValidationResult
	positions map[string]int
	calls     int
}

func newWSSequenceValidator(events *[]string, results map[string][]wsValidationResult) *wsSequenceValidator {
	return &wsSequenceValidator{events: events, results: results, positions: map[string]int{}}
}

func (v *wsSequenceValidator) Validate(r *http.Request) (tokenClaims, string) {
	token := voicejwt.BearerToken(r)
	v.calls++
	*v.events = append(*v.events, "validate:"+token)
	results := v.results[token]
	position := v.positions[token]
	v.positions[token] = position + 1
	if len(results) == 0 {
		return tokenClaims{}, "invalid_token"
	}
	if position >= len(results) {
		position = len(results) - 1
	}
	return results[position].claims, results[position].code
}

type wsBlacklistResult struct {
	revoked bool
	err     error
}

type wsRecordingBlacklist struct {
	events  *[]string
	results []wsBlacklistResult
	calls   int
}

func (b *wsRecordingBlacklist) IsRevoked(_ context.Context, jti string) (bool, error) {
	*b.events = append(*b.events, "blacklist:"+jti)
	position := b.calls
	b.calls++
	if len(b.results) == 0 {
		return false, nil
	}
	if position >= len(b.results) {
		position = len(b.results) - 1
	}
	return b.results[position].revoked, b.results[position].err
}

type wsFloorResult struct {
	minimum int64
	err     error
}

type wsSequenceFloor struct {
	events  *[]string
	results []wsFloorResult
	calls   int
}

func (f *wsSequenceFloor) Minimum(_ context.Context, accountID string) (int64, error) {
	*f.events = append(*f.events, "floor:"+accountID)
	position := f.calls
	f.calls++
	if len(f.results) == 0 {
		return 0, errors.New("missing floor result")
	}
	if position >= len(f.results) {
		position = len(f.results) - 1
	}
	return f.results[position].minimum, f.results[position].err
}

type wsRecordingTicketStore struct {
	records      map[string]wsTicketRecord
	issueCalls   int
	consumeCalls int
}

func newWSRecordingTicketStore() *wsRecordingTicketStore {
	return &wsRecordingTicketStore{records: map[string]wsTicketRecord{}}
}

func (s *wsRecordingTicketStore) Issue(_ context.Context, record wsTicketRecord, _ time.Duration) (string, error) {
	s.issueCalls++
	ticket := fmt.Sprintf("ticket-%d", s.issueCalls)
	s.records[ticket] = record
	return ticket, nil
}

func (s *wsRecordingTicketStore) Consume(_ context.Context, ticket string) (wsTicketRecord, bool, error) {
	s.consumeCalls++
	record, ok := s.records[ticket]
	if !ok {
		return wsTicketRecord{}, false, nil
	}
	delete(s.records, ticket)
	return record, true, nil
}

func assertWSEvents(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("events = %v, want %v", got, want)
	}
}
