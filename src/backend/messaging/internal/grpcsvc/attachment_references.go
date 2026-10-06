package grpcsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/messaging/internal/messageevents"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"
)

type AttachmentReferenceOwner interface {
	AcquireFileReferences(context.Context, *filev1.AcquireFileReferencesRequest, ...grpc.CallOption) (*filev1.AcquireFileReferencesResponse, error)
	ReleaseFileReferences(context.Context, *filev1.ReleaseFileReferencesRequest, ...grpc.CallOption) (*filev1.ReleaseFileReferencesResponse, error)
}
type AttachmentReferenceCoordinator struct {
	Files  AttachmentReferenceOwner
	Issuer *principal.Issuer
}

func (c *AttachmentReferenceCoordinator) Acquire(ctx context.Context, request *filev1.AcquireFileReferencesRequest) error {
	if c == nil || c.Files == nil || c.Issuer == nil {
		return status.Error(codes.Unavailable, "attachment reference owner unavailable")
	}
	call, cancel, err := (&SpaceFileProducerCoordinator{Issuer: c.Issuer}).signed(ctx, filev1.FileService_AcquireFileReferences_FullMethodName, request.OperationId, request)
	if err != nil {
		return err
	}
	defer cancel()
	response, err := c.Files.AcquireFileReferences(call, request)
	if err != nil {
		return err
	}
	receipt := response.GetReceipt()
	if receipt == nil || receipt.ProtocolVersion != 1 || receipt.ReceiptId == "" || receipt.OperationId != request.OperationId || receipt.ProducerId != request.ProducerId || receipt.ReferenceCount != uint64(len(request.References)) || !bytes.Equal(receipt.RequestSha256, lifecycleHash(request)) || receipt.CompletedAt == nil || receipt.CompletedAt.CheckValid() != nil {
		return status.Error(codes.DataLoss, "attachment acquisition receipt mismatch")
	}
	return nil
}
func (c *AttachmentReferenceCoordinator) Release(ctx context.Context, request *filev1.ReleaseFileReferencesRequest) error {
	if c == nil || c.Files == nil || c.Issuer == nil {
		return status.Error(codes.Unavailable, "attachment reference owner unavailable")
	}
	call, cancel, err := (&SpaceFileProducerCoordinator{Issuer: c.Issuer}).signed(ctx, filev1.FileService_ReleaseFileReferences_FullMethodName, request.OperationId, request)
	if err != nil {
		return err
	}
	defer cancel()
	response, err := c.Files.ReleaseFileReferences(call, request)
	if err != nil {
		return err
	}
	receipt := response.GetReceipt()
	if receipt == nil || receipt.ProtocolVersion != 1 || receipt.ReceiptId == "" || receipt.OperationId != request.OperationId || receipt.ProducerId != request.ProducerId || receipt.ReleasedCount > uint64(len(request.References)) || !bytes.Equal(receipt.RequestSha256, lifecycleHash(request)) || receipt.CompletedAt == nil || receipt.CompletedAt.CheckValid() != nil {
		return status.Error(codes.DataLoss, "attachment release receipt mismatch")
	}
	return nil
}

func attachmentAcquisitionRequest(raw string) (*filev1.AcquireFileReferencesRequest, error) {
	var attachments []messageAttachment
	if err := json.Unmarshal([]byte(raw), &attachments); err != nil {
		return nil, err
	}
	unique := map[uuid.UUID]bool{}
	for _, attachment := range attachments {
		if attachment.FileID == "" {
			continue
		}
		file, err := parseUUIDField("attachments.file_id", attachment.FileID)
		if err != nil {
			return nil, err
		}
		unique[file] = true
	}
	ids := make([]uuid.UUID, 0, len(unique))
	for id := range unique {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	request := &filev1.AcquireFileReferencesRequest{ProtocolVersion: 1, ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING}
	for _, id := range ids {
		request.References = append(request.References, &filev1.FileReferenceKey{FileId: id.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE})
	}
	return request, nil
}

func (s *MessagingGRPC) insertMessageWithAttachments(ctx context.Context, row store.MessageRow, events []messageevents.OutboxEvent) (*store.MessageRow, error) {
	policy, err := s.loadMutationChatPolicy(ctx, row.ChatID)
	if err != nil {
		return nil, err
	}
	request, err := attachmentAcquisitionRequest(row.AttachmentsJSON)
	if err != nil {
		return nil, err
	}
	if len(request.References) == 0 || s.AttachmentReferences == nil {
		if len(request.References) > 0 && s.SpaceFileProducer != nil {
			return nil, status.Error(codes.Unavailable, "attachment reference acquisition unavailable")
		}
		saved, _, err := s.Messages.InsertMessageWithOutbox(ctx, row, policy.SpaceID, events)
		return saved, err
	}
	saved, err := s.Messages.InsertMessageWithReferencesAndOutbox(ctx, row, policy.SpaceID, request, s.AttachmentReferences.Acquire, events)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrAttachmentIntentConflict):
			return nil, status.Error(codes.AlreadyExists, "attachment send request conflicts")
		case errors.Is(err, store.ErrAttachmentIntentExpired):
			return nil, status.Error(codes.FailedPrecondition, "attachment send intent expired")
		case errors.Is(err, store.ErrSpaceLifecycleOrder):
			return nil, status.Error(codes.FailedPrecondition, "Space is not LIVE")
		}
		if status.Code(err) != codes.Unknown {
			return nil, err
		}
		return nil, status.Error(codes.Internal, "attachment send persistence failed")
	}
	return saved, nil
}
