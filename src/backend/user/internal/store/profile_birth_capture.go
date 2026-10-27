package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/user/internal/identityassociation"
)

type profileBirthCapture struct {
	identities *identityassociation.Store
	parents    *identityassociation.ParentVerifier
}

// NewProfileStoreWithBirthCapture composes existing User-owned birth primitives.
// The authority is trusted startup configuration, never caller-supplied proof.
func NewProfileStoreWithBirthCapture(pool *pgxpool.Pool, identities *identityassociation.Store, parents *identityassociation.ParentVerifier) (*ProfileStore, error) {
	if pool == nil || !identities.OwnsPool(pool) || parents == nil {
		return nil, identityassociation.ErrIncomplete
	}
	return &ProfileStore{pool: pool, birthCapture: &profileBirthCapture{identities: identities, parents: parents}}, nil
}

func (s *ProfileStore) createSecondaryProfileWithBirth(ctx context.Context, account uuid.UUID, displayName string, usernameHint, accentColor *string) (*ProfileRow, error) {
	boundary := s.birthCapture
	proof, err := boundary.parents.Verify(ctx, account)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := 0; attempt < maxDiscriminatorAttempts; attempt++ {
		id := uuid.New()
		prepared, err := boundary.identities.PrepareBirthSelectors(id, account)
		if err != nil {
			return nil, err
		}
		tx, err := boundary.identities.Begin(ctx)
		if err != nil {
			return nil, err
		}
		_, err = boundary.identities.CreateBirthTx(ctx, tx, prepared, proof)
		var created *ProfileRow
		if err == nil {
			created, err = s.insertSecondaryProfileTx(ctx, tx.SQL(), id, account, displayName, usernameHint, accentColor)
		}
		if err == nil {
			event, eventErr := searchProjectionForProfile(ctx, tx.SQL(), created)
			err = eventErr
			if err == nil {
				err = AppendSearchProjection(ctx, tx.SQL(), event)
			}
		}
		if err == nil {
			err = tx.Commit(ctx)
		}
		if err == nil {
			return created, nil
		}
		_ = tx.Rollback(context.Background())
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			lastErr = err
			continue // A PostgreSQL error requires a fresh owning transaction.
		}
		return nil, err
	}
	return nil, fmt.Errorf("username/discriminator exhausted: %w", lastErr)
}
