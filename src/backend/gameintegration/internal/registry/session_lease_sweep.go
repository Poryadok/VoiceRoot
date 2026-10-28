package registry

import (
	"context"

	"github.com/google/uuid"
)

type expiredSessionLease struct {
	ApplicationID  uuid.UUID
	EnvironmentID  uuid.UUID
	SessionID      uuid.UUID
	RosterRevision int64
}

func (s *Store) expiredActiveSessions(ctx context.Context) ([]expiredSessionLease, error) {
	rows, err := s.Pool.Query(ctx, `SELECT s.application_id,s.environment_id,s.id,s.roster_revision
		FROM gis_sessions s
		WHERE s.session_status='active' AND (
			s.lease_expires_at<=clock_timestamp() OR EXISTS (
				SELECT 1 FROM gis_sessions p WHERE p.application_id=s.application_id AND p.environment_id=s.environment_id
					AND p.kind='party' AND p.external_key=s.parent_party_key AND p.session_status='active'
					AND p.lease_expires_at<=clock_timestamp()))
		ORDER BY (s.parent_party_key IS NOT NULL) DESC,s.lease_expires_at,s.id LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var expired []expiredSessionLease
	for rows.Next() {
		var lease expiredSessionLease
		if err := rows.Scan(&lease.ApplicationID, &lease.EnvironmentID, &lease.SessionID, &lease.RosterRevision); err != nil {
			return nil, err
		}
		expired = append(expired, lease)
	}
	return expired, rows.Err()
}
