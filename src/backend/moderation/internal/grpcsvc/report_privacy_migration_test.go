package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/store"
)

func TestReportPrivacyMigrationRollback(t *testing.T) {
	for _, disposition := range []string{"journal_after_payload_deletion", "recovered_provenance"} {
		t.Run(disposition, func(t *testing.T) {
			pool := reportIntakePool(t)
			ctx := context.Background()
			root := repoRoot(t)
			down, err := os.ReadFile(filepath.Join(root, "src/backend/migrations/moderation_db/000011_report_privacy_recovery.down.sql"))
			require.NoError(t, err)
			_, err = pool.Exec(ctx, string(down))
			require.NoError(t, err, "empty expand migration is reversible")
			integrationtestApplySQL(t, ctx, pool, filepath.Join(root, "src/backend/migrations/moderation_db/000011_report_privacy_recovery.up.sql"))
			reports := newReportFixtureStore(pool)
			row, err := reports.InsertReport(ctx, uuid.New(), "story", uuid.New(), "spam", nil, `{}`)
			require.NoError(t, err)
			subject := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
			if disposition == "journal_after_payload_deletion" {
				tx, beginErr := pool.Begin(ctx)
				require.NoError(t, beginErr)
				require.NoError(t, reports.RecordReportRecoveryTx(ctx, tx, store.ReportRecoveryRecord{ReportID: row.ID, Slot: "REPORTER", Disposition: "BOUND", SubjectID: &subject.ID, EvidenceKind: "USER_OWNER_FACT", EvidenceID: uuid.New(), EvidenceVersion: 1}))
				_, err = tx.Exec(ctx, `DELETE FROM reports WHERE id=$1`, row.ID)
				require.NoError(t, err)
				require.NoError(t, tx.Commit(ctx))
			} else {
				_, err = pool.Exec(ctx, `UPDATE reports SET reporter_account_id=$2,reporter_subject_id=$3,reporter_subject_revision=1,reporter_provenance='OWNER_RECOVERED_LEGACY' WHERE id=$1`, row.ID, subject.AccountID, subject.ID)
				require.NoError(t, err) // Explicit privacy-only provenance fixture.
			}
			_, err = pool.Exec(ctx, string(down))
			require.Error(t, err, "downgrade cannot discard the only durable privacy history")
			var generation uuid.UUID
			require.NoError(t, pool.QueryRow(ctx, `SELECT journal_generation FROM report_privacy_journal_metadata WHERE singleton`).Scan(&generation))
			require.NotEqual(t, uuid.Nil, generation)
		})
	}
}
