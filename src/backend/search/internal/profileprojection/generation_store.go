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
	if _, err = conn.Exec(ctx, `INSERT INTO search_user_profile_checkpoint(generation,singleton)
		VALUES($1,true) ON CONFLICT(generation) DO NOTHING`, generation); err != nil {
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
	result, err := pool.Exec(ctx, `UPDATE search_user_profile_generations SET state='ready',ready_at=now() WHERE generation=$1 AND state='building'`, generation)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("generation %d is not building", generation)
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
	if _, err := tx.Exec(ctx, `UPDATE search_user_profile_generation_route SET active_generation=$1,rollback_generation=$2,updated_at=now() WHERE singleton=true`, route.Active, route.Rollback); err != nil {
		return GenerationRoute{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return GenerationRoute{}, err
	}
	return route, nil
}
