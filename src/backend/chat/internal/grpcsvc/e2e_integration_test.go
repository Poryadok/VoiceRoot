package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
)

// Phase 15 red tests: DM-only opt-in E2E flag on ChatService (docs/PLAN.md, docs/features/encryption.md).

func createDMForE2E(
	t *testing.T,
	client chatv1.ChatServiceClient,
	profiles mapProfileAccounts,
	profA, profB uuid.UUID,
) *chatv1.Chat {
	t.Helper()
	ctxA := withAccountProfileCtx(context.Background(), profiles[profA], profA)
	created, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	chat := created.GetChat()
	require.NotNil(t, chat)
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_DM, chat.GetType())
	return chat
}

// TestEnableChatE2E_DM_Succeeds documents Phase 15: both DM members may enable E2E per chat.
func TestEnableChatE2E_DM_Succeeds(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profA, profB := uuid.New(), uuid.New()
	profiles := profileMap(profA, profB)

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	chat := createDMForE2E(t, client, profiles, profA, profB)

	_, err := client.EnableChatE2E(ctxFor(t, profiles, profA), &chatv1.EnableChatE2ERequest{
		ChatId: chat.GetId(),
	})
	require.NoError(t, err)

	got, err := client.GetChat(ctxFor(t, profiles, profA), &chatv1.GetChatRequest{ChatId: chat.GetId()})
	require.NoError(t, err)
	require.True(t, got.GetChat().GetE2EEnabled(), "DM chat should expose e2e_enabled after EnableChatE2E")
}

// TestEnableChatE2E_Group_FailedPrecondition documents E2E is DM-only (encryption.md).
func TestEnableChatE2E_Group_FailedPrecondition(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New(), uuid.New(), uuid.New())
	ids := make([]uuid.UUID, 0, 3)
	for id := range profiles {
		ids = append(ids, id)
	}
	owner, inviteeA, inviteeB := ids[0], ids[1], ids[2]

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	group := createStandaloneGroup(t, client, profiles, owner, "no-e2e-group", inviteeA, inviteeB)

	_, err := client.EnableChatE2E(ctxFor(t, profiles, owner), &chatv1.EnableChatE2ERequest{
		ChatId: group.GetId(),
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestDisableChatE2E_DM_Succeeds documents opt-out reverts chat to plaintext mode server-side.
func TestDisableChatE2E_DM_Succeeds(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profA, profB := uuid.New(), uuid.New()
	profiles := profileMap(profA, profB)

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	chat := createDMForE2E(t, client, profiles, profA, profB)
	ctxA := ctxFor(t, profiles, profA)

	_, err := client.EnableChatE2E(ctxA, &chatv1.EnableChatE2ERequest{ChatId: chat.GetId()})
	require.NoError(t, err)

	_, err = client.DisableChatE2E(ctxA, &chatv1.DisableChatE2ERequest{ChatId: chat.GetId()})
	require.NoError(t, err)

	got, err := client.GetChat(ctxA, &chatv1.GetChatRequest{ChatId: chat.GetId()})
	require.NoError(t, err)
	require.False(t, got.GetChat().GetE2EEnabled(), "DM chat e2e_enabled should be false after DisableChatE2E")
}

// TestChat_E2EEnabled_ExposedOnGetDM documents Chat.e2e_enabled is returned from GetDM.
func TestChat_E2EEnabled_ExposedOnGetDM(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profA, profB := uuid.New(), uuid.New()
	profiles := profileMap(profA, profB)

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	chat := createDMForE2E(t, client, profiles, profA, profB)
	_, err := client.EnableChatE2E(ctxFor(t, profiles, profA), &chatv1.EnableChatE2ERequest{
		ChatId: chat.GetId(),
	})
	require.NoError(t, err)

	got, err := client.GetDM(ctxFor(t, profiles, profB), &chatv1.GetDMRequest{OtherProfileId: profA.String()})
	require.NoError(t, err)
	require.True(t, got.GetChat().GetE2EEnabled())
}
