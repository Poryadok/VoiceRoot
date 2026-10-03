package lifecyclecoord

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
)

// ExpiryStore decides only when PostgreSQL time has reached the saved deadline.
// It must never turn an early background scan into an unsolicited restore.
type ExpiryStore interface {
	DecideExpiredLifecyclePurge(context.Context, uuid.UUID) (*spacecore.LifecycleAggregate, error)
}
type PurgeFinalizer interface {
	Finalize(context.Context, uuid.UUID) error
}

// Recover resumes one durable phase. Terminal rows are inert; exact owner
// requests and saved receipts are reconstructed by the existing barriers.
func (c *Coordinator) Recover(ctx context.Context, spaceID uuid.UUID, finalizer PurgeFinalizer) error {
	if c == nil || c.dependencies.Store == nil {
		return ErrCoordinatorNotConfigured
	}
	aggregate, err := c.dependencies.Store.LoadLifecycle(ctx, spaceID)
	if err != nil {
		return err
	}
	switch aggregate.Phase() {
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED:
		return nil
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING:
		return c.ScheduleDeletion(ctx, spaceID)
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED:
		_, _, err = c.ApplyFenceBarrier(ctx, spaceID)
		return err
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULED:
		expiry, ok := c.dependencies.Store.(ExpiryStore)
		if !ok {
			return ErrCoordinatorNotConfigured
		}
		aggregate, err = expiry.DecideExpiredLifecyclePurge(ctx, spaceID)
		if err != nil {
			return err
		}
		if aggregate.Phase() == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULED {
			return nil
		}
		// A public restore can win between the scan and the locked DB decision.
		// Dispatch its durable phase again instead of attempting a purge barrier.
		if aggregate.Phase() != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED {
			return c.Recover(ctx, spaceID, finalizer)
		}
		fallthrough
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED:
		if finalizer == nil {
			return ErrCoordinatorNotConfigured
		}
		_, _, err = c.ApplyFenceBarrier(ctx, spaceID)
		if err != nil {
			return err
		}
		fallthrough
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING:
		if finalizer == nil {
			return ErrCoordinatorNotConfigured
		}
		if _, err = c.ApplyPurgeBarrier(ctx, spaceID); err != nil {
			return err
		}
		return finalizer.Finalize(ctx, spaceID)
	default:
		return fmt.Errorf("unsupported lifecycle recovery phase %s", aggregate.Phase())
	}
}
