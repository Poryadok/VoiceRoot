package grpcsvc

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	messagingv1 "voice.app/voice/messaging/v1"
)

func TestMessageTextLimitCountsCharactersAcrossSendEditAndForwardCommentary(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applyBaseMessagingMigrations(t, ctx, pool)

	sourceChat := uuid.New()
	targetChat := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profC := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, sourceChat, profA, profB)
	seedDMChat(t, ctx, pool, targetChat, profA, profC)
	client, _ := startMessagingServer(t, pool)

	original, err := client.SendMessage(withProfileCtx(ctx, acctA, profB), &messagingv1.SendMessageRequest{
		Chat: chatDMRef(sourceChat), Content: "forward source", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)

	cases := []struct {
		name string
		text string
	}{
		{name: "ASCII", text: strings.Repeat("a", maxMessageTextCharacters)},
		{name: "Cyrillic", text: strings.Repeat("я", maxMessageTextCharacters)},
		{name: "non-BMP", text: strings.Repeat("😀", maxMessageTextCharacters)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Len(t, []rune(tc.text), maxMessageTextCharacters)
			overLimit := strings.Repeat(string([]rune(tc.text)[0]), maxMessageTextCharacters+1)

			sent, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
				Chat: chatDMRef(sourceChat), Content: tc.text, AttachmentsJson: "[]", MentionsJson: "[]",
			})
			require.NoError(t, err, "send must accept exactly 4,000 documented characters")
			require.Equal(t, tc.text, sent.GetMessage().GetContent())
			_, err = client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
				Chat: chatDMRef(sourceChat), Content: overLimit, AttachmentsJson: "[]", MentionsJson: "[]",
			})
			require.Equal(t, codes.InvalidArgument, status.Code(err), "send must reject 4,001 characters")

			edited, err := client.EditMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.EditMessageRequest{
				MessageId: sent.GetMessage().GetId(), Content: tc.text,
			})
			require.NoError(t, err, "edit must accept exactly 4,000 documented characters")
			require.Equal(t, tc.text, edited.GetMessage().GetContent())
			_, err = client.EditMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.EditMessageRequest{
				MessageId: sent.GetMessage().GetId(), Content: overLimit,
			})
			require.Equal(t, codes.InvalidArgument, status.Code(err), "edit must reject 4,001 characters")

			commentary := tc.text
			_, err = client.ForwardMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.ForwardMessageRequest{
				SourceMessageId: original.GetMessage().GetId(),
				TargetChat:      chatDMRef(targetChat),
				Commentary:      &commentary,
			})
			require.NoError(t, err, "forward commentary must accept exactly 4,000 documented characters")
			overCommentary := overLimit
			_, err = client.ForwardMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.ForwardMessageRequest{
				SourceMessageId: original.GetMessage().GetId(),
				TargetChat:      chatDMRef(targetChat),
				Commentary:      &overCommentary,
			})
			require.Equal(t, codes.InvalidArgument, status.Code(err), "forward commentary must reject 4,001 characters")
		})
	}

	history, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(targetChat),
	})
	require.NoError(t, err)
	for _, tc := range cases {
		found := false
		for _, message := range history.GetMessageList().GetMessages() {
			if message.GetContent() == tc.text {
				found = true
				break
			}
		}
		require.True(t, found, "accepted forward commentary must be stored for %s", tc.name)
	}
}
