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
	t                           *testing.T
	key                         *rsa.PublicKey
	store                       *store.SpaceStore
	presentChecks, absentChecks int
	fail, forceMember           bool
}

func (s *communityRosterRoleStub) GetMemberRoles(ctx context.Context, req *rolev1.GetMemberRolesRequest, _ ...grpc.CallOption) (*rolev1.GetMemberRolesResponse, error) {
	verifyOwnershipCredential(s.t, ctx, req, rolev1.RoleService_GetMemberRoles_FullMethodName, s.key)
	if s.fail {
		return nil, status.Error(codes.Unavailable, "Role authority unavailable")
	}
	// Read the actual committed Space authority, rather than maintaining an
	// assignment map that would accept a pre-publication or stale projection.
	observation, err := s.store.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: req.SpaceId, ProfileId: req.ProfileId})
	if err != nil {
		return nil, err
	}
	if s.forceMember || observation.GetCommunity().GetState() == spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE {
		s.presentChecks++
		return &rolev1.GetMemberRolesResponse{RoleList: &rolev1.RoleList{Roles: []*rolev1.Role{{Id: uuid.NewString(), SpaceId: req.SpaceId, Name: permissions.RoleMember, Managed: true}}}}, nil
	}
	s.absentChecks++
	return &rolev1.GetMemberRolesResponse{RoleList: &rolev1.RoleList{}}, nil
}

func TestApplyCommunityRosterFencesSpaceAndProjectsOnlyBaselineRole(t *testing.T) {
	ctx := context.Background()
	pool := communityPublicationFixture(t, ctx)
	st := &store.SpaceStore{Pool: pool}
	app, env, ownerAccount, ownerProfile := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	space, _, err := st.CreateCommunityBootstrapSpace(ctx, store.CommunityBootstrapSpaceInput{OperationID: uuid.New(), ApplicationID: app, EnvironmentID: env, OwnerAccountID: ownerAccount, OwnerProfileID: ownerProfile, CorporationKey: "corp-rpc", TemplateID: "guild-default"})
	require.NoError(t, err)
	issuer, key := newOwnershipTestIssuer(t)
	roleStub := &communityRosterRoleStub{t: t, key: key, store: st}
	memberAccount := uuid.New()
	profile := uuid.New()
	svc := &SpaceGRPC{Store: st, Roles: deniedCommunityRoleClient{}, OwnershipRoles: roleStub, PrincipalIssuer: issuer, ProfileAccounts: mapProfileAccounts{ownerProfile: ownerAccount, profile: memberAccount}}
	serviceCtx := authctx.WithVerifiedServiceIdentity(ctx, authctx.ServiceIdentityGameIntegration)
	req := &spacev1.ApplyCommunityRosterRequest{OperationId: uuid.NewString(), ApplicationId: app.String(), EnvironmentId: env.String(), CorporationKey: "corp-rpc", SpaceId: space.ID.String(), OwnerGeneration: 1, SourceRevision: 1, SnapshotSha256: make([]byte, 32), ProfileIds: []string{profile.String()}, LeaseExpiresAt: timestamppb.New(time.Now().Add(time.Minute))}
	roleStub.fail = true
	_, err = svc.ApplyCommunityRoster(serviceCtx, req)
	require.Equal(t, codes.Unavailable, status.Code(err), "a stored receipt cannot hide an unavailable Role projection")
	roleStub.fail = false
	first, err := svc.ApplyCommunityRoster(serviceCtx, req)
	require.NoError(t, err)
	require.NotNil(t, first.GetReceiptId())
	require.True(t, first.Replayed, "retry reuses the receipt committed before the Role outage")
	require.Equal(t, 1, roleStub.presentChecks)
	replay, err := svc.ApplyCommunityRoster(serviceCtx, req)
	require.NoError(t, err)
	require.True(t, replay.GetReplayed())
	require.Equal(t, first.ReceiptId, replay.ReceiptId)
	require.Equal(t, first.RequestSha256, replay.RequestSha256)
	require.Equal(t, 2, roleStub.presentChecks)
	for _, field := range []string{"application", "environment", "space", "digest"} {
		changed := proto.Clone(req).(*spacev1.ApplyCommunityRosterRequest)
		switch field {
		case "application":
			changed.ApplicationId = uuid.NewString()
		case "environment":
			changed.EnvironmentId = uuid.NewString()
		case "space":
			changed.SpaceId = uuid.NewString()
		case "digest":
			changed.SnapshotSha256 = bytesOf("changed")
		}
		_, err := svc.ApplyCommunityRoster(serviceCtx, changed)
		require.Equal(t, codes.FailedPrecondition, status.Code(err), "operation cannot be rebound to %s", field)
	}
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
	roleStub.forceMember = true
	_, err = svc.ApplyCommunityRoster(serviceCtx, depart)
	require.Equal(t, codes.Unavailable, status.Code(err), "an unconfirmed revocation cannot return the durable receipt as success")
	roleStub.forceMember = false
	departReceipt, err := svc.ApplyCommunityRoster(serviceCtx, depart)
	require.NoError(t, err)
	require.True(t, departReceipt.Replayed)
	require.Equal(t, depart.GetOperationId(), departReceipt.GetOperationId())
	require.Equal(t, 1, roleStub.absentChecks, "native projection disappears after source departure, without a generic Role mutation")
	departReplay, err := svc.ApplyCommunityRoster(serviceCtx, depart)
	require.NoError(t, err)
	require.Equal(t, departReceipt.ReceiptId, departReplay.ReceiptId)
	require.True(t, departReplay.Replayed)
	roleStub.forceMember = true
	require.Equal(t, codes.Unavailable, status.Code(svc.revokeCommunityMemberRole(ctx, space.ID, profile)), "a retained community Member projection is not a successful revocation")
	roleStub.forceMember = false
	checks := roleStub.presentChecks + roleStub.absentChecks
	require.NoError(t, svc.revokeCommunityMemberRole(ctx, space.ID, ownerProfile), "independent ordinary membership is never revoked")
	require.Equal(t, checks, roleStub.presentChecks+roleStub.absentChecks)
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
