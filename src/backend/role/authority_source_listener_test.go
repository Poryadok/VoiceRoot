package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"voice/backend/pkg/authoritysource"
	"voice/backend/pkg/integrationtest"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/store"
)

func sourceListenerClient(t *testing.T, server *grpc.Server, transport credentials.TransportCredentials) (authorityv1.AuthoritySourceServiceClient, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); <-done })
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(transport))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	return authorityv1.NewAuthoritySourceServiceClient(conn), listener.Addr().String()
}

func sourceCallContext(t *testing.T, key *rsa.PrivateKey, issuer, method, id string, request proto.Message) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: issuer, KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	token, err := signer.IssueService(principal.ServiceInput{Audience: "role", RPC: method, RequestID: id, RequestHash: hash})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
}

// The actual application server factory consumes a real owning store upgraded
// by the pinned migration driver, through mTLS, HTTPS JWKS and Redis replay.
func TestRoleAuthoritySourceActualMigratorProtectedListener(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL and pinned migration driver")
	}
	integrationtest.ConfigureDockerTesting()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pg, err := postgres.Run(ctx, integrationtest.PostgresImage, postgres.BasicWaitStrategies(), postgres.WithDatabase("source_listener"), postgres.WithUsername("source_fixture"), postgres.WithPassword("fixture_only"))
	if pg != nil {
		t.Cleanup(func() {
			cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
			require.True(t, pg.Terminate(cleanup) == nil, "terminate owned PostgreSQL")
		})
	}
	require.True(t, err == nil, "start isolated owning database")
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.True(t, err == nil, "resolve private connection")
	pool, err := pgxpool.New(ctx, strings.Replace(dsn, "localhost", "127.0.0.1", 1))
	require.True(t, err == nil, "connect private owning database")
	t.Cleanup(pool.Close)
	directory := filepath.Join("..", "migrations", "role_db")
	files, err := os.ReadDir(directory)
	require.NoError(t, err)
	copied := []testcontainers.ContainerFile{}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".sql") {
			copied = append(copied, testcontainers.ContainerFile{HostFilePath: filepath.Join(directory, file.Name()), ContainerFilePath: "/migrations/" + file.Name(), FileMode: 0644})
		}
	}
	privateURL := &url.URL{Scheme: "postgres", User: url.UserPassword("source_fixture", "fixture_only"), Host: "127.0.0.1:5432", Path: "/source_listener", RawQuery: "sslmode=disable"}
	process, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{
		Image: "migrate/migrate:v4.18.1", NetworkMode: container.NetworkMode("container:" + pg.GetContainerID()), Files: copied,
		Cmd: []string{"-path", "/migrations", "-database", privateURL.String(), "up"}, WaitingFor: wait.ForExit(),
	}, Started: true})
	if process != nil {
		t.Cleanup(func() {
			cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
			require.True(t, process.Terminate(cleanup) == nil, "terminate owned migration process")
		})
	}
	require.True(t, err == nil, "run pinned owning migration driver")
	result, err := process.State(ctx)
	require.True(t, err == nil, "read pinned driver result")
	require.Equal(t, 0, result.ExitCode)
	reader := &store.RoleStore{Pool: pool}
	space, owner := uuid.New(), uuid.New()
	require.NoError(t, reader.BootstrapSpaceRoles(ctx, space, owner))
	runtime, roots, key, certificate := newMainPrincipalRuntime(t, true)
	_, err = pool.Exec(ctx, `UPDATE schema_migrations SET dirty=true`)
	require.NoError(t, err)
	require.Error(t, runtime.ActivateAuthoritySource(ctx, reader), "dirty catalog must prevent registration")
	_, err = pool.Exec(ctx, `UPDATE schema_migrations SET dirty=false`)
	require.NoError(t, err)
	require.NoError(t, runtime.ActivateAuthoritySource(ctx, reader))
	require.Error(t, runtime.ActivateAuthoritySource(ctx, reader), "serving set must activate once")
	legacy, protected := newRoleGRPCServers(nil, &principalBootstrapRoleServer{}, runtime)
	legacyClient, _ := sourceListenerClient(t, legacy, insecure.NewCredentials())
	client, address := sourceListenerClient(t, protected, credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{certificate}}))
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: space.String()}
	request := &authorityv1.ReadSnapshotRequest{Scope: scope}
	method := authorityv1.AuthoritySourceService_ReadSnapshot_FullMethodName
	for name, transport := range map[string]credentials.TransportCredentials{
		"plaintext":                  insecure.NewCredentials(),
		"missing_client_certificate": credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}),
		"wrong_server_identity":      credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "wrong.role.internal", Certificates: []tls.Certificate{certificate}}),
	} {
		t.Run(name, func(t *testing.T) {
			conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(transport))
			require.NoError(t, err)
			defer func() { require.NoError(t, conn.Close()) }()
			callCtx, stop := context.WithTimeout(sourceCallContext(t, key, "federation", method, "source-tls-"+name, request), 300*time.Millisecond)
			defer stop()
			_, err = authorityv1.NewAuthoritySourceServiceClient(conn).ReadSnapshot(callCtx, request)
			require.Contains(t, []codes.Code{codes.Unavailable, codes.DeadlineExceeded}, status.Code(err), "invalid TLS must stop before source handler")
		})
	}
	callCtx := sourceCallContext(t, key, "federation", method, "source-complete", request)
	_, err = legacyClient.ReadSnapshot(callCtx, request)
	require.Equal(t, codes.Unimplemented, status.Code(err), "legacy listener must not register source RPCs")
	response, err := client.ReadSnapshot(callCtx, request)
	require.NoError(t, err)
	require.True(t, response.Complete)
	require.True(t, proto.Equal(scope, response.Scope))
	require.Equal(t, authorityv1.AuthorityOwner_AUTHORITY_OWNER_ROLE, response.Owner)
	state, err := authoritysource.DecodeRoleState(response.CanonicalState)
	require.NoError(t, err)
	require.Equal(t, state.AllPermissions, state.EffectiveMask(owner.String(), "", true))
	_, err = client.ReadSnapshot(callCtx, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "replayed source credential must fail")
	_, err = client.ReadSnapshot(sourceCallContext(t, key, "space", method, "source-wrong-issuer", request), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "ownership issuer cannot read Federation snapshots")
	tampered := proto.Clone(request).(*authorityv1.ReadSnapshotRequest)
	tampered.Scope.SpaceId = uuid.NewString()
	_, err = client.ReadSnapshot(sourceCallContext(t, key, "federation", method, "source-tamper", request), tampered)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = pool.Exec(ctx, `DELETE FROM member_roles WHERE space_id=$1 AND profile_id=$2`, space, owner)
	require.NoError(t, err)
	revReq := &authorityv1.ReadRevisionRequest{Scope: scope}
	revision, err := client.ReadRevision(sourceCallContext(t, key, "federation", authorityv1.AuthoritySourceService_ReadRevision_FullMethodName, "source-after-removal", revReq), revReq)
	require.NoError(t, err)
	require.Greater(t, revision.Revision, response.Revision)
	after, err := client.ReadSnapshot(sourceCallContext(t, key, "federation", method, "source-after-removal-state", request), request)
	require.NoError(t, err)
	afterState, err := authoritysource.DecodeRoleState(after.CanonicalState)
	require.NoError(t, err)
	require.Zero(t, afterState.EffectiveMask(owner.String(), "", true))
	_, err = pool.Exec(ctx, `UPDATE schema_migrations SET dirty=true`)
	require.NoError(t, err)
	_, err = client.ReadSnapshot(sourceCallContext(t, key, "federation", method, "source-maintenance", request), request)
	require.Equal(t, codes.Unavailable, status.Code(err), "maintenance after registration must stop complete reads")
	// A fully configured source still requires its opt-in and successful preflight.
	disabled, disabledRoots, _, disabledCert := newMainPrincipalRuntime(t)
	require.Error(t, disabled.ActivateAuthoritySource(ctx, reader))
	unusedLegacy, disabledServer := newRoleGRPCServers(nil, &principalBootstrapRoleServer{}, disabled)
	unusedLegacy.Stop()
	disabledClient, _ := sourceListenerClient(t, disabledServer, credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: disabledRoots, Certificates: []tls.Certificate{disabledCert}}))
	_, err = disabledClient.ReadSnapshot(context.Background(), request)
	require.Equal(t, codes.Unimplemented, status.Code(err))
}
