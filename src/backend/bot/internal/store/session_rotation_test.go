package store_test

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/bot/internal/store"
)

// Credential maintenance revokes capabilities, not durable message recipients.
func TestCredentialRotationRevokesSessionCapabilities(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	for _, secret := range []bool{false, true} {
		name := "token"
		if secret {
			name = "webhook_secret"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			st := startBotLifecycleStore(t)
			bot, _, err := st.CreateBot(ctx, uuid.New(), "Rotating", "", "[]", uuid.New())
			require.NoError(t, err)
			other, _, err := st.CreateBot(ctx, uuid.New(), "Unrelated", "", "[]", uuid.New())
			require.NoError(t, err)
			interaction, err := st.EnqueueEvent(ctx, freshSession(t, st, bot.ID), "interaction", map[string]any{"content": "private"}, uuid.NewString())
			require.NoError(t, err)
			deferred, err := st.EnqueueEvent(ctx, freshSession(t, st, bot.ID), "interaction", map[string]any{"content": "deferred-private"}, uuid.NewString())
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `UPDATE bot_event_log SET delivery_status='deferred' WHERE id=$1`, deferred)
			require.NoError(t, err)
			message, err := st.EnqueueEvent(ctx, freshSession(t, st, bot.ID), "message", map[string]any{"content": "keep"}, "")
			require.NoError(t, err)
			unrelated, err := st.EnqueueEvent(ctx, freshSession(t, st, other.ID), "interaction", map[string]any{"content": "other"}, uuid.NewString())
			require.NoError(t, err)
			_, _, err = st.RotateCredential(ctx, bot.ID, bot.OwnerAccountID, secret)
			require.NoError(t, err)
			for _, id := range []uuid.UUID{interaction, deferred} {
				var state, payload string
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT delivery_status,payload::text FROM bot_event_log WHERE id=$1`, id).Scan(&state, &payload))
				require.Equal(t, "canceled", state)
				require.Equal(t, "{}", payload)
			}
			for _, id := range []uuid.UUID{message, unrelated} {
				var state string
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT delivery_status FROM bot_event_log WHERE id=$1`, id).Scan(&state))
				require.Equal(t, "pending", state)
			}
			current, err := st.GetBotByID(ctx, bot.ID)
			require.NoError(t, err)
			if secret {
				require.Equal(t, bot.TokenHash, current.TokenHash)
				require.NotEqual(t, bot.WebhookSecret, current.WebhookSecret)
			} else {
				require.Equal(t, bot.WebhookSecret, current.WebhookSecret)
				require.NotEqual(t, bot.TokenHash, current.TokenHash)
			}
		})
	}
}

func TestCredentialRotationIsAtomicAndOwnerBound(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	st := startBotLifecycleStore(t)
	ctx := context.Background()
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Atomic", "", "[]", uuid.New())
	require.NoError(t, err)
	event, err := st.EnqueueEvent(ctx, bot.Session(), "interaction", map[string]any{"content": "keep-on-rollback"}, uuid.NewString())
	require.NoError(t, err)
	_, _, err = st.RotateCredential(ctx, bot.ID, uuid.New(), false)
	require.ErrorIs(t, err, store.ErrRotationDenied)
	_, err = st.Pool.Exec(ctx, `CREATE FUNCTION reject_rotation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected rotation failure'; END $$;
CREATE TRIGGER reject_rotation BEFORE UPDATE ON bot_event_log FOR EACH ROW EXECUTE FUNCTION reject_rotation()`)
	require.NoError(t, err)
	_, _, err = st.RotateCredential(ctx, bot.ID, bot.OwnerAccountID, false)
	require.Error(t, err)
	current, err := st.GetBotByID(ctx, bot.ID)
	require.NoError(t, err)
	require.Equal(t, bot.TokenHash, current.TokenHash)
	require.Equal(t, bot.WebhookSecret, current.WebhookSecret)
	require.Equal(t, int64(1), current.SessionEpoch)
	var state string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT delivery_status FROM bot_event_log WHERE id=$1`, event).Scan(&state))
	require.Equal(t, "pending", state)
	_, err = st.Pool.Exec(ctx, `DROP TRIGGER reject_rotation ON bot_event_log; DROP FUNCTION reject_rotation()`)
	require.NoError(t, err)
	_, epoch, err := st.RotateCredential(ctx, bot.ID, bot.OwnerAccountID, true)
	require.NoError(t, err)
	require.Equal(t, int64(2), epoch)
}

func TestRotationSerializesActualAdmissionAcrossPools(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	for _, secret := range []bool{false, true} {
		name := "token"
		if secret {
			name = "webhook_secret"
		}
		t.Run(name, func(t *testing.T) {
			st := startBotLifecycleStore(t)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			bot, token, err := st.CreateBot(ctx, uuid.New(), "Guarded", "", "[]", uuid.New())
			require.NoError(t, err)
			otherPool, err := pgxpool.NewWithConfig(ctx, st.Pool.Config().Copy())
			require.NoError(t, err)
			defer otherPool.Close()
			other := &store.BotStore{Pool: otherPool}
			entered, release := make(chan struct{}), make(chan struct{})
			admitted := make(chan error, 1)
			go func() {
				admitted <- st.WithSession(ctx, bot.Authority(), uuid.Nil, func(context.Context, pgx.Tx, *store.BotRow) error {
					close(entered)
					select {
					case <-release:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
			}()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			rotated := make(chan error, 1)
			go func() { _, _, e := other.RotateCredential(ctx, bot.ID, bot.OwnerAccountID, secret); rotated <- e }()
			// Observe the real conflicting database wait before asserting ordering.
			waiting := false
			for !waiting && ctx.Err() == nil {
				err = st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT owner_account_id,status,session_epoch FROM bots%')`).Scan(&waiting)
				require.NoError(t, err)
				runtime.Gosched()
			}
			require.True(t, waiting)
			select {
			case e := <-rotated:
				t.Fatalf("rotation finished before native admission settled: %v", e)
			default:
			}
			close(release)
			require.NoError(t, <-admitted)
			require.NoError(t, <-rotated)
			called := false
			err = st.WithSession(ctx, bot.Authority(), uuid.Nil, func(context.Context, pgx.Tx, *store.BotRow) error { called = true; return nil })
			require.ErrorIs(t, err, store.ErrSessionRevoked)
			require.False(t, called)
			current, lookupErr := other.GetBotByTokenHash(ctx, store.HashToken(token))
			if secret {
				require.NoError(t, lookupErr)
				require.Equal(t, int64(2), current.SessionEpoch)
			} else {
				require.ErrorIs(t, lookupErr, store.ErrNotFound)
			}
		})
	}
}
