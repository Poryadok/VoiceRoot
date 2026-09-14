package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMediaDeletionRetryDelay_isExponentialAndCapped(t *testing.T) {
	require.Equal(t, time.Second, mediaDeletionRetryDelay(0))
	require.Equal(t, time.Second, mediaDeletionRetryDelay(1))
	require.Equal(t, 2*time.Second, mediaDeletionRetryDelay(2))
	require.Equal(t, 4*time.Second, mediaDeletionRetryDelay(3))
	require.Equal(t, mediaDeletionMaxRetry, mediaDeletionRetryDelay(10))
	require.Equal(t, mediaDeletionMaxRetry, mediaDeletionRetryDelay(100))
}
