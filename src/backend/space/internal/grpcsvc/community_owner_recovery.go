package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"
)

func (s *SpaceGRPC) RecoverCommunityOwner(ctx context.Context, req *spacev1.RecoverCommunityOwnerRequest) (*spacev1.RecoverCommunityOwnerResponse, error) {
	identity, ok := authctx.VerifiedServiceIdentity(ctx)
	if !ok || identity != authctx.ServiceIdentityGameIntegration {
		return nil, status.Error(codes.Unauthenticated, "verified GIS service identity required")
	}
	if s == nil || s.Store == nil || s.OwnershipRoles == nil || s.PrincipalIssuer == nil {
		return nil, status.Error(codes.Unavailable, "Space owner recovery authority unavailable")
	}
	parse := func(field, raw string) (uuid.UUID, error) { return parseUUIDField(field, raw) }
	var in store.CommunityOwnerRecoveryInput
	var err error
	if in.OperationID, err = parse("operation_id", req.GetOperationId()); err != nil {
		return nil, err
	}
	if in.ApplicationID, err = parse("application_id", req.GetApplicationId()); err != nil {
		return nil, err
	}
	if in.EnvironmentID, err = parse("environment_id", req.GetEnvironmentId()); err != nil {
		return nil, err
	}
	if in.SpaceID, err = parse("space_id", req.GetSpaceId()); err != nil {
		return nil, err
	}
	if in.PreviousOwnerAccountID, err = parse("previous_owner_account_id", req.GetPreviousOwnerAccountId()); err != nil {
		return nil, err
	}
	if in.PreviousOwnerProfileID, err = parse("previous_owner_profile_id", req.GetPreviousOwnerProfileId()); err != nil {
		return nil, err
	}
	if in.ReplacementAccountID, err = parse("replacement_account_id", req.GetReplacementAccountId()); err != nil {
		return nil, err
	}
	if in.ReplacementProfileID, err = parse("replacement_profile_id", req.GetReplacementProfileId()); err != nil {
		return nil, err
	}
	in.CorporationKey, in.ExpectedGeneration, in.ReasonCode, in.EvidenceSHA256 = req.GetCorporationKey(), req.GetExpectedGeneration(), req.GetReasonCode(), append([]byte(nil), req.GetEvidenceSha256()...)
	if in.ExpectedGeneration <= 0 {
		return nil, status.Error(codes.InvalidArgument, "positive owner generation required")
	}
	op, created, err := s.Store.ReserveCommunityOwnerRecovery(ctx, in)
	if err != nil {
		if errors.Is(err, store.ErrCommunityOwnerRecoveryConflict) {
			return nil, status.Error(codes.FailedPrecondition, "community owner generation or operation conflicts")
		}
		return nil, status.Error(codes.Unavailable, "community owner recovery persistence unavailable")
	}
	if op.Status == "succeeded" {
		return &spacev1.RecoverCommunityOwnerResponse{SpaceId: in.SpaceID.String(), OwnerGeneration: op.ResultGeneration, Replayed: true}, nil
	}
	intent := &rolev1.OwnershipTransferIntent{ProtocolVersion: 2, SpaceId: in.SpaceID.String(), OldOwnerProfileId: in.PreviousOwnerProfileID.String(), NewOwnerProfileId: in.ReplacementProfileID.String(), OperationId: in.OperationID.String()}
	prepare := &rolev1.PrepareOwnershipTransferRequest{Intent: intent}
	prepareCtx, err := s.ownershipRoleContext(ctx, prepare, rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName, in.OperationID)
	if err != nil {
		return nil, err
	}
	prepared, err := s.OwnershipRoles.PrepareOwnershipTransfer(prepareCtx, prepare)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Role owner recovery prepare unavailable")
	}
	preparedReceipt := prepared.GetReceipt()
	if preparedReceipt == nil || !proto.Equal(preparedReceipt.GetIntent(), intent) || (preparedReceipt.GetState() != rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_PREPARED && preparedReceipt.GetState() != rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED) {
		return nil, status.Error(codes.FailedPrecondition, "Role owner recovery prepare receipt mismatch")
	}
	finalize := &rolev1.FinalizeOwnershipTransferRequest{Intent: intent}
	finalizeCtx, err := s.ownershipRoleContext(ctx, finalize, rolev1.RoleService_FinalizeOwnershipTransfer_FullMethodName, in.OperationID)
	if err != nil {
		return nil, err
	}
	finalized, err := s.OwnershipRoles.FinalizeOwnershipTransfer(finalizeCtx, finalize)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Role owner recovery finalize unavailable")
	}
	roleReceipt := finalized.GetReceipt()
	if roleReceipt == nil || !proto.Equal(roleReceipt.GetIntent(), intent) || roleReceipt.GetState() != rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED || roleReceipt.GetCurrentOwnerProfileId() != in.ReplacementProfileID.String() {
		return nil, status.Error(codes.FailedPrecondition, "Role owner recovery finalize receipt mismatch")
	}
	receiptBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(roleReceipt)
	if err != nil {
		return nil, status.Error(codes.Internal, "Role owner recovery receipt encoding failed")
	}
	op, err = s.Store.CompleteCommunityOwnerRecovery(ctx, in, receiptBytes)
	if err != nil {
		if errors.Is(err, store.ErrCommunityOwnerRecoveryConflict) {
			return nil, status.Error(codes.FailedPrecondition, "community owner generation changed")
		}
		return nil, status.Error(codes.Unavailable, "community owner recovery receipt unavailable")
	}
	_ = created
	return &spacev1.RecoverCommunityOwnerResponse{SpaceId: in.SpaceID.String(), OwnerGeneration: op.ResultGeneration, Replayed: !created}, nil
}
