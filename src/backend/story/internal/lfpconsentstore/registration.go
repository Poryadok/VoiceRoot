package lfpconsentstore

import (
	"bytes"
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/pkg/lfpconsent"
)

// SaveRegistration records an authenticated MM response supplied by the private
// transport, never a public body. MM provenance is checked by that transport;
// this transaction checks immutable data/cohort equality only.
func (s *Store) SaveRegistration(ctx context.Context, r lfpconsent.Registration, want [32]byte) error {
	b, h, e := lfpconsent.EncodeRegistration(r)
	if e != nil || h != want {
		return ErrUnavailable
	}
	tx, o, release, e := s.begin(ctx)
	if e != nil {
		return e
	}
	defer release()
	defer func() { _ = tx.Rollback(ctx) }()
	if e = checkRegisteredConsent(ctx, tx, o, r); e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `INSERT INTO story_lfp_registration_acks(consent_id,canonical_registration,registration_sha256) VALUES($1,$2,$3) ON CONFLICT(consent_id) DO NOTHING`, r.ConsentID, b, h[:])
	if e != nil {
		return ErrUnavailable
	}
	var saved, hash []byte
	e = tx.QueryRow(ctx, `SELECT canonical_registration,registration_sha256 FROM story_lfp_registration_acks WHERE consent_id=$1`, r.ConsentID).Scan(&saved, &hash)
	if e != nil {
		return ErrUnavailable
	}
	if !bytes.Equal(saved, b) || !bytes.Equal(hash, h[:]) {
		return ErrConflict
	}
	if e = tx.Commit(ctx); e != nil {
		return ErrUnavailable
	}
	return nil
}
func (s *Store) ReadRegistration(ctx context.Context, id uuid.UUID) (lfpconsent.Registration, [32]byte, error) {
	empty := func(e error) (lfpconsent.Registration, [32]byte, error) {
		return lfpconsent.Registration{}, [32]byte{}, e
	}
	tx, o, release, e := s.begin(ctx)
	if e != nil {
		return empty(e)
	}
	defer release()
	defer func() { _ = tx.Rollback(ctx) }()
	var b, hash []byte
	e = tx.QueryRow(ctx, `SELECT canonical_registration,registration_sha256 FROM story_lfp_registration_acks WHERE consent_id=$1`, id).Scan(&b, &hash)
	if e != nil {
		return empty(ErrUnavailable)
	}
	r, h, e := lfpconsent.DecodeRegistration(b)
	if e != nil || r.ConsentID != id || !bytes.Equal(h[:], hash) {
		return empty(ErrUnavailable)
	}
	if e = checkRegisteredConsent(ctx, tx, o, r); e != nil {
		return empty(e)
	}
	if e = tx.Commit(ctx); e != nil {
		return empty(ErrUnavailable)
	}
	return r, h, nil
}
func checkRegisteredConsent(ctx context.Context, tx pgx.Tx, o origin, r lfpconsent.Registration) error {
	if r.OperationID != r.ConsentID {
		return ErrConflict
	}
	_, _, h, offer, e := readConsent(ctx, tx, r.ConsentID, true)
	if errors.Is(e, pgx.ErrNoRows) {
		return ErrConflict
	}
	if e != nil {
		return ErrUnavailable
	}
	if h != r.ConsentSHA256 {
		return ErrConflict
	}
	if offer.provenance != o {
		return ErrStale
	}
	return nil
}
