package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type retentionStoreFixture struct {
	list  func(context.Context, int) ([]uuid.UUID, error)
	clean func(context.Context, uuid.UUID, int) (int64, error)
}

func (s retentionStoreFixture) ListExpiredAccountBanIntentSpaces(ctx context.Context, n int) ([]uuid.UUID, error) {
	return s.list(ctx, n)
}
func (s retentionStoreFixture) CleanupExpiredAccountBanIntents(ctx context.Context, id uuid.UUID, n int) (int64, error) {
	return s.clean(ctx, id, n)
}

func retentionAwait[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(4 * time.Second):
		t.Fatal("retention worker did not progress")
		var zero T
		return zero
	}
}

func TestAccountBanRetentionBoundsAndDropsBusyTicks(t *testing.T) {
	ticks := make(chan time.Time, 1)
	entered := make(chan int, 4)
	release := make(chan struct{})
	var calls atomic.Int32
	first := uuid.New()
	worker := startAccountBanRetentionWithTicks(context.Background(), retentionStoreFixture{
		list: func(ctx context.Context, n int) ([]uuid.UUID, error) {
			require.Equal(t, 16, n)
			deadline, ok := ctx.Deadline()
			require.True(t, ok)
			require.InDelta(t, 2, time.Until(deadline).Seconds(), 0.2)
			entered <- int(calls.Add(1))
			return []uuid.UUID{first}, nil
		},
		clean: func(ctx context.Context, id uuid.UUID, n int) (int64, error) {
			require.Equal(t, first, id)
			require.Equal(t, 256, n)
			if calls.Load() == 1 {
				<-release
			}
			return 1, nil
		},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), ticks, func() {})
	t.Cleanup(worker.Stop)
	require.Equal(t, 1, retentionAwait(t, entered), "first cycle starts asynchronously without a tick")
	ticks <- time.Now()
	require.Never(t, func() bool { return calls.Load() != 1 }, 30*time.Millisecond, time.Millisecond, "a busy cycle must not overlap")
	close(release)
	require.Never(t, func() bool { return calls.Load() != 1 }, 30*time.Millisecond, time.Millisecond, "a busy tick must not queue another cycle")
	ticks <- time.Now()
	require.Equal(t, 2, retentionAwait(t, entered))
}

func TestAccountBanRetentionStopCancelsAndJoins(t *testing.T) {
	for _, parentShutdown := range []bool{false, true} {
		t.Run(map[bool]string{false: "explicit stop", true: "parent cancellation"}[parentShutdown], func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			entered, canceled, release, stopped := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
			worker := startAccountBanRetentionWithTicks(parent, retentionStoreFixture{
				list: func(ctx context.Context, _ int) ([]uuid.UUID, error) {
					close(entered)
					<-ctx.Done()
					close(canceled)
					<-release // Joining also waits for the cancelled driver's rollback.
					return nil, ctx.Err()
				},
			}, slog.New(slog.NewTextHandler(io.Discard, nil)), make(chan time.Time), func() {})
			retentionAwait(t, entered)
			if parentShutdown {
				cancel()
			}
			go func() { worker.Stop(); close(stopped) }()
			retentionAwait(t, canceled)
			select {
			case <-stopped:
				close(release)
				t.Fatal("Stop returned before the cancelled cycle finished")
			case <-time.After(30 * time.Millisecond):
			}
			close(release)
			retentionAwait(t, stopped)
			worker.Stop() // Repeated Stop and the deferred main hook are safe.
		})
	}
	var absent *accountBanRetentionRuntime
	absent.Stop() // Health-only startup never constructs a worker.
}

type retentionLogFixture struct{ records chan slog.Record }

func (h retentionLogFixture) Enabled(context.Context, slog.Level) bool { return true }
func (h retentionLogFixture) Handle(_ context.Context, r slog.Record) error {
	h.records <- r.Clone()
	return nil
}
func (h retentionLogFixture) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h retentionLogFixture) WithGroup(string) slog.Handler      { return h }

func TestAccountBanRetentionErrorsAreCoarseAndRetryNextTick(t *testing.T) {
	ticks := make(chan time.Time, 1)
	records := make(chan slog.Record, 4)
	finished := make(chan uuid.UUID, 4)
	var lists atomic.Int32
	first, second := uuid.New(), uuid.New()
	secret := "SELECT sensitive-profile-account-operation-body " + first.String()
	worker := startAccountBanRetentionWithTicks(context.Background(), retentionStoreFixture{
		list: func(context.Context, int) ([]uuid.UUID, error) {
			if lists.Add(1) == 1 {
				return nil, errors.New(secret)
			}
			return []uuid.UUID{first, second}, nil
		},
		clean: func(_ context.Context, id uuid.UUID, _ int) (int64, error) {
			finished <- id
			if id == first {
				return 0, errors.New(secret)
			}
			return 1, nil
		},
	}, slog.New(retentionLogFixture{records}), ticks, func() {})
	t.Cleanup(worker.Stop)
	check := func(r slog.Record) {
		require.Equal(t, "Space account Ban outcome retention cycle failed", r.Message)
		attrs := map[string]string{}
		r.Attrs(func(a slog.Attr) bool { attrs[a.Key] = a.Value.String(); return true })
		require.Equal(t, map[string]string{"failure": "database"}, attrs)
	}
	check(retentionAwait(t, records))
	require.Empty(t, finished, "selector failure aborts the cycle")
	ticks <- time.Now()
	require.Equal(t, first, retentionAwait(t, finished))
	require.Equal(t, second, retentionAwait(t, finished), "one Space failure permits the next hint inside the same cycle budget")
	check(retentionAwait(t, records))
	worker.Stop()
	require.Empty(t, records, "at most one warning per failed cycle")
}

func TestAccountBanRetentionWholeCycleDeadlineAndSilentShutdown(t *testing.T) {
	for _, shutdown := range []bool{false, true} {
		t.Run(map[bool]string{false: "deadline", true: "shutdown"}[shutdown], func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			records := make(chan slog.Record, 2)
			started := make(chan struct{})
			worker := startAccountBanRetentionWithTicks(parent, retentionStoreFixture{
				list: func(ctx context.Context, _ int) ([]uuid.UUID, error) {
					close(started)
					<-ctx.Done()
					return nil, ctx.Err()
				},
			}, slog.New(retentionLogFixture{records}), make(chan time.Time), func() {})
			retentionAwait(t, started)
			if shutdown {
				cancel()
				worker.Stop()
				require.Empty(t, records)
			} else {
				r := retentionAwait(t, records)
				var failure string
				r.Attrs(func(a slog.Attr) bool {
					if a.Key == "failure" {
						failure = a.Value.String()
					}
					return true
				})
				require.Equal(t, "deadline", failure)
				worker.Stop()
			}
		})
	}
}
