package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"math"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/store"
)

func canonicalRetirementUUID(field, raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil || id.String() != raw {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "%s must be a canonical non-zero UUID", field)
	}
	return id, nil
}

func retirementRequestInput(ctx context.Context, req *rolev1.RetireSpaceRequest) (store.SpaceRetirementInput, error) {
	var in store.SpaceRetirementInput
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 || req.GetProtocolVersion() != 1 || req.GetGeneration() == 0 || req.GetGeneration() > math.MaxInt64 {
		return in, status.Error(codes.InvalidArgument, "invalid retirement request")
	}
	var err error
	in.SpaceID, err = canonicalRetirementUUID("space_id", req.GetSpaceId())
	if err != nil {
		return in, err
	}
	in.DeletionOperationID, err = canonicalRetirementUUID("deletion_operation_id", req.GetDeletionOperationId())
	if err != nil {
		return in, err
	}
	if req.GetPurgeDecidedAt() == nil || req.GetPurgeDecidedAt().CheckValid() != nil || req.GetPurgeDecidedAt().GetNanos()%1000 != 0 {
		return in, status.Error(codes.InvalidArgument, "purge_decided_at must be a valid microsecond timestamp")
	}
	manifest := req.GetManifest()
	if manifest == nil || len(manifest.ProtoReflect().GetUnknown()) != 0 || len(manifest.GetManifestSha256()) != sha256.Size {
		return in, status.Error(codes.InvalidArgument, "invalid manifest binding")
	}
	manifestID, err := canonicalRetirementUUID("manifest_id", manifest.GetManifestId())
	if err != nil {
		return in, err
	}
	verified, ok := principal.FromContext(ctx)
	rpc := rolev1.RoleService_RetireSpace_FullMethodName
	if !ok || verified.Kind != "service" || verified.Issuer != "space" || verified.Subject != "service:space" || verified.Audience != "role" || verified.RPC != rpc || verified.RequestID == "" {
		return in, status.Error(codes.PermissionDenied, "verified space principal required")
	}
	principalHash, err := principal.RequestHash(req)
	if err != nil {
		return in, status.Error(codes.InvalidArgument, "invalid retirement request")
	}
	if verified.RequestHash != principalHash {
		return in, status.Error(codes.PermissionDenied, "verified request binding required")
	}
	requestBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(req)
	if err != nil {
		return in, status.Error(codes.InvalidArgument, "invalid retirement request")
	}
	payload := make([]byte, 0, len("voice.role.v1.RetireSpaceRequest")+1+len(requestBytes))
	payload = append(payload, "voice.role.v1.RetireSpaceRequest"...)
	payload = append(payload, 0)
	payload = append(payload, requestBytes...)
	requestDigest := sha256.Sum256(payload)
	in.ProtocolVersion = req.GetProtocolVersion()
	in.Generation = req.GetGeneration()
	in.PurgeDecidedAt = req.GetPurgeDecidedAt().AsTime().UTC()
	in.ManifestID = manifestID
	in.ManifestItemCount = manifest.GetItemCount()
	in.ManifestSHA256 = append([]byte(nil), manifest.GetManifestSha256()...)
	in.RequestSHA256 = append([]byte(nil), requestDigest[:]...)
	in.RequestBytes = requestBytes
	return in, nil
}

func retirementResponse(receipt store.SpaceRetirementReceipt) *rolev1.RetireSpaceResponse {
	return &rolev1.RetireSpaceResponse{Receipt: &rolev1.RetireSpaceReceipt{
		ProtocolVersion: receipt.ProtocolVersion, ReceiptId: receipt.ReceiptID.String(), SpaceId: receipt.SpaceID.String(),
		DeletionOperationId: receipt.DeletionOperationID.String(), Generation: receipt.Generation,
		State:         rolev1.RoleRetirementState_ROLE_RETIREMENT_STATE_RETIRED,
		RequestSha256: append([]byte(nil), receipt.RequestSHA256...), ManifestSha256: append([]byte(nil), receipt.ManifestSHA256...),
		RetiredAt: timestamppb.New(receipt.RetiredAt),
	}}
}

func retirementStoreError(err error) error {
	switch {
	case errors.Is(err, store.ErrSpaceRetirementInvalidInput):
		return status.Error(codes.InvalidArgument, "invalid retirement request")
	case errors.Is(err, store.ErrSpaceFrozen), errors.Is(err, store.ErrSpaceRetirementConflict), errors.Is(err, store.ErrSpaceRetired):
		return status.Error(codes.FailedPrecondition, "space retirement state does not match")
	default:
		return status.Error(codes.Unavailable, "space retirement persistence unavailable")
	}
}

func validRetirementResponse(response *rolev1.RetireSpaceResponse, stored store.SpaceRetirementReceipt) bool {
	receipt := response.GetReceipt()
	return receipt != nil && receipt.GetProtocolVersion() == stored.ProtocolVersion &&
		receipt.GetReceiptId() == stored.ReceiptID.String() && receipt.GetSpaceId() == stored.SpaceID.String() &&
		receipt.GetDeletionOperationId() == stored.DeletionOperationID.String() && receipt.GetGeneration() == stored.Generation &&
		receipt.GetState() == rolev1.RoleRetirementState_ROLE_RETIREMENT_STATE_RETIRED &&
		bytes.Equal(receipt.GetRequestSha256(), stored.RequestSHA256) && bytes.Equal(receipt.GetManifestSha256(), stored.ManifestSHA256) &&
		receipt.GetRetiredAt() != nil && receipt.GetRetiredAt().CheckValid() == nil && receipt.GetRetiredAt().AsTime().Equal(stored.RetiredAt)
}

func (s *RoleGRPC) RetireSpace(ctx context.Context, req *rolev1.RetireSpaceRequest) (*rolev1.RetireSpaceResponse, error) {
	in, err := retirementRequestInput(ctx, req)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil || s.Store.Pool == nil {
		return nil, status.Error(codes.Unavailable, "space retirement persistence unavailable")
	}
	receipt, err := s.Store.RetireSpace(ctx, in, func(fields store.SpaceRetirementReceipt) ([]byte, error) {
		return (proto.MarshalOptions{Deterministic: true}).Marshal(retirementResponse(fields))
	})
	if err != nil {
		return nil, retirementStoreError(err)
	}
	var response rolev1.RetireSpaceResponse
	if err := proto.Unmarshal(receipt.ReceiptBytes, &response); err != nil || !validRetirementResponse(&response, receipt) {
		return nil, status.Error(codes.Unavailable, "space retirement receipt unavailable")
	}
	return &response, nil
}
