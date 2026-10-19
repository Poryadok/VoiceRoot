package store

import (
	"context"
	"errors"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PurgeReportAccountLinksTx participates in the verified terminal account purge,
// before the lifecycle owner removes its raw account/opaque-subject binding.
// The caller supplies that authority and transaction; this is not a public API
// or a way to derive account ownership from profile IDs. Unverified legacy NULL
// provenance requires its separate User-owned profile cleanup authority.
func (s *ReportStore) PurgeReportAccountLinksTx(ctx context.Context, tx pgx.Tx, accountID uuid.UUID) error {
	if tx == nil || accountID == uuid.Nil {
		return errors.New("verified account purge transaction required")
	}
	// Same order as BE-045, BE-046 and staff sanction writers: account,
	// opaque subject, reports, journal. Resolve no reciprocal RPC here.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-account-sanction:' || $1::text,0))`, accountID.String()); err != nil {
		return err
	}
	var terminal uuid.UUID
	err := tx.QueryRow(ctx, `SELECT a.subject_id FROM automod_subjects a WHERE a.account_id=$1
 AND EXISTS(SELECT 1 FROM automod_terminal_subjects t WHERE t.subject_id=a.subject_id)
 AND (SELECT count(*) FROM automod_subjects WHERE account_id=$1)=1`, accountID).Scan(&terminal)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrReportPrivacy
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-subject-lifecycle:' || $1::text,0))`, terminal.String()); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT id,COALESCE(reporter_account_id=$1,false),COALESCE(validated_target_account_id=$1,false)
 FROM reports WHERE reporter_account_id=$1 OR validated_target_account_id=$1 ORDER BY id FOR UPDATE`, accountID)
	if err != nil {
		return err
	}
	var ids []uuid.UUID
	type slots struct{ reporter, target bool }
	var matches []slots
	for rows.Next() {
		var id uuid.UUID
		var match slots
		if err = rows.Scan(&id, &match.reporter, &match.target); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
		matches = append(matches, match)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for i, id := range ids {
		if matches[i].reporter {
			if err = s.recordTerminalReportSlotTx(ctx, tx, id, "REPORTER", terminal); err != nil {
				return err
			}
		}
		if matches[i].target {
			if err = s.recordTerminalReportSlotTx(ctx, tx, id, "USER_TARGET", terminal); err != nil {
				return err
			}
		}
	}
	return finishReportPurgeTx(ctx, tx, ids)
}

// PurgeReportSubjectsTx also works after restore when the raw account mapping
// is absent. The lifecycle owner holds each supplied subject lock and has
// inserted its opaque terminal fence in this transaction before this callback.
func (s *ReportStore) PurgeReportSubjectsTx(ctx context.Context, tx pgx.Tx, subjects []uuid.UUID) error {
	if tx == nil || len(subjects) == 0 {
		return errors.New("verified subject purge transaction required")
	}
	for _, id := range subjects {
		if id == uuid.Nil {
			return errors.New("invalid purge subject")
		}
	}
	ordered := append([]uuid.UUID(nil), subjects...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].String() < ordered[j].String() })
	terminal := map[uuid.UUID]bool{}
	for _, id := range ordered {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-subject-lifecycle:' || $1::text,0))`, id.String()); err != nil {
			return err
		}
		var ended bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, id).Scan(&ended); err != nil {
			return err
		}
		if !ended {
			return ErrReportPrivacy
		}
		terminal[id] = true
	}
	rows, err := tx.Query(ctx, `SELECT id,reporter_subject_id,validated_target_subject_id FROM reports WHERE reporter_subject_id=ANY($1::uuid[]) OR validated_target_subject_id=ANY($1::uuid[]) ORDER BY id FOR UPDATE`, ordered)
	if err != nil {
		return err
	}
	var ids []uuid.UUID
	type slots struct{ reporter, target *uuid.UUID }
	var matches []slots
	for rows.Next() {
		var id uuid.UUID
		var match slots
		if err = rows.Scan(&id, &match.reporter, &match.target); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
		matches = append(matches, match)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for i, id := range ids {
		if subject := matches[i].reporter; subject != nil && terminal[*subject] {
			if err = s.recordTerminalReportSlotTx(ctx, tx, id, "REPORTER", *subject); err != nil {
				return err
			}
		}
		if subject := matches[i].target; subject != nil && terminal[*subject] {
			if err = s.recordTerminalReportSlotTx(ctx, tx, id, "USER_TARGET", *subject); err != nil {
				return err
			}
		}
	}
	return finishReportPurgeTx(ctx, tx, ids)
}

func (s *ReportStore) recordTerminalReportSlotTx(ctx context.Context, tx pgx.Tx, reportID uuid.UUID, slot string, subject uuid.UUID) error {
	return s.RecordReportRecoveryTx(ctx, tx, ReportRecoveryRecord{ReportID: reportID, Slot: slot, Disposition: "REDACTED", SubjectID: &subject, EvidenceKind: "AUTH_TERMINAL", EvidenceID: subject, EvidenceVersion: 1})
}

func finishReportPurgeTx(ctx context.Context, tx pgx.Tx, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	// Preserve effective sanction/appeal snapshots without their raw report
	// relation. This does not revoke, expire or replay an existing sanction.
	if _, err := tx.Exec(ctx, `UPDATE sanctions SET report_id=NULL,reason='TARGET_DELETED',updated_at=now() WHERE report_id=ANY($1::uuid[])`, ids); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `DELETE FROM reports WHERE id=ANY($1::uuid[])`, ids)
	return err
}
