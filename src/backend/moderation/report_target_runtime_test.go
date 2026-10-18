package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/reporttargetprincipal"
)

func TestReportTargetRuntimeFixture(t *testing.T) {
	fixture := os.Getenv("REPORT_TARGET_RUNTIME_FIXTURE")
	if fixture == "" {
		t.Skip("generated isolated Linux PKI required")
	}
	const prefix = "MODERATION_REPORT_TARGETS_"
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())
	values := map[string]string{"SIGNING_KEYS_DIR": filepath.Join(fixture, "moderation"), "ACTIVE_KID": "current", "JWKS_LISTEN": addr, "JWKS_TLS_CERT_FILE": filepath.Join(fixture, "moderation.crt"), "JWKS_TLS_KEY_FILE": filepath.Join(fixture, "moderation.key"), "TLS_CA_FILE": filepath.Join(fixture, "ca.crt"), "CLIENT_CERT_FILE": filepath.Join(fixture, "moderation-client.crt"), "CLIENT_KEY_FILE": filepath.Join(fixture, "moderation-client.key")}
	for _, owner := range []string{"USER", "MESSAGING", "SPACE", "STORY"} {
		values[owner+"_GRPC_ADDR"] = "127.0.0.1:1"
		values[owner+"_TLS_SERVER_NAME"] = "fixture-unreachable"
	}
	for name, value := range values {
		t.Setenv(prefix+name, value)
	}
	client, closeRuntime, err := newReportTargetRuntime(context.Background(), slog.Default())
	require.NoError(t, err)
	defer closeRuntime()
	require.Len(t, client.Connections, 4)
	ca, err := os.ReadFile(filepath.Join(fixture, "ca.crt"))
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca))
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "moderation"}}
	defer transport.CloseIdleConnections()
	response, err := (&http.Client{Transport: transport}).Get("https://" + addr + "/.well-known/jwks.json")
	require.NoError(t, err)
	document, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode)
	keys, err := principal.ParseJWKS(document)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	require.NotNil(t, keys["current"])
	require.NotNil(t, keys["next"])
	hash, err := principal.RequestHash(&emptypb.Empty{})
	require.NoError(t, err)
	id := uuid.NewString()
	method := reporttargetprincipal.Method("user")
	token, err := client.Issuer.IssueService(principal.ServiceInput{Audience: "user", RPC: method, RequestID: id, RequestHash: hash})
	require.NoError(t, err)
	_, err = principal.VerifyService(context.Background(), token, principal.VerifyConfig{ExpectedIssuer: "moderation", ExpectedAudience: "user", ExpectedRPC: method, ExpectedRequestID: id, ExpectedRequestHash: hash, KeyResolver: func(_ context.Context, _ string, kid string) (*rsa.PublicKey, error) { return keys[kid], nil }})
	require.NoError(t, err)
	closeRuntime()
	t.Setenv(prefix+"STORY_TLS_SERVER_NAME", "")
	_, _, err = newReportTargetRuntime(context.Background(), slog.Default())
	require.Error(t, err, "partial owner matrix must fail startup")
}
