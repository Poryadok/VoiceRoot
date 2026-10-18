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
func (s *ProfileStore) ResolveReportTarget(ctx context.Context, id uuid.UUID) (commonv1.ReportTargetState, uuid.UUID, error) {
	var deleted bool
	var account uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT p.account_id,p.deleted_at IS NOT NULL OR EXISTS(SELECT 1 FROM user_account_lifecycle a WHERE a.account_id=p.account_id AND a.state='ACCOUNT_INACTIVE') FROM profiles p WHERE p.id=$1`, id).Scan(&account, &deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return commonv1.ReportTargetState_REPORT_TARGET_STATE_MISSING, uuid.Nil, nil
	}
	if err != nil {
		return commonv1.ReportTargetState_REPORT_TARGET_STATE_UNSPECIFIED, uuid.Nil, err
	}
	if deleted {
		return commonv1.ReportTargetState_REPORT_TARGET_STATE_DELETED, uuid.Nil, nil
	}
	if account == uuid.Nil {
		return commonv1.ReportTargetState_REPORT_TARGET_STATE_UNSPECIFIED, uuid.Nil, errors.New("invalid profile account binding")
	}
	return commonv1.ReportTargetState_REPORT_TARGET_STATE_LIVE, account, nil
}
