package grpcsvc

import (
	"context"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/permissions"
)

// Legacy semantic fixtures provide explicit test identities, never mint authority
// from transport headers. Native51/Role25 evidence is exercised separately.
func (f *inviteIngressFixture) settingsContext(ctx context.Context, req proto.Message, method string) (context.Context, error) {
	actor, ok := ctx.Value(inviteFixtureActorKey{}).(inviteFixtureActor)
	if !ok {
		return ctx, nil
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, err
	}
	id := uuid.NewString()
	token, err := f.gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: method, RequestID: id, RequestHash: hash, AccountID: actor.account.String(), ProfileID: actor.profile.String(), AccountType: "regular", SessionEpoch: 1, ClientExpiresAt: time.Now().Add(time.Minute)})
	if err != nil {
		return nil, err
	}
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), nil
}
func (c inviteFixtureClient) CreateVoiceRoom(ctx context.Context, r *spacev1.CreateVoiceRoomRequest, opts ...grpc.CallOption) (*spacev1.CreateVoiceRoomResponse, error) {
	ctx, err := c.fixture.settingsContext(ctx, r, spacev1.SpaceService_CreateVoiceRoom_FullMethodName)
	if err != nil {
		return nil, err
	}
	return c.SpaceServiceClient.CreateVoiceRoom(ctx, r, opts...)
}

func (c inviteFixtureClient) UpdateSpace(ctx context.Context, r *spacev1.UpdateSpaceRequest, opts ...grpc.CallOption) (*spacev1.UpdateSpaceResponse, error) {
	ctx, err := c.fixture.settingsContext(ctx, r, spacev1.SpaceService_UpdateSpace_FullMethodName)
	if err != nil {
		return nil, err
	}
	return c.SpaceServiceClient.UpdateSpace(ctx, r, opts...)
}
func (c inviteFixtureClient) UpdateSpaceMmConfig(ctx context.Context, r *spacev1.UpdateSpaceMmConfigRequest, opts ...grpc.CallOption) (*spacev1.UpdateSpaceMmConfigResponse, error) {
	ctx, err := c.fixture.settingsContext(ctx, r, spacev1.SpaceService_UpdateSpaceMmConfig_FullMethodName)
	if err != nil {
		return nil, err
	}
	return c.SpaceServiceClient.UpdateSpaceMmConfig(ctx, r, opts...)
}

type legacySettingsRoleFixture struct {
	rolev1.RoleServiceClient
	svc     *SpaceGRPC
	fixture *inviteIngressFixture
}

func (c legacySettingsRoleFixture) verify(ctx context.Context, req proto.Message, method string) error {
	md, _ := metadata.FromOutgoingContext(ctx)
	if len(md.Get("authorization")) != 1 || len(md.Get("x-request-id")) != 1 {
		return status.Error(codes.Unauthenticated, "signed fixture decision required")
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return err
	}
	_, err = principal.VerifyService(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "role", ExpectedRPC: method, ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &c.fixture.key.PublicKey, nil }})
	return err
}
func (c legacySettingsRoleFixture) CheckPermission(ctx context.Context, r *rolev1.CheckPermissionRequest, _ ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	if err := c.verify(ctx, r, rolev1.RoleService_CheckPermission_FullMethodName); err != nil {
		return nil, err
	}
	if r.Chat != nil || r.VoiceRoomId != nil || (r.PermissionName != permissions.TextChatCreateInSpace && r.PermissionName != permissions.SpaceManageSettings && r.PermissionName != permissions.SpaceManageMatchmaking && r.PermissionName != permissions.MemberBan) {
		return nil, status.Error(codes.PermissionDenied, "wrong fixture purpose")
	}
	// Model the native Role owner's fresh Space qualification before legacy mask lookup.
	if err := c.svc.Store.CheckOwnershipAvailable(ctx, uuid.MustParse(r.SpaceId)); err != nil {
		return nil, mapSpaceStoreError(err)
	}
	if c.RoleServiceClient != nil {
		result, err := c.GetEffectivePermissions(metadata.NewOutgoingContext(ctx, metadata.MD{}), &rolev1.GetEffectivePermissionsRequest{SpaceId: r.SpaceId, ProfileId: r.ProfileId})
		if err != nil {
			return nil, err
		}
		allowed, err := permissions.HasPermission(result.GetPermissionSet().GetEffectiveMask(), r.PermissionName)
		return &rolev1.CheckPermissionResponse{Allowed: allowed}, err
	}
	row, err := c.svc.Store.GetSpace(ctx, uuid.MustParse(r.SpaceId))
	if err != nil {
		return nil, mapSpaceStoreError(err)
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "space not found")
	}
	return &rolev1.CheckPermissionResponse{Allowed: row != nil && row.OwnerProfileID.String() == r.ProfileId}, nil
}
func (c legacySettingsRoleFixture) GetMemberRoles(ctx context.Context, r *rolev1.GetMemberRolesRequest, opts ...grpc.CallOption) (*rolev1.GetMemberRolesResponse, error) {
	if err := c.verify(ctx, r, rolev1.RoleService_GetMemberRoles_FullMethodName); err != nil {
		return nil, err
	}
	return c.RoleServiceClient.GetMemberRoles(metadata.NewOutgoingContext(ctx, metadata.MD{}), r, opts...)
}

// settingsOwnershipFixture preserves the legacy ownership participant while using
// the verified settings decision fixture on the same service instance.
type settingsOwnershipFixture struct {
	rolev1.RoleServiceClient
	decision legacySettingsRoleFixture
}

func (c settingsOwnershipFixture) CheckPermission(ctx context.Context, r *rolev1.CheckPermissionRequest, opts ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	return c.decision.CheckPermission(ctx, r, opts...)
}

func (f *inviteIngressFixture) serveSettings(ctx context.Context, r any, info *grpc.UnaryServerInfo, svc *SpaceGRPC) (any, error) {
	return f.verifier.Unary()(ctx, r, info, func(ctx context.Context, r any) (any, error) {
		// The isolated view supports sequential legacy semantics only. Concurrency
		// callers must preconfigure svc before launching work, preserving its gate.
		clone := &SpaceGRPC{Store: svc.Store, SpaceEvents: svc.SpaceEvents, MutationLocker: svc.MutationLocker, Logger: svc.Logger,
			PrincipalIssuer: f.space, OwnershipRoles: legacySettingsRoleFixture{RoleServiceClient: svc.Roles, svc: svc, fixture: f}}
		// A preconfigured service retains its original local mutation gate.
		if svc.PrincipalIssuer == f.space {
			clone = svc
		}
		switch req := r.(type) {
		case *spacev1.CreateVoiceRoomRequest:
			return clone.CreateVoiceRoom(ctx, req)
		case *spacev1.UpdateSpaceRequest:
			return clone.UpdateSpace(ctx, req)
		case *spacev1.UpdateSpaceMmConfigRequest:
			return clone.UpdateSpaceMmConfig(ctx, req)
		}
		return nil, status.Error(codes.Unimplemented, "fixture settings method")
	})
}

func settingsFixtureUpdate(t *testing.T, ctx context.Context, svc *SpaceGRPC, req *spacev1.UpdateSpaceRequest) (*spacev1.UpdateSpaceResponse, error) {
	t.Helper()
	return settingsFixtureUpdateWithFixture(t, newInviteIngressFixture(t), ctx, svc, req)
}

func settingsFixtureUpdateWithFixture(t *testing.T, f *inviteIngressFixture, ctx context.Context, svc *SpaceGRPC, req *spacev1.UpdateSpaceRequest) (*spacev1.UpdateSpaceResponse, error) {
	t.Helper()
	signed, err := f.settingsContext(ctx, req, spacev1.SpaceService_UpdateSpace_FullMethodName)
	if err != nil {
		return nil, err
	}
	md, _ := metadata.FromOutgoingContext(signed)
	signed = metadata.NewIncomingContext(signed, md)
	result, err := f.serveSettings(signed, req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_UpdateSpace_FullMethodName}, svc)
	if result == nil {
		return nil, err
	}
	return result.(*spacev1.UpdateSpaceResponse), err
}
