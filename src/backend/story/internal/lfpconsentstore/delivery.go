package lfpconsentstore

import (
	"bytes"
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/pkg/storylfpadmission"
)

// ReadConsentForDelivery checks stored exact provenance before any outbound
// registration. A caller's receipt bytes under a new credential are not enough.
func (s *Store) ReadConsentForDelivery(ctx context.Context, id uuid.UUID, hash [32]byte, expected storylfpadmission.Origin) ([]byte, error) {
	if id == uuid.Nil || hash == [32]byte{} || !expected.Valid() {
		return nil, ErrUnavailable
	}
	tx, o, release, e := s.begin(ctx)
	if e != nil {
		return nil, e
	}
	defer release()
	defer func() { _ = tx.Rollback(ctx) }()
	if !sameOrigin(o, expected) {
		return nil, ErrStale
	}
	_, b, h, offer, e := readConsent(ctx, tx, id, true)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, ErrStale
	}
	if e != nil {
		return nil, ErrUnavailable
	}
	if h != hash {
		return nil, ErrConflict
	}
	if offer.provenance != o {
		return nil, ErrStale
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, ErrUnavailable
	}
	return bytes.Clone(b), nil
}

func sameOrigin(o origin, p storylfpadmission.Origin) bool {
	return o.environment == uuid.UUID(p.Environment()) && o.installation == uuid.UUID(p.Installation()) && o.incarnation == uuid.UUID(p.Incarnation()) && o.database == p.DatabaseBinding()
}
