package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/store"
)

func TestCheckMessageReadEntitlementRequiresSpecificInternalCaller(t *testing.T) {
	service := &ChatGRPC{DM: &store.DMStore{}}
	request := &chatv1.CheckMessageReadEntitlementRequest{
		ChatId: uuid.NewString(), ProfileId: uuid.NewString(), MessageCreatedAt: timestamppb.New(time.Now().UTC()),
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-internal-caller", "untrusted"))
	_, err := service.CheckMessageReadEntitlement(ctx, request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	for _, caller := range []string{"messaging", "search", "file"} {
		internal := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-internal-caller", caller))
		_, err = service.CheckMessageReadEntitlement(internal, request)
		require.Equal(t, codes.Unavailable, status.Code(err), "caller %s must reach the Chat store", caller)
	}
	ambiguous := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-internal-caller", "messaging", "x-voice-internal-caller", "search"))
	_, err = service.CheckMessageReadEntitlement(ambiguous, request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
