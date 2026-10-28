package registry

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrPlayerSessionDenied = errors.New("player session denied")

type PlayerSessionSnapshot struct {
	SessionID      uuid.UUID  `json:"session_id"`
	ApplicationID  uuid.UUID  `json:"application_id"`
	EnvironmentID  uuid.UUID  `json:"environment_id"`
	Kind           string     `json:"kind"`
	RosterRevision int64      `json:"roster_revision"`
	LeaseExpiresAt time.Time  `json:"lease_expires_at"`
	ChatID         *uuid.UUID `json:"chat_id,omitempty"`
}

// GetPlayerSessionSnapshot resolves only the signed Auth account/profile pair.
// Unknown, foreign, stale, incomplete, expired and unbound sessions deliberately
// share one denial result.
func (s *Store) GetPlayerSessionSnapshot(ctx context.Context, accountID, profileID, sessionID uuid.UUID) (PlayerSessionSnapshot, error) {
	if s == nil || s.Pool == nil || accountID == uuid.Nil || profileID == uuid.Nil || sessionID == uuid.Nil {
		return PlayerSessionSnapshot{}, ErrPlayerSessionDenied
	}
	var snapshot PlayerSessionSnapshot
	err := s.Pool.QueryRow(ctx, `
SELECT s.id,s.application_id,s.environment_id,s.kind,s.roster_revision,s.lease_expires_at,s.chat_id
FROM gis_sessions s
JOIN environments e ON e.id=s.environment_id AND e.application_id=s.application_id AND e.status='active'
WHERE s.id=$1 AND s.session_status='active' AND s.roster_complete
  AND s.lease_expires_at > clock_timestamp()
  AND s.members @> ARRAY[$3]::uuid[]
  AND EXISTS (
    SELECT 1 FROM player_bindings b
    WHERE b.application_id=s.application_id AND b.environment_id=s.environment_id
      AND b.account_id=$2 AND b.profile_id=$3 AND b.status='active'
  )`, sessionID, accountID, profileID).Scan(
		&snapshot.SessionID, &snapshot.ApplicationID, &snapshot.EnvironmentID, &snapshot.Kind, &snapshot.RosterRevision, &snapshot.LeaseExpiresAt, &snapshot.ChatID)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlayerSessionSnapshot{}, ErrPlayerSessionDenied
	}
	if err != nil {
		return PlayerSessionSnapshot{}, err
	}
	return snapshot, nil
}

// ListPlayerSessionSnapshots returns only sessions currently authorized for the
// signed account/profile pair. It intentionally does not expose roster members.
func (s *Store) ListPlayerSessionSnapshots(ctx context.Context, accountID, profileID uuid.UUID) ([]PlayerSessionSnapshot, error) {
	if s == nil || s.Pool == nil || accountID == uuid.Nil || profileID == uuid.Nil {
		return nil, ErrPlayerSessionDenied
	}
	rows, err := s.Pool.Query(ctx, `
SELECT s.id,s.application_id,s.environment_id,s.kind,s.roster_revision,s.lease_expires_at,s.chat_id
FROM gis_sessions s
JOIN environments e ON e.id=s.environment_id AND e.application_id=s.application_id AND e.status='active'
WHERE s.session_status='active' AND s.roster_complete
  AND s.lease_expires_at > clock_timestamp()
  AND s.members @> ARRAY[$2]::uuid[]
  AND EXISTS (
    SELECT 1 FROM player_bindings b
    WHERE b.application_id=s.application_id AND b.environment_id=s.environment_id
      AND b.account_id=$1 AND b.profile_id=$2 AND b.status='active'
  )
ORDER BY s.lease_expires_at,s.id`, accountID, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]PlayerSessionSnapshot, 0)
	for rows.Next() {
		var snapshot PlayerSessionSnapshot
		if err := rows.Scan(&snapshot.SessionID, &snapshot.ApplicationID, &snapshot.EnvironmentID, &snapshot.Kind, &snapshot.RosterRevision, &snapshot.LeaseExpiresAt, &snapshot.ChatID); err != nil {
			return nil, err
		}
		result = append(result, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
