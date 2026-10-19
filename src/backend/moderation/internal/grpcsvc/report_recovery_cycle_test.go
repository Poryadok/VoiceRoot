package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/store"
)

func TestReportPrivacyRecoveryRejectsUnknownDeletionCycle(t *testing.T) {
	for _, slot := range []string{"REPORTER", "VALID_USER_TARGET", "REJECTED_USER_TARGET_HISTORY"} {
		t.Run(slot, func(t *testing.T) {
			pool := reportIntakePool(t)
			ctx := context.Background()
			reports := newReportFixtureStore(pool)
			check, err := reportOpenPrivacy(ctx)
			require.NoError(t, err)
			reporter := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
			kind := "user"
			if slot == "REPORTER" {
				kind = "story"
			}
			row, err := reports.InsertSubjectReport(ctx, uuid.New(), reporter, check, kind, uuid.New(), "spam", nil, `{}`)
			require.NoError(t, err)
			binding := reporter
			if slot != "REPORTER" {
				binding = automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
				claim, err := reports.ClaimValidation(ctx)
				require.NoError(t, err)
				claim.ReporterSubject, claim.PrivacyCheck = &reporter, check
				if slot == "VALID_USER_TARGET" {
					claim.TargetSubject = &binding
					_, err = reports.CompleteValidation(ctx, claim, "LIVE", binding.AccountID, retentionEvents(), nil)
				} else {
					require.NoError(t, reports.Subjects.BindSubject(ctx, binding))
					_, err = pool.Exec(ctx, `INSERT INTO report_privacy_journal(report_id,slot,disposition,subject_id,evidence_kind,evidence_id,evidence_version)
 VALUES($1,'USER_TARGET','BOUND',$2,'AUTH_SUBJECT',$2,1)`, row.ID, binding.ID)
					require.NoError(t, err) // Explicit independently retained subject-history fixture.
					_, err = reports.CompleteValidation(ctx, claim, "DELETED", uuid.Nil, nil, nil)
				}
				require.NoError(t, err)
			}
			_, err = pool.Exec(ctx, `UPDATE automod_subjects SET lifecycle_revision=2,lifecycle_state='DELETED',purge_at=clock_timestamp()+interval '1 hour' WHERE subject_id=$1`, binding.ID)
			require.NoError(t, err) // Migration9 deliberately leaves old deletion cycles unknown.
			require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy, "a future deadline does not invent the missing deletion cycle")
			fresh := binding
			deadline := time.Now().UTC().Truncate(time.Microsecond).Add(time.Hour)
			fresh.Revision, fresh.State, fresh.CycleID, fresh.PurgeAt = 3, "DELETED", uuid.New(), &deadline
			tx, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback(ctx) }()
			_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-account-sanction:' || $1::text,0))`, fresh.AccountID.String())
			require.NoError(t, err)
			// Positive fresh owning-authority fixture; never derive a production
			// cycle from the old NULL row or weaken same-revision legacy binding.
			require.NoError(t, reports.Subjects.BindRetainedSubjectTx(ctx, tx, fresh))
			require.NoError(t, tx.Commit(ctx))
			require.NoError(t, reports.RequirePrivacyRecovery(ctx))
			var fences int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_terminal_subjects`).Scan(&fences))
			require.Zero(t, fences)
		})
	}
}

func TestReportDispatchRejectsUnknownDeletionCycle(t *testing.T) {
	for _, slot := range []string{"REPORTER", "USER_TARGET"} {
		t.Run(slot, func(t *testing.T) {
			pool := reportIntakePool(t)
			ctx := context.Background()
			reports := newReportFixtureStore(pool)
			check, err := reportOpenPrivacy(ctx)
			require.NoError(t, err)
			reporter := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
			target := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
			_, err = reports.InsertSubjectReport(ctx, uuid.New(), reporter, check, "user", uuid.New(), "spam", nil, `{}`)
			require.NoError(t, err)
			claim, err := reports.ClaimValidation(ctx)
			require.NoError(t, err)
			claim.ReporterSubject, claim.TargetSubject, claim.PrivacyCheck = &reporter, &target, check
			_, err = reports.CompleteValidation(ctx, claim, "LIVE", target.AccountID, retentionEvents(), nil)
			require.NoError(t, err)
			binding := reporter
			if slot == "USER_TARGET" {
				binding = target
			}
			_, err = pool.Exec(ctx, `UPDATE automod_subjects SET lifecycle_revision=2,lifecycle_state='DELETED',purge_at=clock_timestamp()+interval '1 hour' WHERE subject_id=$1`, binding.ID)
			require.NoError(t, err)
			calls := 0
			publish := func(context.Context, store.IntakeEvent) error { calls++; return nil }
			attempted, err := reports.DeliverIntakeEvent(ctx, publish)
			require.ErrorIs(t, err, store.ErrReportPrivacy)
			require.False(t, attempted)
			require.Zero(t, calls, "unknown cycle must never reach publication")
			deadline := time.Now().UTC().Truncate(time.Microsecond).Add(time.Hour)
			binding.Revision, binding.State, binding.CycleID, binding.PurgeAt = 3, "DELETED", uuid.New(), &deadline
			tx, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback(ctx) }()
			_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-account-sanction:' || $1::text,0))`, binding.AccountID.String())
			require.NoError(t, err)
			require.NoError(t, reports.Subjects.BindRetainedSubjectTx(ctx, tx, binding))
			require.NoError(t, tx.Commit(ctx)) // Explicit fresh owner binding fixture.
			attempted, err = reports.DeliverIntakeEvent(ctx, publish)
			require.NoError(t, err)
			require.True(t, attempted)
			require.Equal(t, 1, calls)
		})
	}
}
