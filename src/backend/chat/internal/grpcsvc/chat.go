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

// DMStore persists chats and lists the caller's inbox (DM + standalone groups).
type DMStore interface {
	EnsureDM(ctx context.Context, callerProfileID, otherProfileID uuid.UUID) (*store.ChatRow, bool, error)
	ListChatsPage(ctx context.Context, viewerProfileID uuid.UUID, cursor string, limit int, inbox string) (*store.ListChatsPage, error)
	DMPeerProfileIDs(ctx context.Context, viewerProfileID uuid.UUID, chatIDs []uuid.UUID) (map[uuid.UUID]uuid.UUID, error)
	FindDMChatByID(ctx context.Context, chatID uuid.UUID) (*store.ChatRow, error)
	FindChatByID(ctx context.Context, chatID uuid.UUID) (*store.ChatRow, error)
	IsChatMember(ctx context.Context, chatID, profileID uuid.UUID) (bool, error)
	ListChatMembers(ctx context.Context, chatID uuid.UUID) ([]store.ChatMemberRow, error)
	SetInboxBucket(ctx context.Context, chatID, profileID uuid.UUID, bucket string) error
	CreateGroupChat(ctx context.Context, creatorProfileID uuid.UUID, name string) (*store.ChatRow, error)
	AddGroupMembers(ctx context.Context, chatID uuid.UUID, profileIDs []uuid.UUID) ([]uuid.UUID, error)
	RemoveGroupMember(ctx context.Context, chatID, profileID uuid.UUID) error
	LeaveGroupChat(ctx context.Context, chatID, profileID uuid.UUID) error
	UpdateGroupChat(ctx context.Context, chatID uuid.UUID, name, avatarURL *string) (*store.ChatRow, error)
	GetMemberRole(ctx context.Context, chatID, profileID uuid.UUID) (string, error)
}
