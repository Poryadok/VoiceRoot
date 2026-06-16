package privacy_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/story/internal/privacy"
)

func TestFriendChecker_nilClient(t *testing.T) {
	fc := privacy.NewFriendChecker(nil)
	ok, err := fc.IsFriend(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	require.False(t, ok)
}
