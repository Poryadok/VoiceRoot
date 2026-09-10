package grpcsvc

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
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

func (r failingVoiceRoomAccessResolver) ResolveVoiceRoomAccess(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*store.VoiceRoomAccessRow, error) {
	return nil, r.err
}

type recordingVoiceRoomAccessResolver struct {
	spaceID     uuid.UUID
	voiceRoomID uuid.UUID
	profileID   uuid.UUID
	called      bool
}

func (r *recordingVoiceRoomAccessResolver) ResolveVoiceRoomAccess(_ context.Context, spaceID, voiceRoomID, profileID uuid.UUID) (*store.VoiceRoomAccessRow, error) {
	r.spaceID = spaceID
	r.voiceRoomID = voiceRoomID
	r.profileID = profileID
	r.called = true
	return &store.VoiceRoomAccessRow{
		SpaceID:      spaceID,
		Member:       true,
		Active:       true,
		Discoverable: true,
		AccessEpoch:  1,
	}, nil
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

func applyVoiceAccessEpochMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, name := range []string{
		"000008_ownership_journal.up.sql",
		"000009_ownership_journal_decision.up.sql",
		"000010_ownership_journal_commit.up.sql",
		"000011_voice_access_epoch.up.sql",
	} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
}

func resolveAccess(t *testing.T, c spacev1.SpaceServiceClient, spaceID, roomID, profileID string) *spacev1.ResolveVoiceRoomAccessResponse {
	t.Helper()
	r, err := c.ResolveVoiceRoomAccess(context.Background(), &spacev1.ResolveVoiceRoomAccessRequest{VoiceRoomId: roomID, ProfileId: profileID, Space: &spacev1.SpaceRef{Id: spaceID}}, grpc.PerRPCCredentials(verifiedVoiceCredentials{}))
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
	applyVoiceAccessEpochMigration(t, context.Background(), pool)
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
	r := resolveAccess(t, c, created.GetSpace().GetId(), room.GetVoiceRoom().GetId(), late.String())
	require.Equal(t, created.GetSpace().GetId(), r.GetSpaceId())
	require.True(t, r.GetMember())
	require.True(t, r.GetActive())
}

func TestResolveVoiceRoomAccess_FailClosedBoundaryAndInputErrors(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ownerProfile, _, ownerCtx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	applyVoiceAccessEpochMigration(t, context.Background(), pool)
	c, cleanup := startVoiceResolverGRPCTestServer(t, pool)
	t.Cleanup(cleanup)
	created, err := c.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Protected"})
	require.NoError(t, err)
	room, err := c.CreateVoiceRoom(ownerCtx, &spacev1.CreateVoiceRoomRequest{SpaceId: created.GetSpace().GetId(), Name: "Room"})
	require.NoError(t, err)
	foreign, err := c.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Foreign"})
	require.NoError(t, err)
	for _, req := range []*spacev1.ResolveVoiceRoomAccessRequest{
		{VoiceRoomId: "bad", ProfileId: uuid.NewString(), Space: &spacev1.SpaceRef{Id: created.GetSpace().GetId()}},
		{VoiceRoomId: room.GetVoiceRoom().GetId(), ProfileId: "bad", Space: &spacev1.SpaceRef{Id: created.GetSpace().GetId()}},
		{VoiceRoomId: room.GetVoiceRoom().GetId(), ProfileId: uuid.NewString()},
		{VoiceRoomId: room.GetVoiceRoom().GetId(), ProfileId: uuid.NewString(), Space: &spacev1.SpaceRef{Id: "bad"}},
	} {
		_, err = c.ResolveVoiceRoomAccess(context.Background(), req, grpc.PerRPCCredentials(verifiedVoiceCredentials{}))
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	_, err = c.ResolveVoiceRoomAccess(metadata.AppendToOutgoingContext(context.Background(), "x-voice-service-id", "voice"), &spacev1.ResolveVoiceRoomAccessRequest{VoiceRoomId: room.GetVoiceRoom().GetId(), ProfileId: uuid.NewString(), Space: &spacev1.SpaceRef{Id: created.GetSpace().GetId()}})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = c.ResolveVoiceRoomAccess(context.Background(), &spacev1.ResolveVoiceRoomAccessRequest{VoiceRoomId: room.GetVoiceRoom().GetId(), ProfileId: uuid.NewString(), Space: &spacev1.SpaceRef{Id: created.GetSpace().GetId()}}, grpc.PerRPCCredentials(verifiedVoiceCredentials{}))
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = c.ResolveVoiceRoomAccess(context.Background(), &spacev1.ResolveVoiceRoomAccessRequest{VoiceRoomId: uuid.NewString(), ProfileId: uuid.NewString(), Space: &spacev1.SpaceRef{Id: created.GetSpace().GetId()}}, grpc.PerRPCCredentials(verifiedVoiceCredentials{}))
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = c.ResolveVoiceRoomAccess(context.Background(), &spacev1.ResolveVoiceRoomAccessRequest{VoiceRoomId: room.GetVoiceRoom().GetId(), ProfileId: ownerProfile.String(), Space: &spacev1.SpaceRef{Id: foreign.GetSpace().GetId()}}, grpc.PerRPCCredentials(verifiedVoiceCredentials{}))
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestResolveVoiceRoomAccess_ForwardsExactPathSpaceToResolver(t *testing.T) {
	spaceID := uuid.New()
	voiceRoomID := uuid.New()
	profileID := uuid.New()
	resolver := &recordingVoiceRoomAccessResolver{}
	svc := &SpaceGRPC{voiceRoomAccessResolver: resolver}
	ctx := authctx.WithVerifiedServiceIdentity(context.Background(), authctx.ServiceIdentityVoice)

	resp, err := svc.ResolveVoiceRoomAccess(ctx, &spacev1.ResolveVoiceRoomAccessRequest{
		VoiceRoomId: voiceRoomID.String(),
		ProfileId:   profileID.String(),
		Space:       &spacev1.SpaceRef{Id: spaceID.String()},
	})
	require.NoError(t, err)
	require.True(t, resolver.called)
	require.Equal(t, spaceID, resolver.spaceID)
	require.Equal(t, voiceRoomID, resolver.voiceRoomID)
	require.Equal(t, profileID, resolver.profileID)
	require.Equal(t, spaceID.String(), resp.GetSpaceId())
}

func TestResolveVoiceRoomAccess_ResolverFailureIsUnavailable(t *testing.T) {
	svc := &SpaceGRPC{voiceRoomAccessResolver: failingVoiceRoomAccessResolver{err: errors.New("space database unavailable")}}
	ctx := authctx.WithVerifiedServiceIdentity(context.Background(), authctx.ServiceIdentityVoice)
	resp, err := svc.ResolveVoiceRoomAccess(ctx, &spacev1.ResolveVoiceRoomAccessRequest{
		VoiceRoomId: uuid.NewString(),
		ProfileId:   uuid.NewString(),
		Space:       &spacev1.SpaceRef{Id: uuid.NewString()},
	})
	require.Nil(t, resp)
	require.Equal(t, codes.Unavailable, status.Code(err))
}
