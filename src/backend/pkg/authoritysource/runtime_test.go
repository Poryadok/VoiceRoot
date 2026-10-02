package authoritysource_test

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"voice/backend/pkg/authoritysource"
	"voice/backend/pkg/integrationtest"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
)

type runtimeReader struct {
	preflightErr error
	calls        atomic.Int64
}

func (r *runtimeReader) CheckAuthoritySourceSchema(context.Context) error { return r.preflightErr }
func (r *runtimeReader) ReadAuthoritySnapshot(context.Context, *authorityv1.SourceScope) (authoritysource.State, error) {
	r.calls.Add(1)
	return authoritysource.State{Complete: true, Revision: 7, CanonicalState: []byte(`{"state":"complete"}`)}, nil
}
func (r *runtimeReader) ReadAuthorityRevision(context.Context, *authorityv1.SourceScope) (uint64, error) {
	r.calls.Add(1)
	return 7, nil
}

func TestSourceRuntimeConfigRequiresExplicitOwnerActivationAndTrust(t *testing.T) {
	names := []string{"ENABLED", "JWKS_CA_FILE", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE", "REPLAY_REDIS_ADDR", "REPLAY_REDIS_PASSWORD", "GRPC_LISTEN"}
	for _, name := range names {
		full := "SPACE_AUTHORITY_SOURCE_" + name
		t.Setenv(full, "")
		require.NoError(t, os.Unsetenv(full))
	}
	t.Setenv("S2S_JWKS_URLS_JSON", `{"social":"https://social.internal/jwks"}`)
	_, enabled, err := authoritysource.LoadRuntimeConfig(authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE, ":9097")
	require.NoError(t, err)
	require.False(t, enabled)
	t.Setenv("SPACE_AUTHORITY_SOURCE_TLS_CERT_FILE", "fixture.crt")
	_, _, err = authoritysource.LoadRuntimeConfig(authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE, ":9097")
	require.Error(t, err)
	t.Setenv("SPACE_AUTHORITY_SOURCE_ENABLED", "false")
	_, enabled, err = authoritysource.LoadRuntimeConfig(authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE, ":9097")
	require.NoError(t, err)
	require.False(t, enabled)
	t.Setenv("SPACE_AUTHORITY_SOURCE_ENABLED", "true")
	for name, value := range map[string]string{"TLS_KEY_FILE": "fixture.key", "CLIENT_CA_FILE": "fixture-client-ca.pem", "REPLAY_REDIS_ADDR": "127.0.0.1:6379"} {
		t.Setenv("SPACE_AUTHORITY_SOURCE_"+name, value)
	}
	_, _, err = authoritysource.LoadRuntimeConfig(authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE, ":9097")
	require.Error(t, err, "shared unrelated keys cannot activate source trust")
	t.Setenv("S2S_JWKS_URLS_JSON", `{"federation":"https://federation.internal/jwks","social":"https://social.internal/jwks"}`)
	cfg, enabled, err := authoritysource.LoadRuntimeConfig(authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE, ":9097")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "https://federation.internal/jwks", cfg.JWKSURL)
	for _, flag := range []string{"", "yes", "1", "TRUE", " true "} {
		t.Setenv("SPACE_AUTHORITY_SOURCE_ENABLED", flag)
		_, _, err = authoritysource.LoadRuntimeConfig(authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE, ":9097")
		require.Error(t, err)
	}
}

func startSourceRuntime(t *testing.T, f integrationtest.SourceTLSFixture, reader *runtimeReader) (authorityv1.AuthoritySourceServiceClient, string) {
	t.Helper()
	runtime, err := authoritysource.NewRuntime(context.Background(), f.Config, reader)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	server := grpc.NewServer(runtime.ServerOptions()...)
	runtime.Register(server)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); <-done })
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(f.ClientCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	return authorityv1.NewAuthoritySourceServiceClient(conn), listener.Addr().String()
}

func TestSourceRuntimePreflightAndMTLSFailBeforeOwnerRead(t *testing.T) {
	f := integrationtest.NewSourceTLSFixture(t, authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE)
	bad := &runtimeReader{preflightErr: errors.New("dirty source")}
	runtime, err := authoritysource.NewRuntime(context.Background(), f.Config, bad)
	require.Error(t, err)
	require.Nil(t, runtime)
	require.Zero(t, bad.calls.Load())
	reader := &runtimeReader{}
	_, address := startSourceRuntime(t, f, reader)
	request := &authorityv1.ReadSnapshotRequest{Scope: &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: uuid.NewString()}}
	for name, transport := range map[string]credentials.TransportCredentials{
		"plaintext":             insecure.NewCredentials(),
		"no_client_certificate": credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: f.Roots}),
		"wrong_server":          credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: f.Roots, ServerName: "wrong.source.internal", Certificates: []tls.Certificate{f.ClientCertificate}}),
	} {
		t.Run(name, func(t *testing.T) {
			conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(transport))
			require.NoError(t, err)
			defer func() { require.NoError(t, conn.Close()) }()
			ctx, cancel := context.WithTimeout(f.SignedContext(t, request, authorityv1.AuthoritySourceService_ReadSnapshot_FullMethodName, name), 300*time.Millisecond)
			defer cancel()
			_, err = authorityv1.NewAuthoritySourceServiceClient(conn).ReadSnapshot(ctx, request)
			require.Contains(t, []codes.Code{codes.Unavailable, codes.DeadlineExceeded}, status.Code(err))
			require.Zero(t, reader.calls.Load())
		})
	}
}

func TestSourceRuntimeLazyTrustRecoveryAndReplayDependency(t *testing.T) {
	f := integrationtest.NewSourceTLSFixture(t, authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE)
	f.JWKS.Set(http.StatusServiceUnavailable, nil)
	reader := &runtimeReader{}
	client, _ := startSourceRuntime(t, f, reader)
	request := &authorityv1.ReadSnapshotRequest{Scope: &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: uuid.NewString()}}
	ctx := f.SignedContext(t, request, authorityv1.AuthoritySourceService_ReadSnapshot_FullMethodName, "source-recovery")
	_, err := client.ReadSnapshot(ctx, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Zero(t, reader.calls.Load())
	require.Empty(t, f.Redis.Keys())
	f.JWKS.Set(http.StatusOK, integrationtest.SourceJWKSDocument(t, f.Current, f.Next))
	require.Eventually(t, func() bool { _, err = client.ReadSnapshot(ctx, request); return err == nil }, time.Second, 20*time.Millisecond)
	require.Equal(t, int64(1), reader.calls.Load())
	_, err = client.ReadSnapshot(ctx, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, int64(1), reader.calls.Load())
	f.Redis.Close()
	_, err = client.ReadSnapshot(f.SignedContext(t, request, authorityv1.AuthoritySourceService_ReadSnapshot_FullMethodName, "source-redis-down"), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, int64(1), reader.calls.Load(), "replay outage must stop before owning state read")
}
