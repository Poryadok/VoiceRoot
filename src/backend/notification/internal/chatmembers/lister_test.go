package chatmembers_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/chatmembers"
)

func TestNoopLister_ReturnsEmpty(t *testing.T) {
	ids, err := chatmembers.NoopLister{}.ListMemberProfileIDs(context.Background(), "chat-1")
	require.NoError(t, err)
	require.Empty(t, ids)
}
