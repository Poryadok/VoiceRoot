package grpcsvc

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/filepreview"
)

type attachmentPreviewOwner interface {
	ResolveMessageAttachmentPreview(context.Context, *filev1.ResolveMessageAttachmentPreviewRequest, ...grpc.CallOption) (*filev1.ResolveMessageAttachmentPreviewResponse, error)
}

func (c *AttachmentReferenceCoordinator) PreparePreview(ctx context.Context, row *store.MessageRow, acquired *filev1.AcquireFileReferencesRequest) error {
	return c.preparePreviews(ctx, row, acquired, true)
}

func (c *AttachmentReferenceCoordinator) preparePreviews(ctx context.Context, row *store.MessageRow, acquired *filev1.AcquireFileReferencesRequest, readyOnly bool) error {
	if row.IsE2E || row.ContentType == "music" {
		return nil
	}
	owner, ok := c.Files.(attachmentPreviewOwner)
	if !ok || c.Issuer == nil {
		return status.Error(codes.Unavailable, "attachment preview owner unavailable")
	}
	snapshots := map[string]*eventsv1.FilePreviewSnapshot{}
	audio := map[string]*filev1.AudioNoGenericPreview{}
	refs := map[string]*filev1.FileReferenceKey{}
	for _, ref := range acquired.References {
		request := &filev1.ResolveMessageAttachmentPreviewRequest{ProtocolVersion: 1, Reference: ref}
		call, cancel, err := (&SpaceFileProducerCoordinator{Issuer: c.Issuer}).signed(ctx, filev1.FileService_ResolveMessageAttachmentPreview_FullMethodName, uuid.NewString(), request)
		if err != nil {
			return err
		}
		response, err := owner.ResolveMessageAttachmentPreview(call, request)
		cancel()
		if err != nil {
			return err
		}
		if response == nil || ref == nil || !proto.Equal(response.GetReference(), ref) || (response.GetSnapshot() == nil) == (response.GetAudioNoGenericPreview() == nil) {
			return status.Error(codes.DataLoss, "attachment preview response mismatch")
		}
		state := ""
		if snapshot := response.GetSnapshot(); snapshot != nil {
			if filepreview.Validate(snapshot) != nil || snapshot.GetFileId() != ref.FileId {
				return status.Error(codes.DataLoss, "attachment preview response mismatch")
			}
			snapshots[ref.FileId] = snapshot
			state = snapshot.Status
		} else {
			result := response.GetAudioNoGenericPreview()
			id, parseErr := uuid.Parse(result.GetFileId())
			if parseErr != nil || id == uuid.Nil || id.String() != result.GetFileId() || result.GetFileId() != ref.FileId || result.GetProtocolVersion() != 1 || len(result.ProtoReflect().GetUnknown()) != 0 {
				return status.Error(codes.DataLoss, "attachment preview response mismatch")
			}
			switch result.GetStatus() {
			case "pending_upload", "processing", "ready", "failed", "deleted", "expired":
			default:
				return status.Error(codes.DataLoss, "attachment preview response mismatch")
			}
			audio[ref.FileId] = result
			state = result.Status
		}
		if readyOnly && state != "ready" {
			return status.Error(codes.FailedPrecondition, "attachment file is not ready")
		}
		refs[ref.FileId] = ref
	}
	var attachments []messageAttachment
	if err := json.Unmarshal([]byte(row.AttachmentsJSON), &attachments); err != nil {
		return err
	}
	row.FilePreviewBindings = []store.FilePreviewBinding{}
	for i, attachment := range attachments {
		if attachment.FileID == "" {
			continue
		}
		if !filepreview.AttachmentKind(attachment.Type) {
			return status.Error(codes.FailedPrecondition, "attachment kind has no file preview projection")
		}
		snapshot := snapshots[attachment.FileID]
		ref := refs[attachment.FileID]
		if result := audio[attachment.FileID]; result != nil {
			if ref == nil || (result.Status == "ready" && !attachmentTypeMatchesFileMeta(attachment.Type, "audio")) {
				return status.Error(codes.FailedPrecondition, "attachment preview type unavailable")
			}
			continue
		}
		if snapshot == nil || ref == nil || (snapshot.Status == "ready" && !attachmentTypeMatchesFileMeta(attachment.Type, snapshot.FileType)) {
			return status.Error(codes.FailedPrecondition, "attachment preview type unavailable")
		}
		row.FilePreviewBindings = append(row.FilePreviewBindings, store.FilePreviewBinding{Slot: i, FileID: attachment.FileID, Kind: attachment.Type, Reference: proto.Clone(ref).(*filev1.FileReferenceKey), Snapshot: proto.Clone(snapshot).(*eventsv1.FilePreviewSnapshot)})
	}
	return nil
}

type forwardFileSelectorKey struct{}
type forwardFileSelector struct {
	message uuid.UUID
	space   *uuid.UUID
}

// The caller sets this only after authorizing the persisted source message.
func fileMetadataRequest(ctx context.Context, ids []string) *filev1.GetBulkMetadataRequest {
	selector, ok := ctx.Value(forwardFileSelectorKey{}).(forwardFileSelector)
	if !ok {
		return &filev1.GetBulkMetadataRequest{FileIds: ids}
	}
	request := &filev1.GetBulkMetadataRequest{}
	for _, id := range ids {
		ref := &filev1.FileReferenceKey{FileId: id, OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, OwnerId: selector.message.String()}
		if selector.space != nil {
			v := selector.space.String()
			ref.ScopeSpaceId = &v
		}
		request.Items = append(request.Items, &filev1.FileAccessItem{FileId: id, Access: &filev1.FileAccessSelector{Selector: &filev1.FileAccessSelector_Reference{Reference: ref}}})
	}
	return request
}
