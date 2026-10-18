package spacemedia

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/voice/internal/spacebarrier"
	"voice/backend/voice/internal/store"
)

// ReadSpaceLifecycle is maintenance authority sampled AFTER observing the exact
// RESTORING gate incarnation. No Redis IO is held under these SQL locks.
func (s *PostgresAdmissionStore) ReadSpaceLifecycle(ctx context.Context, space string) (int64, bool, error) {
	if err := s.CheckSchema(ctx); err != nil {
		return 0, false, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := spacebarrier.Lock(ctx, tx, space); err != nil {
		return 0, false, err
	}
	var generation int64
	var live bool
	err = tx.QueryRow(ctx, `SELECT generation,state='LIVE' FROM voice_space_lifecycle_fence_heads WHERE space_id=$1`, space).Scan(&generation, &live)
	if err == pgx.ErrNoRows {
		generation = 0
		live = true
	} else if err != nil {
		return 0, false, err
	}
	if live && generation > 0 {
		if _, err := spacebarrier.Live(ctx, tx, space); err != nil {
			return 0, false, err
		}
	}
	return generation, live, tx.Commit(ctx)
}

// AuthorizeSpaceProjection follows READY-gate acquisition. It binds the ticket
// to fresh durable authority and the complete native tuple, never cached Redis.
func (s *PostgresAdmissionStore) AuthorizeSpaceProjection(ctx context.Context, operation uuid.UUID, generation string, t store.SpaceProjectionTicket) (store.SpaceProjectionTicket, error) {
	if !s.usable() || t.Incarnation == "" {
		return store.SpaceProjectionTicket{}, ErrAdmissionUnavailable
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return store.SpaceProjectionTicket{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockAdmissionLive(ctx, tx, operation, generation); err != nil {
		return store.SpaceProjectionTicket{}, err
	}
	var savedLifecycle int64
	var space string
	err = tx.QueryRow(ctx, `SELECT a.space_id,a.lifecycle_generation,a.room_id,a.voice_room_id,a.room_generation,a.operation_id,a.generation,a.account_id,a.profile_id,a.livekit_identity
FROM voice_space_media_admissions a JOIN voice_space_media_room_heads h USING(voice_room_id,room_generation)
WHERE a.operation_id=$1 AND a.generation=$2 AND a.state='COMMITTED' AND a.participant_state IN ('PREPARED','ACTIVE') AND h.state='OPEN' FOR UPDATE OF h`, operation, generation).Scan(&space, &savedLifecycle, &t.RoomID, &t.VoiceRoomID, &t.RoomGeneration, &t.OperationID, &t.AdmissionGeneration, &t.AccountID, &t.ProfileID, &t.Identity)
	if err != nil {
		return store.SpaceProjectionTicket{}, err
	}
	if space != t.SpaceID || savedLifecycle != t.Generation {
		return store.SpaceProjectionTicket{}, ErrAdmissionConflict
	}
	if err := spacebarrier.LockIdentities(ctx, tx, t.AccountID, t.ProfileID); err != nil {
		return store.SpaceProjectionTicket{}, err
	}
	var owned bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_account_voice_fences f JOIN voice_profile_account_mappings m ON m.profile_id=f.profile_id
WHERE f.account_id=$1 AND f.profile_id=$2 AND f.room_id=$3 AND f.admission_operation_id=$4 AND f.admission_generation=$5 AND m.account_id=f.account_id)`, t.AccountID, t.ProfileID, t.RoomID, operation, generation).Scan(&owned)
	if err != nil || !owned {
		return store.SpaceProjectionTicket{}, ErrAdmissionConflict
	}
	return t, tx.Commit(ctx)
}

// ProjectionGeneration remains a native inspection API, not dispatch authority.
func (s *PostgresAdmissionStore) ProjectionGeneration(ctx context.Context, operation uuid.UUID, generation string) (int64, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockAdmissionLive(ctx, tx, operation, generation); err != nil {
		return 0, err
	}
	var g int64
	err = tx.QueryRow(ctx, `SELECT a.lifecycle_generation FROM voice_space_media_admissions a JOIN voice_space_media_room_heads h USING(voice_room_id,room_generation) WHERE a.operation_id=$1 AND a.generation=$2 AND a.state='COMMITTED' AND a.participant_state IN ('PREPARED','ACTIVE') AND h.state='OPEN'`, operation, generation).Scan(&g)
	if err != nil {
		return 0, err
	}
	return g, tx.Commit(ctx)
}
func (s *PostgresAdmissionStore) RebuildLifecycleWatermarks(ctx context.Context, gates store.SpaceProjectionGates) error {
	rows, err := s.pool.Query(ctx, `SELECT space_id FROM voice_space_lifecycle_fence_heads UNION SELECT space_id FROM voice_space_media_room_heads`)
	if err != nil {
		return err
	}
	var spaces []string
	for rows.Next() {
		var space string
		if err := rows.Scan(&space); err != nil {
			rows.Close()
			return err
		}
		spaces = append(spaces, space)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, space := range spaces {
		if err := gates.EnsureSpaceLifecycle(ctx, space, s.ReadSpaceLifecycle); err != nil {
			return err
		}
	}
	return nil
}
