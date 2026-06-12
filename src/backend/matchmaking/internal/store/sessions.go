package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	SessionStatusSearching     = "searching"
	SessionStatusPendingAccept = "pending_accept"
	SessionStatusMatched       = "matched"
	SessionStatusTimeout       = "timeout"
	SessionStatusCancelled     = "cancelled"
)

var (
	ErrSessionNotFound      = errors.New("search session not found")
	ErrActiveSearchExists   = errors.New("active search already exists")
	ErrSessionNotSearchable = errors.New("session is not searchable")
)

// SearchSession is a row in search_sessions.
type SearchSession struct {
	ID         uuid.UUID
	ProfileID  uuid.UUID
	PartyID    *uuid.UUID
	GameID     uuid.UUID
	Mode       string
	Criteria   string
	Status     string
	TimeoutAt  *time.Time
	NudgedAt   *time.Time
	MatchedAt  *time.Time
	MatchID    *uuid.UUID
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// SessionStore persists search sessions.
type SessionStore struct {
	Pool *pgxpool.Pool
}

type CreateSessionParams struct {
	ProfileID uuid.UUID
	PartyID   *uuid.UUID
	GameID    uuid.UUID
	Mode      string
	Criteria  string
	TimeoutAt time.Time
}

// Create inserts a new searching session.
func (s *SessionStore) Create(ctx context.Context, p CreateSessionParams) (SearchSession, error) {
	if s == nil || s.Pool == nil {
		return SearchSession{}, errors.New("session store unavailable")
	}
	id := uuid.New()
	now := time.Now().UTC()
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO search_sessions (
			id, profile_id, party_id, game_id, mode, criteria, status, timeout_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9, $9)
		RETURNING id, profile_id, party_id, game_id, mode, criteria::text, status,
		          timeout_at, nudged_at, matched_at, match_id, created_at, updated_at
	`, id, p.ProfileID, p.PartyID, p.GameID, p.Mode, p.Criteria, SessionStatusSearching, p.TimeoutAt, now)
	sess, err := scanSession(row)
	if err != nil {
		if isUniqueViolation(err) {
			return SearchSession{}, ErrActiveSearchExists
		}
		return SearchSession{}, err
	}
	return sess, nil
}

// Get loads a session by ID.
func (s *SessionStore) Get(ctx context.Context, id uuid.UUID) (SearchSession, error) {
	if s == nil || s.Pool == nil {
		return SearchSession{}, errors.New("session store unavailable")
	}
	row := s.Pool.QueryRow(ctx, `
		SELECT id, profile_id, party_id, game_id, mode, criteria::text, status,
		       timeout_at, nudged_at, matched_at, match_id, created_at, updated_at
		FROM search_sessions WHERE id = $1
	`, id)
	sess, err := scanSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return SearchSession{}, ErrSessionNotFound
	}
	return sess, err
}

// GetActiveSearching returns the active searching or pending-accept session for a profile.
func (s *SessionStore) GetActiveSearching(ctx context.Context, profileID uuid.UUID) (SearchSession, error) {
	if s == nil || s.Pool == nil {
		return SearchSession{}, errors.New("session store unavailable")
	}
	row := s.Pool.QueryRow(ctx, `
		SELECT id, profile_id, party_id, game_id, mode, criteria::text, status,
		       timeout_at, nudged_at, matched_at, match_id, created_at, updated_at
		FROM search_sessions
		WHERE profile_id = $1 AND status IN ($2, $3)
		ORDER BY created_at DESC
		LIMIT 1
	`, profileID, SessionStatusSearching, SessionStatusPendingAccept)
	sess, err := scanSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return SearchSession{}, ErrSessionNotFound
	}
	return sess, err
}

// Cancel sets session status to cancelled.
func (s *SessionStore) Cancel(ctx context.Context, id uuid.UUID) (SearchSession, error) {
	if s == nil || s.Pool == nil {
		return SearchSession{}, errors.New("session store unavailable")
	}
	now := time.Now().UTC()
	row := s.Pool.QueryRow(ctx, `
		UPDATE search_sessions
		SET status = $2, updated_at = $3
		WHERE id = $1 AND status = $4
		RETURNING id, profile_id, party_id, game_id, mode, criteria::text, status,
		          timeout_at, nudged_at, matched_at, match_id, created_at, updated_at
	`, id, SessionStatusCancelled, now, SessionStatusSearching)
	sess, err := scanSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return SearchSession{}, ErrSessionNotSearchable
	}
	return sess, err
}

// ResetToSearching clears match link and returns session to searching.
func (s *SessionStore) ResetToSearching(ctx context.Context, id uuid.UUID) (SearchSession, error) {
	if s == nil || s.Pool == nil {
		return SearchSession{}, errors.New("session store unavailable")
	}
	now := time.Now().UTC()
	row := s.Pool.QueryRow(ctx, `
		UPDATE search_sessions
		SET status = $2, match_id = NULL, matched_at = NULL, updated_at = $3
		WHERE id = $1 AND status IN ($4, $5)
		RETURNING id, profile_id, party_id, game_id, mode, criteria::text, status,
		          timeout_at, nudged_at, matched_at, match_id, created_at, updated_at
	`, id, SessionStatusSearching, now, SessionStatusPendingAccept, SessionStatusMatched)
	sess, err := scanSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return SearchSession{}, ErrSessionNotSearchable
	}
	return sess, err
}

// ListSearchingByIDs loads searching sessions for the given IDs.
func (s *SessionStore) ListSearchingByIDs(ctx context.Context, ids []uuid.UUID) ([]SearchSession, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("session store unavailable")
	}
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, profile_id, party_id, game_id, mode, criteria::text, status,
		       timeout_at, nudged_at, matched_at, match_id, created_at, updated_at
		FROM search_sessions
		WHERE id = ANY($1) AND status = $2
	`, ids, SessionStatusSearching)
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

// ListSearchingNeedingNudge returns searching sessions created before cutoff without a nudge.
func (s *SessionStore) ListSearchingNeedingNudge(ctx context.Context, createdBefore time.Time, limit int) ([]SearchSession, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("session store unavailable")
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, profile_id, party_id, game_id, mode, criteria::text, status,
		       timeout_at, nudged_at, matched_at, match_id, created_at, updated_at
		FROM search_sessions
		WHERE status = $1 AND nudged_at IS NULL AND created_at <= $2
		ORDER BY created_at ASC
		LIMIT $3
	`, SessionStatusSearching, createdBefore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessionRows(rows)
}

// MarkNudged sets nudged_at for a searching session (idempotent).
func (s *SessionStore) MarkNudged(ctx context.Context, id uuid.UUID) (SearchSession, error) {
	if s == nil || s.Pool == nil {
		return SearchSession{}, errors.New("session store unavailable")
	}
	now := time.Now().UTC()
	row := s.Pool.QueryRow(ctx, `
		UPDATE search_sessions
		SET nudged_at = $2, updated_at = $2
		WHERE id = $1 AND status = $3 AND nudged_at IS NULL
		RETURNING id, profile_id, party_id, game_id, mode, criteria::text, status,
		          timeout_at, nudged_at, matched_at, match_id, created_at, updated_at
	`, id, now, SessionStatusSearching)
	sess, err := scanSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return SearchSession{}, ErrSessionNotSearchable
	}
	return sess, err
}

// ListSearchingExpired returns searching sessions past timeout_at.
func (s *SessionStore) ListSearchingExpired(ctx context.Context, now time.Time, limit int) ([]SearchSession, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("session store unavailable")
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, profile_id, party_id, game_id, mode, criteria::text, status,
		       timeout_at, nudged_at, matched_at, match_id, created_at, updated_at
		FROM search_sessions
		WHERE status = $1 AND timeout_at IS NOT NULL AND timeout_at <= $2
		ORDER BY timeout_at ASC
		LIMIT $3
	`, SessionStatusSearching, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessionRows(rows)
}

// ExpireSearching moves a searching session to timeout status.
func (s *SessionStore) ExpireSearching(ctx context.Context, id uuid.UUID) (SearchSession, error) {
	if s == nil || s.Pool == nil {
		return SearchSession{}, errors.New("session store unavailable")
	}
	now := time.Now().UTC()
	row := s.Pool.QueryRow(ctx, `
		UPDATE search_sessions
		SET status = $2, updated_at = $3
		WHERE id = $1 AND status = $4
		RETURNING id, profile_id, party_id, game_id, mode, criteria::text, status,
		          timeout_at, nudged_at, matched_at, match_id, created_at, updated_at
	`, id, SessionStatusTimeout, now, SessionStatusSearching)
	sess, err := scanSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return SearchSession{}, ErrSessionNotSearchable
	}
	return sess, err
}

func scanSessionRows(rows pgx.Rows) ([]SearchSession, error) {
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

func scanSession(row pgx.Row) (SearchSession, error) {
	var sess SearchSession
	err := row.Scan(
		&sess.ID, &sess.ProfileID, &sess.PartyID, &sess.GameID, &sess.Mode, &sess.Criteria,
		&sess.Status, &sess.TimeoutAt, &sess.NudgedAt, &sess.MatchedAt, &sess.MatchID, &sess.CreatedAt, &sess.UpdatedAt,
	)
	return sess, err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
