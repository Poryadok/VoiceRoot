package grpcsvc

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	botv1 "voice.app/voice/bot/v1"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/bot/internal/authctx"
	"voice/backend/bot/internal/dispatch"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
)

type observedNativePollStream struct {
	grpc.ServerStream
	sends  int
	second chan struct{}
	active *atomic.Bool
}

func (s *observedNativePollStream) SendMsg(v any) error {
	s.sends++
	if s.sends == 2 {
		close(s.second)
	}
	s.active.Store(true)
	defer s.active.Store(false)
	return s.ServerStream.SendMsg(v)
}

// Actual BotGRPC.PollEvents -> DeliverPollingEvent -> native Send, concurrent
// actual lifecycle receipt. This must execute without -short on Linux/Postgres;
// the no-DB companion separately proves native transport cancellation locally.
func TestPollingTransportActualHandlerStallReleasesPostgresFence(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	t.Setenv("BOT_ENABLE_DEV_POLLING", "true")
	setupCtx, setupCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer setupCancel()
	pool := integrationtest.StartPostgres(t, setupCtx, "botdb", "")
	_, file, _, _ := runtime.Caller(0)
	for _, name := range []string{"000001_init.up.sql", "000002_bot_presence.up.sql", "000003_message_delivery_outbox.up.sql", "000004_slash_interaction_outbox.up.sql", "000005_space_lifecycle.up.sql", "000007_message_token_uniqueness.up.sql", "000008_installation_intents.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "bot_db", name))
		require.NoError(t, err)
		_, err = pool.Exec(setupCtx, string(raw))
		require.NoError(t, err)
	}
	st := &store.BotStore{Pool: pool}
	profile, space, chat, operation := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	bot, token, err := st.CreateBot(setupCtx, uuid.New(), "Stalled Poll", "", `["TEXT_CHAT_READ_HISTORY"]`, profile)
	require.NoError(t, err)
	_, err = pool.Exec(setupCtx, `UPDATE bots SET is_polling_mode=true WHERE id=$1`, bot.ID)
	require.NoError(t, err)
	_, err = st.InstallInSpace(setupCtx, bot.ID, space, profile, []uuid.UUID{chat})
	require.NoError(t, err)
	for i := 0; i < 4; i++ {
		require.NoError(t, st.QueueMessageRecipients(setupCtx, chat, uuid.New(), uuid.NewString(), map[string]any{"chat_id": chat.String(), "content": strings.Repeat("x", 1024*1024)}))
		d, err := st.ClaimDueMessageDelivery(setupCtx)
		require.NoError(t, err)
		require.NotNil(t, d)
		require.NoError(t, st.DeliverMessage(setupCtx, d, func(string, string) error { t.Fatal("unexpected webhook"); return nil }))
	}
	setupCancel()
	// Container startup and fixture writes must not consume the transport proof's
	// unchanged assertion budget or its independent two-second server cap.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	second, handlerReturned := make(chan struct{}), make(chan error, 1)
	var active atomic.Bool
	srv := grpc.NewServer(grpc.InTapHandle(pollEventsTransportTap(2*time.Second)), grpc.StreamInterceptor(func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := handler(srv, &observedNativePollStream{ServerStream: ss, second: second, active: &active})
		handlerReturned <- err
		return err
	}))
	lis := bufconn.Listen(1024 * 1024)
	botv1.RegisterBotServiceServer(srv, NewBotGRPC(st, dispatch.NewHub()))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { srv.Stop(); _ = lis.Close() })
	conn, err := grpc.NewClient("passthrough:///actual-stalled-poll", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithInitialWindowSize(64*1024), grpc.WithInitialConnWindowSize(64*1024))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	// No client deadline, cancellation or Recv may release the server Send.
	clientCtx, clientCancel := context.WithCancel(metadata.NewOutgoingContext(context.Background(), metadata.Pairs(authctx.HeaderBotToken, token)))
	t.Cleanup(clientCancel)
	_, err = botv1.NewBotServiceClient(conn).PollEvents(clientCtx, &botv1.PollEventsRequest{})
	require.NoError(t, err)
	select {
	case <-second:
	case <-ctx.Done():
		t.Fatal("actual handler did not reach second native Send")
	}
	fence := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: &commonv1.ManifestBinding{ManifestId: operation.String(), ManifestSha256: make([]byte, 32), ItemCount: 1}}
	receipt := make(chan error, 1)
	go func() { _, err := st.ApplySpaceLifecycleFence(ctx, fence); receipt <- err }()
	require.Eventually(t, func() bool {
		var n int
		err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event='advisory'`).Scan(&n)
		return err == nil && n > 0
	}, time.Second, time.Millisecond, "freeze must actually contend on native-send Space guard")
	select {
	case err := <-receipt:
		t.Fatalf("receipt before native Send termination: %v", err)
	default:
	}
	select {
	case err := <-handlerReturned:
		require.Error(t, err)
	case <-ctx.Done():
		t.Fatal("actual handler did not finish after transport deadline")
	}
	require.False(t, active.Load(), "handler returned while native Send remained active")
	select {
	case err := <-receipt:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("receipt did not converge after stalled send terminated")
	}
	var pending int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_event_log WHERE bot_id=$1 AND delivery_status='pending'`, bot.ID).Scan(&pending))
	require.Zero(t, pending)
}
