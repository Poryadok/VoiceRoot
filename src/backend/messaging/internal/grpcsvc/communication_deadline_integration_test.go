package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	messagingv1 "voice.app/voice/messaging/v1"
)

func TestCommunicationAttemptContextStatusSurvivesSendForwardPersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL handler transaction lock")
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")
	applyBaseMessagingMigrations(t, ctx, pool)
	applyModerationSchemasForMessagingTest(t, ctx, pool)
	chat, space, profile, peer, account := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	seedGroupChat(t, ctx, pool, chat, profile, peer)
	_, err := pool.Exec(ctx, `UPDATE chats SET space_id=$2 WHERE id=$1`, chat, space)
	require.NoError(t, err)
	svc := startMessagingDirect(t, pool)
	svc.RolePermissions = testRolePerms{allowed: true}
	svc.ChatRolePermissions = testRolePerms{allowed: true}
	svc.CommunicationStates = &communicationStateStub{state: 1}
	caller := incomingProfileCtx(ctx, account, profile)
	source, err := svc.SendMessage(caller, &messagingv1.SendMessageRequest{Chat: chatGroupRef(chat), Content: "source"})
	require.NoError(t, err)
	for _, action := range []string{"send", "forward"} {
		for _, cancelAttempt := range []bool{false, true} {
			name := action + "/deadline"
			want := codes.DeadlineExceeded
			if cancelAttempt {
				name = action + "/cancel"
				want = codes.Canceled
			}
			t.Run(name, func(t *testing.T) {
				lock, err := pool.Begin(ctx)
				require.NoError(t, err)
				defer func() { _ = lock.Rollback(context.Background()) }()
				_, err = lock.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "voice.messaging.message-mutation.v1/"+chat.String())
				require.NoError(t, err)
				attempt, cancel := context.WithCancel(caller)
				defer cancel()
				result := make(chan error, 1)
				go func() {
					if action == "send" {
						_, err := svc.SendMessage(attempt, &messagingv1.SendMessageRequest{Chat: chatGroupRef(chat), Content: "blocked send"})
						result <- err
					} else {
						_, err := svc.ForwardMessage(attempt, &messagingv1.ForwardMessageRequest{SourceMessageId: source.GetMessage().GetId(), TargetChat: chatGroupRef(chat), Commentary: proto.String("blocked commentary")})
						result <- err
					}
				}()
				require.Eventually(t, func() bool {
					var waiting bool
					err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event='advisory')`).Scan(&waiting)
					return err == nil && waiting
				}, time.Second, 10*time.Millisecond, "handler must reach the persistence lock before expiry/cancel")
				if cancelAttempt {
					cancel()
				}
				select {
				case err := <-result:
					require.Equal(t, want, status.Code(err))
				case <-time.After(5 * time.Second):
					t.Fatal("admitted mutation outlived its two-second budget")
				}
				var messages int
				require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&messages))
				require.Equal(t, 1, messages, "failed local attempt must not leave commentary or forward")
			})
		}
	}
}
