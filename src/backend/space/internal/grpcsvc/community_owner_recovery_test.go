package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"
)

type communityRecoveryRoleClient struct {
	rolev1.RoleServiceClient
	prepareCalls    int
	finalizeCalls   int
	finalizeFailure error
	newOwnerProfile uuid.UUID
}

func (c *communityRecoveryRoleClient) PrepareOwnershipTransfer(_ context.Context, req *rolev1.PrepareOwnershipTransferRequest, _ ...grpc.CallOption) (*rolev1.PrepareOwnershipTransferResponse, error) {
	c.prepareCalls++
	return &rolev1.PrepareOwnershipTransferResponse{Receipt: &rolev1.OwnershipTransferReceipt{Intent: proto.Clone(req.GetIntent()).(*rolev1.OwnershipTransferIntent), State: rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_PREPARED}}, nil
}
func (c *communityRecoveryRoleClient) FinalizeOwnershipTransfer(_ context.Context, req *rolev1.FinalizeOwnershipTransferRequest, _ ...grpc.CallOption) (*rolev1.FinalizeOwnershipTransferResponse, error) {
	c.finalizeCalls++
	if c.finalizeFailure != nil {
		err := c.finalizeFailure
		c.finalizeFailure = nil
		return nil, err
	}
	return &rolev1.FinalizeOwnershipTransferResponse{Receipt: &rolev1.OwnershipTransferReceipt{Intent: proto.Clone(req.GetIntent()).(*rolev1.OwnershipTransferIntent), State: rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, CurrentOwnerProfileId: c.newOwnerProfile.String()}}, nil
}

func TestRecoverCommunityOwnerUsesRoleV2ReceiptAndFencesGeneration(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	ownerAccount, ownerProfile, replacementAccount, replacementProfile := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	role := &communityRecoveryRoleClient{newOwnerProfile: replacementProfile}
	issuer := ownershipTestIssuer(t)
	svc := &SpaceGRPC{Store: &store.SpaceStore{Pool: pool}, OwnershipRoles: role, PrincipalIssuer: issuer}
	bootstrap := &spacev1.CreateCommunityBootstrapRequest{OperationId: uuid.NewString(), ApplicationId: uuid.NewString(), EnvironmentId: uuid.NewString(), OwnerAccountId: ownerAccount.String(), OwnerProfileId: ownerProfile.String(), CorporationKey: "corp-7", TemplateId: "guild-default"}
	created, err := svc.CreateCommunityBootstrap(authctx.WithVerifiedServiceIdentity(ctx, authctx.ServiceIdentityGameIntegration), bootstrap)
	require.NoError(t, err)
	spaceID := uuid.MustParse(created.GetSpace().GetId())
	operationID := uuid.New()
	digest := make([]byte, 32)
	for i := range digest {
		digest[i] = byte(i + 1)
	}
	req := &spacev1.RecoverCommunityOwnerRequest{OperationId: operationID.String(), ApplicationId: bootstrap.GetApplicationId(), EnvironmentId: bootstrap.GetEnvironmentId(), CorporationKey: "corp-7", SpaceId: spaceID.String(), PreviousOwnerAccountId: ownerAccount.String(), PreviousOwnerProfileId: ownerProfile.String(), ReplacementAccountId: replacementAccount.String(), ReplacementProfileId: replacementProfile.String(), ExpectedGeneration: 1, ReasonCode: "owner_lost", EvidenceSha256: digest}
	serviceCtx := authctx.WithVerifiedServiceIdentity(ctx, authctx.ServiceIdentityGameIntegration)
	role.finalizeFailure = status.Error(codes.Unavailable, "role timeout")
	_, err = svc.RecoverCommunityOwner(serviceCtx, req)
	require.Equal(t, codes.Unavailable, status.Code(err))
	var authorityStatus string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM community_owner_authority WHERE space_id=$1`, spaceID).Scan(&authorityStatus))
	require.Equal(t, "recovery_pending", authorityStatus, "Role failure keeps the authority frozen")
	recovered, err := svc.RecoverCommunityOwner(serviceCtx, req)
	require.NoError(t, err)
	require.Equal(t, int64(2), recovered.GetOwnerGeneration())
	require.Equal(t, spaceID.String(), recovered.GetSpaceId())
	require.Equal(t, 2, role.prepareCalls)
	require.Equal(t, 2, role.finalizeCalls)
	var ownerProfileNow uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT owner_profile_id FROM spaces WHERE id=$1`, spaceID).Scan(&ownerProfileNow))
	require.Equal(t, replacementProfile, ownerProfileNow)
	var currentAccount, currentProfile uuid.UUID
	var currentGeneration int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT owner_account_id,owner_profile_id,owner_generation FROM community_owner_authority WHERE space_id=$1`, spaceID).Scan(&currentAccount, &currentProfile, &currentGeneration))
	require.Equal(t, replacementAccount, currentAccount)
	require.Equal(t, replacementProfile, currentProfile)
	require.Equal(t, int64(2), currentGeneration)
	var receiptCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM community_owner_recovery_operations WHERE operation_id=$1 AND status='succeeded' AND result_generation=2 AND octet_length(role_receipt)>0`, operationID).Scan(&receiptCount))
	require.Equal(t, 1, receiptCount)
	replay, err := (&SpaceGRPC{Store: &store.SpaceStore{Pool: pool}, OwnershipRoles: role, PrincipalIssuer: issuer}).RecoverCommunityOwner(serviceCtx, req)
	require.NoError(t, err)
	require.True(t, replay.GetReplayed())
	require.Equal(t, 2, role.prepareCalls, "durable success replay does not reapply Role")
	stale := proto.Clone(req).(*spacev1.RecoverCommunityOwnerRequest)
	stale.OperationId = uuid.NewString()
	_, err = svc.RecoverCommunityOwner(serviceCtx, stale)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "stale generation cannot start a second operation after takeover")
	untrustedCtx := ctx
	_, err = svc.RecoverCommunityOwner(untrustedCtx, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}
