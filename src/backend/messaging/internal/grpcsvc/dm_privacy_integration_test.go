package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/privacy"
)

type dmPrivacyStub struct {
	friendsOnly map[uuid.UUID]bool
}

func (s dmPrivacyStub) AllowDMAudience(_ context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if s.friendsOnly[profileID] {
		return privacy.FriendsOnly(), nil
	}
	return privacy.EveryoneWithGuests(), nil
}

func (s dmPrivacyStub) AllowFilesAudience(_ context.Context, _ uuid.UUID) (privacy.Audience, error) {
	return privacy.EveryoneWithGuests(), nil
}

func (s dmPrivacyStub) AllowVoiceMessagesAudience(_ context.Context, _ uuid.UUID) (privacy.Audience, error) {
	return privacy.EveryoneWithGuests(), nil
}

type noFriendsStub struct{}

func (noFriendsStub) AreFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (noFriendsStub) AreFriendsOfFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

// TestSendMessage_FriendsOnlyPrivacy_StrangerDenied documents privacy enforcement on DM SendMessage.
func TestSendMessage_FriendsOnlyPrivacy_StrangerDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000001_init.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000002_client_message_id.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000005_thread_settings.up.sql")

	profOwner, acctOwner := uuid.New(), uuid.New()
	profStranger, acctStranger := uuid.New(), uuid.New()
	profiles := profileAcctMap{profOwner: acctOwner, profStranger: acctStranger}

	chatID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profOwner, profStranger)

	client, cleanup := startMessagingServerWired(t, pool, messagingWire{
		UserProfiles: profiles,
		Privacy:      dmPrivacyStub{friendsOnly: map[uuid.UUID]bool{profOwner: true}},
		Friends:      noFriendsStub{},
	})
	t.Cleanup(cleanup)

	_, err := client.SendMessage(withProfileCtx(ctx, acctStranger, profStranger), &messagingv1.SendMessageRequest{
		Chat:    chatDMRef(chatID),
		Content: "blocked by friends-only privacy",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestSendMessage_FriendsOnlyPrivacy_OwnerAllowed documents DM owner can still send when privacy is friends-only.
func TestSendMessage_FriendsOnlyPrivacy_OwnerAllowed(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000001_init.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000002_client_message_id.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000005_thread_settings.up.sql")

	profOwner, acctOwner := uuid.New(), uuid.New()
	profStranger, acctStranger := uuid.New(), uuid.New()
	profiles := profileAcctMap{profOwner: acctOwner, profStranger: acctStranger}

	chatID := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profOwner, profStranger)

	client, cleanup := startMessagingServerWired(t, pool, messagingWire{
		UserProfiles: profiles,
		Privacy:      dmPrivacyStub{friendsOnly: map[uuid.UUID]bool{profOwner: true}},
		Friends:      noFriendsStub{},
	})
	t.Cleanup(cleanup)

	_, err := client.SendMessage(withProfileCtx(ctx, acctOwner, profOwner), &messagingv1.SendMessageRequest{
		Chat:    chatDMRef(chatID),
		Content: "owner message allowed",
	})
	require.NoError(t, err)
}
