//go:build linux && live

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	composeSpaceTokenReconciliationUnavailable = "Space media reconciliation is not ready"
	composeSpaceTokenRetryMaxAttempts          = 6
	composeSpaceTokenRetryWindow               = 6 * time.Second
)

var composeSpaceTokenRetryDelays = [...]time.Duration{
	100 * time.Millisecond,
	200 * time.Millisecond,
	400 * time.Millisecond,
	800 * time.Millisecond,
	800 * time.Millisecond,
}

type composeJoinTokenFailure struct {
	Code    string `json:"error_code"`
	Message string `json:"message"`
}

func requestComposeJoinToken(
	ctx context.Context,
	client *http.Client,
	base, accessToken, roomID string,
	wait func(context.Context, time.Duration) error,
) (composeJoinToken, error) {
	ctx, cancel := context.WithTimeout(ctx, composeSpaceTokenRetryWindow)
	defer cancel()
	url := fmt.Sprintf("%s/api/v1/voice/calls/%s/token", base, roomID)
	var last composeJoinTokenFailure
	var lastStatus int

	for attempt := 1; attempt <= composeSpaceTokenRetryMaxAttempts; attempt++ {
		// Every attempt is a new GET. A rejected readiness response has not issued a token.
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return composeJoinToken{}, fmt.Errorf("build Space voice token request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err := client.Do(req)
		if err != nil {
			return composeJoinToken{}, fmt.Errorf("send Space voice token request: %w", err)
		}
		if resp.StatusCode == http.StatusOK {
			var result composeJoinToken
			decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&result)
			_ = resp.Body.Close()
			if decodeErr != nil {
				return composeJoinToken{}, fmt.Errorf("decode successful Space voice token response: %w", decodeErr)
			}
			return result, nil
		}

		lastStatus = resp.StatusCode
		last = composeJoinTokenFailure{}
		decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&last)
		_ = resp.Body.Close()
		if decodeErr != nil {
			return composeJoinToken{}, fmt.Errorf("Space voice token failed: HTTP %d response did not contain a valid public error", lastStatus)
		}
		if lastStatus != http.StatusServiceUnavailable || last.Code != "unavailable" || last.Message != composeSpaceTokenReconciliationUnavailable {
			return composeJoinToken{}, fmt.Errorf("Space voice token failed: HTTP %d code=%q message=%q", lastStatus, last.Code, last.Message)
		}
		if attempt == composeSpaceTokenRetryMaxAttempts {
			break
		}
		if err := wait(ctx, composeSpaceTokenRetryDelays[attempt-1]); err != nil {
			return composeJoinToken{}, fmt.Errorf("Space media reconciliation remained unavailable after %d attempts: code=%q message=%q: %w", attempt, last.Code, last.Message, err)
		}
	}
	return composeJoinToken{}, fmt.Errorf("Space media reconciliation remained unavailable after %d attempts: code=%q message=%q", composeSpaceTokenRetryMaxAttempts, last.Code, last.Message)
}

func waitComposeSpaceTokenRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func getComposeSpaceMediaJoinToken(t *testing.T, client *http.Client, base, accessToken, roomID string) composeJoinToken {
	t.Helper()
	parsed, err := requestComposeJoinToken(context.Background(), client, base, accessToken, roomID, waitComposeSpaceTokenRetry)
	require.NoError(t, err)
	require.NotEmpty(t, parsed.JWT)
	return parsed
}

// TestComposeSpaceVoiceRoomBidirectionalAudio_live proves that two distinct
// Space members can publish and receive actual LiveKit audio for one room.
func TestComposeSpaceVoiceRoomBidirectionalAudio_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()
	owner := registerComposeUser(t, client, base, formatComposeEmail("space-media-owner", n), "VoiceQaTest1!")
	member := registerComposeUser(t, client, base, formatComposeEmail("space-media-member", n), "VoiceQaTest1!")
	// Default invite privacy allows friends, so establish that relationship
	// before setting up the two-member Space media fixture.
	sendComposeFriendInvitation(t, client, base, owner.AccessToken, member.ProfileID)
	acceptComposeFriendInvitation(t, client, base, member.AccessToken, owner.ProfileID)

	spaceID := createComposeSpace(t, client, base, owner.AccessToken, "Space media QA", "two-member bidirectional media")
	invite := createComposeSpaceInvite(t, client, base, owner.AccessToken, spaceID)
	joinComposeSpaceByInvite(t, client, base, member.AccessToken, invite.Code)
	voiceRoomID := createComposeSpaceVoiceRoom(t, client, base, owner.AccessToken, spaceID, "Media")

	joinOwner := joinComposeSpaceVoiceRoom(t, client, base, owner.AccessToken, voiceRoomID, spaceID)
	defer leaveComposeSpaceVoiceRoom(t, client, base, owner.AccessToken, voiceRoomID)
	joinMember := joinComposeSpaceVoiceRoom(t, client, base, member.AccessToken, voiceRoomID, spaceID)
	defer leaveComposeSpaceVoiceRoom(t, client, base, member.AccessToken, voiceRoomID)
	require.NotEmpty(t, joinOwner.RoomID)
	require.Equal(t, joinOwner.RoomID, joinMember.RoomID)
	require.NotEmpty(t, joinOwner.LivekitRoomName)
	require.Equal(t, joinOwner.LivekitRoomName, joinMember.LivekitRoomName)

	tokenOwner := getComposeSpaceMediaJoinToken(t, client, base, owner.AccessToken, joinOwner.RoomID)
	tokenMember := getComposeSpaceMediaJoinToken(t, client, base, member.AccessToken, joinMember.RoomID)
	identityOwner := livekitTokenIdentity(t, tokenOwner.JWT)
	identityMember := livekitTokenIdentity(t, tokenMember.JWT)
	require.True(t, strings.HasPrefix(identityOwner, "profile:"+owner.ProfileID+":media:"))
	require.True(t, strings.HasPrefix(identityMember, "profile:"+member.ProfileID+":media:"))
	require.NotEqual(t, identityOwner, identityMember)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	livekitURL := resolveComposeLivekitURL(tokenOwner.LivekitURL, tokenMember.LivekitURL)
	peerOwner := newLivekitCallPeer(ctx, t, livekitURL, tokenOwner.JWT, identityMember)
	defer peerOwner.close()
	peerMember := newLivekitCallPeer(ctx, t, livekitURL, tokenMember.JWT, identityOwner)
	defer peerMember.close()

	require.NoError(t, peerOwner.waitRemoteAudioRTP(45*time.Second), "Space owner did not receive member RTP audio")
	require.NoError(t, peerMember.waitRemoteAudioRTP(45*time.Second), "Space member did not receive owner RTP audio")
}

type composeSpaceVoiceRoomJoin struct {
	RoomID          string `json:"room_id"`
	LivekitRoomName string `json:"livekit_room_name"`
}

const (
	composeSpaceJoinMaxAttempts = 6
	composeSpaceJoinRetryWindow = 6 * time.Second
)

var composeSpaceJoinRetryDelays = [...]time.Duration{
	100 * time.Millisecond,
	200 * time.Millisecond,
	400 * time.Millisecond,
	800 * time.Millisecond,
	800 * time.Millisecond,
}

const composeSpaceAdmissionUnavailable = "Space media admission is not ready"

func joinComposeSpaceVoiceRoom(t *testing.T, client *http.Client, base, accessToken, voiceRoomID, spaceID string) composeSpaceVoiceRoomJoin {
	t.Helper()
	result, err := requestComposeSpaceVoiceRoomJoin(context.Background(), client, base, accessToken, voiceRoomID, spaceID, waitComposeSpaceJoinRetry)
	require.NoError(t, err)
	return result
}

type composeSpaceJoinFailure struct {
	Code    string `json:"error_code"`
	Message string `json:"message"`
}

func requestComposeSpaceVoiceRoomJoin(
	ctx context.Context,
	client *http.Client,
	base, accessToken, voiceRoomID, spaceID string,
	wait func(context.Context, time.Duration) error,
) (composeSpaceVoiceRoomJoin, error) {
	ctx, cancel := context.WithTimeout(ctx, composeSpaceJoinRetryWindow)
	defer cancel()
	url := base + "/api/v1/voice/rooms/" + voiceRoomID + "/join"
	body := fmt.Sprintf(`{"space":{"id":%q}}`, spaceID)
	var last composeSpaceJoinFailure
	var lastStatus int

	for attempt := 1; attempt <= composeSpaceJoinMaxAttempts; attempt++ {
		// Build a fresh request and body every time; retrying a consumed body would
		// turn the second attempt into a different operation.
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
		if err != nil {
			return composeSpaceVoiceRoomJoin{}, fmt.Errorf("build Space voice join request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return composeSpaceVoiceRoomJoin{}, fmt.Errorf("send Space voice join request: %w", err)
		}
		if resp.StatusCode == http.StatusOK {
			var result struct {
				VoiceSession composeSpaceVoiceRoomJoin `json:"voice_session"`
			}
			decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&result)
			_ = resp.Body.Close()
			if decodeErr != nil {
				return composeSpaceVoiceRoomJoin{}, fmt.Errorf("decode successful Space voice join response: %w", decodeErr)
			}
			return result.VoiceSession, nil
		}

		lastStatus = resp.StatusCode
		last = composeSpaceJoinFailure{}
		decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&last)
		_ = resp.Body.Close()
		if decodeErr != nil {
			return composeSpaceVoiceRoomJoin{}, fmt.Errorf("Space voice join failed: HTTP %d response did not contain a valid public error", lastStatus)
		}
		if lastStatus != http.StatusServiceUnavailable || last.Code != "unavailable" || last.Message != composeSpaceAdmissionUnavailable {
			return composeSpaceVoiceRoomJoin{}, fmt.Errorf("Space voice join failed: HTTP %d code=%q message=%q", lastStatus, last.Code, last.Message)
		}
		if attempt == composeSpaceJoinMaxAttempts {
			break
		}
		if err := wait(ctx, composeSpaceJoinRetryDelays[attempt-1]); err != nil {
			return composeSpaceVoiceRoomJoin{}, fmt.Errorf("Space voice join readiness remained unavailable after %d attempts: code=%q message=%q: %w", attempt, last.Code, last.Message, err)
		}
	}
	return composeSpaceVoiceRoomJoin{}, fmt.Errorf("Space voice join readiness remained unavailable after %d attempts: code=%q message=%q", composeSpaceJoinMaxAttempts, last.Code, last.Message)
}

func waitComposeSpaceJoinRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type composeJoinRoundTripper func(*http.Request) (*http.Response, error)

func (f composeJoinRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type composeJoinResponseBody struct {
	reader io.Reader
	closed bool
}

func (b *composeJoinResponseBody) Read(p []byte) (int, error) { return b.reader.Read(p) }
func (b *composeJoinResponseBody) Close() error               { b.closed = true; return nil }

func TestRequestComposeSpaceVoiceRoomJoinRetriesOnlyAdmissionReadiness(t *testing.T) {
	const (
		spaceID = "space-123"
		roomID  = "voice-room-456"
		token   = "fixture-token"
	)
	wantRequestBody := `{"space":{"id":"space-123"}}`
	var requests []*http.Request
	var requestBodies []string
	var responseBodies []*composeJoinResponseBody
	client := &http.Client{Transport: composeJoinRoundTripper(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req)
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		requestBodies = append(requestBodies, string(body))
		require.Equal(t, "Bearer "+token, req.Header.Get("Authorization"))
		status, payload := http.StatusServiceUnavailable,
			`{"error_code":"unavailable","message":"`+composeSpaceAdmissionUnavailable+`"}`
		if len(requests) == 2 {
			status, payload = http.StatusOK, `{"voice_session":{"room_id":"room","livekit_room_name":"lk-room"}}`
		}
		responseBody := &composeJoinResponseBody{reader: strings.NewReader(payload)}
		responseBodies = append(responseBodies, responseBody)
		return &http.Response{StatusCode: status, Body: responseBody, Header: make(http.Header)}, nil
	})}
	var waits []time.Duration
	wait := func(ctx context.Context, delay time.Duration) error {
		require.NoError(t, ctx.Err())
		waits = append(waits, delay)
		return nil
	}

	got, err := requestComposeSpaceVoiceRoomJoin(context.Background(), client, "http://fixture", token, roomID, spaceID, wait)
	require.NoError(t, err)
	require.Equal(t, composeSpaceVoiceRoomJoin{RoomID: "room", LivekitRoomName: "lk-room"}, got)
	require.Len(t, requests, 2)
	require.NotSame(t, requests[0], requests[1])
	require.Equal(t, []string{wantRequestBody, wantRequestBody}, requestBodies)
	require.Equal(t, []time.Duration{100 * time.Millisecond}, waits)
	require.Len(t, responseBodies, 2)
	for _, responseBody := range responseBodies {
		require.True(t, responseBody.closed, "every response body must be closed")
	}
}

func TestRequestComposeSpaceVoiceRoomJoinDoesNotRetryOtherFailures(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		payload string
	}{
		{name: "wrong status", status: http.StatusBadGateway, payload: `{"error_code":"unavailable","message":"` + composeSpaceAdmissionUnavailable + `"}`},
		{name: "wrong code", status: http.StatusServiceUnavailable, payload: `{"error_code":"failed_precondition","message":"` + composeSpaceAdmissionUnavailable + `"}`},
		{name: "wrong message", status: http.StatusServiceUnavailable, payload: `{"error_code":"unavailable","message":"Space voice media authority unavailable"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			body := &composeJoinResponseBody{reader: strings.NewReader(tc.payload)}
			client := &http.Client{Transport: composeJoinRoundTripper(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: tc.status, Body: body, Header: make(http.Header)}, nil
			})}
			waits := 0
			_, err := requestComposeSpaceVoiceRoomJoin(context.Background(), client, "http://fixture", "token", "room", "space", func(context.Context, time.Duration) error {
				waits++
				return nil
			})
			require.Error(t, err)
			require.Equal(t, 1, calls)
			require.Zero(t, waits)
			require.True(t, body.closed)
		})
	}

	calls := 0
	client := &http.Client{Transport: composeJoinRoundTripper(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("fixture transport failure")
	})}
	_, err := requestComposeSpaceVoiceRoomJoin(context.Background(), client, "http://fixture", "token", "room", "space", func(context.Context, time.Duration) error {
		t.Fatal("transport failures must not be retried")
		return nil
	})
	require.Error(t, err)
	require.Equal(t, 1, calls)
}

func TestRequestComposeSpaceVoiceRoomJoinCapsPersistentReadinessRetries(t *testing.T) {
	calls := 0
	var bodies []*composeJoinResponseBody
	client := &http.Client{Transport: composeJoinRoundTripper(func(*http.Request) (*http.Response, error) {
		calls++
		body := &composeJoinResponseBody{reader: strings.NewReader(`{"error_code":"unavailable","message":"` + composeSpaceAdmissionUnavailable + `"}`)}
		bodies = append(bodies, body)
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: body, Header: make(http.Header)}, nil
	})}
	var waits []time.Duration
	_, err := requestComposeSpaceVoiceRoomJoin(context.Background(), client, "http://fixture", "token", "room", "space", func(ctx context.Context, delay time.Duration) error {
		require.NoError(t, ctx.Err())
		waits = append(waits, delay)
		return nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "after 6 attempts")
	require.Contains(t, err.Error(), `code="unavailable"`)
	require.Contains(t, err.Error(), `message="`+composeSpaceAdmissionUnavailable+`"`)
	require.Equal(t, composeSpaceJoinMaxAttempts, calls)
	require.Len(t, waits, composeSpaceJoinMaxAttempts-1)
	require.Equal(t, composeSpaceJoinRetryDelays[:], waits)
	for _, body := range bodies {
		require.True(t, body.closed, "every retry response body must be closed")
	}
}

func TestRequestComposeJoinTokenRetriesOnlyReconciliationReadiness(t *testing.T) {
	const (
		roomID = "voice-room-456"
		token  = "fixture-token"
	)
	var requests []*http.Request
	var responseBodies []*composeJoinResponseBody
	client := &http.Client{Transport: composeJoinRoundTripper(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req)
		require.Equal(t, http.MethodGet, req.Method)
		require.Equal(t, "/api/v1/voice/calls/"+roomID+"/token", req.URL.Path)
		require.Equal(t, "Bearer "+token, req.Header.Get("Authorization"))
		status, payload := http.StatusServiceUnavailable,
			`{"error_code":"unavailable","message":"`+composeSpaceTokenReconciliationUnavailable+`"}`
		if len(requests) == 2 {
			status, payload = http.StatusOK, `{"jwt":"fixture-jwt","livekit_url":"wss://fixture"}`
		}
		body := &composeJoinResponseBody{reader: strings.NewReader(payload)}
		responseBodies = append(responseBodies, body)
		return &http.Response{StatusCode: status, Body: body, Header: make(http.Header)}, nil
	})}
	var waits []time.Duration
	wait := func(ctx context.Context, delay time.Duration) error {
		require.NoError(t, ctx.Err())
		waits = append(waits, delay)
		return nil
	}

	got, err := requestComposeJoinToken(context.Background(), client, "http://fixture", token, roomID, wait)
	require.NoError(t, err)
	require.Equal(t, composeJoinToken{JWT: "fixture-jwt", LivekitURL: "wss://fixture"}, got)
	require.Len(t, requests, 2)
	require.NotSame(t, requests[0], requests[1])
	require.Equal(t, []time.Duration{composeSpaceTokenRetryDelays[0]}, waits)
	require.Len(t, responseBodies, 2)
	for _, body := range responseBodies {
		require.True(t, body.closed, "every retry response body must be closed")
	}
}

func TestRequestComposeJoinTokenDoesNotRetryOtherFailures(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		payload string
	}{
		{name: "wrong status", status: http.StatusBadGateway, payload: `{"error_code":"unavailable","message":"` + composeSpaceTokenReconciliationUnavailable + `"}`},
		{name: "wrong code", status: http.StatusServiceUnavailable, payload: `{"error_code":"failed_precondition","message":"` + composeSpaceTokenReconciliationUnavailable + `"}`},
		{name: "wrong message", status: http.StatusServiceUnavailable, payload: `{"error_code":"unavailable","message":"Space media admission journal is not ready"}`},
		{name: "malformed body", status: http.StatusServiceUnavailable, payload: `{`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls, waits := 0, 0
			body := &composeJoinResponseBody{reader: strings.NewReader(tc.payload)}
			client := &http.Client{Transport: composeJoinRoundTripper(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: tc.status, Body: body, Header: make(http.Header)}, nil
				})}
			_, err := requestComposeJoinToken(context.Background(), client, "http://fixture", "token", "room", func(context.Context, time.Duration) error {
				waits++
				return nil
			})
			require.Error(t, err)
			require.Equal(t, 1, calls)
			require.Zero(t, waits)
			require.True(t, body.closed)
		})
	}

	calls := 0
	client := &http.Client{Transport: composeJoinRoundTripper(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("fixture transport failure")
	})}
	_, err := requestComposeJoinToken(context.Background(), client, "http://fixture", "token", "room", func(context.Context, time.Duration) error {
		t.Fatal("transport failures must not be retried")
		return nil
	})
	require.Error(t, err)
	require.Equal(t, 1, calls)
}

func TestRequestComposeJoinTokenCapsPersistentReadinessRetries(t *testing.T) {
	calls := 0
	var bodies []*composeJoinResponseBody
	client := &http.Client{Transport: composeJoinRoundTripper(func(*http.Request) (*http.Response, error) {
		calls++
		body := &composeJoinResponseBody{reader: strings.NewReader(`{"error_code":"unavailable","message":"` + composeSpaceTokenReconciliationUnavailable + `"}`)}
		bodies = append(bodies, body)
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: body, Header: make(http.Header)}, nil
	})}
	var waits []time.Duration
	_, err := requestComposeJoinToken(context.Background(), client, "http://fixture", "token", "room", func(ctx context.Context, delay time.Duration) error {
		require.NoError(t, ctx.Err())
		waits = append(waits, delay)
		return nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "after 6 attempts")
	require.Contains(t, err.Error(), `code="unavailable"`)
	require.Contains(t, err.Error(), `message="`+composeSpaceTokenReconciliationUnavailable+`"`)
	require.Equal(t, composeSpaceTokenRetryMaxAttempts, calls)
	require.Len(t, waits, composeSpaceTokenRetryMaxAttempts-1)
	require.Equal(t, composeSpaceTokenRetryDelays[:], waits)
	for _, body := range bodies {
		require.True(t, body.closed, "every retry response body must be closed")
	}
}

func leaveComposeSpaceVoiceRoom(t *testing.T, client *http.Client, base, accessToken, voiceRoomID string) {
	t.Helper()
	diagnosticID := fmt.Sprintf("voice503-leave-%d", time.Now().UnixNano())
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/voice/rooms/"+voiceRoomID+"/leave", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Request-Id", diagnosticID)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		logComposeSpaceLeaveDiagnostic(t, resp, diagnosticID)
	}
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func livekitTokenIdentity(t *testing.T, token string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims struct {
		Sub string `json:"sub"`
	}
	require.NoError(t, json.Unmarshal(claimsJSON, &claims))
	require.NotEmpty(t, claims.Sub)
	return claims.Sub
}
