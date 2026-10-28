package grpcsvc_test

import (
	"context"
	"net"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/bot/internal/dispatch"
	grpcsvc "voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/store"
)

func sessionReplica(t *testing.T, original *grpcsvc.BotGRPC) (botv1.BotServiceClient, *grpcsvc.BotGRPC) {
	t.Helper()
	pool, err := pgxpool.NewWithConfig(context.Background(), original.Store.Pool.Config().Copy())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	replica := *original
	replica.Store = &store.BotStore{Pool: pool}
	replica.Hub = dispatch.NewHub()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	botv1.RegisterBotServiceServer(server, &replica)
	go func() { _ = server.Serve(listener) }()
	conn, err := grpc.NewClient("passthrough:///session-replica", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop(); _ = conn.Close(); _ = listener.Close() })
	return botv1.NewBotServiceClient(conn), &replica
}

type sessionBarrierMessaging struct {
	messagingv1.UnimplementedMessagingServiceServer
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func (m *sessionBarrierMessaging) SendMessage(ctx context.Context, req *messagingv1.SendMessageRequest) (*messagingv1.SendMessageResponse, error) {
	if m.calls.Add(1) == 1 {
		close(m.entered)
	}
	select {
	case <-m.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &messagingv1.SendMessageResponse{Message: &messagingv1.Message{Id: uuid.NewString(), Content: req.GetContent()}}, nil
}

func TestDeferredReplicaCompletionSerializesWithRotation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL and protected Messaging")
	}
	for _, secret := range []bool{false, true} {
		name := "token"
		if secret {
			name = "webhook_secret"
		}
		t.Run(name, func(t *testing.T) {
			msg := &sessionBarrierMessaging{entered: make(chan struct{}), release: make(chan struct{})}
			var svc *grpcsvc.BotGRPC
			client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{msg: msg, service: &svc})
			defer cleanup()
			ctx, botID, credential, chat, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
			other, replica := sessionReplica(t, svc)
			bot := uuid.MustParse(botID)
			token := uuid.NewString()
			_, err := st.EnqueueEvent(ctx, freshSession(t, st, bot), "interaction", map[string]any{"chat_id": chat.String(), "chat_type": "CHAT_TYPE_CHANNEL", "invoker_profile_id": uuid.NewString()}, token)
			require.NoError(t, err)
			require.NoError(t, st.MarkEventDeferred(ctx, bot, token))
			session := sessionForToken(t, st, token)
			hub.RegisterDeferred(token, session)
			replica.Hub.RegisterDeferred(token, session)
			call, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			first := make(chan error, 1)
			go func() {
				_, e := client.CompleteInteraction(withBotToken(call, credential), &botv1.CompleteInteractionRequest{InteractionToken: token, Content: "one durable followup"})
				first <- e
			}()
			select {
			case <-msg.entered:
			case <-call.Done():
				t.Fatal(call.Err())
			}
			second := make(chan error, 1)
			go func() {
				_, e := other.CompleteInteraction(withBotToken(call, credential), &botv1.CompleteInteractionRequest{InteractionToken: token, Content: "duplicate"})
				second <- e
			}()
			rotation := make(chan error, 1)
			owner := uuid.MustParse(mustOwner(t, st, bot))
			go func() { _, _, e := replica.Store.RotateCredential(call, bot, owner, secret); rotation <- e }()
			waiting := false
			for !waiting && call.Err() == nil {
				require.NoError(t, st.Pool.QueryRow(call, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT owner_account_id,status,session_epoch FROM bots%')`).Scan(&waiting))
				runtime.Gosched()
			}
			require.True(t, waiting)
			select {
			case e := <-rotation:
				t.Fatalf("rotation escaped active Messaging guard: %v", e)
			default:
			}
			close(msg.release)
			require.NoError(t, <-first)
			require.Error(t, <-second)
			require.NoError(t, <-rotation)
			require.EqualValues(t, 1, msg.calls.Load())
			// The other Hub still remembers the old capability. The DB must deny it.
			_, err = other.CompleteInteraction(withBotToken(call, credential), &botv1.CompleteInteractionRequest{InteractionToken: token, Content: "late"})
			require.Error(t, err)
			require.EqualValues(t, 1, msg.calls.Load())
			var completed bool
			require.NoError(t, st.Pool.QueryRow(call, `SELECT interaction_completed_at IS NOT NULL FROM bot_event_log WHERE interaction_token=$1`, token).Scan(&completed))
			require.True(t, completed)
		})
	}
}

func mustOwner(t *testing.T, st *store.BotStore, bot uuid.UUID) string {
	t.Helper()
	row, err := st.GetBotByID(context.Background(), bot)
	require.NoError(t, err)
	return row.OwnerAccountID.String()
}

func TestAutocompleteDurableReplicaCacheCannotCrossEpoch(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	for _, secret := range []bool{false, true} {
		name := "token"
		if secret {
			name = "webhook_secret"
		}
		t.Run(name, func(t *testing.T) {
			var svc *grpcsvc.BotGRPC
			client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{service: &svc})
			defer cleanup()
			ctx, botID, credential, chat, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
			_, err := client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{BotId: botID, ManifestYaml: "name: BotC\nscopes: [TEXT_CHAT_SEND_MESSAGES]\ncommands:\n  - name: stats\n    description: Stats\n    options:\n      - name: game\n        type: string\n        autocomplete: true\n"})
			require.NoError(t, err)
			other, _ := sessionReplica(t, svc)
			kind := chatv1.ChatType_CHAT_TYPE_CHANNEL
			req := &botv1.AutocompleteSlashOptionRequest{BotId: botID, Chat: &chatv1.ChatRef{Id: chat.String(), Type: &kind}, CommandName: "stats", OptionName: "game", FocusedValue: "cs"}
			pending, err := client.AutocompleteSlashOption(ctx, req)
			require.NoError(t, err)
			require.True(t, pending.GetPending())
			var requestID string
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT interaction_token FROM bot_event_log WHERE bot_id=$1 AND event_type='autocomplete'`, uuid.MustParse(botID)).Scan(&requestID))
			_, err = uuid.Parse(requestID)
			require.NoError(t, err)
			foreign, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{Name: "Foreign", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`})
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `UPDATE bots SET is_polling_mode=true WHERE id=$1`, uuid.MustParse(foreign.GetBot().GetId()))
			require.NoError(t, err)
			complete := &botv1.CompleteAutocompleteRequest{RequestId: requestID, Choices: []*botv1.AutocompleteChoice{{Name: "CS2", Value: "cs2"}}}
			_, err = other.CompleteAutocomplete(withBotToken(context.Background(), foreign.GetTokenResponse().GetToken()), complete)
			require.Equal(t, codes.NotFound, status.Code(err))
			_, err = other.CompleteAutocomplete(withBotToken(context.Background(), credential), complete)
			require.NoError(t, err)
			_, err = client.CompleteAutocomplete(withBotToken(context.Background(), credential), complete)
			require.Equal(t, codes.NotFound, status.Code(err))
			cached, err := client.AutocompleteSlashOption(ctx, req)
			require.NoError(t, err)
			require.Len(t, cached.GetChoices(), 1)
			foreignInvoker := withAccount(context.Background(), uuid.New(), uuid.New())
			uncached, err := other.AutocompleteSlashOption(foreignInvoker, req)
			require.NoError(t, err)
			require.True(t, uncached.GetPending())
			require.Empty(t, uncached.GetChoices())
			bot := uuid.MustParse(botID)
			fresh, _, err := st.RotateCredential(ctx, bot, uuid.MustParse(mustOwner(t, st, bot)), secret)
			require.NoError(t, err)
			if secret {
				fresh = credential
			}
			_, err = other.CompleteAutocomplete(withBotToken(context.Background(), fresh), complete)
			require.Equal(t, codes.NotFound, status.Code(err))
			_, err = client.CompleteAutocomplete(withBotToken(context.Background(), credential), complete)
			require.Error(t, err)
			newPending, err := other.AutocompleteSlashOption(ctx, req)
			require.NoError(t, err)
			require.True(t, newPending.GetPending())
			require.Empty(t, newPending.GetChoices())
			var freshID string
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT interaction_token FROM bot_event_log WHERE bot_id=$1 AND event_type='autocomplete' AND session_epoch=2`, bot).Scan(&freshID))
			require.NotEqual(t, requestID, freshID)
		})
	}
}

type sessionMembershipBarrier struct {
	allowMembershipClient
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func (m *sessionMembershipBarrier) ListMembers(ctx context.Context, req *chatv1.ListMembersRequest, opts ...grpc.CallOption) (*chatv1.ListMembersResponse, error) {
	if m.calls.Add(1) == 2 {
		close(m.entered)
		select {
		case <-m.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return m.allowMembershipClient.ListMembers(ctx, req, opts...)
}

func TestWaitingSlashReplyCannotSurviveRemoteRotation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	for _, queued := range []bool{false, true} {
		name := "timeout"
		if queued {
			name = "queued_reply"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("BOT_WEBHOOK_TIMEOUT", "300ms")
			if queued {
				t.Setenv("BOT_WEBHOOK_TIMEOUT", "3s")
			}
			var svc *grpcsvc.BotGRPC
			msg := &sessionBarrierMessaging{entered: make(chan struct{}), release: make(chan struct{})}
			client, st, hub, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{service: &svc, msg: msg})
			defer cleanup()
			ctx, botID, _, chat, _ := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
			barrier := &sessionMembershipBarrier{entered: make(chan struct{}), release: make(chan struct{})}
			svc.Chat = barrier
			bot := uuid.MustParse(botID)
			owner := uuid.MustParse(mustOwner(t, st, bot))
			call, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			result := make(chan error, 1)
			kind := chatv1.ChatType_CHAT_TYPE_CHANNEL
			go func() {
				_, e := client.ExecuteSlashInteraction(call, &botv1.ExecuteSlashInteractionRequest{BotId: botID, Chat: &chatv1.ChatRef{Id: chat.String(), Type: &kind}, CommandName: "ping"})
				result <- e
			}()
			var token string
			for token == "" && call.Err() == nil {
				_ = st.Pool.QueryRow(call, `SELECT interaction_token FROM bot_event_log WHERE bot_id=$1 AND event_type='interaction'`, bot).Scan(&token)
				runtime.Gosched()
			}
			require.NotEmpty(t, token)
			if queued {
				require.True(t, hub.Complete(token, sessionForToken(t, st, token), store.InteractionReply{Content: "queued before rotation"}))
				select {
				case <-barrier.entered:
				case <-call.Done():
					t.Fatal(call.Err())
				}
			}
			// Simulate another replica's commit without notifying this local Hub.
			_, _, err := st.RotateCredential(call, bot, owner, true)
			require.NoError(t, err)
			if queued {
				close(barrier.release)
			}
			require.Equal(t, codes.NotFound, status.Code(<-result))
			require.Zero(t, msg.calls.Load(), "queued old result must not reach final Messaging")
		})
	}
}
