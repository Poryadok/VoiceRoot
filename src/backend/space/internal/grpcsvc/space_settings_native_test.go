package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	"time"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/store"
)

type settingsEnsureUnavailable struct {
	rolev1.RoleServiceClient
	call func(context.Context, *rolev1.EnsureDefaultMemberRoleRequest)
}

func (c settingsEnsureUnavailable) EnsureDefaultMemberRole(ctx context.Context, r *rolev1.EnsureDefaultMemberRoleRequest, _ ...grpc.CallOption) (*rolev1.EnsureDefaultMemberRoleResponse, error) {
	c.call(ctx, r)
	return nil, status.Error(codes.Unavailable, "controlled Role outage; never fabricate receipt")
}

func TestNative51SettingsAndMissingAlternateMembership(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	owner, alternate, account := uuid.New(), uuid.New(), uuid.New()
	ctx := t.Context()
	pool, row := nativeLifecycleProofFixtureWithSeed(t, ctx, owner, "Native settings and alternate", nil, true)
	st := &store.SpaceStore{Pool: pool}
	f := newInviteIngressFixture(t)
	svc := &SpaceGRPC{Store: st, PrincipalIssuer: f.space, ProfileAccounts: mapProfileAccounts{owner: account, alternate: account}, Blocks: permissiveJoinBlocks{}}
	svc.OwnershipRoles = inviteDecisionFunc{check: func(ctx context.Context, r *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		verifySettingsRoleCall(t, f, ctx, r, rolev1.RoleService_CheckPermission_FullMethodName)
		require.Equal(t, "SPACE_MANAGE_SETTINGS", r.PermissionName)
		return &rolev1.CheckPermissionResponse{Allowed: r.ProfileId == owner.String()}, nil
	}}
	name, visibility := "Changed by verified owner", "public"
	req := &spacev1.UpdateSpaceRequest{SpaceId: row.ID.String(), Name: &name, Visibility: &visibility}
	invokeUpdate := func(actor uuid.UUID) error {
		_, err := f.verifier.Unary()(signedSettingsCall(t, f, actor, req), req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_UpdateSpace_FullMethodName}, func(ctx context.Context, r any) (any, error) {
			return svc.UpdateSpace(ctx, r.(*spacev1.UpdateSpaceRequest))
		})
		return err
	}
	require.NoError(t, invokeUpdate(owner))
	got, err := st.GetSpace(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, name, got.Name)
	denied := "Denied alternate change"
	req.Name = &denied
	require.Equal(t, codes.PermissionDenied, status.Code(invokeUpdate(alternate)))
	got, err = st.GetSpace(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, name, got.Name)
	missing, err := st.GetMembership(ctx, row.ID, alternate)
	require.NoError(t, err)
	require.Nil(t, missing, "store absent-row contract is nil,nil")
	roleCalls := 0
	svc.OwnershipRoles = settingsEnsureUnavailable{call: func(ctx context.Context, r *rolev1.EnsureDefaultMemberRoleRequest) {
		roleCalls++
		require.Equal(t, alternate.String(), r.ProfileId)
		require.Equal(t, row.ID.String(), r.SpaceId)
		verifySettingsRoleCall(t, f, ctx, r, rolev1.RoleService_EnsureDefaultMemberRole_FullMethodName)
	}}
	public := &PublicLifecycleSpace{SpaceGRPC: svc}
	join := func(profile uuid.UUID) (*spacev1.JoinSpaceResponse, error) {
		r := &spacev1.JoinSpaceRequest{SpaceId: row.ID.String()}
		hash, err := principal.RequestHash(r)
		require.NoError(t, err)
		id := uuid.NewString()
		token, err := f.gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_JoinSpace_FullMethodName, RequestID: id, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), AccountType: "regular", SessionEpoch: 1, ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, err)
		signed := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
		result, err := f.verifier.Unary()(signed, r, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_JoinSpace_FullMethodName}, func(ctx context.Context, r any) (any, error) {
			return public.JoinSpace(ctx, r.(*spacev1.JoinSpaceRequest))
		})
		if result == nil {
			return nil, err
		}
		return result.(*spacev1.JoinSpaceResponse), err
	}
	already, err := join(owner)
	require.NoError(t, err)
	require.Equal(t, owner.String(), already.GetSpaceMembership().GetProfileId())
	require.Zero(t, roleCalls)
	_, err = join(alternate)
	require.Equal(t, codes.Unavailable, status.Code(err), "missing member must reach admission, never report empty success")
	require.Equal(t, 1, roleCalls)
	pending, err := st.FindCurrent(ctx, row.ID, alternate)
	require.NoError(t, err)
	require.Equal(t, "PREPARING", pending.State)
	require.Equal(t, account, pending.Input.AccountID)
	require.Equal(t, alternate, pending.Input.ProfileID)
	missing, err = st.GetMembership(ctx, row.ID, alternate)
	require.NoError(t, err)
	require.Nil(t, missing, "Role failure must not publish membership")
}
