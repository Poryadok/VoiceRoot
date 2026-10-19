package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"voice/backend/moderation/internal/reportrecovery"
)

// ReportRecoveryAuthorize binds the exact independently verified restore cut to
// a captured private-repair boot/generation check. It must not use effects/P5.
type ReportRecoveryAuthorize func(context.Context, reportrecovery.Cut) (ReportPrivacyCheck, error)

// ImportReportRecoverySnapshot unions opaque receipts only. It cannot recreate
// raw mappings or substitute for terminal import, cleanup or the final checker.
func (s *ReportStore) ImportReportRecoverySnapshot(ctx context.Context, snapshot *reportrecovery.Snapshot, authorize ReportRecoveryAuthorize) error {
	if s == nil || s.Pool == nil || snapshot == nil || authorize == nil {
		return ErrReportPrivacy
	}
	cut := snapshot.Cut()
	if cut.View.Protocol != 1 || len(cut.Artifacts) == 0 {
		return ErrReportPrivacy
	}
	check, err := authorize(ctx, cut)
	if err != nil {
		return err
	}
	if check == nil {
		return ErrReportPrivacy
	}
	if err := check(ctx); err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Parsed records are strictly ordered and unique across the complete chunk
	// inventory. This phase never requests report/account/subject locks after a
	// journal lock. Later participant cleanup follows the ordinary global order.
	for _, record := range snapshot.Records() {
		if err := importRecoveryRecordTx(ctx, tx, record); err != nil {
			return err
		}
	}
	if err := check(ctx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func importRecoveryRecordTx(ctx context.Context, tx pgx.Tx, record reportrecovery.Record) error {
	for attempt := 0; attempt < 2; attempt++ {
		var old reportrecovery.Record
		err := tx.QueryRow(ctx, `SELECT report_id,slot,disposition,subject_id,evidence_kind,evidence_id,evidence_version
 FROM report_privacy_journal WHERE report_id=$1 AND slot=$2 FOR UPDATE`, record.ReportID, record.Slot).Scan(
			&old.ReportID, &old.Slot, &old.Disposition, &old.SubjectID, &old.EvidenceKind, &old.EvidenceID, &old.EvidenceVersion)
		if err == nil {
			if old.SubjectID != nil && record.SubjectID != nil && *old.SubjectID != *record.SubjectID {
				return ErrReportRecoveryDivergence
			}
			if old.Disposition == "REDACTED" || record.Disposition == "BOUND" {
				return nil
			}
			_, err = tx.Exec(ctx, `UPDATE report_privacy_journal SET disposition='REDACTED',
 subject_id=COALESCE(subject_id,$3),evidence_kind=$4,evidence_id=$5,evidence_version=$6,sequence=DEFAULT
 WHERE report_id=$1 AND slot=$2`, record.ReportID, record.Slot, record.SubjectID, record.EvidenceKind, record.EvidenceID, record.EvidenceVersion)
			return err
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		// The independently authorized source receipt can outlive its raw row.
		// Importing AUTH_TERMINAL does not fabricate an automod terminal fence;
		// the coordinator must import the owning authority's terminal union too.
		tag, err := tx.Exec(ctx, `INSERT INTO report_privacy_journal(report_id,slot,disposition,subject_id,evidence_kind,evidence_id,evidence_version)
 VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(report_id,slot) DO NOTHING`, record.ReportID, record.Slot, record.Disposition, record.SubjectID, record.EvidenceKind, record.EvidenceID, record.EvidenceVersion)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 1 {
			return nil
		}
	}
	return ErrReportRecoveryDivergence
}
