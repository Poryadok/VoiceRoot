package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"

	"voice/backend/space/internal/store"
)

type runtimeOutboxStore struct {
	mu             sync.Mutex
	events         []store.ClaimedOwnershipOutboxEvent
	alerting       int64
	countCalls     int
	claimCalls     int
	failedCalls    int
	deliveredCalls int
	cleanupLimits  []int
	cleanupErr     error
	cleanupStarted chan struct{}
	cleanupExited  chan struct{}
	cleanupRelease chan struct{}
	recorder       *runtimeOrderRecorder
}

func (s *runtimeOutboxStore) ClaimReadyOwnershipOutbox(context.Context, int) ([]store.ClaimedOwnershipOutboxEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claimCalls++
	if s.recorder != nil {
		s.recorder.add("claim")
	}
	events := append([]store.ClaimedOwnershipOutboxEvent(nil), s.events...)
	s.events = nil
	return events, nil
}

func (s *runtimeOutboxStore) MarkOwnershipOutboxFailed(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failedCalls++
	return true, nil
}

func (s *runtimeOutboxStore) MarkOwnershipOutboxDelivered(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliveredCalls++
	if s.recorder != nil {
		s.recorder.add("delivered")
	}
	return true, nil
}

func (s *runtimeOutboxStore) CountAlertingOwnershipOutbox(ctx context.Context) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.countCalls++
	if s.recorder != nil {
		s.recorder.add("count")
	}
	return s.alerting, nil
}

func (s *runtimeOutboxStore) CleanupDeliveredOwnershipOutbox(ctx context.Context, limit int) (int64, error) {
	s.mu.Lock()
	s.cleanupLimits = append(s.cleanupLimits, limit)
	if s.recorder != nil {
		s.recorder.add("cleanup")
	}
	started, exited, release := s.cleanupStarted, s.cleanupExited, s.cleanupRelease
	s.mu.Unlock()
	if started != nil {
		close(started)
		<-ctx.Done()
		if exited != nil {
			close(exited)
		}
		if release != nil {
			<-release
		}
		if s.recorder != nil {
			s.recorder.add("cleanup_exit")
		}
		return 0, ctx.Err()
	}
	return 0, s.cleanupErr
}

type runtimeTransport struct {
	mu       sync.Mutex
	messages []*nats.Msg
	started  chan struct{}
	exited   chan struct{}
	release  chan struct{}
	recorder *runtimeOrderRecorder
}

func (p *runtimeTransport) Publish(ctx context.Context, msg *nats.Msg) (*nats.PubAck, error) {
	p.mu.Lock()
	p.messages = append(p.messages, msg)
	if p.recorder != nil {
		p.recorder.add("publish")
	}
	started, exited, release := p.started, p.exited, p.release
	p.mu.Unlock()
	if started == nil {
		return &nats.PubAck{Stream: "chat_events", Sequence: 1}, nil
	}
	close(started)
	<-ctx.Done()
	close(exited)
	<-release
	if p.recorder != nil {
		p.recorder.add("dispatch_exit")
	}
	return nil, ctx.Err()
}

type runtimeAlertGauge struct {
	mu     sync.Mutex
	values []float64
}

func (g *runtimeAlertGauge) Set(value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.values = append(g.values, value)
}

type runtimeLoopCall struct {
	interval time.Duration
	task     func(context.Context) error
	onError  func(error)
	done     chan struct{}
}

type immediateRuntimeLoop struct {
	calls chan runtimeLoopCall
}

func (r *immediateRuntimeLoop) Run(ctx context.Context, interval time.Duration, task func(context.Context) error, onError func(error)) {
	call := runtimeLoopCall{interval: interval, task: task, onError: onError, done: make(chan struct{})}
	r.calls <- call
	defer close(call.done)
	if err := task(ctx); err != nil && onError != nil {
		onError(err)
	}
}

type runtimeOrderRecorder struct {
	mu     sync.Mutex
	events []string
}

func (r *runtimeOrderRecorder) add(event string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *runtimeOrderRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

func runtimeTestEvent() store.ClaimedOwnershipOutboxEvent {
	return store.ClaimedOwnershipOutboxEvent{
		EventID:    uuid.New(),
		SpaceID:    uuid.New(),
		EventType:  "space.updated",
		CreatedAt:  time.Date(2026, time.September, 12, 4, 0, 0, 0, time.UTC),
		LeaseToken: uuid.New(),
	}
}

func receiveRuntimeLoop(t *testing.T, calls <-chan runtimeLoopCall) runtimeLoopCall {
	t.Helper()
	select {
	case call := <-calls:
		return call
	case <-t.Context().Done():
		t.Fatal("ownership outbox runtime loop did not start")
		return runtimeLoopCall{}
	}
}

func TestOwnershipOutboxRuntimeConfig_DefaultsAndEnvironmentOverrides(t *testing.T) {
	t.Setenv("SPACE_OWNERSHIP_OUTBOX_DISPATCH_INTERVAL", "")
	t.Setenv("SPACE_OWNERSHIP_OUTBOX_CLEANUP_INTERVAL", "")
	config := ownershipOutboxRuntimeConfigFromEnv()
	require.Equal(t, time.Second, config.dispatchInterval)
	require.Equal(t, time.Hour, config.cleanupInterval)

	t.Setenv("SPACE_OWNERSHIP_OUTBOX_DISPATCH_INTERVAL", "250ms")
	t.Setenv("SPACE_OWNERSHIP_OUTBOX_CLEANUP_INTERVAL", "45m")
	config = ownershipOutboxRuntimeConfigFromEnv()
	require.Equal(t, 250*time.Millisecond, config.dispatchInterval)
	require.Equal(t, 45*time.Minute, config.cleanupInterval)
}

func TestOwnershipOutboxRuntime_StartsImmediateDeliveryAlertAndCleanupWithSharedDependencies(t *testing.T) {
	event := runtimeTestEvent()
	recorder := &runtimeOrderRecorder{}
	storeFake := &runtimeOutboxStore{events: []store.ClaimedOwnershipOutboxEvent{event}, alerting: 2, cleanupErr: errors.New("cleanup unavailable"), recorder: recorder}
	transport := &runtimeTransport{recorder: recorder}
	gauge := &runtimeAlertGauge{}
	loop := &immediateRuntimeLoop{calls: make(chan runtimeLoopCall, 2)}
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	config := ownershipOutboxRuntimeConfig{dispatchInterval: 250 * time.Millisecond, cleanupInterval: 45 * time.Minute}

	runtime := startOwnershipOutboxRuntime(context.Background(), config, ownershipOutboxRuntimeDependencies{
		store: storeFake, transport: transport, alertGauge: gauge, logger: logger, runLoop: loop.Run,
	})
	require.NotNil(t, runtime)
	first := receiveRuntimeLoop(t, loop.calls)
	second := receiveRuntimeLoop(t, loop.calls)
	require.ElementsMatch(t, []time.Duration{config.dispatchInterval, config.cleanupInterval},
		[]time.Duration{first.interval, second.interval})
	<-first.done
	<-second.done
	runtime.Stop()

	storeFake.mu.Lock()
	require.Equal(t, 1, storeFake.claimCalls, "delivery must run immediately without waiting for its interval")
	require.Equal(t, 1, storeFake.deliveredCalls)
	require.Zero(t, storeFake.failedCalls)
	require.Equal(t, []int{100}, storeFake.cleanupLimits, "cleanup must use the documented bounded batch")
	require.Equal(t, 2, storeFake.countCalls, "alert state must refresh at startup and after the immediate dispatch")
	storeFake.mu.Unlock()
	transport.mu.Lock()
	require.Len(t, transport.messages, 1, "the configured NATS transport must publish the event from the configured store")
	require.Equal(t, event.EventID.String(), transport.messages[0].Header.Get(nats.MsgIdHdr))
	transport.mu.Unlock()
	gauge.mu.Lock()
	require.Equal(t, []float64{2, 2}, gauge.values)
	gauge.mu.Unlock()
	order := recorder.snapshot()
	require.Less(t, indexRuntimeEvent(order, "count"), indexRuntimeEvent(order, "claim"),
		"startup alert refresh must complete before the immediate delivery pass")
	require.Less(t, indexRuntimeEvent(order, "claim"), indexRuntimeEvent(order, "publish"))
	require.Less(t, indexRuntimeEvent(order, "publish"), indexRuntimeEvent(order, "delivered"))
	require.Less(t, indexRuntimeEvent(order, "delivered"), lastIndexRuntimeEvent(order, "count"),
		"the completed delivery pass must refresh durable alert state")
	require.Contains(t, logs.String(), "ownership outbox cleanup")
	require.Contains(t, logs.String(), storeFake.cleanupErr.Error())
}

func TestOwnershipOutboxRuntime_MissingDatabaseOrNATSStartsNoWorkers(t *testing.T) {
	config := ownershipOutboxRuntimeConfig{dispatchInterval: time.Second, cleanupInterval: time.Hour}
	for _, test := range []struct {
		name      string
		store     *runtimeOutboxStore
		transport *runtimeTransport
	}{
		{name: "database", transport: &runtimeTransport{}},
		{name: "nats", store: &runtimeOutboxStore{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			loop := &immediateRuntimeLoop{calls: make(chan runtimeLoopCall, 2)}
			runtime := startOwnershipOutboxRuntime(context.Background(), config, ownershipOutboxRuntimeDependencies{
				store: test.store, transport: test.transport, alertGauge: &runtimeAlertGauge{}, runLoop: loop.Run,
			})
			require.Nil(t, runtime)
			select {
			case <-loop.calls:
				t.Fatal("worker started without both database and NATS")
			default:
			}
		})
	}
}

func TestOwnershipOutboxRuntime_StopCancelsAndWaitsBeforePublisherAndPoolClose(t *testing.T) {
	recorder := &runtimeOrderRecorder{}
	storeFake := &runtimeOutboxStore{
		events:         []store.ClaimedOwnershipOutboxEvent{runtimeTestEvent()},
		cleanupStarted: make(chan struct{}), cleanupExited: make(chan struct{}), cleanupRelease: make(chan struct{}),
		recorder: recorder,
	}
	transport := &runtimeTransport{
		started: make(chan struct{}), exited: make(chan struct{}), release: make(chan struct{}), recorder: recorder,
	}
	loop := &immediateRuntimeLoop{calls: make(chan runtimeLoopCall, 2)}
	runtime := startOwnershipOutboxRuntime(context.Background(), ownershipOutboxRuntimeConfig{
		dispatchInterval: time.Second, cleanupInterval: time.Hour,
	}, ownershipOutboxRuntimeDependencies{
		store: storeFake, transport: transport, alertGauge: &runtimeAlertGauge{}, runLoop: loop.Run,
	})
	require.NotNil(t, runtime)
	receiveRuntimeLoop(t, loop.calls)
	receiveRuntimeLoop(t, loop.calls)
	<-transport.started
	<-storeFake.cleanupStarted

	stopped := make(chan struct{})
	go func() {
		runtime.Stop()
		recorder.add("runtime_stopped")
		close(stopped)
	}()
	<-transport.exited
	<-storeFake.cleanupExited
	select {
	case <-stopped:
		t.Fatal("runtime returned before in-flight delivery and cleanup exited")
	default:
	}
	close(transport.release)
	close(storeFake.cleanupRelease)
	<-stopped
	recorder.add("publisher_close")
	recorder.add("pool_close")

	order := recorder.snapshot()
	for _, event := range []string{"dispatch_exit", "cleanup_exit", "runtime_stopped", "publisher_close", "pool_close"} {
		require.Contains(t, order, event)
	}
	require.Less(t, indexRuntimeEvent(order, "dispatch_exit"), indexRuntimeEvent(order, "runtime_stopped"))
	require.Less(t, indexRuntimeEvent(order, "cleanup_exit"), indexRuntimeEvent(order, "runtime_stopped"))
	require.Less(t, indexRuntimeEvent(order, "runtime_stopped"), indexRuntimeEvent(order, "publisher_close"))
	require.Less(t, indexRuntimeEvent(order, "runtime_stopped"), indexRuntimeEvent(order, "pool_close"))
}

func indexRuntimeEvent(events []string, want string) int {
	for index, event := range events {
		if event == want {
			return index
		}
	}
	return -1
}

func lastIndexRuntimeEvent(events []string, want string) int {
	for index := len(events) - 1; index >= 0; index-- {
		if events[index] == want {
			return index
		}
	}
	return -1
}
