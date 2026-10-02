package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
	"voice/backend/role/permissions"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"
)

type communityRosterRoleStub struct {
	rolev1.RoleServiceClient
	assigned []uuid.UUID
	revoked  []uuid.UUID
	members  map[uuid.UUID]bool
}

func (s *communityRosterRoleStub) GetMemberRoles(_ context.Context, req *rolev1.GetMemberRolesRequest, _ ...grpc.CallOption) (*rolev1.GetMemberRolesResponse, error) {
	if s.members[uuid.MustParse(req.GetProfileId())] {
		return &rolev1.GetMemberRolesResponse{RoleList: &rolev1.RoleList{Roles: []*rolev1.Role{{Id: "member-role", Name: permissions.RoleMember}}}}, nil
	}
	return &rolev1.GetMemberRolesResponse{RoleList: &rolev1.RoleList{}}, nil
}
func (s *communityRosterRoleStub) GetDefaultJoinRole(context.Context, *rolev1.GetDefaultJoinRoleRequest, ...grpc.CallOption) (*rolev1.GetDefaultJoinRoleResponse, error) {
	return &rolev1.GetDefaultJoinRoleResponse{Role: &rolev1.Role{Id: "member-role", Name: permissions.RoleMember}}, nil
}
func (s *communityRosterRoleStub) AssignRole(_ context.Context, req *rolev1.AssignRoleRequest, _ ...grpc.CallOption) (*rolev1.AssignRoleResponse, error) {
	id := uuid.MustParse(req.GetProfileId())
	s.members[id] = true
	s.assigned = append(s.assigned, id)
	return &rolev1.AssignRoleResponse{}, nil
}
func (s *communityRosterRoleStub) ListRoles(context.Context, *rolev1.ListRolesRequest, ...grpc.CallOption) (*rolev1.ListRolesResponse, error) {
	return &rolev1.ListRolesResponse{RoleList: &rolev1.RoleList{Roles: []*rolev1.Role{{Id: "member-role", Name: permissions.RoleMember}}}}, nil
}
func (s *communityRosterRoleStub) RevokeRole(_ context.Context, req *rolev1.RevokeRoleRequest, _ ...grpc.CallOption) (*rolev1.RevokeRoleResponse, error) {
	id := uuid.MustParse(req.GetProfileId())
	delete(s.members, id)
	s.revoked = append(s.revoked, id)
	return &rolev1.RevokeRoleResponse{}, nil
}

func TestApplyCommunityRosterFencesSpaceAndProjectsOnlyBaselineRole(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	st := &store.SpaceStore{Pool: pool}
	app, env, ownerAccount, ownerProfile := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	space, _, err := st.CreateCommunityBootstrapSpace(ctx, store.CommunityBootstrapSpaceInput{OperationID: uuid.New(), ApplicationID: app, EnvironmentID: env, OwnerAccountID: ownerAccount, OwnerProfileID: ownerProfile, CorporationKey: "corp-rpc", TemplateID: "guild-default"})
	require.NoError(t, err)
	roleStub := &communityRosterRoleStub{members: map[uuid.UUID]bool{}}
	memberAccount := uuid.New()
	profile := uuid.New()
	svc := &SpaceGRPC{Store: st, Roles: roleStub, ProfileAccounts: mapProfileAccounts{ownerProfile: ownerAccount, profile: memberAccount}}
	serviceCtx := authctx.WithVerifiedServiceIdentity(ctx, authctx.ServiceIdentityGameIntegration)
	req := &spacev1.ApplyCommunityRosterRequest{OperationId: uuid.NewString(), ApplicationId: app.String(), EnvironmentId: env.String(), CorporationKey: "corp-rpc", SpaceId: space.ID.String(), OwnerGeneration: 1, SourceRevision: 1, SnapshotSha256: make([]byte, 32), ProfileIds: []string{profile.String()}, LeaseExpiresAt: timestamppb.New(time.Now().Add(time.Minute))}
	first, err := svc.ApplyCommunityRoster(serviceCtx, req)
	require.NoError(t, err)
	require.NotNil(t, first.GetReceiptId())
	require.Equal(t, 1, len(roleStub.assigned))
	require.Equal(t, profile, roleStub.assigned[0])
	replay, err := svc.ApplyCommunityRoster(serviceCtx, req)
	require.NoError(t, err)
	require.True(t, replay.GetReplayed())
	require.Len(t, roleStub.assigned, 1)
	coReq := &spacev1.AreCoMembersRequest{ProfileIdA: ownerProfile.String(), ProfileIdB: profile.String(), SpaceIds: []string{space.ID.String()}}
	hash, err := principal.RequestHash(coReq)
	require.NoError(t, err)
	socialPrincipal := principal.Principal{Kind: "service", Issuer: "social", Subject: "service:social", Audience: "space", RPC: socialprincipal.Method("space"), RequestHash: hash}
	coCtx := principal.WithVerified(ctx, socialPrincipal)
	coResponse, err := svc.AreCoMembers(coCtx, coReq)
	require.NoError(t, err)
	require.True(t, coResponse.GetCoMembers(), "Chat's Space audience check sees the active roster member")
	_, err = pool.Exec(ctx, `INSERT INTO space_bans(space_id,account_id,banned_by_profile_id,reason) VALUES($1,$2,$3,'policy')`, space.ID, memberAccount, ownerProfile)
	require.NoError(t, err)
	coResponse, err = svc.AreCoMembers(coCtx, coReq)
	require.NoError(t, err)
	require.False(t, coResponse.GetCoMembers(), "Chat/Search audience check immediately excludes a banned account")
	_, err = pool.Exec(ctx, `DELETE FROM space_bans WHERE space_id=$1 AND account_id=$2`, space.ID, memberAccount)
	require.NoError(t, err)
	depart := proto.Clone(req).(*spacev1.ApplyCommunityRosterRequest)
	depart.OperationId = uuid.NewString()
	depart.SourceRevision = 2
	depart.SnapshotSha256 = bytesOf("2")
	depart.ProfileIds = nil
	departReceipt, err := svc.ApplyCommunityRoster(serviceCtx, depart)
	require.NoError(t, err)
	require.Equal(t, depart.GetOperationId(), departReceipt.GetOperationId())
	require.Equal(t, []uuid.UUID{profile}, roleStub.revoked, "only the baseline Member role is revoked after source departure")
	_, err = pool.Exec(ctx, `UPDATE community_owner_authority SET owner_generation=2,roster_source_revision=0,roster_sha256=NULL,roster_lease_expires_at=NULL WHERE space_id=$1`, space.ID)
	require.NoError(t, err)
	stale := proto.Clone(req).(*spacev1.ApplyCommunityRosterRequest)
	stale.OperationId = uuid.NewString()
	stale.SourceRevision = 3
	stale.LeaseExpiresAt = timestamppb.New(time.Now().Add(time.Minute))
	_, err = svc.ApplyCommunityRoster(serviceCtx, stale)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "stale generation cannot restore Space or Role access")
	_, err = svc.ApplyCommunityRoster(ctx, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "forwarded IDs cannot replace verified GIS service identity")
}

func bytesOf(value string) []byte { b := make([]byte, 32); copy(b, []byte(value)); return b }
