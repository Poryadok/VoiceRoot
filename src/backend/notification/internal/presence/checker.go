package presence

import (
	"context"

	"github.com/google/uuid"
)

// Checker resolves whether a profile is currently online (WS-connected).
type Checker interface {
	IsOnline(ctx context.Context, profileID uuid.UUID) (bool, error)
}

// OfflineChecker explicitly treats every profile as offline. Message routing must not use it as an authority fallback.
type OfflineChecker struct{}

func (OfflineChecker) IsOnline(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}
