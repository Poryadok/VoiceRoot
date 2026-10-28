package registry

import (
	"context"
	"fmt"
	"time"
)

// RunSessionWorker polls durable pending operations. The database lease is the
// cross-process single-worker guard; cancellation never erases a claimed stage.
func RunSessionWorker(ctx context.Context, orchestrator *SessionOrchestrator, interval time.Duration) {
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	retentionTicker := time.NewTicker(time.Minute)
	defer retentionTicker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		expired, expiryErr := orchestrator.Store.expiredActiveSessions(ctx)
		if expiryErr == nil {
			for _, lease := range expired {
				if ctx.Err() != nil {
					return
				}
				principal := SessionPrincipal{ApplicationID: lease.ApplicationID, EnvironmentID: lease.EnvironmentID}
				operationID := deterministicOwnerID(lease.ApplicationID, lease.EnvironmentID, lease.SessionID,
					"roster_lease_close", fmt.Sprint(lease.RosterRevision))
				_, _ = orchestrator.CloseSession(ctx, principal, lease.SessionID, operationID)
			}
		}
		keys, err := orchestrator.Store.due(ctx)
		if err == nil {
			for _, key := range keys {
				if ctx.Err() != nil {
					return
				}
				_, _ = orchestrator.AdvanceScoped(ctx, key)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-retentionTicker.C:
			_, _ = orchestrator.Store.PurgeExpiredSessionData(ctx, 50)
		}
	}
}
