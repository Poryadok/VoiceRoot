package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"voice/backend/messaging/internal/gameprotocol"
	"voice/backend/messaging/internal/store"
)

// Legacy game fixtures explicitly model the documented standalone group.
// Production obtains this classification from Chat's authoritative metadata.
func standaloneGameCommunicationAdmission(ctx context.Context, chatID, profileID uuid.UUID) (context.Context, context.CancelFunc, error) {
	svc := &MessagingGRPC{CommunicationScope: communicationScopeStub{policy: &store.ChatThreadPolicy{ChatType: "group"}}}
	return svc.BeginCommunicationAction(ctx, chatID, profileID)
}

func TestSDKCommunicationBindsAuthPermitProfileAndExemptsOwnDelete(t *testing.T) {
	profile, chat := uuid.New(), uuid.New()
	calls := 0
	processor := &VerifiedGameMessageProcessor{CommunicationAdmission: func(ctx context.Context, actualChat, actualProfile uuid.UUID) (context.Context, context.CancelFunc, error) {
		calls++
		require.Equal(t, chat, actualChat)
		require.Equal(t, profile, actualProfile)
		return nil, nil, status.Error(codes.PermissionDenied, "member timed out")
	}}
	for _, operation := range []string{"create", "edit"} {
		_, _, err := processor.beginGameCommunicationAction(t.Context(), gameprotocol.Message{Operation: operation, ChatID: chat}, profile)
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	}
	require.Equal(t, 2, calls)
	processor.CommunicationAdmission = nil
	_, _, err := processor.beginGameCommunicationAction(t.Context(), gameprotocol.Message{Operation: "create", ChatID: chat}, profile)
	require.Equal(t, codes.Unavailable, status.Code(err))
	ctx, cancel, err := processor.beginGameCommunicationAction(t.Context(), gameprotocol.Message{Operation: "delete", ChatID: chat}, profile)
	require.NoError(t, err)
	require.Equal(t, t.Context(), ctx)
	cancel()
	require.Equal(t, 2, calls)
}
