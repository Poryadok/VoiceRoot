package presence

import (
	"context"

	"github.com/google/uuid"
)

// Checker resolves whether a profile is currently online (WS-connected).
type Checker interface {
	IsOnline(ctx context.Context, profileID uuid.UUID) (bool, error)
}

// OfflineChecker treats all profiles as offline (degraded when USER_GRPC_ADDR unset).
type OfflineChecker struct{}

func (OfflineChecker) IsOnline(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}
