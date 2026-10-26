package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net"
	"path/filepath"
	"testing"
	"time"
	"voice/backend/gameintegration/internal/registry"
	"voice/backend/pkg/authoritysource"
	"voice/backend/pkg/integrationtest"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
)

func TestGISAuthoritySourceActualMigratorProtectedFactory(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join("..", "migrations", "game_integration_db"))
	_, err := db.Pool.Exec(ctx, `CREATE DATABASE game_integration_db`)
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, `CREATE ROLE gameintegration_runtime NOLOGIN`)
	require.NoError(t, err)
	db.Run(true, "up")
	f := integrationtest.NewSourceTLSFixture(t, authorityv1.AuthorityOwner_AUTHORITY_OWNER_GAME_INTEGRATION)
	reader := &registry.Store{Pool: db.Pool}
	_, err = db.Pool.Exec(ctx, `UPDATE schema_migrations SET dirty=true`)
	require.NoError(t, err)
	server, runtime, err := newGISAuthorityServer(ctx, f.Config, reader)
	require.Error(t, err)
	require.Nil(t, server)
	require.Nil(t, runtime)
	_, err = db.Pool.Exec(ctx, `UPDATE schema_migrations SET dirty=false`)
	require.NoError(t, err)
	server, runtime, err = newGISAuthorityServer(ctx, f.Config, reader)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); <-done })
	require.Len(t, server.GetServiceInfo(), 1, "source listener exposes only owning read service")
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(f.ClientCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	client := authorityv1.NewAuthoritySourceServiceClient(conn)
	request := &authorityv1.ReadSnapshotRequest{Scope: &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: uuid.NewString(), EnvironmentId: uuid.NewString(), ProfileIds: []string{uuid.NewString()}}}
	method := authorityv1.AuthoritySourceService_ReadSnapshot_FullMethodName
	call := f.SignedContext(t, request, method, "gis-source-first")
	first, err := client.ReadSnapshot(call, request)
	require.NoError(t, err)
	require.True(t, first.Complete)
	state, err := authoritysource.DecodeGISState(first.CanonicalState)
	require.NoError(t, err)
	require.False(t, state.EnvironmentExists)
	_, err = client.ReadSnapshot(call, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "replayed principal must fail")
	_, err = client.ReadSnapshot(ctx, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	revisionRequest := &authorityv1.ReadRevisionRequest{Scope: request.Scope}
	revision, err := client.ReadRevision(f.SignedContext(t, revisionRequest, authorityv1.AuthoritySourceService_ReadRevision_FullMethodName, "gis-revision"), revisionRequest)
	require.NoError(t, err)
	require.Equal(t, first.Revision, revision.Revision)
	_, err = db.Pool.Exec(ctx, `ALTER TABLE player_bindings DISABLE TRIGGER gis_authority_changed`)
	require.NoError(t, err)
	_, err = client.ReadSnapshot(f.SignedContext(t, request, method, "gis-source-disabled-trigger"), request)
	require.Equal(t, codes.Unavailable, status.Code(err))
	_, err = db.Pool.Exec(ctx, `ALTER TABLE player_bindings ENABLE TRIGGER gis_authority_changed`)
	require.NoError(t, err)
	db.Run(false, "down", "1")
	_, err = client.ReadSnapshot(f.SignedContext(t, request, method, "gis-source-maintenance"), request)
	require.Equal(t, codes.Unavailable, status.Code(err))
}
