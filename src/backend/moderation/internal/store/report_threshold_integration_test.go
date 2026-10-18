package store_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/store"
	"voice/backend/pkg/integrationtest"
)

func thresholdPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "moderationdb", "")
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
	files, err := filepath.Glob(filepath.Join(root, "src/backend/migrations/moderation_db/*.up.sql"))
	require.NoError(t, err)
	for _, file := range files {
		sql, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sql))
		require.NoError(t, err, file)
	}
	return pool
}
func thresholdStore(pool *pgxpool.Pool) *store.ReportThresholdStore {
	authority := automod.NewStore(pool)
	return &store.ReportThresholdStore{Pool: pool, Sanctions: &store.SanctionStore{Pool: pool}, BindLiveSubjectTx: func(ctx context.Context, tx pgx.Tx, account, subject uuid.UUID, revision uint64) error {
		return authority.BindLiveSubjectTx(ctx, tx, account, subject, int64(revision))
	}}
}
func insertVote(t *testing.T, pool *pgxpool.Pool, profile, account, reporter uuid.UUID, age time.Duration, validation, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	var reporterAccount any = reporter
	if reporter == uuid.Nil {
		reporterAccount = nil
	}
	_, err := pool.Exec(context.Background(), `INSERT INTO reports(id,reporter_profile_id,reporter_account_id,target_type,target_id,category,status,
validation_state,validated_target_account_id,created_at) VALUES($1,$2,$3,'user',$4,'spam',$5,$6,$7,clock_timestamp()-interval '1 second'*$8)`, id, uuid.New(), reporterAccount, profile, status, validation, account, age.Seconds())
	require.NoError(t, err)
	return id
}
func queueVote(t *testing.T, s *store.ReportThresholdStore, id uuid.UUID) {
	t.Helper()
	tx, err := s.Pool.Begin(context.Background())
	require.NoError(t, err)
	require.NoError(t, s.EnqueueTx(context.Background(), tx, id))
	require.NoError(t, tx.Commit(context.Background()))
}
func observation(claim *store.ReportThresholdClaim, subject uuid.UUID, audience int64) store.ReportThresholdObservation {
	now := time.Now().UTC()
	return store.ReportThresholdObservation{EvaluationID: claim.ClaimID, ProfileID: claim.ProfileID, AccountID: claim.AccountID, SubjectID: subject, SnapshotID: uuid.New(), ProfileRevision: 1, LifecycleRevision: 1, Audience: audience, Metric: store.ReportAudienceMetric, StartedAt: now, CompletedAt: now, ExpiresAt: now.Add(30 * time.Second)}
}
func testThresholdEvents(c store.ReportThresholdCase) ([]store.ReportThresholdEvent, error) {
	return []store.ReportThresholdEvent{{ID: uuid.New(), Kind: "sanction_applied", Subject: "moderation.sanction_applied", Payload: []byte(c.SanctionID.String())}}, nil
}

func TestReportThresholdDistinctAccountsAndAtomicRollback(t *testing.T) {
	pool := thresholdPool(t)
	s := thresholdStore(pool)
	ctx := context.Background()
	profile, account, subject := uuid.New(), uuid.New(), uuid.New()
	for i := 0; i < 9; i++ {
		insertVote(t, pool, profile, account, uuid.New(), time.Minute, "VALID", "pending")
	}
	duplicate := uuid.New()
	for i := 0; i < 20; i++ {
		insertVote(t, pool, profile, account, duplicate, time.Minute, "VALID", "pending")
	}
	insertVote(t, pool, profile, account, uuid.Nil, time.Minute, "VALID", "pending")
	insertVote(t, pool, profile, account, account, time.Minute, "VALID", "pending")
	insertVote(t, pool, profile, account, uuid.New(), time.Minute, "PENDING", "pending")
	insertVote(t, pool, profile, account, uuid.New(), time.Minute, "VALID", "resolved")
	insertVote(t, pool, uuid.New(), account, uuid.New(), time.Minute, "VALID", "pending")
	insertVote(t, pool, profile, account, uuid.New(), 25*time.Hour, "VALID", "pending")
	trigger := insertVote(t, pool, profile, account, duplicate, time.Minute, "VALID", "pending")
	queueVote(t, s, trigger)
	claim, err := s.Claim(ctx)
	require.NoError(t, err)
	obs := observation(claim, subject, 1000)
	require.Error(t, s.Complete(ctx, claim, obs, func(store.ReportThresholdCase) ([]store.ReportThresholdEvent, error) {
		return nil, errors.New("serializer failed")
	}))
	for _, table := range []string{"report_threshold_cases", "sanctions", "auto_mod_log", "report_threshold_outbox", "automod_subjects"} {
		var count int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count))
		require.Zero(t, count, table)
	}
	require.NoError(t, s.Complete(ctx, claim, obs, testThresholdEvents))
	var reporters, threshold int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT reporter_accounts,threshold_accounts FROM report_threshold_cases`).Scan(&reporters, &threshold))
	require.Equal(t, int64(10), reporters)
	require.Equal(t, int64(10), threshold)
	var sanctions, logs, outbox int
	require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM sanctions),(SELECT count(*) FROM auto_mod_log),(SELECT count(*) FROM report_threshold_outbox)`).Scan(&sanctions, &logs, &outbox))
	require.Equal(t, 1, sanctions)
	require.Equal(t, 1, logs)
	require.Equal(t, 1, outbox)
	require.NoError(t, s.Complete(ctx, claim, obs, testThresholdEvents), "uncertain commit retry must not duplicate effects")
	changed := obs
	changed.Audience++
	require.ErrorContains(t, s.Complete(ctx, claim, changed, testThresholdEvents), "authority conflict")
	var profileRevision int64
	var evaluation uuid.UUID
	var started, completed, expires time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT profile_revision,evaluation_id,observed_at,observation_completed_at,context_expires_at FROM report_threshold_cases`).Scan(&profileRevision, &evaluation, &started, &completed, &expires))
	require.Equal(t, int64(obs.ProfileRevision), profileRevision)
	require.Equal(t, obs.EvaluationID, evaluation)
	require.WithinDuration(t, obs.StartedAt, started, time.Microsecond)
	require.WithinDuration(t, obs.CompletedAt, completed, time.Microsecond)
	require.WithinDuration(t, obs.ExpiresAt, expires, time.Microsecond)
}

func TestReportThresholdConcurrentProfilesCreateOneAccountCase(t *testing.T) {
	pool := thresholdPool(t)
	s := thresholdStore(pool)
	ctx := context.Background()
	account, subject := uuid.New(), uuid.New()
	for i := 0; i < 2; i++ {
		profile := uuid.New()
		for j := 0; j < 10; j++ {
			vote := insertVote(t, pool, profile, account, uuid.New(), time.Minute, "VALID", "pending")
			queueVote(t, s, vote)
		}
	}
	claims := make([]*store.ReportThresholdClaim, 20)
	for i := range claims {
		var err error
		claims[i], err = s.Claim(ctx)
		require.NoError(t, err)
	}
	results := make(chan error, len(claims))
	var wg sync.WaitGroup
	for _, claim := range claims {
		wg.Go(func() { results <- s.Complete(ctx, claim, observation(claim, subject, 1000), testThresholdEvents) })
	}
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	var cases, sanctions, outbox int
	require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM report_threshold_cases),(SELECT count(*) FROM sanctions),(SELECT count(*) FROM report_threshold_outbox)`).Scan(&cases, &sanctions, &outbox))
	require.Equal(t, 1, cases)
	require.Equal(t, 1, sanctions)
	require.Equal(t, 1, outbox)
}

func TestReportThresholdRejectsStaleAuthorityAndPreservesRollbackEvidence(t *testing.T) {
	pool := thresholdPool(t)
	s := thresholdStore(pool)
	ctx := context.Background()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	down, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../migrations/moderation_db/000010_report_threshold.down.sql"))
	require.NoError(t, err)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, string(down))
	require.NoError(t, err, "empty unused schema can roll back")
	require.NoError(t, tx.Rollback(ctx))
	profile, account, subject := uuid.New(), uuid.New(), uuid.New()
	var trigger uuid.UUID
	for i := 0; i < 10; i++ {
		trigger = insertVote(t, pool, profile, account, uuid.New(), time.Minute, "VALID", "pending")
	}
	queueVote(t, s, trigger)
	claim, err := s.Claim(ctx)
	require.NoError(t, err)
	for _, mutate := range []func(*store.ReportThresholdObservation){
		func(o *store.ReportThresholdObservation) {
			o.StartedAt = time.Now().Add(-31 * time.Second)
			o.CompletedAt = o.StartedAt
			o.ExpiresAt = time.Now().Add(-time.Second)
		},
		func(o *store.ReportThresholdObservation) { o.AccountID = uuid.New() },
		func(o *store.ReportThresholdObservation) { o.SubjectID = uuid.Nil },
		func(o *store.ReportThresholdObservation) { o.ProfileRevision = 0 },
		func(o *store.ReportThresholdObservation) { o.LifecycleRevision = 0 },
		func(o *store.ReportThresholdObservation) { o.Metric = "UNKNOWN" },
		func(o *store.ReportThresholdObservation) { o.Audience = -1 },
	} {
		obs := observation(claim, subject, 1000)
		mutate(&obs)
		require.Error(t, s.Complete(ctx, claim, obs, testThresholdEvents))
	}
	var effects int
	require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM sanctions)+(SELECT count(*) FROM report_threshold_cases)+(SELECT count(*) FROM report_threshold_outbox)+(SELECT count(*) FROM automod_subjects)`).Scan(&effects))
	require.Zero(t, effects)
	_, err = pool.Exec(ctx, string(down))
	require.ErrorContains(t, err, "report threshold evidence prevents rollback")
	var retained int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_threshold_evaluations`).Scan(&retained))
	require.Equal(t, 1, retained)
	require.NoError(t, s.Complete(ctx, claim, observation(claim, subject, 0), testThresholdEvents))
	var outcome string
	require.NoError(t, pool.QueryRow(ctx, `SELECT outcome FROM report_threshold_evaluations WHERE report_id=$1`, trigger).Scan(&outcome))
	require.Equal(t, "EMPTY_AUDIENCE", outcome)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM sanctions`).Scan(&effects))
	require.Zero(t, effects, "authoritative zero is manual, never a default threshold")
}

func TestReportThresholdCutoffOutboxRetryAndTerminalPurge(t *testing.T) {
	pool := thresholdPool(t)
	s := thresholdStore(pool)
	ctx := context.Background()
	profile, account, subject := uuid.New(), uuid.New(), uuid.New()
	var trigger uuid.UUID
	for i := 0; i < 10; i++ {
		trigger = insertVote(t, pool, profile, account, uuid.New(), time.Minute, "VALID", "pending")
	}
	queueVote(t, s, trigger)
	claim, err := s.Claim(ctx)
	require.NoError(t, err)
	require.NoError(t, s.Complete(ctx, claim, observation(claim, subject, 1000), testThresholdEvents))
	var first store.ReportThresholdEvent
	worked, err := s.DeliverEvent(ctx, func(_ context.Context, event store.ReportThresholdEvent) error {
		first = event
		return errors.New("uncertain ACK")
	})
	require.True(t, worked)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `UPDATE report_threshold_outbox SET available_at=clock_timestamp()-interval '1 second'`)
	require.NoError(t, err)
	worked, err = s.DeliverEvent(ctx, func(_ context.Context, event store.ReportThresholdEvent) error {
		require.Equal(t, first, event, "restart/retry retains event identity and bytes")
		return nil
	})
	require.True(t, worked)
	require.NoError(t, err)
	var sanctionID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM sanctions`).Scan(&sanctionID))
	require.NoError(t, s.Sanctions.WithAccountLock(ctx, account, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE sanctions SET revoked_at=clock_timestamp() WHERE id=$1`, sanctionID)
		if err != nil {
			return err
		}
		return s.AfterSanctionClosedTx(ctx, tx, &store.SanctionRow{ID: sanctionID, TargetAccountID: account, Type: "shadow_ban"})
	}))
	// A new report after closure must not borrow any old account evidence.
	newVote := insertVote(t, pool, profile, account, uuid.New(), 0, "VALID", "pending")
	queueVote(t, s, newVote)
	claim, err = s.Claim(ctx)
	require.NoError(t, err)
	require.NoError(t, s.Complete(ctx, claim, observation(claim, subject, 1000), testThresholdEvents))
	var active int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_threshold_cases WHERE closed_at IS NULL`).Scan(&active))
	require.Zero(t, active)
	for i := 0; i < 9; i++ {
		newVote = insertVote(t, pool, profile, account, uuid.New(), 0, "VALID", "pending")
	}
	queueVote(t, s, newVote)
	claim, err = s.Claim(ctx)
	require.NoError(t, err)
	require.NoError(t, s.Complete(ctx, claim, observation(claim, subject, 1000), testThresholdEvents))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_threshold_cases WHERE closed_at IS NULL`).Scan(&active))
	require.Equal(t, 1, active)
	authority := automod.NewStore(pool)
	authority.BeforeTerminalPurge = func(ctx context.Context, tx pgx.Tx, account uuid.UUID) error {
		if err := s.PurgeAccountTx(ctx, tx, account); err != nil {
			return err
		}
		return (&store.ReportStore{Pool: pool}).PurgeReportAccountLinksTx(ctx, tx, account)
	}
	require.NoError(t, authority.Purge(ctx, subject))
	for _, table := range []string{"report_threshold_evaluations", "report_threshold_cases", "report_threshold_outbox", "report_threshold_case_reports", "report_threshold_account_cutoffs", "sanctions", "auto_mod_log", "reports", "automod_subjects"} {
		var count int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count))
		require.Zero(t, count, table)
	}
	var terminal bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, subject).Scan(&terminal))
	require.True(t, terminal)
	worked, err = s.DeliverEvent(ctx, func(context.Context, store.ReportThresholdEvent) error {
		t.Fatal("erased pending effect must not publish")
		return nil
	})
	require.NoError(t, err)
	require.False(t, worked)
	// Delayed/backup-era work cannot recreate a permanently erased subject.
	for i := 0; i < 10; i++ {
		newVote = insertVote(t, pool, profile, account, uuid.New(), 0, "VALID", "pending")
	}
	queueVote(t, s, newVote)
	claim, err = s.Claim(ctx)
	require.NoError(t, err)
	require.ErrorIs(t, s.Complete(ctx, claim, observation(claim, subject, 1000), testThresholdEvents), automod.ErrPurged)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM sanctions`).Scan(&active))
	require.Zero(t, active)
}

func TestReportThresholdPreservesHumanSanctionAndRechecksCommitFreshness(t *testing.T) {
	pool := thresholdPool(t)
	s := thresholdStore(pool)
	ctx := context.Background()
	for _, human := range []bool{true, false} {
		profile, account, subject := uuid.New(), uuid.New(), uuid.New()
		var trigger uuid.UUID
		for i := 0; i < 10; i++ {
			trigger = insertVote(t, pool, profile, account, uuid.New(), time.Minute, "VALID", "pending")
		}
		queueVote(t, s, trigger)
		claim, err := s.Claim(ctx)
		require.NoError(t, err)
		obs := observation(claim, subject, 1000)
		if human {
			issuer, sanction := uuid.New(), uuid.New()
			_, err := pool.Exec(ctx, `INSERT INTO sanctions(id,target_account_id,type,reason,issued_by) VALUES($1,$2,'shadow_ban','Human review decision',$3)`, sanction, account, issuer)
			require.NoError(t, err)
			require.NoError(t, s.Complete(ctx, claim, obs, func(store.ReportThresholdCase) ([]store.ReportThresholdEvent, error) {
				t.Fatal("existing shadow ban cannot issue another event")
				return nil, nil
			}))
			var actualIssuer uuid.UUID
			var actualReason, outcome string
			require.NoError(t, pool.QueryRow(ctx, `SELECT issued_by,reason FROM sanctions WHERE id=$1`, sanction).Scan(&actualIssuer, &actualReason))
			require.Equal(t, issuer, actualIssuer)
			require.Equal(t, "Human review decision", actualReason)
			require.NoError(t, pool.QueryRow(ctx, `SELECT outcome FROM report_threshold_evaluations WHERE report_id=$1`, trigger).Scan(&outcome))
			require.Equal(t, "EXISTING_SHADOW_BAN", outcome)
		} else {
			obs.ExpiresAt = time.Now().Add(150 * time.Millisecond)
			require.Error(t, s.Complete(ctx, claim, obs, func(c store.ReportThresholdCase) ([]store.ReportThresholdEvent, error) {
				time.Sleep(200 * time.Millisecond)
				return testThresholdEvents(c)
			}))
			var effects int
			require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM sanctions WHERE target_account_id=$1)+(SELECT count(*) FROM report_threshold_cases WHERE account_id=$1)+(SELECT count(*) FROM automod_subjects WHERE account_id=$1)`, account).Scan(&effects))
			require.Zero(t, effects, "authority expiring during action rolls back every effect")
			require.NoError(t, s.FinishManual(ctx, claim, "INELIGIBLE_EVIDENCE"))
		}
	}
	var cases, events int
	require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM report_threshold_cases),(SELECT count(*) FROM report_threshold_outbox)`).Scan(&cases, &events))
	require.Zero(t, cases)
	require.Zero(t, events)
}
