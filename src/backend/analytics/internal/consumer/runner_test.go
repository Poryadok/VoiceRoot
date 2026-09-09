package consumer

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func TestIsJetStreamNotFound(t *testing.T) {
	require.True(t, isJetStreamNotFound(nats.ErrStreamNotFound))
	require.True(t, isJetStreamNotFound(fmt.Errorf("subscribe: %w", nats.ErrStreamNotFound)))
	require.True(t, isJetStreamNotFound(errors.New("nats: stream not found")))
	require.False(t, isJetStreamNotFound(errors.New("permission denied")))
}

func TestSubscribeCreateOrBindRetriesOnlyMissingPublisherStream(t *testing.T) {
	bindCalled := false
	_, err := subscribeCreateOrBind(
		func() (*nats.Subscription, error) { return nil, nats.ErrStreamNotFound },
		func() (*nats.Subscription, error) { bindCalled = true; return nil, nil },
	)

	require.ErrorIs(t, err, nats.ErrStreamNotFound)
	require.False(t, bindCalled)
}

func TestSubscribeCreateOrBindUsesExistingDurable(t *testing.T) {
	createCalls, bindCalls := 0, 0
	want := &nats.Subscription{}
	got, err := subscribeCreateOrBind(
		func() (*nats.Subscription, error) { createCalls++; return nil, errors.New("consumer already exists") },
		func() (*nats.Subscription, error) { bindCalls++; return want, nil },
	)

	require.NoError(t, err)
	require.Same(t, want, got)
	require.Equal(t, 1, createCalls)
	require.Equal(t, 1, bindCalls)
}

func TestSubscribeJetStreamWithRetryRetriesMissingStream(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	attempts := 0
	want := &nats.Subscription{}
	got, err := subscribeJetStreamWithRetry(ctx, nil, "message_events", func() (*nats.Subscription, error) {
		attempts++
		if attempts == 1 {
			return nil, nats.ErrStreamNotFound
		}
		return want, nil
	})

	require.NoError(t, err)
	require.Same(t, want, got)
	require.Equal(t, 2, attempts)
}

func TestSubscribeJetStreamWithRetryReturnsNonStreamErrorImmediately(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	attempts := 0
	want := errors.New("permission denied")
	_, err := subscribeJetStreamWithRetry(ctx, nil, "message_events", func() (*nats.Subscription, error) {
		attempts++
		return nil, want
	})

	require.ErrorIs(t, err, want)
	require.Equal(t, 1, attempts)
}
