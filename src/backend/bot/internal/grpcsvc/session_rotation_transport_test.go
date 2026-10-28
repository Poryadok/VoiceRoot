package grpcsvc

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/bot/internal/authctx"
	"voice/backend/bot/internal/dispatch"
	"voice/backend/bot/internal/store"
	"voice/backend/bot/internal/testdb"
)

type rotationObservedStream struct {
	grpc.ServerStream
	entered chan struct{}
	calls   *atomic.Int32
	active  *atomic.Bool
}

func (s *rotationObservedStream) SendMsg(message any) error {
	if s.calls.Add(1) == 2 {
		close(s.entered)
	}
	s.active.Store(true)
	defer s.active.Store(false)
	return s.ServerStream.SendMsg(message)
}

func TestProductionPollingBudgetReleasesCredentialRotation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL and production25s native HTTP2 budget")
	}
	for _, secret := range []bool{false, true} {
		name := "token"
		if secret {
			name = "webhook_secret"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("BOT_ENABLE_DEV_POLLING", "true")
			setup, cancelSetup := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancelSetup()
			pool := testdb.StartPostgres(t, setup, "rotation_transport", "")
			for _, name := range []string{"000001_init.up.sql", "000002_bot_presence.up.sql", "000003_message_delivery_outbox.up.sql", "000004_slash_interaction_outbox.up.sql", "000005_space_lifecycle.up.sql", "000007_message_token_uniqueness.up.sql", "000009_session_epoch.up.sql"} {
				raw, e := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "bot_db", name))
				require.NoError(t, e)
				_, e = pool.Exec(setup, string(raw))
				require.NoError(t, e)
			}
			st := &store.BotStore{Pool: pool}
			profile, space, chat := uuid.New(), uuid.New(), uuid.New()
			bot, token, err := st.CreateBot(setup, uuid.New(), "Rotation stream", "", `["TEXT_CHAT_READ_HISTORY"]`, profile)
			require.NoError(t, err)
			_, err = pool.Exec(setup, `UPDATE bots SET is_polling_mode=true WHERE id=$1`, bot.ID)
			require.NoError(t, err)
			_, err = st.InstallInSpace(setup, bot.ID, space, profile, []uuid.UUID{chat})
			require.NoError(t, err)
			for range 4 {
				require.NoError(t, st.QueueMessageRecipients(setup, chat, uuid.New(), uuid.NewString(), map[string]any{"chat_id": chat.String(), "content": strings.Repeat("x", 1024*1024)}))
				d, e := st.ClaimDueMessageDelivery(setup)
				require.NoError(t, e)
				require.NotNil(t, d)
				require.NoError(t, st.DeliverMessage(setup, d, func(context.Context, string, string) error { t.Fatal("polling must not POST"); return nil }))
			}
			cancelSetup()
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer cancel()
			otherPool, err := pgxpool.NewWithConfig(ctx, pool.Config().Copy())
			require.NoError(t, err)
			defer otherPool.Close()
			other := &store.BotStore{Pool: otherPool}
			second, returned := make(chan struct{}), make(chan error, 1)
			var calls atomic.Int32
			var active, observed atomic.Bool
			server := grpc.NewServer(grpc.InTapHandle(PollEventsTransportTap()), grpc.StreamInterceptor(func(service any, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
				if observed.CompareAndSwap(false, true) {
					e := handler(service, &rotationObservedStream{ServerStream: stream, entered: second, calls: &calls, active: &active})
					returned <- e
					return e
				}
				return handler(service, stream)
			}))
			listener := bufconn.Listen(1024 * 1024)
			botv1.RegisterBotServiceServer(server, NewBotGRPC(st, dispatch.NewHub()))
			go func() { _ = server.Serve(listener) }()
			t.Cleanup(func() { server.Stop(); _ = listener.Close() })
			dial := func() *grpc.ClientConn {
				conn, e := grpc.NewClient("passthrough:///rotation-stream", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithInitialWindowSize(64*1024), grpc.WithInitialConnWindowSize(64*1024))
				require.NoError(t, e)
				t.Cleanup(func() { _ = conn.Close() })
				return conn
			}
			old := dial()
			oldContext, oldCancel := context.WithCancel(metadata.NewOutgoingContext(context.Background(), metadata.Pairs(authctx.HeaderBotToken, token)))
			t.Cleanup(oldCancel)
			started := time.Now()
			_, err = botv1.NewBotServiceClient(old).PollEvents(oldContext, &botv1.PollEventsRequest{})
			require.NoError(t, err)
			// This client never calls Recv or cancels before the server deadline.
			select {
			case <-second:
			case <-ctx.Done():
				t.Fatal("second native Send was not reached")
			}
			type result struct {
				credential string
				err        error
			}
			rotation := make(chan result, 1)
			go func() {
				plain, _, e := other.RotateCredential(ctx, bot.ID, bot.OwnerAccountID, secret)
				rotation <- result{credential: plain, err: e}
			}()
			waiting := false
			for !waiting && ctx.Err() == nil {
				require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT owner_account_id,status,session_epoch FROM bots%')`).Scan(&waiting))
				runtime.Gosched()
			}
			require.True(t, waiting, "rotation must actually contend on native Send's bot guard")
			select {
			case r := <-rotation:
				t.Fatalf("rotation completed during an active Send: %v", r.err)
			default:
			}
			select {
			case e := <-returned:
				require.Error(t, e)
			case <-ctx.Done():
				t.Fatal("production deadline did not terminate native Send")
			}
			require.GreaterOrEqual(t, time.Since(started), PollEventsTransportLimit-time.Second)
			require.False(t, active.Load())
			require.EqualValues(t, 2, calls.Load(), "no later event was admitted by old stream")
			var outcome result
			select {
			case outcome = <-rotation:
				require.NoError(t, outcome.err)
			case <-ctx.Done():
				t.Fatal("rotation did not converge after Send ended")
			}
			ids, _, _, err := st.ListPendingEvents(ctx, bot.ID, 25)
			require.NoError(t, err)
			require.Len(t, ids, 3, "credential upkeep must preserve ordinary messages")
			credential := outcome.credential
			if secret {
				credential = token
			}
			freshContext, freshCancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs(authctx.HeaderBotToken, credential)), 10*time.Second)
			defer freshCancel()
			stream, err := botv1.NewBotServiceClient(dial()).PollEvents(freshContext, &botv1.PollEventsRequest{})
			require.NoError(t, err)
			received := map[string]bool{}
			for {
				event, e := stream.Recv()
				if e == io.EOF {
					break
				}
				require.NoError(t, e)
				received[event.GetBotEvent().GetEventId()] = true
			}
			require.Len(t, received, 3)
			for _, id := range ids {
				require.True(t, received[id.String()], "stable pending delivery identity must survive rotation")
			}
		})
	}
}
