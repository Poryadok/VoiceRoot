package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNotificationReadinessRequiresEveryConsumer(t *testing.T) {
	names := []string{"story", "message", "matchmaking", "voice", "social", "moderation", "subscription"}
	readiness := newNotificationConsumerReadiness(names...)
	handler := notificationHTTPHandlerWithReadiness(serviceName, readiness)
	status := func() int {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
		return rec.Code
	}
	require.Equal(t, http.StatusServiceUnavailable, status())
	for _, name := range names[:len(names)-1] {
		readiness.set(name, true)
	}
	require.Equal(t, http.StatusServiceUnavailable, status(), "one missing durable must block readiness")
	readiness.set(names[len(names)-1], true)
	require.Equal(t, http.StatusOK, status())
	readiness.set("message", false)
	require.Equal(t, http.StatusServiceUnavailable, status(), "lost binding must clear readiness")
}

func TestNotificationConsumerRetriesBindAndRebindsAfterLoss(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	readiness := newNotificationConsumerReadiness("message")
	bindFailed := make(chan struct{})
	bound := make(chan struct{})
	lost := make(chan struct{})
	rebindAttempted := make(chan struct{})
	allowRebind := make(chan struct{})
	rebound := make(chan struct{})
	var attempts atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		runNotificationConsumerLoop(ctx, readiness, "message", time.Millisecond, func(runCtx context.Context) error {
			switch attempts.Add(1) {
			case 1:
				close(bindFailed)
				return errors.New("missing or drifted durable")
			case 2:
				markNotificationConsumerBound(runCtx)
				close(bound)
				<-lost
				return errors.New("connection lost")
			default:
				close(rebindAttempted)
				<-allowRebind
				markNotificationConsumerBound(runCtx)
				close(rebound)
				<-runCtx.Done()
				return runCtx.Err()
			}
		}, nil)
	}()
	<-bindFailed
	require.False(t, readiness.ready())
	<-bound
	require.True(t, readiness.ready())
	close(lost)
	select {
	case <-rebindAttempted:
	case <-time.After(time.Second):
		t.Fatal("consumer did not retry after connection loss")
	}
	require.False(t, readiness.ready(), "lost binding must block readiness before rebinding")
	close(allowRebind)
	select {
	case <-rebound:
	case <-time.After(time.Second):
		t.Fatal("consumer did not retry after connection loss")
	}
	require.True(t, readiness.ready())
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("consumer did not stop with root context")
	}
	require.False(t, readiness.ready())
	require.EqualValues(t, 3, attempts.Load())
}

func TestNotificationConsumerWatchDetectsDurableDrift(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lost := make(chan struct{})
	drift := errors.New("durable filter widened")
	err := watchNotificationConsumer(ctx, lost, time.Millisecond, func() error { return drift })
	require.ErrorIs(t, err, drift)

	close(lost)
	err = watchNotificationConsumer(ctx, lost, time.Hour, func() error { t.Fatal("check after disconnect"); return nil })
	require.Error(t, err)
}
