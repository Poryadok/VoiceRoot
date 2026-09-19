package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
)

// GetLifecycleRecoverySpace returns only the recorded owner's frozen recovery
// projection. The caller must authenticate actorProfileID before entering this
// store boundary. Ordinary access and public activation are separate paths.
func (s *SpaceStore) GetLifecycleRecoverySpace(ctx context.Context, spaceID, actorProfileID uuid.UUID) (*spacev1.Space, error) {
	if s == nil || s.Pool == nil || s.tx != nil || spaceID == uuid.Nil || actorProfileID == uuid.Nil {
		return nil, ErrOwnershipScopeUnavailable
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOwnershipScopeUnavailable, err)
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err := lockLifecycleSpace(ctx, tx, spaceID); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOwnershipScopeUnavailable, err)
	}
	var name string
	var owner uuid.UUID
	var phase string
	var scheduled, purgeAfter *time.Time
	err = tx.QueryRow(ctx, `SELECT name,owner_profile_id FROM spaces WHERE id=$1`, spaceID).Scan(&name, &owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, pgx.ErrNoRows
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOwnershipScopeUnavailable, err)
	}
	if owner != actorProfileID {
		return nil, pgx.ErrNoRows
	}
	err = tx.QueryRow(ctx, `SELECT phase,scheduled_at,purge_after FROM space_lifecycle_aggregates WHERE space_id=$1`, spaceID).Scan(&phase, &scheduled, &purgeAfter)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrLifecycleStateTransition
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOwnershipScopeUnavailable, err)
	}
	if phase == "PURGED" {
		return nil, pgx.ErrNoRows
	}
	if phase == "LIVE" {
		return nil, ErrLifecycleStateTransition
	}
	if err := checkOwnershipJournalAvailable(ctx, tx, []uuid.UUID{spaceID}); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOwnershipScopeUnavailable, err)
	}
	if name == "" {
		return nil, ErrOwnershipScopeUnavailable
	}
	result := &spacev1.Space{Id: spaceID.String(), Name: name}
	switch phase {
	case "SCHEDULE_PENDING", "FREEZE_PENDING":
		if scheduled != nil || purgeAfter != nil {
			return nil, ErrOwnershipScopeUnavailable
		}
	case "SCHEDULED", "RESTORE_DECIDED", "PURGE_DECIDED", "PURGING":
		if scheduled == nil || purgeAfter == nil || scheduled.IsZero() || !purgeAfter.Equal(scheduled.Add(7*24*time.Hour)) {
			return nil, ErrOwnershipScopeUnavailable
		}
		result.DeletionScheduledAt = timestamppb.New(scheduled.UTC())
		result.PurgeAfter = timestamppb.New(purgeAfter.UTC())
		if result.DeletionScheduledAt.CheckValid() != nil || result.PurgeAfter.CheckValid() != nil {
			return nil, ErrOwnershipScopeUnavailable
		}
	default:
		return nil, ErrOwnershipScopeUnavailable
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOwnershipScopeUnavailable, err)
	}
	return result, nil
}
