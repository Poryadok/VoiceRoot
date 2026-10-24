package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// InitializeNativeMembership is the administrative deployment producer. It owns
// one committed RC/fenced transaction; no token or outer transaction crosses
// this first full UNKNOWN cutover. The existing scoped producer authenticates
// all source facts and exact catalog. Its protected SQL privileges refuse the
// ordinary serving LOGIN. This is not a public RPC, read repair or app fallback.
func (s *SpaceStore) InitializeNativeMembership(ctx context.Context, space uuid.UUID) error {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil || ctx.Err() != nil || space == uuid.Nil {
		return ErrEffectiveEpisodeUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return ErrEffectiveEpisodeUnavailable
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err = lockLifecycleSpace(ctx, tx, space); err != nil {
		return ErrEffectiveEpisodeUnavailable
	}
	scoped := *s
	scoped.tx = tx
	if err = scoped.InitializeScopedNativeMembership(ctx, space); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return ErrEffectiveEpisodeUnavailable
	}
	return nil
}
