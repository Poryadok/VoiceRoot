package store

import (
	"bytes"
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/matchmaking/internal/mmadmission"
)

type NativeTimeoutResult struct {
	Expired, Replay bool
	Sessions        []SearchSession
}

// ExpireNativeSearchingParty accepts a scan candidate, never a scalar timeout
// authorization. All source/session/intent checks and DB time occur after the
// complete canonical locks. No Redis, broker or source RPC runs in this tx.
func (s *MatchStore) ExpireNativeSearchingParty(ctx context.Context, candidate uuid.UUID) (NativeTimeoutResult, error) {
	if s == nil || s.Pool == nil || s.NativeAdmission == nil || ctx == nil || ctx.Err() != nil || candidate == uuid.Nil {
		return NativeTimeoutResult{}, ErrSourcePartyUnavailable
	}
	binding := copyNativeBinding(s.NativeAdmission)
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return NativeTimeoutResult{}, err
	}
	defer rollbackPartySearch(tx)
	if err = mmadmission.AcquireTerminal(ctx, tx, binding); err != nil {
		return NativeTimeoutResult{}, err
	}
	var partyID *uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT party_id FROM search_sessions WHERE id=$1`, candidate).Scan(&partyID); err != nil || partyID == nil {
		return NativeTimeoutResult{}, ErrSourcePartyUnavailable
	}
	party, err := scanSourceParty(tx.QueryRow(ctx, `SELECT `+sourcePartyCols+` FROM parties WHERE id=$1 FOR UPDATE`, *partyID))
	if err != nil {
		return NativeTimeoutResult{}, ErrSourcePartyUnavailable
	}
	rows, err := tx.Query(ctx, `SELECT `+sessionSelectCols+` FROM search_sessions WHERE party_id=$1 ORDER BY id FOR UPDATE`, party.ID)
	if err != nil {
		return NativeTimeoutResult{}, err
	}
	sessions, err := scanSessionRows(rows)
	rows.Close()
	if err != nil {
		return NativeTimeoutResult{}, err
	}
	var revision int64
	var action string
	if err = tx.QueryRow(ctx, `SELECT revision,action FROM matchmaking_party_search_queue_intents WHERE party_id=$1 FOR UPDATE`, party.ID).Scan(&revision, &action); err != nil {
		return NativeTimeoutResult{}, ErrSourcePartyUnavailable
	}
	if _, err = readCandidateOriginsMode(ctx, tx, binding, []SourceParty{party}, true); err != nil {
		return NativeTimeoutResult{}, ErrSourcePartyUnavailable
	}
	if err = checkNativeTimeoutSet(party, sessions, candidate); err != nil {
		return NativeTimeoutResult{}, err
	}
	allTimeout, allSearching, allCancelled := true, true, true
	for _, session := range sessions {
		allTimeout = allTimeout && session.Status == SessionStatusTimeout
		allSearching = allSearching && session.Status == SessionStatusSearching
		allCancelled = allCancelled && session.Status == SessionStatusCancelled
	}
	if allTimeout {
		if party.DisbandedAt == nil || action != "RELEASE" || checkNativeTimeoutReplay(ctx, tx, party, sessions) != nil {
			return NativeTimeoutResult{}, ErrSourcePartyUnavailable
		}
		if err = tx.Commit(ctx); err != nil {
			return NativeTimeoutResult{}, err
		}
		return NativeTimeoutResult{Replay: true, Sessions: sessions}, nil
	}
	if allCancelled {
		return NativeTimeoutResult{}, ErrSessionNotSearchable
	}
	if !allSearching || party.DisbandedAt != nil {
		return NativeTimeoutResult{}, ErrSessionNotSearchable
	}
	if action != "ENQUEUE" || revision == math.MaxInt64 {
		return NativeTimeoutResult{}, ErrSourcePartyUnavailable
	}
	var now time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return NativeTimeoutResult{}, err
	}
	if sessions[0].TimeoutAt.After(now) {
		return NativeTimeoutResult{}, tx.Commit(ctx)
	}
	for _, session := range sessions {
		if session.RecoveryGeneration == math.MaxInt64 {
			return NativeTimeoutResult{}, ErrSourcePartyUnavailable
		}
	}
	tag, err := tx.Exec(ctx, `UPDATE search_sessions SET status='timeout',updated_at=$2,recovery_generation=recovery_generation+1 WHERE party_id=$1 AND status='searching' AND match_id IS NULL`, party.ID, now)
	if err != nil || tag.RowsAffected() != int64(len(sessions)) {
		if err != nil {
			return NativeTimeoutResult{}, err
		}
		return NativeTimeoutResult{}, ErrSourcePartyUnavailable
	}
	if _, err = tx.Exec(ctx, `UPDATE parties SET disbanded_at=$2 WHERE id=$1 AND disbanded_at IS NULL`, party.ID, now); err != nil {
		return NativeTimeoutResult{}, err
	}
	if err = advanceRosterQueueIntent(ctx, tx, party.ID, "RELEASE", true); err != nil {
		return NativeTimeoutResult{}, err
	}
	for i := range sessions {
		sessions[i].Status = SessionStatusTimeout
		sessions[i].RecoveryGeneration++
		sessions[i].UpdatedAt = now
		id := uuid.New()
		payload, encodeErr := encodeNativeTimeout(id, sessions[i], now)
		if encodeErr != nil {
			return NativeTimeoutResult{}, encodeErr
		}
		if _, err = tx.Exec(ctx, `INSERT INTO matchmaking_native_timeout_outbox(event_id,party_id,search_session_id,profile_id,terminal_generation,occurred_at,payload) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, party.ID, sessions[i].ID, sessions[i].ProfileID, sessions[i].RecoveryGeneration, now, payload); err != nil {
			return NativeTimeoutResult{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return NativeTimeoutResult{}, err
	}
	return NativeTimeoutResult{Expired: true, Sessions: sessions}, nil
}

func checkNativeTimeoutSet(party SourceParty, sessions []SearchSession, candidate uuid.UUID) error {
	if len(sessions) == 0 || len(sessions) != len(party.MemberProfileIDs) {
		return ErrSourcePartyUnavailable
	}
	members := make(map[uuid.UUID]bool, len(party.MemberProfileIDs))
	for _, id := range party.MemberProfileIDs {
		members[id] = false
	}
	found := false
	for _, session := range sessions {
		seen, exists := members[session.ProfileID]
		if !exists || seen || session.PartyID == nil || *session.PartyID != party.ID || session.GameID != party.GameID || session.Mode != party.Mode || session.Criteria != party.Criteria || !session.CreatedAt.Equal(party.CreatedAt) || session.TimeoutAt == nil || !session.TimeoutAt.Equal(*sessions[0].TimeoutAt) || !samePartyUUID(session.SpaceID, sessions[0].SpaceID) || session.RecoveryGeneration < 0 {
			return ErrSourcePartyUnavailable
		}
		if session.MatchID != nil {
			return ErrSessionNotSearchable
		}
		members[session.ProfileID] = true
		found = found || session.ID == candidate
	}
	if !found {
		return ErrSourcePartyUnavailable
	}
	return nil
}

func encodeNativeTimeout(id uuid.UUID, session SearchSession, at time.Time) ([]byte, error) {
	return proto.MarshalOptions{Deterministic: true}.Marshal(&eventsv1.MatchmakingStreamEvent{EventId: id.String(), OccurredAt: timestamppb.New(at), Payload: &eventsv1.MatchmakingStreamEvent_MatchTimeout{MatchTimeout: &eventsv1.MatchTimeout{SessionId: session.ID.String(), ProfileId: session.ProfileID.String(), GameId: session.GameID.String(), Mode: session.Mode}}})
}

func checkNativeTimeoutReplay(ctx context.Context, tx pgx.Tx, party SourceParty, sessions []SearchSession) error {
	var count int
	if tx.QueryRow(ctx, `SELECT count(*) FROM matchmaking_native_timeout_outbox WHERE party_id=$1`, party.ID).Scan(&count) != nil || count != len(sessions) {
		return ErrSourcePartyUnavailable
	}
	for _, session := range sessions {
		var id, profile uuid.UUID
		var generation int64
		var at time.Time
		var payload []byte
		if tx.QueryRow(ctx, `SELECT event_id,profile_id,terminal_generation,occurred_at,payload FROM matchmaking_native_timeout_outbox WHERE party_id=$1 AND search_session_id=$2`, party.ID, session.ID).Scan(&id, &profile, &generation, &at, &payload) != nil || id == uuid.Nil || profile != session.ProfileID || generation != session.RecoveryGeneration || !at.Equal(*party.DisbandedAt) || !at.Equal(session.UpdatedAt) {
			return ErrSourcePartyUnavailable
		}
		expected, err := encodeNativeTimeout(id, session, at)
		if err != nil || !bytes.Equal(expected, payload) {
			return ErrSourcePartyUnavailable
		}
	}
	return nil
}
