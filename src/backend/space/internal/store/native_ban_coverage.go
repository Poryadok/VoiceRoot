package store

import (
	"context"

	"github.com/google/uuid"
)

// checkKnownNativeBanCoverage is a local complete-known-ordinary subset of276.
// Missing legacy qualification or any community history needs the owner-capture
// and suppression protocol; it must not become a partial successful account ban.
// The caller already holds the shared Space mutation fence and transaction.
func (s *SpaceStore) checkKnownNativeBanCoverage(ctx context.Context, space, account, owner uuid.UUID, hint *uuid.UUID) (bool, error) {
	if s == nil || s.tx == nil {
		return false, ErrMemberAuthorityUnavailable
	}
	var native, tracked bool
	if err := s.db().QueryRow(ctx, `SELECT to_regclass('public.space_ordinary_member_heads') IS NOT NULL,to_regclass('public.schema_migrations') IS NOT NULL`).Scan(&native, &tracked); err != nil {
		return false, ErrMemberAuthorityUnavailable
	}
	if tracked {
		var valid bool
		if err := s.db().QueryRow(ctx, `SELECT count(*)=1 AND bool_and(NOT dirty AND (($1::boolean AND version IN (33,34,35,36)) OR (NOT $1::boolean AND version<33))) FROM public.schema_migrations`, native).Scan(&valid); err != nil || !valid {
			return false, ErrMemberAuthorityUnavailable
		}
	}
	if !native {
		return false, nil // Existing pre33 service, not a native publication path.
	}
	if !tracked {
		return false, ErrMemberAuthorityUnavailable
	}
	var ownerAccount *uuid.UUID
	var legacy bool
	if err := s.db().QueryRow(ctx, `SELECT account_id,legacy FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2`, space, owner).Scan(&ownerAccount, &legacy); err != nil || ownerAccount == nil || legacy {
		return false, ErrMemberAuthorityUnavailable
	}
	if *ownerAccount == account {
		return false, ErrCannotBanOwner
	}
	var unknown bool
	if err := s.db().QueryRow(ctx, `SELECT
	 EXISTS(SELECT 1 FROM space_members m LEFT JOIN space_ordinary_member_heads h
	   ON h.space_id=m.space_id AND h.profile_id=m.profile_id WHERE m.space_id=$1
	   AND (h.profile_id IS NULL OR h.state<>'ACTIVE' OR h.legacy OR h.account_id IS NULL
	     OR m.membership_generation IS DISTINCT FROM h.generation OR m.admission_operation_id IS DISTINCT FROM h.publication_id))
	 OR EXISTS(SELECT 1 FROM space_ordinary_member_heads h WHERE h.space_id=$1 AND h.state IN ('ACTIVE','PREPARING')
	   AND (h.account_id IS NULL OR h.legacy
	     OR (h.state='ACTIVE' AND NOT EXISTS(SELECT 1 FROM space_members m WHERE m.space_id=h.space_id AND m.profile_id=h.profile_id))
	     OR (h.state='PREPARING' AND NOT EXISTS(SELECT 1 FROM space_member_admission_intents i
	       WHERE i.operation_id=h.publication_id AND i.space_id=h.space_id AND i.profile_id=h.profile_id
	       AND i.account_id=h.account_id AND i.generation=h.generation AND i.state='PREPARING' AND i.binding_sha256=h.admission_binding_sha256))))
	 OR EXISTS(SELECT 1 FROM community_owner_authority WHERE space_id=$1)
	 OR EXISTS(SELECT 1 FROM community_roster_members WHERE space_id=$1)
	 OR EXISTS(SELECT 1 FROM space_community_member_heads WHERE space_id=$1)
	 OR NOT EXISTS(SELECT 1 FROM space_membership_ownership_heads WHERE space_id=$1 AND owner_profile_id=$2 AND state='ACTIVE')`, space, owner).Scan(&unknown); err != nil || unknown {
		return false, ErrMemberAuthorityUnavailable
	}
	if hint != nil {
		var selected *uuid.UUID
		if err := s.db().QueryRow(ctx, `SELECT account_id FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2 AND NOT legacy`, space, *hint).Scan(&selected); err != nil || selected == nil {
			return false, ErrMemberAuthorityUnavailable
		}
		if *selected != account {
			return false, ErrMemberAdmissionConflict
		}
	}
	return true, nil
}
