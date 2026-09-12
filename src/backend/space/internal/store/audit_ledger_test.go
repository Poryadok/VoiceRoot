package store

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func applyAuditLedgerMigrationForTest(t *testing.T, ctx context.Context, st *SpaceStore) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000015_audit_ledger.up.sql"))
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, string(raw))
	require.NoError(t, err)
}

func auditLedgerStoreFixture(t *testing.T) *SpaceStore {
	t.Helper()
	if testing.Short() {
		t.Skip("requires PostgreSQL audit ledger migration")
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	applyAuditLedgerMigrationForTest(t, ctx, st)
	return st
}

func insertAuditFixture(t *testing.T, st *SpaceStore, id, spaceID, actorID uuid.UUID, action string, createdAt time.Time) {
	t.Helper()
	targetType := "profile"
	if action == "member_banned" || action == "member_unbanned" {
		targetType = "account"
	}
	_, err := st.Pool.Exec(context.Background(), `
INSERT INTO audit_log(id,space_id,actor_profile_id,action,target_type,target_id,details,created_at)
VALUES($1,$2,$3,$4,$5,$6,'{}'::jsonb,$7)`, id, spaceID, actorID, action, targetType, uuid.New(), createdAt)
	require.NoError(t, err)
}

func TestAuditLedgerMigration_InsertCreatesSameIDOutboxAndGuardsLedger(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	ctx := context.Background()
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Audit ledger", "", "private")
	require.NoError(t, err)
	auditID := uuid.New()
	insertAuditFixture(t, st, auditID, space.ID, owner, "member_banned", time.Now().UTC())

	var outboxID, outboxSpaceID uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT audit_event_id,space_id FROM audit_outbox WHERE audit_event_id=$1`, auditID).Scan(&outboxID, &outboxSpaceID))
	require.Equal(t, auditID, outboxID)
	require.Equal(t, space.ID, outboxSpaceID)

	_, err = st.Pool.Exec(ctx, `UPDATE audit_log SET action='changed' WHERE id=$1`, auditID)
	require.Error(t, err, "audit entries must be immutable")
	_, err = st.Pool.Exec(ctx, `DELETE FROM audit_log WHERE id=$1`, auditID)
	require.Error(t, err, "ordinary callers must not delete audit entries")

	_, err = st.Pool.Exec(ctx, `
INSERT INTO audit_log(space_id,actor_profile_id,action,target_type,target_id,details)
VALUES($1,$2,'oversized','space',$1,jsonb_build_object('payload',repeat('x',4097)))`, space.ID, owner)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "23514", pgErr.Code)

	require.Error(t, st.DeleteSpace(ctx, space.ID), "legacy hard delete must remain disabled until the terminal P3 purge transaction exists")
	assertAuditLedgerEvidence(t, st, space.ID, auditID, true)
}

func assertAuditLedgerEvidence(t *testing.T, st *SpaceStore, spaceID, auditID uuid.UUID, expected bool) {
	t.Helper()
	ctx := context.Background()
	var spaceExists, auditExists, outboxExists bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM spaces WHERE id=$1)`, spaceID).Scan(&spaceExists))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM audit_log WHERE id=$1)`, auditID).Scan(&auditExists))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM audit_outbox WHERE audit_event_id=$1)`, auditID).Scan(&outboxExists))
	require.Equal(t, expected, spaceExists)
	require.Equal(t, expected, auditExists)
	require.Equal(t, expected, outboxExists)
}

func TestAuditLedgerMigration_DeleteModesRequireDatabaseVerifiedPredicates(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	ctx := context.Background()
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Audit delete predicates", "", "private")
	require.NoError(t, err)

	freshDelivered := uuid.New()
	oldPending := uuid.New()
	freshPending := uuid.New()
	unauthorizedOwnership := uuid.New()
	insertAuditFixture(t, st, freshDelivered, space.ID, owner, "member_kicked", time.Now().UTC())
	insertAuditFixture(t, st, oldPending, space.ID, owner, "member_kicked", time.Now().UTC().Add(-366*24*time.Hour))
	insertAuditFixture(t, st, freshPending, space.ID, owner, "member_kicked", time.Now().UTC())
	_, err = st.Pool.Exec(ctx, `
INSERT INTO audit_log(id,space_id,actor_profile_id,action,target_type,target_id,details,created_at)
VALUES($1,$2,$3,'ownership_transferred','profile',$3,'{}'::jsonb,clock_timestamp())`, unauthorizedOwnership, space.ID, owner)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE audit_outbox SET delivered_at=clock_timestamp() WHERE audit_event_id=$1`, freshDelivered)
	require.NoError(t, err)

	for _, test := range []struct {
		name string
		mode string
		id   uuid.UUID
	}{
		{"retention rejects fresh delivered", "retention", freshDelivered},
		{"retention rejects old pending", "retention", oldPending},
		{"compensation rejects arbitrary pending audit", "compensation", freshPending},
		{"compensation rejects ownership audit without row capability", "compensation", unauthorizedOwnership},
		{"unknown mode rejects pending audit", "anything", freshPending},
	} {
		t.Run(test.name, func(t *testing.T) {
			tx, txErr := st.Pool.Begin(ctx)
			require.NoError(t, txErr)
			defer func() { _ = tx.Rollback(ctx) }()
			_, txErr = tx.Exec(ctx, `SELECT set_config('voice.audit_delete_mode',$1,true)`, test.mode)
			require.NoError(t, txErr)
			_, txErr = tx.Exec(ctx, `DELETE FROM audit_log WHERE id=$1`, test.id)
			require.Error(t, txErr)
		})
	}

	for _, id := range []uuid.UUID{freshDelivered, oldPending, freshPending, unauthorizedOwnership} {
		assertAuditLedgerEvidence(t, st, space.ID, id, true)
	}
}

func TestAuditLedgerMigration_DirectParentDeleteCannotCascadeEvidence(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	ctx := context.Background()
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "No direct purge", "", "private")
	require.NoError(t, err)
	auditID := uuid.New()
	insertAuditFixture(t, st, auditID, space.ID, owner, "member_kicked", time.Now().UTC())

	_, err = st.Pool.Exec(ctx, `DELETE FROM spaces WHERE id=$1`, space.ID)
	require.Error(t, err)
	assertAuditLedgerEvidence(t, st, space.ID, auditID, true)
}

func TestAuditLedgerMigration_EnforcesClosedActionTargetAndDetailsRegistry(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	ctx := context.Background()
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Audit registry", "", "private")
	require.NoError(t, err)
	assertCheckViolation := func(t *testing.T, action, targetType, details string) {
		t.Helper()
		_, err := st.Pool.Exec(ctx, `
INSERT INTO audit_log(space_id,actor_profile_id,action,target_type,target_id,details)
VALUES($1,$2,$3,$4,$5,$6::jsonb)`, space.ID, owner, action, targetType, uuid.New(), details)
		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr)
		require.Equal(t, "23514", pgErr.Code)
	}
	assertCheckViolation(t, "unknown_action", "profile", `{}`)
	assertCheckViolation(t, "member_banned", "profile", `{}`)
	assertCheckViolation(t, "member_banned", "account", `{"proof":"must not enter audit"}`)
	assertCheckViolation(t, "member_timed_out", "profile", `{"duration_seconds":"60"}`)
	assertCheckViolation(t, "member_timed_out", "profile", `{}`)

	_, err = st.Pool.Exec(ctx, `
INSERT INTO audit_log(space_id,actor_profile_id,action,target_type,target_id,details)
VALUES($1,$2,'member_banned','account',$3,'{"reason":"spam"}'::jsonb),
      ($1,$2,'member_timed_out','profile',$4,'{"duration_seconds":60,"reason":"spam"}'::jsonb)`,
		space.ID, owner, uuid.New(), uuid.New())
	require.NoError(t, err)
}

func TestAuditLedgerMigration_FailedMutationLeavesNoAuditOrOutbox(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	ctx := context.Background()
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Before", "", "private")
	require.NoError(t, err)

	tx, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `UPDATE spaces SET name='After' WHERE id=$1`, space.ID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `
INSERT INTO audit_log(space_id,actor_profile_id,action,target_type,target_id,details)
VALUES($1,$2,'space_updated','space',$1,jsonb_build_object('payload',repeat('x',4097)))`, space.ID, owner)
	require.Error(t, err)
	require.NoError(t, tx.Rollback(ctx))

	var name string
	var audits, outbox int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT name FROM spaces WHERE id=$1`, space.ID).Scan(&name))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1`, space.ID).Scan(&audits))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_outbox WHERE space_id=$1`, space.ID).Scan(&outbox))
	require.Equal(t, "Before", name)
	require.Zero(t, audits)
	require.Zero(t, outbox)
}

func TestSpaceStore_DeleteAuditLogEntry_LegacyCompensationRequiresExactCapabilityAndPredicates(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	ctx := context.Background()
	previousOwner := uuid.New()
	newOwner := uuid.New()
	space, err := st.CreateSpace(ctx, previousOwner, "Audit compensation", "", "private")
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, newOwner)
	require.NoError(t, err)
	require.NoError(t, st.TransferOwnership(ctx, space.ID, previousOwner, newOwner))
	auditID := uuid.New()
	const token = "1111111111111111111111111111111111111111111111111111111111111111"
	legacy := st.LegacyOwnershipLeaseStoreForTest()
	require.NoError(t, legacy.RecordOwnershipTransferred(ctx, auditID, space.ID, previousOwner, newOwner, token))

	require.Error(t, legacy.DeleteAuditLogEntry(ctx, auditID, "2222222222222222222222222222222222222222222222222222222222222222"))
	assertAuditLedgerEvidence(t, st, space.ID, auditID, true)
	require.NoError(t, legacy.DeleteAuditLogEntry(ctx, auditID, token))
	var auditExists, outboxExists bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM audit_log WHERE id=$1)`, auditID).Scan(&auditExists))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM audit_outbox WHERE audit_event_id=$1)`, auditID).Scan(&outboxExists))
	require.False(t, auditExists)
	require.False(t, outboxExists)

	deliveredID := uuid.New()
	const deliveredToken = "4444444444444444444444444444444444444444444444444444444444444444"
	require.NoError(t, legacy.RecordOwnershipTransferred(ctx, deliveredID, space.ID, previousOwner, newOwner, deliveredToken))
	_, err = st.Pool.Exec(ctx, `UPDATE audit_outbox SET delivered_at=clock_timestamp() WHERE audit_event_id=$1`, deliveredID)
	require.NoError(t, err)
	require.Error(t, legacy.DeleteAuditLogEntry(ctx, deliveredID, deliveredToken), "delivered ownership evidence is no longer compensatable")
	assertAuditLedgerEvidence(t, st, space.ID, deliveredID, true)

	ownerMismatchID := uuid.New()
	otherTarget := uuid.New()
	const ownerMismatchToken = "5555555555555555555555555555555555555555555555555555555555555555"
	require.NoError(t, legacy.RecordOwnershipTransferred(ctx, ownerMismatchID, space.ID, previousOwner, otherTarget, ownerMismatchToken))
	require.Error(t, legacy.DeleteAuditLogEntry(ctx, ownerMismatchID, ownerMismatchToken), "database owner must still equal the exact transfer target")
	assertAuditLedgerEvidence(t, st, space.ID, ownerMismatchID, true)
}

func TestAuditLedgerMigration_ConcurrentOutboxClaimRejectsCompensationAndPreservesEvidence(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	previousOwner := uuid.New()
	newOwner := uuid.New()
	space, err := st.CreateSpace(ctx, previousOwner, "Audit claim race", "", "private")
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, newOwner)
	require.NoError(t, err)
	require.NoError(t, st.TransferOwnership(ctx, space.ID, previousOwner, newOwner))
	auditID := uuid.New()
	const token = "6666666666666666666666666666666666666666666666666666666666666666"
	legacy := st.LegacyOwnershipLeaseStoreForTest()
	require.NoError(t, legacy.RecordOwnershipTransferred(ctx, auditID, space.ID, previousOwner, newOwner, token))

	claimTx, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	var claimedAuditID, leaseToken uuid.UUID
	err = claimTx.QueryRow(ctx, `
WITH candidates AS (
	SELECT audit_event_id
	FROM audit_outbox
	WHERE delivered_at IS NULL
	  AND next_attempt_at <= clock_timestamp()
	  AND (lease_expires_at IS NULL OR lease_expires_at <= clock_timestamp())
	ORDER BY created_at,audit_event_id
	FOR UPDATE SKIP LOCKED
	LIMIT 1
), claimed AS (
	UPDATE audit_outbox AS outbox
	SET lease_token=gen_random_uuid(),
		lease_expires_at=clock_timestamp()+interval '1 microsecond' * $1
	FROM candidates
	WHERE outbox.audit_event_id=candidates.audit_event_id
	RETURNING outbox.audit_event_id,outbox.lease_token,outbox.lease_expires_at
)
SELECT audit.id,claimed.lease_token
FROM claimed
JOIN audit_log AS audit ON audit.id=claimed.audit_event_id
ORDER BY audit.created_at,audit.id`, time.Minute.Microseconds()).Scan(&claimedAuditID, &leaseToken)
	require.NoError(t, err)
	require.Equal(t, auditID, claimedAuditID)
	require.NotEqual(t, uuid.Nil, leaseToken)

	compensationConn, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	var compensationPID int32
	require.NoError(t, compensationConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&compensationPID))
	compensationTx, err := compensationConn.Begin(ctx)
	require.NoError(t, err)
	_, err = compensationTx.Exec(ctx, `SELECT set_config('voice.audit_delete_mode','compensation',true),set_config('voice.audit_compensation_token',$1,true)`, token)
	require.NoError(t, err)

	deleteDone := make(chan error, 1)
	deleteFinished := false
	claimClosed := false
	compensationClosed := false
	t.Cleanup(func() {
		cancel()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if !claimClosed {
			_ = claimTx.Rollback(cleanupCtx)
		}
		if !deleteFinished {
			select {
			case <-deleteDone:
			case <-cleanupCtx.Done():
			}
		}
		if !compensationClosed {
			_ = compensationTx.Rollback(cleanupCtx)
		}
		compensationConn.Release()
	})
	go func() {
		_, deleteErr := compensationTx.Exec(ctx, `DELETE FROM audit_log WHERE id=$1`, auditID)
		deleteDone <- deleteErr
	}()

	require.Eventually(t, func() bool {
		var waitEventType string
		queryErr := st.Pool.QueryRow(ctx, `
SELECT COALESCE(wait_event_type,'')
FROM pg_stat_activity
WHERE pid=$1`, compensationPID).Scan(&waitEventType)
		return queryErr == nil && waitEventType == "Lock"
	}, 5*time.Second, 20*time.Millisecond, "compensation must reach the outbox row lock before the claim commits")

	require.NoError(t, claimTx.Commit(ctx))
	claimClosed = true
	var deleteErr error
	select {
	case deleteErr = <-deleteDone:
		deleteFinished = true
	case <-ctx.Done():
		require.FailNow(t, "compensation did not finish after the concurrent claim committed", ctx.Err().Error())
	}
	if deleteErr == nil {
		require.NoError(t, compensationTx.Commit(ctx))
	} else {
		require.NoError(t, compensationTx.Rollback(ctx))
	}
	compensationClosed = true

	assertAuditLedgerEvidence(t, st, space.ID, auditID, true)
	require.Error(t, deleteErr, "a committed outbox claim must make legacy compensation fail closed")
	var persistedLease uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT lease_token FROM audit_outbox WHERE audit_event_id=$1`, auditID).Scan(&persistedLease))
	require.Equal(t, leaseToken, persistedLease)
}

func TestSpaceStore_DeleteAuditLogEntry_RollbackPreservesAuthorizedEvidence(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	ctx := context.Background()
	previousOwner := uuid.New()
	newOwner := uuid.New()
	space, err := st.CreateSpace(ctx, previousOwner, "Audit compensation rollback", "", "private")
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, newOwner)
	require.NoError(t, err)
	require.NoError(t, st.TransferOwnership(ctx, space.ID, previousOwner, newOwner))
	auditID := uuid.New()
	const token = "3333333333333333333333333333333333333333333333333333333333333333"
	legacy := st.LegacyOwnershipLeaseStoreForTest()
	require.NoError(t, legacy.RecordOwnershipTransferred(ctx, auditID, space.ID, previousOwner, newOwner, token))

	tx, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `SELECT set_config('voice.audit_delete_mode','compensation',true),set_config('voice.audit_compensation_token',$1,true)`, token)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `DELETE FROM audit_log WHERE id=$1`, auditID)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	assertAuditLedgerEvidence(t, st, space.ID, auditID, true)
}

func TestSpaceStore_KickMember_CommitsMutationAuditAndOutboxAtomically(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	ctx := context.Background()
	owner := uuid.New()
	member := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Kick audit", "", "private")
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, member)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE spaces SET member_count=2 WHERE id=$1`, space.ID)
	require.NoError(t, err)

	require.NoError(t, st.KickMember(ctx, space.ID, member, owner))
	var members, audits, outbox int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, member).Scan(&members))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='member_kicked' AND target_id=$2`, space.ID, member).Scan(&audits))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_outbox o JOIN audit_log a ON a.id=o.audit_event_id WHERE a.space_id=$1 AND a.action='member_kicked'`, space.ID).Scan(&outbox))
	require.Zero(t, members)
	require.Equal(t, 1, audits)
	require.Equal(t, 1, outbox)
}

func TestSpaceStore_KickMember_AuditFailureRollsBackMutation(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	ctx := context.Background()
	owner := uuid.New()
	member := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Kick rollback", "", "private")
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, member)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE spaces SET member_count=2 WHERE id=$1`, space.ID)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `
CREATE FUNCTION reject_kick_audit() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
	IF NEW.action='member_kicked' THEN RAISE EXCEPTION 'injected audit failure'; END IF;
	RETURN NEW;
END $$;
CREATE TRIGGER reject_kick_audit BEFORE INSERT ON audit_log FOR EACH ROW EXECUTE FUNCTION reject_kick_audit()`)
	require.NoError(t, err)

	require.Error(t, st.KickMember(ctx, space.ID, member, owner))
	var members, memberCount, audits, outbox int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, member).Scan(&members))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id=$1`, space.ID).Scan(&memberCount))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1`, space.ID).Scan(&audits))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_outbox WHERE space_id=$1`, space.ID).Scan(&outbox))
	require.Equal(t, 1, members)
	require.Equal(t, 2, memberCount)
	require.Zero(t, audits)
	require.Zero(t, outbox)
}

func TestSpaceStore_ModerationAuditDetailsUseAllowlistedCanonicalFields(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	ctx := context.Background()
	owner := uuid.New()
	member := uuid.New()
	account := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Moderation details", "", "private")
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, member)
	require.NoError(t, err)
	reason := "repeated spam"
	require.NoError(t, st.BanMember(ctx, space.ID, account, owner, &reason, nil))
	require.NoError(t, st.SetMemberTimeout(ctx, space.ID, member, owner, 600, &reason))

	var banDetails, timeoutDetails string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT details::text FROM audit_log WHERE space_id=$1 AND action='member_banned'`, space.ID).Scan(&banDetails))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT details::text FROM audit_log WHERE space_id=$1 AND action='member_timed_out'`, space.ID).Scan(&timeoutDetails))
	require.JSONEq(t, `{"reason":"repeated spam"}`, banDetails)
	require.JSONEq(t, `{"duration_seconds":600,"reason":"repeated spam"}`, timeoutDetails)
	var outbox int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_outbox WHERE space_id=$1`, space.ID).Scan(&outbox))
	require.Equal(t, 2, outbox)
}

func TestSpaceStore_AuditOutboxClaimFailureRetryAndDelivery(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	ctx := context.Background()
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Outbox delivery", "", "private")
	require.NoError(t, err)
	first := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	second := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	createdAt := time.Now().UTC().Add(-time.Minute)
	insertAuditFixture(t, st, first, space.ID, owner, "member_kicked", createdAt)
	insertAuditFixture(t, st, second, space.ID, owner, "member_timeout_removed", createdAt.Add(time.Second))

	claimed, err := st.ClaimAuditOutbox(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, first, claimed[0].AuditEventID)
	require.NotEqual(t, uuid.Nil, claimed[0].LeaseToken)

	marked, err := st.MarkAuditOutboxFailed(ctx, first, claimed[0].LeaseToken)
	require.NoError(t, err)
	require.True(t, marked)
	_, err = st.Pool.Exec(ctx, `UPDATE audit_outbox SET next_attempt_at=clock_timestamp()+interval '1 hour' WHERE audit_event_id=$1`, first)
	require.NoError(t, err)
	claimed, err = st.ClaimAuditOutbox(ctx, 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, claimed, 1, "failed event must wait for its retry deadline")
	require.Equal(t, second, claimed[0].AuditEventID)

	delivered, err := st.MarkAuditOutboxDelivered(ctx, second, claimed[0].LeaseToken)
	require.NoError(t, err)
	require.True(t, delivered)
	delivered, err = st.MarkAuditOutboxDelivered(ctx, second, claimed[0].LeaseToken)
	require.NoError(t, err)
	require.False(t, delivered, "delivery acknowledgement is lease-token idempotent")
	_, err = st.Pool.Exec(ctx, `UPDATE audit_outbox SET next_attempt_at='-infinity'::timestamptz WHERE audit_event_id=$1`, first)
	require.NoError(t, err)
	claimed, err = st.ClaimAuditOutbox(ctx, 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, first, claimed[0].AuditEventID, "retry keeps the stable audit_event_id dedupe key")
}

func TestSpaceStore_ListAuditLogFilteredPage_BindsSignedSnapshotCursor(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	ctx := context.Background()
	owner := uuid.New()
	otherActor := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Filtered audit", "", "private")
	require.NoError(t, err)
	other, err := st.CreateSpace(ctx, owner, "Other audit", "", "private")
	require.NoError(t, err)
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	from := now.Add(-time.Hour)
	to := now.Add(time.Hour)
	matchingIDs := []uuid.UUID{
		uuid.MustParse("ffffffff-ffff-4fff-8fff-ffffffffffff"),
		uuid.MustParse("eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"),
		uuid.MustParse("dddddddd-dddd-4ddd-8ddd-dddddddddddd"),
	}
	for i, id := range matchingIDs {
		insertAuditFixture(t, st, id, space.ID, owner, "member_banned", now.Add(-time.Duration(i)*time.Minute))
	}
	insertAuditFixture(t, st, uuid.New(), space.ID, otherActor, "member_banned", now)
	insertAuditFixture(t, st, uuid.New(), space.ID, owner, "member_unbanned", now)
	insertAuditFixture(t, st, uuid.New(), other.ID, owner, "member_banned", now)
	insertAuditFixture(t, st, uuid.New(), space.ID, owner, "member_banned", from.Add(-time.Nanosecond))
	insertAuditFixture(t, st, uuid.New(), space.ID, owner, "member_banned", to)

	action := "member_banned"
	query := AuditLogQuery{
		SpaceID:        space.ID,
		ActorProfileID: &owner,
		Action:         &action,
		From:           &from,
		To:             &to,
		PageSize:       1,
		AccessScope:    "reader:" + owner.String(),
	}
	key := bytes.Repeat([]byte{0xa5}, 32)
	page1, err := st.ListAuditLogFilteredPage(ctx, query, key, now)
	require.NoError(t, err)
	require.Len(t, page1.Rows, 1)
	require.Equal(t, matchingIDs[0], page1.Rows[0].ID)
	require.NotEmpty(t, page1.NextCursor)

	insertedAfterSnapshot := uuid.New()
	insertAuditFixture(t, st, insertedAfterSnapshot, space.ID, owner, action, now.Add(30*time.Minute))
	query.Cursor = page1.NextCursor
	page2, err := st.ListAuditLogFilteredPage(ctx, query, key, now.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, page2.Rows, 1)
	require.Equal(t, matchingIDs[1], page2.Rows[0].ID)
	require.NotEqual(t, insertedAfterSnapshot, page2.Rows[0].ID)

	query.Cursor = page2.NextCursor
	page3, err := st.ListAuditLogFilteredPage(ctx, query, key, now.Add(2*time.Minute))
	require.NoError(t, err)
	require.Len(t, page3.Rows, 1)
	require.Equal(t, matchingIDs[2], page3.Rows[0].ID)
	require.Empty(t, page3.NextCursor)
}

func TestSpaceStore_ListAuditLogFilteredPage_RejectsCursorReplayAndExpiry(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	ctx := context.Background()
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Cursor binding", "", "private")
	require.NoError(t, err)
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	insertAuditFixture(t, st, uuid.New(), space.ID, owner, "member_banned", now)
	insertAuditFixture(t, st, uuid.New(), space.ID, owner, "member_banned", now.Add(-time.Minute))
	key := bytes.Repeat([]byte{0x3c}, 32)
	action := "member_banned"
	base := AuditLogQuery{SpaceID: space.ID, Action: &action, PageSize: 1, AccessScope: "scope-a"}
	page, err := st.ListAuditLogFilteredPage(ctx, base, key, now)
	require.NoError(t, err)
	require.NotEmpty(t, page.NextCursor)

	assertInvalid := func(t *testing.T, q AuditLogQuery, cursorKey []byte, observedAt time.Time) {
		t.Helper()
		q.Cursor = page.NextCursor
		_, err := st.ListAuditLogFilteredPage(ctx, q, cursorKey, observedAt)
		require.ErrorIs(t, err, ErrInvalidAuditCursor)
	}
	t.Run("space", func(t *testing.T) { q := base; q.SpaceID = uuid.New(); assertInvalid(t, q, key, now) })
	t.Run("access scope", func(t *testing.T) { q := base; q.AccessScope = "scope-b"; assertInvalid(t, q, key, now) })
	t.Run("action", func(t *testing.T) {
		q := base
		changed := "member_unbanned"
		q.Action = &changed
		assertInvalid(t, q, key, now)
	})
	t.Run("page size", func(t *testing.T) { q := base; q.PageSize = 2; assertInvalid(t, q, key, now) })
	t.Run("signature", func(t *testing.T) { assertInvalid(t, base, bytes.Repeat([]byte{0x4d}, 32), now) })
	t.Run("expired", func(t *testing.T) { assertInvalid(t, base, key, now.Add(15*time.Minute+time.Nanosecond)) })
	t.Run("tampered", func(t *testing.T) {
		q := base
		replacement := byte('A')
		if page.NextCursor[0] == replacement {
			replacement = 'B'
		}
		q.Cursor = string(replacement) + page.NextCursor[1:]
		_, err := st.ListAuditLogFilteredPage(ctx, q, key, now)
		require.ErrorIs(t, err, ErrInvalidAuditCursor)
	})
}

func TestSpaceStore_ListAuditLogSignedPage_UsesDatabaseKeyAcrossStoreInstances(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	ctx := context.Background()
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Database cursor key", "", "private")
	require.NoError(t, err)
	now := time.Now().UTC()
	insertAuditFixture(t, st, uuid.New(), space.ID, owner, "member_kicked", now)
	insertAuditFixture(t, st, uuid.New(), space.ID, owner, "member_kicked", now.Add(-time.Minute))

	page1, err := st.ListAuditLogSignedPage(ctx, space.ID, "", 1, "reader:"+owner.String(), nil)
	require.NoError(t, err)
	require.Len(t, page1.Rows, 1)
	require.NotEmpty(t, page1.NextCursor)
	otherStore := &SpaceStore{Pool: st.Pool}
	page2, err := otherStore.ListAuditLogSignedPage(ctx, space.ID, page1.NextCursor, 1, "reader:"+owner.String(), nil)
	require.NoError(t, err)
	require.Len(t, page2.Rows, 1)
	require.Empty(t, page2.NextCursor)
}

func TestSpaceStore_CleanupExpiredAuditLog_OnlyDeliveredPastRetention(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	ctx := context.Background()
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Retention", "", "private")
	require.NoError(t, err)
	oldDelivered := uuid.New()
	oldPending := uuid.New()
	freshDelivered := uuid.New()
	insertAuditFixture(t, st, oldDelivered, space.ID, owner, "member_kicked", time.Now().UTC().Add(-366*24*time.Hour))
	insertAuditFixture(t, st, oldPending, space.ID, owner, "member_kicked", time.Now().UTC().Add(-366*24*time.Hour))
	insertAuditFixture(t, st, freshDelivered, space.ID, owner, "member_kicked", time.Now().UTC().Add(-364*24*time.Hour))
	_, err = st.Pool.Exec(ctx, `UPDATE audit_outbox SET delivered_at=clock_timestamp() WHERE audit_event_id IN ($1,$2)`, oldDelivered, freshDelivered)
	require.NoError(t, err)

	deleted, err := st.CleanupExpiredAuditLog(ctx, 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)
	for id, want := range map[uuid.UUID]bool{oldDelivered: false, oldPending: true, freshDelivered: true} {
		var exists bool
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM audit_log WHERE id=$1)`, id).Scan(&exists))
		require.Equal(t, want, exists, id.String())
	}
}

func TestAuditLedgerMigration_DownRefusesFreshPendingOutboxAndPreservesEvidence(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	ctx := context.Background()
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Pending migration evidence", "", "private")
	require.NoError(t, err)
	auditID := uuid.New()
	insertAuditFixture(t, st, auditID, space.ID, owner, "member_kicked", time.Now().UTC())

	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000015_audit_ledger.down.sql"))
	require.NoError(t, err)
	conn, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, string(raw))
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "P0001", pgErr.Code)
	_, _ = conn.Exec(ctx, "ROLLBACK")

	assertAuditLedgerEvidence(t, st, space.ID, auditID, true)
	var triggerExists bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='audit_log_immutable' AND NOT tgisinternal)`).Scan(&triggerExists))
	require.True(t, triggerExists)
}
