package s2s

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/messaging/internal/gameprotocol"
)

type executionPermitRoundTripper func(*http.Request) (*http.Response, error)

func (f executionPermitRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestGameMessagePermitClientIssuesExactBoundRequestAndCompletes(t *testing.T) {
	endpoint := mustPermitEndpoint(t)
	permitID, operationID := uuid.New(), uuid.New()
	mutation := []byte(`{"version":1,"operation":"create"}`)
	digest := sha256.Sum256(mutation)
	authority := gameprotocol.DeviceAuthority{AssertionJWS: "auth.assertion.signature"}
	var calls int
	client := &GameMessageExecutionPermitClient{endpoint: endpoint, client: &http.Client{Transport: executionPermitRoundTripper(func(request *http.Request) (*http.Response, error) {
		calls++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		if calls == 1 {
			require.Equal(t, http.MethodPost, request.Method)
			require.Equal(t, "/api/v1/auth/sdk/game-message/execution-permits", request.URL.Path)
			require.Equal(t, authority.AssertionJWS, request.Header.Get("X-Voice-Device-Authority"))
			require.Equal(t, "application/json", request.Header.Get("Content-Type"))
			require.Equal(t, "no-store", request.Header.Get("Cache-Control"))
			var payload map[string]any
			require.NoError(t, json.Unmarshal(body, &payload))
			require.Len(t, payload, 2)
			require.Equal(t, operationID.String(), payload["operation_id"])
			require.Equal(t, hex.EncodeToString(digest[:]), payload["request_sha256"])
			return permitHTTPResponse(http.StatusOK, `{"permit_jws":"one.two.three"}`), nil
		}
		require.Equal(t, http.MethodPost, request.Method)
		require.Equal(t, "/api/v1/auth/sdk/game-message/execution-permits/"+permitID.String()+"/completion", request.URL.Path)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(body, &payload))
		require.Equal(t, map[string]any{"operation_id": operationID.String(), "outcome": "committed"}, payload)
		return permitHTTPResponse(http.StatusOK, `{"permit_id":"`+permitID.String()+`","operation_id":"`+operationID.String()+`","outcome":"committed","status":"completed"}`), nil
	})}}
	permit, err := client.Issue(context.Background(), authority, operationID, mutation)
	require.NoError(t, err)
	require.Equal(t, "one.two.three", permit)
	require.NoError(t, client.Complete(context.Background(), permitID, operationID, "committed"))
	require.Equal(t, 2, calls)
}

func TestGameMessagePermitClientRejectsMalformedOrCacheablePermitResponses(t *testing.T) {
	endpoint := mustPermitEndpoint(t)
	for name, response := range map[string]*http.Response{
		"unknown field":      permitHTTPResponse(http.StatusOK, `{"permit_jws":"a.b.c","extra":true}`),
		"duplicate field":    permitHTTPResponse(http.StatusOK, `{"permit_jws":"a.b.c","permit_jws":"x.y.z"}`),
		"trailing JSON":      permitHTTPResponse(http.StatusOK, `{"permit_jws":"a.b.c"} {}`),
		"empty token":        permitHTTPResponse(http.StatusOK, `{"permit_jws":""}`),
		"cacheable":          permitHTTPResponseWithoutNoStore(http.StatusOK, `{"permit_jws":"a.b.c"}`),
		"wrong content type": permitHTTPResponseType(http.StatusOK, "text/plain", `{"permit_jws":"a.b.c"}`),
		"denied status":      permitHTTPResponse(http.StatusOK, `{"permit_jws":"a.b.c"}`),
	} {
		t.Run(name, func(t *testing.T) {
			if name == "denied status" {
				response.StatusCode = http.StatusForbidden
			}
			client := &GameMessageExecutionPermitClient{endpoint: endpoint, client: &http.Client{Transport: executionPermitRoundTripper(func(*http.Request) (*http.Response, error) { return response, nil })}}
			_, err := client.Issue(context.Background(), gameprotocol.DeviceAuthority{AssertionJWS: "assertion"}, uuid.New(), []byte(`{}`))
			require.Error(t, err)
		})
	}
}

func TestGameMessagePermitClientRequiresFixedHTTPSAndCompleteMTLSConfig(t *testing.T) {
	for _, config := range []GameMessageExecutionPermitConfig{
		{}, {Endpoint: "http://auth.internal/api/v1/auth/sdk/game-message/execution-permits", TLSCertFile: "cert", TLSKeyFile: "key", CAFile: "ca"},
		{Endpoint: "https://auth.internal/api/v1/auth/sdk/game-message/execution-permits", TLSCertFile: "cert", TLSKeyFile: "key"},
	} {
		_, err := NewGameMessageExecutionPermitClient(config)
		require.Error(t, err)
	}
}

func mustPermitEndpoint(t *testing.T) *url.URL {
	t.Helper()
	endpoint, err := url.Parse("https://auth.internal/api/v1/auth/sdk/game-message/execution-permits")
	require.NoError(t, err)
	return endpoint
}

func permitHTTPResponse(status int, body string) *http.Response {
	return permitHTTPResponseType(status, "application/json", body)
}

func permitHTTPResponseType(status int, contentType, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{contentType}, "Cache-Control": []string{"no-store"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func permitHTTPResponseWithoutNoStore(status int, body string) *http.Response {
	response := permitHTTPResponse(status, body)
	response.Header.Del("Cache-Control")
	return response
}
