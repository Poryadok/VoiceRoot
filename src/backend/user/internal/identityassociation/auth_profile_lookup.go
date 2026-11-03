package identityassociation

import (
	"bytes"
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

// AuthProfileSource is the qualified stream's binding, not an authority grant.
// The caller must hold that stream through the complete call and commit.
type AuthProfileSource struct {
	HistoryGeneration, UserSourceGeneration uuid.UUID
	KeysetDigest                            [32]byte
	Physical                                PhysicalSourceIdentity
}
type AuthProfileResult struct {
	ProfileID                        uuid.UUID
	Match                            string
	Fact                             Fact
	CurrentAccount                   uuid.UUID
	CurrentRevision                  uint64
	Deleted, Frozen, AccountInactive bool
}
type AuthProfileBatch struct {
	Cut                                             SourceCut
	ObservedAt                                      time.Time
	UnresolvedErasureIssued, VerifiedErasureCovered uint64
	Results                                         []AuthProfileResult
}

// ReadAuthProfiles reads only supplied identifiers; zero account selects the
// single current-target observation mode. It never captures a missing fact.
func (s *Store) ReadAuthProfiles(ctx context.Context, account uuid.UUID, ids []uuid.UUID, source AuthProfileSource) (AuthProfileBatch, error) {
	var out AuthProfileBatch
	if s == nil || s.keys == nil || ctx == nil || len(ids) == 0 || len(ids) > 256 || (account == uuid.Nil && len(ids) != 1) {
		return out, ErrIncomplete
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	lookups := make([]PreparedProfileLookup, len(ids))
	for i, id := range ids {
		if id == uuid.Nil || (i > 0 && bytes.Compare(ids[i-1][:], id[:]) >= 0) {
			return out, ErrIncomplete
		}
		var err error
		lookups[i], err = s.keys.PrepareProfileLookup(id)
		if err != nil {
			return out, err
		}
	}
	var accountLookup PreparedAccountLookup
	if account != uuid.Nil {
		var err error
		accountLookup, err = s.keys.PrepareAccountLookup(account)
		if err != nil {
			return out, err
		}
	}
	// Non-authoritative preparation hint only. The locked read must match it.
	var targetAccount uuid.UUID
	if account == uuid.Nil {
		err := s.pool.QueryRow(bounded, `SELECT account_id FROM profiles WHERE id=$1`, ids[0]).Scan(&targetAccount)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return out, err
		}
		if err == nil {
			accountLookup, err = s.keys.PrepareAccountLookup(targetAccount)
			if err != nil {
				return out, err
			}
		}
	}
	tx, err := s.Begin(bounded)
	if err != nil {
		return out, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = s.ValidateForActivationTx(bounded, tx, source.HistoryGeneration, source.UserSourceGeneration, source.KeysetDigest, source.Physical); err != nil {
		return out, err
	}
	for i, lookup := range lookups {
		result := AuthProfileResult{ProfileID: ids[i], Match: "INCOMPLETE"}
		fact, readErr := s.ReadByProfileSelectorsTx(bounded, tx, lookup)
		if errors.Is(readErr, ErrIncomplete) || errors.Is(readErr, ErrConflict) || errors.Is(readErr, pgx.ErrNoRows) {
			out.Results = append(out.Results, result)
			continue
		}
		if readErr != nil {
			return AuthProfileBatch{}, readErr
		}
		if account != uuid.Nil {
			result.Match, err = s.matchAuthAccountTx(bounded, tx, fact.AssociationID, accountLookup)
			if err != nil {
				return AuthProfileBatch{}, err
			}
			if result.Match != "INCOMPLETE" {
				result.Fact = fact
			}
		} else {
			// The raw tuple is private, current User evidence. It is never reconstructed
			// from a retired association and never grants concurrent profile admission.
			var association *uuid.UUID
			err = tx.SQL().QueryRow(bounded, `SELECT account_id,identity_association_id,sdk_eligibility_revision,deleted_at IS NOT NULL,frozen_at IS NOT NULL,EXISTS(SELECT 1 FROM user_account_lifecycle l WHERE l.account_id=p.account_id AND l.state='ACCOUNT_INACTIVE') FROM profiles p WHERE id=$1`, ids[i]).Scan(&result.CurrentAccount, &association, &result.CurrentRevision, &result.Deleted, &result.Frozen, &result.AccountInactive)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return AuthProfileBatch{}, err
			}
			if fact.Terminal != nil {
				result.Fact = fact
				result.CurrentAccount = uuid.Nil
				result.CurrentRevision = 0
			} else if err == nil && result.CurrentAccount == targetAccount && targetAccount != uuid.Nil && association != nil && *association == fact.AssociationID && result.CurrentRevision > 0 {
				// A captured PROFILE alias alone does not establish the raw account tuple.
				match, matchErr := s.matchAuthAccountTx(bounded, tx, fact.AssociationID, accountLookup)
				if matchErr != nil {
					return AuthProfileBatch{}, matchErr
				}
				if match == "MATCH" {
					result.Fact = fact
				} else {
					result.CurrentAccount = uuid.Nil
					result.CurrentRevision = 0
				}
			} else {
				result.CurrentAccount = uuid.Nil
				result.CurrentRevision = 0
			}
		}
		out.Results = append(out.Results, result)
	}
	var digest []byte
	err = tx.SQL().QueryRow(bounded, `SELECT m.user_source_generation,a.revision,m.history_generation,m.history_head,m.keyset_revision,m.head_digest,m.retained_floor,m.unresolved_erase_issued,m.unresolved_erase_covered,clock_timestamp() FROM identity_meta m CROSS JOIN user_authority_revision a WHERE m.singleton AND a.singleton`).Scan(&out.Cut.UserSourceGeneration, &out.Cut.UserAuthorityRevision, &out.Cut.HistoryGeneration, &out.Cut.ThroughRevision, &out.Cut.KeysetRevision, &digest, &out.Cut.RetainedFloor, &out.UnresolvedErasureIssued, &out.VerifiedErasureCovered, &out.ObservedAt)
	if err != nil {
		return AuthProfileBatch{}, err
	}
	if len(digest) != 32 {
		return AuthProfileBatch{}, ErrIncomplete
	}
	copy(out.Cut.Digest[:], digest)
	if err = tx.Commit(bounded); err != nil {
		return AuthProfileBatch{}, err
	}
	return out, nil
}

func (s *Store) matchAuthAccountTx(ctx context.Context, tx *IdentityTx, association uuid.UUID, lookup PreparedAccountLookup) (string, error) {
	rows, err := tx.SQL().Query(ctx, `SELECT key_version,selector FROM association_selector_aliases WHERE association_id=$1 AND kind='ACCOUNT' ORDER BY key_version`, association)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	expected := make(map[uint32][32]byte, len(lookup.aliases))
	for _, alias := range lookup.aliases {
		expected[alias.version] = alias.account
	}
	seen, matches := 0, 0
	for rows.Next() {
		var version uint32
		var selector []byte
		if err = rows.Scan(&version, &selector); err != nil {
			return "", err
		}
		want, ok := expected[version]
		if !ok || len(selector) != 32 {
			return "INCOMPLETE", nil
		}
		seen++
		if bytes.Equal(want[:], selector) {
			matches++
		}
	}
	if err = rows.Err(); err != nil {
		return "", err
	}
	if seen != len(expected) || seen == 0 {
		return "INCOMPLETE", nil
	}
	if matches == seen {
		return "MATCH", nil
	}
	if matches == 0 {
		return "DIFFERENT_ACCOUNT", nil
	}
	return "INCOMPLETE", nil
}
