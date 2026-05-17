package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	chatv1 "voice.app/voice/chat/v1"

	"voice/backend/chat/internal/store"
)

// ChatGRPC implements ChatService RPCs backed by chat_db (Phase 1: DM).
type ChatGRPC struct {
	chatv1.UnimplementedChatServiceServer
	DM       DMEnsureStore
	Profiles UserProfileLookup
	Blocks   AccountBlockChecker
}

// DMEnsureStore loads or creates a DM between two profiles.
type DMEnsureStore interface {
	EnsureDM(ctx context.Context, callerProfileID, otherProfileID uuid.UUID) (*store.ChatRow, error)
}
