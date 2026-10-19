package identityassociation

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	"time"
)

// ExpectedFence is a pinned owner-generation/keyset expectation, not a grant.
type ExpectedFence struct {
	Generation     uuid.UUID
	KeysetRevision uint64
}
type IdentityTx struct {
	tx             pgx.Tx
	generation     uuid.UUID
	keysetRevision uint64
	txid           int64
	closed         bool
}

func BeginIdentityTx(ctx context.Context, pool *pgxpool.Pool, expected *ExpectedFence) (*IdentityTx, error) {
	if pool == nil {
		return nil, ErrIncomplete
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	identity, err := acquireIdentityTx(ctx, tx, expected)
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	return identity, nil
}
func acquireIdentityTx(ctx context.Context, tx pgx.Tx, expected *ExpectedFence) (*IdentityTx, error) {
	if tx == nil || expected == nil || expected.Generation == uuid.Nil || expected.KeysetRevision > math.MaxInt64 {
		return nil, ErrIncomplete
	}
	var isolation string
	if err := tx.QueryRow(ctx, `SHOW transaction_isolation`).Scan(&isolation); err != nil {
		return nil, err
	}
	if isolation != "read committed" {
		return nil, ErrConflict
	}
	// Acquisition is the first participating row lock. The caller must perform
	// authoritative multirow reads in a subsequent command after this returns.
	if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout='2s'; SET LOCAL statement_timeout='2s'`); err != nil {
		return nil, err
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cap := &IdentityTx{tx: tx}
	if err := tx.QueryRow(bounded, `SELECT history_generation,keyset_revision,txid_current() FROM public.identity_meta WHERE singleton FOR UPDATE`).Scan(&cap.generation, &cap.keysetRevision, &cap.txid); err != nil {
		return nil, err
	}
	if cap.generation != expected.Generation || cap.keysetRevision != expected.KeysetRevision {
		return nil, ErrConflict
	}
	return cap, nil
}
func (t *IdentityTx) SQL() pgx.Tx {
	if t == nil || t.closed {
		return nil
	}
	return t.tx
}
func (t *IdentityTx) Commit(ctx context.Context) error {
	if t == nil || t.closed {
		return ErrConflict
	}
	t.closed = true
	return t.tx.Commit(ctx)
}
func (t *IdentityTx) Rollback(ctx context.Context) error {
	if t == nil || t.closed {
		return nil
	}
	t.closed = true
	return t.tx.Rollback(ctx)
}
