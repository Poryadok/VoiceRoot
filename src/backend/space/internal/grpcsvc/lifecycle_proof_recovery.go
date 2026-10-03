package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"time"
	authv1 "voice.app/voice/auth/v1"
)

func (s *SpaceGRPC) RecoverLifecycleProof(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if s == nil || s.Store == nil || s.OwnershipAuth == nil {
		return status.Error(codes.Unavailable, "deletion proof recovery unavailable")
	}
	saved, err := s.Store.LifecycleDeletionProofRecorded(ctx, id)
	if err != nil || saved {
		return err
	}
	request, err := s.Store.LifecycleProofLookup(ctx, id)
	if err != nil {
		return err
	}
	signed, err := s.ownershipAuthContext(ctx, request, authv1.AuthService_GetSpaceDeletionProofReceipt_FullMethodName, uuid.MustParse(request.OperationId))
	if err != nil {
		return err
	}
	response, err := s.OwnershipAuth.GetSpaceDeletionProofReceipt(signed, request)
	if err != nil {
		return err
	}
	if response == nil || response.Receipt == nil {
		return status.Error(codes.Unavailable, "missing deletion proof receipt")
	}
	raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(&authv1.ConsumeSpaceDeletionProofResponse{Receipt: response.Receipt})
	if err != nil {
		return err
	}
	wrapped := protowire.AppendBytes(protowire.AppendTag(nil, 1, protowire.BytesType), raw)
	_, err = s.Store.RecordLifecycleDeletionProofReceipt(ctx, uuid.MustParse(request.ProfileId), uuid.MustParse(request.OperationId), wrapped, lifecycleEvidenceHash("voice.auth.v1.ConsumeSpaceDeletionProofResponse", wrapped))
	return err
}
