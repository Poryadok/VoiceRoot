package threadlist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrMembershipUnavailable = errors.New("thread membership unavailable")

// MembershipInbox persists Chat's committed Q prefix separately from Messaging's
// local message/visibility journal J. It never establishes read authorization.
type MembershipInbox struct{ Pool *pgxpool.Pool }

// Replay resumes a durable tail without depending on another broker delivery.
// complete is false for either a gap or more than one bounded batch of work.
// A true result covers received events only. A builder must separately require
// applied Q to reach a fresh authoritative Chat head/admission; an unseen tail
// cannot be detected from this inbox. Replay never proves READY or permission.
func (s MembershipInbox) Replay(ctx context.Context, chat, incarnation uuid.UUID) (complete bool, err error) {
	if s.Pool == nil || chat == uuid.Nil || incarnation == uuid.Nil {
		return false, ErrMembershipUnavailable
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var current uuid.UUID
	var applied int64
	var quarantined bool
	err = tx.QueryRow(ctx, `SELECT incarnation,applied_q,quarantined FROM thread_list_membership_heads
WHERE chat_id=$1 FOR UPDATE`, chat).Scan(&current, &applied, &quarantined)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrMembershipUnavailable
	}
	if err != nil {
		return false, err
	}
	if current != incarnation || quarantined {
		return false, ErrMembershipUnavailable
	}
	if err = replayMembership(ctx, tx, chat, incarnation, applied); err != nil {
		return false, err
	}
	err = tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM thread_list_membership_inbox i
WHERE i.chat_id=h.chat_id AND i.incarnation=h.incarnation AND i.journal_offset>h.applied_q)
FROM thread_list_membership_heads h WHERE h.chat_id=$1`, chat).Scan(&complete)
	if err != nil {
		return false, err
	}
	return complete, tx.Commit(ctx)
}

func (s MembershipInbox) Track(ctx context.Context, chat, incarnation uuid.UUID) error {
	if s.Pool == nil || chat == uuid.Nil || incarnation == uuid.Nil {
		return ErrMembershipUnavailable
	}
	// Only an explicitly reconciled source may register its incarnation. A
	// delayed event cannot reset an existing prefix or clear quarantine.
	_, err := s.Pool.Exec(ctx, `INSERT INTO thread_list_membership_heads(chat_id,incarnation)
VALUES($1,$2) ON CONFLICT(chat_id) DO NOTHING`, chat, incarnation)
	if err != nil {
		return err
	}
	var same bool
	err = s.Pool.QueryRow(ctx, `SELECT incarnation=$2 AND NOT quarantined FROM thread_list_membership_heads WHERE chat_id=$1`, chat, incarnation).Scan(&same)
	if err != nil {
		return err
	}
	if !same {
		return ErrMembershipUnavailable
	}
	return nil
}

func (s MembershipInbox) Apply(ctx context.Context, e MembershipRecord) error {
	if e.ChatID == uuid.Nil || e.Incarnation == uuid.Nil {
		return ErrInvalidMembership
	}
	if s.Pool == nil {
		return ErrMembershipUnavailable
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var incarnation uuid.UUID
	var applied int64
	var quarantined bool
	err = tx.QueryRow(ctx, `SELECT incarnation,applied_q,quarantined FROM thread_list_membership_heads
WHERE chat_id=$1 FOR UPDATE`, e.ChatID).Scan(&incarnation, &applied, &quarantined)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrMembershipUnavailable
	}
	if err != nil {
		return err
	}
	if incarnation != e.Incarnation || quarantined {
		return ErrMembershipUnavailable
	}
	payload, err := canonicalMembership(e)
	if err != nil {
		return err
	}
	// Check all identities before INSERT. The chat head lock serializes every
	// delivery for this scope, including conflicting offset/event/pair identities.
	rows, err := tx.Query(ctx, `SELECT payload FROM thread_list_membership_inbox
WHERE chat_id=$1 AND incarnation=$2 AND
(event_id=$3 OR journal_offset=$4 OR (profile_id=$5 AND membership_revision=$6))`,
		e.ChatID, e.Incarnation, e.EventID, e.Offset, e.ProfileID, e.Revision)
	if err != nil {
		return err
	}
	found, conflict := false, false
	for rows.Next() {
		var old []byte
		if err = rows.Scan(&old); err != nil {
			rows.Close()
			return err
		}
		found = true
		conflict = conflict || !bytes.Equal(old, payload)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if conflict {
		return quarantineMembership(ctx, tx, e.ChatID)
	}
	// A malformed delivery with an already retained identity is a conflict;
	// only a novel invalid record may be rejected without poisoning the scope.
	if err = e.Validate(); err != nil {
		return err
	}
	if !found {
		// With complete origin replay, an unseen event at/below the applied
		// prefix proves missing/conflicting identity history, never a stale no-op.
		if e.Offset <= applied {
			return quarantineMembership(ctx, tx, e.ChatID)
		}
		_, err = tx.Exec(ctx, `INSERT INTO thread_list_membership_inbox
(chat_id,incarnation,journal_offset,event_id,profile_id,membership_revision,payload)
VALUES($1,$2,$3,$4,$5,$6,$7)`, e.ChatID, e.Incarnation, e.Offset, e.EventID, e.ProfileID, e.Revision, payload)
		if err != nil {
			return err
		}
	}
	if err = replayMembership(ctx, tx, e.ChatID, incarnation, applied); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func replayMembership(ctx context.Context, tx pgx.Tx, chat, incarnation uuid.UUID, applied int64) error {
	// Bounded replay can continue on exact redelivery or explicit Replay. Persisting a received
	// tail is safe, but Pair refuses reads until the entire received prefix is
	// consumed. A high pair revision is never substituted for complete chat Q.
	for n := 0; n < 256 && applied < math.MaxInt64; n++ {
		var raw []byte
		err := tx.QueryRow(ctx, `SELECT payload FROM thread_list_membership_inbox
WHERE chat_id=$1 AND incarnation=$2 AND journal_offset=$3`, chat, incarnation, applied+1).Scan(&raw)
		if errors.Is(err, pgx.ErrNoRows) {
			break
		}
		if err != nil {
			return err
		}
		var next MembershipRecord
		if json.Unmarshal(raw, &next) != nil || next.Validate() != nil || next.ChatID != chat || next.Incarnation != incarnation || next.Offset != applied+1 {
			return quarantineMembership(ctx, tx, chat)
		}
		canonical, marshalErr := canonicalMembership(next)
		if marshalErr != nil || !bytes.Equal(raw, canonical) {
			return quarantineMembership(ctx, tx, chat)
		}
		if err = applyMembershipPair(ctx, tx, next); errors.Is(err, ErrMembershipConflict) {
			return quarantineMembership(ctx, tx, chat)
		} else if err != nil {
			return err
		}
		applied++
		_, err = tx.Exec(ctx, `UPDATE thread_list_membership_heads SET applied_q=$2 WHERE chat_id=$1`, chat, applied)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s MembershipInbox) Pair(ctx context.Context, chat, incarnation, profile uuid.UUID, requiredQ int64) (MembershipPair, error) {
	var p MembershipPair
	if s.Pool == nil || chat == uuid.Nil || incarnation == uuid.Nil || profile == uuid.Nil || requiredQ < 0 {
		return p, ErrMembershipUnavailable
	}
	err := s.Pool.QueryRow(ctx, `SELECT p.membership_revision,p.lineage,p.status
FROM thread_list_membership_heads h JOIN thread_list_membership_pairs p USING(chat_id)
WHERE h.chat_id=$1 AND h.incarnation=$2 AND p.profile_id=$3 AND NOT h.quarantined AND h.applied_q >= $4
AND NOT EXISTS(SELECT 1 FROM thread_list_membership_inbox i WHERE i.chat_id=h.chat_id AND i.incarnation=h.incarnation AND i.journal_offset>h.applied_q)`,
		chat, incarnation, profile, requiredQ).Scan(&p.Revision, &p.Lineage, &p.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return MembershipPair{}, ErrMembershipUnavailable
	}
	return p, err
}

func quarantineMembership(ctx context.Context, tx pgx.Tx, chat uuid.UUID) error {
	// Commit the quarantine instead of returning an error that would roll it
	// back. A transport retry/restart cannot silently restore this scope.
	if _, err := tx.Exec(ctx, `UPDATE thread_list_membership_heads SET quarantined=true WHERE chat_id=$1`, chat); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return ErrMembershipConflict
}

func applyMembershipPair(ctx context.Context, tx pgx.Tx, e MembershipRecord) error {
	var previous MembershipPair
	err := tx.QueryRow(ctx, `SELECT membership_revision,lineage,status FROM thread_list_membership_pairs
WHERE chat_id=$1 AND profile_id=$2`, e.ChatID, e.ProfileID).Scan(&previous.Revision, &previous.Lineage, &previous.Status)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	next, err := previous.Next(e)
	if err != nil {
		return err
	}
	if e.Kind == MemberJoined {
		var retired bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM thread_list_retired_lineages
WHERE chat_id=$1 AND profile_id=$2 AND lineage=$3)`, e.ChatID, e.ProfileID, e.Lineage).Scan(&retired); err != nil {
			return err
		}
		if retired {
			return ErrMembershipConflict
		}
	} else {
		if _, err = tx.Exec(ctx, `INSERT INTO thread_list_retired_lineages(chat_id,profile_id,lineage)
VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, e.ChatID, e.ProfileID, e.Lineage); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO thread_list_membership_pairs(chat_id,profile_id,membership_revision,lineage,status)
VALUES($1,$2,$3,$4,$5) ON CONFLICT(chat_id,profile_id) DO UPDATE
SET membership_revision=EXCLUDED.membership_revision,lineage=EXCLUDED.lineage,status=EXCLUDED.status`,
		e.ChatID, e.ProfileID, next.Revision, next.Lineage, next.Status)
	return err
}
