package store

import (
	"context"
	"errors"
	"math"

	"github.com/google/uuid"
	chatv1 "voice.app/voice/chat/v1"
)

// ReadSpaceReportAttachment observes a saved native binding in one snapshot.
// Absence, a different current binding or missing history never proves retirement.
// No lifecycle fence, permission or destination Space is exposed by this read.
func (s *DMStore) ReadSpaceReportAttachment(ctx context.Context, chatID, spaceID uuid.UUID, saved uint64) (*chatv1.ReadSpaceReportAttachmentResponse, error) {
	if s == nil || s.Pool == nil || chatID == uuid.Nil || spaceID == uuid.Nil || saved == 0 || saved > math.MaxInt64 {
		return nil, errors.New("Chat attachment authority unavailable")
	}
	var reason, event string
	var revision int64
	var unchanged bool
	err := s.Pool.QueryRow(ctx, `SELECT COALESCE(r.reason,''),COALESCE(r.owner_event_id::text,''),COALESCE(r.owner_revision,0),
        EXISTS(SELECT 1 FROM chats c JOIN chat_scope_authority_heads h ON h.chat_id=c.id
          WHERE c.id=$1 AND c.space_id=$2 AND c.report_attachment_generation=$3
            AND h.space_id=c.space_id AND h.present AND h.revision>0
            AND h.scope_incarnation=c.scope_incarnation AND h.report_attachment_generation=c.report_attachment_generation)
        FROM (SELECT 1) seed LEFT JOIN chat_report_attachment_retirements r
          ON r.chat_id=$1 AND r.space_id=$2 AND r.attachment_generation=$3`, chatID, spaceID, int64(saved)).Scan(&reason, &event, &revision, &unchanged)
	if err != nil {
		return nil, err
	}
	response := &chatv1.ReadSpaceReportAttachmentResponse{ProtocolVersion: 1, ChatId: chatID.String(), ExpectedSpaceId: spaceID.String(), SavedAttachmentGeneration: saved,
		Disposition: chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_UNKNOWN}
	if reason == "" {
		if unchanged {
			response.Disposition = chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_UNCHANGED
		}
		return response, nil
	}
	id, parseErr := uuid.Parse(event)
	if unchanged || revision <= 0 || parseErr != nil || id == uuid.Nil || id.String() != event {
		return nil, errors.New("Chat attachment authority inconsistent")
	}
	switch reason {
	case "TARGET_DELETED":
		response.Reason = chatv1.SpaceReportAttachmentRetirementReason_SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_TARGET_DELETED
	case "TARGET_MOVED":
		response.Reason = chatv1.SpaceReportAttachmentRetirementReason_SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_TARGET_MOVED
	default:
		return nil, errors.New("Chat attachment authority inconsistent")
	}
	response.Disposition = chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_RETIRED
	response.OwnerEventId, response.OwnerRevision = event, uint64(revision)
	return response, nil
}
