package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PurgeAccountTx precedes BE-137's provenance scrub and BE-045's shared terminal
// subject removal under the same account lock. A failure rolls back all erasure.
// Own machine-case sanctions are removed by the accepted shared purge contract;
// this never changes Auth status or creates a restriction reconciliation intent.
func (s *ReportThresholdStore) PurgeAccountTx(ctx context.Context, tx pgx.Tx, account uuid.UUID) error {
	if tx == nil || account == uuid.Nil {
		return errStoreNotConfigured
	}
	queries := []string{
		`DELETE FROM report_threshold_evaluations e USING reports r WHERE e.report_id=r.id AND (r.reporter_account_id=$1 OR r.validated_target_account_id=$1)`,
		`DELETE FROM appeals WHERE sanction_id IN (SELECT s.id FROM sanctions s JOIN report_threshold_cases c ON c.id=s.machine_case_id WHERE c.account_id=$1 AND s.issuer_service='service:moderation/automod')`,
		`DELETE FROM sanctions s USING report_threshold_cases c WHERE s.machine_case_id=c.id AND c.account_id=$1 AND s.issuer_service='service:moderation/automod'`,
		`DELETE FROM auto_mod_log WHERE details->>'case_id' IN (SELECT id::text FROM report_threshold_cases WHERE account_id=$1)`,
		`DELETE FROM report_threshold_cases WHERE account_id=$1`,
		`DELETE FROM report_threshold_account_cutoffs WHERE account_id=$1`,
	}
	for _, query := range queries {
		if _, err := tx.Exec(ctx, query, account); err != nil {
			return err
		}
	}
	return nil
}
