package grpcsvc

import (
	"context"

	"github.com/google/uuid"

	chatv1 "voice.app/voice/chat/v1"
)

// shouldPublishReadReceipt implements privacy.md's symmetric DM opt-out.
// The private cursor is written independently. Public receipt state is emitted
// only after an authoritative chat-type lookup and both DM participants' policy
// reads succeed. Any missing or failing DM dependency therefore fails closed.
func (s *MessagingGRPC) shouldPublishReadReceipt(ctx context.Context, chatID, readerProfileID uuid.UUID) bool {
	if s == nil || s.ChatTypeResolver == nil {
		return false
	}
	chatType, err := s.ChatTypeResolver.ResolveChatType(ctx, chatID, readerProfileID)
	if err != nil {
		return false
	}
	if chatType != chatv1.ChatType_CHAT_TYPE_DM {
		return true
	}
	if s.ChatGuard == nil {
		return false
	}
	peerProfileID, err := s.ChatGuard.DMOtherProfileID(ctx, chatID, readerProfileID)
	if err != nil {
		return false
	}
	checker, ok := s.Privacy.(ReadReceiptPrivacyChecker)
	if !ok || checker == nil {
		return false
	}
	readerEnabled, err := checker.ShowReadReceipts(ctx, readerProfileID)
	if err != nil || !readerEnabled {
		return false
	}
	peerEnabled, err := checker.ShowReadReceipts(ctx, peerProfileID)
	return err == nil && peerEnabled
}
