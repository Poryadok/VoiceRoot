package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/messaging/internal/store"

	chatv1 "voice.app/voice/chat/v1"
)

// shouldPublishReadReceipt implements privacy.md's symmetric DM opt-out.
// It deliberately returns false for a failing policy dependency: MarkRead still
// persists the caller's private cursor, but no public receipt or event leaks.
// A non-DM has no peer receipt and remains unaffected by this toggle.
func (s *MessagingGRPC) shouldPublishReadReceipt(ctx context.Context, chatID, readerProfileID uuid.UUID) bool {
	if s == nil || s.ChatGuard == nil {
		return true
	}
	if s.ChatTypeResolver != nil {
		chatType, err := s.ChatTypeResolver.ResolveChatType(ctx, chatID, readerProfileID)
		if err != nil {
			return false
		}
		if chatType != chatv1.ChatType_CHAT_TYPE_DM {
			return true
		}
	}
	peerProfileID, err := s.ChatGuard.DMOtherProfileID(ctx, chatID, readerProfileID)
	if err != nil {
		if errors.Is(err, store.ErrNotChatMember) {
			return false
		}
		if st, ok := status.FromError(err); ok && st.Code() == codes.FailedPrecondition {
			return true
		}
		if strings.Contains(err.Error(), "dm must have exactly two members") {
			return true
		}
		return false
	}
	checker, ok := s.Privacy.(ReadReceiptPrivacyChecker)
	if !ok {
		return true
	}
	readerEnabled, err := checker.ShowReadReceipts(ctx, readerProfileID)
	if err != nil || !readerEnabled {
		return false
	}
	peerEnabled, err := checker.ShowReadReceipts(ctx, peerProfileID)
	return err == nil && peerEnabled
}
