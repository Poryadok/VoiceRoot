package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	messagingv1 "voice.app/voice/messaging/v1"
)

// TestSendMessage_GuestInExistingDM_AllowGuestDMDenied documents privacy.md allow_guest_dm gate for guest senders.
func TestSendMessage_GuestInExistingDM_AllowGuestDMDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000001_init.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000002_client_message_id.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000005_thread_settings.up.sql")

	profRegular, acctRegular := uuid.New(), uuid.New()
	profGuest, acctGuest := uuid.New(), uuid.New()
	profiles := profileAcctMap{profRegular: acctRegular, profGuest: acctGuest}

	chatID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profRegular, profGuest)

	client, cleanup := startMessagingServerWired(t, pool, messagingWire{
		UserProfiles: profiles,
		Privacy: dmPrivacyStub{
			allowGuestDM: map[uuid.UUID]bool{profRegular: false},
		},
		Friends: noFriendsStub{},
	})
	t.Cleanup(cleanup)

	_, err := client.SendMessage(withGuestProfileCtx(ctx, acctGuest, profGuest), &messagingv1.SendMessageRequest{
		Chat:    chatDMRef(chatID),
		Content: "guest reply blocked",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestSendMessage_GuestInExistingDM_AllowedWhenRecipientPermits documents guest reply when allow_guest_dm=true.
func TestSendMessage_GuestInExistingDM_AllowedWhenRecipientPermits(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000001_init.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000002_client_message_id.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000005_thread_settings.up.sql")

	profRegular, acctRegular := uuid.New(), uuid.New()
	profGuest, acctGuest := uuid.New(), uuid.New()
	profiles := profileAcctMap{profRegular: acctRegular, profGuest: acctGuest}

	chatID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profRegular, profGuest)

	client, cleanup := startMessagingServerWired(t, pool, messagingWire{
		UserProfiles: profiles,
		Privacy: dmPrivacyStub{
			allowGuestDM: map[uuid.UUID]bool{profRegular: true},
		},
		Friends: noFriendsStub{},
	})
	t.Cleanup(cleanup)

	_, err := client.SendMessage(withGuestProfileCtx(ctx, acctGuest, profGuest), &messagingv1.SendMessageRequest{
		Chat:    chatDMRef(chatID),
		Content: "guest reply allowed",
	})
	require.NoError(t, err)
}
