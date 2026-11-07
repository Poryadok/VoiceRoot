package threadlist

import (
	"bytes"
	"context"
	"errors"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/pkg/effectivemembership"
)

type MembershipCheckpoint struct {
	ChatID, Incarnation, SnapshotID uuid.UUID
	AppliedQ, BaselineQ             int64
	Quarantined                     bool
}

type BaselineReceipt struct {
	SnapshotID, Generation, Incarnation uuid.UUID
	Q                                   int64
	Current                             bool
}

// ValidatedMembershipBaseline binds the checkpoint captured BEFORE remote
// intake to that intake's private capability. Callers cannot attach a cached
// snapshot to a newer local checkpoint. This in-process capability belongs to
// one pool; restart or different-pool continuation requires fresh intake.
type ValidatedMembershipBaseline struct {
	expected MembershipCheckpoint
	snapshot effectivemembership.ValidatedSnapshot
	owner    *pgxpool.Pool
}

func (b ValidatedMembershipBaseline) PriorCheckpoint() MembershipCheckpoint { return b.expected }
func (b ValidatedMembershipBaseline) Export() effectivemembership.Export    { return b.snapshot.Export() }

func (s MembershipInbox) ReceiveBaseline(ctx context.Context, source effectivemembership.SourceReader, chat uuid.UUID) (ValidatedMembershipBaseline, error) {
	if ctx == nil || source == nil || s.Pool == nil || chat == uuid.Nil {
		return ValidatedMembershipBaseline{}, ErrMembershipUnavailable
	}
	expected, err := s.Checkpoint(ctx, chat)
	if err != nil {
		return ValidatedMembershipBaseline{}, err
	}
	snapshot, err := effectivemembership.Receive(ctx, source, chat)
	if err != nil {
		return ValidatedMembershipBaseline{}, err
	}
	return ValidatedMembershipBaseline{expected: expected, snapshot: snapshot, owner: s.Pool}, nil
}

func (s MembershipInbox) Checkpoint(ctx context.Context, chat uuid.UUID) (MembershipCheckpoint, error) {
	if s.Pool == nil || chat == uuid.Nil {
		return MembershipCheckpoint{}, ErrMembershipUnavailable
	}
	return readMembershipCheckpoint(ctx, s.Pool, chat, false)
}

// InstallBaseline consumes only a complete intake capability obtained from the
// application's trusted authenticated owner adapter. It publishes membership
// BUILDING/REVOKED state, never viewer READY, roots or read permission. Every
// subsequent builder/page still needs fresh admission and a transactional CAS.
func (s MembershipInbox) InstallBaseline(ctx context.Context, baseline ValidatedMembershipBaseline) (BaselineReceipt, error) {
	var receipt BaselineReceipt
	if baseline.owner == nil || baseline.owner != s.Pool {
		return receipt, ErrMembershipUnavailable
	}
	expected, snapshot := baseline.expected, baseline.snapshot
	h := snapshot.Export()
	if s.Pool == nil || h.SnapshotID == uuid.Nil || h.ChatID == uuid.Nil || h.Incarnation == uuid.Nil || expected.ChatID != h.ChatID || h.Count > math.MaxInt64 {
		return receipt, ErrMembershipUnavailable
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return receipt, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	current, err := readMembershipCheckpoint(ctx, tx, h.ChatID, true)
	if err != nil {
		return receipt, err
	}
	if err = baselinePinLive(ctx, tx, h); err != nil {
		return receipt, err
	}
	binding := effectivemembership.ExportDigest(h)
	var original []byte
	err = tx.QueryRow(ctx, `SELECT incarnation,generation,journal_offset,binding_digest FROM thread_list_membership_baselines WHERE chat_id=$1 AND snapshot_id=$2`, h.ChatID, h.SnapshotID).Scan(&receipt.Incarnation, &receipt.Generation, &receipt.Q, &original)
	if err == nil {
		if !bytes.Equal(original, binding[:]) {
			if h.Incarnation == current.Incarnation {
				return BaselineReceipt{}, quarantineBaseline(ctx, tx, h)
			}
			return BaselineReceipt{}, ErrMembershipConflict
		}
		if current.Quarantined {
			return BaselineReceipt{}, ErrMembershipUnavailable
		}
		receipt.SnapshotID = h.SnapshotID
		receipt.Current = current.SnapshotID == h.SnapshotID && current.Incarnation == h.Incarnation
		return receipt, tx.Commit(ctx) // Historical retry never reapplies an older generation.
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return receipt, err
	}
	if current != expected || (current.Quarantined && current.Incarnation == h.Incarnation) || (current.Incarnation == h.Incarnation && h.Q < current.AppliedQ) {
		return receipt, ErrMembershipUnavailable
	}
	var generationConflict bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM thread_list_membership_baselines WHERE chat_id=$1 AND incarnation=$2 AND generation=$3 AND content_digest<>$4)`, h.ChatID, h.Incarnation, h.Generation, h.Digest[:]).Scan(&generationConflict)
	if err != nil {
		return receipt, err
	}
	if generationConflict {
		return receipt, quarantineBaseline(ctx, tx, h)
	}
	known, err := loadKnownMembership(ctx, tx, current)
	if err != nil {
		return receipt, err
	}
	reset := current.Incarnation != h.Incarnation
	if reset {
		var retired bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM thread_list_membership_retired_incarnations WHERE chat_id=$1 AND incarnation=$2)`, h.ChatID, h.Incarnation).Scan(&retired)
		if err != nil {
			return receipt, err
		}
		if retired {
			return receipt, ErrMembershipUnavailable
		}
	}
	pendingRetirements := make(lineageEvidence)
	if !reset {
		if err = collectCoveredMembership(ctx, tx, current, h.Q, known, pendingRetirements); err != nil {
			return receipt, err
		}
	}
	members := snapshot.Members()
	for _, m := range members {
		if m.Allowed && pendingRetirements.contains(m.ProfileID, m.Lineage) {
			return receipt, ErrMembershipConflict
		}
		if old, ok := known[m.ProfileID]; ok {
			if err = checkBaselineTransition(old, m, reset); err != nil {
				return receipt, err
			}
		}
		if m.Allowed {
			var retired bool
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM thread_list_retired_lineages WHERE chat_id=$1 AND profile_id=$2 AND lineage=$3)`, h.ChatID, m.ProfileID, m.Lineage).Scan(&retired)
			if err != nil {
				return receipt, err
			}
			if retired {
				return receipt, ErrMembershipConflict
			}
		}
	}
	// Persist all mutations under the same head lock. Any database failure below
	// rolls back receipt, retirements, pair rows, baseline floor and replay.
	_, err = tx.Exec(ctx, `INSERT INTO thread_list_membership_baselines(chat_id,snapshot_id,incarnation,generation,journal_offset,pin_id,retain_until,member_count,content_digest,binding_digest) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, h.ChatID, h.SnapshotID, h.Incarnation, h.Generation, h.Q, h.PinID, h.RetainUntil, int64(h.Count), h.Digest[:], binding[:])
	if err != nil {
		return receipt, err
	}
	if reset {
		if _, err = tx.Exec(ctx, `INSERT INTO thread_list_membership_retired_incarnations(chat_id,incarnation) VALUES($1,$2) ON CONFLICT DO NOTHING`, h.ChatID, current.Incarnation); err != nil {
			return receipt, err
		}
		for id, old := range known {
			if err = retireBaselineLineage(ctx, tx, h.ChatID, id, old.member.Lineage); err != nil {
				return receipt, err
			}
		}
		if _, err = tx.Exec(ctx, `DELETE FROM thread_list_membership_pairs WHERE chat_id=$1`, h.ChatID); err != nil {
			return receipt, err
		}
	}
	for profile, lineages := range pendingRetirements {
		for lineage := range lineages {
			if err = retireBaselineLineage(ctx, tx, h.ChatID, profile, lineage); err != nil {
				return receipt, err
			}
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM thread_list_membership_snapshot_pairs WHERE chat_id=$1`, h.ChatID); err != nil {
		return receipt, err
	}
	remaining := make(map[uuid.UUID]knownMembership, len(known))
	if !reset {
		for id, old := range known {
			remaining[id] = old
		}
	}
	for _, m := range members {
		if old, ok := known[m.ProfileID]; ok && (reset || !m.Allowed || old.member.Lineage != m.Lineage) {
			if err = retireBaselineLineage(ctx, tx, h.ChatID, m.ProfileID, old.member.Lineage); err != nil {
				return receipt, err
			}
		}
		if !m.Allowed {
			if err = retireBaselineLineage(ctx, tx, h.ChatID, m.ProfileID, m.Lineage); err != nil {
				return receipt, err
			}
		}
		if err = writeBaselinePair(ctx, tx, h.ChatID, h.Incarnation, m, false); err != nil {
			return receipt, err
		}
		delete(remaining, m.ProfileID)
	}
	for id, old := range remaining {
		if err = retireBaselineLineage(ctx, tx, h.ChatID, id, old.member.Lineage); err != nil {
			return receipt, err
		}
		old.member.Allowed = false
		if err = writeBaselinePair(ctx, tx, h.ChatID, h.Incarnation, old.member, true); err != nil {
			return receipt, err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE thread_list_membership_heads SET incarnation=$2,applied_q=$3,baseline_q=$3,baseline_snapshot_id=$4,quarantined=false WHERE chat_id=$1`, h.ChatID, h.Incarnation, h.Q, h.SnapshotID)
	if err != nil {
		return receipt, err
	}
	replayErr := replayMembership(ctx, tx, h.ChatID, h.Incarnation, h.Q)
	if replayErr != nil && !errors.Is(replayErr, ErrMembershipConflict) {
		return receipt, replayErr
	}
	if errors.Is(replayErr, ErrMembershipConflict) {
		if err = markMembershipQuarantine(ctx, tx, h.ChatID); err != nil {
			return receipt, err
		}
	}
	if err = baselinePinLive(ctx, tx, h); err != nil {
		return receipt, err
	}
	receipt = BaselineReceipt{SnapshotID: h.SnapshotID, Generation: h.Generation, Incarnation: h.Incarnation, Q: h.Q, Current: true}
	if err = tx.Commit(ctx); err != nil {
		return BaselineReceipt{}, err
	}
	if replayErr != nil {
		return BaselineReceipt{}, replayErr
	}
	return receipt, nil
}

type checkpointReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func readMembershipCheckpoint(ctx context.Context, q checkpointReader, chat uuid.UUID, lock bool) (MembershipCheckpoint, error) {
	c := MembershipCheckpoint{ChatID: chat}
	query := `SELECT incarnation,applied_q,baseline_q,baseline_snapshot_id,quarantined FROM thread_list_membership_heads WHERE chat_id=$1`
	if lock {
		query += ` FOR UPDATE`
	}
	var snapshot *uuid.UUID
	err := q.QueryRow(ctx, query, chat).Scan(&c.Incarnation, &c.AppliedQ, &c.BaselineQ, &snapshot, &c.Quarantined)
	if errors.Is(err, pgx.ErrNoRows) {
		return MembershipCheckpoint{}, ErrMembershipUnavailable
	}
	if snapshot != nil {
		c.SnapshotID = *snapshot
	}
	return c, err
}

func baselinePinLive(ctx context.Context, tx pgx.Tx, h effectivemembership.Export) error {
	var live bool
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()<$1::timestamptz`, h.RetainUntil).Scan(&live); err != nil {
		return err
	}
	if !live {
		return ErrMembershipUnavailable
	}
	return nil
}

func quarantineBaseline(ctx context.Context, tx pgx.Tx, h effectivemembership.Export) error {
	if err := markMembershipQuarantine(ctx, tx, h.ChatID); err != nil {
		return err
	}
	if err := baselinePinLive(ctx, tx, h); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return ErrMembershipConflict
}

func retireBaselineLineage(ctx context.Context, tx pgx.Tx, chat, profile, lineage uuid.UUID) error {
	if lineage == uuid.Nil {
		return nil
	}
	_, err := tx.Exec(ctx, `INSERT INTO thread_list_retired_lineages(chat_id,profile_id,lineage) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, chat, profile, lineage)
	return err
}

func writeBaselinePair(ctx context.Context, tx pgx.Tx, chat, incarnation uuid.UUID, m effectivemembership.Member, omitted bool) error {
	raw, err := canonicalBaselineMember(m)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO thread_list_membership_snapshot_pairs(chat_id,profile_id,incarnation,member_state,omitted) VALUES($1,$2,$3,$4,$5) ON CONFLICT(chat_id,profile_id) DO UPDATE SET incarnation=EXCLUDED.incarnation,member_state=EXCLUDED.member_state,omitted=EXCLUDED.omitted`, chat, m.ProfileID, incarnation, raw, omitted)
	if err != nil {
		return err
	}
	if m.Revision == 0 {
		return nil
	} // Explicit never-eligible/never-joined negative fact.
	state := MemberRevoked
	if m.Allowed {
		state = MemberBuilding
	}
	_, err = tx.Exec(ctx, `INSERT INTO thread_list_membership_pairs(chat_id,profile_id,membership_revision,lineage,status) VALUES($1,$2,$3,$4,$5) ON CONFLICT(chat_id,profile_id) DO UPDATE SET membership_revision=EXCLUDED.membership_revision,lineage=EXCLUDED.lineage,status=EXCLUDED.status`, chat, m.ProfileID, m.Revision, m.Lineage, state)
	return err
}
