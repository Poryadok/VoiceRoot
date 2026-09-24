package main

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

type notificationConsumerReadiness struct {
	mu    sync.RWMutex
	bound map[string]bool
}

func newNotificationConsumerReadiness(names ...string) *notificationConsumerReadiness {
	r := &notificationConsumerReadiness{bound: make(map[string]bool, len(names))}
	for _, name := range names {
		r.bound[name] = false
	}
	return r
}
func (r *notificationConsumerReadiness) set(name string, ready bool) {
	r.mu.Lock()
	r.bound[name] = ready
	r.mu.Unlock()
}
func (r *notificationConsumerReadiness) ready() bool {
	if r == nil {
		return true
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ready := range r.bound {
		if !ready {
			return false
		}
	}
	return true
}

type notificationConsumerReadinessKey struct{}
type notificationConsumerReadinessBinding struct {
	tracker *notificationConsumerReadiness
	name    string
}

func withNotificationConsumerReadiness(ctx context.Context, tracker *notificationConsumerReadiness, name string) context.Context {
	return context.WithValue(ctx, notificationConsumerReadinessKey{}, notificationConsumerReadinessBinding{tracker, name})
}
func markNotificationConsumerBound(ctx context.Context) {
	binding, _ := ctx.Value(notificationConsumerReadinessKey{}).(notificationConsumerReadinessBinding)
	if binding.tracker != nil {
		binding.tracker.set(binding.name, true)
	}
}

// runNotificationConsumerLoop owns one durable for the lifetime of the service.
// Readiness is cleared on every bind failure, disconnect, and shutdown.
func runNotificationConsumerLoop(ctx context.Context, tracker *notificationConsumerReadiness, name string, retryDelay time.Duration, run func(context.Context) error, logger *slog.Logger) {
	for ctx.Err() == nil {
		tracker.set(name, false)
		err := run(withNotificationConsumerReadiness(ctx, tracker, name))
		tracker.set(name, false)
		if ctx.Err() != nil {
			return
		}
		if logger != nil {
			logger.Warn("notification consumer binding lost; retrying", slog.String("consumer", name), slog.Any("error", err))
		}
		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// A disconnected connection must revalidate and rebind its durable. Reconnect
// alone cannot prove the durable or its exact filter still exists.
func connectNotificationConsumer(url, name string) (*nats.Conn, <-chan struct{}, error) {
	lost := make(chan struct{})
	var once sync.Once
	nc, err := nats.Connect(url,
		nats.Name("voice-notification-"+name),
		nats.CustomInboxPrefix("_INBOX.voice.notification"),
		nats.Timeout(10*time.Second),
		nats.MaxReconnects(0),
		nats.DisconnectErrHandler(func(*nats.Conn, error) { once.Do(func() { close(lost) }) }),
		nats.ClosedHandler(func(*nats.Conn) { once.Do(func() { close(lost) }) }),
	)
	return nc, lost, err
}

func waitForNotificationConsumer(ctx context.Context, lost <-chan struct{}, js nats.JetStreamContext, stream, durable, subject, target string) error {
	return watchNotificationConsumer(ctx, lost, 5*time.Second, func() error {
		info, err := js.ConsumerInfo(stream, durable)
		if err != nil {
			return err
		}
		return validateNotificationConsumerFilter(info, stream, durable, subject, target)
	})
}

func watchNotificationConsumer(ctx context.Context, lost <-chan struct{}, interval time.Duration, check func() error) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-lost:
			return errors.New("NATS connection lost")
		case <-ticker.C:
			if err := check(); err != nil {
				return err
			}
		}
	}
}
