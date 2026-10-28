package grpcsvc_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/bot/internal/dispatch"
	grpcsvc "voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/store"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
)

type allowMembershipClient struct{ chatv1.ChatServiceClient }

func (allowMembershipClient) GetChat(_ context.Context, req *chatv1.GetChatRequest, _ ...grpc.CallOption) (*chatv1.GetChatResponse, error) {
	return &chatv1.GetChatResponse{Chat: &chatv1.Chat{Id: req.GetChatId()}}, nil
}

func (allowMembershipClient) ListMembers(context.Context, *chatv1.ListMembersRequest, ...grpc.CallOption) (*chatv1.ListMembersResponse, error) {
	return &chatv1.ListMembersResponse{}, nil
}

func (allowMembershipClient) AddMembers(context.Context, *chatv1.AddMembersRequest, ...grpc.CallOption) (*chatv1.AddMembersResponse, error) {
	return &chatv1.AddMembersResponse{}, nil
}

type changingSlashMembership struct {
	allowMembershipClient
	preflight chan struct{}
	revoked   atomic.Bool
	calls     atomic.Int32
	err       error
}

func (m *changingSlashMembership) GetChat(ctx context.Context, req *chatv1.GetChatRequest, opts ...grpc.CallOption) (*chatv1.GetChatResponse, error) {
	if m.calls.Add(1) == 1 {
		close(m.preflight)
		return m.allowMembershipClient.GetChat(ctx, req, opts...)
	}
	if m.revoked.Load() {
		return nil, m.err
	}
	return m.allowMembershipClient.GetChat(ctx, req, opts...)
}

func TestSlashOutboxRechecksMembershipAfterGuardWait(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL lock barrier")
	}
	for _, tc := range []struct {
		name  string
		err   error
		state string
		code  codes.Code
	}{
		{name: "removed", err: status.Error(codes.PermissionDenied, "removed member"), state: "failed", code: codes.PermissionDenied},
		{name: "authority_unavailable", err: status.Error(codes.Unavailable, "chat unavailable"), state: "pending", code: codes.Unavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, st, cleanup := startBotGRPC(t)
			defer cleanup()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			bot, _, err := st.CreateBot(ctx, uuid.New(), "MembershipBot", "", `["TEXT_CHAT_SEND_MESSAGES"]`, uuid.New())
			require.NoError(t, err)
			chatID := uuid.New()
			_, err = installSendFixture(t, st, ctx, bot.ID, uuid.New(), uuid.New(), []uuid.UUID{chatID})
			require.NoError(t, err)
			var posts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				posts.Add(1)
				_, _ = w.Write([]byte(`{"content":"sent"}`))
			}))
			defer server.Close()
			_, err = st.Pool.Exec(ctx, `UPDATE bots SET webhook_url=$2 WHERE id=$1`, bot.ID, server.URL)
			require.NoError(t, err)
			token := uuid.NewString()
			_, err = st.EnqueueEvent(ctx, freshSession(t, st, bot.ID), "interaction", map[string]any{
				"interaction_token": token, "command_name": "ping", "chat_id": chatID.String(),
				"chat_type": "CHAT_TYPE_CHANNEL", "invoker_profile_id": uuid.NewString(),
			}, token)
			require.NoError(t, err)

			// A separate pool owns the row lock until membership changes. The
			// worker can claim and pass preflight, but cannot enter its callback.
			lock, err := st.Pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = lock.Rollback(context.Background()) }()
			var lockedBot uuid.UUID
			require.NoError(t, lock.QueryRow(ctx, `SELECT id FROM bots WHERE id=$1 FOR UPDATE`, bot.ID).Scan(&lockedBot))
			workerPool, err := pgxpool.NewWithConfig(ctx, st.Pool.Config().Copy())
			require.NoError(t, err)
			defer workerPool.Close()
			worker := grpcsvc.NewBotGRPC(&store.BotStore{Pool: workerPool}, dispatch.NewHub())
			membership := &changingSlashMembership{preflight: make(chan struct{}), err: tc.err}
			worker.Chat = membership
			type result struct {
				worked bool
				err    error
			}
			done := make(chan result, 1)
			go func() {
				worked, e := worker.ProcessNextSlashInteraction(ctx)
				done <- result{worked: worked, err: e}
			}()
			select {
			case <-membership.preflight:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			var waiting bool
			for !waiting {
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT id,owner_account_id,name%')`).Scan(&waiting))
				if waiting {
					break
				}
				select {
				case result := <-done:
					t.Fatalf("worker escaped guarded lock: %v", result.err)
				case <-time.After(10 * time.Millisecond):
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}
			membership.revoked.Store(true)
			require.NoError(t, lock.Commit(ctx))
			select {
			case result := <-done:
				require.True(t, result.worked)
				require.Equal(t, tc.code, status.Code(result.err))
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			require.EqualValues(t, 2, membership.calls.Load())
			require.Zero(t, posts.Load(), "revoked invoker must not trigger a webhook")
			var state string
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT delivery_status FROM bot_event_log WHERE interaction_token=$1`, token).Scan(&state))
			require.Equal(t, tc.state, state)
		})
	}
}

func TestSlashInteractionRejectsFailedDurableEnqueueBeforeWebhook(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{Name: "DurableBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`})
	require.NoError(t, err)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"content":"done"}`))
	}))
	defer srv.Close()
	_, err = client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{BotId: reg.GetBot().GetId(), ManifestYaml: "name: DurableBot\nwebhook_url: " + srv.URL + "\nscopes: [TEXT_CHAT_SEND_MESSAGES]\ncommands:\n  - name: play\n    description: play\n"})
	require.NoError(t, err)
	botID := uuid.MustParse(reg.GetBot().GetId())
	chatID := uuid.New()
	_, err = installSendFixture(t, st, ctx, botID, uuid.New(), uuid.New(), []uuid.UUID{chatID})
	require.NoError(t, err)
	require.NoError(t, st.TouchPresence(ctx, botID))
	_, err = st.Pool.Exec(ctx, `CREATE FUNCTION reject_slash_enqueue() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.event_type = 'interaction' THEN RAISE EXCEPTION 'enqueue unavailable'; END IF; RETURN NEW; END $$`)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `CREATE TRIGGER reject_slash_enqueue BEFORE INSERT ON bot_event_log FOR EACH ROW EXECUTE FUNCTION reject_slash_enqueue()`)
	require.NoError(t, err)
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	_, err = client.ExecuteSlashInteraction(ctx, &botv1.ExecuteSlashInteractionRequest{BotId: botID.String(), Chat: &chatv1.ChatRef{Id: chatID.String(), Type: &chatType}, CommandName: "play"})
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.EqualValues(t, 0, calls.Load())
}

func TestSlashOutboxWorkerRecoversAcceptedWebhookAfterRestart(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Setenv("BOT_WEBHOOK_DELIVERY_ATTEMPTS", "1")
	_, st, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := context.Background()
	bot, _, err := st.CreateBot(ctx, uuid.New(), "DurableBot", "", `["TEXT_CHAT_SEND_MESSAGES"]`, uuid.New())
	require.NoError(t, err)
	chatID := uuid.New()
	_, err = installSendFixture(t, st, ctx, bot.ID, uuid.New(), uuid.New(), []uuid.UUID{chatID})
	require.NoError(t, err)
	var calls atomic.Int32
	var receivedMu sync.Mutex
	var receivedTokens []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			InteractionToken string `json:"interaction_token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		receivedMu.Lock()
		receivedTokens = append(receivedTokens, payload.InteractionToken)
		receivedMu.Unlock()
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"deferred":true}`))
	}))
	defer srv.Close()
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET webhook_url = $2 WHERE id = $1`, bot.ID, srv.URL)
	require.NoError(t, err)
	token := uuid.NewString()
	_, err = st.EnqueueEvent(ctx, freshSession(t, st, bot.ID), "interaction", map[string]any{"interaction_token": token, "command_name": "play", "chat_id": chatID.String(), "chat_type": "CHAT_TYPE_CHANNEL", "invoker_profile_id": uuid.NewString()}, token)
	require.NoError(t, err)
	// A fresh service has no Hub waiter. Delivery must come from the durable row.
	svc := grpcsvc.NewBotGRPC(st, dispatch.NewHub())
	svc.Chat = allowMembershipClient{}
	worked, err := svc.ProcessNextSlashInteraction(ctx)
	require.True(t, worked)
	require.Error(t, err)
	require.EqualValues(t, 1, calls.Load())
	var deliveryStatus string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT delivery_status FROM bot_event_log WHERE bot_id = $1 AND interaction_token = $2`, bot.ID, token).Scan(&deliveryStatus))
	require.Equal(t, "pending", deliveryStatus)
	_, err = st.Pool.Exec(ctx, `UPDATE bot_event_log SET next_attempt_at = now() WHERE bot_id = $1 AND interaction_token = $2`, bot.ID, token)
	require.NoError(t, err)
	worked, err = svc.ProcessNextSlashInteraction(ctx)
	require.NoError(t, err)
	require.True(t, worked)
	require.EqualValues(t, 2, calls.Load())
	receivedMu.Lock()
	gotTokens := append([]string(nil), receivedTokens...)
	receivedMu.Unlock()
	require.Equal(t, []string{token, token}, gotTokens)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT delivery_status FROM bot_event_log WHERE bot_id = $1 AND interaction_token = $2`, bot.ID, token).Scan(&deliveryStatus))
	require.Equal(t, "deferred", deliveryStatus)
	worked, err = svc.ProcessNextSlashInteraction(ctx)
	require.NoError(t, err)
	require.False(t, worked)
}
