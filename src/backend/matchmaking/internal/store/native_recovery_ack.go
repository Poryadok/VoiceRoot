package store

import (
	"bytes"
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	admission "voice/backend/matchmaking/internal/mmadmission"
	wire "voice/backend/pkg/mmadmissionwire"
)

// AcknowledgeNativeRecoveryEffects consumes qualified whole-party projection
// evidence. It performs no network operation and grants no forward authority.
func (s *MatchStore) AcknowledgeNativeRecoveryEffects(ctx context.Context, limit int) (int, error) {
	if s == nil || s.Pool == nil || s.NativeAdmission == nil || ctx == nil || ctx.Err() != nil || limit < 1 || limit > 1000 {
		return 0, ErrSourcePartyUnavailable
	}
	binding := copyNativeBinding(s.NativeAdmission)
	if admission.WireBinding(*binding).Validate() != nil {
		return 0, ErrSourcePartyUnavailable
	}
	rows, err := s.Pool.Query(ctx, `SELECT match_id,search_session_id,generation FROM matchmaking_match_recovery_effects WHERE applied_at IS NULL ORDER BY CASE WHEN action='release' THEN 0 ELSE 1 END,created_at,match_id,search_session_id LIMIT $1`, limit)
	if err != nil {
		return 0, ErrSourcePartyUnavailable
	}
	type effect struct {
		match, session uuid.UUID
		generation     int64
	}
	var pending []effect
	for rows.Next() {
		var item effect
		if rows.Scan(&item.match, &item.session, &item.generation) != nil {
			rows.Close()
			return 0, ErrSourcePartyUnavailable
		}
		pending = append(pending, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, ErrSourcePartyUnavailable
	}
	count := 0
	var incomplete error
	for _, item := range pending {
		tx, e := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		if e != nil {
			return count, ErrSourcePartyUnavailable
		}
		// This transaction only acknowledges already committed physical queue
		// projection (or an obsolete generation). It never dispatches/requeues,
		// so it needs neither forward locks nor a network-under-SQL callback.
		// RC history verification uses its existing nonblocking shared try-gate;
		// a closer cannot make this acknowledgement wait behind inverse locks.
		var action string
		e = tx.QueryRow(ctx, `SELECT action FROM matchmaking_match_recovery_effects WHERE match_id=$1 AND search_session_id=$2 AND generation=$3 AND applied_at IS NULL FOR UPDATE`, item.match, item.session, item.generation).Scan(&action)
		if e == pgx.ErrNoRows {
			_ = tx.Rollback(ctx)
			continue
		}
		if e == nil {
			e = nativeRecoveryProjectionSatisfied(ctx, tx, binding, item.session, item.generation, action)
		}
		if e == nil {
			_, e = tx.Exec(ctx, `UPDATE matchmaking_match_recovery_effects SET applied_at=clock_timestamp() WHERE match_id=$1 AND search_session_id=$2 AND generation=$3 AND applied_at IS NULL`, item.match, item.session, item.generation)
		}
		if e == nil {
			e = tx.Commit(ctx)
		} else {
			_ = tx.Rollback(ctx)
		}
		if e != nil {
			incomplete = ErrSourcePartyUnavailable
			continue
		}
		count++
	}
	return count, incomplete
}

func nativeRecoveryProjectionSatisfied(ctx context.Context, tx pgx.Tx, binding *admission.Binding, session uuid.UUID, generation int64, action string) error {
	if action != "enqueue" && action != "release" {
		return ErrSourcePartyUnavailable
	}
	var currentGeneration int64
	var status string
	var party *uuid.UUID
	if tx.QueryRow(ctx, `SELECT recovery_generation,status,party_id FROM search_sessions WHERE id=$1`, session).Scan(&currentGeneration, &status, &party) != nil {
		return ErrSourcePartyUnavailable
	}
	if currentGeneration != generation || (action == "enqueue" && status != SessionStatusSearching) || (action == "release" && status != SessionStatusCancelled) {
		return nil
	}
	if party == nil {
		return ErrSourcePartyUnavailable
	}
	var state, stamp []byte
	var applied bool
	if tx.QueryRow(ctx, `SELECT projected_state_sha256,projected_admission_stamp_sha256,projected_at IS NOT NULL FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, *party).Scan(&state, &stamp, &applied) != nil || !applied {
		return ErrSourcePartyUnavailable
	}
	hash, err := partyQueueProjectionHash(ctx, tx, *party)
	if err != nil || !bytes.Equal(state, hash[:]) {
		return ErrSourcePartyUnavailable
	}
	if action == "release" {
		if len(stamp) != 0 {
			return ErrSourcePartyUnavailable
		}
		return nil
	}
	source, err := scanSourceParty(tx.QueryRow(ctx, `SELECT `+sourcePartyCols+` FROM parties WHERE id=$1`, *party))
	if err != nil {
		return ErrSourcePartyUnavailable
	}
	origins, err := readCandidateOrigins(ctx, tx, binding, []SourceParty{source})
	if err != nil {
		return err
	}
	expected, err := wire.StampHash(origins.Stamp)
	if err != nil || !bytes.Equal(stamp, expected[:]) {
		return ErrSourcePartyUnavailable
	}
	return nil
}
