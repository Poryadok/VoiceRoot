package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/role/internal/grpcsvc"
	"voice/backend/role/internal/store"
)

func TestBotRemovalActualRole25ProtectedReceipt(t *testing.T) {
	if testing.Short() {
		t.Skip("requires private PostgreSQL and official migration driver")
	}
	ctx := context.Background()
	f := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join("..", "migrations", "role_db"))
	f.Run(true, "goto", "21")
	f.Run(true, "goto", "25")
	roles := &store.RoleStore{Pool: f.Pool}
	runtime, roots, key, cert := newMainPrincipalRuntime(t, false, true)
	legacy, protected := newRoleGRPCServers(nil, &grpcsvc.RoleGRPC{Store: roles}, runtime)
	legacy.Stop()
	_, address := sourceListenerClient(t, protected, credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{cert}}))
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{cert}})))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	client := rolev1.NewRoleServiceClient(conn)
	req := &rolev1.ApplyBotInstallationRemovalRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), DecisionId: uuid.NewString(), DecisionSha256: bytes.Repeat([]byte{1}, 32), Purpose: "space_uninstall", BotId: uuid.NewString(), ActorProfileId: uuid.NewString(), SpaceId: uuid.NewString(), FrozenScopeSha256: bytes.Repeat([]byte{2}, 32), Evidence: &rolev1.ApplyBotInstallationRemovalRequest_Native{Native: &rolev1.BotRemovalNativeEvidence{InstallationId: uuid.NewString(), InstallOperationId: uuid.NewString(), Generation: 1}}}
	role := uuid.New()
	_, err = f.Pool.Exec(ctx, `INSERT INTO roles(id,space_id,name,is_system,created_by_profile_id) VALUES($1,$2,'Bot custom',false,$3)`, role, req.SpaceId, req.ActorProfileId)
	require.NoError(t, err)
	method := rolev1.RoleService_ApplyBotInstallationRemoval_FullMethodName
	firstCtx := sourceCallContext(t, key, "bot", method, req.OperationId, req)
	response, err := client.ApplyBotInstallationRemoval(firstCtx, req)
	require.NoError(t, err)
	require.EqualValues(t, 1, response.GetReceipt().GetDeletedRoleCount())
	_, err = client.ApplyBotInstallationRemoval(firstCtx, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "transport replay stops before persistence")
	replayed, err := client.ApplyBotInstallationRemoval(sourceCallContext(t, key, "bot", method, req.OperationId, req), req)
	require.NoError(t, err)
	require.True(t, proto.Equal(response, replayed), "fresh signed retry returns persisted historical receipt")
	changed := proto.Clone(req).(*rolev1.ApplyBotInstallationRemovalRequest)
	changed.FrozenScopeSha256[0] ^= 1
	_, err = client.ApplyBotInstallationRemoval(sourceCallContext(t, key, "bot", method, changed.OperationId, changed), changed)
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	_, err = client.ApplyBotInstallationRemoval(sourceCallContext(t, key, "space", method, req.OperationId, req), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = f.Pool.Exec(ctx, `ALTER TABLE role_bot_global_removals DISABLE TRIGGER role_bot_global_removals_guard`)
	require.NoError(t, err)
	next := proto.Clone(req).(*rolev1.ApplyBotInstallationRemovalRequest)
	next.OperationId = uuid.NewString()
	_, err = client.ApplyBotInstallationRemoval(sourceCallContext(t, key, "bot", method, next.OperationId, next), next)
	require.Equal(t, codes.Unavailable, status.Code(err))
	var count int
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT count(*) FROM role_bot_removal_operations`).Scan(&count))
	require.Equal(t, 1, count, "catalog failure has no new receipt/effects")
}
