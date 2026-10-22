package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/grpcsvc"
	"voice/backend/role/internal/store"
	"voice/backend/role/permissions"
)

// Inject only a database query cancellation, never alternate authorization.
type bootstrapPGCancelInsert struct{ calls atomic.Int64 }

func (f *bootstrapPGCancelInsert) TraceQueryStart(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
	if strings.Contains(q.SQL, "INSERT INTO member_roles") {
		f.calls.Add(1)
		c, cancel := context.WithCancel(ctx)
		cancel()
		return c
	}
	return ctx
}
func (*bootstrapPGCancelInsert) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestBootstrapActualRole20PG(t *testing.T) {
	if testing.Short() {
		t.Skip("requires explicitly owned private PG20 and real Redis")
	}
	dsn, replay := os.Getenv("VOICE_ROLE_BOOTSTRAP_PG_DSN"), os.Getenv("VOICE_ROLE_BOOTSTRAP_REDIS_ADDR")
	if dsn == "" || replay == "" {
		t.Skip("owned private fixture inputs not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.True(t, err == nil, "open owned PG without exposing credentials")
	t.Cleanup(pool.Close)
	var version int
	var dirty bool
	require.NoError(t, pool.QueryRow(ctx, "SELECT version,dirty FROM schema_migrations").Scan(&version, &dirty))
	require.Equal(t, 20, version)
	require.False(t, dirty)
	roles := &store.RoleStore{Pool: pool}
	require.NoError(t, roles.CheckAuthoritySourceSchema(ctx), "actual Role20 catalog")
	runtime, roots, key, cert := newBootstrapPGTransport(t, replay)
	start := func(db *pgxpool.Pool) rolev1.RoleServiceClient {
		legacy, server := newRoleGRPCServers(nil, &grpcsvc.RoleGRPC{Store: &store.RoleStore{Pool: db}}, runtime)
		legacy.Stop()
		lis, e := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, e)
		done := make(chan error, 1)
		go func() { done <- server.Serve(lis) }()
		t.Cleanup(func() { server.Stop(); <-done })
		conn, e := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{cert}})))
		require.NoError(t, e)
		t.Cleanup(func() { _ = conn.Close() })
		return rolev1.NewRoleServiceClient(conn)
	}
	client := start(pool)
	signed := func(req *rolev1.BootstrapSpaceRolesRequest) metadata.MD {
		hash, e := principal.RequestHash(req)
		require.NoError(t, e)
		id := uuid.NewString() // Fresh transport UUID, never native publication identity.
		return metadata.Pairs("authorization", "Bearer "+mainPrincipalToken(t, key, rolev1.RoleService_BootstrapSpaceRoles_FullMethodName, id, hash), "x-request-id", id)
	}
	call := func(c rolev1.RoleServiceClient, req *rolev1.BootstrapSpaceRolesRequest, md metadata.MD) error {
		cctx, stop := context.WithTimeout(metadata.NewOutgoingContext(ctx, md.Copy()), 3*time.Second)
		defer stop()
		_, e := c.BootstrapSpaceRoles(cctx, req)
		require.NoError(t, cctx.Err(), "RPC context must remain live")
		return e
	}
	space, owner := uuid.New(), uuid.New()
	req := &rolev1.BootstrapSpaceRolesRequest{SpaceId: space.String(), OwnerProfileId: owner.String()}
	count := func(id uuid.UUID) int {
		var n int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM roles WHERE space_id=$1", id).Scan(&n))
		return n
	}
	require.Zero(t, count(space), "no seeded Owner authority before actual RPC")
	md := signed(req)
	require.NoError(t, call(client, req, md))
	specs, e := permissions.SystemRoles()
	require.NoError(t, e)
	rows, e := roles.ListRoles(ctx, space)
	require.NoError(t, e)
	require.Len(t, rows, 5)
	ids := map[string]uuid.UUID{}
	for _, r := range rows {
		ids[r.Name] = r.ID
		require.True(t, r.Managed)
		var matched bool
		for _, spec := range specs {
			if spec.Name == r.Name {
				matched = true
				require.Equal(t, spec.Mask, r.PermissionsMask)
				require.EqualValues(t, spec.Position, r.Position)
			}
		}
		require.True(t, matched)
	}
	checkOwner := func() {
		var profile uuid.UUID
		var n int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM member_roles WHERE space_id=$1 AND role_id=$2", space, ids[permissions.RoleOwner]).Scan(&n))
		require.Equal(t, 1, n)
		require.NoError(t, pool.QueryRow(ctx, "SELECT profile_id FROM member_roles WHERE space_id=$1 AND role_id=$2", space, ids[permissions.RoleOwner]).Scan(&profile))
		require.Equal(t, owner, profile)
	}
	checkOwner()
	def, e := roles.GetDefaultJoinRole(ctx, space)
	require.NoError(t, e)
	require.Equal(t, ids[permissions.RoleMember], def.ID)
	require.Equal(t, codes.Unauthenticated, status.Code(call(client, req, md)), "spent JTI")
	require.NoError(t, call(client, req, signed(req)), "fresh dispatch exact idempotence")
	restarted := start(pool)
	require.NoError(t, call(restarted, req, signed(req)), "actual service re-instantiation same durable store")
	rows2, e := roles.ListRoles(ctx, space)
	require.NoError(t, e)
	require.Equal(t, rows, rows2)
	checkOwner()
	other := proto.Clone(req).(*rolev1.BootstrapSpaceRolesRequest)
	other.OwnerProfileId = uuid.NewString()
	require.Error(t, call(client, other, signed(other)), "bootstrap cannot replace canonical Owner")
	require.Equal(t, 5, count(space))
	checkOwner()
	bad := &rolev1.BootstrapSpaceRolesRequest{SpaceId: uuid.NewString(), OwnerProfileId: uuid.Nil.String()}
	require.Equal(t, codes.InvalidArgument, status.Code(call(client, bad, signed(bad))))
	require.Zero(t, count(uuid.MustParse(bad.SpaceId)))

	intent := &rolev1.OwnershipTransferIntent{ProtocolVersion: 2, SpaceId: space.String(), OldOwnerProfileId: owner.String(), NewOwnerProfileId: uuid.NewString(), OperationId: uuid.NewString()}
	wire, e := proto.MarshalOptions{Deterministic: true}.Marshal(intent)
	require.NoError(t, e)
	digest := sha256.Sum256(wire)
	in := store.OwnershipTransferV2Input{ProtocolVersion: 2, SpaceID: space, OldOwnerProfileID: owner, NewOwnerProfileID: uuid.MustParse(intent.NewOwnerProfileId), OperationID: uuid.MustParse(intent.OperationId), IntentBytes: wire, RequestHash: digest[:]}
	_, e = roles.PrepareOwnershipTransfer(ctx, in)
	require.NoError(t, e)
	require.Equal(t, codes.Unavailable, status.Code(call(client, req, signed(req))), "prepared fence before exact bootstrap retry")
	_, e = roles.AbortOwnershipTransfer(ctx, in)
	require.NoError(t, e)
	require.NoError(t, call(client, req, signed(req)))
	checkOwner()

	tracer := &bootstrapPGCancelInsert{}
	cfg := pool.Config().Copy()
	cfg.ConnConfig.Tracer = tracer
	failing, e := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, e)
	t.Cleanup(failing.Close)
	broken := start(failing)
	retry := &rolev1.BootstrapSpaceRolesRequest{SpaceId: uuid.NewString(), OwnerProfileId: uuid.NewString()}
	require.Error(t, call(broken, retry, signed(retry)), "actual Owner INSERT cancellation must roll back seed")
	require.EqualValues(t, 1, tracer.calls.Load())
	require.Zero(t, count(uuid.MustParse(retry.SpaceId)))
	require.NoError(t, call(client, retry, signed(retry)))
	require.Equal(t, 5, count(uuid.MustParse(retry.SpaceId)))
	t.Log("actual Role20 durable Owner/default/idempotence/replay/fence/rollback proven; no external identity/native21 contribution authority inferred")
}
