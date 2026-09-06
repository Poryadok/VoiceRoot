package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
)

func TestCheckDMPrivacyForSend_MissingDependenciesFailClosed(t *testing.T) {
	t.Parallel()
	err := (&MessagingGRPC{}).checkDMPrivacyForSend(context.Background(), chatv1.ChatType_CHAT_TYPE_DM, uuid.New(), uuid.New())
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestDMDependencyGates_SkipNonDMChats(t *testing.T) {
	t.Parallel()
	svc := &MessagingGRPC{}
	chatID, profileID := uuid.New(), uuid.New()
	require.NoError(t, svc.checkDMBlocksForSend(context.Background(), chatv1.ChatType_CHAT_TYPE_GROUP, chatID, profileID))
	require.NoError(t, svc.checkDMPrivacyForSend(context.Background(), chatv1.ChatType_CHAT_TYPE_CHANNEL, chatID, profileID))
}
