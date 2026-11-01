package identityassociation

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FreshSourcePin comes from the creation receipt of an owned new User source.
// Reading these values from an existing database is not recovery provenance.
type FreshSourcePin struct {
	HistoryGeneration    uuid.UUID
	UserSourceGeneration uuid.UUID
}

// NewFreshStore provisions the complete, caller-supplied User-only keyset only
// on a pristine source with both independently pinned creation generations.
// It cannot reopen or adopt an existing source and does not activate guards.
func NewFreshStore(ctx context.Context, pool *pgxpool.Pool, keys *Keyset, pin FreshSourcePin) (*Store, error) {
	return newPristineStore(ctx, pool, keys, pin.HistoryGeneration, pin.UserSourceGeneration)
}

// MigrationSourcePin is independently recorded during an exclusive, fenced
// schema migration. It identifies new identity history, not a continuity or
// guard-activation assertion for the pre-existing source.
type MigrationSourcePin struct {
	HistoryGeneration    uuid.UUID
	UserSourceGeneration uuid.UUID
}

// NewMigratedStore provisions a pristine identity domain after the reviewed
// schema migration; existing profile rows are retained. It cannot authorize
// source reopening or activate writer guards.
func NewMigratedStore(ctx context.Context, pool *pgxpool.Pool, keys *Keyset, pin MigrationSourcePin) (*Store, error) {
	return newPristineStore(ctx, pool, keys, pin.HistoryGeneration, pin.UserSourceGeneration)
}

func newPristineStore(ctx context.Context, pool *pgxpool.Pool, keys *Keyset, historyGeneration, userSourceGeneration uuid.UUID) (*Store, error) {
	if historyGeneration == uuid.Nil || userSourceGeneration == uuid.Nil || historyGeneration == userSourceGeneration || keys == nil || keys.snapshot.Revision != 1 {
		return nil, ErrIncomplete
	}
	s, err := NewStore(pool, keys, historyGeneration)
	if err != nil {
		return nil, err
	}
	tx, err := BeginIdentityTx(ctx, pool, &ExpectedFence{Generation: historyGeneration, KeysetRevision: 0})
	if err != nil {
		return nil, ErrIncomplete
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var pristine bool
	err = tx.SQL().QueryRow(ctx, `SELECT user_source_generation=$1 AND history_generation=$2
		AND history_head=0 AND retained_floor=0 AND keyset_revision=0 AND active_key_version IS NULL
		AND NOT guards_active AND baseline_cut=0 AND unresolved_erase_issued=0 AND unresolved_erase_covered=0
		AND NOT EXISTS(SELECT 1 FROM identity_key_versions)
		AND NOT EXISTS(SELECT 1 FROM identity_associations)
		AND NOT EXISTS(SELECT 1 FROM identity_profile_reservations)
		AND NOT EXISTS(SELECT 1 FROM identity_journal)
		AND NOT EXISTS(SELECT 1 FROM association_selector_aliases)
		AND NOT EXISTS(SELECT 1 FROM identity_account_parent_aliases)
		AND NOT EXISTS(SELECT 1 FROM identity_parent_fences)
		FROM identity_meta WHERE singleton`, userSourceGeneration, historyGeneration).Scan(&pristine)
	if err != nil {
		return nil, err
	}
	if !pristine {
		return nil, ErrIncomplete
	}
	for version, fingerprint := range keys.snapshot.Versions {
		if _, err = tx.SQL().Exec(ctx, `INSERT INTO identity_key_versions(version,fingerprint) VALUES($1,$2)`, version, fingerprint[:]); err != nil {
			return nil, err
		}
	}
	if _, err = tx.SQL().Exec(ctx, `UPDATE identity_meta SET keyset_revision=$1,active_key_version=$2 WHERE singleton`, keys.snapshot.Revision, keys.snapshot.ActiveVersion); err != nil {
		return nil, err
	}
	if _, err = tx.SQL().Exec(ctx, `SELECT identity_append_event(NULL,NULL,'KEYSET',NULL,NULL,NULL)`); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s, nil
}
