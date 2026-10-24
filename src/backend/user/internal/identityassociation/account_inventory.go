package identityassociation

import (
	"context"
	"sort"

	"github.com/google/uuid"
)

// PreparedAccountLookup is purpose-bound local material, prepared before I.
// Neither the raw account nor its User-only selectors are an exported payload.
type PreparedAccountLookup struct {
	accountID uuid.UUID
	snapshot  KeysetSnapshot
	aliases   []selectorAlias
}

func (k *Keyset) PrepareAccountLookup(account uuid.UUID) (PreparedAccountLookup, error) {
	if k == nil || account == uuid.Nil || len(k.keys) == 0 {
		return PreparedAccountLookup{}, ErrIncomplete
	}
	versions := make([]uint32, 0, len(k.keys))
	for version := range k.keys {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })
	lookup := PreparedAccountLookup{accountID: account, snapshot: k.Snapshot()}
	for _, version := range versions {
		lookup.aliases = append(lookup.aliases, selectorAlias{version: version, account: selector(k.keys[version], "ACCOUNT", account)})
	}
	return lookup, nil
}

func (PreparedAccountLookup) String() string   { return "User account selector lookup [redacted]" }
func (PreparedAccountLookup) GoString() string { return "User account selector lookup [redacted]" }

// CapturedAccountInventory is only the finite set already represented by the
// canonical store. It neither certifies all historical profiles nor seals an
// Auth terminal roster. Surviving uncaptured rows and lost prehistory are not
// manufactured from this set, even when no unresolved erasure gap is recorded.
// The caller publishes the result only after its IdentityTx commits.
type CapturedAccountInventory struct {
	Cut                     SourceCut
	Facts                   []Fact
	UnresolvedErasureIssued uint64
	VerifiedErasureCovered  uint64
}

func (s *Store) ReadCapturedAccountInventoryTx(ctx context.Context, tx *IdentityTx, lookup PreparedAccountLookup) (CapturedAccountInventory, error) {
	var out CapturedAccountInventory
	if s == nil || s.keys == nil || lookup.accountID == uuid.Nil || lookup.snapshot.Revision != s.keys.snapshot.Revision || lookup.snapshot.ActiveVersion != s.keys.snapshot.ActiveVersion || len(lookup.aliases) != len(s.keys.snapshot.Versions) || len(lookup.snapshot.Versions) != len(s.keys.snapshot.Versions) {
		return out, ErrIncomplete
	}
	for version, fingerprint := range s.keys.snapshot.Versions {
		if actual, ok := lookup.snapshot.Versions[version]; !ok || actual != fingerprint {
			return out, ErrIncomplete
		}
	}
	if err := s.checkKeyset(ctx, tx); err != nil {
		return out, err
	}
	versions := make([]int64, 0, len(lookup.aliases))
	selectors := make([][]byte, 0, len(lookup.aliases))
	expected := map[uint32][32]byte{}
	for _, a := range lookup.aliases {
		versions = append(versions, int64(a.version))
		selectors = append(selectors, a.account[:])
		expected[a.version] = a.account
	}
	rows, err := tx.SQL().Query(ctx, `SELECT stored.association_id,stored.key_version,stored.selector FROM association_selector_aliases stored WHERE stored.kind='ACCOUNT' AND stored.association_id IN (
	 SELECT matched.association_id FROM association_selector_aliases matched JOIN unnest($1::bigint[],$2::bytea[]) requested(version,selector) ON matched.key_version=requested.version AND matched.selector=requested.selector WHERE matched.kind='ACCOUNT') ORDER BY stored.association_id,stored.key_version`, versions, selectors)
	if err != nil {
		return out, err
	}
	var ids []uuid.UUID
	seen := map[uuid.UUID]bool{}
	for rows.Next() {
		var association uuid.UUID
		var version uint32
		var selector []byte
		if err = rows.Scan(&association, &version, &selector); err != nil {
			rows.Close()
			return out, err
		}
		value, required := expected[version]
		if !required || len(selector) != 32 || string(selector) != string(value[:]) {
			rows.Close()
			return out, ErrConflict
		}
		if !seen[association] {
			seen[association] = true
			ids = append(ids, association)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = tx.SQL().Query(ctx, `SELECT `+factColumns+` FROM identity_associations WHERE association_id=ANY($1) ORDER BY association_id`, ids)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		fact, scanErr := scanFact(rows, tx.generation)
		if scanErr != nil {
			rows.Close()
			return out, scanErr
		}
		out.Facts = append(out.Facts, fact)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(out.Facts) != len(ids) {
		return out, ErrIncomplete
	}
	var digest []byte
	err = tx.SQL().QueryRow(ctx, `SELECT m.user_source_generation,a.revision,m.history_generation,m.history_head,m.keyset_revision,m.head_digest,m.retained_floor,m.unresolved_erase_issued,m.unresolved_erase_covered FROM identity_meta m CROSS JOIN user_authority_revision a WHERE m.singleton AND a.singleton`).Scan(&out.Cut.UserSourceGeneration, &out.Cut.UserAuthorityRevision, &out.Cut.HistoryGeneration, &out.Cut.ThroughRevision, &out.Cut.KeysetRevision, &digest, &out.Cut.RetainedFloor, &out.UnresolvedErasureIssued, &out.VerifiedErasureCovered)
	if err != nil {
		return out, err
	}
	if len(digest) != 32 {
		return out, ErrIncomplete
	}
	copy(out.Cut.Digest[:], digest)
	return out, nil
}
