// Package messageviews reconciles the Messaging-owned view store with User's
// explicit profile soft-deletion state. Missing profiles never imply deletion.
package messageviews

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"log/slog"
	"time"
)

type DeletionState uint8

const (
	Unknown DeletionState = iota
	NotDeleted
	SoftDeleted
)
const PageSize = 200

type Store interface {
	MessageViewProfiles(context.Context, uuid.UUID, int) ([]uuid.UUID, error)
	DeleteProfileMessageViews(context.Context, []uuid.UUID) error
}
type DeletionChecker interface {
	ProfileDeletionStates(context.Context, []uuid.UUID) (map[uuid.UUID]DeletionState, error)
}
type Compactor struct {
	Store   Store
	Checker DeletionChecker
	Logger  *slog.Logger
}

func (c *Compactor) Sweep(ctx context.Context) error {
	if c == nil || c.Store == nil || c.Checker == nil {
		return errors.New("message view compactor unavailable")
	}
	after := uuid.Nil
	for {
		ids, err := c.Store.MessageViewProfiles(ctx, after, PageSize)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		if len(ids) > PageSize {
			return errors.New("message view compaction page exceeds bound")
		}
		states, err := c.Checker.ProfileDeletionStates(ctx, ids)
		if err != nil {
			return err
		}
		if len(states) != len(ids) {
			return errors.New("incomplete profile deletion states")
		}
		deleted := make([]uuid.UUID, 0, len(ids))
		unknown := 0
		for _, id := range ids {
			state, ok := states[id]
			if !ok || state > SoftDeleted {
				return errors.New("invalid profile deletion states")
			}
			switch state {
			case SoftDeleted:
				deleted = append(deleted, id)
			case Unknown:
				unknown++
			}
		}
		// Validate the complete authority chunk before performing any local deletion.
		if err = c.Store.DeleteProfileMessageViews(ctx, deleted); err != nil {
			return err
		}
		if unknown > 0 && c.Logger != nil {
			c.Logger.Warn("message view compaction retains unknown profiles", slog.Int("count", unknown))
		}
		after = ids[len(ids)-1]
		if len(ids) < PageSize {
			return nil
		}
	}
}

func (c *Compactor) Run(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		if err := c.Sweep(ctx); err != nil && ctx.Err() == nil && c.Logger != nil {
			c.Logger.Warn("message view compaction failed", slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
