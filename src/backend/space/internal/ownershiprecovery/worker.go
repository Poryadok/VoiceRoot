package ownershiprecovery

import (
	"context"
	"errors"

	"voice/backend/space/internal/store"
)

type Scanner interface {
	ListPendingOwnership(context.Context, int) ([]*store.OwnershipJournal, error)
}

// Worker performs one bounded restart-safe recovery pass. The store owns
// ordering and the coordinator never clears a freeze without terminal evidence.
type Worker struct {
	Coordinator *Coordinator
	Scanner     Scanner
	BatchSize   int
}

func (w Worker) RunOnce(ctx context.Context) error {
	if w.Coordinator == nil || w.Scanner == nil || w.BatchSize < 1 {
		return errors.New("ownership recovery worker not configured")
	}
	journals, err := w.Scanner.ListPendingOwnership(ctx, w.BatchSize)
	if err != nil {
		return err
	}
	var failures []error
	for _, journal := range journals {
		if _, err := w.Coordinator.Recover(ctx, journal); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
