package grpcsvc

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"
)

func TestProjectGameActionResultIsRequestBoundImmutableAndReadWithCard(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applyBaseMessagingMigrations(t, ctx, pool)
	chatID, author, member, messageID, appID, envID, actionID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.NewString()
	seedGroupChat(t, ctx, pool, chatID, author, member)
	card := fmt.Sprintf(`{"schema_version":1,"revision":"3","title":"Relic","safe_summary":"A relic","actions":[{"action_id":%q,"action_type":"relic.inspect","label":"Inspect","arguments_json":"{}","state_version":"s-3","expires_at":%q}]}`, actionID, time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano))
	_, err := pool.Exec(ctx, `INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content,type,attachments,mentions,client_message_id,ghost_only,is_e2e) VALUES($1,$2,'group',$3,'Relic','regular','[]'::jsonb,'[]'::jsonb,$4,false,false)`, messageID, chatID, author, uuid.New())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO message_game_cards(message_id,app_id,environment_id,installation_id,bot_id,card_json,card_sha256,actions_enabled) VALUES($1,$2,$3,$4,$5,$6::jsonb,$7,false)`, messageID, appID, envID, uuid.New(), uuid.New(), card, fmt.Sprintf("%x", sha256.Sum256([]byte(card))))
	require.NoError(t, err)
	request := &messagingv1.ProjectGameActionResultRequest{MessageId: messageID.String(), ApplicationId: appID.String(), EnvironmentId: envID.String(), OperationId: uuid.NewString(), ActionId: actionID, ResultId: uuid.NewString(), StateVersion: "s-4", Status: "succeeded", SafeSummary: "Item collected"}
	call := func(req *messagingv1.ProjectGameActionResultRequest) (*messagingv1.ProjectGameActionResultResponse, error) {
		hash, e := principal.RequestHash(req)
		require.NoError(t, e)
		rpcCtx := principal.WithVerified(ctx, principal.Principal{Kind: "service", Issuer: "gameintegration", Subject: "service:gameintegration", Audience: "messaging", RPC: messagingv1.MessagingService_ProjectGameActionResult_FullMethodName, RequestID: req.GetOperationId(), RequestHash: hash})
		return (&MessagingGRPC{Messages: &store.MessagesStore{Pool: pool}}).ProjectGameActionResult(rpcCtx, req)
	}
	first, err := call(request)
	require.NoError(t, err)
	require.False(t, first.GetReplayed())
	require.Equal(t, request.GetResultId(), first.GetResult().GetResultId())
	replay, err := call(request)
	require.NoError(t, err)
	require.True(t, replay.GetReplayed())
	changed := proto.Clone(request).(*messagingv1.ProjectGameActionResultRequest)
	changed.SafeSummary = "different result"
	_, err = call(changed)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	foreign := proto.Clone(request).(*messagingv1.ProjectGameActionResultRequest)
	foreign.OperationId = uuid.NewString()
	foreign.ResultId = uuid.NewString()
	foreign.ApplicationId = uuid.NewString()
	_, err = call(foreign)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	row, err := (&store.MessagesStore{Pool: pool}).GetMessageByID(ctx, messageID)
	require.NoError(t, err)
	require.Len(t, row.GameActionResults, 1)
	require.Equal(t, request.GetOperationId(), row.GameActionResults[0].OperationID.String())
	projectedMessage := messageRowToProto(row, messagingv1.MessageKind_MESSAGE_KIND_UNSPECIFIED, "", false)
	require.Len(t, projectedMessage.GetGameActionResults(), 1)
	require.Equal(t, "Item collected", projectedMessage.GetGameActionResults()[0].GetSafeSummary())
	_, err = call(&messagingv1.ProjectGameActionResultRequest{MessageId: request.GetMessageId(), ApplicationId: request.GetApplicationId(), EnvironmentId: request.GetEnvironmentId(), OperationId: uuid.NewString(), ActionId: uuid.NewString(), ResultId: uuid.NewString(), StateVersion: "s-4", Status: "succeeded"})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}
