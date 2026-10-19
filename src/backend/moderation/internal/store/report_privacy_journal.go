package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrReportRecoveryDivergence = errors.New("report recovery identity divergence")

// ReportRecoveryRecord contains no raw profile/account identifier or content.
// Evidence references come from verified owning authority, never generated as
// substitutes for an irreversible fact. The journal is not report eligibility.
type ReportRecoveryRecord struct {
	ReportID        uuid.UUID  `json:"report_id"`
	Slot            string     `json:"slot"`
	Disposition     string     `json:"disposition"`
	SubjectID       *uuid.UUID `json:"subject_id,omitempty"`
	EvidenceKind    string     `json:"evidence_kind"`
	EvidenceID      uuid.UUID  `json:"evidence_id"`
	EvidenceVersion int64      `json:"evidence_version"`
}

// RecordReportRecoveryTx records a verified participant fact in the caller's
// report mutation transaction, after its account->subject->report locks.
func (s *ReportStore) RecordReportRecoveryTx(ctx context.Context, tx pgx.Tx, record ReportRecoveryRecord) error {
	return s.recordReportRecoveryTx(ctx, tx, record, false)
}

func validReportRecoveryRecord(r ReportRecoveryRecord) bool {
	if r.ReportID == uuid.Nil || r.EvidenceID == uuid.Nil || r.EvidenceVersion <= 0 ||
		(r.Slot != "REPORTER" && r.Slot != "USER_TARGET") || (r.SubjectID != nil && *r.SubjectID == uuid.Nil) {
		return false
	}
	if r.Disposition == "BOUND" {
		return r.SubjectID != nil && (r.EvidenceKind == "USER_OWNER_FACT" || (r.EvidenceKind == "AUTH_SUBJECT" && *r.SubjectID == r.EvidenceID))
	}
	return r.Disposition == "REDACTED" && (r.EvidenceKind == "AUTH_TERMINAL" || r.EvidenceKind == "USER_ERASURE_FACT")
}

func (s *ReportStore) recordReportRecoveryTx(ctx context.Context, tx pgx.Tx, record ReportRecoveryRecord, imported bool) error {
	if s == nil || tx == nil || !validReportRecoveryRecord(record) {
		return ErrReportPrivacy
	}
	if record.EvidenceKind == "AUTH_TERMINAL" {
		if record.SubjectID != nil && *record.SubjectID != record.EvidenceID {
			return ErrReportRecoveryDivergence
		}
		var terminal bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, record.EvidenceID).Scan(&terminal); err != nil {
			return err
		}
		if !terminal {
			return ErrReportPrivacy
		}
		subject := record.EvidenceID
		record.SubjectID = &subject
	}
	// The caller already holds every account and subject lock. Lock raw report
	// before journal, including when replaying an old slot with no raw row.
	var reportID uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM reports WHERE id=$1 FOR UPDATE`, record.ReportID).Scan(&reportID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	present := err == nil
	for attempt := 0; attempt < 2; attempt++ {
		var old ReportRecoveryRecord
		err = tx.QueryRow(ctx, `SELECT report_id,slot,disposition,subject_id,evidence_kind,evidence_id,evidence_version
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
		if !present && !imported {
			return ErrReportPrivacy
		}
		tag, insertErr := tx.Exec(ctx, `INSERT INTO report_privacy_journal(report_id,slot,disposition,subject_id,evidence_kind,evidence_id,evidence_version)
 VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(report_id,slot) DO NOTHING`, record.ReportID, record.Slot, record.Disposition, record.SubjectID, record.EvidenceKind, record.EvidenceID, record.EvidenceVersion)
		if insertErr != nil {
			return insertErr
		}
		if tag.RowsAffected() == 1 {
			return nil
		}
	}
	return ErrReportRecoveryDivergence
}
