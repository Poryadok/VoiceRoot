package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/store"
)

// TestMarkRead_DMReceiptOptOutKeepsPrivateCursor proves the privacy boundary
// against PostgreSQL: the reader's cursor and unread count advance while no
// peer-visible receipt row or message.read event is created.
func TestMarkRead_DMReceiptOptOutKeepsPrivateCursor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyA1ReadProofMigrations(t, ctx, pool)
	chatID, sender, reader := uuid.New(), uuid.New(), uuid.New()
	senderAccount, readerAccount := uuid.New(), uuid.New()
	seedDMChat(t, ctx, pool, chatID, sender, reader)
	events := &spyMessageEvents{}
	client, _ := startMessagingServerWired(t, pool, messagingWire{
		Privacy:       receiptPrivacyStub{enabled: map[uuid.UUID]bool{sender: true, reader: false}},
		MessageEvents: events,
	})
	sent, err := client.SendMessage(withProfileCtx(ctx, senderAccount, sender), &messagingv1.SendMessageRequest{Chat: chatDMRef(chatID), Content: "private receipt", AttachmentsJson: "[]", MentionsJson: "[]"})
	require.NoError(t, err)
	messageID := uuid.MustParse(sent.GetMessage().GetId())
	_, err = client.MarkRead(withProfileCtx(ctx, readerAccount, reader), &messagingv1.MarkReadRequest{Chat: chatDMRef(chatID), LastReadMessageId: messageID.String()})
	require.NoError(t, err)

	messages := &store.MessagesStore{Pool: pool}
	private, _, err := messages.GetReadPosition(ctx, chatID, reader)
	require.NoError(t, err)
	require.Equal(t, messageID, *private)
	public, _, err := messages.GetReadReceipt(ctx, chatID, reader)
	require.NoError(t, err)
	require.Nil(t, public)
	events.mu.Lock()
	reads := len(events.read)
	events.mu.Unlock()
	require.Zero(t, reads)
	metadata := getA1ProofMetadata(t, ctx, client, senderAccount, sender, chatID)
	require.Equal(t, messagingv1.LastMessageDeliveryState_LAST_MESSAGE_DELIVERY_STATE_SENT, metadata.GetLastMessageDeliveryState())
	readerMeta := getA1ProofMetadata(t, ctx, client, readerAccount, reader, chatID)
	require.Zero(t, readerMeta.GetUnreadCount())
}
