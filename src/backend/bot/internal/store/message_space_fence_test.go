package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/bot/internal/store"
)

func messageSpaceFixture(t *testing.T) (*store.BotStore, store.BotRow, uuid.UUID, *commonv1.SpaceLifecycleFenceRequest) {
	t.Helper()
	t.Setenv("BOT_ENABLE_DEV_POLLING", "true")
	st := startBotLifecycleStore(t)
	ctx := context.Background()
	profile, space, chat, operation := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Fenced Reader", "", `["TEXT_CHAT_READ_HISTORY"]`, profile)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET is_polling_mode=true WHERE id=$1`, bot.ID)
	require.NoError(t, err)
	_, err = st.InstallInSpace(ctx, bot.ID, space, profile, []uuid.UUID{chat})
	require.NoError(t, err)
	fence := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: 1,
		DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN,
		Manifest:     &commonv1.ManifestBinding{ManifestId: operation.String(), ManifestSha256: make([]byte, 32), ItemCount: 1}}
	return st, bot, chat, fence
}

func TestSpaceMessageNewAdmissionAndStaleWorkerDeniedAfterFence(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	st, bot, chat, fence := messageSpaceFixture(t)
	_, err := st.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	message := uuid.New()
	require.NoError(t, st.QueueMessageRecipients(ctx, chat, message, uuid.NewString(), map[string]any{"chat_id": chat.String()}))
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_message_deliveries WHERE message_id=$1`, message).Scan(&count))
	require.Zero(t, count, "a retained enabled whitelist cannot admit new work after receipt")
	// A stale worker/recovery row must be rejected independently of producer admission.
	delivery := uuid.New()
	_, err = st.Pool.Exec(ctx, `INSERT INTO bot_message_deliveries(id,bot_id,message_id,chat_id,payload) VALUES($1,$2,$3,$4,'{"content":"private"}')`, delivery, bot.ID, uuid.New(), chat)
	require.NoError(t, err)
	d, err := st.ClaimDueMessageDelivery(ctx)
	require.NoError(t, err)
	require.NotNil(t, d)
	called := false
	require.NoError(t, st.DeliverMessage(ctx, d, func(string, string) error { called = true; return nil }))
	require.False(t, called)
	var state, payload string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT status,payload::text FROM bot_message_deliveries WHERE id=$1`, delivery).Scan(&state, &payload))
	require.Equal(t, "canceled", state)
	require.Equal(t, "{}", payload)
	require.NoError(t, st.FailMessageDelivery(ctx, delivery))
	require.NoError(t, st.RetryMessageDelivery(ctx, delivery, 1))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT status FROM bot_message_deliveries WHERE id=$1`, delivery).Scan(&state))
	require.Equal(t, "canceled", state)
}

func TestSpaceMessagePollingEventCarriesSpaceAndFreezeScrubsIt(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	st, bot, chat, fence := messageSpaceFixture(t)
	require.NoError(t, st.QueueMessageRecipients(ctx, chat, uuid.New(), uuid.NewString(), map[string]any{"chat_id": chat.String(), "content": "private"}))
	d, err := st.ClaimDueMessageDelivery(ctx)
	require.NoError(t, err)
	require.NotNil(t, d)
	require.NoError(t, st.DeliverMessage(ctx, d, func(string, string) error { t.Fatal("polling must not POST"); return nil }))
	var space string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT space_id::text FROM bot_event_log WHERE bot_id=$1`, bot.ID).Scan(&space))
	require.Equal(t, fence.SpaceId, space)
	_, err = st.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	ids, _, _, err := st.ListPendingEvents(ctx, bot.ID, 25)
	require.NoError(t, err)
	require.Empty(t, ids)
}

func waitSpaceContender(t *testing.T, st *store.BotStore, count int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for ctx.Err() == nil {
		var waiting int
		err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event='advisory'`).Scan(&waiting)
		require.NoError(t, err)
		if waiting >= count {
			return
		}
		select {
		case <-ctx.Done():
		case <-time.After(time.Millisecond):
		}
	}
	t.Fatal("contender never reached the advisory lock")
}

func TestSpaceMessageWebhookAttemptDelaysFenceReceipt(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	st, bot, chat, fence := messageSpaceFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := st.Pool.Exec(ctx, `UPDATE bots SET is_polling_mode=false,webhook_url='https://bot.invalid/hook' WHERE id=$1`, bot.ID)
	require.NoError(t, err)
	require.NoError(t, st.QueueMessageRecipients(ctx, chat, uuid.New(), uuid.NewString(), map[string]any{"chat_id": chat.String()}))
	d, err := st.ClaimDueMessageDelivery(ctx)
	require.NoError(t, err)
	require.NotNil(t, d)
	entered, release := make(chan struct{}), make(chan struct{})
	delivered := make(chan error, 1)
	go func() {
		delivered <- st.DeliverMessage(ctx, d, func(string, string) error {
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
		t.Fatal("webhook did not start")
	}
	frozen := make(chan error, 1)
	go func() { _, err := st.ApplySpaceLifecycleFence(ctx, fence); frozen <- err }()
	waitSpaceContender(t, st, 1)
	select {
	case err := <-frozen:
		t.Fatalf("receipt returned before admitted callback completed: %v", err)
	default:
	}
	close(release)
	require.NoError(t, <-delivered)
	require.NoError(t, <-frozen)
	var count int
	require.NoError(t, st.QueueMessageRecipients(ctx, chat, uuid.New(), uuid.NewString(), map[string]any{"chat_id": chat.String()}))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_message_deliveries WHERE status='pending'`).Scan(&count))
	require.Zero(t, count)
}

func TestSpaceMessageInstallWaitsWithoutHoldingBotLock(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	st, bot, chat, fence := messageSpaceFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	space := uuid.MustParse(fence.SpaceId)
	blocker, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,71431))`, space.String())
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { _, err := st.InstallInSpace(ctx, bot.ID, space, uuid.New(), []uuid.UUID{chat}); done <- err }()
	waitSpaceContender(t, st, 1)
	probe, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	var id uuid.UUID
	err = probe.QueryRow(ctx, `SELECT id FROM bots WHERE id=$1 FOR UPDATE NOWAIT`, bot.ID).Scan(&id)
	_ = probe.Rollback(ctx)
	require.NoError(t, err, "waiting Install must not hold bot UPDATE before Space")
	require.NoError(t, blocker.Commit(ctx))
	require.NoError(t, <-done)
}

func TestSpaceMessagePollingLegacyAndStaleHintsCannotSendAfterReceipt(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	st, bot, chat, fence := messageSpaceFixture(t)
	legacy := uuid.New()
	_, err := st.Pool.Exec(ctx, `INSERT INTO bot_event_log(id,bot_id,event_type,payload,delivery_status,interaction_token) VALUES($1,$2,'message',jsonb_build_object('chat_id',$3::text,'content','private'),'pending','')`, legacy, bot.ID, chat.String())
	require.NoError(t, err)
	ids, _, _, err := st.ListPendingEvents(ctx, bot.ID, 25)
	require.NoError(t, err)
	require.Contains(t, ids, legacy)
	_, err = st.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	called := false
	require.NoError(t, st.DeliverPollingEvent(ctx, bot.ID, legacy, func(string, string) error { called = true; return nil }))
	require.False(t, called)
	var state, payload string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT delivery_status,payload::text FROM bot_event_log WHERE id=$1`, legacy).Scan(&state, &payload))
	require.Equal(t, "canceled", state)
	require.Equal(t, "{}", payload)
	// Old-version recovery can reinsert a NULL-space message after the receipt.
	stale := uuid.New()
	_, err = st.Pool.Exec(ctx, `INSERT INTO bot_event_log(id,bot_id,event_type,payload,delivery_status,interaction_token) VALUES($1,$2,'message',jsonb_build_object('chat_id',$3::text),'pending','')`, stale, bot.ID, chat.String())
	require.NoError(t, err)
	require.NoError(t, st.DeliverPollingEvent(ctx, bot.ID, stale, func(string, string) error { called = true; return nil }))
	require.False(t, called)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT delivery_status,payload::text FROM bot_event_log WHERE id=$1`, stale).Scan(&state, &payload))
	require.Equal(t, "canceled", state)
	require.Equal(t, "{}", payload)
}

func TestSpaceMessagePollingSendDelaysFenceAndRestoredFreshMessageWorks(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	st, bot, chat, fence := messageSpaceFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	queue := func() {
		require.NoError(t, st.QueueMessageRecipients(ctx, chat, uuid.New(), uuid.NewString(), map[string]any{"chat_id": chat.String()}))
		d, err := st.ClaimDueMessageDelivery(ctx)
		require.NoError(t, err)
		require.NotNil(t, d)
		require.NoError(t, st.DeliverMessage(ctx, d, func(string, string) error { t.Fatal("unexpected POST"); return nil }))
	}
	queue()
	ids, _, _, err := st.ListPendingEvents(ctx, bot.ID, 25)
	require.NoError(t, err)
	require.Len(t, ids, 1)
	entered, release := make(chan struct{}), make(chan struct{})
	sent := make(chan error, 1)
	go func() {
		sent <- st.DeliverPollingEvent(ctx, bot.ID, ids[0], func(string, string) error {
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
		t.Fatal("stream did not start")
	}
	frozen := make(chan error, 1)
	go func() { _, err := st.ApplySpaceLifecycleFence(ctx, fence); frozen <- err }()
	waitSpaceContender(t, st, 1)
	select {
	case err := <-frozen:
		t.Fatalf("receipt before stream completion: %v", err)
	default:
	}
	close(release)
	require.NoError(t, <-sent)
	require.NoError(t, <-frozen)
	fence.Generation = 2
	fence.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE
	_, err = st.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	queue()
	ids, _, _, err = st.ListPendingEvents(ctx, bot.ID, 25)
	require.NoError(t, err)
	require.Len(t, ids, 1)
	count := 0
	require.NoError(t, st.DeliverPollingEvent(ctx, bot.ID, ids[0], func(string, string) error { count++; return nil }))
	require.Equal(t, 1, count)
}
