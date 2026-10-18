package spacemedia

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *PostgresAdmissionStore) ProjectionGeneration(ctx context.Context, operation uuid.UUID, generation string) (int64, error) {
	if !s.usable() {
		return 0, ErrAdmissionUnavailable
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockAdmissionLive(ctx, tx, operation, generation); err != nil {
		return 0, err
	}
	var lifecycle int64
	err = tx.QueryRow(ctx, `SELECT a.lifecycle_generation FROM voice_space_media_admissions a JOIN voice_space_media_room_heads h USING(voice_room_id,room_generation)
WHERE a.operation_id=$1 AND a.generation=$2 AND a.state='COMMITTED' AND a.participant_state IN ('PREPARED','ACTIVE') AND h.state='OPEN'`, operation, generation).Scan(&lifecycle)
	if err != nil {
		return 0, err
	}
	return lifecycle, tx.Commit(ctx)
}

// RebuildLifecycleWatermarks runs before ANY projection recovery. Missing Redis
// keys never authorize writers. It also restores restrictive heads with no rows.
func (s *PostgresAdmissionStore) RebuildLifecycleWatermarks(ctx context.Context, setter interface {
	SetSpaceLifecycle(context.Context, string, int64, bool) error
}) error {
	rows, err := s.pool.Query(ctx, `SELECT space_id,generation,state='LIVE' FROM voice_space_lifecycle_fence_heads
UNION ALL SELECT DISTINCT a.space_id,0,TRUE FROM voice_space_media_room_heads a WHERE NOT EXISTS(SELECT 1 FROM voice_space_lifecycle_fence_heads f WHERE f.space_id=a.space_id)`)
	if err != nil {
		return err
	}
	type mark struct {
		space      string
		generation int64
		live       bool
	}
	var marks []mark
	for rows.Next() {
		var m mark
		if err := rows.Scan(&m.space, &m.generation, &m.live); err != nil {
			rows.Close()
			return err
		}
		marks = append(marks, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, m := range marks {
		if err := setter.SetSpaceLifecycle(ctx, m.space, m.generation, m.live); err != nil {
			return err
		}
	}
	return nil
}
