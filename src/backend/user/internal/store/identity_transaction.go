package store

import (
	"context"

	"github.com/jackc/pgx/v5"
	"voice/backend/user/internal/identityassociation"
)

// identityMutationTx lets existing SQL helpers retain pgx.Tx signatures while
// carrying proof that their outer transaction acquired I before any local lock.
type identityMutationTx struct {
	pgx.Tx
	identity *identityassociation.IdentityTx
}

func (s *ProfileStore) beginIdentityMutationTx(ctx context.Context) (*identityMutationTx, error) {
	if s == nil || s.pool == nil {
		return nil, identityassociation.ErrIncomplete
	}
	expected := &identityassociation.ExpectedFence{}
	// This is an unprivileged expectation read outside the mutation. Rotation
	// between this read and I conflicts; missing source metadata never falls back.
	if err := s.pool.QueryRow(ctx, `SELECT history_generation,keyset_revision FROM public.identity_meta WHERE singleton`).Scan(&expected.Generation, &expected.KeysetRevision); err != nil {
		return nil, err
	}
	identity, err := identityassociation.BeginIdentityTx(ctx, s.pool, expected)
	if err != nil {
		return nil, err
	}
	return &identityMutationTx{Tx: identity.SQL(), identity: identity}, nil
}
func (t *identityMutationTx) Commit(ctx context.Context) error   { return t.identity.Commit(ctx) }
func (t *identityMutationTx) Rollback(ctx context.Context) error { return t.identity.Rollback(ctx) }

func identityTxForMutation(tx pgx.Tx) (*identityassociation.IdentityTx, error) {
	owned, ok := tx.(*identityMutationTx)
	if !ok || owned == nil || owned.identity == nil || owned.identity.SQL() == nil {
		return nil, identityassociation.ErrIncomplete
	}
	return owned.identity, nil
}
