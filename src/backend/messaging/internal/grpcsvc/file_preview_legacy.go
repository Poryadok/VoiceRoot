package grpcsvc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/messaging/internal/store"
)

// ReconcileLegacyFilePreviews is a bounded owner migration, not a viewer read.
// Failures remain durable per row; subsequent watermark passes retry them.
func (s *MessagingGRPC) ReconcileLegacyFilePreviews(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if s.AttachmentReferences == nil {
		return status.Error(codes.Unavailable, "attachment preview recovery unavailable")
	}
	rows, err := s.Messages.LegacyPreviewRows(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		policy, err := s.loadMutationChatPolicy(ctx, row.ChatID)
		if err != nil {
			_ = s.Messages.RecordLegacyPreviewError(ctx, row.ID, "chat_policy")
			continue
		}
		request, err := attachmentAcquisitionRequest(row.AttachmentsJSON)
		if err != nil {
			_ = s.Messages.RecordLegacyPreviewError(ctx, row.ID, "attachment_shape")
			continue
		}
		if len(request.References) == 0 {
			continue
		}
		request.OperationId = uuid.NewSHA1(row.ID, []byte("voice.message.preview.backfill.acquire.v1")).String()
		for _, ref := range request.References {
			ref.OwnerId = row.ID.String()
			if policy.SpaceID != nil {
				v := policy.SpaceID.String()
				ref.ScopeSpaceId = &v
			}
		}
		err = s.AttachmentReferences.preparePreviews(ctx, &row, request, false)
		if status.Code(err) == codes.FailedPrecondition {
			// Only canonical persisted owner rows may materialize a missing exact ref.
			// Durable intent precedes RPC and local commit rechecks live message/fences.
			err = s.Messages.RepairLegacyFilePreviews(ctx, row, policy.SpaceID, request, func(call context.Context, prepared *store.MessageRow, refs *filev1.AcquireFileReferencesRequest) error {
				if e := s.AttachmentReferences.Acquire(call, refs); e != nil {
					return e
				}
				return s.AttachmentReferences.preparePreviews(call, prepared, refs, false)
			})
			if errors.Is(err, store.ErrAttachmentIntentConflict) {
				_ = s.Messages.RecordLegacyPreviewError(ctx, row.ID, "existing_owner_intent_requires_reference_repair")
				continue
			}
			if err == nil {
				continue
			}
		}
		if err != nil {
			_ = s.Messages.RecordLegacyPreviewError(ctx, row.ID, "exact_reference_or_snapshot")
			continue
		}
		if err = s.Messages.BindLegacyFilePreviews(ctx, row, policy.SpaceID); err != nil {
			_ = s.Messages.RecordLegacyPreviewError(ctx, row.ID, "mutation_fence_or_commit")
		}
	}
	return nil
}
