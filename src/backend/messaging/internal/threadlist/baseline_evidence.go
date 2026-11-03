package threadlist

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/pkg/effectivemembership"
)

type lineageEvidence map[uuid.UUID]map[uuid.UUID]bool

func (e lineageEvidence) add(profile, lineage uuid.UUID) {
	if lineage == uuid.Nil {
		return
	}
	if e[profile] == nil {
		e[profile] = make(map[uuid.UUID]bool)
	}
	e[profile][lineage] = true
}
func (e lineageEvidence) contains(profile, lineage uuid.UUID) bool { return e[profile][lineage] }

func membershipRecordMember(e MembershipRecord) effectivemembership.Member {
	m := effectivemembership.Member{ProfileID: e.ProfileID, Revision: e.Revision, Lineage: e.Lineage, Allowed: e.Kind == MemberJoined}
	for _, s := range e.Sources {
		m.Sources = append(m.Sources, effectivemembership.Source(s))
	}
	return m
}

func collectCoveredMembership(ctx context.Context, tx pgx.Tx, c MembershipCheckpoint, q int64, known map[uuid.UUID]knownMembership, retired lineageEvidence) error {
	rows, err := tx.Query(ctx, `SELECT payload FROM thread_list_membership_inbox WHERE chat_id=$1 AND incarnation=$2 AND journal_offset>$3 AND journal_offset<=$4 ORDER BY journal_offset`, c.ChatID, c.Incarnation, c.AppliedQ, q)
	if err != nil {
		return err
	}
	var records []MembershipRecord
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return err
		}
		var e MembershipRecord
		if json.Unmarshal(raw, &e) != nil || e.Validate() != nil || e.ChatID != c.ChatID || e.Incarnation != c.Incarnation || e.Offset <= c.AppliedQ || e.Offset > q {
			rows.Close()
			return ErrMembershipUnavailable
		}
		canonical, marshalErr := canonicalMembership(e)
		if marshalErr != nil || !bytes.Equal(raw, canonical) {
			rows.Close()
			return ErrMembershipUnavailable
		}
		records = append(records, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, e := range records {
		if e.Kind == MemberJoined {
			if retired.contains(e.ProfileID, e.Lineage) {
				return ErrMembershipConflict
			}
			var wasRetired bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM thread_list_retired_lineages WHERE chat_id=$1 AND profile_id=$2 AND lineage=$3)`, e.ChatID, e.ProfileID, e.Lineage).Scan(&wasRetired); err != nil {
				return err
			}
			if wasRetired {
				return ErrMembershipConflict
			}
		}
		m := membershipRecordMember(e)
		if old, ok := known[e.ProfileID]; ok {
			if m.Revision <= old.member.Revision {
				return ErrMembershipConflict
			}
			if _, err = continuousBaselineSources(old.member.Sources, m.Sources); err != nil {
				return err
			}
			if e.Kind == MemberJoined && old.member.Lineage == e.Lineage && e.Lineage != uuid.Nil {
				return ErrMembershipConflict
			}
			if old.member.Lineage != e.Lineage {
				retired.add(e.ProfileID, old.member.Lineage)
			}
		}
		if e.Kind != MemberJoined {
			retired.add(e.ProfileID, e.Lineage)
		}
		known[e.ProfileID] = knownMembership{member: m}
	}
	return nil
}

// Covered deliveries retain exact identity and restrictive history. They can
// strengthen negative metadata, but can never create a grant at the current cut.
func applyCoveredMembershipEvidence(ctx context.Context, tx pgx.Tx, e MembershipRecord) error {
	c, err := readMembershipCheckpoint(ctx, tx, e.ChatID, false)
	if err != nil {
		return err
	}
	known, err := loadKnownMembershipScope(ctx, tx, c, e.ProfileID)
	if err != nil {
		return err
	}
	old, exists := known[e.ProfileID]
	m := membershipRecordMember(e)
	if exists && !old.omitted {
		if e.Revision > old.member.Revision {
			return ErrMembershipConflict
		}
		continuous, compareErr := continuousBaselineSources(m.Sources, old.member.Sources)
		if compareErr != nil {
			return compareErr
		}
		if !continuous && old.member.Allowed && old.member.Lineage == e.Lineage {
			return ErrMembershipConflict
		}
		if e.Revision == old.member.Revision {
			a, err := canonicalBaselineMember(m)
			if err != nil {
				return err
			}
			b, err := canonicalBaselineMember(old.member)
			if err != nil {
				return err
			}
			if !bytes.Equal(a, b) {
				return ErrMembershipConflict
			}
		}
	}
	if e.Kind != MemberJoined || !exists || old.omitted || old.member.Lineage != e.Lineage {
		if err = retireBaselineLineage(ctx, tx, e.ChatID, e.ProfileID, e.Lineage); err != nil {
			return err
		}
	}
	if e.Kind != MemberJoined && exists && old.member.Allowed && old.member.Lineage == e.Lineage {
		return ErrMembershipConflict
	}
	if !exists || (old.omitted && e.Revision > old.member.Revision) {
		if exists {
			if _, err = continuousBaselineSources(old.member.Sources, m.Sources); err != nil {
				return err
			}
			if err = retireBaselineLineage(ctx, tx, e.ChatID, e.ProfileID, old.member.Lineage); err != nil {
				return err
			}
		}
		m.Allowed = false
		return writeBaselinePair(ctx, tx, c.ChatID, c.Incarnation, m, true)
	}
	return nil
}
