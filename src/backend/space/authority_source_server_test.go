package main

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"voice/backend/pkg/authoritysource"
	"voice/backend/pkg/integrationtest"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
	"voice/backend/space/internal/store"
)

func TestSpaceAuthoritySourceActualMigratorProtectedFactory(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL and real protected source listener")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join("..", "migrations", "space_db"))
	database.Run(true, "up")
	reader := &store.SpaceStore{Pool: database.Pool}
	owner, member := uuid.New(), uuid.New()
	space, err := reader.CreateSpace(ctx, owner, "actual source", "", "private")
	require.NoError(t, err)
	_, err = database.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, member)
	require.NoError(t, err)
	f := integrationtest.NewSourceTLSFixture(t, authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE)
	_, err = database.Pool.Exec(ctx, `UPDATE schema_migrations SET dirty=true`)
	require.NoError(t, err)
	server, runtime, err := newSpaceAuthorityServer(ctx, nil, f.Config, reader)
	require.Error(t, err)
	require.Nil(t, server)
	require.Nil(t, runtime)
	_, err = database.Pool.Exec(ctx, `UPDATE schema_migrations SET dirty=false`)
	require.NoError(t, err)
	server, runtime, err = newSpaceAuthorityServer(ctx, nil, f.Config, reader)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); <-done })
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(f.ClientCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	client := authorityv1.NewAuthoritySourceServiceClient(conn)
	request := &authorityv1.ReadSnapshotRequest{Scope: &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: space.ID.String()}}
	method := authorityv1.AuthoritySourceService_ReadSnapshot_FullMethodName
	callCtx := f.SignedContext(t, request, method, "space-source-first")
	snapshot, err := client.ReadSnapshot(callCtx, request)
	require.NoError(t, err)
	require.True(t, snapshot.Complete)
	state, err := authoritysource.DecodeSpaceState(snapshot.CanonicalState)
	require.NoError(t, err)
	require.True(t, state.MemberAt(member.String(), time.Now().UnixMilli()))
	_, err = client.ReadSnapshot(callCtx, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = database.Pool.Exec(ctx, `DELETE FROM space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, member)
	require.NoError(t, err)
	removed, err := client.ReadSnapshot(f.SignedContext(t, request, method, "space-source-removal"), request)
	require.NoError(t, err)
	require.Greater(t, removed.Revision, snapshot.Revision)
	state, err = authoritysource.DecodeSpaceState(removed.CanonicalState)
	require.NoError(t, err)
	require.False(t, state.MemberAt(member.String(), time.Now().UnixMilli()))
	database.Run(false, "down", "1")
	_, err = client.ReadSnapshot(f.SignedContext(t, request, method, "space-source-refused-down"), request)
	require.Equal(t, codes.Unavailable, status.Code(err), "driver dirty maintenance must stop an already serving source")
	var retained int
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, owner).Scan(&retained))
	require.Equal(t, 1, retained, "refused rollback must preserve ordinary owner state")
}
