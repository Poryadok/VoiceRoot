package store_test

import (
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/bot/internal/store"
)

func TestInstallationPersistsOriginalChatObservationBeforeRemoteAdd(t *testing.T) {
	if testing.Short() {
		t.Skip("real Bot PostgreSQL source persistence")
	}
	ctx := t.Context()
	st := startBotStore(t)
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Source persistence", "", `[]`, uuid.New())
	require.NoError(t, err)
	chat, space, operation := uuid.New(), uuid.New(), uuid.New()
	request := &chatv1.GetBotChatScopeAuthorityRequest{ProtocolVersion: 1, ChatId: chat.String(), ExpectedSpaceId: space.String(), OperationId: operation.String()}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	require.NoError(t, err)
	digest := sha256.Sum256(body)
	response := &chatv1.GetBotChatScopeAuthorityResponse{RequestSha256: digest[:], Scope: &chatv1.GetSpaceChatScopeAuthorityResponse{ProtocolVersion: 1, ChatId: chat.String(), ExpectedSpaceId: space.String(), ActualSpaceId: space.String(), OperationId: operation.String(), Exists: true, Live: true, ChatType: chatv1.ChatType_CHAT_TYPE_CHANNEL, ChatScopeIncarnation: uuid.NewString(), ChatAuthorityRevision: 1, LifecycleState: chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE, ManagedKind: chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY}}
	observation, err := store.NewChatScopeObservation(request, response, true)
	require.NoError(t, err)
	ctx = store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{observation})
	intent, err := st.PrepareInstallation(ctx, bot.ID, space, uuid.New(), []uuid.UUID{chat})
	require.NoError(t, err)
	var raw []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT to_jsonb(i)->'chat_scope_observation_ids' FROM bot_installation_intents i WHERE operation_id=$1`, intent.OperationID).Scan(&raw))
	require.JSONEq(t, `["`+operation.String()+`"]`, string(raw), "native intent must retain the exact accepted read before any remote Add")
}
