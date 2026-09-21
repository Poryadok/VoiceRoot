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
	route, err := LoadGenerationRoute(ctx, m.Pool)
	if err != nil {
		return Quarantined, err
	}
	active := (&StoreAdapter{Pool: m.Pool, Generation: route.Active})
	result, err := active.ApplyAndCheckpoint(ctx, event, checkpoint)
	if err != nil {
		return result, err
	}
	if route.Rollback != 0 && route.Rollback != route.Active {
		if _, err := (&StoreAdapter{Pool: m.Pool, Generation: route.Rollback}).ApplyAndCheckpoint(ctx, event, checkpoint); err != nil {
			return result, err
		}
	}
	return result, nil
}
