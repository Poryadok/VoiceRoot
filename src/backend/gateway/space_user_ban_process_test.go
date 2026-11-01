//go:build spacebanintegration

package main

import (
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

// Invoked by the Space/User process integration fixture. Login claims are
// controlled; the Gateway HTTP route and protected Space transport are real.
func TestSpaceUserBanGatewayProcess(t *testing.T) {
	address := os.Getenv("VOICE_BAN_SPACE_ADDR")
	if address == "" {
		t.Skip("invoked by the Space/User Ban process fixture")
	}
	read := func(name string) string { value := os.Getenv(name); require.NotEmpty(t, value, name); return value }
	config := lifecycleClientConfig{address: address, ca: read("VOICE_BAN_CA"), serverName: "localhost", cert: read("VOICE_BAN_CLIENT_CERT"), key: read("VOICE_BAN_CLIENT_KEY")}
	conn, err := config.dial()
	require.NoError(t, err)
	defer conn.Close()
	keyPEM, err := os.ReadFile(read("VOICE_BAN_SIGNING_KEY"))
	require.NoError(t, err)
	block, _ := pem.Decode(keyPEM)
	require.NotNil(t, block)
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	account, owner, space, target := read("VOICE_BAN_ACTOR_ACCOUNT"), read("VOICE_BAN_ACTOR_PROFILE"), read("VOICE_BAN_SPACE"), read("VOICE_BAN_TARGET_ACCOUNT")
	fixture := newCreatorFixture(t)
	fixture.payload["sub"] = account
	fixture.payload["user_id"] = account
	fixture.payload["profile_id"] = owner
	fixture.redis.Set("auth:session:min_epoch:"+account, "7")
	fixture.issuer = issuer
	handler := newGateway(fixture.config(spacev1.NewSpaceServiceClient(conn)))
	body := `{"operation_id":"` + uuid.NewString() + `","account_id":"` + target + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/"+space+"/bans", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+signConfigJWT(t, "creator-key", fixture.key, fixture.payload))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
}
