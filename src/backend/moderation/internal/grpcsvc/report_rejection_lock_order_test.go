package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/moderation/internal/store"
)

func TestReportUserRejectionLockFollowsSubjectJournal(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	row, err := insertReportFixture(t, reports, ctx, uuid.New(), "user", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	claim, err := reports.ClaimValidation(ctx)
	require.NoError(t, err)
	reporter, _, err := store.ResolveReportSubject(ctx, reportFixtureResolver(), *claim.ReporterAccountID, *claim.ReporterSubjectID)
	require.NoError(t, err)
	target, _, err := store.ResolveReportSubject(ctx, reportFixtureResolver(), uuid.New(), uuid.Nil)
	require.NoError(t, err)
	claim.ReporterSubject, claim.TargetSubject = &reporter, &target
	claim.PrivacyCheck, err = reportOpenPrivacy(ctx)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE FUNCTION hold_target_journal() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN IF NEW.slot='USER_TARGET' THEN PERFORM pg_advisory_xact_lock(14413752::bigint); END IF; RETURN NEW; END $$;
 CREATE TRIGGER hold_target_journal BEFORE INSERT ON report_privacy_journal FOR EACH ROW EXECUTE FUNCTION hold_target_journal()`)
	require.NoError(t, err)
	holder, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = holder.Exec(ctx, `SELECT pg_advisory_xact_lock(14413752::bigint)`)
	require.NoError(t, err)
	workCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	type result struct {
		applied bool
		err     error
	}
	done := make(chan result, 1)
	received := false
	t.Cleanup(func() {
		_ = holder.Rollback(ctx)
		cancel()
		if !received {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("completion did not stop")
			}
		}
	})
	go func() {
		applied, e := reports.CompleteValidation(workCtx, claim, "LIVE", target.AccountID, retentionEvents(), nil)
		done <- result{applied, e}
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		e := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock'
 AND wait_event='advisory' AND query LIKE '%INSERT INTO report_privacy_journal%')`).Scan(&waiting)
		return e == nil && waiting
	}, 10*time.Second, 25*time.Millisecond)
	probe, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = probe.Rollback(ctx) }()
	var unlocked bool
	require.NoError(t, probe.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended('moderation-report-rejection:' || $1::text,0))`, row.ID.String()).Scan(&unlocked))
	require.NoError(t, probe.Rollback(ctx))
	require.True(t, unlocked, "subject-journal write must precede every rejection lock")
	require.NoError(t, holder.Commit(ctx))
	select {
	case completed := <-done:
		received = true
		require.NoError(t, completed.err)
		require.True(t, completed.applied)
	case <-workCtx.Done():
		t.Fatal("completion exceeded bounded fixture")
	}
}
