package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/space/internal/ownershiprecovery"
	"voice/backend/space/internal/store"
)

type ownershipRecoveryRoleClientStub struct {
	rolev1.RoleServiceClient
	prepare  func(context.Context, *rolev1.PrepareOwnershipTransferRequest) (*rolev1.PrepareOwnershipTransferResponse, error)
	finalize func(context.Context, *rolev1.FinalizeOwnershipTransferRequest) (*rolev1.FinalizeOwnershipTransferResponse, error)
	abort    func(context.Context, *rolev1.AbortOwnershipTransferRequest) (*rolev1.AbortOwnershipTransferResponse, error)
}

func (c *ownershipRecoveryRoleClientStub) PrepareOwnershipTransfer(ctx context.Context, request *rolev1.PrepareOwnershipTransferRequest, _ ...grpc.CallOption) (*rolev1.PrepareOwnershipTransferResponse, error) {
	return c.prepare(ctx, request)
}

func (c *ownershipRecoveryRoleClientStub) FinalizeOwnershipTransfer(ctx context.Context, request *rolev1.FinalizeOwnershipTransferRequest, _ ...grpc.CallOption) (*rolev1.FinalizeOwnershipTransferResponse, error) {
	return c.finalize(ctx, request)
}

func (c *ownershipRecoveryRoleClientStub) AbortOwnershipTransfer(ctx context.Context, request *rolev1.AbortOwnershipTransferRequest, _ ...grpc.CallOption) (*rolev1.AbortOwnershipTransferResponse, error) {
	return c.abort(ctx, request)
}

func TestNewOwnershipRecoveryCoordinator_MissingProtectedDependenciesFailsClosed(t *testing.T) {
	binding := store.OwnershipBinding{ProtocolVersion: 2, SpaceID: uuid.New(), AccountID: uuid.New(), ActorProfileID: uuid.New(), NewOwnerProfileID: uuid.New(), OperationID: uuid.New(), SessionEpoch: 1, ProofDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	for _, service := range []*SpaceGRPC{nil, {}, {Store: &store.SpaceStore{}}} {
		_, err := NewOwnershipRecoveryCoordinator(service).Execute(context.Background(), binding, "proof")
		require.ErrorIs(t, err, ownershiprecovery.ErrCoordinatorNotConfigured)
	}
}

func TestOwnershipRecoveryRole_UsesFreshExactSignedProtocolTwoRequests(t *testing.T) {
	issuer, key := newOwnershipTestIssuer(t)
	binding := store.OwnershipBinding{ProtocolVersion: 2, SpaceID: uuid.New(), AccountID: uuid.New(), ActorProfileID: uuid.New(), NewOwnerProfileID: uuid.New(), OperationID: uuid.New(), SessionEpoch: 1, ProofDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	assertIntent := func(intent *rolev1.OwnershipTransferIntent) {
		require.Equal(t, uint32(2), intent.GetProtocolVersion())
		require.Equal(t, binding.SpaceID.String(), intent.GetSpaceId())
		require.Equal(t, binding.ActorProfileID.String(), intent.GetOldOwnerProfileId())
		require.Equal(t, binding.NewOwnerProfileID.String(), intent.GetNewOwnerProfileId())
		require.Equal(t, binding.OperationID.String(), intent.GetOperationId())
	}
	client := &ownershipRecoveryRoleClientStub{}
	client.prepare = func(ctx context.Context, request *rolev1.PrepareOwnershipTransferRequest) (*rolev1.PrepareOwnershipTransferResponse, error) {
		assertIntent(request.GetIntent())
		verifyOwnershipCredential(t, ctx, request, rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName, key)
		return &rolev1.PrepareOwnershipTransferResponse{Receipt: &rolev1.OwnershipTransferReceipt{}}, nil
	}
	client.finalize = func(ctx context.Context, request *rolev1.FinalizeOwnershipTransferRequest) (*rolev1.FinalizeOwnershipTransferResponse, error) {
		assertIntent(request.GetIntent())
		verifyOwnershipCredential(t, ctx, request, rolev1.RoleService_FinalizeOwnershipTransfer_FullMethodName, key)
		return &rolev1.FinalizeOwnershipTransferResponse{Receipt: &rolev1.OwnershipTransferReceipt{}}, nil
	}
	client.abort = func(ctx context.Context, request *rolev1.AbortOwnershipTransferRequest) (*rolev1.AbortOwnershipTransferResponse, error) {
		assertIntent(request.GetIntent())
		verifyOwnershipCredential(t, ctx, request, rolev1.RoleService_AbortOwnershipTransfer_FullMethodName, key)
		return &rolev1.AbortOwnershipTransferResponse{Receipt: &rolev1.OwnershipTransferReceipt{}}, nil
	}
	role := ownershipRecoveryRole{service: &SpaceGRPC{PrincipalIssuer: issuer, OwnershipRoles: client}}
	_, err := role.Prepare(context.Background(), binding)
	require.NoError(t, err)
	_, err = role.Finalize(context.Background(), binding)
	require.NoError(t, err)
	_, err = role.Abort(context.Background(), binding)
	require.NoError(t, err)
}
