package store

import (
	"context"

	"github.com/google/uuid"
)

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
