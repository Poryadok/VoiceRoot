package grpcsvc

import (
	"context"

	rolev1 "voice.app/voice/role/v1"
	"voice/backend/space/internal/ownershiprecovery"
	"voice/backend/space/internal/store"
)

// NewOwnershipRecoveryCoordinator binds the already-private principal clients
// to the durable Space journal. It is deliberately not connected to the public
// TransferOwnership RPC; activation belongs to the later Gateway vertical.
func NewOwnershipRecoveryCoordinator(service *SpaceGRPC) *ownershiprecovery.Coordinator {
	return ownershiprecovery.NewCoordinator(ownershiprecovery.Dependencies{
		Store: service.Store,
		Auth:  ownershipRecoveryAuth{service: service},
		Role:  ownershipRecoveryRole{service: service},
	})
}

type ownershipRecoveryAuth struct{ service *SpaceGRPC }

func (a ownershipRecoveryAuth) Consume(ctx context.Context, binding store.OwnershipBinding, proof string) (store.OwnershipAuthReceipt, error) {
	return a.service.consumeOwnershipAuthProof(ctx, binding, proof)
}

func (a ownershipRecoveryAuth) Lookup(ctx context.Context, binding store.OwnershipBinding) (store.OwnershipAuthReceipt, error) {
	return a.service.lookupOwnershipAuthReceipt(ctx, binding)
}

type ownershipRecoveryRole struct{ service *SpaceGRPC }

func (r ownershipRecoveryRole) Prepare(ctx context.Context, binding store.OwnershipBinding) (*rolev1.OwnershipTransferReceipt, error) {
	req := &rolev1.PrepareOwnershipTransferRequest{Intent: ownershipIntent(binding)}
	signed, err := r.service.ownershipRoleContext(ctx, req, rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName, binding.OperationID)
	if err != nil {
		return nil, err
	}
	response, err := r.service.OwnershipRoles.PrepareOwnershipTransfer(signed, req)
	if err != nil {
		return nil, err
	}
	return response.GetReceipt(), nil
}

func (r ownershipRecoveryRole) Finalize(ctx context.Context, binding store.OwnershipBinding) (*rolev1.OwnershipTransferReceipt, error) {
	req := &rolev1.FinalizeOwnershipTransferRequest{Intent: ownershipIntent(binding)}
	signed, err := r.service.ownershipRoleContext(ctx, req, rolev1.RoleService_FinalizeOwnershipTransfer_FullMethodName, binding.OperationID)
	if err != nil {
		return nil, err
	}
	response, err := r.service.OwnershipRoles.FinalizeOwnershipTransfer(signed, req)
	if err != nil {
		return nil, err
	}
	return response.GetReceipt(), nil
}

func (r ownershipRecoveryRole) Abort(ctx context.Context, binding store.OwnershipBinding) (*rolev1.OwnershipTransferReceipt, error) {
	req := &rolev1.AbortOwnershipTransferRequest{Intent: ownershipIntent(binding)}
	signed, err := r.service.ownershipRoleContext(ctx, req, rolev1.RoleService_AbortOwnershipTransfer_FullMethodName, binding.OperationID)
	if err != nil {
		return nil, err
	}
	response, err := r.service.OwnershipRoles.AbortOwnershipTransfer(signed, req)
	if err != nil {
		return nil, err
	}
	return response.GetReceipt(), nil
}

func ownershipIntent(binding store.OwnershipBinding) *rolev1.OwnershipTransferIntent {
	return &rolev1.OwnershipTransferIntent{ProtocolVersion: 2, SpaceId: binding.SpaceID.String(), OldOwnerProfileId: binding.ActorProfileID.String(), NewOwnerProfileId: binding.NewOwnerProfileID.String(), OperationId: binding.OperationID.String()}
}
