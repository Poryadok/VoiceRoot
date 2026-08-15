package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type AuditRow struct {
	ID             uuid.UUID
	ActorProfileID uuid.UUID
	Action         string
	TargetType     string
	TargetID       uuid.UUID
	Details        string
	CreatedAt      time.Time
}

func (s *AuditLogStore) ListAuditLog(ctx context.Context, limit int) ([]AuditRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	if limit <= 0 {
		limit = 1000
	}
	if limit > 10000 {
		limit = 10000
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, actor_profile_id, action, target_type, target_id, details::text, created_at
FROM moderation_audit_log
ORDER BY created_at DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditRow
	for rows.Next() {
		var row AuditRow
		if err := rows.Scan(
			&row.ID,
			&row.ActorProfileID,
			&row.Action,
			&row.TargetType,
			&row.TargetID,
			&row.Details,
			&row.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *AuditLogStore) InsertAudit(
	ctx context.Context,
	actorProfileID uuid.UUID,
	action, targetType string,
	targetID uuid.UUID,
	detailsJSON string,
) error {
	if s == nil || s.Pool == nil {
		return errStoreNotConfigured
	}
	if detailsJSON == "" {
		detailsJSON = "{}"
	}
	_, err := s.Pool.Exec(ctx, `
INSERT INTO moderation_audit_log (actor_profile_id, action, target_type, target_id, details)
VALUES ($1, $2, $3, $4, $5::jsonb)`,
		actorProfileID, action, targetType, targetID, detailsJSON,
	)
	return err
}
