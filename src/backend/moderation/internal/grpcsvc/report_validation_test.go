package grpcsvc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/moderation/internal/reportintake"
	"voice/backend/moderation/internal/store"
)

type validationHookFunc func(context.Context, pgx.Tx, store.ValidationFact) error

func (f validationHookFunc) ReportValidated(ctx context.Context, tx pgx.Tx, fact store.ValidationFact) error {
	return f(ctx, tx, fact)
}

func TestReportValidationDurability(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	insert := func() *store.ReportRow {
		description := "private evidence"
		row, err := insertReportFixture(t, reports, ctx, uuid.New(), "user", uuid.New(), "spam", &description, `{"private":true}`)
		require.NoError(t, err)
		return row
	}
	event := func() []store.IntakeEvent {
		return []store.IntakeEvent{{ID: uuid.New(), Kind: "domain.report_created", Subject: "moderation.report_created", Payload: []byte("stable-domain")}, {ID: uuid.New(), Kind: "analytics.report_created", Subject: "analytics.moderation.report_created", Payload: []byte("stable-analytics")}}
	}
	t.Run("retry_survives_worker_restart", func(t *testing.T) {
		row := insert()
		claim, err := reports.ClaimValidation(ctx)
		require.NoError(t, err)
		require.Equal(t, row.ID, claim.ReportID)
		_, err = reports.ClaimValidation(ctx)
		require.ErrorIs(t, err, pgx.ErrNoRows)
		require.NoError(t, reports.RetryValidation(ctx, claim))
		_, err = reports.ClaimValidation(ctx)
		require.ErrorIs(t, err, pgx.ErrNoRows)
		_, err = pool.Exec(ctx, `UPDATE reports SET validation_next_at=now()-interval '1 second' WHERE id=$1`, row.ID)
		require.NoError(t, err)
		restarted := newReportFixtureStore(pool)
		fresh, err := restarted.ClaimValidation(ctx)
		require.NoError(t, err)
		require.NotEqual(t, claim.ClaimID, fresh.ClaimID)
		require.Equal(t, 2, fresh.Attempts)
		applied, err := completeReportFixture(t, reports, ctx, claim, "LIVE", uuid.New(), event(), nil)
		require.NoError(t, err)
		require.False(t, applied)
		applied, err = completeReportFixture(t, restarted, ctx, fresh, "MISSING", uuid.Nil, nil, nil)
		require.NoError(t, err)
		require.True(t, applied)
		var state, status, evidence string
		var description *string
		require.NoError(t, pool.QueryRow(ctx, `SELECT validation_state,status,description,evidence::text FROM reports WHERE id=$1`, row.ID).Scan(&state, &status, &description, &evidence))
		require.Equal(t, "MISSING", state)
		require.Equal(t, "dismissed", status)
		require.Nil(t, description)
		require.JSONEq(t, `{}`, evidence)
	})
	t.Run("activation_hook_and_events_are_atomic", func(t *testing.T) {
		row := insert()
		claim, err := reports.ClaimValidation(ctx)
		require.NoError(t, err)
		account := uuid.New()
		events := event()
		hook := validationHookFunc(func(ctx context.Context, tx pgx.Tx, fact store.ValidationFact) error {
			require.Equal(t, row.ID, fact.ReportID)
			require.Equal(t, &account, fact.TargetAccountID)
			require.NotNil(t, fact.ReporterAccountID)
			require.NotNil(t, fact.ReporterSubjectID)
			_, err := tx.Exec(ctx, `INSERT INTO auto_mod_log(target_profile_id,trigger,action,details) VALUES($1,'hook','test','{}')`, fact.TargetID)
			require.NoError(t, err)
			return errors.New("injected threshold enqueue failure")
		})
		applied, err := completeReportFixture(t, reports, ctx, claim, "LIVE", account, events, hook)
		require.Error(t, err)
		require.False(t, applied)
		var state string
		var count int
		require.NoError(t, pool.QueryRow(ctx, `SELECT validation_state FROM reports WHERE id=$1`, row.ID).Scan(&state))
		require.Equal(t, "PENDING", state)
		require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM auto_mod_log`).Scan(&count))
		require.Zero(t, count)
		require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM report_intake_outbox`).Scan(&count))
		require.Zero(t, count)
		applied, err = completeReportFixture(t, reports, ctx, claim, "LIVE", account, events, nil)
		require.NoError(t, err)
		require.True(t, applied)
		applied, err = completeReportFixture(t, reports, ctx, claim, "LIVE", account, event(), nil)
		require.NoError(t, err)
		require.False(t, applied)
		require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM report_intake_outbox WHERE report_id=$1`, row.ID).Scan(&count))
		require.Equal(t, 2, count)
		queue, err := reports.ListReports(ctx, "", 50)
		require.NoError(t, err)
		require.Len(t, queue, 1)
	})
	t.Run("purge_wins_inflight_validation", func(t *testing.T) {
		row := insert()
		claim, err := reports.ClaimValidation(ctx)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `DELETE FROM reports WHERE id=$1`, row.ID)
		require.NoError(t, err)
		applied, err := completeReportFixture(t, reports, ctx, claim, "LIVE", uuid.New(), event(), nil)
		require.NoError(t, err)
		require.False(t, applied)
		var count int
		require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM report_intake_outbox WHERE report_id=$1`, row.ID).Scan(&count))
		require.Zero(t, count)
	})
	t.Run("unknown_authority_cannot_activate", func(t *testing.T) {
		row := insert()
		claim, err := reports.ClaimValidation(ctx)
		require.NoError(t, err)
		applied, err := completeReportFixture(t, reports, ctx, claim, "UNSPECIFIED", uuid.Nil, event(), nil)
		require.Error(t, err)
		require.False(t, applied)
		applied, err = completeReportFixture(t, reports, ctx, claim, "LIVE", uuid.Nil, event(), nil)
		require.Error(t, err)
		require.False(t, applied)
		applied, err = completeReportFixture(t, reports, ctx, claim, "DELETED", uuid.Nil, nil, nil)
		require.NoError(t, err)
		require.True(t, applied)
		var state string
		require.NoError(t, pool.QueryRow(ctx, `SELECT validation_state FROM reports WHERE id=$1`, row.ID).Scan(&state))
		require.Equal(t, "DELETED", state)
	})
}

type reportTargetFunc func(context.Context, uuid.UUID, string, uuid.UUID) (string, uuid.UUID, error)

func TestReportAccountPurgeParticipant(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	account := uuid.New()
	targetAccount := uuid.New()
	row, err := insertVerifiedReportFixture(t, reports, ctx, uuid.New(), account, "user", uuid.New(), "spam", nil, `{"private":true}`)
	require.NoError(t, err)
	claim, err := reports.ClaimValidation(ctx)
	require.NoError(t, err)
	events := []store.IntakeEvent{{ID: uuid.New(), Kind: "domain.report_created", Subject: "moderation.report_created", Payload: []byte("private-domain")}, {ID: uuid.New(), Kind: "analytics.report_created", Subject: "analytics.moderation.report_created", Payload: []byte("private-analytics")}}
	applied, err := completeReportFixture(t, reports, ctx, claim, "LIVE", targetAccount, events, nil)
	require.NoError(t, err)
	require.True(t, applied)
	sanctions := &store.SanctionStore{Pool: pool}
	sanction, err := sanctions.InsertSanction(ctx, targetAccount, "shadow_ban", "private report reason", &row.ID, uuid.New(), nil)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO automod_terminal_subjects(subject_id) VALUES($1)`, *claim.ReporterSubjectID)
	require.NoError(t, err) // Explicit verified terminal-authority fixture.
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, reports.PurgeReportAccountLinksTx(ctx, tx, account))
	require.NoError(t, tx.Rollback(ctx))
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM reports WHERE id=$1`, row.ID).Scan(&count))
	require.Equal(t, 1, count, "owner transaction rollback preserves all records")
	tx, err = pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, reports.PurgeReportAccountLinksTx(ctx, tx, account))
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM reports WHERE id=$1`, row.ID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM report_intake_outbox WHERE report_id=$1`, row.ID).Scan(&count))
	require.Zero(t, count)
	var reportID *uuid.UUID
	var reason string
	var revoked *time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT report_id,reason,revoked_at FROM sanctions WHERE id=$1`, sanction.ID).Scan(&reportID, &reason, &revoked))
	require.Nil(t, reportID)
	require.Equal(t, "TARGET_DELETED", reason)
	require.Nil(t, revoked)
	applied, err = completeReportFixture(t, reports, ctx, claim, "LIVE", targetAccount, events, nil)
	require.NoError(t, err)
	require.False(t, applied)
}

func TestReportValidationConcurrentWorkers(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	_, err := pool.Exec(ctx, `CREATE TABLE validation_hook_calls(report_id UUID PRIMARY KEY)`)
	require.NoError(t, err)
	for range 20 {
		_, err = insertReportFixture(t, reports, ctx, uuid.New(), "story", uuid.New(), "spam", nil, `{}`)
		require.NoError(t, err)
	}
	worker := &reportintake.Worker{Reports: reports, Subjects: reportFixtureResolver(), Privacy: reportOpenPrivacy, Targets: reportTargetFunc(func(context.Context, uuid.UUID, string, uuid.UUID) (string, uuid.UUID, error) {
		return "LIVE", uuid.Nil, nil
	}), Validated: validationHookFunc(func(ctx context.Context, tx pgx.Tx, fact store.ValidationFact) error {
		_, err := tx.Exec(ctx, `INSERT INTO validation_hook_calls(report_id) VALUES($1)`, fact.ReportID)
		return err
	})}
	var group sync.WaitGroup
	start := make(chan struct{})
	failures := make(chan error, 160)
	for range 20 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			for range 8 {
				_, err := worker.ValidateOne(ctx)
				if err != nil {
					failures <- err
				}
			}
		}()
	}
	close(start)
	group.Wait()
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}
	for query, want := range map[string]int{`SELECT COUNT(*) FROM reports WHERE validation_state='VALID'`: 20, `SELECT COUNT(*) FROM validation_hook_calls`: 20, `SELECT COUNT(*) FROM report_intake_outbox`: 40} {
		var count int
		require.NoError(t, pool.QueryRow(ctx, query).Scan(&count))
		require.Equal(t, want, count)
	}
	seen := sync.Map{}
	delivered := make(chan uuid.UUID, 40)
	failures = make(chan error, 160)
	publish := func(_ context.Context, event store.IntakeEvent) error {
		if _, duplicate := seen.LoadOrStore(event.ID, true); duplicate {
			return errors.New("parallel duplicate dispatch")
		}
		delivered <- event.ID
		return nil
	}
	for range 20 {
		group.Add(1)
		go func() {
			defer group.Done()
			for range 8 {
				_, err := reports.DeliverIntakeEvent(ctx, publish)
				if err != nil {
					failures <- err
				}
			}
		}()
	}
	group.Wait()
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}
	close(delivered)
	require.Len(t, delivered, 40)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM report_intake_outbox WHERE delivered_at IS NOT NULL`).Scan(&count))
	require.Equal(t, 40, count)
}

func TestReportIntakeMigrationLegacyProvenance(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	root := repoRoot(t)
	down, err := os.ReadFile(filepath.Join(root, "src/backend/migrations/moderation_db/000007_report_intake.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.NoError(t, err, "empty expand migration can roll back")
	pending, closed := uuid.New(), uuid.New()
	for id, state := range map[uuid.UUID]string{pending: "pending", closed: "resolved"} {
		_, err = pool.Exec(ctx, `INSERT INTO reports(id,reporter_profile_id,target_type,target_id,category,status) VALUES($1,$2,'story',$3,'spam',$4)`, id, uuid.New(), uuid.New(), state)
		require.NoError(t, err)
	}
	integrationtestApplySQL(t, ctx, pool, filepath.Join(root, "src/backend/migrations/moderation_db/000007_report_intake.up.sql"))
	var state string
	var legacy bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT validation_state,validation_publication_legacy FROM reports WHERE id=$1`, pending).Scan(&state, &legacy))
	require.Equal(t, "PENDING", state)
	require.True(t, legacy)
	require.NoError(t, pool.QueryRow(ctx, `SELECT validation_state FROM reports WHERE id=$1`, closed).Scan(&state))
	require.Equal(t, "VALID", state)
	_, err = pool.Exec(ctx, string(down))
	require.Error(t, err, "unverified historical work must survive rollback attempts")
	reports := newReportFixtureStore(pool)
	claim, err := reports.ClaimValidation(ctx)
	require.ErrorIs(t, err, pgx.ErrNoRows, "c18 privacy recovery does not promote old pending history")
	require.Nil(t, claim)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM report_intake_outbox`).Scan(&count))
	require.Zero(t, count, "revalidated history must not replay already published report-created events")
	_, err = insertReportFixture(t, reports, ctx, uuid.New(), "story", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.Error(t, err, "new admitted work must survive rollback attempts")
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM reports`).Scan(&count))
	require.Equal(t, 3, count)
}

func (f reportTargetFunc) Resolve(ctx context.Context, id uuid.UUID, kind string, target uuid.UUID) (string, uuid.UUID, error) {
	return f(ctx, id, kind, target)
}

func TestReportValidationWorkerOutbox(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	row, err := insertReportFixture(t, reports, ctx, uuid.New(), "story", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	worker := &reportintake.Worker{Reports: reports, Subjects: reportFixtureResolver(), Privacy: reportOpenPrivacy, Targets: reportTargetFunc(func(_ context.Context, id uuid.UUID, kind string, target uuid.UUID) (string, uuid.UUID, error) {
		require.Equal(t, row.ID, id)
		require.Equal(t, "story", kind)
		require.Equal(t, row.TargetID, target)
		return "UNSPECIFIED", uuid.Nil, nil
	})}
	worked, err := worker.ValidateOne(ctx)
	require.NoError(t, err)
	require.True(t, worked)
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT validation_state FROM reports WHERE id=$1`, row.ID).Scan(&state))
	require.Equal(t, "PENDING", state)
	_, err = pool.Exec(ctx, `UPDATE reports SET validation_next_at=now()-interval '1 second' WHERE id=$1`, row.ID)
	require.NoError(t, err)
	worker.Targets = reportTargetFunc(func(context.Context, uuid.UUID, string, uuid.UUID) (string, uuid.UUID, error) {
		return "LIVE", uuid.Nil, nil
	})
	worked, err = worker.ValidateOne(ctx)
	require.NoError(t, err)
	require.True(t, worked)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM report_intake_outbox WHERE report_id=$1`, row.ID).Scan(&count))
	require.Equal(t, 2, count)
	seen := map[uuid.UUID][]byte{}
	// Simulate a server ACK lost after delivery: retry must keep the exact ID/body.
	attempted, err := reports.DeliverIntakeEvent(ctx, func(_ context.Context, event store.IntakeEvent) error {
		seen[event.ID] = append([]byte(nil), event.Payload...)
		return errors.New("lost ACK")
	})
	require.True(t, attempted)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `UPDATE report_intake_outbox SET available_at=now()-interval '1 second'`)
	require.NoError(t, err)
	retried := false
	for i := 0; i < 2; i++ {
		attempted, err = reports.DeliverIntakeEvent(ctx, func(_ context.Context, event store.IntakeEvent) error {
			if prior, ok := seen[event.ID]; ok {
				require.Equal(t, prior, event.Payload)
				retried = true
			}
			return nil
		})
		require.NoError(t, err)
		require.True(t, attempted)
	}
	require.True(t, retried)
	attempted, err = reports.DeliverIntakeEvent(ctx, func(context.Context, store.IntakeEvent) error { t.Fatal("already delivered"); return nil })
	require.NoError(t, err)
	require.False(t, attempted)
	// A queued event cannot survive removal of its parent report.
	_, err = pool.Exec(ctx, `UPDATE report_intake_outbox SET delivered_at=NULL`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM reports WHERE id=$1`, row.ID)
	require.NoError(t, err)
	attempted, err = reports.DeliverIntakeEvent(ctx, func(context.Context, store.IntakeEvent) error { t.Fatal("purged event published"); return nil })
	require.NoError(t, err)
	require.False(t, attempted)
}
