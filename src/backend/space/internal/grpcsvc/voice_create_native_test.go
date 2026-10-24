package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

// Real native51 persistence and signed ingress; Role is an explicit controlled
// decision dependency. Role's actual protected creation listener is tested in
// principalruntime, not replaced by this fixture's allow/deny function.
func TestVoiceCreateNative51SignedHandlerAndRollback(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	owner, account := uuid.New(), uuid.New()
	ctx := t.Context()
	pool, row := nativeLifecycleProofFixtureWithSeed(t, ctx, owner, "Native voice creation", nil, true)
	st := &store.SpaceStore{Pool: pool}
	f := newInviteIngressFixture(t)
	svc := &SpaceGRPC{Store: st, PrincipalIssuer: f.space}
	svc.OwnershipRoles = inviteDecisionFunc{check: func(ctx context.Context, r *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		verifySettingsRoleCall(t, f, ctx, r, rolev1.RoleService_CheckPermission_FullMethodName)
		require.Equal(t, "TEXT_CHAT_CREATE_IN_SPACE", r.PermissionName)
		require.Equal(t, row.ID.String(), r.SpaceId)
		require.Nil(t, r.Chat)
		require.Nil(t, r.VoiceRoomId)
		return &rolev1.CheckPermissionResponse{Allowed: r.ProfileId == owner.String()}, nil
	}}
	req := &spacev1.CreateVoiceRoomRequest{SpaceId: row.ID.String(), Name: "Team voice"}
	invoke := func(actor uuid.UUID) (*spacev1.CreateVoiceRoomResponse, error) {
		explicit := context.WithValue(ctx, inviteFixtureActorKey{}, inviteFixtureActor{account, actor})
		signed, err := f.settingsContext(explicit, req, spacev1.SpaceService_CreateVoiceRoom_FullMethodName)
		require.NoError(t, err)
		md, _ := metadata.FromOutgoingContext(signed)
		out, err := f.verifier.Unary()(metadata.NewIncomingContext(ctx, md), req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_CreateVoiceRoom_FullMethodName}, func(ctx context.Context, r any) (any, error) {
			return svc.CreateVoiceRoom(ctx, r.(*spacev1.CreateVoiceRoomRequest))
		})
		if out == nil {
			return nil, err
		}
		return out.(*spacev1.CreateVoiceRoomResponse), err
	}
	out, err := invoke(owner)
	require.NoError(t, err)
	require.Equal(t, row.ID.String(), out.GetVoiceRoom().GetSpaceId())
	require.Equal(t, req.Name, out.GetVoiceRoom().GetName())
	var rooms, nodes int
	count := func() {
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_rooms WHERE space_id=$1`, row.ID).Scan(&rooms))
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_tree_nodes WHERE space_id=$1 AND kind='voice_room'`, row.ID).Scan(&nodes))
	}
	count()
	require.Equal(t, 1, rooms)
	require.Equal(t, 1, nodes)
	_, err = invoke(uuid.New())
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	_, err = svc.CreateVoiceRoom(metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-profile-id", owner.String())), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = lifecycleprincipal.OrdinaryUnary()(ctx, req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_CreateVoiceRoom_FullMethodName}, func(context.Context, any) (any, error) { t.Fatal("ordinary request reached handler"); return nil, nil })
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	// This callback runs only after actual ownership/lifecycle store fences. A
	// refusal must roll back both first-write and pre-commit validation failures.
	for _, failAt := range []int{1, 2} {
		calls := 0
		_, _, err = st.CreateVoiceRoom(ctx, row.ID, "must roll back", nil, func(context.Context) error {
			calls++
			if calls == failAt {
				return status.Error(codes.Unavailable, "original admission observation expired")
			}
			return nil
		})
		require.Equal(t, codes.Unavailable, status.Code(err))
		require.Equal(t, failAt, calls)
		count()
		require.Equal(t, 1, rooms)
		require.Equal(t, 1, nodes)
	}
	// Elapsed original ingress budget cannot become a fresh write authorization.
	now := time.Now()
	f.verifier.Clock = func() time.Time { return now }
	svc.OwnershipRoles = inviteDecisionFunc{check: func(context.Context, *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		now = now.Add(5 * time.Second)
		return &rolev1.CheckPermissionResponse{Allowed: true}, nil
	}}
	_, err = invoke(owner)
	require.Equal(t, codes.Unavailable, status.Code(err))
	count()
	require.Equal(t, 1, rooms)
	require.Equal(t, 1, nodes)
}
