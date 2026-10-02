package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/store"
)

type SpaceLifecyclePurgeStore interface {
	SpaceManifestChatPage(context.Context, uuid.UUID, uuid.UUID, uint64, string, []byte, uint64, uint64) ([]uuid.UUID, error)
	CompleteSpaceLifecyclePurge(context.Context, *messagingv1.PurgeSpaceRequest, string) ([]byte, error)
}

func (s *MessagingGRPC) PurgeSpace(ctx context.Context, req *messagingv1.PurgeSpaceRequest) (*messagingv1.PurgeSpaceResponse, error) {
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 || req.GetPurge() == nil || hasAnyUnknown(req.GetPurge()) {
		return nil, status.Error(codes.InvalidArgument, "invalid Messaging Space purge request")
	}
	purge := req.GetPurge()
	if err := trustedMessagingSpaceLifecycle(ctx, req, messagingv1.MessagingService_PurgeSpace_FullMethodName); err != nil {
		return nil, err
	}
	if purge.GetProtocolVersion() != 1 || purge.GetGeneration() < 2 || purge.GetGeneration() > math.MaxInt64 ||
		purge.GetParticipantId() != commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING || purge.GetPurgeDecidedAt() == nil ||
		purge.GetPurgeDecidedAt().CheckValid() != nil || purge.GetManifest() == nil || purge.GetManifest().GetManifestId() == "" ||
		len(purge.GetManifest().GetManifestSha256()) != sha256.Size ||
		purge.GetManifest().GetItemCount() > math.MaxInt64 {
		return nil, status.Error(codes.InvalidArgument, "invalid Messaging Space purge binding")
	}
	spaceID, err := canonicalSpaceLifecycleUUID(purge.GetSpaceId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Messaging Space purge binding")
	}
	deletionID, err := canonicalSpaceLifecycleUUID(purge.GetDeletionOperationId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Messaging Space purge binding")
	}
	if s == nil || s.SpaceLifecyclePurger == nil || s.ManagedChatPurger == nil || s.SpaceFileProducer == nil {
		return nil, status.Error(codes.Unavailable, "Messaging Space purge dependencies unavailable")
	}
	manifest := purge.GetManifest()
	ctx, err = store.SpacePurgeContext(ctx, req)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "bound Space purge parent required")
	}
	ctx, err = s.SpaceFileProducer.ReleaseContext(ctx, purge)
	if err != nil {
		return nil, err
	}
	pageCount := (manifest.GetItemCount() + spacePurgeManifestPageSize - 1) / spacePurgeManifestPageSize
	for pageIndex := uint64(0); pageIndex < pageCount; pageIndex++ {
		chatIDs, err := s.SpaceLifecyclePurger.SpaceManifestChatPage(ctx, spaceID, deletionID, purge.GetGeneration()-1,
			manifest.GetManifestId(), manifest.GetManifestSha256(), manifest.GetItemCount(), pageIndex)
		if err != nil {
			if errors.Is(err, store.ErrSpaceManifestBinding) || errors.Is(err, store.ErrSpaceManifestNotSealed) {
				return nil, status.Error(codes.FailedPrecondition, "Messaging saved manifest does not match purge request")
			}
			return nil, status.Error(codes.Unavailable, "Messaging manifest is unavailable")
		}
		for _, chatID := range chatIDs {
			childOperation := spaceChatPurgeOperationID(spaceID, deletionID, chatID)
			childRequest := &messagingv1.PurgeManagedChatContentRequest{
				OperationId: childOperation.String(), ChatId: chatID.String(),
				PurgeAfter: proto.Clone(purge.GetPurgeDecidedAt()).(*timestamppb.Timestamp),
			}
			childReceipt, err := s.ManagedChatPurger.PurgeManagedChatContent(ctx, childRequest)
			if err != nil {
				if rpcStatus, ok := status.FromError(err); ok && rpcStatus.Code() == codes.FailedPrecondition {
					return nil, err
				}
				return nil, status.Errorf(codes.Unavailable, "Messaging chat purge %s is incomplete: %s", chatID, err)
			}
			if !validManagedChatPurgeReceipt(childReceipt, childRequest) {
				return nil, status.Errorf(codes.Unavailable, "Messaging chat purge %s returned invalid owner receipts", chatID)
			}
		}
	}
	requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Messaging Space purge request")
	}
	requestHash := domainSHA(string(req.ProtoReflect().Descriptor().FullName()), requestBytes)
	savedBytes, err := s.SpaceLifecyclePurger.CompleteSpaceLifecyclePurge(ctx, req, uuid.NewString())
	if err != nil {
		if errors.Is(err, store.ErrSpacePurgeReceiptBinding) {
			return nil, status.Error(codes.FailedPrecondition, "Messaging Space purge conflicts with saved evidence")
		}
		return nil, status.Error(codes.Unavailable, fmt.Sprintf("Messaging Space purge completion failed: %v", err))
	}
	response := &messagingv1.PurgeSpaceResponse{}
	if err := proto.Unmarshal(savedBytes, response); err != nil || !validSpacePurgeCompletionReceipt(response.GetReceipt(), purge, requestHash) {
		return nil, status.Error(codes.Internal, "stored Messaging Space purge receipt is invalid")
	}
	return response, nil
}

func spaceChatPurgeOperationID(spaceID, deletionID, chatID uuid.UUID) uuid.UUID {
	name := fmt.Sprintf("voice.messaging.v1.SpaceChatPurge\x00%s\x00%s\x00%s", spaceID, deletionID, chatID)
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(name))
}

func validSpacePurgeCompletionReceipt(receipt *commonv1.SpacePurgeReceipt, purge *commonv1.SpacePurgeRequest, requestHash []byte) bool {
	if receipt == nil || receipt.GetProtocolVersion() != 1 || receipt.GetSpaceId() != purge.GetSpaceId() ||
		receipt.GetDeletionOperationId() != purge.GetDeletionOperationId() || receipt.GetGeneration() != purge.GetGeneration() ||
		receipt.GetParticipantId() != commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING ||
		receipt.GetState() != commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED ||
		len(receipt.GetRequestSha256()) != sha256.Size || !bytes.Equal(receipt.GetRequestSha256(), requestHash) ||
		receipt.GetCompletedAt() == nil || receipt.GetCompletedAt().CheckValid() != nil || receipt.GetCompletedAt().AsTime().Unix() <= 0 {
		return false
	}
	id, err := uuid.Parse(receipt.GetReceiptId())
	return err == nil && id != uuid.Nil && id.String() == receipt.GetReceiptId()
}

func validManagedChatPurgeReceipt(receipt *messagingv1.PurgeManagedChatContentResponse, request *messagingv1.PurgeManagedChatContentRequest) bool {
	if receipt == nil || receipt.GetOperationId() != request.GetOperationId() || receipt.GetChatId() != request.GetChatId() ||
		receipt.GetPurgeAfter() == nil || !proto.Equal(receipt.GetPurgeAfter(), request.GetPurgeAfter()) ||
		len(receipt.GetFileReceiptSha256()) != sha256.Size || len(receipt.GetSearchReceiptSha256()) != sha256.Size ||
		receipt.GetCompletedAt() == nil || receipt.GetCompletedAt().CheckValid() != nil || receipt.GetCompletedAt().AsTime().Unix() <= 0 {
		return false
	}
	receiptID, err := uuid.Parse(receipt.GetReceiptId())
	if err != nil || receiptID == uuid.Nil || receiptID.String() != receipt.GetReceiptId() {
		return false
	}
	wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return false
	}
	// This child is the existing T33 managed-chat protocol, whose receipt uses
	// SHA-256 of deterministic request bytes. Only the parent P3 receipt uses
	// the message-name domain. Keep already committed child evidence valid.
	digest := sha256.Sum256(wire)
	return bytes.Equal(receipt.GetRequestSha256(), digest[:])
}
