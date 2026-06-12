package presence_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/presence"
)

func TestOfflineChecker_AlwaysOffline(t *testing.T) {
	online, err := presence.OfflineChecker{}.IsOnline(context.Background(), uuid.New())
	require.NoError(t, err)
	require.False(t, online)
}
