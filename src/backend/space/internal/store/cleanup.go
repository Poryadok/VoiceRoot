package store

import (
	"context"
	"time"
)

const cleanupTimeout = 5 * time.Second

// BoundedCleanupContext preserves request values while detaching cancellation
// and deadline. Compensation and transaction cleanup must not be abandoned
// merely because the request that exposed the failure has ended.
func BoundedCleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
}
