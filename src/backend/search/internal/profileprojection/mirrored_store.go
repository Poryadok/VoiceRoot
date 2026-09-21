package profileprojection

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	userv1 "voice.app/voice/user/v1"
)

// MirroredStoreAdapter resolves the durable route on every delivery. Once a
// promotion has committed, User journal records are applied to both the active
// and rollback generations. A failed second apply leaves the message unacked;
// replay is safe through each generation's revision fence.
type MirroredStoreAdapter struct{ Pool *pgxpool.Pool }

func (m *MirroredStoreAdapter) ApplyAndCheckpoint(ctx context.Context, event *userv1.SearchProfileProjectionEvent, checkpoint uint64) (ApplyResult, error) {
	if m == nil || m.Pool == nil {
		return Quarantined, fmt.Errorf("profile projection store unavailable")
	}
	tx, err := m.Pool.Begin(ctx)
	if err != nil {
		return Quarantined, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var route GenerationRoute
	var rollback *int64
	if err := tx.QueryRow(ctx, `SELECT active_generation,rollback_generation FROM search_user_profile_generation_route WHERE singleton=true FOR UPDATE`).Scan(&route.Active, &rollback); err != nil {
		return Quarantined, err
	}
	if rollback != nil {
		route.Rollback = uint64(*rollback)
	}
	if route.Active == 0 {
		return Quarantined, fmt.Errorf("invalid active generation route")
	}
	active := (&StoreAdapter{Pool: m.Pool, Generation: route.Active})
	result, err := active.applyInTransaction(ctx, tx, event, checkpoint)
	if err != nil && result != Quarantined {
		return result, err
	}
	applyErr := err
	if route.Rollback != 0 && route.Rollback != route.Active {
		rollbackResult, rollbackErr := (&StoreAdapter{Pool: m.Pool, Generation: route.Rollback}).applyInTransaction(ctx, tx, event, checkpoint)
		if rollbackErr != nil && rollbackResult != Quarantined {
			return result, rollbackErr
		}
		if applyErr == nil {
			applyErr = rollbackErr
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Quarantined, err
	}
	return result, applyErr
}
