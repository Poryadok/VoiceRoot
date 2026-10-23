package identityassociation

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"sort"
)

type PreparedProfileLookup struct {
	profileID uuid.UUID
	snapshot  KeysetSnapshot
	aliases   []selectorAlias
}

func (k *Keyset) PrepareProfileLookup(profile uuid.UUID) (PreparedProfileLookup, error) {
	if k == nil || profile == uuid.Nil || len(k.keys) == 0 {
		return PreparedProfileLookup{}, ErrIncomplete
	}
	versions := make([]uint32, 0, len(k.keys))
	for version := range k.keys {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })
	lookup := PreparedProfileLookup{profileID: profile, snapshot: k.Snapshot()}
	for _, version := range versions {
		lookup.aliases = append(lookup.aliases, selectorAlias{version: version, profile: selector(k.keys[version], "PROFILE", profile)})
	}
	return lookup, nil
}
func (PreparedProfileLookup) String() string   { return "User profile selector lookup [redacted]" }
func (PreparedProfileLookup) GoString() string { return "User profile selector lookup [redacted]" }
func (s *Store) ReadByProfileSelectorsTx(ctx context.Context, tx *IdentityTx, lookup PreparedProfileLookup) (Fact, error) {
	if s == nil || lookup.profileID == uuid.Nil || lookup.snapshot.Revision != s.keys.snapshot.Revision || lookup.snapshot.ActiveVersion != s.keys.snapshot.ActiveVersion || len(lookup.aliases) != len(s.keys.snapshot.Versions) || len(lookup.snapshot.Versions) != len(s.keys.snapshot.Versions) {
		return Fact{}, ErrIncomplete
	}
	for version, fingerprint := range s.keys.snapshot.Versions {
		if actual, ok := lookup.snapshot.Versions[version]; !ok || actual != fingerprint {
			return Fact{}, ErrIncomplete
		}
	}
	if err := s.checkKeyset(ctx, tx); err != nil {
		return Fact{}, err
	}
	var association uuid.UUID
	for _, a := range lookup.aliases {
		var found uuid.UUID
		err := tx.SQL().QueryRow(ctx, `SELECT association_id FROM association_selector_aliases WHERE key_version=$1 AND kind='PROFILE' AND selector=$2`, a.version, a.profile[:]).Scan(&found)
		if errors.Is(err, pgx.ErrNoRows) {
			return Fact{}, ErrIncomplete
		}
		if err != nil {
			return Fact{}, err
		}
		if association != uuid.Nil && association != found {
			return Fact{}, ErrConflict
		}
		association = found
	}
	if association == uuid.Nil {
		return Fact{}, ErrIncomplete
	}
	return s.readFact(ctx, tx, association)
}
func (s *Store) PrepareTerminalDeleteTx(ctx context.Context, tx *IdentityTx, p PreparedSelectors, expectedRevision uint64) error {
	if s == nil || expectedRevision == 0 {
		return ErrIncomplete
	}
	if err := s.checkPreparation(ctx, tx, p); err != nil {
		return err
	}
	var account uuid.UUID
	var association *uuid.UUID
	err := tx.SQL().QueryRow(ctx, `SELECT account_id,identity_association_id FROM profiles WHERE id=$1 FOR UPDATE`, p.profileID).Scan(&account, &association)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUnknownProfile
	}
	if err != nil {
		return err
	}
	if account != p.accountID {
		return ErrConflict
	}
	if association == nil {
		return ErrIncomplete
	}
	if err = verifyCaptureAliases(ctx, tx, p, *association); err != nil {
		return err
	}
	fact, err := s.readFact(ctx, tx, *association)
	if err != nil {
		return err
	}
	if fact.Revision != expectedRevision {
		return ErrConflict
	}
	if fact.Terminal == nil {
		return ErrIncomplete
	}
	return prepareIdentitySQL(ctx, tx, p.profileID, p.accountID, *association, "TERMINAL_DELETE")
}
