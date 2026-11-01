package grpcsvc

import (
	"context"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

func signedSettingsCall(t *testing.T, f *inviteIngressFixture, actor uuid.UUID, req *spacev1.UpdateSpaceRequest) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	id := uuid.NewString()
	token, err := f.gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_UpdateSpace_FullMethodName, RequestID: id, RequestHash: hash, AccountID: uuid.NewString(), ProfileID: actor.String(), AccountType: "regular", SessionEpoch: 1, ClientExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	return metadata.NewIncomingContext(t.Context(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
}
func verifySettingsRoleCall(t *testing.T, f *inviteIngressFixture, ctx context.Context, req proto.Message, method string) {
	t.Helper()
	md, _ := metadata.FromOutgoingContext(ctx)
	require.Len(t, md, 2)
	require.Len(t, md.Get("authorization"), 1)
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	p, err := principal.VerifyService(ctx, md.Get("authorization")[0][7:], principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "role", ExpectedRPC: method, ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &f.key.PublicKey, nil }})
	require.NoError(t, err)
	require.Equal(t, "service:space", p.Subject)
}

func TestUpdateSpaceSettingsDecisionPrecedesGateAndKeepsIngressBudget(t *testing.T) {
	f := newInviteIngressFixture(t)
	now := time.Now()
	f.verifier.Clock = func() time.Time { return now }
	space, actor := uuid.New(), uuid.New()
	name := "updated"
	req := &spacev1.UpdateSpaceRequest{SpaceId: space.String(), Name: &name}
	svc := &SpaceGRPC{Store: &store.SpaceStore{}, PrincipalIssuer: f.space}
	called := false
	svc.OwnershipRoles = inviteDecisionFunc{check: func(ctx context.Context, r *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		called = true
		require.Equal(t, actor.String(), r.ProfileId)
		require.Equal(t, space.String(), r.SpaceId)
		require.Equal(t, "SPACE_MANAGE_SETTINGS", r.PermissionName)
		require.Nil(t, r.Chat)
		require.Nil(t, r.VoiceRoomId)
		verifySettingsRoleCall(t, f, ctx, r, rolev1.RoleService_CheckPermission_FullMethodName)
		bounded, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		release, err := svc.lockSpaceMutation(bounded, space)
		require.NoError(t, err, "Role callback must precede Space mutation lock")
		if err == nil {
			release()
		}
		now = now.Add(5 * time.Second)
		return &rolev1.CheckPermissionResponse{Allowed: true}, nil
	}}
	_, err := f.verifier.Unary()(signedSettingsCall(t, f, actor, req), req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_UpdateSpace_FullMethodName}, func(ctx context.Context, r any) (any, error) {
		return svc.UpdateSpace(ctx, r.(*spacev1.UpdateSpaceRequest))
	})
	require.True(t, called)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestUpdateSpaceSettingsRejectsRawAndMixedMatchmaking(t *testing.T) {
	req := &spacev1.UpdateSpaceRequest{SpaceId: uuid.NewString()}
	_, err := (&SpaceGRPC{}).UpdateSpace(metadata.NewIncomingContext(t.Context(), metadata.Pairs("x-voice-profile-id", uuid.NewString())), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	calls := 0
	_, err = lifecycleprincipal.OrdinaryUnary()(t.Context(), req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_UpdateSpace_FullMethodName}, func(context.Context, any) (any, error) { calls++; return nil, nil })
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Zero(t, calls)
	f := newInviteIngressFixture(t)
	mm := "{}"
	req.MmConfigJson = &mm
	description := "mixed settings"
	req.Description = &description
	var decisions []string
	svc := &SpaceGRPC{Store: &store.SpaceStore{}, PrincipalIssuer: f.space, OwnershipRoles: inviteDecisionFunc{check: func(ctx context.Context, r *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		verifySettingsRoleCall(t, f, ctx, r, rolev1.RoleService_CheckPermission_FullMethodName)
		decisions = append(decisions, r.PermissionName)
		return &rolev1.CheckPermissionResponse{Allowed: r.PermissionName == "SPACE_MANAGE_MATCHMAKING"}, nil
	}}}
	_, err = f.verifier.Unary()(signedSettingsCall(t, f, uuid.New(), req), req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_UpdateSpace_FullMethodName}, func(ctx context.Context, r any) (any, error) {
		return svc.UpdateSpace(ctx, r.(*spacev1.UpdateSpaceRequest))
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Equal(t, []string{"SPACE_MANAGE_MATCHMAKING", "SPACE_MANAGE_SETTINGS"}, decisions)
}

type settingsRoleNamesClient struct {
	rolev1.RoleServiceClient
	read func(context.Context, *rolev1.GetMemberRolesRequest) (*rolev1.GetMemberRolesResponse, error)
}

func (c settingsRoleNamesClient) GetMemberRoles(ctx context.Context, r *rolev1.GetMemberRolesRequest, _ ...grpc.CallOption) (*rolev1.GetMemberRolesResponse, error) {
	return c.read(ctx, r)
}
func TestMemberRoleNamesUsesProtectedSpaceRead(t *testing.T) {
	f := newInviteIngressFixture(t)
	space, profile := uuid.New(), uuid.New()
	ordinaryCalls, protectedCalls := 0, 0
	svc := &SpaceGRPC{PrincipalIssuer: f.space, Roles: settingsRoleNamesClient{read: func(context.Context, *rolev1.GetMemberRolesRequest) (*rolev1.GetMemberRolesResponse, error) {
		ordinaryCalls++
		return nil, status.Error(codes.Unauthenticated, "native principal required")
	}}}
	svc.OwnershipRoles = settingsRoleNamesClient{read: func(ctx context.Context, r *rolev1.GetMemberRolesRequest) (*rolev1.GetMemberRolesResponse, error) {
		protectedCalls++
		require.Equal(t, space.String(), r.SpaceId)
		require.Equal(t, profile.String(), r.ProfileId)
		verifySettingsRoleCall(t, f, ctx, r, rolev1.RoleService_GetMemberRoles_FullMethodName)
		return &rolev1.GetMemberRolesResponse{RoleList: &rolev1.RoleList{Roles: []*rolev1.Role{{Name: "Owner"}}}}, nil
	}}
	require.Equal(t, []string{"Owner"}, svc.memberRoleNames(t.Context(), space, profile))
	require.Equal(t, 1, protectedCalls)
	require.Zero(t, ordinaryCalls)
	svc.OwnershipRoles = nil
	require.Empty(t, svc.memberRoleNames(t.Context(), space, profile))
	require.Zero(t, ordinaryCalls)
}

func TestBanPermissionUsesProtectedSpaceDecision(t *testing.T) {
	f := newInviteIngressFixture(t)
	space, actor := uuid.New(), uuid.New()
	called := false
	svc := &SpaceGRPC{PrincipalIssuer: f.space, Roles: inviteDecisionFunc{check: func(context.Context, *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		t.Fatal("Ban must not use the ordinary Role listener")
		return nil, nil
	}}}
	svc.OwnershipRoles = inviteDecisionFunc{check: func(ctx context.Context, req *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		called = true
		require.Equal(t, space.String(), req.SpaceId)
		require.Equal(t, actor.String(), req.ProfileId)
		require.Equal(t, "MEMBER_BAN", req.PermissionName)
		verifySettingsRoleCall(t, f, ctx, req, rolev1.RoleService_CheckPermission_FullMethodName)
		return &rolev1.CheckPermissionResponse{Allowed: true}, nil
	}}
	require.NoError(t, svc.checkSpacePermissionForProfile(t.Context(), space, actor, "MEMBER_BAN"))
	require.True(t, called)
}

func TestBanPermissionFailsClosedWithoutProtectedRole(t *testing.T) {
	err := (&SpaceGRPC{}).checkSpacePermissionForProfile(t.Context(), uuid.New(), uuid.New(), "MEMBER_BAN")
	require.Equal(t, codes.Unavailable, status.Code(err))
}
