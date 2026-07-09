package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrPreKeyBundleNotFound = errors.New("pre-key bundle not found")

// E2EPreKeyStore persists Signal pre-key bundles per profile (encryption (docs/features/encryption.md)).
type E2EPreKeyStore struct {
	Pool *pgxpool.Pool
}

func (s *E2EPreKeyStore) UpsertBundle(ctx context.Context, profileID uuid.UUID, bundle string) error {
	if s == nil || s.Pool == nil {
		return errors.New("e2e prekey store: pool not configured")
	}
	_, err := s.Pool.Exec(ctx, `
INSERT INTO e2e_prekey_bundles (profile_id, bundle, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (profile_id) DO UPDATE
SET bundle = EXCLUDED.bundle, updated_at = now()
`, profileID, bundle)
	return err
}

func (s *E2EPreKeyStore) GetBundle(ctx context.Context, profileID uuid.UUID) (string, error) {
	if s == nil || s.Pool == nil {
		return "", errors.New("e2e prekey store: pool not configured")
	}
	var bundle string
	err := s.Pool.QueryRow(ctx, `
SELECT bundle FROM e2e_prekey_bundles WHERE profile_id = $1
`, profileID).Scan(&bundle)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrPreKeyBundleNotFound
	}
	return bundle, err
}
