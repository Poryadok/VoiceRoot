package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
)

func TestEpochSchemaPreservesNativeMembershipSource(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	st, source, _, _ := messageSendStore(t) // Applies the complete actual migration catalog.
	ctx := context.Background()
	_, err := st.Pool.Exec(ctx, `UPDATE schema_migrations SET version=9`)
	require.NoError(t, err)
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Epoch source", "", `["TEXT_CHAT_SEND_MESSAGES"]`, uuid.New())
	require.NoError(t, err)
	typ := chatv1.ChatType_CHAT_TYPE_GROUP
	req := &botv1.InstallBotInSpaceRequest{BotId: bot.ID.String(), SpaceId: uuid.NewString(), AllowedChats: []*chatv1.ChatRef{{Id: uuid.NewString(), Type: &typ}}}
	_, err = st.PrepareInstall(ctx, source, nativeAdmin(t, req, botv1.BotService_InstallBotInSpace_FullMethodName), req)
	require.NoError(t, err, "the fully migrated epoch schema must retain the existing native source authority")
	var retained uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT source_generation FROM bot_space_membership_source WHERE singleton`).Scan(&retained))
	require.Equal(t, source, retained, "credential schema upgrade must not relabel installation authority")
}
