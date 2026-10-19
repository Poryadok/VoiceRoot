package store

import (
	"bytes"
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"sort"
	"time"
	eventsv1 "voice.app/voice/events/v1"
)

// ApplyVoiceRosterEvent commits inbox identity, party cancellation, queue deny
// generations and immutable cancellation envelopes before the consumer ACKs.
func (s *MatchStore) ApplyVoiceRosterEvent(ctx context.Context, raw []byte) (VoiceRosterResetResult, error) {
	fact, decodeErr := decodeVoiceRosterFact(raw)
	return s.rosterTransaction(ctx, func(tx pgx.Tx) (VoiceRosterResetResult, error) {
		mismatch := decodeErr != nil
		selectors := []voiceRosterFact{fact}
		if !mismatch {
			label := "LEFT"
			if fact.Joined {
				label = "JOINED"
			}
			tag, err := tx.Exec(ctx, `INSERT INTO matchmaking_voice_roster_inbox(event_id,payload_sha256,room_id,roster_version,profile_id,change) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, fact.EventID, fact.Hash[:], fact.RoomID, fact.Version, fact.ProfileID, label)
			if err != nil {
				return VoiceRosterResetResult{}, err
			}
			if tag.RowsAffected() == 0 {
				rows, err := tx.Query(ctx, `SELECT event_id,payload_sha256,room_id,roster_version,profile_id,change FROM matchmaking_voice_roster_inbox WHERE event_id=$1 OR (room_id=$2 AND roster_version=$3) ORDER BY event_id FOR UPDATE`, fact.EventID, fact.RoomID, fact.Version)
				if err != nil {
					return VoiceRosterResetResult{}, err
				}
				exact := false
				for rows.Next() {
					var prior voiceRosterFact
					var hash []byte
					var change string
					if err = rows.Scan(&prior.EventID, &hash, &prior.RoomID, &prior.Version, &prior.ProfileID, &change); err != nil {
						rows.Close()
						return VoiceRosterResetResult{}, err
					}
					prior.Joined = change == "JOINED"
					selectors = append(selectors, prior)
					if prior.EventID == fact.EventID && bytes.Equal(hash, fact.Hash[:]) {
						exact = true
					} else {
						mismatch = true
					}
				}
				err = rows.Err()
				rows.Close()
				if err != nil {
					return VoiceRosterResetResult{}, err
				}
				if exact && !mismatch {
					return VoiceRosterResetResult{}, nil
				}
				mismatch = true
			}
		}
		if mismatch {
			if _, err := tx.Exec(ctx, `INSERT INTO matchmaking_voice_roster_contract_conflicts(payload_sha256,payload) VALUES($1,$2) ON CONFLICT DO NOTHING`, fact.Hash[:], raw); err != nil {
				return VoiceRosterResetResult{}, err
			}
		}
		targets := map[uuid.UUID]bool{}
		for _, selector := range selectors {
			rows, err := tx.Query(ctx, `SELECT p.id FROM parties p WHERE p.disbanded_at IS NULL AND p.search_operation_id IS NOT NULL
AND ((p.voice_source_kind='VOICE_ROSTER' AND p.voice_source_room_id=$1 AND ($5 OR p.voice_source_roster_version<$2))
 OR (p.voice_source_kind='SOLO' AND p.leader_profile_id=$3 AND $4))
AND EXISTS(SELECT 1 FROM search_sessions s WHERE s.party_id=p.id AND s.status IN ('searching','pending_accept'))`, selector.RoomID, selector.Version, selector.ProfileID, selector.Joined, mismatch)
			if err != nil {
				return VoiceRosterResetResult{}, err
			}
			for rows.Next() {
				var id uuid.UUID
				if err = rows.Scan(&id); err != nil {
					rows.Close()
					return VoiceRosterResetResult{}, err
				}
				targets[id] = true
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return VoiceRosterResetResult{}, err
			}
		}
		count, err := resetCapturedRosterParties(ctx, tx, targets)
		return VoiceRosterResetResult{ContractMismatch: mismatch, CancelledSessions: count}, err
	})
}

// ResetVoiceParty is shared by definitive snapshot revalidation failures.
// Unknown/unavailable authority must never call this boundary.
func (s *MatchStore) ResetVoiceParty(ctx context.Context, partyID uuid.UUID) (VoiceRosterResetResult, error) {
	return s.ResetVoiceParties(ctx, []uuid.UUID{partyID})
}

// ResetVoiceParties atomically handles the complete definitive changed set from
// one protected provision candidate. Unknown validation must not enter here.
func (s *MatchStore) ResetVoiceParties(ctx context.Context, partyIDs []uuid.UUID) (VoiceRosterResetResult, error) {
	targets := make(map[uuid.UUID]bool, len(partyIDs))
	for _, id := range partyIDs {
		if id == uuid.Nil {
			return VoiceRosterResetResult{}, errVoiceRosterCutChanged
		}
		targets[id] = true
	}
	return s.rosterTransaction(ctx, func(tx pgx.Tx) (VoiceRosterResetResult, error) {
		count, err := resetCapturedRosterParties(ctx, tx, targets)
		return VoiceRosterResetResult{CancelledSessions: count}, err
	})
}

func (s *MatchStore) rosterTransaction(ctx context.Context, apply func(pgx.Tx) (VoiceRosterResetResult, error)) (VoiceRosterResetResult, error) {
	if s == nil || s.Pool == nil {
		return VoiceRosterResetResult{}, errors.New("roster reset store unavailable")
	}
	for range 4 {
		tx, err := s.Pool.Begin(ctx)
		if err != nil {
			return VoiceRosterResetResult{}, err
		}
		result, err := apply(tx)
		if err == nil {
			err = tx.Commit(ctx)
		} else {
			_ = tx.Rollback(ctx)
		}
		if err == nil {
			return result, nil
		}
		_ = tx.Rollback(ctx)
		var pgErr *pgconn.PgError
		retry := errors.Is(err, errVoiceRosterCutChanged) || (errors.As(err, &pgErr) && (pgErr.Code == "40P01" || pgErr.Code == "40001"))
		if !retry || ctx.Err() != nil {
			return VoiceRosterResetResult{}, err
		}
	}
	return VoiceRosterResetResult{}, errVoiceRosterCutChanged
}

func sortedRosterIDs(ids map[uuid.UUID]bool) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return bytes.Compare(out[i][:], out[j][:]) < 0 })
	return out
}

// Lock order is matches, complete party union, then complete session union.
// A proposal committed between discovery and the party locks causes a fresh
// transaction; no second match is discovered while holding party/session locks.
func resetCapturedRosterParties(ctx context.Context, tx pgx.Tx, targets map[uuid.UUID]bool) (int, error) {
	return resetCapturedRosterPartiesChecked(ctx, tx, targets, nil, "VOICE_ROSTER_CHANGED")
}

// The optional check runs only after the complete canonical party/session lock
// set and before any terminal mutation. It performs SQL-only validation.
func resetCapturedRosterPartiesChecked(ctx context.Context, tx pgx.Tx, targets map[uuid.UUID]bool, check func() error, cancellationReason string) (int, error) {
	if len(targets) == 0 {
		return 0, nil
	}
	targetIDs := sortedRosterIDs(targets)
	rows, err := tx.Query(ctx, `SELECT DISTINCT match_id FROM search_sessions WHERE party_id=ANY($1) AND status='pending_accept' AND match_id IS NOT NULL`, targetIDs)
	if err != nil {
		return 0, err
	}
	matchIDs := map[uuid.UUID]bool{}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		matchIDs[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	matches := map[uuid.UUID]Match{}
	for _, id := range sortedRosterIDs(matchIDs) {
		m, err := scanMatch(tx.QueryRow(ctx, `SELECT id,game_id,mode,region,participants,left_profile_ids,voice_room_id,chat_id,status,created_at,completed_at FROM matches WHERE id=$1 FOR UPDATE`, id))
		if err != nil {
			return 0, err
		}
		matches[id] = m
	}
	partyIDs := map[uuid.UUID]bool{}
	for id := range targets {
		partyIDs[id] = true
	}
	rows, err = tx.Query(ctx, `SELECT DISTINCT party_id FROM search_sessions WHERE match_id=ANY($1) AND party_id IS NOT NULL`, sortedRosterIDs(matchIDs))
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		partyIDs[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	rows, err = tx.Query(ctx, `SELECT id FROM parties WHERE id=ANY($1) ORDER BY id FOR UPDATE`, sortedRosterIDs(partyIDs))
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	// A closed/legacy party is not an authority for a new reset.
	rows, err = tx.Query(ctx, `SELECT id FROM parties WHERE id=ANY($1) AND disbanded_at IS NULL AND search_operation_id IS NOT NULL`, targetIDs)
	if err != nil {
		return 0, err
	}
	activeTargets := map[uuid.UUID]bool{}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		activeTargets[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	targets = activeTargets
	if len(targets) == 0 {
		return 0, nil
	}
	rows, err = tx.Query(ctx, `SELECT `+sessionSelectCols+` FROM search_sessions WHERE party_id=ANY($1) OR match_id=ANY($2) ORDER BY id FOR UPDATE`, sortedRosterIDs(partyIDs), sortedRosterIDs(matchIDs))
	if err != nil {
		return 0, err
	}
	wasActive := map[uuid.UUID]bool{}
	for rows.Next() {
		sess, scanErr := scanSession(rows)
		if scanErr != nil {
			rows.Close()
			return 0, scanErr
		}
		if sess.PartyID != nil && targets[*sess.PartyID] && (sess.Status == SessionStatusSearching || sess.Status == SessionStatusPendingAccept) {
			if sess.Status == SessionStatusPendingAccept && (sess.MatchID == nil || !matchIDs[*sess.MatchID]) {
				rows.Close()
				return 0, errVoiceRosterCutChanged
			}
			wasActive[sess.ID] = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	if check != nil {
		if err = check(); err != nil {
			return 0, err
		}
	}
	var now time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return 0, err
	}
	enqueueParties := map[uuid.UUID]bool{}
	for _, id := range sortedRosterIDs(matchIDs) {
		m := matches[id]
		if m.Status != MatchStatusPendingAccept {
			continue
		}
		proposals, err := loadMatchProposalsForUpdate(ctx, tx, id)
		if err != nil {
			return 0, err
		}
		sessions, err := loadMatchSessionsForUpdate(ctx, tx, id)
		if err != nil {
			return 0, err
		}
		declined := ""
		for _, p := range proposals {
			if p.PartyID != nil && targets[*p.PartyID] {
				declined = responsePartyKey(p)
				break
			}
		}
		if declined == "" {
			continue
		}
		changed, err := recoverDeclinedMatchLocked(ctx, tx, m, proposals, sessions, declined, now, "VOICE_ROSTER_CHANGED")
		if err != nil {
			return 0, err
		}
		for _, sess := range changed {
			if sess.PartyID != nil && !targets[*sess.PartyID] && sess.Status == SessionStatusSearching {
				enqueueParties[*sess.PartyID] = true
			}
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE search_sessions SET status='cancelled',updated_at=$2,recovery_generation=recovery_generation+1 WHERE party_id=ANY($1) AND status IN ('searching','pending_accept')`, sortedRosterIDs(targets), now); err != nil {
		return 0, err
	}
	count := 0
	closedParties := map[uuid.UUID]bool{}
	for sessionID := range wasActive {
		sess, err := scanSession(tx.QueryRow(ctx, `SELECT `+sessionSelectCols+` FROM search_sessions WHERE id=$1`, sessionID))
		if err != nil {
			return 0, err
		}
		if sess.Status != SessionStatusCancelled {
			return 0, errVoiceRosterCutChanged
		}
		if err = appendRosterCancellation(ctx, tx, sess, now, cancellationReason); err != nil {
			return 0, err
		}
		count++
		closedParties[*sess.PartyID] = true
	}
	for _, id := range sortedRosterIDs(closedParties) {
		if _, err = tx.Exec(ctx, `UPDATE parties SET disbanded_at=$2 WHERE id=$1 AND disbanded_at IS NULL`, id, now); err != nil {
			return 0, err
		}
		if err = advanceRosterQueueIntent(ctx, tx, id, "RELEASE", true); err != nil {
			return 0, err
		}
	}
	for _, id := range sortedRosterIDs(enqueueParties) {
		if err = advanceRosterQueueIntent(ctx, tx, id, "ENQUEUE", false); err != nil {
			return 0, err
		}
	}
	return count, nil
}

func advanceRosterQueueIntent(ctx context.Context, tx pgx.Tx, party uuid.UUID, action string, required bool) error {
	tag, err := tx.Exec(ctx, `UPDATE matchmaking_party_search_queue_intents SET revision=revision+1,action=$2,lease_token=NULL,lease_until=NULL,next_attempt_at=clock_timestamp(),projected_at=NULL WHERE party_id=$1`, party, action)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 && required {
		return errors.New("captured party queue intent missing")
	}
	return nil
}

func appendRosterCancellation(ctx context.Context, tx pgx.Tx, sess SearchSession, at time.Time, reason string) error {
	id := uuid.New()
	payload, err := proto.MarshalOptions{Deterministic: true}.Marshal(&eventsv1.MatchmakingStreamEvent{EventId: id.String(), OccurredAt: timestamppb.New(at), Payload: &eventsv1.MatchmakingStreamEvent_SearchCancelled{SearchCancelled: &eventsv1.SearchCancelled{SessionId: sess.ID.String(), ProfileId: sess.ProfileID.String(), Reason: reason}}})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO matchmaking_voice_roster_cancellation_outbox(event_id,search_session_id,profile_id,occurred_at,payload) VALUES($1,$2,$3,$4,$5) ON CONFLICT(search_session_id) DO NOTHING`, id, sess.ID, sess.ProfileID, at, payload)
	return err
}
