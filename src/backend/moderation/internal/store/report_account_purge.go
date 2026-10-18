package store

import (
	"context"
	"errors"

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
	// Same order as BE-045, BE-046 and staff sanction writers: account, reports.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-account-sanction:' || $1::text,0))`, accountID.String()); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT id FROM reports WHERE reporter_account_id=$1 OR validated_target_account_id=$1 ORDER BY id FOR UPDATE`, accountID)
	if err != nil {
		return err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	// Keep effective sanction and appeal snapshots, without report content or a
	// dangling report relation. No expiry/revocation/status projection changes.
	if _, err = tx.Exec(ctx, `UPDATE sanctions SET report_id=NULL,reason='TARGET_DELETED',updated_at=now() WHERE report_id=ANY($1::uuid[])`, ids); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `DELETE FROM reports WHERE id=ANY($1::uuid[])`, ids)
	return err
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
	rows, err := tx.Query(ctx, `SELECT id FROM reports WHERE reporter_subject_id=ANY($1::uuid[]) OR validated_target_subject_id=ANY($1::uuid[]) ORDER BY id FOR UPDATE`, subjects)
	if err != nil {
		return err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	if _, err = tx.Exec(ctx, `UPDATE sanctions SET report_id=NULL,reason='TARGET_DELETED',updated_at=now() WHERE report_id=ANY($1::uuid[])`, ids); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `DELETE FROM reports WHERE id=ANY($1::uuid[])`, ids)
	return err
}
