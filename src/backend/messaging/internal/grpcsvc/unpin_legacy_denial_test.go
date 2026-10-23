package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/store"
)

func TestLegacyBotUnpinCannotBypassProtectedCleanup(t *testing.T) {
	service := &MessagingGRPC{Pins: &store.PinsStore{}}
	request := &messagingv1.UnpinMessagesBySenderInChatsRequest{SenderProfileId: uuid.NewString(), ChatIds: []string{uuid.NewString()}}
	for _, ctx := range []context.Context{context.Background(), metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-internal-caller", "bot", "x-profile-id", uuid.NewString()))} {
		response, err := service.UnpinMessagesBySenderInChats(ctx, request)
		require.Nil(t, response)
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	}
}
