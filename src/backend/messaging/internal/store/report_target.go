package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	commonv1 "voice.app/voice/common/v1"
)

// ResolveReportTarget reads only owner lifecycle/existence. Privacy, viewer ACL,
// ghost/FOR_ME and retained Story expiration never narrow this global intake.
func (s *MessagesStore) ResolveReportTarget(ctx context.Context, id uuid.UUID) (commonv1.ReportTargetState, error) {
	var deleted bool

	err := s.Pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM messages WHERE id=$1`, id).Scan(&deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return commonv1.ReportTargetState_REPORT_TARGET_STATE_MISSING, nil
	}
	if err != nil {
		return commonv1.ReportTargetState_REPORT_TARGET_STATE_UNSPECIFIED, err
	}
	if deleted {
		return commonv1.ReportTargetState_REPORT_TARGET_STATE_DELETED, nil
	}

	return commonv1.ReportTargetState_REPORT_TARGET_STATE_LIVE, nil
}
