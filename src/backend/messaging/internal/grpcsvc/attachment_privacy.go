package grpcsvc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/guestguard"
	"voice/backend/pkg/privacy"
)

func attachmentRequiresVoicePrivacy(attType string) bool {
	switch strings.TrimSpace(attType) {
	case "voice_message":
		return true
	default:
		return false
	}
}

func (s *MessagingGRPC) checkAttachmentPrivacyForSend(ctx context.Context, chatID, senderProfileID uuid.UUID, attachmentsJSON string) error {
	if s.Privacy == nil || s.ChatGuard == nil {
		return nil
	}
	var attachments []messageAttachment
	if err := json.Unmarshal([]byte(attachmentsJSON), &attachments); err != nil || len(attachments) == 0 {
		return nil
	}
	recipientProfileID, err := s.ChatGuard.DMOtherProfileID(ctx, chatID, senderProfileID)
	if err != nil {
		if errors.Is(err, store.ErrNotChatMember) {
			return status.Error(codes.PermissionDenied, "not a chat member")
		}
		if st, ok := status.FromError(err); ok && st.Code() == codes.FailedPrecondition {
			return nil
		}
		if strings.Contains(err.Error(), "dm must have exactly two members") {
			return nil
		}
		return status.Error(codes.Internal, err.Error())
	}
	needsVoice := false
	needsFile := false
	for _, att := range attachments {
		if attachmentRequiresVoicePrivacy(att.Type) {
			needsVoice = true
		} else {
			needsFile = true
		}
	}
	matcher := privacy.Matcher{Social: s.Friends, Space: s.SpaceCoMembership}
	if needsVoice {
		audience, err := s.Privacy.AllowVoiceMessagesAudience(ctx, recipientProfileID)
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
		if err := privacy.CheckAllowed(matcher, ctx, recipientProfileID, senderProfileID, audience, guestguard.IsGuest(ctx)); err != nil {
			if errors.Is(err, privacy.ErrDenied) {
				return status.Error(codes.PermissionDenied, "voice message blocked by recipient privacy settings")
			}
			return status.Error(codes.Internal, err.Error())
		}
	}
	if needsFile {
		audience, err := s.Privacy.AllowFilesAudience(ctx, recipientProfileID)
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
		if err := privacy.CheckAllowed(matcher, ctx, recipientProfileID, senderProfileID, audience, guestguard.IsGuest(ctx)); err != nil {
			if errors.Is(err, privacy.ErrDenied) {
				return status.Error(codes.PermissionDenied, "file attachment blocked by recipient privacy settings")
			}
			return status.Error(codes.Internal, err.Error())
		}
	}
	return nil
}
