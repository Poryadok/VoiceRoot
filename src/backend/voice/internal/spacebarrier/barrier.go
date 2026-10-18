// Package spacebarrier defines the shared PostgreSQL serialization boundary for
// Space lifecycle and native media writers. No external IO belongs under it.
package spacebarrier

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const ClassID int32 = 0x53504345

var ErrRestricted = errors.New("Space lifecycle denies native media activation")

func Lock(ctx context.Context, tx pgx.Tx, space string) error {
	id, err := uuid.Parse(space)
	if err != nil || id == uuid.Nil || id.String() != space {
		return ErrRestricted
	}
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,hashtext($2::text))`, ClassID, space)
	return err
}

// Live must be called in a separate statement after Lock at READ COMMITTED.
// An absent head is generation0, never an invented LIVE generation. Missing
// source schema fails the query; startup readiness checks the migrated schema.
func Live(ctx context.Context, tx pgx.Tx, space string) (int64, error) {
	var generation int64
	var state string
	err := tx.QueryRow(ctx, `SELECT generation,state FROM voice_space_lifecycle_fence_heads WHERE space_id=$1`, space).Scan(&generation, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if state == "LIVE" {
		var proven bool
		if err := tx.QueryRow(ctx, `SELECT coverage_sealed AND retirement_proven AND receipt_bytes IS NOT NULL FROM voice_space_lifecycle_fence_receipts WHERE space_id=$1 AND generation=$2 AND applied_state='FROZEN'`, space, generation-1).Scan(&proven); err != nil || !proven {
			return 0, ErrRestricted
		}
	}
	if state != "LIVE" {
		return 0, ErrRestricted
	}
	return generation, nil
}

func LockLive(ctx context.Context, tx pgx.Tx, space string) (int64, error) {
	if err := Lock(ctx, tx, space); err != nil {
		return 0, err
	}
	return Live(ctx, tx, space)
}
