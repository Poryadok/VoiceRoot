package grpcsvc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"voice/backend/moderation/internal/store"

	moderationv1 "voice.app/voice/moderation/v1"
)

type effectiveBanAuthCall struct {
	accountID uuid.UUID
	status    string
	reason    string
}

type effectiveBanAuthClient struct {
	mu            sync.Mutex
	calls         []effectiveBanAuthCall
	activeStarted chan struct{}
	releaseActive chan struct{}
	blockActive   bool
	releaseOnce   sync.Once
}

func (c *effectiveBanAuthClient) SetAccountStatus(_ context.Context, accountID uuid.UUID, status, reason string) error {
	if status == "active" && c.blockActive {
		close(c.activeStarted)
		<-c.releaseActive
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, effectiveBanAuthCall{accountID: accountID, status: status, reason: reason})
	return nil
}

func (c *effectiveBanAuthClient) release() { c.releaseOnce.Do(func() { close(c.releaseActive) }) }

func (c *effectiveBanAuthClient) snapshot() []effectiveBanAuthCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]effectiveBanAuthCall(nil), c.calls...)
}

func awaitAccountAdvisoryLockWait(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		err := pool.QueryRow(waitCtx, "SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND wait_event = 'advisory' AND query LIKE '%pg_advisory_xact_lock%')").Scan(&waiting)
		if err == nil && waiting {
			return
		}
		select {
		case <-waitCtx.Done():
			t.Fatal("PostgreSQL did not report the contender blocked on the account advisory lock")
		case <-ticker.C:
		}
	}
}

func TestApplyCommitFailureReconcilesAmbiguousAuthSuspension(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	auth := &effectiveBanAuthClient{}
	client, cleanup := startModerationGRPCTestServer(t, pool, func(svc *ModerationGRPC) { svc.Auth = auth })
	defer cleanup()
	accountID, moderator := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, "CREATE FUNCTION fail_sanction_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected sanction commit failure'; END; $$")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "CREATE CONSTRAINT TRIGGER fail_sanction_insert_commit AFTER INSERT ON sanctions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fail_sanction_insert()")
	require.NoError(t, err)
	_, err = client.ApplySanction(withInternalModCtx(ctx, moderator), &moderationv1.ApplySanctionRequest{
		TargetAccountId: accountID.String(), Type: "perm_ban", Reason: "commit failure",
	})
	require.Error(t, err)
	calls := auth.snapshot()
	require.Len(t, calls, 2)
	require.Equal(t, "suspended", calls[0].status)
	require.Equal(t, "active", calls[1].status, "recovery derives status from the sanction state PostgreSQL retained")
	rows, err := (&store.SanctionStore{Pool: pool}).ListByAccount(ctx, accountID)
	require.NoError(t, err)
	require.Empty(t, rows, "the failed transaction must not leave a sanction")
	pending, err := (&store.SanctionStore{Pool: pool}).ListPendingAccountStatusSync(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, pending, "successful compensation must clear its durable work")
	_, err = pool.Exec(ctx, "DROP TRIGGER fail_sanction_insert_commit ON sanctions")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "DROP FUNCTION fail_sanction_insert()")
	require.NoError(t, err)
}

func TestRevokeCommitFailureNeverCallsAuthActiveAndLeavesBanEffective(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	auth := &effectiveBanAuthClient{}
	client, cleanup := startModerationGRPCTestServer(t, pool, func(svc *ModerationGRPC) { svc.Auth = auth })
	defer cleanup()
	accountID, moderator, issuer := uuid.New(), uuid.New(), autoModIssuerProfileID()
	ban, err := (&store.SanctionStore{Pool: pool}).InsertSanction(ctx, accountID, "perm_ban", "commit failure", nil, issuer, nil)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "CREATE FUNCTION fail_account_status_sync_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected deferred commit failure'; END; $$")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "CREATE CONSTRAINT TRIGGER fail_account_status_sync_insert_commit AFTER INSERT ON moderation_account_status_sync DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fail_account_status_sync_insert()")
	require.NoError(t, err)
	_, err = client.RevokeSanction(withInternalModCtx(ctx, moderator), &moderationv1.RevokeSanctionRequest{SanctionId: ban.ID.String()})
	require.Error(t, err)
	require.Empty(t, auth.snapshot(), "Auth must not receive active before the sanction mutation commits")
	current, err := (&store.SanctionStore{Pool: pool}).GetByID(ctx, ban.ID)
	require.NoError(t, err)
	require.Nil(t, current.RevokedAt, "failed commit must leave the effective ban intact")
	_, err = pool.Exec(ctx, "DROP TRIGGER fail_account_status_sync_insert_commit ON moderation_account_status_sync")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "DROP FUNCTION fail_account_status_sync_insert()")
	require.NoError(t, err)
}

func TestApprovedAppealCommitFailureLeavesBanEffective(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	auth := &effectiveBanAuthClient{}
	client, cleanup := startModerationGRPCTestServer(t, pool, func(svc *ModerationGRPC) { svc.Auth = auth })
	defer cleanup()
	accountID, moderator, issuer := uuid.New(), uuid.New(), autoModIssuerProfileID()
	ban, err := (&store.SanctionStore{Pool: pool}).InsertSanction(ctx, accountID, "perm_ban", "appeal commit failure", nil, issuer, nil)
	require.NoError(t, err)
	appeal, err := (&store.AppealStore{Pool: pool}).InsertAppeal(ctx, ban.ID, accountID, "review")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "CREATE FUNCTION fail_appeal_status_sync_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected appeal commit failure'; END; $$")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "CREATE CONSTRAINT TRIGGER fail_appeal_status_sync_insert_commit AFTER INSERT ON moderation_account_status_sync DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fail_appeal_status_sync_insert()")
	require.NoError(t, err)
	_, err = client.ReviewAppeal(withInternalModCtx(ctx, moderator), &moderationv1.ReviewAppealRequest{AppealId: appeal.ID.String(), Status: "approved"})
	require.Error(t, err)
	require.Empty(t, auth.snapshot(), "Auth status must not change before the appeal and sanction transaction commits")
	currentBan, err := (&store.SanctionStore{Pool: pool}).GetByID(ctx, ban.ID)
	require.NoError(t, err)
	require.Nil(t, currentBan.RevokedAt, "failed approval commit must preserve the effective ban")
	currentAppeal, err := (&store.AppealStore{Pool: pool}).GetByID(ctx, appeal.ID)
	require.NoError(t, err)
	require.Equal(t, "pending", currentAppeal.Status, "failed approval commit must preserve the pending appeal")
	_, err = pool.Exec(ctx, "DROP TRIGGER fail_appeal_status_sync_insert_commit ON moderation_account_status_sync")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "DROP FUNCTION fail_appeal_status_sync_insert()")
	require.NoError(t, err)
}

func TestAuthSuccessThenReconciliationCommitFailureKeepsDurableTransitionAndRetry(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	auth := &effectiveBanAuthClient{}
	client, cleanup := startModerationGRPCTestServer(t, pool, func(svc *ModerationGRPC) { svc.Auth = auth })
	defer cleanup()
	accountID, moderator, issuer := uuid.New(), uuid.New(), autoModIssuerProfileID()
	ban, err := (&store.SanctionStore{Pool: pool}).InsertSanction(ctx, accountID, "perm_ban", "reconciliation commit failure", nil, issuer, nil)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "CREATE FUNCTION fail_account_status_sync_delete() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected deferred acknowledgement commit failure'; END; $$")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "CREATE CONSTRAINT TRIGGER fail_account_status_sync_delete_commit AFTER DELETE ON moderation_account_status_sync DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fail_account_status_sync_delete()")
	require.NoError(t, err)
	_, err = client.RevokeSanction(withInternalModCtx(ctx, moderator), &moderationv1.RevokeSanctionRequest{SanctionId: ban.ID.String()})
	require.Error(t, err)
	calls := auth.snapshot()
	require.Len(t, calls, 1)
	require.Equal(t, "active", calls[0].status)
	current, err := (&store.SanctionStore{Pool: pool}).GetByID(ctx, ban.ID)
	require.NoError(t, err)
	require.NotNil(t, current.RevokedAt, "Auth active is sent only after durable sanction removal")
	pending, err := (&store.SanctionStore{Pool: pool}).ListPendingAccountStatusSync(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{accountID}, pending, "failed acknowledgement must retain retry work")
	_, err = pool.Exec(ctx, "DROP TRIGGER fail_account_status_sync_delete_commit ON moderation_account_status_sync")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "DROP FUNCTION fail_account_status_sync_delete()")
	require.NoError(t, err)
	_, err = (&ModerationGRPC{Sanctions: &store.SanctionStore{Pool: pool}, Auth: auth}).ProcessExpiredTempBans(ctx, 10)
	require.NoError(t, err)
	require.Len(t, auth.snapshot(), 2)
	pending, err = (&store.SanctionStore{Pool: pool}).ListPendingAccountStatusSync(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, pending)
}

func TestProcessExpiredTempBansKeepsAccountSuspendedUntilLastEffectiveBan(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	sanctions := &store.SanctionStore{Pool: pool}
	accountID := uuid.New()
	issuer := autoModIssuerProfileID()
	expired, err := sanctions.InsertSanction(ctx, accountID, "temp_ban", "expired", nil, issuer, ptr(time.Now().UTC().Add(-time.Minute)))
	require.NoError(t, err)
	permanent, err := sanctions.InsertSanction(ctx, accountID, "perm_ban", "surviving", nil, issuer, nil)
	require.NoError(t, err)
	auth := &effectiveBanAuthClient{}
	svc := &ModerationGRPC{Sanctions: sanctions, Auth: auth}

	processed, err := svc.ProcessExpiredTempBans(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, processed)
	revoked, err := sanctions.GetByID(ctx, expired.ID)
	require.NoError(t, err)
	require.NotNil(t, revoked.RevokedAt)
	calls := auth.snapshot()
	require.Len(t, calls, 1)
	require.Equal(t, "suspended", calls[0].status, "expiry reconciliation must preserve suspension while a permanent ban survives")

	require.NoError(t, sanctions.RevokeSanction(ctx, permanent.ID, issuer))

	// After the final account ban is removed, a later expiry can reactivate the account.
	finalExpired, err := sanctions.InsertSanction(ctx, accountID, "temp_ban", "final expired", nil, issuer, ptr(time.Now().UTC().Add(-time.Minute)))
	require.NoError(t, err)

	processed, err = svc.ProcessExpiredTempBans(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, processed)
	calls = auth.snapshot()
	require.Len(t, calls, 2)
	require.Equal(t, accountID, calls[0].accountID)
	require.Equal(t, "suspended", calls[0].status)
	require.Equal(t, "active", calls[1].status)
	final, err := sanctions.GetByID(ctx, finalExpired.ID)
	require.NoError(t, err)
	require.NotNil(t, final.RevokedAt)
}

func TestRevokeSanctionOnlyActivatesAfterLastEffectiveAccountBan(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	auth := &effectiveBanAuthClient{}
	client, cleanup := startModerationGRPCTestServer(t, pool, func(svc *ModerationGRPC) { svc.Auth = auth })
	defer cleanup()
	accountID := uuid.New()
	moderator := uuid.New()
	issuer := autoModIssuerProfileID()
	first, err := (&store.SanctionStore{Pool: pool}).InsertSanction(ctx, accountID, "perm_ban", "first", nil, issuer, nil)
	require.NoError(t, err)
	second, err := (&store.SanctionStore{Pool: pool}).InsertSanction(ctx, accountID, "temp_ban", "second", nil, issuer, ptr(time.Now().UTC().Add(time.Hour)))
	require.NoError(t, err)

	_, err = client.RevokeSanction(withInternalModCtx(ctx, moderator), &moderationv1.RevokeSanctionRequest{SanctionId: first.ID.String()})
	require.NoError(t, err)
	calls := auth.snapshot()
	require.Len(t, calls, 1)
	require.Equal(t, "suspended", calls[0].status, "reconciliation must preserve Auth suspension while another ban survives")

	_, err = client.RevokeSanction(withInternalModCtx(ctx, moderator), &moderationv1.RevokeSanctionRequest{SanctionId: second.ID.String()})
	require.NoError(t, err)
	calls = auth.snapshot()
	require.Len(t, calls, 2)
	require.Equal(t, "suspended", calls[0].status)
	require.Equal(t, "active", calls[1].status)
}

func TestReviewAppealKeepsAccountSuspendedWhileAnotherBanSurvives(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	auth := &effectiveBanAuthClient{}
	client, cleanup := startModerationGRPCTestServer(t, pool, func(svc *ModerationGRPC) { svc.Auth = auth })
	defer cleanup()
	accountID := uuid.New()
	moderator := uuid.New()
	issuer := autoModIssuerProfileID()
	first, err := (&store.SanctionStore{Pool: pool}).InsertSanction(ctx, accountID, "perm_ban", "appealed", nil, issuer, nil)
	require.NoError(t, err)
	survivor, err := (&store.SanctionStore{Pool: pool}).InsertSanction(ctx, accountID, "perm_ban", "survivor", nil, issuer, nil)
	require.NoError(t, err)
	appeal, err := (&store.AppealStore{Pool: pool}).InsertAppeal(ctx, first.ID, accountID, "please review")
	require.NoError(t, err)

	_, err = client.ReviewAppeal(withInternalModCtx(ctx, moderator), &moderationv1.ReviewAppealRequest{AppealId: appeal.ID.String(), Status: "approved"})
	require.NoError(t, err)
	calls := auth.snapshot()
	require.Len(t, calls, 1)
	require.Equal(t, "suspended", calls[0].status, "appeal reconciliation must preserve Auth suspension while another ban survives")
	revoked, err := (&store.SanctionStore{Pool: pool}).GetByID(ctx, first.ID)
	require.NoError(t, err)
	require.NotNil(t, revoked.RevokedAt)

	lastAppeal, err := (&store.AppealStore{Pool: pool}).InsertAppeal(ctx, survivor.ID, accountID, "review final ban")
	require.NoError(t, err)
	_, err = client.ReviewAppeal(withInternalModCtx(ctx, moderator), &moderationv1.ReviewAppealRequest{AppealId: lastAppeal.ID.String(), Status: "approved"})
	require.NoError(t, err)
	calls = auth.snapshot()
	require.Len(t, calls, 2)
	require.Equal(t, accountID, calls[0].accountID)
	require.Equal(t, "suspended", calls[0].status)
	require.Equal(t, "active", calls[1].status)
}

func TestAccountBanRemovalSerializesWithNewBanApplication(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	old, err := (&store.SanctionStore{Pool: pool}).InsertSanction(ctx, uuid.New(), "perm_ban", "old", nil, autoModIssuerProfileID(), nil)
	require.NoError(t, err)
	accountID := old.TargetAccountID
	auth := &effectiveBanAuthClient{activeStarted: make(chan struct{}), releaseActive: make(chan struct{}), blockActive: true}
	client, cleanup := startModerationGRPCTestServer(t, pool, func(svc *ModerationGRPC) { svc.Auth = auth })
	defer cleanup()
	defer auth.release()
	moderator := uuid.New()

	revokeDone := make(chan error, 1)
	go func() {
		_, err := client.RevokeSanction(withInternalModCtx(ctx, moderator), &moderationv1.RevokeSanctionRequest{SanctionId: old.ID.String()})
		revokeDone <- err
	}()
	select {
	case <-auth.activeStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("revoke did not reach Auth status synchronization")
	}

	applyDone := make(chan error, 1)
	go func() {
		_, err := client.ApplySanction(withInternalModCtx(ctx, moderator), &moderationv1.ApplySanctionRequest{
			TargetAccountId: accountID.String(), Type: "perm_ban", Reason: "new ban",
		})
		applyDone <- err
	}()
	awaitAccountAdvisoryLockWait(t, pool)
	select {
	case err := <-applyDone:
		t.Fatalf("new ban completed although PostgreSQL reported its account advisory lock was blocked: %v", err)
	default:
	}

	auth.release()
	select {
	case err := <-revokeDone:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("revoke did not finish after Auth synchronization resumed")
	}
	select {
	case err := <-applyDone:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("new ban did not finish after account lock was released")
	}
	calls := auth.snapshot()
	require.Len(t, calls, 2)
	require.Equal(t, "active", calls[0].status)
	require.Equal(t, "suspended", calls[1].status)
	require.Equal(t, "suspended", calls[len(calls)-1].status)
	rows, err := (&store.SanctionStore{Pool: pool}).ListByAccount(ctx, accountID)
	require.NoError(t, err)
	var effective int
	for _, row := range rows {
		if row.Type == "perm_ban" && row.RevokedAt == nil {
			effective++
		}
	}
	require.Equal(t, 1, effective)
}
