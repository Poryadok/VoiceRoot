package threadlist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/pkg/effectivemembership"
)

type knownMembership struct {
	member  effectivemembership.Member
	omitted bool
}

func canonicalBaselineMember(m effectivemembership.Member) ([]byte, error) {
	m.Sources = effectivemembership.CanonicalSources(m.Sources)
	return json.Marshal(m)
}

func loadKnownMembership(ctx context.Context, tx pgx.Tx, c MembershipCheckpoint) (map[uuid.UUID]knownMembership, error) {
	return loadKnownMembershipScope(ctx, tx, c, uuid.Nil)
}

func loadKnownMembershipScope(ctx context.Context, tx pgx.Tx, c MembershipCheckpoint, profile uuid.UUID) (map[uuid.UUID]knownMembership, error) {
	known := make(map[uuid.UUID]knownMembership)
	suffix := ""
	args := []any{c.ChatID}
	if profile != uuid.Nil {
		suffix = " AND profile_id=$2"
		args = append(args, profile)
	}
	rows, err := tx.Query(ctx, `SELECT profile_id,incarnation,member_state,omitted FROM thread_list_membership_snapshot_pairs WHERE chat_id=$1`+suffix, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, inc uuid.UUID
		var raw []byte
		var k knownMembership
		if err = rows.Scan(&id, &inc, &raw, &k.omitted); err != nil {
			rows.Close()
			return nil, err
		}
		if json.Unmarshal(raw, &k.member) != nil || inc != c.Incarnation || id != k.member.ProfileID || (k.omitted && k.member.Allowed) || effectivemembership.ValidateSources(c.ChatID, id, k.member.Sources) != nil {
			rows.Close()
			return nil, ErrMembershipUnavailable
		}
		canonical, marshalErr := canonicalBaselineMember(k.member)
		if marshalErr != nil || !bytes.Equal(raw, canonical) {
			rows.Close()
			return nil, ErrMembershipUnavailable
		}
		known[id] = k
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	type pair struct {
		id    uuid.UUID
		state MembershipPair
	}
	var pairs []pair
	rows, err = tx.Query(ctx, `SELECT profile_id,membership_revision,lineage,status FROM thread_list_membership_pairs WHERE chat_id=$1`+suffix, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p pair
		if err = rows.Scan(&p.id, &p.state.Revision, &p.state.Lineage, &p.state.Status); err != nil {
			rows.Close()
			return nil, err
		}
		pairs = append(pairs, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, p := range pairs {
		if old, ok := known[p.id]; ok && old.member.Revision == p.state.Revision {
			if old.member.Lineage != p.state.Lineage || old.member.Allowed != (p.state.Status == MemberBuilding) {
				return nil, ErrMembershipUnavailable
			}
			continue
		} else if ok && old.member.Revision > p.state.Revision {
			return nil, ErrMembershipUnavailable
		}
		var raw []byte
		err = tx.QueryRow(ctx, `SELECT payload FROM thread_list_membership_inbox WHERE chat_id=$1 AND incarnation=$2 AND profile_id=$3 AND membership_revision=$4`, c.ChatID, c.Incarnation, p.id, p.state.Revision).Scan(&raw)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMembershipUnavailable
		}
		if err != nil {
			return nil, err
		}
		var record MembershipRecord
		if json.Unmarshal(raw, &record) != nil || record.Validate() != nil || record.ChatID != c.ChatID || record.Incarnation != c.Incarnation || record.ProfileID != p.id || record.Revision != p.state.Revision || record.Lineage != p.state.Lineage {
			return nil, ErrMembershipUnavailable
		}
		canonical, marshalErr := canonicalMembership(record)
		if marshalErr != nil || !bytes.Equal(raw, canonical) {
			return nil, ErrMembershipUnavailable
		}
		m := effectivemembership.Member{ProfileID: p.id, Revision: p.state.Revision, Lineage: p.state.Lineage, Allowed: p.state.Status == MemberBuilding}
		for _, source := range record.Sources {
			m.Sources = append(m.Sources, effectivemembership.Source(source))
		}
		known[p.id] = knownMembership{member: m}
	}
	return known, nil
}

func checkBaselineTransition(old knownMembership, next effectivemembership.Member, reset bool) error {
	previous := old.member
	if reset {
		if next.Allowed && previous.Lineage != uuid.Nil && next.Lineage == previous.Lineage {
			return ErrMembershipUnavailable
		}
		return nil
	}
	if next.Revision < previous.Revision {
		return ErrMembershipConflict
	}
	if next.Revision == previous.Revision {
		a, err := canonicalBaselineMember(previous)
		if err != nil {
			return err
		}
		b, err := canonicalBaselineMember(next)
		if err != nil {
			return err
		}
		if !bytes.Equal(a, b) {
			return ErrMembershipConflict
		}
		return nil
	}
	continuous, err := continuousBaselineSources(previous.Sources, next.Sources)
	if err != nil {
		return err
	}
	if next.Allowed && (!previous.Allowed || old.omitted || !continuous) && next.Lineage == previous.Lineage {
		return ErrMembershipUnavailable
	}
	return nil
}

func continuousBaselineSources(old, next []effectivemembership.Source) (bool, error) {
	type key struct {
		owner string
		scope uuid.UUID
	}
	current := make(map[key]effectivemembership.Source, len(next))
	for _, s := range next {
		current[key{s.Owner, s.Scope}] = s
	}
	continuous := len(old) == len(next)
	for _, before := range old {
		after, ok := current[key{before.Owner, before.Scope}]
		if !ok || before.Incarnation != after.Incarnation {
			continuous = false
			continue
		}
		if after.Revision < before.Revision || after.RevocationRevision < before.RevocationRevision || (after.Revision == before.Revision && after.RevocationRevision != before.RevocationRevision) {
			return false, ErrMembershipConflict
		}
		if after.RevocationRevision != before.RevocationRevision {
			continuous = false
		}
	}
	return continuous, nil
}
