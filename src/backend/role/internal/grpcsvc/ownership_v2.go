package grpcsvc

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/principalgrpc"
	"voice/backend/role/internal/store"
)

var ownershipV2SupportedMethods = []string{
	rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName,
	rolev1.RoleService_FinalizeOwnershipTransfer_FullMethodName,
	rolev1.RoleService_AbortOwnershipTransfer_FullMethodName,
}

func (s *RoleGRPC) GetOwnershipTransferCapabilities(ctx context.Context, req *rolev1.GetOwnershipTransferCapabilitiesRequest) (*rolev1.GetOwnershipTransferCapabilitiesResponse, error) {
	rpc := rolev1.RoleService_GetOwnershipTransferCapabilities_FullMethodName
	verified, ok := principal.FromContext(ctx)
	if !ok || verified.Kind != "service" || verified.Issuer != "space" || verified.Subject != "service:space" || verified.Audience != "role" || verified.RPC != rpc || verified.RequestID == "" {
		return nil, status.Error(codes.PermissionDenied, "verified space principal required")
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid capability request")
	}
	if verified.RequestHash != hash {
		return nil, status.Error(codes.PermissionDenied, "verified request binding required")
	}
	if !principalgrpc.OwnershipV2CapabilitiesActive(ctx) {
		return nil, status.Error(codes.Unavailable, "ownership v2 activation hold")
	}
	return &rolev1.GetOwnershipTransferCapabilitiesResponse{
		ProtocolVersion:  2,
		SupportedMethods: append([]string(nil), ownershipV2SupportedMethods...),
	}, nil
}

type ownershipV2Request interface {
	proto.Message
	GetIntent() *rolev1.OwnershipTransferIntent
}

func ownershipV2Input(ctx context.Context, rpc string, req ownershipV2Request) (store.OwnershipTransferV2Input, error) {
	var in store.OwnershipTransferV2Input
	intent := req.GetIntent()
	if intent == nil {
		return in, status.Error(codes.InvalidArgument, "ownership intent required")
	}
	var err error
	in.SpaceID, err = parseUUIDField("space_id", intent.SpaceId)
	if err != nil {
		return in, err
	}
	in.OldOwnerProfileID, err = parseUUIDField("old_owner_profile_id", intent.OldOwnerProfileId)
	if err != nil {
		return in, err
	}
	in.NewOwnerProfileID, err = parseUUIDField("new_owner_profile_id", intent.NewOwnerProfileId)
	if err != nil {
		return in, err
	}
	in.OperationID, err = parseUUIDField("operation_id", intent.OperationId)
	if err != nil {
		return in, err
	}
	if in.OldOwnerProfileID == in.NewOwnerProfileID {
		return in, status.Error(codes.InvalidArgument, "distinct owners required")
	}
	verified, ok := principal.FromContext(ctx)
	if !ok || verified.Kind != "service" || verified.Issuer != "space" || verified.Subject != "service:space" || verified.Audience != "role" || verified.RPC != rpc || verified.RequestID == "" {
		return in, status.Error(codes.PermissionDenied, "verified space principal required")
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return in, status.Error(codes.InvalidArgument, "invalid ownership request")
	}
	if verified.RequestHash != hash {
		return in, status.Error(codes.PermissionDenied, "verified request binding required")
	}
	in.RequestHash, err = hex.DecodeString(strings.TrimPrefix(hash, "sha256:"))
	if err != nil {
		return in, status.Error(codes.InvalidArgument, "invalid ownership request hash")
	}
	in.IntentBytes, err = (proto.MarshalOptions{Deterministic: true}).Marshal(intent)
	if err != nil {
		return in, status.Error(codes.InvalidArgument, "invalid ownership intent")
	}
	in.ProtocolVersion = intent.ProtocolVersion
	// Existing operation version mismatches must reach immutable ledger lookup.
	return in, nil
}

func ownershipV2StoreError(err error) error {
	switch {
	case errors.Is(err, store.ErrOwnershipTransferConflict):
		return status.Error(codes.AlreadyExists, "ownership transfer conflicts with recorded request")
	case errors.Is(err, store.ErrOwnershipTransferV2InvalidInput):
		return status.Error(codes.InvalidArgument, "invalid ownership transfer intent")
	case errors.Is(err, store.ErrOwnershipTransferState), errors.Is(err, store.ErrOwnershipTransferMissing), errors.Is(err, store.ErrSpaceRetired):
		return status.Error(codes.FailedPrecondition, "ownership transfer state does not match")
	default:
		return status.Error(codes.Unavailable, "ownership transfer persistence unavailable")
	}
}

func (s *RoleGRPC) ownershipV2(ctx context.Context, rpc string, req ownershipV2Request) (*rolev1.OwnershipTransferReceipt, error) {
	in, err := ownershipV2Input(ctx, rpc, req)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "ownership transfer persistence unavailable")
	}
	var receipt store.OwnershipTransferV2Receipt
	switch rpc {
	case rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName:
		receipt, err = s.Store.PrepareOwnershipTransfer(ctx, in)
	case rolev1.RoleService_FinalizeOwnershipTransfer_FullMethodName:
		receipt, err = s.Store.FinalizeOwnershipTransfer(ctx, in)
	case rolev1.RoleService_AbortOwnershipTransfer_FullMethodName:
		receipt, err = s.Store.AbortOwnershipTransfer(ctx, in)
	default:
		return nil, status.Error(codes.PermissionDenied, "ownership method denied")
	}
	if err != nil {
		return nil, ownershipV2StoreError(err)
	}
	intent := &rolev1.OwnershipTransferIntent{}
	if err := proto.Unmarshal(receipt.IntentBytes, intent); err != nil {
		return nil, status.Error(codes.Unavailable, "ownership receipt unavailable")
	}
	states := map[string]rolev1.OwnershipTransferState{
		"prepared":  rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_PREPARED,
		"finalized": rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED,
		"aborted":   rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED,
	}
	state, ok := states[receipt.State]
	if !ok {
		return nil, status.Error(codes.Unavailable, "ownership receipt unavailable")
	}
	out := &rolev1.OwnershipTransferReceipt{Intent: intent, State: state}
	if receipt.State != "prepared" {
		out.CurrentOwnerProfileId = receipt.CurrentOwnerProfileID.String()
	}
	return out, nil
}

func (s *RoleGRPC) PrepareOwnershipTransfer(ctx context.Context, req *rolev1.PrepareOwnershipTransferRequest) (*rolev1.PrepareOwnershipTransferResponse, error) {
	receipt, err := s.ownershipV2(ctx, rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName, req)
	if err != nil {
		return nil, err
	}
	return &rolev1.PrepareOwnershipTransferResponse{Receipt: receipt}, nil
}

func (s *RoleGRPC) FinalizeOwnershipTransfer(ctx context.Context, req *rolev1.FinalizeOwnershipTransferRequest) (*rolev1.FinalizeOwnershipTransferResponse, error) {
	receipt, err := s.ownershipV2(ctx, rolev1.RoleService_FinalizeOwnershipTransfer_FullMethodName, req)
	if err != nil {
		return nil, err
	}
	return &rolev1.FinalizeOwnershipTransferResponse{Receipt: receipt}, nil
}

func (s *RoleGRPC) AbortOwnershipTransfer(ctx context.Context, req *rolev1.AbortOwnershipTransferRequest) (*rolev1.AbortOwnershipTransferResponse, error) {
	receipt, err := s.ownershipV2(ctx, rolev1.RoleService_AbortOwnershipTransfer_FullMethodName, req)
	if err != nil {
		return nil, err
	}
	return &rolev1.AbortOwnershipTransferResponse{Receipt: receipt}, nil
}
