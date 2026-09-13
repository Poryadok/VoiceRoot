package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestScheduledMessagesMigrationEnforcesShapeAndIdempotencyKey(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	chatID, senderID, clientID := uuid.New(), uuid.New(), uuid.New()

	_, err := pool.Exec(ctx, `INSERT INTO scheduled_messages (id, chat_id, sender_profile_id, client_message_id, payload, schedule_kind, scheduled_at) VALUES ($1,$2,$3,$4,'{}'::jsonb,'at',now())`, uuid.New(), chatID, senderID, clientID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO scheduled_messages (id, chat_id, sender_profile_id, client_message_id, payload, schedule_kind, scheduled_at) VALUES ($1,$2,$3,$4,'{}'::jsonb,'at',now())`, uuid.New(), chatID, senderID, clientID)
	require.Error(t, err, "partial client id index rejects duplicate schedule rows")

	_, err = pool.Exec(ctx, `INSERT INTO scheduled_messages (id, chat_id, sender_profile_id, payload, schedule_kind) VALUES ($1,$2,$3,'{}'::jsonb,'when_online')`, uuid.New(), chatID, senderID)
	require.NoError(t, err, "when-online is the permitted timestamp-free shape")
	for _, query := range []string{
		`INSERT INTO scheduled_messages (id, chat_id, sender_profile_id, payload, schedule_kind) VALUES (gen_random_uuid(), $1,$2,'{}'::jsonb,'at')`,
		`INSERT INTO scheduled_messages (id, chat_id, sender_profile_id, payload, schedule_kind, scheduled_at) VALUES (gen_random_uuid(), $1,$2,'{}'::jsonb,'when_online',now())`,
		`INSERT INTO scheduled_messages (id, chat_id, sender_profile_id, payload, schedule_kind, scheduled_at) VALUES (gen_random_uuid(), $1,$2,'{}'::jsonb,'later',now())`,
		`INSERT INTO scheduled_messages (id, chat_id, sender_profile_id, payload, schedule_kind, scheduled_at, status) VALUES (gen_random_uuid(), $1,$2,'{}'::jsonb,'at',now(),'dispatching')`,
		`INSERT INTO scheduled_messages (id, chat_id, sender_profile_id, payload, schedule_kind, scheduled_at, attempt_count) VALUES (gen_random_uuid(), $1,$2,'{}'::jsonb,'at',now(),-1)`,
	} {
		_, err = pool.Exec(ctx, query, chatID, senderID)
		require.Error(t, err, query)
	}
}

func TestScheduledMessagesMigrationPersistsDispatchMetadataAndIndexes(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	var gotEvent, gotSent *uuid.UUID
	var attempts int
	var lastError, owner *string
	var nextAttempt, failedAt, leaseExpiry *time.Time
	err := pool.QueryRow(ctx, `
INSERT INTO scheduled_messages (id,chat_id,sender_profile_id,payload,schedule_kind,scheduled_at,status,attempt_count,next_attempt_at,last_error_code,dispatch_lease_owner,dispatch_lease_expires_at)
VALUES ($1,$2,$3,'{}'::jsonb,'at',now(),'pending',2,now()+interval '1 minute','unavailable','worker-a',now()+interval '2 minutes')
RETURNING dispatch_event_id,sent_message_id,attempt_count,next_attempt_at,last_error_code,failed_at,dispatch_lease_owner,dispatch_lease_expires_at
`, uuid.New(), uuid.New(), uuid.New()).Scan(&gotEvent, &gotSent, &attempts, &nextAttempt, &lastError, &failedAt, &owner, &leaseExpiry)
	require.NoError(t, err)
	require.Nil(t, gotEvent)
	require.Nil(t, gotSent)
	require.Equal(t, 2, attempts)
	require.Equal(t, "unavailable", *lastError)
	require.Equal(t, "worker-a", *owner)
	require.False(t, nextAttempt.IsZero())
	require.Nil(t, failedAt)
	require.False(t, leaseExpiry.IsZero())
	var indexCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_indexes WHERE schemaname='public' AND tablename='scheduled_messages' AND indexname IN ('scheduled_messages_due_pending_idx','scheduled_messages_owner_pending_idx')`).Scan(&indexCount))
	require.Equal(t, 2, indexCount)
}
