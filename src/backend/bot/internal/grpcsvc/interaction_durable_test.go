package grpcsvc_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/bot/internal/dispatch"
	grpcsvc "voice/backend/bot/internal/grpcsvc"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
)

type allowMembershipClient struct{ chatv1.ChatServiceClient }

func (allowMembershipClient) ListMembers(context.Context, *chatv1.ListMembersRequest, ...grpc.CallOption) (*chatv1.ListMembersResponse, error) {
	return &chatv1.ListMembersResponse{}, nil
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
	_, err = st.InstallInSpace(ctx, botID, uuid.New(), uuid.New(), []uuid.UUID{chatID})
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
	_, err = st.InstallInSpace(ctx, bot.ID, uuid.New(), uuid.New(), []uuid.UUID{chatID})
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
	_, err = st.EnqueueEvent(ctx, bot.ID, "interaction", map[string]any{"interaction_token": token, "command_name": "play", "chat_id": chatID.String(), "chat_type": "CHAT_TYPE_CHANNEL", "invoker_profile_id": uuid.NewString()}, token)
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
