package store

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	MatchAcceptWindow        = 30 * time.Second
	MatchStatusPendingAccept = "pending_accept"
	MatchStatusActive        = "active"
	MatchStatusCompleted     = "completed"
	MatchStatusAbandoned     = "abandoned"

	ProposalResponsePending  = "pending"
	ProposalResponseAccepted = "accepted"
	ProposalResponseDeclined = "declined"
)

// ExpirePendingMatchAtDeadline applies the common proposal deadline under the
// match lock. It is also the request-time fence: a request that arrives after
// the deadline cannot win merely because the background sweeper has not run.
// It returns changed sessions so callers can project the committed recovery to
// the queue; accepted parties retain their original criteria and scope.
func (s *MatchStore) ExpirePendingMatchAtDeadline(ctx context.Context, matchID uuid.UUID) (bool, []SearchSession, error) {
	if s == nil || s.Pool == nil {
		return false, nil, errors.New("match store unavailable")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	match, err := scanMatch(tx.QueryRow(ctx, `
		SELECT id, game_id, mode, region, participants, left_profile_ids, voice_room_id, chat_id, status, created_at, completed_at
		FROM matches WHERE id = $1 FOR UPDATE
	`, matchID))
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil, ErrMatchNotFound
	}
	if err != nil {
		return false, nil, err
	}
	if match.Status != MatchStatusPendingAccept {
		return false, nil, nil
	}

	proposals, err := loadMatchProposalsForUpdate(ctx, tx, matchID)
	if err != nil {
		return false, nil, err
	}
	sessions, err := loadMatchSessionsForUpdate(ctx, tx, matchID)
	if err != nil {
		return false, nil, err
	}
	var dbNow time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&dbNow); err != nil {
		return false, nil, err
	}
	if dbNow.Before(match.CreatedAt.Add(MatchAcceptWindow)) {
		return false, nil, nil
	}
	allAccepted := len(proposals) > 0
	for _, proposal := range proposals {
		if proposal.Response != ProposalResponseAccepted {
			allAccepted = false
			break
		}
	}
	if allAccepted {
		// Acceptance completed before the deadline; provisioning may safely
		// finish or be retried after it.
		return false, nil, nil
	}

	changed, err := expirePendingMatchLocked(ctx, tx, match, proposals, sessions, dbNow)
	if err != nil {
		return false, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, nil, err
	}
	return true, changed, nil
}

// ListPendingDeadlineMatchIDs returns candidate matches using database time;
// ExpirePendingMatchAtDeadline rechecks each under its row lock.
func (s *MatchStore) ListPendingDeadlineMatchIDs(ctx context.Context, limit int) ([]uuid.UUID, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("match store unavailable")
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id FROM matches
		WHERE status = $1 AND created_at + ($2 * interval '1 millisecond') <= clock_timestamp()
		ORDER BY created_at, id
		LIMIT $3
	`, MatchStatusPendingAccept, MatchAcceptWindow.Milliseconds(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func expirePendingMatchLocked(ctx context.Context, tx pgx.Tx, match Match, proposals []MatchProposal, sessions []SearchSession, dbNow time.Time) ([]SearchSession, error) {
	partyAccepted := make(map[string]bool)
	partyHasPending := make(map[string]bool)
	for _, proposal := range proposals {
		key := responsePartyKey(proposal)
		if _, exists := partyAccepted[key]; !exists {
			partyAccepted[key] = true
		}
		if proposal.Response != ProposalResponseAccepted {
			partyAccepted[key] = false
		}
		if proposal.Response == ProposalResponsePending {
			partyHasPending[key] = true
		}
	}
	for _, proposal := range proposals {
		if proposal.Response != ProposalResponsePending || !partyHasPending[responsePartyKey(proposal)] {
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE match_proposals SET response = $3, updated_at = $4
			WHERE id = $1 AND match_id = $2 AND response = $5
		`, proposal.ID, match.ID, ProposalResponseDeclined, dbNow, ProposalResponsePending); err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE matches SET status = $2, completed_at = $3
		WHERE id = $1 AND status = $4
	`, match.ID, MatchStatusAbandoned, dbNow, MatchStatusPendingAccept); err != nil {
		return nil, err
	}

	var changed []SearchSession
	for _, sess := range sessions {
		if sess.Status != SessionStatusPendingAccept {
			continue
		}
		status := SessionStatusCancelled
		key := responseSessionPartyKey(sess, proposals)
		if partyAccepted[key] && !partyHasPending[key] {
			status = SessionStatusSearching
		}
		if status == SessionStatusSearching {
			updated, err := scanSession(tx.QueryRow(ctx, `
				UPDATE search_sessions
				SET status = $2, match_id = NULL, matched_at = NULL, updated_at = $3,
				    recovery_generation = recovery_generation + 1
				WHERE id = $1 AND match_id = $4 AND status = $5
				RETURNING `+sessionSelectCols+`
			`, sess.ID, status, dbNow, match.ID, SessionStatusPendingAccept))
			if err != nil {
				return nil, err
			}
			if err := insertRecoveryEffect(ctx, tx, match.ID, updated, "enqueue"); err != nil {
				return nil, err
			}
			changed = append(changed, updated)
			continue
		}
		updated, err := scanSession(tx.QueryRow(ctx, `
			UPDATE search_sessions SET status = $2, updated_at = $3,
			    recovery_generation = recovery_generation + 1
			WHERE id = $1 AND match_id = $4 AND status = $5
			RETURNING `+sessionSelectCols+`
		`, sess.ID, SessionStatusCancelled, dbNow, match.ID, SessionStatusPendingAccept))
		if err != nil {
			return nil, err
		}
		if err := insertRecoveryEffect(ctx, tx, match.ID, updated, "release"); err != nil {
			return nil, err
		}
		changed = append(changed, updated)
	}
	return changed, nil
}

func responsePartyKey(p MatchProposal) string {
	if p.PartyID != nil {
		return "party:" + p.PartyID.String()
	}
	return "profile:" + p.ProfileID.String()
}

func samePartyID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func responseSessionPartyKey(sess SearchSession, proposals []MatchProposal) string {
	for _, p := range proposals {
		if p.SearchSessionID == sess.ID {
			return responsePartyKey(p)
		}
	}
	return ""
}

func recoverDeclinedMatchLocked(ctx context.Context, tx pgx.Tx, match Match, proposals []MatchProposal, sessions []SearchSession, declinedParty string, dbNow time.Time) ([]SearchSession, error) {
	for _, proposal := range proposals {
		if responsePartyKey(proposal) != declinedParty || proposal.Response != ProposalResponsePending {
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE match_proposals SET response = $3, updated_at = $4
			WHERE id = $1 AND match_id = $2 AND response = $5
		`, proposal.ID, match.ID, ProposalResponseDeclined, dbNow, ProposalResponsePending); err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE matches SET status = $2, completed_at = $3
		WHERE id = $1 AND status = $4
	`, match.ID, MatchStatusAbandoned, dbNow, MatchStatusPendingAccept); err != nil {
		return nil, err
	}
	changed := make([]SearchSession, 0, len(sessions))
	for _, sess := range sessions {
		if sess.Status != SessionStatusPendingAccept {
			continue
		}
		party := responseSessionPartyKey(sess, proposals)
		if party == declinedParty {
			updated, err := scanSession(tx.QueryRow(ctx, `
			UPDATE search_sessions SET status = $2, updated_at = $3,
			    recovery_generation = recovery_generation + 1
			WHERE id = $1 AND match_id = $4 AND status = $5
				RETURNING `+sessionSelectCols+`
			`, sess.ID, SessionStatusCancelled, dbNow, match.ID, SessionStatusPendingAccept))
			if err != nil {
				return nil, err
			}
			if err := insertRecoveryEffect(ctx, tx, match.ID, updated, "release"); err != nil {
				return nil, err
			}
			changed = append(changed, updated)
			continue
		}
		updated, err := scanSession(tx.QueryRow(ctx, `
			UPDATE search_sessions
			SET status = $2, match_id = NULL, matched_at = NULL, updated_at = $3,
			    recovery_generation = recovery_generation + 1
			WHERE id = $1 AND match_id = $4 AND status = $5
			RETURNING `+sessionSelectCols+`
		`, sess.ID, SessionStatusSearching, dbNow, match.ID, SessionStatusPendingAccept))
		if err != nil {
			return nil, err
		}
		if err := insertRecoveryEffect(ctx, tx, match.ID, updated, "enqueue"); err != nil {
			return nil, err
		}
		changed = append(changed, updated)
	}
	return changed, nil
}

func insertRecoveryEffect(ctx context.Context, tx pgx.Tx, matchID uuid.UUID, sess SearchSession, action string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO matchmaking_match_recovery_effects (match_id, search_session_id, generation, action)
		VALUES ($1, $2, $3, $4)
	`, matchID, sess.ID, sess.RecoveryGeneration, action)
	return err
}

// ApplyPendingRecoveryEffects projects committed session recovery to an
// external queue while holding the match and session rows. A later transition
// increments recovery_generation, making any delayed effect obsolete.
func (s *MatchStore) ApplyPendingRecoveryEffects(ctx context.Context, limit int, apply func(context.Context, SearchSession, string) error) error {
	if s == nil || s.Pool == nil {
		return errors.New("match store unavailable")
	}
	if apply == nil {
		return errors.New("recovery effect projector unavailable")
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT match_id, search_session_id, generation
		FROM matchmaking_match_recovery_effects
		WHERE applied_at IS NULL
		ORDER BY created_at, match_id, search_session_id
		LIMIT $1
	`, limit)
	if err != nil {
		return err
	}
	type candidate struct {
		matchID, sessionID uuid.UUID
		generation         int64
	}
	var candidates []candidate
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.matchID, &item.sessionID, &item.generation); err != nil {
			rows.Close()
			return err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, item := range candidates {
		tx, err := s.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		var lockedMatch uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT id FROM matches WHERE id = $1 FOR UPDATE`, item.matchID).Scan(&lockedMatch); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		sess, err := scanSession(tx.QueryRow(ctx, `SELECT `+sessionSelectCols+` FROM search_sessions WHERE id = $1 FOR UPDATE`, item.sessionID))
		if err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		var action string
		err = tx.QueryRow(ctx, `
			SELECT action FROM matchmaking_match_recovery_effects
			WHERE match_id = $1 AND search_session_id = $2 AND generation = $3 AND applied_at IS NULL
			FOR UPDATE
		`, item.matchID, item.sessionID, item.generation).Scan(&action)
		if errors.Is(err, pgx.ErrNoRows) {
			if err := tx.Commit(ctx); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		valid := sess.RecoveryGeneration == item.generation &&
			((action == "enqueue" && sess.Status == SessionStatusSearching) || (action == "release" && sess.Status == SessionStatusCancelled))
		if valid {
			projectionCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			if err := apply(projectionCtx, sess, action); err != nil {
				cancel()
				_ = tx.Rollback(ctx)
				return err
			}
			cancel()
		}
		if _, err := tx.Exec(ctx, `
			UPDATE matchmaking_match_recovery_effects SET applied_at = clock_timestamp()
			WHERE match_id = $1 AND search_session_id = $2 AND generation = $3 AND applied_at IS NULL
		`, item.matchID, item.sessionID, item.generation); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func loadMatchProposalsForUpdate(ctx context.Context, tx pgx.Tx, matchID uuid.UUID) ([]MatchProposal, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, match_id, search_session_id, profile_id, party_id, response, created_at, updated_at
		FROM match_proposals WHERE match_id = $1 ORDER BY profile_id, id FOR UPDATE
	`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MatchProposal
	for rows.Next() {
		var p MatchProposal
		if err := rows.Scan(&p.ID, &p.MatchID, &p.SearchSessionID, &p.ProfileID, &p.PartyID, &p.Response, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func loadMatchSessionsForUpdate(ctx context.Context, tx pgx.Tx, matchID uuid.UUID) ([]SearchSession, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+sessionSelectCols+`
		FROM search_sessions WHERE match_id = $1 ORDER BY id FOR UPDATE
	`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SearchSession
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

type AcceptanceResult struct {
	Match           Match
	Proposal        MatchProposal
	ChangedSessions []SearchSession
	AllAccepted     bool
	Expired         bool
	Replayed        bool
}

// RecordDecline applies a party decline and the resulting whole-match recovery
// under the same match/proposal/session locks used by deadline expiry.
func (s *MatchStore) RecordDecline(ctx context.Context, matchID, profileID uuid.UUID) (AcceptanceResult, error) {
	if s == nil || s.Pool == nil {
		return AcceptanceResult{}, errors.New("match store unavailable")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return AcceptanceResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	match, err := scanMatch(tx.QueryRow(ctx, `
		SELECT id, game_id, mode, region, participants, left_profile_ids, voice_room_id, chat_id, status, created_at, completed_at
		FROM matches WHERE id = $1 FOR UPDATE
	`, matchID))
	if errors.Is(err, pgx.ErrNoRows) {
		return AcceptanceResult{}, ErrMatchNotFound
	}
	if err != nil {
		return AcceptanceResult{}, err
	}
	proposals, err := loadMatchProposalsForUpdate(ctx, tx, matchID)
	if err != nil {
		return AcceptanceResult{}, err
	}
	sessions, err := loadMatchSessionsForUpdate(ctx, tx, matchID)
	if err != nil {
		return AcceptanceResult{}, err
	}
	if len(proposals) == 0 || len(sessions) != len(proposals) {
		return AcceptanceResult{}, errors.New("match proposal/session set is incomplete")
	}
	result := AcceptanceResult{Match: match}
	for _, proposal := range proposals {
		if proposal.ProfileID == profileID {
			result.Proposal = proposal
			break
		}
	}
	if result.Proposal.ID == uuid.Nil {
		return AcceptanceResult{}, ErrProposalNotFound
	}
	if match.Status != MatchStatusPendingAccept {
		result.Replayed = result.Proposal.Response == ProposalResponseDeclined
		if err := tx.Commit(ctx); err != nil {
			return AcceptanceResult{}, err
		}
		return result, nil
	}
	var dbNow time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&dbNow); err != nil {
		return AcceptanceResult{}, err
	}
	if result.Proposal.Response != ProposalResponsePending {
		result.Replayed = result.Proposal.Response == ProposalResponseDeclined
		if err := tx.Commit(ctx); err != nil {
			return AcceptanceResult{}, err
		}
		return result, nil
	}
	if err := tx.QueryRow(ctx, `
		UPDATE match_proposals SET response = $3, updated_at = $4
		WHERE id = $1 AND match_id = $2 AND response = $5
		RETURNING id, match_id, search_session_id, profile_id, party_id, response, created_at, updated_at
	`, result.Proposal.ID, matchID, ProposalResponseDeclined, dbNow, ProposalResponsePending).Scan(
		&result.Proposal.ID, &result.Proposal.MatchID, &result.Proposal.SearchSessionID, &result.Proposal.ProfileID,
		&result.Proposal.PartyID, &result.Proposal.Response, &result.Proposal.CreatedAt, &result.Proposal.UpdatedAt,
	); err != nil {
		return AcceptanceResult{}, err
	}
	changed, err := recoverDeclinedMatchLocked(ctx, tx, match, proposals, sessions, responsePartyKey(result.Proposal), dbNow)
	if err != nil {
		return AcceptanceResult{}, err
	}
	result.Match.Status = MatchStatusAbandoned
	result.Match.CompletedAt = &dbNow
	result.ChangedSessions = changed
	result.Expired = true
	if err := tx.Commit(ctx); err != nil {
		return AcceptanceResult{}, err
	}
	return result, nil
}

// RecordAcceptance serializes a new Accept with expiry. The database clock is
// sampled only after the match, proposals, and sessions are locked, so a
// request blocked across the deadline cannot commit as timely.
func (s *MatchStore) RecordAcceptance(ctx context.Context, matchID, profileID uuid.UUID) (AcceptanceResult, error) {
	if s == nil || s.Pool == nil {
		return AcceptanceResult{}, errors.New("match store unavailable")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return AcceptanceResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	match, err := scanMatch(tx.QueryRow(ctx, `
		SELECT id, game_id, mode, region, participants, left_profile_ids, voice_room_id, chat_id, status, created_at, completed_at
		FROM matches WHERE id = $1 FOR UPDATE
	`, matchID))
	if errors.Is(err, pgx.ErrNoRows) {
		return AcceptanceResult{}, ErrMatchNotFound
	}
	if err != nil {
		return AcceptanceResult{}, err
	}
	proposals, err := loadMatchProposalsForUpdate(ctx, tx, matchID)
	if err != nil {
		return AcceptanceResult{}, err
	}
	sessions, err := loadMatchSessionsForUpdate(ctx, tx, matchID)
	if err != nil {
		return AcceptanceResult{}, err
	}
	if len(proposals) == 0 || len(sessions) != len(proposals) {
		return AcceptanceResult{}, errors.New("match proposal/session set is incomplete")
	}
	var proposal MatchProposal
	found := false
	for _, candidate := range proposals {
		if candidate.ProfileID == profileID {
			proposal, found = candidate, true
			break
		}
	}
	if !found {
		return AcceptanceResult{}, ErrProposalNotFound
	}
	var dbNow time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&dbNow); err != nil {
		return AcceptanceResult{}, err
	}
	result := AcceptanceResult{Match: match, Proposal: proposal}
	if match.Status != MatchStatusPendingAccept {
		result.Replayed = proposal.Response == ProposalResponseAccepted
		result.AllAccepted = result.Replayed
		if err := tx.Commit(ctx); err != nil {
			return AcceptanceResult{}, err
		}
		return result, nil
	}

	allAccepted := true
	for _, candidate := range proposals {
		if candidate.Response != ProposalResponseAccepted {
			allAccepted = false
			break
		}
	}
	if !dbNow.Before(match.CreatedAt.Add(MatchAcceptWindow)) && !allAccepted {
		changed, err := expirePendingMatchLocked(ctx, tx, match, proposals, sessions, dbNow)
		if err != nil {
			return AcceptanceResult{}, err
		}
		result.Match.Status = MatchStatusAbandoned
		result.Match.CompletedAt = &dbNow
		result.ChangedSessions = changed
		result.Expired = true
		result.Replayed = proposal.Response == ProposalResponseAccepted
		if proposal.Response == ProposalResponsePending {
			result.Proposal.Response = ProposalResponseDeclined
			result.Proposal.UpdatedAt = dbNow
		}
		if err := tx.Commit(ctx); err != nil {
			return AcceptanceResult{}, err
		}
		return result, nil
	}

	if proposal.Response == ProposalResponsePending {
		err := tx.QueryRow(ctx, `
			UPDATE match_proposals SET response = $3, updated_at = $4
			WHERE id = $1 AND match_id = $2 AND response = $5
			RETURNING id, match_id, search_session_id, profile_id, party_id, response, created_at, updated_at
		`, proposal.ID, matchID, ProposalResponseAccepted, dbNow, ProposalResponsePending).Scan(
			&result.Proposal.ID, &result.Proposal.MatchID, &result.Proposal.SearchSessionID, &result.Proposal.ProfileID,
			&result.Proposal.PartyID, &result.Proposal.Response, &result.Proposal.CreatedAt, &result.Proposal.UpdatedAt,
		)
		if err != nil {
			return AcceptanceResult{}, err
		}
		allAccepted = true
		for _, candidate := range proposals {
			if candidate.ProfileID != profileID && candidate.Response != ProposalResponseAccepted {
				allAccepted = false
				break
			}
		}
	} else {
		result.Replayed = proposal.Response == ProposalResponseAccepted
	}
	result.AllAccepted = allAccepted
	if err := tx.Commit(ctx); err != nil {
		return AcceptanceResult{}, err
	}
	return result, nil
}

var (
	ErrMatchNotFound       = errors.New("match not found")
	ErrProposalNotFound    = errors.New("match proposal not found")
	ErrNotMatchParticipant = errors.New("not a match participant")
)

// MatchParticipant is one row in matches.participants jsonb.
type MatchParticipant struct {
	ProfileID string `json:"profile_id"`
	SessionID string `json:"session_id"`
}

// Match is a row in matches.
type Match struct {
	ID             uuid.UUID
	GameID         uuid.UUID
	Mode           string
	Region         string
	Participants   []MatchParticipant
	LeftProfileIDs []uuid.UUID
	VoiceRoomID    *string
	ChatID         *string
	Status         string
	CreatedAt      time.Time
	CompletedAt    *time.Time
}

// HasLeft reports whether profileID has left the squad.
func (m Match) HasLeft(profileID uuid.UUID) bool {
	for _, id := range m.LeftProfileIDs {
		if id == profileID {
			return true
		}
	}
	return false
}

func allParticipantsLeft(m Match) bool {
	if len(m.Participants) == 0 {
		return false
	}
	left := make(map[uuid.UUID]bool, len(m.LeftProfileIDs))
	for _, id := range m.LeftProfileIDs {
		left[id] = true
	}
	for _, p := range m.Participants {
		pid, err := uuid.Parse(p.ProfileID)
		if err != nil || !left[pid] {
			return false
		}
	}
	return true
}

// SlotCount returns the number of participant slots in the match.
func (m Match) SlotCount() int {
	return len(m.Participants)
}

// ProfileIDs returns participant profile IDs in order.
func (m Match) ProfileIDs() []uuid.UUID {
	out := make([]uuid.UUID, 0, len(m.Participants))
	for _, p := range m.Participants {
		id, err := uuid.Parse(p.ProfileID)
		if err != nil {
			continue
		}
		out = append(out, id)
	}
	return out
}

// MatchProposal is a row in match_proposals.
type MatchProposal struct {
	ID              uuid.UUID
	MatchID         uuid.UUID
	SearchSessionID uuid.UUID
	ProfileID       uuid.UUID
	PartyID         *uuid.UUID
	Response        string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// MatchStore persists matches and proposals.
type MatchStore struct {
	Pool *pgxpool.Pool
}

// ProposalSession links a search session to a new match proposal.
type ProposalSession struct {
	SessionID uuid.UUID
	ProfileID uuid.UUID
	PartyID   *uuid.UUID
}

// CreateProposalParams inputs for atomic match proposal creation.
type CreateProposalParams struct {
	GameID   uuid.UUID
	Mode     string
	Region   string
	Sessions []ProposalSession
}

// CreateProposalResult is the created match and per-participant proposals.
type CreateProposalResult struct {
	Match     Match
	Proposals []MatchProposal
}

// CreateProposal inserts match, proposals, and moves sessions to pending_accept.
func (s *MatchStore) CreateProposal(ctx context.Context, p CreateProposalParams) (CreateProposalResult, error) {
	if s == nil || s.Pool == nil {
		return CreateProposalResult{}, errors.New("match store unavailable")
	}
	if len(p.Sessions) == 0 {
		return CreateProposalResult{}, errors.New("no sessions")
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return CreateProposalResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	proposalSessions := append([]ProposalSession(nil), p.Sessions...)
	sort.Slice(proposalSessions, func(i, j int) bool {
		return proposalSessions[i].ProfileID.String() < proposalSessions[j].ProfileID.String()
	})
	sessionIDs := make([]uuid.UUID, 0, len(proposalSessions))
	requestedByID := make(map[uuid.UUID]ProposalSession, len(proposalSessions))
	for _, candidate := range proposalSessions {
		if _, exists := requestedByID[candidate.SessionID]; exists {
			return CreateProposalResult{}, errors.New("duplicate proposal session")
		}
		requestedByID[candidate.SessionID] = candidate
		sessionIDs = append(sessionIDs, candidate.SessionID)
	}
	lockedRows, err := tx.Query(ctx, `
		SELECT `+sessionSelectCols+`
		FROM search_sessions WHERE id = ANY($1) ORDER BY id FOR UPDATE
	`, sessionIDs)
	if err != nil {
		return CreateProposalResult{}, err
	}
	lockedSessions := make(map[uuid.UUID]SearchSession, len(proposalSessions))
	for lockedRows.Next() {
		sess, err := scanSession(lockedRows)
		if err != nil {
			lockedRows.Close()
			return CreateProposalResult{}, err
		}
		lockedSessions[sess.ID] = sess
	}
	if err := lockedRows.Err(); err != nil {
		lockedRows.Close()
		return CreateProposalResult{}, err
	}
	lockedRows.Close()
	if len(lockedSessions) != len(proposalSessions) {
		return CreateProposalResult{}, ErrSessionNotSearchable
	}
	for _, candidate := range proposalSessions {
		sess, ok := lockedSessions[candidate.SessionID]
		if !ok || sess.Status != SessionStatusSearching || sess.ProfileID != candidate.ProfileID || !samePartyID(sess.PartyID, candidate.PartyID) {
			return CreateProposalResult{}, ErrSessionNotSearchable
		}
	}

	matchID := uuid.New()
	now := time.Now().UTC()
	participants := make([]MatchParticipant, len(proposalSessions))
	for i, sess := range proposalSessions {
		participants[i] = MatchParticipant{
			ProfileID: sess.ProfileID.String(),
			SessionID: sess.SessionID.String(),
		}
	}
	participantsJSON, err := json.Marshal(participants)
	if err != nil {
		return CreateProposalResult{}, err
	}

	var match Match
	var leftJSON []byte
	err = tx.QueryRow(ctx, `
		INSERT INTO matches (id, game_id, mode, region, participants, status, created_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)
		RETURNING id, game_id, mode, region, participants, left_profile_ids, voice_room_id, chat_id, status, created_at, completed_at
	`, matchID, p.GameID, p.Mode, p.Region, string(participantsJSON), MatchStatusPendingAccept, now).Scan(
		&match.ID, &match.GameID, &match.Mode, &match.Region, &participantsJSON, &leftJSON,
		&match.VoiceRoomID, &match.ChatID, &match.Status, &match.CreatedAt, &match.CompletedAt,
	)
	if err != nil {
		return CreateProposalResult{}, err
	}
	if err := json.Unmarshal(participantsJSON, &match.Participants); err != nil {
		return CreateProposalResult{}, err
	}
	if err := unmarshalLeftProfileIDs(leftJSON, &match.LeftProfileIDs); err != nil {
		return CreateProposalResult{}, err
	}

	proposals := make([]MatchProposal, 0, len(proposalSessions))
	for _, sess := range proposalSessions {
		var proposal MatchProposal
		err = tx.QueryRow(ctx, `
			INSERT INTO match_proposals (match_id, search_session_id, profile_id, party_id, response, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $6)
			RETURNING id, match_id, search_session_id, profile_id, party_id, response, created_at, updated_at
		`, matchID, sess.SessionID, sess.ProfileID, sess.PartyID, ProposalResponsePending, now).Scan(
			&proposal.ID, &proposal.MatchID, &proposal.SearchSessionID, &proposal.ProfileID,
			&proposal.PartyID, &proposal.Response, &proposal.CreatedAt, &proposal.UpdatedAt,
		)
		if err != nil {
			return CreateProposalResult{}, err
		}
		proposals = append(proposals, proposal)

		var updatedID uuid.UUID
		err = tx.QueryRow(ctx, `
			UPDATE search_sessions
			SET status = $2, match_id = $3, matched_at = $4, updated_at = $4,
			    recovery_generation = recovery_generation + 1
			WHERE id = $1 AND status = $5
			RETURNING id
		`, sess.SessionID, SessionStatusPendingAccept, matchID, now, SessionStatusSearching).Scan(&updatedID)
		if err != nil {
			return CreateProposalResult{}, err
		}
		if updatedID != sess.SessionID {
			return CreateProposalResult{}, ErrSessionNotSearchable
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return CreateProposalResult{}, err
	}
	return CreateProposalResult{Match: match, Proposals: proposals}, nil
}

// Get loads a match by ID.
func (s *MatchStore) Get(ctx context.Context, id uuid.UUID) (Match, error) {
	if s == nil || s.Pool == nil {
		return Match{}, errors.New("match store unavailable")
	}
	match, err := scanMatch(s.Pool.QueryRow(ctx, `
		SELECT id, game_id, mode, region, participants, left_profile_ids, voice_room_id, chat_id, status, created_at, completed_at
		FROM matches WHERE id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Match{}, ErrMatchNotFound
	}
	return match, err
}

// DatabaseNow returns the database clock used by match deadline decisions.
func (s *MatchStore) DatabaseNow(ctx context.Context) (time.Time, error) {
	if s == nil || s.Pool == nil {
		return time.Time{}, errors.New("match store unavailable")
	}
	var now time.Time
	if err := s.Pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return time.Time{}, err
	}
	return now, nil
}

func scanMatch(row pgx.Row) (Match, error) {
	var participantsJSON, leftJSON []byte
	var m Match
	err := row.Scan(
		&m.ID, &m.GameID, &m.Mode, &m.Region, &participantsJSON, &leftJSON,
		&m.VoiceRoomID, &m.ChatID, &m.Status, &m.CreatedAt, &m.CompletedAt,
	)
	if err != nil {
		return Match{}, err
	}
	if err := json.Unmarshal(participantsJSON, &m.Participants); err != nil {
		return Match{}, err
	}
	if err := unmarshalLeftProfileIDs(leftJSON, &m.LeftProfileIDs); err != nil {
		return Match{}, err
	}
	return m, nil
}

// ListProposals returns proposals for a match.
func (s *MatchStore) ListProposals(ctx context.Context, matchID uuid.UUID) ([]MatchProposal, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("match store unavailable")
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, match_id, search_session_id, profile_id, party_id, response, created_at, updated_at
		FROM match_proposals WHERE match_id = $1 ORDER BY created_at
	`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MatchProposal
	for rows.Next() {
		var p MatchProposal
		if err := rows.Scan(
			&p.ID, &p.MatchID, &p.SearchSessionID, &p.ProfileID, &p.PartyID,
			&p.Response, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetProposalForProfile returns the proposal for a profile in a match.
func (s *MatchStore) GetProposalForProfile(ctx context.Context, matchID, profileID uuid.UUID) (MatchProposal, error) {
	if s == nil || s.Pool == nil {
		return MatchProposal{}, errors.New("match store unavailable")
	}
	var p MatchProposal
	err := s.Pool.QueryRow(ctx, `
		SELECT id, match_id, search_session_id, profile_id, party_id, response, created_at, updated_at
		FROM match_proposals WHERE match_id = $1 AND profile_id = $2
	`, matchID, profileID).Scan(
		&p.ID, &p.MatchID, &p.SearchSessionID, &p.ProfileID, &p.PartyID,
		&p.Response, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return MatchProposal{}, ErrProposalNotFound
	}
	return p, err
}

// SetProposalResponse updates a participant response.
func (s *MatchStore) SetProposalResponse(ctx context.Context, matchID, profileID uuid.UUID, response string) (MatchProposal, error) {
	if s == nil || s.Pool == nil {
		return MatchProposal{}, errors.New("match store unavailable")
	}
	now := time.Now().UTC()
	var p MatchProposal
	err := s.Pool.QueryRow(ctx, `
		UPDATE match_proposals
		SET response = $3, updated_at = $4
		WHERE match_id = $1 AND profile_id = $2 AND response = $5
		RETURNING id, match_id, search_session_id, profile_id, party_id, response, created_at, updated_at
	`, matchID, profileID, response, now, ProposalResponsePending).Scan(
		&p.ID, &p.MatchID, &p.SearchSessionID, &p.ProfileID, &p.PartyID,
		&p.Response, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return MatchProposal{}, ErrProposalNotFound
	}
	return p, err
}

// AllProposalsAccepted reports whether every proposal is accepted.
func (s *MatchStore) AllProposalsAccepted(ctx context.Context, matchID uuid.UUID) (bool, error) {
	proposals, err := s.ListProposals(ctx, matchID)
	if err != nil {
		return false, err
	}
	if len(proposals) == 0 {
		return false, nil
	}
	for _, p := range proposals {
		if p.Response != ProposalResponseAccepted {
			return false, nil
		}
	}
	return true, nil
}

// ActivateMatch sets squad IDs and active status; marks sessions matched.
func (s *MatchStore) ActivateMatch(ctx context.Context, matchID uuid.UUID, voiceRoomID, chatID string) (Match, error) {
	if s == nil || s.Pool == nil {
		return Match{}, errors.New("match store unavailable")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Match{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	var participantsJSON, leftJSON []byte
	var m Match
	err = tx.QueryRow(ctx, `
		UPDATE matches
		SET status = $2, voice_room_id = $3, chat_id = $4
		WHERE id = $1 AND status = $5
		RETURNING id, game_id, mode, region, participants, left_profile_ids, voice_room_id, chat_id, status, created_at, completed_at
	`, matchID, MatchStatusActive, voiceRoomID, chatID, MatchStatusPendingAccept).Scan(
		&m.ID, &m.GameID, &m.Mode, &m.Region, &participantsJSON, &leftJSON,
		&m.VoiceRoomID, &m.ChatID, &m.Status, &m.CreatedAt, &m.CompletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Match{}, ErrMatchNotFound
	}
	if err != nil {
		return Match{}, err
	}
	if err := json.Unmarshal(participantsJSON, &m.Participants); err != nil {
		return Match{}, err
	}
	if err := unmarshalLeftProfileIDs(leftJSON, &m.LeftProfileIDs); err != nil {
		return Match{}, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE search_sessions
		SET status = $2, updated_at = $3, recovery_generation = recovery_generation + 1
		WHERE match_id = $1 AND status = $4
	`, matchID, SessionStatusMatched, now, SessionStatusPendingAccept)
	if err != nil {
		return Match{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Match{}, err
	}
	return m, nil
}

func unmarshalLeftProfileIDs(raw []byte, out *[]uuid.UUID) error {
	if len(raw) == 0 {
		*out = nil
		return nil
	}
	var ids []string
	if err := json.Unmarshal(raw, &ids); err != nil {
		return err
	}
	parsed := make([]uuid.UUID, 0, len(ids))
	for _, s := range ids {
		id, err := uuid.Parse(s)
		if err != nil {
			continue
		}
		parsed = append(parsed, id)
	}
	*out = parsed
	return nil
}

func marshalLeftProfileIDs(ids []uuid.UUID) (string, error) {
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = id.String()
	}
	b, err := json.Marshal(strs)
	return string(b), err
}

// CompleteMatchLeave records a participant leaving an active match squad.
func (s *MatchStore) CompleteMatchLeave(ctx context.Context, matchID, profileID uuid.UUID) (Match, error) {
	match, _, err := s.CompleteMatchLeaveWithTransition(ctx, matchID, profileID)
	return match, err
}

// CompleteMatchLeaveWithTransition records a participant leave and reports
// whether this call performed the active-to-completed transition. Callers use
// that signal for exactly-once terminal side effects such as event publication.
func (s *MatchStore) CompleteMatchLeaveWithTransition(ctx context.Context, matchID, profileID uuid.UUID) (Match, bool, error) {
	if s == nil || s.Pool == nil {
		return Match{}, false, errors.New("match store unavailable")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Match{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Serialize the read-modify-write union of left_profile_ids. Without this
	// lock, simultaneous leaves can each read the same old JSON value and the
	// later update discards the earlier participant's departure.
	match, err := scanMatch(tx.QueryRow(ctx, `
		SELECT id, game_id, mode, region, participants, left_profile_ids, voice_room_id, chat_id, status, created_at, completed_at
		FROM matches WHERE id = $1 FOR UPDATE
	`, matchID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Match{}, false, ErrMatchNotFound
	}
	if err != nil {
		return Match{}, false, err
	}
	if !matchHasProfileID(match, profileID) {
		return Match{}, false, ErrNotMatchParticipant
	}
	if match.Status != MatchStatusActive && match.Status != MatchStatusCompleted {
		return Match{}, false, errors.New("match not leaveable")
	}

	left := append([]uuid.UUID{}, match.LeftProfileIDs...)
	if !match.HasLeft(profileID) {
		left = append(left, profileID)
	}
	leftJSON, err := marshalLeftProfileIDs(left)
	if err != nil {
		return Match{}, false, err
	}

	now := time.Now().UTC()
	status := match.Status
	var completedAt *time.Time
	if allParticipantsLeft(Match{Participants: match.Participants, LeftProfileIDs: left}) {
		status = MatchStatusCompleted
		completedAt = &now
	}

	var participantsJSON, leftRaw []byte
	var m Match
	err = tx.QueryRow(ctx, `
		UPDATE matches
		SET left_profile_ids = $2::jsonb,
		    status = $3,
		    completed_at = COALESCE($4, completed_at)
		WHERE id = $1 AND status IN ('active', 'completed')
		RETURNING id, game_id, mode, region, participants, left_profile_ids, voice_room_id, chat_id, status, created_at, completed_at
	`, matchID, leftJSON, status, completedAt).Scan(
		&m.ID, &m.GameID, &m.Mode, &m.Region, &participantsJSON, &leftRaw,
		&m.VoiceRoomID, &m.ChatID, &m.Status, &m.CreatedAt, &m.CompletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Match{}, false, ErrMatchNotFound
	}
	if err != nil {
		return Match{}, false, err
	}
	if err := json.Unmarshal(participantsJSON, &m.Participants); err != nil {
		return Match{}, false, err
	}
	if err := unmarshalLeftProfileIDs(leftRaw, &m.LeftProfileIDs); err != nil {
		return Match{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Match{}, false, err
	}
	return m, match.Status != MatchStatusCompleted && m.Status == MatchStatusCompleted, nil
}

func matchHasProfileID(match Match, profileID uuid.UUID) bool {
	for _, id := range match.ProfileIDs() {
		if id == profileID {
			return true
		}
	}
	return false
}

// ListMatchHistoryParams filters paginated match history for a profile.
type ListMatchHistoryParams struct {
	ProfileID uuid.UUID
	Cursor    string
	PageSize  int32
}

// ListMatchHistoryResult is a page of match history.
type ListMatchHistoryResult struct {
	Matches    []Match
	NextCursor string
}

// ListHistoryForProfile returns active and completed matches the profile participated in.
func (s *MatchStore) ListHistoryForProfile(ctx context.Context, p ListMatchHistoryParams) (ListMatchHistoryResult, error) {
	if s == nil || s.Pool == nil {
		return ListMatchHistoryResult{}, errors.New("match store unavailable")
	}
	limit := int(p.PageSize)
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	cursorTime, cursorID, err := decodeCursor(p.Cursor)
	if err != nil {
		return ListMatchHistoryResult{}, err
	}

	profileID := p.ProfileID.String()
	args := []any{profileID, limit + 1}
	query := `
		SELECT id, game_id, mode, region, participants, left_profile_ids, voice_room_id, chat_id, status, created_at, completed_at
		FROM matches
		WHERE status IN ('active', 'completed')
		  AND EXISTS (
		    SELECT 1 FROM jsonb_array_elements(participants) AS elem
		    WHERE elem->>'profile_id' = $1
		  )
	`
	if cursorTime != nil && cursorID != nil {
		query += ` AND (created_at, id) < ($3, $4)`
		args = append(args, *cursorTime, *cursorID)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $2`

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return ListMatchHistoryResult{}, err
	}
	defer rows.Close()

	var matches []Match
	for rows.Next() {
		m, err := scanMatchRow(rows)
		if err != nil {
			return ListMatchHistoryResult{}, err
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return ListMatchHistoryResult{}, err
	}

	var nextCursor string
	if len(matches) > limit {
		last := matches[limit-1]
		nextCursor = encodeCursor(last.CreatedAt, last.ID)
		matches = matches[:limit]
	}
	return ListMatchHistoryResult{Matches: matches, NextCursor: nextCursor}, nil
}

func scanMatchRow(rows pgx.Rows) (Match, error) {
	var participantsJSON, leftJSON []byte
	var m Match
	err := rows.Scan(
		&m.ID, &m.GameID, &m.Mode, &m.Region, &participantsJSON, &leftJSON,
		&m.VoiceRoomID, &m.ChatID, &m.Status, &m.CreatedAt, &m.CompletedAt,
	)
	if err != nil {
		return Match{}, err
	}
	if err := json.Unmarshal(participantsJSON, &m.Participants); err != nil {
		return Match{}, err
	}
	if err := unmarshalLeftProfileIDs(leftJSON, &m.LeftProfileIDs); err != nil {
		return Match{}, err
	}
	return m, nil
}

// AbandonMatch marks match abandoned and cancels pending sessions.
func (s *MatchStore) AbandonMatch(ctx context.Context, matchID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("match store unavailable")
	}
	now := time.Now().UTC()
	_, err := s.Pool.Exec(ctx, `
		UPDATE matches SET status = $2, completed_at = $3
		WHERE id = $1 AND status = $4
	`, matchID, MatchStatusAbandoned, now, MatchStatusPendingAccept)
	return err
}
