package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/messaging/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

func TestValidateChatRefDM(t *testing.T) {
	t.Parallel()
	err := validateChatRefDM(nil)
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	group := chatv1.ChatType_CHAT_TYPE_GROUP
	require.Equal(t, codes.InvalidArgument, status.Code(validateChatRefDM(&chatv1.ChatRef{Id: uuid.New().String(), Type: &group})))

	dm := chatv1.ChatType_CHAT_TYPE_DM
	require.NoError(t, validateChatRefDM(&chatv1.ChatRef{Id: uuid.New().String(), Type: &dm}))
	require.NoError(t, validateChatRefDM(&chatv1.ChatRef{Id: uuid.New().String()}))
}

func TestMessageRowToProto(t *testing.T) {
	t.Parallel()
	require.Nil(t, messageRowToProto(nil, messagingv1.MessageKind_MESSAGE_KIND_UNSPECIFIED))

	now := time.Now().UTC()
	thread := uuid.New()
	edited := now.Add(-time.Minute)
	deleted := now.Add(-time.Second)
	row := &store.MessageRow{
		ID:              uuid.New(),
		ChatID:          uuid.New(),
		SenderProfileID: uuid.New(),
		Content:         "hi",
		Type:            "system",
		ThreadParentID:  &thread,
		AttachmentsJSON:   "[]",
		MentionsJSON:    "[]",
		EditedAt:        &edited,
		DeletedAt:       &deleted,
		CreatedAt:       now,
	}
	out := messageRowToProto(row, messagingv1.MessageKind_MESSAGE_KIND_UNSPECIFIED)
	require.Equal(t, thread.String(), out.GetThreadParentId())
	require.NotNil(t, out.GetEditedAt())
	require.NotNil(t, out.GetDeletedAt())
	require.Equal(t, messagingv1.MessageKind_MESSAGE_KIND_SYSTEM, out.GetMessageKind())

	row.Type = "forward"
	out = messageRowToProto(row, messagingv1.MessageKind_MESSAGE_KIND_UNSPECIFIED)
	require.Equal(t, messagingv1.MessageKind_MESSAGE_KIND_FORWARD, out.GetMessageKind())

	regKind := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	out = messageRowToProto(row, regKind)
	require.Equal(t, regKind, out.GetMessageKind())
}

func TestNextCursorForPage(t *testing.T) {
	t.Parallel()
	id1 := uuid.New()
	id2 := uuid.New()
	rows := []store.MessageRow{{ID: id1}, {ID: id2}}

	require.Empty(t, nextCursorForPage(store.ListLatest, nil))
	require.Equal(t, store.EncodeBeforeCursor(id2), nextCursorForPage(store.ListBeforeID, rows))
	require.Equal(t, store.EncodeAfterCursor(id2), nextCursorForPage(store.ListAfterID, rows))
	require.Equal(t, store.EncodeBeforeCursor(id2), nextCursorForPage(store.ListLatest, rows))
}

func TestMessagingGRPC_nilReceiver(t *testing.T) {
	t.Parallel()
	var s *MessagingGRPC
	_, err := s.SendMessage(context.Background(), &messagingv1.SendMessageRequest{})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}
