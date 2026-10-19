package grpcsvc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"
)

func TestCreateCommunityBootstrapIsPrivateOwnerBoundAndReplaySafe(t *testing.T) {
	ctx := context.Background()
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	database.Run(true, "up")
	pool := database.Pool
	ownerProfile := uuid.New()
	ownerAccount := uuid.New()
	serviceCtx := authctx.WithVerifiedServiceIdentity(ctx, authctx.ServiceIdentityGameIntegration)
	issuer, key := newOwnershipTestIssuer(t)
	ordinaryCalls, protectedCalls := 0, 0
	ordinary := &creatorBootstrapRoleClient{bootstrap: func(context.Context, *rolev1.BootstrapSpaceRolesRequest) (*rolev1.BootstrapSpaceRolesResponse, error) {
		ordinaryCalls++
		return nil, status.Error(codes.Unavailable, "protected lifecycle listener required")
	}}
	protected := &creatorBootstrapRoleClient{bootstrap: func(ctx context.Context, req *rolev1.BootstrapSpaceRolesRequest) (*rolev1.BootstrapSpaceRolesResponse, error) {
		protectedCalls++
		verifyOwnershipCredential(t, ctx, req, rolev1.RoleService_BootstrapSpaceRoles_FullMethodName, key)
		require.Equal(t, ownerProfile.String(), req.GetOwnerProfileId())
		return &rolev1.BootstrapSpaceRolesResponse{}, nil
	}}
	svc := &SpaceGRPC{Store: &store.SpaceStore{Pool: pool}, Roles: ordinary, OwnershipRoles: protected, PrincipalIssuer: issuer}
	request := &spacev1.CreateCommunityBootstrapRequest{
		OperationId: uuid.NewString(), ApplicationId: uuid.NewString(), EnvironmentId: uuid.NewString(),
		OwnerProfileId: ownerProfile.String(), OwnerAccountId: ownerAccount.String(), CorporationKey: "corp-7", TemplateId: "guild-default",
	}
	for _, missing := range []string{"issuer", "protected_client"} {
		unready := &SpaceGRPC{Store: svc.Store, Roles: ordinary, OwnershipRoles: protected, PrincipalIssuer: issuer}
		if missing == "issuer" {
			unready.PrincipalIssuer = nil
		} else {
			unready.OwnershipRoles = nil
		}
		_, err := unready.CreateCommunityBootstrap(serviceCtx, request)
		require.Equal(t, codes.Unavailable, status.Code(err))
		var count int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM community_bootstrap_operations WHERE operation_id=$1`, request.GetOperationId()).Scan(&count))
		require.Zero(t, count, "missing protected dependency must not create durable community state")
	}
	first, err := svc.CreateCommunityBootstrap(serviceCtx, request)
	require.NoError(t, err)
	require.NotNil(t, first.GetSpace())
	require.Equal(t, ownerProfile.String(), first.GetSpace().GetOwnerProfileId())
	require.Equal(t, "private", first.GetSpace().GetVisibility())
	require.False(t, first.GetReplayed())

	replay, err := svc.CreateCommunityBootstrap(serviceCtx, request)
	require.NoError(t, err)
	require.True(t, replay.GetReplayed())
	require.Equal(t, first.GetSpace().GetId(), replay.GetSpace().GetId())
	require.Equal(t, 0, ordinaryCalls, "ordinary Role bootstrap must remain denied")
	require.Equal(t, 2, protectedCalls, "creation and durable replay both restore the protected Owner baseline")
	changed := proto.Clone(request).(*spacev1.CreateCommunityBootstrapRequest)
	changed.CorporationKey = "corp-8"
	_, err = svc.CreateCommunityBootstrap(serviceCtx, changed)
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	unknownTemplate := proto.Clone(request).(*spacev1.CreateCommunityBootstrapRequest)
	unknownTemplate.OperationId = uuid.NewString()
	unknownTemplate.CorporationKey = "corp-8"
	unknownTemplate.TemplateId = "unreviewed-template"
	_, err = svc.CreateCommunityBootstrap(serviceCtx, unknownTemplate)
	require.Equal(t, codes.PermissionDenied, status.Code(err), "Space accepts only the reviewed built-in private baseline")
	_, err = svc.CreateCommunityBootstrap(ctx, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "forwarded Owner fields cannot substitute for verified GIS service identity")
	var memberCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, first.GetSpace().GetId(), ownerProfile).Scan(&memberCount))
	require.Equal(t, 1, memberCount)
	var operationCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM community_bootstrap_operations WHERE operation_id=$1`, request.GetOperationId()).Scan(&operationCount))
	require.Equal(t, 1, operationCount)
}
