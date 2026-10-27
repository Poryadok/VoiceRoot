package store_test

import (
	"context"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/bot/internal/store"
)

func rotationPollingFixture(t *testing.T) (*store.BotStore, store.BotRow, string, uuid.UUID, uuid.UUID) {
	t.Helper()
	t.Setenv("BOT_ENABLE_DEV_POLLING", "true")
	st := startBotLifecycleStore(t)
	ctx := context.Background()
	space, chat, profile := uuid.New(), uuid.New(), uuid.New()
	bot, token, err := st.CreateBot(ctx, uuid.New(), "Rotating poll", "", `["TEXT_CHAT_READ_HISTORY","TEXT_CHAT_SEND_MESSAGES"]`, profile)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET is_polling_mode=true WHERE id=$1`, bot.ID)
	require.NoError(t, err)
	_, err = st.InstallInSpace(ctx, bot.ID, space, profile, []uuid.UUID{chat})
	require.NoError(t, err)
	return st, bot, token, space, chat
}

func TestRotationRejectsListedPollingAuthorityAndPreservesMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	for _, secret := range []bool{false, true} {
		name := "token"
		if secret {
			name = "webhook_secret"
		}
		t.Run(name, func(t *testing.T) {
			st, bot, token, _, chat := rotationPollingFixture(t)
			ctx := context.Background()
			for range 2 {
				require.NoError(t, st.QueueMessageRecipients(ctx, chat, uuid.New(), uuid.NewString(), map[string]any{"chat_id": chat.String(), "content": "preserved"}))
				d, err := st.ClaimDueMessageDelivery(ctx)
				require.NoError(t, err)
				require.NotNil(t, d)
				require.NoError(t, st.DeliverMessage(ctx, d, func(context.Context, string, string) error { t.Fatal("polling must not POST"); return nil }))
			}
			var capabilities []uuid.UUID
			for _, kind := range []string{"interaction", "autocomplete"} {
				id, err := st.EnqueueEvent(ctx, bot.Session(), kind, map[string]any{"chat_id": chat.String(), "chat_type": "CHAT_TYPE_CHANNEL", "invoker_profile_id": uuid.NewString(), "request_id": uuid.NewString()}, uuid.NewString())
				require.NoError(t, err)
				capabilities = append(capabilities, id)
			}
			ids, types, _, err := st.ListPendingEvents(ctx, bot.ID, 25)
			require.NoError(t, err)
			var messages []uuid.UUID
			for i, id := range ids {
				if types[i] == "message" {
					messages = append(messages, id)
				}
			}
			require.Len(t, messages, 2)
			sends := 0
			require.NoError(t, st.DeliverPollingEvent(ctx, bot.Authority(), messages[0], func(string, string) error { sends++; return nil }))
			credential, _, err := st.RotateCredential(ctx, bot.ID, bot.OwnerAccountID, secret)
			require.NoError(t, err)
			err = st.DeliverPollingEvent(ctx, bot.Authority(), messages[1], func(string, string) error { sends++; return nil })
			require.ErrorIs(t, err, store.ErrSessionRevoked)
			require.Equal(t, 1, sends)
			if secret {
				credential = token
			}
			fresh, err := st.GetBotByTokenHash(ctx, store.HashToken(credential))
			require.NoError(t, err)
			require.Equal(t, int64(2), fresh.SessionEpoch)
			require.NoError(t, st.DeliverPollingEvent(ctx, fresh.Authority(), messages[1], func(string, string) error { sends++; return nil }))
			require.Equal(t, 2, sends)
			for _, id := range capabilities {
				require.NoError(t, st.DeliverPollingEvent(ctx, fresh.Authority(), id, func(string, string) error { t.Fatal("new session cannot consume old capability"); return nil }))
				var state, payload string
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT delivery_status,payload::text FROM bot_event_log WHERE id=$1`, id).Scan(&state, &payload))
				require.Equal(t, "canceled", state)
				require.Equal(t, "{}", payload)
			}
			var delivered int
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_message_deliveries WHERE bot_id=$1 AND status='delivered'`, bot.ID).Scan(&delivered))
			require.Equal(t, 2, delivered)
		})
	}
}

func TestGuardConnectionLossCannotMintFreshOldEpochAdmission(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	st := startBotLifecycleStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Guard loss", "", "[]", uuid.New())
	require.NoError(t, err)
	otherPool, err := pgxpool.NewWithConfig(ctx, st.Pool.Config().Copy())
	require.NoError(t, err)
	defer otherPool.Close()
	other := &store.BotStore{Pool: otherPool}
	backend := make(chan int32, 1)
	release := make(chan struct{})
	settled := make(chan error, 1)
	go func() {
		settled <- st.WithSession(ctx, bot.Authority(), uuid.Nil, func(call context.Context, tx pgx.Tx, _ *store.BotRow) error {
			var pid int32
			if e := tx.QueryRow(call, `SELECT pg_backend_pid()`).Scan(&pid); e != nil {
				return e
			}
			backend <- pid
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()
	var pid int32
	select {
	case pid = <-backend:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var terminated bool
	require.NoError(t, otherPool.QueryRow(ctx, `SELECT pg_terminate_backend($1)`, pid).Scan(&terminated))
	require.True(t, terminated, "only the fixture's own guarded backend is terminated")
	_, _, err = other.RotateCredential(ctx, bot.ID, bot.OwnerAccountID, false)
	require.NoError(t, err)
	close(release)
	require.Error(t, <-settled, "previously admitted external work has unknown effect after guard loss")
	called := false
	err = st.WithSession(ctx, bot.Authority(), uuid.Nil, func(context.Context, pgx.Tx, *store.BotRow) error { called = true; return nil })
	require.ErrorIs(t, err, store.ErrSessionRevoked)
	require.False(t, called)
}

func TestConcurrentRotationsRemainMonotonicAndDeleteDoesNotReopen(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	st := startBotLifecycleStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Monotonic", "", "[]", uuid.New())
	require.NoError(t, err)
	otherPool, err := pgxpool.NewWithConfig(ctx, st.Pool.Config().Copy())
	require.NoError(t, err)
	defer otherPool.Close()
	other := &store.BotStore{Pool: otherPool}
	epochs := make(chan int64, 8)
	failures := make(chan error, 8)
	var group sync.WaitGroup
	for i := range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			target := st
			if i%2 == 1 {
				target = other
			}
			_, epoch, e := target.RotateCredential(ctx, bot.ID, bot.OwnerAccountID, i%2 == 0)
			if e != nil {
				failures <- e
			} else {
				epochs <- epoch
			}
		}()
	}
	group.Wait()
	close(epochs)
	close(failures)
	for e := range failures {
		require.NoError(t, e)
	}
	var actual []int64
	for epoch := range epochs {
		actual = append(actual, epoch)
	}
	sort.Slice(actual, func(i, j int) bool { return actual[i] < actual[j] })
	require.Equal(t, []int64{2, 3, 4, 5, 6, 7, 8, 9}, actual)
	epoch, err := other.RevokeBot(ctx, bot.ID, bot.OwnerAccountID)
	require.NoError(t, err)
	require.Equal(t, int64(10), epoch)
	_, _, err = st.RotateCredential(ctx, bot.ID, bot.OwnerAccountID, true)
	require.ErrorIs(t, err, store.ErrRotationDenied)
	repeated, err := st.RevokeBot(ctx, bot.ID, bot.OwnerAccountID)
	require.NoError(t, err)
	require.Equal(t, epoch, repeated)
}

func TestSessionMigrationCancelsLegacyCapabilitiesWithoutLosingMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	st, bot, token, space, chat := rotationPollingFixture(t)
	ctx := context.Background()
	down, err := os.ReadFile("../../../migrations/bot_db/000009_session_epoch.down.sql")
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, string(down))
	require.NoError(t, err)
	legacy, message := uuid.New(), uuid.New()
	_, err = st.Pool.Exec(ctx, `INSERT INTO bot_event_log(id,bot_id,event_type,payload,delivery_status,interaction_token,space_id) VALUES($1,$2,'interaction','{"content":"legacy private"}','deferred',$3,$4),($5,$2,'message','{"content":"preserved"}','pending','',$4)`, legacy, bot.ID, uuid.NewString(), space, message)
	require.NoError(t, err)
	up, err := os.ReadFile("../../../migrations/bot_db/000009_session_epoch.up.sql")
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, string(up))
	require.NoError(t, err)
	var state, payload string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT delivery_status,payload::text FROM bot_event_log WHERE id=$1`, legacy).Scan(&state, &payload))
	require.Equal(t, "canceled", state)
	require.Equal(t, "{}", payload)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT delivery_status FROM bot_event_log WHERE id=$1`, message).Scan(&state))
	require.Equal(t, "pending", state)
	fresh, err := st.GetBotByTokenHash(ctx, store.HashToken(token))
	require.NoError(t, err)
	require.Equal(t, int64(1), fresh.SessionEpoch)
	unsupported := uuid.New()
	_, err = st.Pool.Exec(ctx, `INSERT INTO bot_event_log(id,bot_id,event_type,payload,space_id) VALUES($1,$2,'interaction',jsonb_build_object('chat_id',$3::text,'invoker_profile_id',$4::text),$5)`, unsupported, bot.ID, chat, uuid.New(), space)
	require.NoError(t, err)
	called := false
	require.NoError(t, st.DeliverPollingEvent(ctx, fresh.Authority(), unsupported, func(string, string) error { called = true; return nil }))
	require.False(t, called, "unbound old writer rows cannot inherit current epoch")
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT delivery_status FROM bot_event_log WHERE id=$1`, unsupported).Scan(&state))
	require.Equal(t, "canceled", state)
	deferred, err := st.ListDeferredSessions(ctx)
	require.NoError(t, err)
	require.Empty(t, deferred)
}
