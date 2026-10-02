package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"
)

func TestCreateCommunityBootstrapIsPrivateOwnerBoundAndReplaySafe(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	ownerProfile := uuid.New()
	ownerAccount := uuid.New()
	serviceCtx := authctx.WithVerifiedServiceIdentity(ctx, authctx.ServiceIdentityGameIntegration)
	svc := &SpaceGRPC{Store: &store.SpaceStore{Pool: pool}}
	request := &spacev1.CreateCommunityBootstrapRequest{
		OperationId: uuid.NewString(), ApplicationId: uuid.NewString(), EnvironmentId: uuid.NewString(),
		OwnerProfileId: ownerProfile.String(), OwnerAccountId: ownerAccount.String(), CorporationKey: "corp-7", TemplateId: "guild-default",
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
