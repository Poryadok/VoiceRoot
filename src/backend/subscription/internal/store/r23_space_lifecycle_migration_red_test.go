package store

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

const (
	r23MigrationBase      = "000001_init.up.sql"
	r23MigrationReminder  = "000002_grace_reminders.up.sql"
	r23MigrationUp        = "000003_space_lifecycle_provider_dedup.up.sql"
	r23MigrationDown      = "000003_space_lifecycle_provider_dedup.down.sql"
	r23ProviderHMACDomain = "voice-subscription-provider-dedup-v1"
)

var r23OwnedTables = []string{
	"subscription_provider_event_fences",
	"subscription_space_lifecycle_fences",
	"subscription_space_lifecycle_operations",
}

// These narrow test interfaces deliberately bind tests to production methods
// without supplying a test implementation. The RED signal is the missing
// SubscriptionStore roots, not a fake that can make the contract pass itself.
type r23ProviderEventRecorder interface {
	RecordProviderEventTx(
		ctx context.Context,
		tx pgx.Tx,
		provider string,
		eventID []byte,
		terminalOutcomeClass string,
		keyVersion string,
		hmacKey []byte,
	) (replayed bool, err error)
}

type r23LifecycleEvidenceCleaner interface {
	CleanupSpaceLifecycleEvidence(ctx context.Context, now time.Time) (compacted int64, err error)
}

func TestR23ProviderEventHMACDomainVector(t *testing.T) {
	key := []byte("r23-subscription-dedup-test-key")
	digest := r23ProviderEventDigest(key, "PADDLE", []byte("evt_01HZZ raw bytes \x00 preserved"))
	require.Equal(t, "ebcef3a585e946853ad59ad7d73b833a43e7044325ada0ff112f8b6a9937a6c9", hex.EncodeToString(digest))
	require.Equal(t, digest, r23ProviderEventDigest(key, "paddle", []byte("evt_01HZZ raw bytes \x00 preserved")), "provider is canonical lowercase")
	require.NotEqual(t, digest, r23ProviderEventDigest(key, "cloudpayments", []byte("evt_01HZZ raw bytes \x00 preserved")))
	require.NotEqual(t, digest, r23ProviderEventDigest(key, "paddle", []byte("evt_01hzz raw bytes \x00 preserved")), "event bytes are exact and case-sensitive")
}

func TestR23SubscriptionMigrationSchemaAndPermanentFences(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL migration contract requires testcontainers")
	}
	ctx := context.Background()
	pool := r23StartSubscriptionPostgres(t, ctx)

	require.Equal(t, r23OwnedTables, r23ExistingOwnedTables(t, ctx, pool))
	require.Equal(t, []string{
		"first_seen_at",
		"key_version",
		"provider",
		"provider_event_hmac",
		"retain_until",
		"terminal_outcome_class",
	}, r23Columns(t, ctx, pool, "subscription_provider_event_fences"), "compact provider fence must retain no raw provider event ID or billing detail")
	require.Equal(t, []string{
		"first_seen_at|timestamptz|NOT NULL",
		"key_version|text|NOT NULL",
		"provider|text|NOT NULL",
		"provider_event_hmac|bytea|NOT NULL",
		"retain_until|timestamptz|NOT NULL",
		"terminal_outcome_class|text|NOT NULL",
	}, r23ColumnMetadata(t, ctx, pool, "subscription_provider_event_fences"))
	require.Equal(t, []string{
		"applied_at",
		"deletion_operation_id",
		"generation",
		"manifest_id",
		"manifest_item_count",
		"manifest_sha256",
		"receipt_id",
		"request_sha256",
		"space_id",
		"state",
	}, r23Columns(t, ctx, pool, "subscription_space_lifecycle_fences"))
	require.Equal(t, []string{
		"applied_at|timestamptz|NOT NULL",
		"deletion_operation_id|uuid|NOT NULL",
		"generation|int8|NOT NULL",
		"manifest_id|text|NOT NULL",
		"manifest_item_count|int8|NOT NULL",
		"manifest_sha256|bytea|NOT NULL",
		"receipt_id|uuid|NOT NULL",
		"request_sha256|bytea|NOT NULL",
		"space_id|uuid|NOT NULL",
		"state|text|NOT NULL",
	}, r23ColumnMetadata(t, ctx, pool, "subscription_space_lifecycle_fences"))
	require.Equal(t, []string{
		"completed_at",
		"deletion_operation_id",
		"generation",
		"operation_kind",
		"provider_cancel_attempts",
		"provider_cancel_idempotency_key",
		"provider_cancel_last_error",
		"provider_cancel_next_attempt_at",
		"provider_cancel_state",
		"receipt_bytes",
		"receipt_id",
		"request_bytes",
		"request_sha256",
		"retain_until",
		"space_id",
		"terminal_state",
	}, r23Columns(t, ctx, pool, "subscription_space_lifecycle_operations"))
	require.Equal(t, []string{
		"completed_at|timestamptz|NULL",
		"deletion_operation_id|uuid|NOT NULL",
		"generation|int8|NOT NULL",
		"operation_kind|text|NOT NULL",
		"provider_cancel_attempts|int4|NOT NULL",
		"provider_cancel_idempotency_key|text|NULL",
		"provider_cancel_last_error|text|NULL",
		"provider_cancel_next_attempt_at|timestamptz|NULL",
		"provider_cancel_state|text|NULL",
		"receipt_bytes|bytea|NULL",
		"receipt_id|uuid|NULL",
		"request_bytes|bytea|NULL",
		"request_sha256|bytea|NOT NULL",
		"retain_until|timestamptz|NULL",
		"space_id|uuid|NOT NULL",
		"terminal_state|text|NOT NULL",
	}, r23ColumnMetadata(t, ctx, pool, "subscription_space_lifecycle_operations"))
	require.Equal(t, []string{"provider", "provider_event_hmac"}, r23PrimaryKeyColumns(t, ctx, pool, "subscription_provider_event_fences"))
	require.Equal(t, []string{"space_id"}, r23PrimaryKeyColumns(t, ctx, pool, "subscription_space_lifecycle_fences"))
	require.Equal(t, []string{"space_id", "deletion_operation_id", "generation", "operation_kind"}, r23PrimaryKeyColumns(t, ctx, pool, "subscription_space_lifecycle_operations"))
	require.Equal(t, []string{"receipt_id"}, r23SingleColumnUniqueKeys(t, ctx, pool, "subscription_space_lifecycle_fences"))
	require.Equal(t, []string{"receipt_id"}, r23SingleColumnUniqueKeys(t, ctx, pool, "subscription_space_lifecycle_operations"))
	for _, table := range r23OwnedTables {
		require.Zero(t, r23ForeignKeyCount(t, ctx, pool, table), "%s must remain authoritative after business-detail erasure and cannot FK to it", table)
	}

	spaceID, deletionID := uuid.New(), uuid.New()
	require.NoError(t, r23InsertLifecycleFence(ctx, pool, spaceID, deletionID, 9, "PURGED"))
	r23AssertEveryColumnImmutable(t, ctx, pool, "subscription_space_lifecycle_fences", "space_id", spaceID)

	digest := r23ProviderEventDigest([]byte("migration-key"), "paddle", []byte("evt-permanent"))
	require.NoError(t, r23InsertProviderFence(ctx, pool, "paddle", digest, "space_pro_activated", "kms-test-v1"))
	r23AssertEveryColumnImmutable(t, ctx, pool, "subscription_provider_event_fences", "provider_event_hmac", digest)

	completed := time.Now().UTC().Truncate(time.Microsecond)
	opSpace, opDeletion := uuid.New(), uuid.New()
	require.NoError(t, r23InsertLifecycleOperation(ctx, pool, opSpace, opDeletion, 9, completed, completed.Add(30*24*time.Hour)))
	r23AssertEveryColumnImmutable(t, ctx, pool, "subscription_space_lifecycle_operations", "space_id", opSpace)

	invalidEvidence := []struct {
		name string
		sql  string
		args []any
	}{
		{"provider must be canonical", `INSERT INTO subscription_provider_event_fences(provider,provider_event_hmac,first_seen_at,terminal_outcome_class,key_version,retain_until) VALUES('PADDLE',$1,now(),'accepted','v1','infinity')`, []any{bytesOf(32, 1)}},
		{"provider set is closed", `INSERT INTO subscription_provider_event_fences(provider,provider_event_hmac,first_seen_at,terminal_outcome_class,key_version,retain_until) VALUES('stripe',$1,now(),'accepted','v1','infinity')`, []any{bytesOf(32, 1)}},
		{"provider digest is sha256", `INSERT INTO subscription_provider_event_fences(provider,provider_event_hmac,first_seen_at,terminal_outcome_class,key_version,retain_until) VALUES('paddle',$1,now(),'accepted','v1','infinity')`, []any{bytesOf(31, 1)}},
		{"provider outcome is stable nonempty evidence", `INSERT INTO subscription_provider_event_fences(provider,provider_event_hmac,first_seen_at,terminal_outcome_class,key_version,retain_until) VALUES('paddle',$1,now(),'','v1','infinity')`, []any{bytesOf(32, 9)}},
		{"provider key version is nonempty evidence", `INSERT INTO subscription_provider_event_fences(provider,provider_event_hmac,first_seen_at,terminal_outcome_class,key_version,retain_until) VALUES('paddle',$1,now(),'accepted','','infinity')`, []any{bytesOf(32, 10)}},
		{"provider retention is permanent", `INSERT INTO subscription_provider_event_fences(provider,provider_event_hmac,first_seen_at,terminal_outcome_class,key_version,retain_until) VALUES('cloudpayments',$1,now(),'accepted','v1',now()+interval '1 year')`, []any{bytesOf(32, 2)}},
		{"lifecycle generation is positive", `INSERT INTO subscription_space_lifecycle_fences(space_id,deletion_operation_id,generation,state,manifest_id,manifest_sha256,manifest_item_count,request_sha256,receipt_id,applied_at) VALUES($1,$2,0,'FROZEN',$2::uuid::text,$3,0,$4,$5,now())`, []any{uuid.New(), uuid.New(), bytesOf(32, 1), bytesOf(32, 2), uuid.New()}},
		{"lifecycle state is closed", `INSERT INTO subscription_space_lifecycle_fences(space_id,deletion_operation_id,generation,state,manifest_id,manifest_sha256,manifest_item_count,request_sha256,receipt_id,applied_at) VALUES($1,$2,1,'UNKNOWN',$2::uuid::text,$3,0,$4,$5,now())`, []any{uuid.New(), uuid.New(), bytesOf(32, 1), bytesOf(32, 2), uuid.New()}},
		{"lifecycle hashes are sha256", `INSERT INTO subscription_space_lifecycle_fences(space_id,deletion_operation_id,generation,state,manifest_id,manifest_sha256,manifest_item_count,request_sha256,receipt_id,applied_at) VALUES($1,$2,1,'FROZEN',$2::uuid::text,$3,0,$4,$5,now())`, []any{uuid.New(), uuid.New(), bytesOf(31, 1), bytesOf(32, 2), uuid.New()}},
		{"manifest count is nonnegative", `INSERT INTO subscription_space_lifecycle_fences(space_id,deletion_operation_id,generation,state,manifest_id,manifest_sha256,manifest_item_count,request_sha256,receipt_id,applied_at) VALUES($1,$2,1,'FROZEN',$2::uuid::text,$3,-1,$4,$5,now())`, []any{uuid.New(), uuid.New(), bytesOf(32, 1), bytesOf(32, 2), uuid.New()}},
		{"full evidence retention is exact", `INSERT INTO subscription_space_lifecycle_operations(space_id,deletion_operation_id,generation,operation_kind,request_sha256,request_bytes,receipt_id,receipt_bytes,terminal_state,completed_at,retain_until,provider_cancel_state,provider_cancel_attempts,provider_cancel_idempotency_key) VALUES($1,$2,1,'PURGE',$3,'request',$4,'receipt','COMPLETED',$5::timestamptz,$5::timestamptz+interval '29 days','COMPLETED',1,'key')`, []any{uuid.New(), uuid.New(), bytesOf(32, 3), uuid.New(), completed}},
		{"full evidence hashes are sha256", `INSERT INTO subscription_space_lifecycle_operations(space_id,deletion_operation_id,generation,operation_kind,request_sha256,request_bytes,receipt_id,receipt_bytes,terminal_state,completed_at,retain_until,provider_cancel_state,provider_cancel_attempts,provider_cancel_idempotency_key) VALUES($1,$2,1,'PURGE',$3,'request',$4,'receipt','COMPLETED',$5::timestamptz,$5::timestamptz+interval '30 days','COMPLETED',1,'key')`, []any{uuid.New(), uuid.New(), bytesOf(31, 3), uuid.New(), completed}},
		{"operation generation is positive", `INSERT INTO subscription_space_lifecycle_operations(space_id,deletion_operation_id,generation,operation_kind,request_sha256,request_bytes,receipt_id,receipt_bytes,terminal_state,completed_at,retain_until,provider_cancel_state,provider_cancel_attempts,provider_cancel_idempotency_key) VALUES($1,$2,0,'PURGE',$3,'request',$4,'receipt','COMPLETED',$5::timestamptz,$5::timestamptz+interval '30 days','COMPLETED',1,'key')`, []any{uuid.New(), uuid.New(), bytesOf(32, 3), uuid.New(), completed}},
		{"operation kind is closed", `INSERT INTO subscription_space_lifecycle_operations(space_id,deletion_operation_id,generation,operation_kind,request_sha256,request_bytes,receipt_id,receipt_bytes,terminal_state,completed_at,retain_until,provider_cancel_state,provider_cancel_attempts,provider_cancel_idempotency_key) VALUES($1,$2,1,'UNKNOWN',$3,'request',$4,'receipt','COMPLETED',$5::timestamptz,$5::timestamptz+interval '30 days','COMPLETED',1,'key')`, []any{uuid.New(), uuid.New(), bytesOf(32, 3), uuid.New(), completed}},
		{"operation terminal state is closed", `INSERT INTO subscription_space_lifecycle_operations(space_id,deletion_operation_id,generation,operation_kind,request_sha256,request_bytes,receipt_id,receipt_bytes,terminal_state,completed_at,retain_until,provider_cancel_state,provider_cancel_attempts,provider_cancel_idempotency_key) VALUES($1,$2,1,'PURGE',$3,'request',$4,'receipt','UNKNOWN',$5::timestamptz,$5::timestamptz+interval '30 days','COMPLETED',1,'key')`, []any{uuid.New(), uuid.New(), bytesOf(32, 3), uuid.New(), completed}},
		{"provider cancellation state is closed", `INSERT INTO subscription_space_lifecycle_operations(space_id,deletion_operation_id,generation,operation_kind,request_sha256,request_bytes,receipt_id,receipt_bytes,terminal_state,completed_at,retain_until,provider_cancel_state,provider_cancel_attempts,provider_cancel_idempotency_key) VALUES($1,$2,1,'PURGE',$3,'request',$4,'receipt','COMPLETED',$5::timestamptz,$5::timestamptz+interval '30 days','UNKNOWN',1,'key')`, []any{uuid.New(), uuid.New(), bytesOf(32, 3), uuid.New(), completed}},
		{"provider cancellation attempts are nonnegative", `INSERT INTO subscription_space_lifecycle_operations(space_id,deletion_operation_id,generation,operation_kind,request_sha256,request_bytes,receipt_id,receipt_bytes,terminal_state,completed_at,retain_until,provider_cancel_state,provider_cancel_attempts,provider_cancel_idempotency_key) VALUES($1,$2,1,'PURGE',$3,'request',$4,'receipt','COMPLETED',$5::timestamptz,$5::timestamptz+interval '30 days','COMPLETED',-1,'key')`, []any{uuid.New(), uuid.New(), bytesOf(32, 3), uuid.New(), completed}},
		{"provider cancellation key is nonempty", `INSERT INTO subscription_space_lifecycle_operations(space_id,deletion_operation_id,generation,operation_kind,request_sha256,request_bytes,receipt_id,receipt_bytes,terminal_state,completed_at,retain_until,provider_cancel_state,provider_cancel_attempts,provider_cancel_idempotency_key) VALUES($1,$2,1,'PURGE',$3,'request',$4,'receipt','COMPLETED',$5::timestamptz,$5::timestamptz+interval '30 days','COMPLETED',1,'')`, []any{uuid.New(), uuid.New(), bytesOf(32, 3), uuid.New(), completed}},
		{"completed operation requires receipt and retention", `INSERT INTO subscription_space_lifecycle_operations(space_id,deletion_operation_id,generation,operation_kind,request_sha256,request_bytes,terminal_state,provider_cancel_state,provider_cancel_attempts,provider_cancel_idempotency_key) VALUES($1,$2,1,'PURGE',$3,'request','COMPLETED','COMPLETED',1,'key')`, []any{uuid.New(), uuid.New(), bytesOf(32, 3)}},
		{"retryable operation requires error and next attempt", `INSERT INTO subscription_space_lifecycle_operations(space_id,deletion_operation_id,generation,operation_kind,request_sha256,request_bytes,terminal_state,provider_cancel_state,provider_cancel_attempts,provider_cancel_idempotency_key) VALUES($1,$2,1,'PURGE',$3,'request','PENDING','RETRYABLE',1,'key')`, []any{uuid.New(), uuid.New(), bytesOf(32, 3)}},
		{"completed cancellation clears retry residue", `INSERT INTO subscription_space_lifecycle_operations(space_id,deletion_operation_id,generation,operation_kind,request_sha256,request_bytes,receipt_id,receipt_bytes,terminal_state,completed_at,retain_until,provider_cancel_state,provider_cancel_attempts,provider_cancel_idempotency_key,provider_cancel_last_error,provider_cancel_next_attempt_at) VALUES($1,$2,1,'PURGE',$3,'request',$4,'receipt','COMPLETED',$5::timestamptz,$5::timestamptz+interval '30 days','COMPLETED',2,'key','old error',$5)`, []any{uuid.New(), uuid.New(), bytesOf(32, 3), uuid.New(), completed}},
		{"pending operation cannot claim cancellation complete", `INSERT INTO subscription_space_lifecycle_operations(space_id,deletion_operation_id,generation,operation_kind,request_sha256,request_bytes,terminal_state,provider_cancel_state,provider_cancel_attempts,provider_cancel_idempotency_key) VALUES($1,$2,1,'PURGE',$3,'request','PENDING','COMPLETED',1,'key')`, []any{uuid.New(), uuid.New(), bytesOf(32, 3)}},
	}
	for _, tc := range invalidEvidence {
		t.Run(tc.name, func(t *testing.T) {
			_, insertErr := pool.Exec(ctx, tc.sql, tc.args...)
			r23RequireSQLState(t, insertErr, "23514")
		})
	}
}

func TestR23ProviderEventRecorderExactReplayConflictAndFailClosedKey(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL provider dedup contract requires testcontainers")
	}
	ctx := context.Background()
	pool := r23StartSubscriptionPostgres(t, ctx)
	store := &SubscriptionStore{Pool: pool}
	recorder, ok := any(store).(r23ProviderEventRecorder)
	require.True(t, ok, "production SubscriptionStore.RecordProviderEventTx root is absent")

	key := []byte("provider-key-material-for-r23")
	eventID := []byte("evt_exact_bytes_\x00_01")

	rolledBack, err := pool.Begin(ctx)
	require.NoError(t, err)
	replayed, err := recorder.RecordProviderEventTx(ctx, rolledBack, "PADDLE", eventID, "space_pro_activated", "kms-2026-09", key)
	require.NoError(t, err)
	require.False(t, replayed)
	require.NoError(t, rolledBack.Rollback(ctx))
	var rows int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM subscription_provider_event_fences`).Scan(&rows))
	require.Zero(t, rows, "a rolled-back provider attempt writes no fence")

	replayed, err = r23RecordProviderEvent(t, ctx, pool, recorder, "PADDLE", eventID, "space_pro_activated", "kms-2026-09", key)
	require.NoError(t, err)
	require.False(t, replayed)
	var firstSeen time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT first_seen_at FROM subscription_provider_event_fences`).Scan(&firstSeen))
	replayed, err = r23RecordProviderEvent(t, ctx, pool, recorder, "paddle", append([]byte(nil), eventID...), "space_pro_activated", "kms-2026-09", key)
	require.NoError(t, err)
	require.True(t, replayed, "canonical provider plus exact event bytes must replay")
	var replayFirstSeen time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT first_seen_at FROM subscription_provider_event_fences`).Scan(&replayFirstSeen))
	require.Equal(t, firstSeen, replayFirstSeen, "exact replay cannot rewrite first_seen_at")

	_, err = r23RecordProviderEvent(t, ctx, pool, recorder, "paddle", eventID, "space_pro_cancelled", "kms-2026-09", key)
	require.Error(t, err, "same permanent key with a changed terminal outcome must conflict")
	_, err = r23RecordProviderEvent(t, ctx, pool, recorder, "paddle", []byte("evt-no-key"), "space_pro_activated", "", nil)
	require.Error(t, err, "key lookup failure must fail closed")
	_, err = r23RecordProviderEvent(t, ctx, pool, recorder, "stripe", []byte("evt-unknown-provider"), "accepted", "kms-2026-09", key)
	require.Error(t, err, "the accepted provider set is Paddle and CloudPayments")
	replayed, err = r23RecordProviderEvent(t, ctx, pool, recorder, "CloudPayments", []byte("cp-event-exact"), "payment_settled", "kms-2026-09", key)
	require.NoError(t, err)
	require.False(t, replayed)

	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM subscription_provider_event_fences`).Scan(&rows))
	require.Equal(t, int64(2), rows, "Paddle and CloudPayments persist independently; failed and conflicting attempts write no fence")
	var provider, outcome, keyVersion string
	var gotDigest []byte
	var permanent bool
	require.NoError(t, pool.QueryRow(ctx, `
	SELECT provider,provider_event_hmac,terminal_outcome_class,key_version,retain_until='infinity'::timestamptz
	FROM subscription_provider_event_fences WHERE provider='paddle'`).Scan(&provider, &gotDigest, &outcome, &keyVersion, &permanent))
	require.Equal(t, "paddle", provider)
	require.Equal(t, r23ProviderEventDigest(key, "paddle", eventID), gotDigest)
	require.Equal(t, "space_pro_activated", outcome)
	require.Equal(t, "kms-2026-09", keyVersion)
	require.True(t, permanent, "permanent fence uses an unbounded retain_until")
}

func TestR23ProviderEventRecorderConcurrentOneDimensionMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL provider dedup races require testcontainers")
	}
	t.Run("identical first delivery and replay", func(t *testing.T) {
		ctx := context.Background()
		pool := r23StartSubscriptionPostgres(t, ctx)
		recorder, ok := any(&SubscriptionStore{Pool: pool}).(r23ProviderEventRecorder)
		require.True(t, ok, "production SubscriptionStore.RecordProviderEventTx root is absent")
		const callers = 8
		results := make(chan r23RecordResult, callers)
		start := make(chan struct{})
		for range callers {
			go func() {
				<-start
				replayed, err := r23RecordProviderEventConcurrent(ctx, pool, recorder, "paddle", []byte("evt-concurrent-identical"), "space_pro_activated", "kms-v1", []byte("same-key"))
				results <- r23RecordResult{replayed: replayed, err: err}
			}()
		}
		close(start)
		fresh, replayed := 0, 0
		for range callers {
			got := <-results
			require.NoError(t, got.err)
			if got.replayed {
				replayed++
			} else {
				fresh++
			}
		}
		require.Equal(t, 1, fresh)
		require.Equal(t, callers-1, replayed)
		require.Equal(t, int64(1), r23ProviderFenceCount(t, ctx, pool, "paddle", []byte("same-key"), []byte("evt-concurrent-identical")))
	})

	t.Run("changed terminal outcome only", func(t *testing.T) {
		ctx := context.Background()
		pool := r23StartSubscriptionPostgres(t, ctx)
		recorder, ok := any(&SubscriptionStore{Pool: pool}).(r23ProviderEventRecorder)
		require.True(t, ok, "production SubscriptionStore.RecordProviderEventTx root is absent")
		results := make(chan r23RecordResult, 2)
		start := make(chan struct{})
		for _, outcome := range []string{"space_pro_activated", "space_pro_cancelled"} {
			outcome := outcome
			go func() {
				<-start
				replayed, err := r23RecordProviderEventConcurrent(ctx, pool, recorder, "paddle", []byte("evt-concurrent-outcome"), outcome, "kms-v1", []byte("same-key"))
				results <- r23RecordResult{replayed: replayed, err: err}
			}()
		}
		close(start)
		successes, conflicts := 0, 0
		for range 2 {
			got := <-results
			if got.err != nil {
				conflicts++
				continue
			}
			require.False(t, got.replayed)
			successes++
		}
		require.Equal(t, 1, successes)
		require.Equal(t, 1, conflicts)
		require.Equal(t, int64(1), r23ProviderFenceCount(t, ctx, pool, "paddle", []byte("same-key"), []byte("evt-concurrent-outcome")))
	})

	t.Run("missing HMAC key only", func(t *testing.T) {
		ctx := context.Background()
		pool := r23StartSubscriptionPostgres(t, ctx)
		recorder, ok := any(&SubscriptionStore{Pool: pool}).(r23ProviderEventRecorder)
		require.True(t, ok, "production SubscriptionStore.RecordProviderEventTx root is absent")
		results := make(chan r23RecordResult, 2)
		start := make(chan struct{})
		for _, key := range [][]byte{nil, []byte("present-key")} {
			key := append([]byte(nil), key...)
			go func() {
				<-start
				replayed, err := r23RecordProviderEventConcurrent(ctx, pool, recorder, "cloudpayments", []byte("evt-concurrent-key"), "payment_settled", "kms-v1", key)
				results <- r23RecordResult{replayed: replayed, err: err}
			}()
		}
		close(start)
		successes, failedClosed := 0, 0
		for range 2 {
			got := <-results
			if got.err != nil {
				failedClosed++
				continue
			}
			require.False(t, got.replayed)
			successes++
		}
		require.Equal(t, 1, successes)
		require.Equal(t, 1, failedClosed)
		require.Equal(t, int64(1), r23ProviderFenceCount(t, ctx, pool, "cloudpayments", []byte("present-key"), []byte("evt-concurrent-key")))
	})
}

func TestR23LifecycleEvidenceCleanupRemovesOnlyExpiredFullBytes(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL retention contract requires testcontainers")
	}
	ctx := context.Background()
	pool := r23StartSubscriptionPostgres(t, ctx)
	store := &SubscriptionStore{Pool: pool}
	cleaner, ok := any(store).(r23LifecycleEvidenceCleaner)
	require.True(t, ok, "production SubscriptionStore.CleanupSpaceLifecycleEvidence root is absent")

	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	expiredSpace, liveSpace := uuid.New(), uuid.New()
	for _, seed := range []struct {
		spaceID    uuid.UUID
		completed  time.Time
		retainTill time.Time
	}{
		{expiredSpace, now.Add(-31 * 24 * time.Hour), now.Add(-24 * time.Hour)},
		{liveSpace, now.Add(-29 * 24 * time.Hour), now.Add(24 * time.Hour)},
	} {
		deletionID := uuid.New()
		require.NoError(t, r23InsertLifecycleOperation(ctx, pool, seed.spaceID, deletionID, 4, seed.completed, seed.retainTill))
		require.NoError(t, r23InsertLifecycleFence(ctx, pool, seed.spaceID, deletionID, 4, "PURGED"))
	}
	require.NoError(t, r23InsertProviderFence(ctx, pool, "cloudpayments", bytesOf(32, 8), "payment_settled", "kms-old-permanent"))

	compacted, err := cleaner.CleanupSpaceLifecycleEvidence(ctx, now)
	require.NoError(t, err)
	require.Equal(t, int64(1), compacted)
	require.Equal(t, []bool{true, false}, []bool{
		r23OperationBytesCleared(t, ctx, pool, expiredSpace),
		r23OperationBytesCleared(t, ctx, pool, liveSpace),
	})
	require.Equal(t, "PURGED", r23FenceState(t, ctx, pool, expiredSpace), "compact PURGED fence survives full-byte erasure")
	require.Equal(t, "PURGED", r23FenceState(t, ctx, pool, liveSpace))
	var providerFences int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM subscription_provider_event_fences`).Scan(&providerFences))
	require.Equal(t, int64(1), providerFences, "lifecycle cleanup cannot erase permanent provider fences or old key versions")
}

func TestR23SubscriptionMigrationGuardedDownRefusesEveryEvidenceClass(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL guarded DOWN contract requires testcontainers")
	}
	for _, table := range r23OwnedTables {
		t.Run(table, func(t *testing.T) {
			ctx := context.Background()
			pool := r23StartSubscriptionPostgres(t, ctx)
			r23SeedOneEvidenceRow(t, ctx, pool, table)
			before := r23SchemaSnapshot(t, ctx, pool)
			downSQL := r23ReadMigration(t, r23MigrationDown)
			conn, err := pool.Acquire(ctx)
			require.NoError(t, err)
			defer conn.Release()
			_, err = conn.Exec(ctx, downSQL)
			r23RequireSQLState(t, err, "55000")
			_, rollbackErr := conn.Exec(ctx, "ROLLBACK")
			require.NoError(t, rollbackErr)
			require.Equal(t, before, r23SchemaSnapshot(t, ctx, pool), "DOWN refusal must preserve the complete schema")
			var count int64
			require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count))
			require.Equal(t, int64(1), count)
		})
	}
}

func TestR23SubscriptionMigrationDownRechecksConcurrentCommitAfterLocks(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL guarded DOWN race requires testcontainers")
	}
	for _, table := range r23OwnedTables {
		t.Run(table, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			pool := r23StartSubscriptionPostgres(t, ctx)
			before := r23SchemaSnapshot(t, ctx, pool)

			writer, err := pool.Acquire(ctx)
			require.NoError(t, err)
			defer writer.Release()
			tx, err := writer.Begin(ctx)
			require.NoError(t, err)
			require.NoError(t, r23SeedEvidenceTx(ctx, tx, table))

			downConn, err := pool.Acquire(ctx)
			require.NoError(t, err)
			defer downConn.Release()
			var downPID int
			require.NoError(t, downConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&downPID))
			downSQL := r23ReadMigration(t, r23MigrationDown)
			result := make(chan error, 1)
			go func() {
				_, execErr := downConn.Exec(ctx, downSQL)
				result <- execErr
			}()
			r23WaitForLock(t, ctx, pool, downPID)
			require.NoError(t, tx.Commit(ctx))
			downErr := <-result
			r23RequireSQLState(t, downErr, "55000")
			_, err = downConn.Exec(ctx, "ROLLBACK")
			require.NoError(t, err)
			require.Equal(t, before, r23SchemaSnapshot(t, ctx, pool), "DOWN must lock then recheck each evidence table")
			var count int64
			require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count))
			require.Equal(t, int64(1), count)
		})
	}
}

func TestR23SubscriptionMigrationEmptyDownRoundTripsWithoutCollateralDamage(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL migration round-trip requires testcontainers")
	}
	ctx := context.Background()
	pool := r23StartSubscriptionPostgres(t, ctx)
	_, err := pool.Exec(ctx, `CREATE TABLE r23_subscription_sentinel(id integer PRIMARY KEY,payload text NOT NULL)`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO r23_subscription_sentinel VALUES(1,'preserve')`)
	require.NoError(t, err)

	downSQL := strings.ToLower(strings.Join(strings.Fields(r23ReadMigration(t, r23MigrationDown)), " "))
	require.True(t, strings.HasPrefix(downSQL, "begin;"))
	require.True(t, strings.HasSuffix(downSQL, "commit;"))
	require.NotContains(t, downSQL, "cascade")
	for _, table := range r23OwnedTables {
		require.Contains(t, downSQL, "lock table "+table+" in access exclusive mode")
		require.Contains(t, downSQL, "exists (select 1 from "+table+")")
	}
	require.Contains(t, downSQL, "errcode = '55000'")
	_, err = pool.Exec(ctx, r23ReadMigration(t, r23MigrationDown))
	require.NoError(t, err)
	require.Empty(t, r23ExistingOwnedTables(t, ctx, pool))
	var payload string
	require.NoError(t, pool.QueryRow(ctx, `SELECT payload FROM r23_subscription_sentinel WHERE id=1`).Scan(&payload))
	require.Equal(t, "preserve", payload)
	var baseTables int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('subscriptions','space_subscriptions','billing_events')`).Scan(&baseTables))
	require.Equal(t, int64(3), baseTables)
}

func r23StartSubscriptionPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pool := integrationtest.StartPostgres(t, ctx, "r23subscription", "")
	for _, name := range []string{r23MigrationBase, r23MigrationReminder, r23MigrationUp} {
		_, err := pool.Exec(ctx, r23ReadMigration(t, name))
		require.NoError(t, err, "apply %s", name)
	}
	return pool
}

func r23ReadMigration(t *testing.T, name string) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", "..", "migrations", "subscription_db", name))
	contents, err := os.ReadFile(path)
	require.NoError(t, err, "required production migration root %s is absent", path)
	return string(contents)
}

func r23ProviderEventDigest(key []byte, provider string, eventID []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(r23ProviderHMACDomain))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(strings.ToLower(provider)))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write(eventID)
	return mac.Sum(nil)
}

func r23RecordProviderEvent(t *testing.T, ctx context.Context, pool *pgxpool.Pool, recorder r23ProviderEventRecorder, provider string, eventID []byte, outcome, keyVersion string, key []byte) (bool, error) {
	t.Helper()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	replayed, recordErr := recorder.RecordProviderEventTx(ctx, tx, provider, eventID, outcome, keyVersion, key)
	if recordErr != nil {
		require.NoError(t, tx.Rollback(ctx))
		return false, recordErr
	}
	require.NoError(t, tx.Commit(ctx))
	return replayed, nil
}

type r23RecordResult struct {
	replayed bool
	err      error
}

func r23RecordProviderEventConcurrent(ctx context.Context, pool *pgxpool.Pool, recorder r23ProviderEventRecorder, provider string, eventID []byte, outcome, keyVersion string, key []byte) (bool, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	replayed, err := recorder.RecordProviderEventTx(ctx, tx, provider, eventID, outcome, keyVersion, key)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return replayed, nil
}

func r23ProviderFenceCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, provider string, key, eventID []byte) int64 {
	t.Helper()
	var count int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM subscription_provider_event_fences WHERE provider=$1 AND provider_event_hmac=$2`, provider, r23ProviderEventDigest(key, provider, eventID)).Scan(&count))
	return count
}

func r23ExistingOwnedTables(t *testing.T, ctx context.Context, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_name=ANY($1) ORDER BY table_name`, r23OwnedTables)
	require.NoError(t, err)
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var table string
		require.NoError(t, rows.Scan(&table))
		tables = append(tables, table)
	}
	require.NoError(t, rows.Err())
	return tables
}

func r23Columns(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) []string {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT column_name FROM information_schema.columns WHERE table_schema='public' AND table_name=$1 ORDER BY column_name`, table)
	require.NoError(t, err)
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var column string
		require.NoError(t, rows.Scan(&column))
		columns = append(columns, column)
	}
	require.NoError(t, rows.Err())
	return columns
}

func r23ColumnMetadata(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) []string {
	t.Helper()
	rows, err := pool.Query(ctx, `
SELECT column_name,udt_name,CASE is_nullable WHEN 'NO' THEN 'NOT NULL' ELSE 'NULL' END
FROM information_schema.columns
WHERE table_schema='public' AND table_name=$1
ORDER BY column_name`, table)
	require.NoError(t, err)
	defer rows.Close()
	var metadata []string
	for rows.Next() {
		var name, dataType, nullable string
		require.NoError(t, rows.Scan(&name, &dataType, &nullable))
		metadata = append(metadata, name+"|"+dataType+"|"+nullable)
	}
	require.NoError(t, rows.Err())
	return metadata
}

func r23PrimaryKeyColumns(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) []string {
	t.Helper()
	rows, err := pool.Query(ctx, `
SELECT attribute.attname
FROM pg_constraint constraint_row
JOIN pg_class relation ON relation.oid=constraint_row.conrelid
JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
JOIN unnest(constraint_row.conkey) WITH ORDINALITY key_column(attnum,position) ON true
JOIN pg_attribute attribute ON attribute.attrelid=relation.oid AND attribute.attnum=key_column.attnum
WHERE namespace.nspname='public' AND relation.relname=$1 AND constraint_row.contype='p'
ORDER BY key_column.position`, table)
	require.NoError(t, err)
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var column string
		require.NoError(t, rows.Scan(&column))
		columns = append(columns, column)
	}
	require.NoError(t, rows.Err())
	return columns
}

func r23SingleColumnUniqueKeys(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) []string {
	t.Helper()
	rows, err := pool.Query(ctx, `
SELECT attribute.attname
FROM pg_constraint constraint_row
JOIN pg_class relation ON relation.oid=constraint_row.conrelid
JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
JOIN pg_attribute attribute ON attribute.attrelid=relation.oid AND attribute.attnum=constraint_row.conkey[1]
WHERE namespace.nspname='public' AND relation.relname=$1 AND constraint_row.contype='u'
  AND cardinality(constraint_row.conkey)=1
ORDER BY attribute.attname`, table)
	require.NoError(t, err)
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var column string
		require.NoError(t, rows.Scan(&column))
		columns = append(columns, column)
	}
	require.NoError(t, rows.Err())
	return columns
}

func r23ForeignKeyCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var count int
	require.NoError(t, pool.QueryRow(ctx, `
SELECT count(*)
FROM pg_constraint constraint_row
JOIN pg_class relation ON relation.oid=constraint_row.conrelid
JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
WHERE namespace.nspname='public' AND relation.relname=$1 AND constraint_row.contype='f'`, table).Scan(&count))
	return count
}

func r23AssertEveryColumnImmutable(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table, keyColumn string, keyValue any) {
	t.Helper()
	quotedTable := pgx.Identifier{table}.Sanitize()
	quotedKey := pgx.Identifier{keyColumn}.Sanitize()
	for _, column := range r23Columns(t, ctx, pool, table) {
		t.Run(table+"/immutable/"+column, func(t *testing.T) {
			quotedColumn := pgx.Identifier{column}.Sanitize()
			_, err := pool.Exec(ctx, "UPDATE "+quotedTable+" SET "+quotedColumn+"="+quotedColumn+" WHERE "+quotedKey+"=$1", keyValue)
			r23RequireSQLState(t, err, "55000")
		})
	}
	_, err := pool.Exec(ctx, "DELETE FROM "+quotedTable+" WHERE "+quotedKey+"=$1", keyValue)
	r23RequireSQLState(t, err, "55000")
}

func r23InsertLifecycleFence(ctx context.Context, pool *pgxpool.Pool, spaceID, deletionID uuid.UUID, generation int64, state string) error {
	_, err := pool.Exec(ctx, `
INSERT INTO subscription_space_lifecycle_fences(
  space_id,deletion_operation_id,generation,state,manifest_id,manifest_sha256,manifest_item_count,
  request_sha256,receipt_id,applied_at
) VALUES($1,$2,$3,$4,$2::uuid::text,$5,0,$6,$7,now())`, spaceID, deletionID, generation, state, bytesOf(32, 1), bytesOf(32, 2), uuid.New())
	return err
}

func r23InsertLifecycleOperation(ctx context.Context, pool *pgxpool.Pool, spaceID, deletionID uuid.UUID, generation int64, completedAt, retainUntil time.Time) error {
	_, err := pool.Exec(ctx, `
INSERT INTO subscription_space_lifecycle_operations(
  space_id,deletion_operation_id,generation,operation_kind,request_sha256,request_bytes,receipt_id,
  receipt_bytes,terminal_state,completed_at,retain_until,provider_cancel_state,provider_cancel_attempts,
  provider_cancel_idempotency_key
) VALUES($1,$2,$3,'PURGE',$4,$5,$6,$7,'COMPLETED',$8,$9,'COMPLETED',1,$10)`,
		spaceID, deletionID, generation, bytesOf(32, 3), []byte("request"), uuid.New(), []byte("receipt"), completedAt, retainUntil, "purge:"+deletionID.String())
	return err
}

func r23InsertProviderFence(ctx context.Context, pool *pgxpool.Pool, provider string, digest []byte, outcome, keyVersion string) error {
	_, err := pool.Exec(ctx, `
INSERT INTO subscription_provider_event_fences(provider,provider_event_hmac,first_seen_at,terminal_outcome_class,key_version,retain_until)
VALUES($1,$2,now(),$3,$4,'infinity')`, provider, digest, outcome, keyVersion)
	return err
}

func r23SeedOneEvidenceRow(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) {
	t.Helper()
	switch table {
	case "subscription_provider_event_fences":
		require.NoError(t, r23InsertProviderFence(ctx, pool, "paddle", bytesOf(32, 4), "accepted", "v1"))
	case "subscription_space_lifecycle_operations":
		require.NoError(t, r23InsertLifecycleOperation(ctx, pool, uuid.New(), uuid.New(), 1, time.Now().UTC(), time.Now().UTC().Add(30*24*time.Hour)))
	case "subscription_space_lifecycle_fences":
		require.NoError(t, r23InsertLifecycleFence(ctx, pool, uuid.New(), uuid.New(), 1, "FROZEN"))
	default:
		t.Fatalf("unknown table %q", table)
	}
}

func r23SeedEvidenceTx(ctx context.Context, tx pgx.Tx, table string) error {
	// Each race inserts exactly one evidence class. Replica mode suppresses only
	// cross-table FK/immutability triggers so the DOWN lock for that class is the
	// sole synchronization point under test; CHECK constraints still execute.
	if _, err := tx.Exec(ctx, `SET LOCAL session_replication_role = replica`); err != nil {
		return err
	}
	switch table {
	case "subscription_provider_event_fences":
		_, err := tx.Exec(ctx, `INSERT INTO subscription_provider_event_fences(provider,provider_event_hmac,first_seen_at,terminal_outcome_class,key_version,retain_until) VALUES('paddle',$1,now(),'accepted','v1','infinity')`, bytesOf(32, 7))
		return err
	case "subscription_space_lifecycle_operations":
		completed := time.Now().UTC()
		_, err := tx.Exec(ctx, `
INSERT INTO subscription_space_lifecycle_operations(
 space_id,deletion_operation_id,generation,operation_kind,request_sha256,request_bytes,receipt_id,
 receipt_bytes,terminal_state,completed_at,retain_until,provider_cancel_state,provider_cancel_attempts,
 provider_cancel_idempotency_key)
VALUES($1,$2,1,'PURGE',$3,'request',$4,'receipt','COMPLETED',$5::timestamptz,$5::timestamptz+interval '30 days','COMPLETED',1,'race-key')`, uuid.New(), uuid.New(), bytesOf(32, 3), uuid.New(), completed)
		return err
	case "subscription_space_lifecycle_fences":
		_, err := tx.Exec(ctx, `
INSERT INTO subscription_space_lifecycle_fences(
 space_id,deletion_operation_id,generation,state,manifest_id,manifest_sha256,manifest_item_count,
 request_sha256,receipt_id,applied_at)
VALUES($1,$2,1,'FROZEN',$2::uuid::text,$3,0,$4,$5,now())`, uuid.New(), uuid.New(), bytesOf(32, 1), bytesOf(32, 2), uuid.New())
		return err
	default:
		return errors.New("unknown R23 evidence table: " + table)
	}
}

func r23OperationBytesCleared(t *testing.T, ctx context.Context, pool *pgxpool.Pool, spaceID uuid.UUID) bool {
	t.Helper()
	var cleared bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT request_bytes IS NULL AND receipt_bytes IS NULL FROM subscription_space_lifecycle_operations WHERE space_id=$1`, spaceID).Scan(&cleared))
	return cleared
}

func r23FenceState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, spaceID uuid.UUID) string {
	t.Helper()
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM subscription_space_lifecycle_fences WHERE space_id=$1`, spaceID).Scan(&state))
	return state
}

func r23SchemaSnapshot(t *testing.T, ctx context.Context, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(ctx, `
SELECT object_kind,object_name,definition FROM (
  SELECT 'table'::text object_kind,c.relname object_name,
    array_to_string(ARRAY(SELECT a.attname||':'||format_type(a.atttypid,a.atttypmod)||':'||a.attnotnull::text FROM pg_attribute a WHERE a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped ORDER BY a.attnum),',') definition
  FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
  WHERE n.nspname='public' AND c.relkind='r' AND c.relname=ANY($1)
  UNION ALL
  SELECT 'constraint',r.relname||'.'||c.conname,pg_get_constraintdef(c.oid)
  FROM pg_constraint c JOIN pg_class r ON r.oid=c.conrelid JOIN pg_namespace n ON n.oid=r.relnamespace
  WHERE n.nspname='public' AND r.relname=ANY($1)
  UNION ALL
  SELECT 'index',tablename||'.'||indexname,indexdef FROM pg_indexes WHERE schemaname='public' AND tablename=ANY($1)
  UNION ALL
  SELECT 'trigger',r.relname||'.'||g.tgname,pg_get_triggerdef(g.oid)
  FROM pg_trigger g JOIN pg_class r ON r.oid=g.tgrelid JOIN pg_namespace n ON n.oid=r.relnamespace
  WHERE n.nspname='public' AND r.relname=ANY($1) AND NOT g.tgisinternal
) objects ORDER BY object_kind,object_name,definition`, r23OwnedTables)
	require.NoError(t, err)
	defer rows.Close()
	var result []string
	for rows.Next() {
		var kind, name, definition string
		require.NoError(t, rows.Scan(&kind, &name, &definition))
		result = append(result, kind+"|"+name+"|"+definition)
	}
	require.NoError(t, rows.Err())
	sort.Strings(result)
	return result
}

func r23RequireSQLState(t *testing.T, err error, state string) {
	t.Helper()
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "expected PostgreSQL error, got %T: %v", err, err)
	require.Equal(t, state, pgErr.Code)
}

func r23WaitForLock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, pid int) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		err := pool.QueryRow(ctx, `SELECT COALESCE(wait_event_type='Lock',false) FROM pg_stat_activity WHERE pid=$1`, pid).Scan(&waiting)
		require.NoError(t, err)
		if waiting {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("DOWN did not reach the evidence-table lock")
		case <-ticker.C:
		}
	}
}

func bytesOf(length int, value byte) []byte {
	result := make([]byte, length)
	for i := range result {
		result[i] = value
	}
	return result
}
