package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	chatv1 "voice.app/voice/chat/v1"

	"voice/backend/chat/internal/chatevents"
	"voice/backend/chat/internal/store"
)

// ChatGRPC implements ChatService RPCs backed by chat_db (Phase 1: DM).
type ChatGRPC struct {
	chatv1.UnimplementedChatServiceServer
	DM         DMStore
	Profiles   UserProfileLookup
	Blocks     AccountBlockChecker
	ListEnrich ListChatsEnrichment // optional; Messaging S2S for preview + unread
	// ChatEvents is optional; when set, new DM creation publishes to NATS JetStream (stream chat_events, subjects chat.*).
	ChatEvents chatevents.Publisher
}

// DMStore persists DM chats and lists the caller's chats (Phase 1).
type DMStore interface {
	EnsureDM(ctx context.Context, callerProfileID, otherProfileID uuid.UUID) (*store.ChatRow, bool, error)
	ListChatsPage(ctx context.Context, viewerProfileID uuid.UUID, cursor string, limit int) (*store.ListChatsPage, error)
	FindDMChatByID(ctx context.Context, chatID uuid.UUID) (*store.ChatRow, error)
	IsChatMember(ctx context.Context, chatID, profileID uuid.UUID) (bool, error)
	ListChatMembers(ctx context.Context, chatID uuid.UUID) ([]store.ChatMemberRow, error)
}
