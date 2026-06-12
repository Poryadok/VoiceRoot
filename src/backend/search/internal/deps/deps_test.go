package deps

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMessagingFetcher_nilClient(t *testing.T) {
	t.Parallel()
	_, _, err := (&MessagingFetcher{}).GetMessageBody(context.Background(), uuid.Nil, uuid.Nil)
	require.Error(t, err)
}

func TestSocialBlocks_nilClient(t *testing.T) {
	t.Parallel()
	out, err := (&SocialBlocks{}).BlockedAccountIDs(context.Background())
	require.NoError(t, err)
	require.Nil(t, out)
}

func TestProfileHydrator_nilClient(t *testing.T) {
	t.Parallel()
	_, _, _, _, err := (&ProfileHydrator{}).LoadProfile(context.Background(), uuid.Nil)
	require.Error(t, err)
}
