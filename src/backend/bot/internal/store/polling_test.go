package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDevPollingRequiresExplicitOptIn(t *testing.T) {
	t.Setenv("BOT_ENABLE_DEV_POLLING", "")
	require.False(t, DevPollingEnabled())
	t.Setenv("BOT_ENABLE_DEV_POLLING", "true")
	require.True(t, DevPollingEnabled())
	t.Setenv("BOT_ENABLE_DEV_POLLING", "false")
	require.False(t, DevPollingEnabled())
}
