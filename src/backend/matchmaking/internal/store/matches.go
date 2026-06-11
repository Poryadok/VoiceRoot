package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	MatchStatusPendingAccept = "pending_accept"
	MatchStatusActive        = "active"
	MatchStatusCompleted     = "completed"
	MatchStatusAbandoned     = "abandoned"

	ProposalResponsePending  = "pending"
	ProposalResponseAccepted = "accepted"
	ProposalResponseDeclined = "declined"
)

var (
	ErrMatchNotFound      = errors.New("match not found")
	ErrProposalNotFound   = errors.New("match proposal not found")
	ErrNotMatchParticipant = errors.New("not a match participant")
)

// MatchParticipant is one row in matches.participants jsonb.
type MatchParticipant struct {
	ProfileID string `json:"profile_id"`
	SessionID string `json:"session_id"`
}

// Match is a row in matches.
type Match struct {
	ID              uuid.UUID
	GameID          uuid.UUID
	Mode            string
	Region          string
	Participants    []MatchParticipant
	LeftProfileIDs  []uuid.UUID
	VoiceRoomID     *string
	ChatID          *string
	Status          string
	CreatedAt       time.Time
	CompletedAt     *time.Time
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

	matchID := uuid.New()
	now := time.Now().UTC()
	participants := make([]MatchParticipant, len(p.Sessions))
	for i, sess := range p.Sessions {
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

	proposals := make([]MatchProposal, 0, len(p.Sessions))
	for _, sess := range p.Sessions {
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

		_, err = tx.Exec(ctx, `
			UPDATE search_sessions
			SET status = $2, match_id = $3, matched_at = $4, updated_at = $4
			WHERE id = $1 AND status = $5
		`, sess.SessionID, SessionStatusPendingAccept, matchID, now, SessionStatusSearching)
		if err != nil {
			return CreateProposalResult{}, err
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
	var participantsJSON, leftJSON []byte
	var m Match
	err := s.Pool.QueryRow(ctx, `
		SELECT id, game_id, mode, region, participants, left_profile_ids, voice_room_id, chat_id, status, created_at, completed_at
		FROM matches WHERE id = $1
	`, id).Scan(
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
		SET status = $2, updated_at = $3
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
	if s == nil || s.Pool == nil {
		return Match{}, errors.New("match store unavailable")
	}
	match, err := s.Get(ctx, matchID)
	if err != nil {
		return Match{}, err
	}
	if !matchHasProfileID(match, profileID) {
		return Match{}, ErrNotMatchParticipant
	}
	if match.Status != MatchStatusActive && match.Status != MatchStatusCompleted {
		return Match{}, errors.New("match not leaveable")
	}

	left := append([]uuid.UUID{}, match.LeftProfileIDs...)
	if !match.HasLeft(profileID) {
		left = append(left, profileID)
	}
	leftJSON, err := marshalLeftProfileIDs(left)
	if err != nil {
		return Match{}, err
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
	err = s.Pool.QueryRow(ctx, `
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
		return Match{}, ErrMatchNotFound
	}
	if err != nil {
		return Match{}, err
	}
	if err := json.Unmarshal(participantsJSON, &m.Participants); err != nil {
		return Match{}, err
	}
	if err := unmarshalLeftProfileIDs(leftRaw, &m.LeftProfileIDs); err != nil {
		return Match{}, err
	}
	return m, nil
}

func matchHasProfileID(match Match, profileID uuid.UUID) bool {
	for _, id := range match.ProfileIDs() {
		if id == profileID {
			return true
		}
	}
	return false
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
