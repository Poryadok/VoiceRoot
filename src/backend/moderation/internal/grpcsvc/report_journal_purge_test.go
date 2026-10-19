package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/moderation/internal/automod"
)

func TestReportJournalTerminalPurgePaths(t *testing.T) {
	for _, path := range []string{"mapped_account_first", "unmapped_subject", "late_null_target"} {
		t.Run(path, func(t *testing.T) {
			pool := reportIntakePool(t)
			ctx := context.Background()
			reports := newReportFixtureStore(pool)
			check, _ := reportOpenPrivacy(ctx)
			reporter := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
			kind := "story"
			if path == "late_null_target" {
				kind = "user"
			}
			row, err := reports.InsertSubjectReport(ctx, uuid.New(), reporter, check, kind, uuid.New(), "spam", nil, `{}`)
			require.NoError(t, err)
			reports.Subjects.BeforeTerminalPurge = reports.PurgeReportAccountLinksTx
			reports.Subjects.BeforeTerminalSubjectPurge = func(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
				return reports.PurgeReportSubjectsTx(ctx, tx, []uuid.UUID{id})
			}
			terminal := reporter.ID
			slot := "REPORTER"
			if path == "unmapped_subject" {
				_, err = pool.Exec(ctx, `DELETE FROM automod_subjects WHERE subject_id=$1`, reporter.ID)
				require.NoError(t, err) // Restored missing mapping, never inferred account.
			}
			if path == "late_null_target" {
				target := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
				terminal = target.ID
				slot = "USER_TARGET"
				claim, claimErr := reports.ClaimValidation(ctx)
				require.NoError(t, claimErr)
				claim.ReporterSubject, claim.TargetSubject, claim.PrivacyCheck = &reporter, &target, check
				require.NoError(t, reports.Subjects.Purge(ctx, target.ID))
				applied, completeErr := reports.CompleteValidation(ctx, claim, "LIVE", target.AccountID, retentionEvents(), nil)
				require.NoError(t, completeErr)
				require.False(t, applied)
			} else {
				require.NoError(t, reports.Subjects.Purge(ctx, terminal))
			}
			var count int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE id=$1`, row.ID).Scan(&count))
			require.Zero(t, count)
			var disposition, evidence string
			var evidenceID uuid.UUID
			require.NoError(t, pool.QueryRow(ctx, `SELECT disposition,evidence_kind,evidence_id FROM report_privacy_journal WHERE report_id=$1 AND slot=$2`, row.ID, slot).Scan(&disposition, &evidence, &evidenceID))
			require.Equal(t, "REDACTED", disposition, "terminal cleanup needs a durable report receipt after payload deletion")
			require.Equal(t, "AUTH_TERMINAL", evidence)
			require.Equal(t, terminal, evidenceID)
		})
	}
}
