package spacemedia

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/voice/internal/spacebarrier"
)

// Immutable locator read takes no row locks. Under the Space barrier the second
// read verifies both the locator and the saved lifecycle generation.
func lockAdmissionLive(ctx context.Context, tx pgx.Tx, operation uuid.UUID, generation string) error {
	var space string
	if err := tx.QueryRow(ctx, `SELECT space_id FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2`, operation, generation).Scan(&space); err != nil {
		return err
	}
	current, err := spacebarrier.LockLive(ctx, tx, space)
	if err != nil {
		return err
	}
	var savedSpace string
	var saved int64
	if err := tx.QueryRow(ctx, `SELECT space_id,lifecycle_generation FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2`, operation, generation).Scan(&savedSpace, &saved); err != nil {
		return err
	}
	if space != savedSpace || saved != current {
		return ErrAdmissionConflict
	}
	return nil
}

func lockAdmissionCleanup(ctx context.Context, tx pgx.Tx, operation uuid.UUID, generation string) error {
	var space string
	if err := tx.QueryRow(ctx, `SELECT space_id FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2`, operation, generation).Scan(&space); err != nil {
		return err
	}
	return spacebarrier.Lock(ctx, tx, space)
}
