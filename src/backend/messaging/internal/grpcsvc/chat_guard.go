package grpcsvc

import (
	"context"

	"github.com/google/uuid"
)

// ChatGuard validates chat membership and resolves DM peers (Phase 1: DM, two members).
// Implemented by SQL against chat_db or by S2S calls to ChatService.
type ChatGuard interface {
	EnsureMember(ctx context.Context, chatID, profileID uuid.UUID) error
	DMOtherProfileID(ctx context.Context, chatID, profileID uuid.UUID) (uuid.UUID, error)
}

// ProfileAccountLookup resolves profile_id → account_id (User Service).
type ProfileAccountLookup interface {
	AccountIDByProfileID(ctx context.Context, profileID uuid.UUID) (uuid.UUID, error)
}

// AccountPairBlockChecker reports whether two accounts must not exchange DM messages (Social IsBlocked, both directions).
type AccountPairBlockChecker interface {
	AccountPairBlocked(ctx context.Context, viewerAccountID, otherAccountID uuid.UUID) (bool, error)
}
