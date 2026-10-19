package grpcclient

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestUnaryBreakerRejectsUnavailableMethodWithoutReplaying(t *testing.T) {
	config := DefaultBreakerConfig()
	config.FailureThreshold = 2
	breaker, err := NewUnaryBreaker(config)
	require.NoError(t, err)
	now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	breaker.now = func() time.Time { return now }
	intercept := breaker.UnaryClientInterceptor()
	hits := map[string]int{}
	invoke := func(_ context.Context, method string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		hits[method]++
		if method == "/test.Service/Unavailable" {
			return status.Error(codes.Unavailable, "upstream unavailable")
		}
		return nil
	}
	call := func(method string) error {
		return intercept(context.Background(), method, nil, nil, nil, invoke)
	}
	for range 2 {
		require.Equal(t, codes.Unavailable, status.Code(call("/test.Service/Unavailable")))
	}
	require.Equal(t, 2, hits["/test.Service/Unavailable"], "one invoker call per admitted logical call")
	require.Equal(t, codes.Unavailable, status.Code(call("/test.Service/Unavailable")))
	require.Equal(t, 2, hits["/test.Service/Unavailable"], "OPEN must reject before the invoker")
	require.NoError(t, call("/test.Service/Healthy"))
	require.Equal(t, 1, hits["/test.Service/Healthy"], "a distinct method must remain independent")

	now = now.Add(config.OpenTimeout)
	require.Equal(t, codes.Unavailable, status.Code(call("/test.Service/Unavailable")))
	require.Equal(t, 3, hits["/test.Service/Unavailable"], "cooldown admits one real probe")
	require.Equal(t, codes.Unavailable, status.Code(call("/test.Service/Unavailable")))
	require.Equal(t, 3, hits["/test.Service/Unavailable"], "a failed probe must reopen")
}

type breakerClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *breakerClock) Now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *breakerClock) Add(d time.Duration) { c.mu.Lock(); c.now = c.now.Add(d); c.mu.Unlock() }

func testUnaryBreaker(t *testing.T, config BreakerConfig) (*UnaryBreaker, *breakerClock) {
	t.Helper()
	b, err := NewUnaryBreaker(config)
	require.NoError(t, err)
	clock := &breakerClock{now: time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)}
	b.now = clock.Now
	return b, clock
}

func invokeUnary(b *UnaryBreaker, ctx context.Context, method string, invoke func(context.Context) error) error {
	return b.UnaryClientInterceptor()(ctx, method, nil, nil, nil,
		func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			return invoke(ctx)
		})
}

// A pre-lock sample may be delayed behind another operation for arbitrarily
// long. Model that schedule without sleeps: only a sample serialized with the
// state sees the current time; an earlier sample sees the old time.
func TestUnaryBreakerTimeAndStateShareLinearization(t *testing.T) {
	for _, scenario := range []string{"failure_window", "full_cooldown", "probe_admission"} {
		t.Run(scenario, func(t *testing.T) {
			config := DefaultBreakerConfig()
			b, clock := testUnaryBreaker(t, config)
			start := clock.Now()
			entry := &methodCircuit{state: breakerClosed, outstanding: 1,
				failures: config.FailureThreshold - 1, firstFailure: start, lastActivity: start}
			b.methods["/test/Timed"] = entry
			current := start.Add(40 * time.Second)
			b.now = func() time.Time {
				if b.mu.TryLock() {
					b.mu.Unlock()
					return start
				}
				return current
			}
			switch scenario {
			case "failure_window":
				b.complete(breakerAdmission{method: "/test/Timed", entry: entry}, breakerFailure)
				require.Equal(t, breakerClosed, entry.state, "a delayed sample must not join an expired failure window")
				require.Equal(t, 1, entry.failures)
				require.Equal(t, current, entry.firstFailure)
			case "full_cooldown":
				entry.firstFailure = current
				b.complete(breakerAdmission{method: "/test/Timed", entry: entry}, breakerFailure)
				require.Equal(t, breakerOpen, entry.state)
				require.Equal(t, current.Add(config.OpenTimeout), entry.openUntil, "cooldown starts at the serialized transition")
			case "probe_admission":
				entry.state = breakerOpen
				entry.outstanding = 0
				entry.openUntil = start.Add(config.OpenTimeout)
				token, event := b.admit("/test/Timed")
				require.Same(t, entry, token.entry, "a stale admission timestamp must not delay an eligible probe")
				require.Equal(t, "probe", event.Reason)
				require.Equal(t, current, entry.lastActivity)
			}
		})
	}
}

func TestUnaryBreakerClassifiesStatusesWithoutReplaying(t *testing.T) {
	for code := codes.OK; code <= codes.Unauthenticated; code++ {
		t.Run(code.String(), func(t *testing.T) {
			config := DefaultBreakerConfig()
			config.FailureThreshold = 1
			b, _ := testUnaryBreaker(t, config)
			original := status.Error(code, "unchanged upstream result")
			hits := 0
			got := invokeUnary(b, context.Background(), "/test/Mutation", func(context.Context) error { hits++; return original })
			if original == nil {
				require.NoError(t, got)
			} else {
				require.Same(t, original, got)
			}
			require.Equal(t, 1, hits, "no application replay, including ambiguous mutation failure")
			got = invokeUnary(b, context.Background(), "/test/Mutation", func(context.Context) error { hits++; return nil })
			if code == codes.Unavailable || code == codes.DeadlineExceeded {
				require.Equal(t, codes.Unavailable, status.Code(got))
				require.Equal(t, 1, hits)
			} else {
				require.NoError(t, got)
				require.Equal(t, 2, hits)
			}
		})
	}
	config := DefaultBreakerConfig()
	config.FailureThreshold = 1
	b, _ := testUnaryBreaker(t, config)
	original := errors.New("non-status errors are not classified as transport failure")
	require.Same(t, original, invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { return original }))
	require.NoError(t, invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { return nil }))
}

func TestUnaryBreakerCallerCancellationDoesNotAllocateOrTrip(t *testing.T) {
	config := DefaultBreakerConfig()
	config.FailureThreshold, config.MaxMethods = 1, 1
	b, _ := testUnaryBreaker(t, config)
	for _, expired := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		if expired {
			cancel()
			ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		} else {
			cancel()
		}
		err := invokeUnary(b, ctx, "/test/AlreadyCanceled", func(context.Context) error { t.Error("canceled invocation reached upstream"); return nil })
		cancel()
		require.Equal(t, status.Code(status.FromContextError(ctx.Err()).Err()), status.Code(err))
	}
	ctx, cancel := context.WithCancel(context.Background())
	original := status.Error(codes.Unavailable, "ambiguous cancellation race")
	require.Same(t, original, invokeUnary(b, ctx, "/test/Real", func(context.Context) error { cancel(); return original }))
	require.NoError(t, invokeUnary(b, context.Background(), "/test/Real", func(context.Context) error { return nil }))
}

func TestUnaryBreakerFailureWindowSuccessAndNeutral(t *testing.T) {
	for _, mode := range []string{"neutral-preserves", "success-resets", "window-equality-resets"} {
		t.Run(mode, func(t *testing.T) {
			config := DefaultBreakerConfig()
			config.FailureThreshold = 2
			b, clock := testUnaryBreaker(t, config)
			failure := status.Error(codes.Unavailable, "unavailable")
			fail := func() error {
				return invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { return failure })
			}
			require.Same(t, failure, fail())
			switch mode {
			case "neutral-preserves":
				denied := status.Error(codes.PermissionDenied, "denied")
				require.Same(t, denied, invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { return denied }))
			case "success-resets":
				require.NoError(t, invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { return nil }))
			case "window-equality-resets":
				clock.Add(config.FailureWindow)
			}
			require.Same(t, failure, fail())
			hits := 0
			err := invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { hits++; return nil })
			if mode == "neutral-preserves" {
				require.Equal(t, codes.Unavailable, status.Code(err))
				require.Zero(t, hits)
			} else {
				require.NoError(t, err)
				require.Equal(t, 1, hits)
			}
		})
	}
}

func waitUnaryResult(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("breaker did not release invocation")
		return nil
	}
}

func TestUnaryBreakerOneHalfOpenProbeAndStaleCompletions(t *testing.T) {
	for _, staleSuccess := range []bool{false, true} {
		t.Run(map[bool]string{false: "failure", true: "success"}[staleSuccess], func(t *testing.T) {
			config := DefaultBreakerConfig()
			config.FailureThreshold = 1
			b, clock := testUnaryBreaker(t, config)
			oldStarted, releaseOld := make(chan struct{}), make(chan struct{})
			oldDone := make(chan error, 1)
			go func() {
				oldDone <- invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error {
					close(oldStarted)
					<-releaseOld
					if staleSuccess {
						return nil
					}
					return status.Error(codes.Unavailable, "stale")
				})
			}()
			<-oldStarted
			require.Error(t, invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { return status.Error(codes.Unavailable, "trip") }))
			clock.Add(config.OpenTimeout)
			probeStarted, releaseProbe := make(chan struct{}), make(chan struct{})
			probeDone := make(chan error, 1)
			go func() {
				probeDone <- invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { close(probeStarted); <-releaseProbe; return nil })
			}()
			<-probeStarted
			var group sync.WaitGroup
			var extra atomic.Int64
			for range 20 {
				group.Add(1)
				go func() {
					defer group.Done()
					err := invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { extra.Add(1); return nil })
					if status.Code(err) != codes.Unavailable {
						t.Errorf("concurrent probe admitted: %v", err)
					}
				}()
			}
			group.Wait()
			require.Zero(t, extra.Load())
			close(releaseOld)
			_ = waitUnaryResult(t, oldDone)
			require.Equal(t, codes.Unavailable, status.Code(invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { t.Error("stale completion released current probe"); return nil })))
			close(releaseProbe)
			require.NoError(t, waitUnaryResult(t, probeDone))
			require.NoError(t, invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { return nil }))
		})
	}
}

func TestUnaryBreakerNeutralAndPanickingProbeRelease(t *testing.T) {
	for _, panics := range []bool{false, true} {
		t.Run(map[bool]string{false: "neutral", true: "panic"}[panics], func(t *testing.T) {
			config := DefaultBreakerConfig()
			config.FailureThreshold = 1
			b, clock := testUnaryBreaker(t, config)
			require.Error(t, invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { return status.Error(codes.Unavailable, "trip") }))
			clock.Add(config.OpenTimeout)
			probe := func() {
				_ = invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error {
					if panics {
						panic("caller panic")
					}
					return status.Error(codes.PermissionDenied, "not recovery")
				})
			}
			if panics {
				require.PanicsWithValue(t, "caller panic", probe)
			} else {
				probe()
			}
			require.Equal(t, codes.Unavailable, status.Code(invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { t.Error("neutral probe incorrectly recovered"); return nil })))
			clock.Add(config.OpenTimeout)
			require.NoError(t, invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { return nil }))
		})
	}
}

func TestUnaryBreakerCapacityIdleEvictionAndHealthBypass(t *testing.T) {
	config := DefaultBreakerConfig()
	config.FailureThreshold, config.MaxMethods = 1, 1
	b, clock := testUnaryBreaker(t, config)
	for _, method := range []string{"/grpc.health.v1.Health/Check", "/grpc.health.v1.Health/List"} {
		require.Error(t, invokeUnary(b, context.Background(), method, func(context.Context) error { return status.Error(codes.Unavailable, "health unavailable") }))
	}
	require.NoError(t, invokeUnary(b, context.Background(), "/test/First", func(context.Context) error { return nil }))
	require.Equal(t, codes.Unavailable, status.Code(invokeUnary(b, context.Background(), "/test/New", func(context.Context) error { t.Error("capacity bypassed"); return nil })))
	clock.Add(config.IdleTimeout)
	require.NoError(t, invokeUnary(b, context.Background(), "/test/New", func(context.Context) error { return nil }))
	require.Error(t, invokeUnary(b, context.Background(), "/test/New", func(context.Context) error { return status.Error(codes.Unavailable, "trip") }))
	clock.Add(config.IdleTimeout)
	require.Equal(t, codes.Unavailable, status.Code(invokeUnary(b, context.Background(), "/test/Another", func(context.Context) error { t.Error("OPEN entry evicted"); return nil })))
	other, _ := testUnaryBreaker(t, config)
	require.NoError(t, invokeUnary(other, context.Background(), "/test/New", func(context.Context) error { return nil }))
}

func TestUnaryBreakerConfigurationBoundsAndImmutableCopy(t *testing.T) {
	for _, change := range []func(*BreakerConfig){
		func(c *BreakerConfig) { c.FailureThreshold = 0 }, func(c *BreakerConfig) { c.FailureThreshold = 101 },
		func(c *BreakerConfig) { c.FailureWindow = 0 }, func(c *BreakerConfig) { c.FailureWindow = 5*time.Minute + 1 },
		func(c *BreakerConfig) { c.OpenTimeout = 0 }, func(c *BreakerConfig) { c.OpenTimeout = time.Minute + 1 },
		func(c *BreakerConfig) { c.MaxMethods = 0 }, func(c *BreakerConfig) { c.MaxMethods = 4097 },
		func(c *BreakerConfig) { c.IdleTimeout = 0 }, func(c *BreakerConfig) { c.IdleTimeout = time.Hour + 1 },
	} {
		config := DefaultBreakerConfig()
		change(&config)
		_, err := NewUnaryBreaker(config)
		require.Error(t, err)
	}
	config := DefaultBreakerConfig()
	config.FailureThreshold = 2
	b, _ := testUnaryBreaker(t, config)
	config.FailureThreshold = 1
	admitted := 0
	for range 2 {
		require.Error(t, invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { admitted++; return status.Error(codes.Unavailable, "failure") }))
	}
	require.Equal(t, 2, admitted, "changing the caller's config must not change the constructed breaker")
	hits := 0
	require.Equal(t, codes.Unavailable, status.Code(invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { hits++; return nil })))
	require.Zero(t, hits)
}

func TestUnaryBreakerPreservesContextOptionsReplyAndError(t *testing.T) {
	b, _ := testUnaryBreaker(t, DefaultBreakerConfig())
	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-test", "authority-unchanged")), time.Second)
	defer cancel()
	request, reply := &struct{ Value string }{"request"}, &struct{ Value string }{}
	original := status.Error(codes.PermissionDenied, "exact original result")
	option := grpc.WaitForReady(false)
	err := b.UnaryClientInterceptor()(ctx, "/test/Method", request, reply, nil, func(got context.Context, method string, req, res any, _ *grpc.ClientConn, opts ...grpc.CallOption) error {
		require.Same(t, ctx, got)
		require.Equal(t, "/test/Method", method)
		require.Same(t, request, req)
		require.Same(t, reply, res)
		require.Equal(t, []grpc.CallOption{option}, opts)
		md, _ := metadata.FromOutgoingContext(got)
		require.Equal(t, []string{"authority-unchanged"}, md.Get("x-test"))
		reply.Value = "upstream response"
		return original
	}, option)
	require.Same(t, original, err)
	require.Equal(t, "upstream response", reply.Value)
}

func TestUnaryBreakerOutstandingClosedEntryCannotBeEvicted(t *testing.T) {
	config := DefaultBreakerConfig()
	config.MaxMethods = 1
	b, clock := testUnaryBreaker(t, config)
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- invokeUnary(b, context.Background(), "/test/Old", func(context.Context) error { close(started); <-release; return nil })
	}()
	<-started
	clock.Add(config.IdleTimeout)
	require.Equal(t, codes.Unavailable, status.Code(invokeUnary(b, context.Background(), "/test/New", func(context.Context) error { t.Error("outstanding entry evicted"); return nil })))
	close(release)
	require.NoError(t, waitUnaryResult(t, done))
	clock.Add(config.IdleTimeout)
	require.NoError(t, invokeUnary(b, context.Background(), "/test/New", func(context.Context) error { return nil }))
}

func TestUnaryBreakerStaleCompletionCannotChangeOpenDeadlineOrRecoveredState(t *testing.T) {
	for _, phase := range []string{"open", "recovered"} {
		for _, oldSuccess := range []bool{false, true} {
			t.Run(phase+map[bool]string{false: "/failure", true: "/success"}[oldSuccess], func(t *testing.T) {
				config := DefaultBreakerConfig()
				config.FailureThreshold = 1
				b, clock := testUnaryBreaker(t, config)
				started, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
				go func() {
					done <- invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error {
						close(started)
						<-release
						if oldSuccess {
							return nil
						}
						return status.Error(codes.Unavailable, "old failure")
					})
				}()
				<-started
				require.Error(t, invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { return status.Error(codes.Unavailable, "trip") }))
				if phase == "recovered" {
					clock.Add(config.OpenTimeout)
					require.NoError(t, invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { return nil }))
				} else {
					clock.Add(config.OpenTimeout - time.Second)
				}
				close(release)
				_ = waitUnaryResult(t, done)
				if phase == "open" {
					require.Equal(t, codes.Unavailable, status.Code(invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { t.Error("stale success closed OPEN"); return nil })))
					clock.Add(time.Second)
				}
				require.NoError(t, invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { return nil }), "old failure cannot extend OPEN or reopen recovered state")
			})
		}
	}
}

func TestUnaryBreakerCanceledNilProbeIsNotRecovery(t *testing.T) {
	config := DefaultBreakerConfig()
	config.FailureThreshold = 1
	b, clock := testUnaryBreaker(t, config)
	require.Error(t, invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { return status.Error(codes.Unavailable, "trip") }))
	clock.Add(config.OpenTimeout)
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, invokeUnary(b, ctx, "/test/Method", func(context.Context) error { cancel(); return nil }))
	require.Equal(t, codes.Unavailable, status.Code(invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { t.Error("canceled nil response claimed recovery"); return nil })))
	clock.Add(config.OpenTimeout)
	require.NoError(t, invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { return nil }))
}

func TestUnaryBreakerObservationRunsOutsideStateLock(t *testing.T) {
	config := DefaultBreakerConfig()
	config.FailureThreshold = 1
	var b *UnaryBreaker
	var observations atomic.Int64
	config.Observe = func(event BreakerEvent) {
		observations.Add(1)
		require.Equal(t, "/test/Method", event.Method)
		require.NoError(t, invokeUnary(b, context.Background(), "/test/Healthy", func(context.Context) error { return nil }))
	}
	b, _ = testUnaryBreaker(t, config)
	done := make(chan error, 1)
	go func() {
		done <- invokeUnary(b, context.Background(), "/test/Method", func(context.Context) error { return status.Error(codes.Unavailable, "trip") })
	}()
	require.Error(t, waitUnaryResult(t, done))
	require.EqualValues(t, 1, observations.Load())
}
