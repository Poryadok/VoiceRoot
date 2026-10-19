package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/store"
)

func retentionEvents() []store.IntakeEvent {
	return []store.IntakeEvent{{ID: uuid.New(), Kind: "domain.report_created", Subject: "domain", Payload: []byte("stable")}, {ID: uuid.New(), Kind: "analytics.report_created", Subject: "analytics", Payload: []byte("stable")}}
}

func TestReportValidationRetainedDeadlineBeforeCommit(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	check, _ := reportOpenPrivacy(ctx)
	reporter := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	row, err := reports.InsertSubjectReport(ctx, uuid.New(), reporter, check, "story", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	claim, err := reports.ClaimValidation(ctx)
	require.NoError(t, err)
	var deadline time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT clock_timestamp()+interval '5 seconds'`).Scan(&deadline))
	reporter.Revision = 2
	reporter.State = "DELETED"
	reporter.CycleID = uuid.New()
	reporter.PurgeAt = &deadline
	claim.ReporterSubject = &reporter
	claim.PrivacyCheck = check
	hooked := false
	applied, err := reports.CompleteValidation(ctx, claim, "LIVE", uuid.Nil, retentionEvents(), validationHookFunc(func(ctx context.Context, tx pgx.Tx, _ store.ValidationFact) error {
		hooked = true
		// Wait for the authoritative DB clock to cross the retained deadline,
		// rather than assuming a sleep duration or a 50ms scheduling guarantee.
		_, err := tx.Exec(ctx, `SELECT pg_sleep(GREATEST(0,EXTRACT(EPOCH FROM ($1::timestamptz-clock_timestamp())))+0.02)`, deadline)
		return err
	}))
	require.True(t, hooked)
	require.ErrorIs(t, err, store.ErrReportPrivacy, "retained identity must not commit after authoritative deadline")
	require.False(t, applied)
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT validation_state FROM reports WHERE id=$1`, row.ID).Scan(&state))
	require.Equal(t, "PENDING", state)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_intake_outbox`).Scan(&count))
	require.Zero(t, count)
}

func TestReportValidationTerminalPreflightBeforeAnyBind(t *testing.T) {
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
	claim.ReporterSubject = &reporter
	claim.TargetSubject = &target
	claim.PrivacyCheck = check
	// Replay has imported a terminal fence while the old pending row still
	// awaits its participant. The validator must not add any other raw binding.
	_, err = pool.Exec(ctx, `INSERT INTO automod_terminal_subjects(subject_id) VALUES($1)`, reporter.ID)
	require.NoError(t, err)
	applied, err := reports.CompleteValidation(ctx, claim, "LIVE", target.AccountID, retentionEvents(), nil)
	require.NoError(t, err)
	require.False(t, applied)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE id=$1`, row.ID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_subjects WHERE subject_id=$1`, target.ID).Scan(&count))
	require.Zero(t, count, "terminal plan cannot commit an unrelated raw target binding")
}

func TestReportValidationRestoredRevisionDominatesExpiredCapture(t *testing.T) {
	for _, freshLive := range []bool{false, true} {
		t.Run(map[bool]string{false: "stale_deleted", true: "fresh_live"}[freshLive], func(t *testing.T) {
			pool := reportIntakePool(t)
			ctx := context.Background()
			reports := newReportFixtureStore(pool)
			check, _ := reportOpenPrivacy(ctx)
			reporter := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
			row, err := reports.InsertSubjectReport(ctx, uuid.New(), reporter, check, "story", uuid.New(), "spam", nil, `{}`)
			require.NoError(t, err)
			claim, err := reports.ClaimValidation(ctx)
			require.NoError(t, err)
			var expired time.Time
			require.NoError(t, pool.QueryRow(ctx, `SELECT clock_timestamp()-interval '1 second'`).Scan(&expired))
			_, err = pool.Exec(ctx, `UPDATE automod_subjects SET lifecycle_revision=$2,lifecycle_state=$3,purge_at=$4 WHERE subject_id=$1`, reporter.ID, map[bool]int{false: 3, true: 2}[freshLive], map[bool]string{false: "LIVE", true: "DELETED"}[freshLive], map[bool]*time.Time{false: nil, true: &expired}[freshLive])
			require.NoError(t, err)
			reporter.Revision = 2
			reporter.State = "DELETED"
			reporter.CycleID = uuid.New()
			reporter.PurgeAt = &expired
			if freshLive {
				reporter.Revision, reporter.State, reporter.PurgeAt = 3, "LIVE", nil
				reporter.CycleID = uuid.Nil
			}
			claim.ReporterSubject, claim.PrivacyCheck = &reporter, check
			applied, err := reports.CompleteValidation(ctx, claim, "LIVE", uuid.Nil, retentionEvents(), nil)
			if freshLive {
				require.NoError(t, err)
				require.True(t, applied, "fresh newer LIVE can replace an older cached retained deadline")
			} else {
				require.ErrorIs(t, err, store.ErrReportPrivacy)
				require.False(t, applied)
			}
			var count int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE id=$1`, row.ID).Scan(&count))
			require.Equal(t, 1, count, "expired snapshot must not manufacture erasure")
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_terminal_subjects WHERE subject_id=$1`, reporter.ID).Scan(&count))
			require.Zero(t, count)
		})
	}
}

func TestReportValidationRejectsUnclaimedTargetBinding(t *testing.T) {
	for _, state := range []string{"LIVE", "MISSING"} {
		t.Run(state, func(t *testing.T) {
			pool := reportIntakePool(t)
			ctx := context.Background()
			reports := newReportFixtureStore(pool)
			check, _ := reportOpenPrivacy(ctx)
			reporter := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
			target := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
			row, err := reports.InsertSubjectReport(ctx, uuid.New(), reporter, check, "story", uuid.New(), "spam", nil, `{}`)
			require.NoError(t, err)
			claim, err := reports.ClaimValidation(ctx)
			require.NoError(t, err)
			claim.ReporterSubject, claim.TargetSubject, claim.PrivacyCheck = &reporter, &target, check
			applied, err := reports.CompleteValidation(ctx, claim, state, uuid.Nil, retentionEvents(), nil)
			require.ErrorIs(t, err, store.ErrReportPrivacy)
			require.False(t, applied)
			var current string
			require.NoError(t, pool.QueryRow(ctx, `SELECT validation_state FROM reports WHERE id=$1`, row.ID).Scan(&current))
			require.Equal(t, "PENDING", current)
			var count int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_subjects WHERE subject_id=$1`, target.ID).Scan(&count))
			require.Zero(t, count, "unclaimed target account must not be bound without its account lock")
		})
	}
}
