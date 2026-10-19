package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"voice/backend/pkg/integrationtest"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
	"voice/backend/pkg/principal"
)

func TestModerationJWKS_RawIdentityCannotEnterBootstrap(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	handler := moderationJWKSHandler([]principal.JWKSKey{{KeyID: "current", PublicKey: &key.PublicKey}})
	request := httptest.NewRequest(http.MethodGet, "https://private/internal/principals/jwks", nil)
	request.Header.Set("authorization", "Bearer raw-identity")
	request.Header.Set("x-service-name", "auth")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusForbidden, response.Code, "issuer bootstrap requires verified client certificate, never a raw header")
}

func TestModerationJWKS_PrivateMtlsStartsWithoutPeerOrDomainReadiness(t *testing.T) {
	f := integrationtest.NewSourceTLSFixture(t, authorityv1.AuthorityOwner_AUTHORITY_OWNER_USER)
	keys := []principal.JWKSKey{{KeyID: "current", PublicKey: &f.Current.PublicKey}, {KeyID: "next", PublicKey: &f.Next.PublicKey}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runtime, err := startModerationJWKS(ctx, keys, "127.0.0.1:0", f.Config.TLSCertFile, f.Config.TLSKeyFile, f.Config.ClientCAFile)
	require.NoError(t, err)
	defer runtime.Close()
	endpoint := "https://" + runtime.listener.Addr().String() + "/internal/principals/jwks"
	call := func(config *tls.Config) (*http.Response, error) {
		transport := &http.Transport{TLSClientConfig: config}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
		return client.Get(endpoint)
	}
	valid := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: f.Roots, Certificates: []tls.Certificate{f.ClientCertificate}}
	response, err := call(valid)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	published, err := principal.ParseJWKS(body)
	require.NoError(t, err)
	require.Len(t, published, 2)
	for name, config := range map[string]*tls.Config{
		"anonymous":  {MinVersion: tls.VersionTLS12, RootCAs: f.Roots},
		"wrong_ca":   {MinVersion: tls.VersionTLS12, RootCAs: x509.NewCertPool(), Certificates: []tls.Certificate{f.ClientCertificate}},
		"wrong_host": {MinVersion: tls.VersionTLS12, RootCAs: f.Roots, Certificates: []tls.Certificate{f.ClientCertificate}, ServerName: "wrong.invalid"},
	} {
		t.Run(name, func(t *testing.T) {
			response, err := call(config)
			if response != nil {
				_ = response.Body.Close()
			}
			require.Error(t, err)
		})
	}
	_, err = startModerationJWKS(ctx, keys, "127.0.0.1:0", f.Config.TLSCertFile, "", f.Config.ClientCAFile)
	require.Error(t, err, "partial workload TLS cannot open an anonymous fallback")
	cancel()
	select {
	case <-runtime.done:
	case <-time.After(3 * time.Second):
		t.Fatal("owned bootstrap listener did not stop")
	}
}
