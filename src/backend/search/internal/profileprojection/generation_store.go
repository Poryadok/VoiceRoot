package profileprojection

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

const generationRebuildLock int64 = 0x534541524348

// LoadGenerationRoute returns the only route permitted to serve profile
// queries. Callers must treat an absent row as unavailable rather than falling
// back to the legacy unscoped table.
func LoadGenerationRoute(ctx context.Context, pool *pgxpool.Pool) (GenerationRoute, error) {
	var route GenerationRoute
	var rollback *int64
	if pool == nil {
		return route, fmt.Errorf("generation route store unavailable")
	}
	if err := pool.QueryRow(ctx, `SELECT active_generation,rollback_generation FROM search_user_profile_generation_route WHERE singleton=true`).Scan(&route.Active, &rollback); err != nil {
		return route, err
	}
	if rollback != nil {
		route.Rollback = uint64(*rollback)
	}
	if route.Active == 0 {
		return GenerationRoute{}, fmt.Errorf("invalid active generation route")
	}
	return route, nil
}

// TryStartGeneration serializes rebuild admission across replicas. The caller
// owns the returned connection until FinishGenerationRebuild, so a process
// crash releases PostgreSQL's advisory lock and a later retry can resume.
func TryStartGeneration(ctx context.Context, pool *pgxpool.Pool, generation uint64) (*pgxpool.Conn, bool, error) {
	if pool == nil || generation == 0 {
		return nil, false, fmt.Errorf("invalid generation rebuild request")
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, false, err
	}
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, generationRebuildLock).Scan(&locked); err != nil || !locked {
		conn.Release()
		return nil, false, err
	}
	created, err := conn.Exec(ctx, `INSERT INTO search_user_profile_generations(generation,state)
		VALUES($1,'building') ON CONFLICT(generation) DO NOTHING`, generation)
	if err != nil {
		_, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, generationRebuildLock)
		conn.Release()
		return nil, false, err
	}
	if created.RowsAffected() == 0 {
		var state GenerationState
		if err = conn.QueryRow(ctx, `SELECT state FROM search_user_profile_generations WHERE generation=$1`, generation).Scan(&state); err != nil {
			_, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, generationRebuildLock)
			conn.Release()
			return nil, false, err
		}
		if state == GenerationReady {
			_, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, generationRebuildLock)
			conn.Release()
			return nil, false, nil
		}
	}
	if _, err = conn.Exec(ctx, `INSERT INTO search_user_profile_generation_checkpoint(generation)
		VALUES($1) ON CONFLICT(generation) DO NOTHING`, generation); err != nil {
		_, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, generationRebuildLock)
		conn.Release()
		return nil, false, err
	}
	return conn, true, nil
}

func FinishGenerationRebuild(ctx context.Context, conn *pgxpool.Conn) {
	if conn == nil {
		return
	}
	_, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, generationRebuildLock)
	conn.Release()
}

func MarkGenerationReady(ctx context.Context, pool *pgxpool.Pool, generation uint64) error {
	var highWatermark, cutoff, count uint64
	var digest []byte
	if err := pool.QueryRow(ctx, `SELECT snapshot_high_watermark,journal_offset,evidence_count,evidence_digest FROM search_user_profile_generation_checkpoint WHERE generation=$1 AND snapshot_phase='replay'`, generation).Scan(&highWatermark, &cutoff, &count, &digest); err != nil {
		return fmt.Errorf("generation %d has incomplete snapshot evidence: %w", generation, err)
	}
	if count == 0 || len(digest) != 32 {
		return fmt.Errorf("generation %d has incomplete event evidence", generation)
	}
	result, err := pool.Exec(ctx, `UPDATE search_user_profile_generations SET state='ready',journal_cutoff=$2,high_watermark=$3,evidence_sha256=$4,ready_at=now()
		WHERE generation=$1 AND state='building'`, generation, cutoff, highWatermark, digest)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("generation %d is not building", generation)
	}
	return nil
}

// VerifyGenerationEvidence compares an independently replayed protected stream
// with the candidate's transactional local evidence before readiness/promotion.
func VerifyGenerationEvidence(ctx context.Context, pool *pgxpool.Pool, generation uint64, expected ReadinessEvidence) error {
	var count, first, last, cutoff uint64
	var digest []byte
	if err := pool.QueryRow(ctx, `SELECT evidence_count,evidence_first_offset,evidence_last_offset,journal_offset,evidence_digest FROM search_user_profile_generation_checkpoint WHERE generation=$1`, generation).Scan(&count, &first, &last, &cutoff, &digest); err != nil {
		return err
	}
	if expected.Generation != generation || count != expected.Count || first != expected.First || last != expected.Last || cutoff != expected.Cutoff || len(digest) != len(expected.Digest) || string(digest) != string(expected.Digest[:]) {
		return fmt.Errorf("generation %d protected replay evidence mismatch", generation)
	}
	return nil
}

func PromoteGeneration(ctx context.Context, pool *pgxpool.Pool, target uint64) (GenerationRoute, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return GenerationRoute{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var route GenerationRoute
	var rollback *int64
	if err := tx.QueryRow(ctx, `SELECT active_generation,rollback_generation FROM search_user_profile_generation_route WHERE singleton=true FOR UPDATE`).Scan(&route.Active, &rollback); err != nil {
		return GenerationRoute{}, err
	}
	if rollback != nil {
		route.Rollback = uint64(*rollback)
	}
	var state GenerationState
	if err := tx.QueryRow(ctx, `SELECT state FROM search_user_profile_generations WHERE generation=$1`, target).Scan(&state); err != nil {
		return GenerationRoute{}, err
	}
	if err := route.Promote(target, map[uint64]GenerationState{target: state}); err != nil {
		return GenerationRoute{}, err
	}
	var activeOffset, targetOffset int64
	var quarantined bool
	var evidence []byte
	if err := tx.QueryRow(ctx, `SELECT journal_offset FROM search_user_profile_generation_checkpoint WHERE generation=$1`, route.Rollback).Scan(&activeOffset); err != nil {
		return GenerationRoute{}, err
	}
	if err := tx.QueryRow(ctx, `SELECT journal_offset FROM search_user_profile_generation_checkpoint WHERE generation=$1`, route.Active).Scan(&targetOffset); err != nil {
		return GenerationRoute{}, err
	}
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM search_user_profile_generation_inbox WHERE generation=$1 AND quarantined_at IS NOT NULL)`, route.Active).Scan(&quarantined); err != nil {
		return GenerationRoute{}, err
	}
	if err := tx.QueryRow(ctx, `SELECT evidence_sha256 FROM search_user_profile_generations WHERE generation=$1`, route.Active).Scan(&evidence); err != nil {
		return GenerationRoute{}, err
	}
	if quarantined || len(evidence) != 32 || targetOffset < activeOffset {
		return GenerationRoute{}, fmt.Errorf("generation %d has not converged to the active checkpoint", route.Active)
	}
	if _, err := tx.Exec(ctx, `UPDATE search_user_profile_generation_route SET active_generation=$1,rollback_generation=$2,updated_at=now() WHERE singleton=true`, route.Active, route.Rollback); err != nil {
		return GenerationRoute{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return GenerationRoute{}, err
	}
	return route, nil
}

func RollbackGeneration(ctx context.Context, pool *pgxpool.Pool) (GenerationRoute, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return GenerationRoute{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var route GenerationRoute
	var rollback *int64
	if err := tx.QueryRow(ctx, `SELECT active_generation,rollback_generation FROM search_user_profile_generation_route WHERE singleton=true FOR UPDATE`).Scan(&route.Active, &rollback); err != nil {
		return GenerationRoute{}, err
	}
	if rollback != nil {
		route.Rollback = uint64(*rollback)
	}
	if err := route.RollbackToPrevious(); err != nil {
		return GenerationRoute{}, err
	}
	var activeOffset, rollbackOffset int64
	var quarantined bool
	if err := tx.QueryRow(ctx, `SELECT journal_offset FROM search_user_profile_generation_checkpoint WHERE generation=$1`, route.Active).Scan(&activeOffset); err != nil {
		return GenerationRoute{}, err
	}
	if err := tx.QueryRow(ctx, `SELECT journal_offset FROM search_user_profile_generation_checkpoint WHERE generation=$1`, route.Rollback).Scan(&rollbackOffset); err != nil {
		return GenerationRoute{}, err
	}
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM search_user_profile_generation_inbox WHERE generation IN ($1,$2) AND quarantined_at IS NOT NULL)`, route.Active, route.Rollback).Scan(&quarantined); err != nil {
		return GenerationRoute{}, err
	}
	if quarantined || activeOffset != rollbackOffset {
		return GenerationRoute{}, fmt.Errorf("rollback generations are not equally healthy")
	}
	if _, err := tx.Exec(ctx, `UPDATE search_user_profile_generation_route SET active_generation=$1,rollback_generation=$2,updated_at=now() WHERE singleton=true`, route.Active, route.Rollback); err != nil {
		return GenerationRoute{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return GenerationRoute{}, err
	}
	return route, nil
}
