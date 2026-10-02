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
	"voice/backend/user/internal/store"
)

func TestUserAuthoritySourceActualMigratorProtectedFactory(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL and protected source listener")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join("..", "migrations", "user_db"))
	database.Run(true, "up")
	reader := store.NewProfileStore(database.Pool)
	account, profile := uuid.NewString(), uuid.NewString()
	_, err := database.Pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'factory','0001','private')`, profile, account)
	require.NoError(t, err)
	f := integrationtest.NewSourceTLSFixture(t, authorityv1.AuthorityOwner_AUTHORITY_OWNER_USER)
	_, err = database.Pool.Exec(ctx, `UPDATE schema_migrations SET dirty=true`)
	require.NoError(t, err)
	server, runtime, err := newUserAuthorityServer(ctx, nil, f.Config, reader)
	require.Error(t, err)
	require.Nil(t, server)
	require.Nil(t, runtime)
	_, err = database.Pool.Exec(ctx, `UPDATE schema_migrations SET dirty=false`)
	require.NoError(t, err)
	server, runtime, err = newUserAuthorityServer(ctx, nil, f.Config, reader)
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
	request := &authorityv1.ReadSnapshotRequest{Scope: &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: uuid.NewString(), ProfileIds: []string{profile}}}
	method := authorityv1.AuthoritySourceService_ReadSnapshot_FullMethodName
	call := f.SignedContext(t, request, method, "user-source-first")
	first, err := client.ReadSnapshot(call, request)
	require.NoError(t, err)
	require.True(t, first.Complete)
	state, err := authoritysource.DecodeUserState(first.CanonicalState)
	require.NoError(t, err)
	require.True(t, state.EligibleProfile(profile, account))
	_, err = client.ReadSnapshot(call, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = database.Pool.Exec(ctx, `INSERT INTO user_account_lifecycle(account_id,state,source_event_id,occurred_at) VALUES($1,'ACCOUNT_INACTIVE',$2,now())`, account, uuid.NewString())
	require.NoError(t, err)
	revoked, err := client.ReadSnapshot(f.SignedContext(t, request, method, "user-source-inactive"), request)
	require.NoError(t, err)
	require.Greater(t, revoked.Revision, first.Revision)
	state, err = authoritysource.DecodeUserState(revoked.CanonicalState)
	require.NoError(t, err)
	require.False(t, state.EligibleProfile(profile, account))
	database.Run(false, "down", "1")
	_, err = client.ReadSnapshot(f.SignedContext(t, request, method, "user-source-maintenance"), request)
	require.Equal(t, codes.Unavailable, status.Code(err))
	var retained int
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT count(*) FROM user_account_lifecycle WHERE account_id=$1`, account).Scan(&retained))
	require.Equal(t, 1, retained)
}
