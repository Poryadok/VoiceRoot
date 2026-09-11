package outboxdelivery

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type scriptedAlertCounter struct {
	mu     sync.Mutex
	counts []int64
	errs   []error
	calls  int
	order  *[]string
}

func (c *scriptedAlertCounter) CountAlertingOwnershipOutbox(context.Context) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.order != nil {
		*c.order = append(*c.order, "count")
	}
	index := c.calls
	c.calls++
	var count int64
	if index < len(c.counts) {
		count = c.counts[index]
	}
	if index < len(c.errs) {
		return count, c.errs[index]
	}
	return count, nil
}

type recordingAlertGauge struct {
	values []float64
	order  *[]string
}

func (g *recordingAlertGauge) Set(value float64) {
	if g.order != nil {
		*g.order = append(*g.order, "gauge")
	}
	g.values = append(g.values, value)
}

type alertDispatcher struct {
	err   error
	order *[]string
}

func (d *alertDispatcher) DispatchOnce(context.Context) error {
	if d.order != nil {
		*d.order = append(*d.order, "dispatch")
	}
	return d.err
}

func TestAlertMonitor_RefreshImmediatelyProjectsExactDurableCount(t *testing.T) {
	counter := &scriptedAlertCounter{counts: []int64{0, 3, 11}}
	gauge := &recordingAlertGauge{}
	monitor := NewAlertMonitor(counter, gauge, &alertDispatcher{})

	for range 3 {
		require.NoError(t, monitor.Refresh(context.Background()))
	}
	require.Equal(t, []float64{0, 3, 11}, gauge.values)
	require.Equal(t, 3, counter.calls, "every startup or scheduled refresh must query durable PostgreSQL state immediately")
}

func TestAlertMonitor_DispatchOnceRefreshesAfterErroredPass(t *testing.T) {
	var order []string
	dispatchErr := errors.New("publish pass failed")
	counter := &scriptedAlertCounter{counts: []int64{1}, order: &order}
	gauge := &recordingAlertGauge{order: &order}
	monitor := NewAlertMonitor(counter, gauge, &alertDispatcher{err: dispatchErr, order: &order})

	err := monitor.DispatchOnce(context.Background())
	require.ErrorIs(t, err, dispatchErr)
	require.Equal(t, []string{"dispatch", "count", "gauge"}, order,
		"an errored delivery pass must still project the failure count persisted by that pass")
	require.Equal(t, []float64{1}, gauge.values)
}

func TestAlertMonitor_CountFailurePropagatesWithoutInventingZero(t *testing.T) {
	queryErr := errors.New("postgres unavailable")
	counter := &scriptedAlertCounter{counts: []int64{0}, errs: []error{queryErr}}
	gauge := &recordingAlertGauge{values: []float64{7}}
	monitor := NewAlertMonitor(counter, gauge, &alertDispatcher{})

	err := monitor.Refresh(context.Background())
	require.ErrorIs(t, err, queryErr)
	require.Equal(t, []float64{7}, gauge.values,
		"a failed durable query must preserve the previous gauge instead of publishing an invented zero")
}

type cancellationAlertCounter struct {
	started chan struct{}
	exited  chan struct{}
}

func (c *cancellationAlertCounter) CountAlertingOwnershipOutbox(ctx context.Context) (int64, error) {
	close(c.started)
	<-ctx.Done()
	close(c.exited)
	return 0, ctx.Err()
}

func TestAlertMonitor_CancellationWaitsForRefreshAndDoesNotMutateGauge(t *testing.T) {
	counter := &cancellationAlertCounter{started: make(chan struct{}), exited: make(chan struct{})}
	gauge := &recordingAlertGauge{}
	monitor := NewAlertMonitor(counter, gauge, &alertDispatcher{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- monitor.DispatchOnce(ctx) }()

	<-counter.started
	cancel()
	<-counter.exited
	require.ErrorIs(t, <-done, context.Canceled)
	require.Empty(t, gauge.values, "a canceled post-dispatch refresh must not publish a stale value")
}
