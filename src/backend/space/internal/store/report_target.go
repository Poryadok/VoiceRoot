package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
	commonv1 "voice.app/voice/common/v1"
)

// ResolveReportTarget uses the same Space advisory fence as lifecycle writers.
// An indeterminate freeze/restore or ownership transition cannot certify LIVE.
func (s *SpaceStore) ResolveReportTarget(ctx context.Context, id uuid.UUID) (commonv1.ReportTargetState, error) {
	unknown := commonv1.ReportTargetState_REPORT_TARGET_STATE_UNSPECIFIED
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return unknown, err
	}
	defer func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }()
	if err = lockLifecycleSpace(ctx, tx, id); err != nil {
		return unknown, err
	}
	var purged bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_permanent_purge_fences WHERE space_id=$1)`, id).Scan(&purged); err != nil {
		return unknown, err
	}
	if purged {
		return commonv1.ReportTargetState_REPORT_TARGET_STATE_DELETED, nil
	}
	if err = checkOwnershipJournalAvailable(ctx, tx, []uuid.UUID{id}); err != nil {
		return unknown, err
	}
	var phase string
	var scheduled, purge *time.Time
	err = tx.QueryRow(ctx, `SELECT phase,scheduled_at,purge_after FROM space_lifecycle_aggregates WHERE space_id=$1`, id).Scan(&phase, &scheduled, &purge)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return unknown, err
	}
	if err == nil && phase != "LIVE" {
		switch phase {
		case "PURGED":
			return commonv1.ReportTargetState_REPORT_TARGET_STATE_DELETED, nil
		case "SCHEDULED", "PURGE_DECIDED", "PURGING":
			if scheduled == nil || purge == nil || !purge.Equal(scheduled.Add(7*24*time.Hour)) {
				return unknown, ErrLifecycleEvidenceInvalid
			}
			return commonv1.ReportTargetState_REPORT_TARGET_STATE_DELETED, nil
		default:
			return unknown, ErrLifecycleFrozen
		}
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM spaces WHERE id=$1)`, id).Scan(&exists); err != nil {
		return unknown, err
	}
	if !exists {
		return commonv1.ReportTargetState_REPORT_TARGET_STATE_MISSING, nil
	}
	return commonv1.ReportTargetState_REPORT_TARGET_STATE_LIVE, nil
}
