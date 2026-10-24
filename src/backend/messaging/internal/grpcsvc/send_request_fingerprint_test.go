package grpcsvc

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"testing"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

func TestExactSendFingerprintPreservesDocumentedNormalization(t *testing.T) {
	chat := uuid.New()
	kind := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	first := &messagingv1.SendMessageRequest{Content: "  hello  ", AttachmentsJson: `[{"type":"photo","file_id":"file"}]`}
	second := proto.Clone(first).(*messagingv1.SendMessageRequest)
	second.Content = "hello"
	second.AttachmentsJson = `[ { "file_id":"file", "type":"photo" } ]`
	second.MentionsJson = "[]"
	second.MessageKind = &kind
	second.PostedAsChat = proto.Bool(false)
	second.IsE2E = proto.Bool(false)
	a, err := sendRequestFingerprint(first, chat, chatv1.ChatType_CHAT_TYPE_GROUP, nil)
	require.NoError(t, err)
	b, err := sendRequestFingerprint(second, chat, chatv1.ChatType_CHAT_TYPE_GROUP, nil)
	require.NoError(t, err)
	require.Equal(t, a, b)
	second.SendSilent = true
	b, err = sendRequestFingerprint(second, chat, chatv1.ChatType_CHAT_TYPE_GROUP, nil)
	require.NoError(t, err)
	require.NotEqual(t, a, b)
	second.SendSilent = false
	second.Content = "new effect"
	b, err = sendRequestFingerprint(second, chat, chatv1.ChatType_CHAT_TYPE_GROUP, nil)
	require.NoError(t, err)
	require.NotEqual(t, a, b)
}
