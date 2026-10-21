package main

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"net"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/grpcsvc"
	"voice/backend/role/internal/store"
	"voice/backend/role/permissions"
)

type chatACLCommitSQL struct{}
type chatACLCommitExpiry struct {
	enabled, fresh atomic.Bool
	calls          atomic.Int64
}

func (d *chatACLCommitExpiry) TraceQueryStart(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, chatACLCommitSQL{}, strings.TrimSpace(q.SQL))
}
func (d *chatACLCommitExpiry) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if sql, _ := ctx.Value(chatACLCommitSQL{}).(string); sql != "commit" {
		return
	}
	p, ok := principal.FromContext(ctx)
	if !ok || p.Issuer != "chat" || !d.enabled.Swap(false) {
		return
	}
	d.calls.Add(1)
	d.fresh.Store(time.Now().Before(p.ExpiresAt))
	for time.Now().Before(p.ExpiresAt) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Millisecond):
		}
	}
}

func chatACLContext(t *testing.T, key *rsa.PrivateKey, req *rolev1.CheckPermissionRequest, clock func() time.Time) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	i, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "chat", KeyID: "current", PrivateKey: key, Clock: clock})
	require.NoError(t, err)
	id := uuid.NewString()
	token, err := i.IssueService(principal.ServiceInput{Audience: "role", RPC: rolev1.RoleService_CheckPermission_FullMethodName, RequestID: id, RequestHash: hash})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestChatCurrentACLActualPGFenceAndExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("requires owned Role PostgreSQL and Redis")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	f := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join("..", "migrations", "role_db"))
	f.Run(true, "goto", "20")
	rdb, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "redis:7-alpine", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForListeningPort("6379/tcp")}, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() {
		c, stop := context.WithTimeout(context.Background(), 20*time.Second)
		defer stop()
		require.NoError(t, rdb.Terminate(c))
	})
	host, err := rdb.Host(ctx)
	require.NoError(t, err)
	port, err := rdb.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)
	runtime, roots, key, certificate := newMainPrincipalRuntimeWithReplay(t, net.JoinHostPort(host, port.Port()), false, false, true)
	roles := &store.RoleStore{Pool: f.Pool}
	space, owner, profile, chat := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, roles.BootstrapSpaceRoles(ctx, space, owner))
	member, err := roles.RoleIDByName(ctx, space, permissions.RoleMember)
	require.NoError(t, err)
	require.NoError(t, roles.AssignMemberRole(ctx, space, profile, member, owner))
	legacy, server := newRoleGRPCServers(nil, &grpcsvc.RoleGRPC{Store: roles}, runtime)
	legacy.Stop()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{certificate}})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := rolev1.NewRoleServiceClient(conn)
	req := &rolev1.CheckPermissionRequest{SpaceId: space.String(), ProfileId: profile.String(), PermissionName: permissions.TextChatView, Chat: &chatv1.ChatRef{Id: chat.String(), Type: chatv1.ChatType_CHAT_TYPE_GROUP.Enum()}}
	decision := func(want bool) {
		r, err := client.CheckPermission(chatACLContext(t, key, req, nil), req)
		require.NoError(t, err)
		require.Equal(t, want, r.Allowed)
	}
	decision(true)
	view, err := permissions.MaskFor(permissions.TextChatView)
	require.NoError(t, err)
	require.NoError(t, roles.SetChatOverride(ctx, chat, member, 0, view))
	decision(false)
	require.NoError(t, roles.SetChatOverride(ctx, chat, member, 0, 0))
	decision(true)
	require.NoError(t, roles.RevokeMemberRole(ctx, space, profile, member))
	decision(false)
	require.NoError(t, roles.AssignMemberRole(ctx, space, profile, member, owner))
	decision(true)
	other := uuid.New()
	require.NoError(t, roles.BootstrapSpaceRoles(ctx, other, uuid.New()))
	foreign := proto.Clone(req).(*rolev1.CheckPermissionRequest)
	foreign.SpaceId = other.String()
	r, err := client.CheckPermission(chatACLContext(t, key, foreign, nil), foreign)
	require.NoError(t, err)
	require.False(t, r.Allowed, "subject has no roles in another Space")
	intent := &rolev1.OwnershipTransferIntent{ProtocolVersion: 2, SpaceId: space.String(), OldOwnerProfileId: owner.String(), NewOwnerProfileId: uuid.NewString(), OperationId: uuid.NewString()}
	wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(intent)
	require.NoError(t, err)
	digest := sha256.Sum256(wire)
	in := store.OwnershipTransferV2Input{ProtocolVersion: 2, SpaceID: space, OldOwnerProfileID: owner, NewOwnerProfileID: uuid.MustParse(intent.NewOwnerProfileId), OperationID: uuid.MustParse(intent.OperationId), IntentBytes: wire, RequestHash: digest[:]}
	_, err = roles.PrepareOwnershipTransfer(ctx, in)
	require.NoError(t, err)
	_, err = client.CheckPermission(chatACLContext(t, key, req, nil), req)
	require.Equal(t, codes.Unavailable, status.Code(err), "prepared lifecycle denies even an otherwise positive ACL")
	_, err = roles.AbortOwnershipTransfer(ctx, in)
	require.NoError(t, err)
	decision(true)

	tracer := &chatACLCommitExpiry{}
	tracer.enabled.Store(true)
	cfg := f.Pool.Config().Copy()
	cfg.ConnConfig.Tracer = tracer
	traced, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(traced.Close)
	unused, delayed := newRoleGRPCServers(nil, &grpcsvc.RoleGRPC{Store: &store.RoleStore{Pool: traced}}, runtime)
	unused.Stop()
	dl, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = delayed.Serve(dl) }()
	t.Cleanup(delayed.Stop)
	dc, err := grpc.NewClient(dl.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{certificate}})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = dc.Close() })
	expiring := chatACLContext(t, key, req, func() time.Time { return time.Now().Add(-28 * time.Second) })
	_, err = rolev1.NewRoleServiceClient(dc).CheckPermission(expiring, req)
	require.NoError(t, expiring.Err(), "request deadline must remain live, not explain denial")
	require.Equal(t, codes.Unavailable, status.Code(err), "expiry after owning read COMMIT cannot return allow")
	require.True(t, tracer.fresh.Load())
	require.EqualValues(t, 1, tracer.calls.Load())
	decision(true)
}
