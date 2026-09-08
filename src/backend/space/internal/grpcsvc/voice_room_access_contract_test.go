package grpcsvc

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"
)

type failingVoiceRoomAccessResolver struct{ err error }

func (r failingVoiceRoomAccessResolver) ResolveVoiceRoomAccess(context.Context, uuid.UUID, uuid.UUID) (*store.VoiceRoomAccessRow, error) {
	return nil, r.err
}

type verifiedVoiceCredentials struct{}

func (verifiedVoiceCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer verified-test-voice-service"}, nil
}
func (verifiedVoiceCredentials) RequireTransportSecurity() bool { return false }

var _ credentials.PerRPCCredentials = verifiedVoiceCredentials{}

func startVoiceResolverGRPCTestServer(t *testing.T, pool *pgxpool.Pool) (spacev1.SpaceServiceClient, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer(grpc.UnaryInterceptor(authctx.VerifiedServiceIdentityUnaryInterceptor("verified-test-voice-service")))
	spacev1.RegisterSpaceServiceServer(srv, &SpaceGRPC{Store: &store.SpaceStore{Pool: pool}})
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///resolver-bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	return spacev1.NewSpaceServiceClient(conn), func() { _ = conn.Close(); srv.Stop() }
}

func resolveAccess(t *testing.T, c spacev1.SpaceServiceClient, roomID, profileID string) *spacev1.ResolveVoiceRoomAccessResponse {
	t.Helper()
	r, err := c.ResolveVoiceRoomAccess(context.Background(), &spacev1.ResolveVoiceRoomAccessRequest{VoiceRoomId: roomID, ProfileId: profileID}, grpc.PerRPCCredentials(verifiedVoiceCredentials{}))
	require.NoError(t, err)
	return r
}

func TestResolveVoiceRoomAccess_TrustedVoiceGetsCanonicalRoomAndExactMembership(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	c, cleanup := startVoiceResolverGRPCTestServer(t, pool)
	t.Cleanup(cleanup)
	created, err := c.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Exact membership"})
	require.NoError(t, err)
	room, err := c.CreateVoiceRoom(ownerCtx, &spacev1.CreateVoiceRoomRequest{SpaceId: created.GetSpace().GetId(), Name: "Late member"})
	require.NoError(t, err)
	late := uuid.New()
	for i := 0; i < 499; i++ {
		_, err = pool.Exec(context.Background(), "INSERT INTO space_members (space_id, profile_id) VALUES ($1, $2)", created.GetSpace().GetId(), uuid.New())
		require.NoError(t, err)
	}
	_, err = pool.Exec(context.Background(), "INSERT INTO space_members (space_id, profile_id) VALUES ($1, $2)", created.GetSpace().GetId(), late)
	require.NoError(t, err)
	r := resolveAccess(t, c, room.GetVoiceRoom().GetId(), late.String())
	require.Equal(t, created.GetSpace().GetId(), r.GetSpaceId())
	require.True(t, r.GetMember())
	require.True(t, r.GetActive())
}

func TestResolveVoiceRoomAccess_FailClosedBoundaryAndInputErrors(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	c, cleanup := startVoiceResolverGRPCTestServer(t, pool)
	t.Cleanup(cleanup)
	created, err := c.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Protected"})
	require.NoError(t, err)
	room, err := c.CreateVoiceRoom(ownerCtx, &spacev1.CreateVoiceRoomRequest{SpaceId: created.GetSpace().GetId(), Name: "Room"})
	require.NoError(t, err)
	for _, req := range []*spacev1.ResolveVoiceRoomAccessRequest{{VoiceRoomId: "bad", ProfileId: uuid.NewString()}, {VoiceRoomId: room.GetVoiceRoom().GetId(), ProfileId: "bad"}} {
		_, err = c.ResolveVoiceRoomAccess(context.Background(), req, grpc.PerRPCCredentials(verifiedVoiceCredentials{}))
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	_, err = c.ResolveVoiceRoomAccess(metadata.AppendToOutgoingContext(context.Background(), "x-voice-service-id", "voice"), &spacev1.ResolveVoiceRoomAccessRequest{VoiceRoomId: room.GetVoiceRoom().GetId(), ProfileId: uuid.NewString()})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	r := resolveAccess(t, c, room.GetVoiceRoom().GetId(), uuid.NewString())
	require.Equal(t, created.GetSpace().GetId(), r.GetSpaceId())
	require.False(t, r.GetMember())
	require.True(t, r.GetActive())
	_, err = c.ResolveVoiceRoomAccess(context.Background(), &spacev1.ResolveVoiceRoomAccessRequest{VoiceRoomId: uuid.NewString(), ProfileId: uuid.NewString()}, grpc.PerRPCCredentials(verifiedVoiceCredentials{}))
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestResolveVoiceRoomAccess_ResolverFailureIsUnavailable(t *testing.T) {
	svc := &SpaceGRPC{voiceRoomAccessResolver: failingVoiceRoomAccessResolver{err: errors.New("space database unavailable")}}
	ctx := authctx.WithVerifiedServiceIdentity(context.Background(), authctx.ServiceIdentityVoice)
	resp, err := svc.ResolveVoiceRoomAccess(ctx, &spacev1.ResolveVoiceRoomAccessRequest{
		VoiceRoomId: uuid.NewString(),
		ProfileId:   uuid.NewString(),
	})
	require.Nil(t, resp)
	require.Equal(t, codes.Unavailable, status.Code(err))
}
