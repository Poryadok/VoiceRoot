package consumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRetryUntilContextDoneRetriesTransientSubscriptionFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	attempts := 0
	err := retryUntilContextDone(ctx, time.Millisecond, func() error {
		attempts++
		if attempts == 1 {
			return errors.New("nats: stream not found")
		}
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, 2, attempts)
}

func TestRetryUntilContextDoneStopsWhenContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	err := retryUntilContextDone(ctx, time.Millisecond, func() error {
		attempts++
		cancel()
		return errors.New("nats: stream not found")
	})

	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, attempts)
}
