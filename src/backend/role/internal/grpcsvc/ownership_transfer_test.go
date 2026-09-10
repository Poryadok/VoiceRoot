package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"voice/backend/pkg/principal"
	"voice/backend/role/internal/store"
	"voice/backend/role/permissions"

	rolev1 "voice.app/voice/role/v1"
)

func trustedOwnershipTransferContext(ctx context.Context, rpc, spaceID, oldOwnerID, newOwnerID, operationID string) context.Context {
	var req proto.Message = &rolev1.ApplyOwnershipTransferRequest{SpaceId: spaceID, OldOwnerProfileId: oldOwnerID, NewOwnerProfileId: newOwnerID, OperationId: operationID}
	if rpc == compensateOwnershipTransferRPC {
		req = &rolev1.CompensateOwnershipTransferRequest{SpaceId: spaceID, OldOwnerProfileId: oldOwnerID, NewOwnerProfileId: newOwnerID, OperationId: operationID}
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		panic(err)
	}
	return principal.WithVerified(ctx, principal.Principal{
		Kind:        "service",
		Issuer:      "space",
		Subject:     "service:space",
		Audience:    "role",
		RPC:         rpc,
		RequestID:   "test-request",
		RequestHash: hash,
	})
}

func ownerRoleNames(t *testing.T, s *store.RoleStore, spaceID, profileID uuid.UUID) []string {
	t.Helper()
	roles, err := s.GetMemberRoles(context.Background(), spaceID, profileID)
	require.NoError(t, err)
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.Name)
	}
	return names
}

func TestApplyOwnershipTransfer_RequiresVerifiedSpacePrincipal(t *testing.T) {
	spaceID, oldOwnerID, newOwnerID := uuid.New(), uuid.New(), uuid.New()
	_, err := ownershipTransferInput(context.Background(), applyOwnershipTransferRPC, &rolev1.ApplyOwnershipTransferRequest{SpaceId: spaceID.String(), OldOwnerProfileId: oldOwnerID.String(), NewOwnerProfileId: newOwnerID.String(), OperationId: uuid.NewString()})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestOwnershipTransferInput_RequiresExactVerifiedBinding(t *testing.T) {
	spaceID, oldOwnerID, newOwnerID, operationID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	ctx := trustedOwnershipTransferContext(context.Background(), applyOwnershipTransferRPC, spaceID, oldOwnerID, newOwnerID, operationID)
	_, err := ownershipTransferInput(ctx, applyOwnershipTransferRPC, &rolev1.ApplyOwnershipTransferRequest{SpaceId: spaceID, OldOwnerProfileId: oldOwnerID, NewOwnerProfileId: newOwnerID, OperationId: operationID})
	require.NoError(t, err)

	wrongRPC := trustedOwnershipTransferContext(context.Background(), compensateOwnershipTransferRPC, spaceID, oldOwnerID, newOwnerID, operationID)
	_, err = ownershipTransferInput(wrongRPC, applyOwnershipTransferRPC, &rolev1.ApplyOwnershipTransferRequest{SpaceId: spaceID, OldOwnerProfileId: oldOwnerID, NewOwnerProfileId: newOwnerID, OperationId: operationID})
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	wrongBinding := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "role", RPC: applyOwnershipTransferRPC, RequestID: "request", RequestHash: "sha256:not-the-request"})
	_, err = ownershipTransferInput(wrongBinding, applyOwnershipTransferRPC, &rolev1.ApplyOwnershipTransferRequest{SpaceId: spaceID, OldOwnerProfileId: oldOwnerID, NewOwnerProfileId: newOwnerID, OperationId: operationID})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestOwnershipTransferLifecycle_AppliesReplaysConflictsAndCompensates(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	spaceID, oldOwnerID, newOwnerID := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(context.Background(), spaceID, oldOwnerID))
	svc := &RoleGRPC{Store: s}
	opID := uuid.NewString()
	apply := &rolev1.ApplyOwnershipTransferRequest{SpaceId: spaceID.String(), OldOwnerProfileId: oldOwnerID.String(), NewOwnerProfileId: newOwnerID.String(), OperationId: opID}
	applyCtx := trustedOwnershipTransferContext(context.Background(), "/voice.role.v1.RoleService/ApplyOwnershipTransfer", apply.SpaceId, apply.OldOwnerProfileId, apply.NewOwnerProfileId, apply.OperationId)

	resp, err := svc.ApplyOwnershipTransfer(applyCtx, apply)
	require.NoError(t, err)
	require.Equal(t, newOwnerID.String(), resp.GetCurrentOwnerProfileId())
	require.NotContains(t, ownerRoleNames(t, s, spaceID, oldOwnerID), permissions.RoleOwner)
	require.Contains(t, ownerRoleNames(t, s, spaceID, newOwnerID), permissions.RoleOwner)

	resp, err = svc.ApplyOwnershipTransfer(applyCtx, apply)
	require.NoError(t, err)
	require.Equal(t, newOwnerID.String(), resp.GetCurrentOwnerProfileId())

	changed := &rolev1.ApplyOwnershipTransferRequest{SpaceId: spaceID.String(), OldOwnerProfileId: oldOwnerID.String(), NewOwnerProfileId: uuid.NewString(), OperationId: opID}
	changedCtx := trustedOwnershipTransferContext(context.Background(), "/voice.role.v1.RoleService/ApplyOwnershipTransfer", changed.SpaceId, changed.OldOwnerProfileId, changed.NewOwnerProfileId, changed.OperationId)
	_, err = svc.ApplyOwnershipTransfer(changedCtx, changed)
	require.Equal(t, codes.AlreadyExists, status.Code(err))

	compensate := &rolev1.CompensateOwnershipTransferRequest{SpaceId: spaceID.String(), OldOwnerProfileId: oldOwnerID.String(), NewOwnerProfileId: newOwnerID.String(), OperationId: opID}
	compensateCtx := trustedOwnershipTransferContext(context.Background(), "/voice.role.v1.RoleService/CompensateOwnershipTransfer", compensate.SpaceId, compensate.OldOwnerProfileId, compensate.NewOwnerProfileId, compensate.OperationId)
	compensateResp, err := svc.CompensateOwnershipTransfer(compensateCtx, compensate)
	require.NoError(t, err)
	require.Equal(t, oldOwnerID.String(), compensateResp.GetCurrentOwnerProfileId())
	require.Contains(t, ownerRoleNames(t, s, spaceID, oldOwnerID), permissions.RoleOwner)
	require.NotContains(t, ownerRoleNames(t, s, spaceID, newOwnerID), permissions.RoleOwner)

	compensateResp, err = svc.CompensateOwnershipTransfer(compensateCtx, compensate)
	require.NoError(t, err)
	require.Equal(t, oldOwnerID.String(), compensateResp.GetCurrentOwnerProfileId())
}
