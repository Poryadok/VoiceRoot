package identityassociation

import (
	"bytes"
	"context"
)

// Bootstrap initializes only a pristine, explicitly provisioned source. An
// existing source must match the complete configured keyset; missing retained
// fingerprints are not repaired by initialization.
func (s *Store) Bootstrap(ctx context.Context) error {
	if s == nil {
		return ErrIncomplete
	}
	var revision uint64
	if err := s.pool.QueryRow(ctx, `SELECT keyset_revision FROM identity_meta WHERE singleton`).Scan(&revision); err != nil {
		return err
	}
	tx, err := BeginIdentityTx(ctx, s.pool, &ExpectedFence{Generation: s.generation, KeysetRevision: revision})
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if revision != 0 {
		if err = s.checkKeyset(ctx, tx); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if s.keys.snapshot.Revision != 1 {
		return ErrIncomplete
	}
	var pristine bool
	err = tx.SQL().QueryRow(ctx, `SELECT history_head=0 AND NOT guards_active AND baseline_cut=0 AND NOT EXISTS(SELECT 1 FROM identity_key_versions) AND NOT EXISTS(SELECT 1 FROM identity_associations) AND NOT EXISTS(SELECT 1 FROM identity_profile_reservations) AND NOT EXISTS(SELECT 1 FROM identity_journal) FROM identity_meta WHERE singleton`).Scan(&pristine)
	if err != nil {
		return err
	}
	if !pristine {
		return ErrIncomplete
	}
	for version, fingerprint := range s.keys.snapshot.Versions {
		if _, err = tx.SQL().Exec(ctx, `INSERT INTO identity_key_versions(version,fingerprint) VALUES($1,$2)`, version, fingerprint[:]); err != nil {
			return err
		}
	}
	if _, err = tx.SQL().Exec(ctx, `UPDATE identity_meta SET keyset_revision=$1,active_key_version=$2 WHERE singleton`, s.keys.snapshot.Revision, s.keys.snapshot.ActiveVersion); err != nil {
		return err
	}
	if _, err = tx.SQL().Exec(ctx, `SELECT identity_append_event(NULL,NULL,'KEYSET',NULL,NULL,NULL)`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) checkKeyset(ctx context.Context, tx *IdentityTx) error {
	if s == nil || tx == nil || tx.SQL() == nil || tx.generation != s.generation {
		return ErrIncomplete
	}
	var revision uint64
	var active uint32
	var intact bool
	err := tx.SQL().QueryRow(ctx, `SELECT keyset_revision,active_key_version,history_head>0 AND EXISTS(SELECT 1 FROM identity_journal j WHERE j.revision=m.history_head AND j.digest=m.head_digest) FROM identity_meta m WHERE singleton`).Scan(&revision, &active, &intact)
	if err != nil {
		return err
	}
	if !intact || revision != s.keys.snapshot.Revision || active != s.keys.snapshot.ActiveVersion {
		return ErrIncomplete
	}
	rows, err := tx.SQL().Query(ctx, `SELECT version,fingerprint FROM identity_key_versions ORDER BY version`)
	if err != nil {
		return err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var version uint32
		var fingerprint []byte
		if err = rows.Scan(&version, &fingerprint); err != nil {
			return err
		}
		expected, ok := s.keys.snapshot.Versions[version]
		if !ok || !bytes.Equal(expected[:], fingerprint) {
			return ErrIncomplete
		}
		count++
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if count != len(s.keys.snapshot.Versions) {
		return ErrIncomplete
	}
	return nil
}

func prepareIdentitySQL(ctx context.Context, tx *IdentityTx, profile, account, association any, kind string) error {
	if _, err := tx.SQL().Exec(ctx, `CREATE TEMP TABLE IF NOT EXISTS user_identity_preparation(transaction_id bigint NOT NULL,profile_id uuid,account_id uuid,association_id uuid,kind text NOT NULL) ON COMMIT DROP`); err != nil {
		return err
	}
	_, err := tx.SQL().Exec(ctx, `INSERT INTO pg_temp.user_identity_preparation VALUES(txid_current(),$1,$2,$3,$4)`, profile, account, association, kind)
	return err
}

// ActivateGuards is a local cutover primitive, not a deployment readiness
// assertion. Production activation additionally requires the complete writer
// integration, catalog and native proof inventory.
func (s *Store) ActivateGuards(ctx context.Context) error {
	tx, err := s.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err = s.checkKeyset(ctx, tx); err != nil {
		return err
	}
	var active bool
	var next uint64
	if err = tx.SQL().QueryRow(ctx, `SELECT guards_active,history_head+1 FROM identity_meta WHERE singleton`).Scan(&active, &next); err != nil {
		return err
	}
	if active {
		return tx.Commit(ctx)
	}
	if err = prepareIdentitySQL(ctx, tx, nil, nil, nil, "ACTIVATE"); err != nil {
		return err
	}
	if _, err = tx.SQL().Exec(ctx, `UPDATE profiles SET identity_baseline_cut=$1 WHERE identity_association_id IS NULL`, next); err != nil {
		return err
	}
	if _, err = tx.SQL().Exec(ctx, `UPDATE identity_meta SET guards_active=true,baseline_cut=$1 WHERE singleton`, next); err != nil {
		return err
	}
	if _, err = tx.SQL().Exec(ctx, `SELECT identity_append_event(NULL,NULL,'GUARDS_ACTIVATED',NULL,NULL,NULL)`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
