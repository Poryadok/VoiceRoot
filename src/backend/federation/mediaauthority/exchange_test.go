package mediaauthority

import (
	"bytes"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func grantFixture(t *testing.T) (map[string]ed25519.PublicKey, ed25519.PrivateKey, Grant, time.Time) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	grant := Grant{Version: 1, Issuer: "master", Audience: "voice-node-media", Environment: "sandbox", NodeID: uuid.NewString(), SpaceID: uuid.NewString(), Generation: 1, AuthorityEpoch: 1, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), ResourceID: uuid.NewString(), SessionEpoch: 1, RoomName: "canonical-room", RoutingGeneration: 1, Nonce: uuid.NewString(), IssuedAt: now.UnixMilli(), ExpiresAt: now.Add(30 * time.Second).UnixMilli()}
	return map[string]ed25519.PublicKey{"current": public}, private, grant, now
}

func TestNodeExchangeUsesOnlyVerifiedGrantAndCurrentPolicyForPrivateJWT(t *testing.T) {
	public, private, grant, now := grantFixture(t)
	registry, err := NewRegistry(Verifier{Issuer: grant.Issuer, Environment: grant.Environment, NodeID: grant.NodeID, Keys: public}, 250*time.Millisecond)
	require.NoError(t, err)
	require.NoError(t, registry.Apply(authorityBundle(t, private, grant, 1, now.Add(2*time.Second), now, true), now))
	endpoint, err := NewExchange(registry, "node-api-key", "node-local-secret", "wss://media.node.example")
	require.NoError(t, err)
	endpoint.now = func() time.Time { return now }
	credential, err := Sign(private, "current", grant, now)
	require.NoError(t, err)
	input := ExchangeRequest{Credential: credential, RoomName: grant.RoomName, ProfileID: grant.ProfileID}
	call := func(body any, secure bool) *httptest.ResponseRecorder {
		raw, e := json.Marshal(body)
		require.NoError(t, e)
		r := httptest.NewRequest(http.MethodPost, "/v1/media/token", bytes.NewReader(raw))
		if secure {
			r.TLS = &tls.ConnectionState{Version: tls.VersionTLS13}
		}
		response := httptest.NewRecorder()
		endpoint.ServeHTTP(response, r)
		return response
	}
	response := call(input, true)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	var result ExchangeResult
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.True(t, result.JWT != "")
	require.Equal(t, "wss://media.node.example", result.LivekitURL)
	require.LessOrEqual(t, result.ExpiresAt, grant.ExpiresAt)
	parts := strings.Split(result.JWT, ".")
	require.Len(t, parts, 3)
	mac := hmac.New(sha256.New, []byte("node-local-secret"))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err)
	require.True(t, hmac.Equal(signature, mac.Sum(nil)))
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(raw, &claims))
	require.True(t, claims[GrantClaim] == credential)
	require.True(t, claims["sub"] == grant.ProfileID)
	require.NotContains(t, claims, "metadata")
	require.NotContains(t, claims, "attributes")
	video := claims["video"].(map[string]any)
	require.False(t, video["canPublish"].(bool), "default grant is listen-only")
	require.False(t, video["canPublishData"].(bool))
	for name, change := range map[string]func(*ExchangeRequest){
		"room":       func(r *ExchangeRequest) { r.RoomName += "-other" },
		"profile":    func(r *ExchangeRequest) { r.ProfileID = grant.AccountID },
		"credential": func(r *ExchangeRequest) { r.Credential += "invalid" },
	} {
		t.Run(name, func(t *testing.T) {
			other := input
			change(&other)
			require.Equal(t, http.StatusForbidden, call(other, true).Code)
		})
	}
	injected := map[string]any{"credential": credential, "room_name": grant.RoomName, "profile_id": grant.ProfileID, "can_publish": true}
	require.Equal(t, http.StatusBadRequest, call(injected, true).Code)
	require.Equal(t, http.StatusForbidden, call(input, false).Code)
	require.NoError(t, registry.Apply(authorityBundle(t, private, grant, 2, now.Add(2*time.Second), now, false), now))
	require.Equal(t, http.StatusForbidden, call(input, true).Code, "previously signed bearer cannot bypass policy revocation")
	endpoint.now = func() time.Time { return now.Add(2 * time.Second) }
	require.Equal(t, http.StatusForbidden, call(input, true).Code)
}

func TestPublishingGrantRequiresCurrentPublishPermissionOnAdmissionAndActiveMedia(t *testing.T) {
	public, private, grant, now := grantFixture(t)
	registry, err := NewRegistry(Verifier{Issuer: grant.Issuer, Environment: grant.Environment, NodeID: grant.NodeID, Keys: public}, 250*time.Millisecond)
	require.NoError(t, err)
	listenPolicy := authorityBundle(t, private, grant, 1, now.Add(2*time.Second), now, true)
	grant.CanPublish = true
	credential, err := Sign(private, "current", grant, now)
	require.NoError(t, err)
	require.NoError(t, registry.Apply(listenPolicy, now))
	_, err = registry.Admit(credential, grant.RoomName, grant.ProfileID, now)
	require.ErrorIs(t, err, ErrDenied)
	require.NoError(t, registry.Apply(authorityBundle(t, private, grant, 2, now.Add(2*time.Second), now, true), now))
	admission, err := registry.Admit(credential, grant.RoomName, grant.ProfileID, now)
	require.NoError(t, err)
	require.True(t, admission.CanPublish())
	grant.CanPublish = false
	require.NoError(t, registry.Apply(authorityBundle(t, private, grant, 3, now.Add(2*time.Second), now, true), now))
	require.ErrorIs(t, registry.Check(admission, now), ErrDenied, "publish permission removal closes a prior publisher even when listening remains allowed")
}
