package store_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/bot/internal/store"
	"voice/backend/bot/internal/webhook"
)

func TestHTTPRetryAfterGuardLossCannotReauthenticateOldEpoch(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL and HTTP")
	}
	t.Setenv("BOT_WEBHOOK_DELIVERY_ATTEMPTS", "3")
	t.Setenv("BOT_WEBHOOK_RETRY_BACKOFF_MS", "1")
	st := startBotLifecycleStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Guarded HTTP", "", "[]", uuid.New())
	require.NoError(t, err)
	pool, err := pgxpool.NewWithConfig(ctx, st.Pool.Config().Copy())
	require.NoError(t, err)
	defer pool.Close()
	other := &store.BotStore{Pool: pool}
	var pid atomic.Int32
	var calls atomic.Int32
	mutated := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) != 1 {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"content":"stale retry escaped"}`))
			return
		}
		var killed bool
		e := pool.QueryRow(r.Context(), `SELECT pg_terminate_backend($1)`, pid.Load()).Scan(&killed)
		if e == nil && !killed {
			e = fmt.Errorf("owned guarded backend was not terminated")
		}
		if e == nil {
			_, _, e = other.RotateCredential(r.Context(), bot.ID, bot.OwnerAccountID, false)
		}
		mutated <- e
		// Recipient already observed the first POST. Its outcome is deliberately
		// ambiguous; a fresh retry must check the original, now lost transaction.
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	err = st.WithSession(ctx, bot.Authority(), uuid.Nil, func(call context.Context, tx pgx.Tx, current *store.BotRow) error {
		var backend int32
		if e := tx.QueryRow(call, `SELECT pg_backend_pid()`).Scan(&backend); e != nil {
			return e
		}
		pid.Store(backend)
		client := webhook.GuardedClient(server.Client(), func(request context.Context) error { return st.CheckGuardedSession(request, current.Session()) })
		_, e := webhook.DeliverPOST(call, client, server.URL, current.WebhookSecret, webhook.InteractionPayload{Type: "interaction"}, time.Second)
		return e
	})
	require.Error(t, err)
	require.NoError(t, <-mutated)
	require.EqualValues(t, 1, calls.Load(), "no second native POST may refresh authority from the pool")
	fresh, err := st.GetBotByID(ctx, bot.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), fresh.SessionEpoch)
}

func TestRotatedSlashClaimCannotPostOrReopenCapability(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL and HTTP")
	}
	for _, secret := range []bool{false, true} {
		name := "token"
		if secret {
			name = "webhook_secret"
		}
		t.Run(name, func(t *testing.T) {
			st, bot, _, _, chat := rotationPollingFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				_, _ = w.Write([]byte(`{"content":"stale"}`))
			}))
			defer server.Close()
			_, err := st.Pool.Exec(ctx, `UPDATE bots SET is_polling_mode=false,webhook_url=$2 WHERE id=$1`, bot.ID, server.URL)
			require.NoError(t, err)
			token := uuid.NewString()
			id, err := st.EnqueueEvent(ctx, bot.Session(), "interaction", map[string]any{"chat_id": chat.String(), "chat_type": "CHAT_TYPE_CHANNEL", "invoker_profile_id": uuid.NewString()}, token)
			require.NoError(t, err)
			claim, err := st.ClaimSlashInteraction(ctx, id)
			require.NoError(t, err)
			require.NotNil(t, claim)
			_, _, err = st.RotateCredential(ctx, bot.ID, bot.OwnerAccountID, secret)
			require.NoError(t, err)
			applied, err := st.DeliverSlashInteraction(ctx, claim, func(call context.Context, current *store.BotRow) (store.SlashResult, error) {
				client := webhook.GuardedClient(server.Client(), func(request context.Context) error { return st.CheckSlashAttempt(request, claim) })
				_, e := webhook.DeliverPOST(call, client, server.URL, current.WebhookSecret, webhook.InteractionPayload{Type: "interaction"}, time.Second)
				return store.SlashResult{}, e
			})
			require.ErrorIs(t, err, store.ErrSessionRevoked)
			require.False(t, applied)
			require.Zero(t, calls.Load())
			applied, err = st.RetrySlashDelivery(ctx, id, claim.Attempts)
			require.NoError(t, err)
			require.False(t, applied)
			applied, err = st.CompleteSlashDelivery(ctx, id, claim.Attempts, true)
			require.NoError(t, err)
			require.False(t, applied)
			var state, payload string
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT delivery_status,payload::text FROM bot_event_log WHERE id=$1`, id).Scan(&state, &payload))
			require.Equal(t, "canceled", state)
			require.Equal(t, "{}", payload)
		})
	}
}

func TestSlashClaimExpiryIsRecheckedAfterEventLockWait(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	st, bot, _, _, chat := rotationPollingFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err := st.Pool.Exec(ctx, `UPDATE bots SET is_polling_mode=false,webhook_url='http://unused.invalid' WHERE id=$1`, bot.ID)
	require.NoError(t, err)
	id, err := st.EnqueueEvent(ctx, bot.Session(), "interaction", map[string]any{"chat_id": chat.String(), "chat_type": "CHAT_TYPE_CHANNEL", "invoker_profile_id": uuid.NewString()}, uuid.NewString())
	require.NoError(t, err)
	claim, err := st.ClaimSlashInteraction(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, claim)
	blocker, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer blocker.Rollback(context.Background())
	var locked uuid.UUID
	require.NoError(t, blocker.QueryRow(ctx, `SELECT id FROM bot_event_log WHERE id=$1 FOR UPDATE`, id).Scan(&locked))
	type result struct {
		applied bool
		err     error
	}
	finished := make(chan result, 1)
	var calls atomic.Int32
	go func() {
		applied, e := st.DeliverSlashInteraction(ctx, claim, func(context.Context, *store.BotRow) (store.SlashResult, error) {
			calls.Add(1)
			return store.SlashResult{}, nil
		})
		finished <- result{applied, e}
	}()
	waiting := false
	for !waiting && ctx.Err() == nil {
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT payload::text,interaction_token,session_epoch,claimed_until,space_id FROM bot_event_log%')`).Scan(&waiting))
		runtime.Gosched()
	}
	require.True(t, waiting, "delivery must actually wait at the event row after upstream guards")
	_, err = blocker.Exec(ctx, `UPDATE bot_event_log SET claimed_until=clock_timestamp()-interval '1 millisecond' WHERE id=$1`, id)
	require.NoError(t, err)
	require.NoError(t, blocker.Commit(ctx))
	outcome := <-finished
	require.NoError(t, outcome.err)
	require.False(t, outcome.applied)
	require.Zero(t, calls.Load())
	applied, err := st.CompleteSlashDelivery(ctx, id, claim.Attempts, true)
	require.NoError(t, err)
	require.False(t, applied, "expired bookkeeping cannot reopen a deferred capability")
}
