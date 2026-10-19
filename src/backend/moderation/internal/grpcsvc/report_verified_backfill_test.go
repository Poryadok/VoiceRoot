package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/store"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

func TestReportVerifiedJournalBackfillRetainedPairs(t *testing.T) {
	for _, state := range []string{"LIVE", "DELETED"} {
		t.Run(state, func(t *testing.T) {
			pool := reportIntakePool(t)
			ctx := context.Background()
			reports := newReportFixtureStore(pool)
			check, _ := reportOpenPrivacy(ctx)
			reporter := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
			target := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
			row, err := reports.InsertSubjectReport(ctx, uuid.New(), reporter, check, "user", uuid.New(), "spam", nil, `{}`)
			require.NoError(t, err)
			claim, err := reports.ClaimValidation(ctx)
			require.NoError(t, err)
			claim.ReporterSubject, claim.TargetSubject, claim.PrivacyCheck = &reporter, &target, check
			applied, err := reports.CompleteValidation(ctx, claim, "LIVE", target.AccountID, retentionEvents(), nil)
			require.NoError(t, err)
			require.True(t, applied)
			_, err = pool.Exec(ctx, `DELETE FROM report_privacy_journal; DELETE FROM automod_subjects`)
			require.NoError(t, err) // Trusted pre-journal raw tuple plus missing local mapping fixture.
			calls := 0
			resolver := reportSubjectResolverFunc(func(callCtx context.Context, account, expected uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error) {
				calls++
				if account == reporter.AccountID {
					require.Equal(t, reporter.ID, expected)
				} else {
					require.Equal(t, target.AccountID, account)
					require.Equal(t, target.ID, expected)
				}
				probe, probeErr := pool.Begin(callCtx)
				require.NoError(t, probeErr)
				var unlocked bool
				require.NoError(t, probe.QueryRow(callCtx, `SELECT pg_try_advisory_xact_lock(hashtextextended('moderation-account-sanction:' || $1::text,0))`, account.String()).Scan(&unlocked))
				require.True(t, unlocked, "fresh authority runs before SQL account locks")
				require.NoError(t, probe.Rollback(callCtx))
				response := reportSubjectResponse(account, expected)
				response.Subject.LifecycleRevision = 2
				if state == "DELETED" {
					cycle := uuid.NewString()
					response.Subject.State = automodv1.SubjectState_SUBJECT_STATE_DELETED
					response.Subject.DeletionCycleId = &cycle
					response.Subject.PurgeAt = timestamppb.New(time.Now().Add(time.Hour).Truncate(time.Microsecond))
					response.AuthRestricted = true
				}
				return response, nil
			})
			batch, err := reports.BackfillVerifiedReportJournal(ctx, resolver, reportOpenPrivacy, 10)
			require.NoError(t, err)
			require.Equal(t, 2, calls)
			require.Equal(t, 2, batch.Journaled)
			require.Zero(t, batch.Purged)
			require.False(t, batch.More)
			require.NoError(t, reports.RequirePrivacyRecovery(ctx))
			var reporterClass, targetClass, validation string
			require.NoError(t, pool.QueryRow(ctx, `SELECT reporter_provenance,validated_target_provenance,validation_state FROM reports WHERE id=$1`, row.ID).Scan(&reporterClass, &targetClass, &validation))
			require.Equal(t, "VERIFIED_AT_SUBMISSION", reporterClass)
			require.Equal(t, "VERIFIED_AT_VALIDATION", targetClass)
			require.Equal(t, "VALID", validation)
			var events int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_intake_outbox`).Scan(&events))
			require.Equal(t, 2, events, "backfill must not add activation events")
			var streams int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_subjects WHERE source_epoch IS NOT NULL OR through_sequence<>0 OR post_mute_count<>0`).Scan(&streams))
			require.Zero(t, streams, "privacy repair cannot invent Messaging epochs, cursors or strikes")
			batch, err = reports.BackfillVerifiedReportJournal(ctx, resolver, reportOpenPrivacy, 10)
			require.NoError(t, err)
			require.Zero(t, batch.Journaled)
			require.Equal(t, 2, calls, "idempotent empty replay performs no extra RPC")
		})
	}
}

func TestReportVerifiedJournalBackfillNeverPromotesLegacy(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	row, err := reports.InsertReport(ctx, uuid.New(), "story", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE reports SET reporter_account_id=$2,reporter_subject_id=$3,reporter_subject_revision=1,reporter_provenance='OWNER_RECOVERED_LEGACY' WHERE id=$1`, row.ID, uuid.New(), uuid.New())
	require.NoError(t, err)
	_, err = reports.InsertReport(ctx, uuid.New(), "user", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	resolver := reportSubjectResolverFunc(func(context.Context, uuid.UUID, uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error) {
		t.Fatal("legacy owner fact cannot be fabricated by Auth lookup")
		return nil, nil
	})
	batch, err := reports.BackfillVerifiedReportJournal(ctx, resolver, reportOpenPrivacy, 10)
	require.NoError(t, err)
	require.Zero(t, batch.Journaled)
	require.Zero(t, batch.Purged)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy)
	_, err = reports.BackfillVerifiedReportJournal(ctx, nil, reportOpenPrivacy, 10)
	require.ErrorIs(t, err, store.ErrReportPrivacy)
}

func TestReportVerifiedJournalBackfillPositiveTerminalOnly(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	check, _ := reportOpenPrivacy(ctx)
	reporter := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	row, err := reports.InsertSubjectReport(ctx, uuid.New(), reporter, check, "story", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM report_privacy_journal WHERE report_id=$1`, row.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM automod_subjects WHERE subject_id=$1`, reporter.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO automod_terminal_subjects(subject_id) VALUES($1)`, reporter.ID)
	require.NoError(t, err)
	resolver := reportSubjectResolverFunc(func(context.Context, uuid.UUID, uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error) {
		t.Fatal("positive permanent terminal needs no guessed unavailable lookup")
		return nil, nil
	})
	batch, err := reports.BackfillVerifiedReportJournal(ctx, resolver, reportOpenPrivacy, 10)
	require.NoError(t, err)
	require.Equal(t, 1, batch.Purged)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx))
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports`).Scan(&count))
	require.Zero(t, count)
}
