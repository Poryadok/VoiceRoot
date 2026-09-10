package grpcsvc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"voice/backend/pkg/principal"
	"voice/backend/role/internal/store"

	rolev1 "voice.app/voice/role/v1"
)

const (
	applyOwnershipTransferRPC      = "/voice.role.v1.RoleService/ApplyOwnershipTransfer"
	compensateOwnershipTransferRPC = "/voice.role.v1.RoleService/CompensateOwnershipTransfer"
)

type ownershipTransferRequest interface {
	proto.Message
	GetSpaceId() string
	GetOldOwnerProfileId() string
	GetNewOwnerProfileId() string
	GetOperationId() string
}

func ownershipTransferInput(ctx context.Context, rpc string, req ownershipTransferRequest) (store.OwnershipTransferInput, error) {
	spaceRaw, oldRaw, newRaw, operationRaw := req.GetSpaceId(), req.GetOldOwnerProfileId(), req.GetNewOwnerProfileId(), req.GetOperationId()
	spaceID, err := parseUUIDField("space_id", spaceRaw)
	if err != nil {
		return store.OwnershipTransferInput{}, err
	}
	oldOwnerID, err := parseUUIDField("old_owner_profile_id", oldRaw)
	if err != nil {
		return store.OwnershipTransferInput{}, err
	}
	newOwnerID, err := parseUUIDField("new_owner_profile_id", newRaw)
	if err != nil {
		return store.OwnershipTransferInput{}, err
	}
	operationID, err := parseUUIDField("operation_id", operationRaw)
	if err != nil {
		return store.OwnershipTransferInput{}, err
	}
	if oldOwnerID == newOwnerID {
		return store.OwnershipTransferInput{}, status.Error(codes.FailedPrecondition, "ownership transfer requires distinct owners")
	}
	verified, ok := principal.FromContext(ctx)
	if !ok || verified.Kind != "service" || verified.Issuer != "space" || verified.Subject != "service:space" || verified.Audience != "role" || verified.RPC != rpc || verified.RequestID == "" {
		return store.OwnershipTransferInput{}, status.Error(codes.PermissionDenied, "verified space principal required")
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return store.OwnershipTransferInput{}, status.Error(codes.InvalidArgument, "invalid ownership request")
	}
	if verified.RequestHash != hash {
		return store.OwnershipTransferInput{}, status.Error(codes.PermissionDenied, "verified request binding required")
	}
	return store.OwnershipTransferInput{SpaceID: spaceID, OldOwnerProfileID: oldOwnerID, NewOwnerProfileID: newOwnerID, OperationID: operationID, RequestHash: hash}, nil
}

func ownershipTransferStoreError(err error) error {
	switch {
	case errors.Is(err, store.ErrOwnershipTransferConflict):
		return status.Error(codes.AlreadyExists, "ownership transfer operation conflicts with recorded request")
	case errors.Is(err, store.ErrOwnershipTransferMissing), errors.Is(err, store.ErrOwnershipTransferState):
		return status.Error(codes.FailedPrecondition, "ownership transfer state does not match")
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func (s *RoleGRPC) ApplyOwnershipTransfer(ctx context.Context, req *rolev1.ApplyOwnershipTransferRequest) (*rolev1.ApplyOwnershipTransferResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	in, err := ownershipTransferInput(ctx, applyOwnershipTransferRPC, req)
	if err != nil {
		return nil, err
	}
	current, err := s.Store.ApplyOwnershipTransfer(ctx, in)
	if err != nil {
		return nil, ownershipTransferStoreError(err)
	}
	return &rolev1.ApplyOwnershipTransferResponse{CurrentOwnerProfileId: current.String()}, nil
}

func (s *RoleGRPC) CompensateOwnershipTransfer(ctx context.Context, req *rolev1.CompensateOwnershipTransferRequest) (*rolev1.CompensateOwnershipTransferResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	in, err := ownershipTransferInput(ctx, compensateOwnershipTransferRPC, req)
	if err != nil {
		return nil, err
	}
	current, err := s.Store.CompensateOwnershipTransfer(ctx, in)
	if err != nil {
		return nil, ownershipTransferStoreError(err)
	}
	return &rolev1.CompensateOwnershipTransferResponse{CurrentOwnerProfileId: current.String()}, nil
}
