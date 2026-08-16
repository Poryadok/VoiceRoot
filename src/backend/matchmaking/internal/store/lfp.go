package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	LfpResponseJoin   = "JOIN"
	LfpResponseInvite = "INVITE"

	LfpRequestPending  = "pending"
	LfpRequestAccepted = "accepted"
	LfpRequestDeclined = "declined"
	LfpRequestExpired  = "expired"
)

var (
	ErrLfpListingNotFound = errors.New("lfp listing not found")
	ErrLfpRequestNotFound = errors.New("lfp request not found")
	ErrLfpRequestNotPending = errors.New("lfp request not pending")
)

// LfpListing tracks an active Looking-for-party story for MM Social Discovery.
type LfpListing struct {
	StoryID         uuid.UUID
	AuthorProfileID uuid.UUID
	CriteriaJSON    string
	CreatedAt       time.Time
	InactiveAt      *time.Time
}

// LfpRequest is a JOIN/INVITE response awaiting author Accept/Decline.
type LfpRequest struct {
	ID                 uuid.UUID
	StoryID            uuid.UUID
	AuthorProfileID    uuid.UUID
	ResponderProfileID uuid.UUID
	ResponseType       string
	Status             string
	PartyID            *uuid.UUID
	CreatedAt          time.Time
	DecidedAt          *time.Time
}

// LfpStore persists LFP listings and pending requests.
type LfpStore struct {
	Pool *pgxpool.Pool
}

// UpsertListing records or refreshes an LFP story listing from story.lfp_created.
func (s *LfpStore) UpsertListing(ctx context.Context, storyID, authorID uuid.UUID, criteriaJSON string) (LfpListing, error) {
	if s == nil || s.Pool == nil {
		return LfpListing{}, errors.New("lfp store unavailable")
	}
	criteriaJSON = strings.TrimSpace(criteriaJSON)
	if criteriaJSON == "" {
		criteriaJSON = "{}"
	}
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO lfp_listings (story_id, author_profile_id, criteria_json)
		VALUES ($1, $2, $3::jsonb)
		ON CONFLICT (story_id) DO UPDATE SET
			author_profile_id = EXCLUDED.author_profile_id,
			criteria_json = EXCLUDED.criteria_json,
			inactive_at = NULL
		RETURNING story_id, author_profile_id, criteria_json::text, created_at, inactive_at
	`, storyID, authorID, criteriaJSON)
	return scanLfpListing(row)
}

// GetListing loads an LFP listing by story id.
func (s *LfpStore) GetListing(ctx context.Context, storyID uuid.UUID) (LfpListing, error) {
	if s == nil || s.Pool == nil {
		return LfpListing{}, errors.New("lfp store unavailable")
	}
	row := s.Pool.QueryRow(ctx, `
		SELECT story_id, author_profile_id, criteria_json::text, created_at, inactive_at
		FROM lfp_listings WHERE story_id = $1
	`, storyID)
	listing, err := scanLfpListing(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return LfpListing{}, ErrLfpListingNotFound
	}
	return listing, err
}

// MarkListingInactive deactivates Join/Invite for a story (party found / left MM).
func (s *LfpStore) MarkListingInactive(ctx context.Context, storyID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("lfp store unavailable")
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE lfp_listings SET inactive_at = now()
		WHERE story_id = $1 AND inactive_at IS NULL
	`, storyID)
	return err
}

// UpsertRequest records a JOIN/INVITE from story.lfp_response (pending).
func (s *LfpStore) UpsertRequest(ctx context.Context, storyID, authorID, responderID uuid.UUID, responseType string) (LfpRequest, error) {
	if s == nil || s.Pool == nil {
		return LfpRequest{}, errors.New("lfp store unavailable")
	}
	responseType = strings.ToUpper(strings.TrimSpace(responseType))
	if responseType != LfpResponseJoin && responseType != LfpResponseInvite {
		return LfpRequest{}, errors.New("response_type must be JOIN or INVITE")
	}
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO lfp_requests (
			story_id, author_profile_id, responder_profile_id, response_type, status
		) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (story_id, responder_profile_id, response_type) DO UPDATE SET
			author_profile_id = EXCLUDED.author_profile_id,
			status = $5,
			party_id = NULL,
			decided_at = NULL,
			created_at = now()
		RETURNING id, story_id, author_profile_id, responder_profile_id, response_type, status,
		          party_id, created_at, decided_at
	`, storyID, authorID, responderID, responseType, LfpRequestPending)
	return scanLfpRequest(row)
}

// GetPendingRequest loads a pending request for author decision.
func (s *LfpStore) GetPendingRequest(ctx context.Context, storyID, responderID uuid.UUID, responseType string) (LfpRequest, error) {
	if s == nil || s.Pool == nil {
		return LfpRequest{}, errors.New("lfp store unavailable")
	}
	responseType = strings.ToUpper(strings.TrimSpace(responseType))
	row := s.Pool.QueryRow(ctx, `
		SELECT id, story_id, author_profile_id, responder_profile_id, response_type, status,
		       party_id, created_at, decided_at
		FROM lfp_requests
		WHERE story_id = $1 AND responder_profile_id = $2 AND response_type = $3
	`, storyID, responderID, responseType)
	req, err := scanLfpRequest(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return LfpRequest{}, ErrLfpRequestNotFound
	}
	return req, err
}

// DecideRequest marks a pending request accepted/declined and optionally attaches party_id.
func (s *LfpStore) DecideRequest(ctx context.Context, requestID uuid.UUID, status string, partyID *uuid.UUID) (LfpRequest, error) {
	if s == nil || s.Pool == nil {
		return LfpRequest{}, errors.New("lfp store unavailable")
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status != LfpRequestAccepted && status != LfpRequestDeclined {
		return LfpRequest{}, errors.New("status must be accepted or declined")
	}
	row := s.Pool.QueryRow(ctx, `
		UPDATE lfp_requests
		SET status = $2, party_id = $3, decided_at = now()
		WHERE id = $1 AND status = $4
		RETURNING id, story_id, author_profile_id, responder_profile_id, response_type, status,
		          party_id, created_at, decided_at
	`, requestID, status, partyID, LfpRequestPending)
	req, err := scanLfpRequest(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return LfpRequest{}, ErrLfpRequestNotPending
	}
	return req, err
}

func scanLfpListing(row pgx.Row) (LfpListing, error) {
	var l LfpListing
	err := row.Scan(&l.StoryID, &l.AuthorProfileID, &l.CriteriaJSON, &l.CreatedAt, &l.InactiveAt)
	return l, err
}

func scanLfpRequest(row pgx.Row) (LfpRequest, error) {
	var r LfpRequest
	err := row.Scan(
		&r.ID, &r.StoryID, &r.AuthorProfileID, &r.ResponderProfileID, &r.ResponseType, &r.Status,
		&r.PartyID, &r.CreatedAt, &r.DecidedAt,
	)
	return r, err
}
