package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"voice/backend/pkg/principal"
)

func TestChatPrincipalIssuerLoadsRotationKeysAndSignsChatIdentity(t *testing.T) {
	directory := t.TempDir()
	writeChatPrincipalKey(t, directory, "active")
	writeChatPrincipalKey(t, directory, "next")
	t.Setenv("CHAT_PRINCIPAL_SIGNING_KEYS_DIR", directory)
	t.Setenv("CHAT_PRINCIPAL_ACTIVE_KID", "active")

	issuer, jwks, err := loadChatPrincipalIssuerFromEnv()
	require.NoError(t, err)
	require.NotNil(t, issuer)
	require.Len(t, jwks.Keys, 2)
	document, err := json.Marshal(jwks)
	require.NoError(t, err)
	keys, err := principal.ParseJWKS(document)
	require.NoError(t, err)
	token, err := issuer.IssueService(principal.ServiceInput{
		Audience: "messaging", RPC: "/voice.messaging.v1.MessagingService/GetSpacePurgeReceipt",
		RequestID: "operation-1", RequestHash: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	})
	require.NoError(t, err)
	_, err = principal.VerifyService(context.Background(), token, principal.VerifyConfig{
		ExpectedIssuer: "chat", ExpectedAudience: "messaging",
		ExpectedRPC:       "/voice.messaging.v1.MessagingService/GetSpacePurgeReceipt",
		ExpectedRequestID: "operation-1", ExpectedRequestHash: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		KeyResolver: func(_ context.Context, _, kid string) (*rsa.PublicKey, error) { return keys[kid], nil },
	})
	require.NoError(t, err)
}

func TestChatPrincipalIssuerRejectsPartialConfiguration(t *testing.T) {
	t.Setenv("CHAT_PRINCIPAL_SIGNING_KEYS_DIR", t.TempDir())
	t.Setenv("CHAT_PRINCIPAL_ACTIVE_KID", "")
	_, _, err := loadChatPrincipalIssuerFromEnv()
	require.Error(t, err)
}

func TestChatPrincipalJWKSHandlerPublishesOnlyReadEndpoint(t *testing.T) {
	document := chatPrincipalJWKS{Keys: []chatPrincipalJWK{{Kid: "active", Kty: "RSA", Use: "sig", Alg: "RS256"}}}
	handler := chatPrincipalJWKSHandler(document)
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/.well-known/principal-jwks.json", nil))
	require.Equal(t, http.StatusOK, get.Code)
	require.Equal(t, "application/json", get.Header().Get("Content-Type"))
	require.Contains(t, get.Body.String(), `"kid":"active"`)

	post := httptest.NewRecorder()
	handler.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/.well-known/principal-jwks.json", nil))
	require.Equal(t, http.StatusMethodNotAllowed, post.Code)
	require.Equal(t, http.MethodGet, post.Header().Get("Allow"))
}

func writeChatPrincipalKey(t *testing.T, directory, kid string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	path := filepath.Join(directory, kid+".pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0o600))
}
