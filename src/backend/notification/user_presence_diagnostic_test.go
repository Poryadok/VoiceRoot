package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUserPresenceDiagnosticAllowsOnlyKnownCategories(t *testing.T) {
	var output userPresenceDiagnostic
	secret := "SECRET-postgres://password@host/private"
	_, err := output.Write([]byte("listen tcp 127.0.0.1:123: bind: address already in "))
	require.NoError(t, err)
	_, err = output.Write([]byte("use " + secret))
	require.NoError(t, err)
	require.Equal(t, "listener address in use", output.summary())
	require.NotContains(t, output.summary(), secret)

	var unknown userPresenceDiagnostic
	_, err = unknown.Write([]byte(secret))
	require.NoError(t, err)
	require.Equal(t, "unclassified startup failure", unknown.summary())
}

func TestUserPresenceReadinessReportsChildExitWithoutWaiting(t *testing.T) {
	done := make(chan struct{})
	close(done)
	started := time.Now()
	err := waitUserPresenceReady(context.Background(), done, time.Second, func(context.Context) bool { return false })
	require.ErrorIs(t, err, errUserPresenceExited)
	require.Less(t, time.Since(started), 500*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = waitUserPresenceReady(ctx, make(chan struct{}), time.Second, func(context.Context) bool { return false })
	require.True(t, errors.Is(err, context.Canceled))
}

func TestUserPresenceReadinessStillRequiresSuccessfulProbe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	err := waitUserPresenceReady(ctx, make(chan struct{}), time.Millisecond, func(context.Context) bool { return false })
	require.ErrorIs(t, err, context.DeadlineExceeded)
	err = waitUserPresenceReady(context.Background(), make(chan struct{}), time.Second, func(context.Context) bool { return true })
	require.NoError(t, err)
}

func TestUserPresenceReadinessRejectsExitDuringSuccessfulProbe(t *testing.T) {
	done := make(chan struct{})
	err := waitUserPresenceReady(context.Background(), done, time.Second, func(context.Context) bool {
		close(done)
		return true
	})
	require.ErrorIs(t, err, errUserPresenceExited)
}

func TestUserPresenceDiagnosticBoundsInput(t *testing.T) {
	var output userPresenceDiagnostic
	_, err := output.Write([]byte(strings.Repeat("x", 1<<20) + "address already in use"))
	require.NoError(t, err)
	require.Equal(t, "unclassified startup failure (output truncated)", output.summary())
}
