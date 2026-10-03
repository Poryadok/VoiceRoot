package registry

import (
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
)

func TestBuildSessionOwnerProtoBindsDurableSessionAndReceipts(t *testing.T) {
	app, env, session, op, room, chat, chatReceipt := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	members := []uuid.UUID{uuid.New(), uuid.New()}
	request := SessionOwnerRequest{ApplicationID: app, EnvironmentID: env, SessionID: session, OperationID: op,
		Kind: "party", ExternalKey: "party-a", DisplayName: "Party A", ChatID: chat,
		ChatCreateReceiptID: chatReceipt, VoiceRoomID: room, RosterRevision: 7, Members: members}

	createdChat, err := BuildSessionOwnerProto("chat_create", request)
	require.NoError(t, err)
	chatCreate := createdChat.(*chatv1.ProvisionManagedChatRequest)
	require.Equal(t, app.String(), chatCreate.ApplicationId)
	require.Equal(t, env.String(), chatCreate.EnvironmentId)
	require.Equal(t, op.String(), chatCreate.OperationId)
	require.Equal(t, "party-a", chatCreate.ExternalChatKey)

	syncedChat, err := BuildSessionOwnerProto("chat_roster", request)
	require.NoError(t, err)
	chatSync := syncedChat.(*chatv1.SyncManagedChatMembersRequest)
	require.Equal(t, chat.String(), chatSync.ChatId)
	expectedMembers := []string{members[0].String(), members[1].String()}
	slices.Sort(expectedMembers)
	require.Equal(t, expectedMembers, chatSync.ProfileIds)

	provisionedVoice, err := BuildSessionOwnerProto("voice_provision", request)
	require.NoError(t, err)
	voice := provisionedVoice.(*callsv1.ProvisionGameSessionRoomRequest)
	require.Equal(t, session.String(), voice.SessionId)
	require.Equal(t, chatReceipt.String(), voice.ChatCreationOperationId)

	grants, err := BuildSessionOwnerProto("role_apply", request)
	require.NoError(t, err)
	apply := grants.(*rolev1.ApplyGameSessionGrantsRequest)
	require.Equal(t, session.String(), apply.SessionId)
	require.Equal(t, room.String(), apply.VoiceRoomId)
	require.Equal(t, uint64(7), apply.RosterRevision)

	closed, err := BuildSessionOwnerProto("terminalize_voice_close", request)
	require.NoError(t, err)
	closeRequest := closed.(*callsv1.CloseGameSessionRoomRequest)
	require.Equal(t, session.String(), closeRequest.SessionId)
	require.Equal(t, chatReceipt.String(), closeRequest.ChatCreationOperationId)

	revoked, err := BuildSessionOwnerProto("terminalize_role_revoke", request)
	require.NoError(t, err)
	revoke := revoked.(*rolev1.RevokeGameSessionGrantsRequest)
	require.Equal(t, session.String(), revoke.SessionId)
	require.Equal(t, op.String(), revoke.OperationId)
}
