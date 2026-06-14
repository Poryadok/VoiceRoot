package grpcsvc

import (
	"context"

	"github.com/google/uuid"
)

// PlatformModerationChecker enforces platform-level shadow ban and spam mutes (Moderation Service).
type PlatformModerationChecker interface {
	IsShadowBanned(ctx context.Context, accountID uuid.UUID) (bool, error)
	CheckMessageAllowed(ctx context.Context, profileID uuid.UUID, chatID uuid.UUID, content string) error
}
