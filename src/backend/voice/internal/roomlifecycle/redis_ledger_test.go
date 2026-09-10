package roomlifecycle

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRedisLedger_AtomicBeginAndCompletionSemantics(t *testing.T) {
	mr := miniredis.RunT(t)
	binding := newLedgerBinding(t, time.Now().UTC())
	runRedisCrossClientAtomicity(t, mr.Addr(), binding)
}

func TestRedisLedger_CompletionTTLIsAtLeast24HoursAndReplayDoesNotResetIt(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	ledger := NewRedisLedger(client)
	binding := newLedgerBinding(t, time.Now().UTC())
	reservation, _, err := ledger.Begin(context.Background(), binding)
	require.NoError(t, err)
	receipt := Receipt{Outcome: JoinSucceeded, VoiceSessionID: uuid.NewString()}
	require.NoError(t, ledger.Complete(context.Background(), reservation, receipt))

	key := redisLedgerKey(binding)
	require.Equal(t, "voice:lifecycle:op:"+binding.ActorProfileID()+":"+binding.OperationID(), key)
	initialTTL, err := client.PTTL(context.Background(), key).Result()
	require.NoError(t, err)
	require.GreaterOrEqual(t, initialTTL, completedReceiptRetention)

	mr.FastForward(23 * time.Hour)
	remainingBeforeReplay, err := client.PTTL(context.Background(), key).Result()
	require.NoError(t, err)
	_, replay, err := ledger.Begin(context.Background(), binding)
	require.NoError(t, err)
	require.Equal(t, &receipt, replay)
	remainingAfterReplay, err := client.PTTL(context.Background(), key).Result()
	require.NoError(t, err)
	require.Equal(t, remainingBeforeReplay, remainingAfterReplay, "completed replay must not reset retention")

	mr.FastForward(time.Hour + time.Millisecond)
	newReservation, replay, err := ledger.Begin(context.Background(), binding)
	require.NoError(t, err)
	require.Nil(t, replay)
	require.NotEmpty(t, newReservation.OwnerToken())
}

func TestRedisLedger_PendingHasNoTTLOrTimeBasedTakeover(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	ledger := NewRedisLedger(client)
	binding := newLedgerBinding(t, time.Now().UTC())
	reservation, _, err := ledger.Begin(context.Background(), binding)
	require.NoError(t, err)

	ttl, err := client.PTTL(context.Background(), redisLedgerKey(binding)).Result()
	require.NoError(t, err)
	require.Equal(t, time.Duration(-1), ttl)
	mr.FastForward(100 * 365 * 24 * time.Hour)
	_, _, err = ledger.Begin(context.Background(), binding)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.NoError(t, ledger.Complete(context.Background(), reservation, Receipt{Outcome: JoinSucceeded, VoiceSessionID: uuid.NewString()}))
}

func TestRedisLedger_OutageAndCorruptStateFailClosedWithoutFallback(t *testing.T) {
	t.Run("begin outage", func(t *testing.T) {
		mr := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: mr.Addr(), DialTimeout: 50 * time.Millisecond, ReadTimeout: 50 * time.Millisecond, WriteTimeout: 50 * time.Millisecond})
		require.NoError(t, client.Ping(context.Background()).Err())
		mr.Close()
		_, _, err := NewRedisLedger(client).Begin(context.Background(), newLedgerBinding(t, time.Now().UTC()))
		require.Equal(t, codes.Unavailable, status.Code(err))
	})

	t.Run("complete outage has no local fallback", func(t *testing.T) {
		mr := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: mr.Addr(), DialTimeout: 50 * time.Millisecond, ReadTimeout: 50 * time.Millisecond, WriteTimeout: 50 * time.Millisecond, MaxRetries: 0})
		t.Cleanup(func() { _ = client.Close() })
		ledger := NewRedisLedger(client)
		binding := newLedgerBinding(t, time.Now().UTC())
		reservation, _, err := ledger.Begin(context.Background(), binding)
		require.NoError(t, err)
		mr.Close()
		err = ledger.Complete(context.Background(), reservation, Receipt{Outcome: JoinSucceeded, VoiceSessionID: uuid.NewString()})
		require.Equal(t, codes.Unavailable, status.Code(err))
		_, replay, err := ledger.Begin(context.Background(), binding)
		require.Equal(t, codes.Unavailable, status.Code(err), "ledger must stay fail-closed throughout the outage")
		require.Nil(t, replay, "failed completion must not be served from a local fallback while Redis is down")
		require.NoError(t, mr.Restart())
		_, replay, err = ledger.Begin(context.Background(), binding)
		require.Equal(t, codes.FailedPrecondition, status.Code(err), "failed transport must not fabricate a local completion or release pending ownership")
		require.Nil(t, replay)
	})

	t.Run("corrupt begin record", func(t *testing.T) {
		mr := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { _ = client.Close() })
		binding := newLedgerBinding(t, time.Now().UTC())
		require.NoError(t, client.Set(context.Background(), redisLedgerKey(binding), "corrupt", 0).Err())
		_, _, err := NewRedisLedger(client).Begin(context.Background(), binding)
		require.Equal(t, codes.Unavailable, status.Code(err))
	})

	t.Run("corrupt completion record", func(t *testing.T) {
		mr := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { _ = client.Close() })
		ledger := NewRedisLedger(client)
		binding := newLedgerBinding(t, time.Now().UTC())
		reservation, _, err := ledger.Begin(context.Background(), binding)
		require.NoError(t, err)
		require.NoError(t, client.Set(context.Background(), redisLedgerKey(binding), "corrupt", 0).Err())
		err = ledger.Complete(context.Background(), reservation, Receipt{Outcome: JoinSucceeded, VoiceSessionID: uuid.NewString()})
		require.Equal(t, codes.Unavailable, status.Code(err))
	})
}

func TestRedisLedger_StructurallyCompleteMalformedRecordsFailClosed(t *testing.T) {
	validOwner := strings.Repeat("a", 64)
	validSessionID := uuid.NewString()
	tests := []struct {
		name        string
		state       string
		method      string
		fingerprint string
		owner       string
		outcome     string
		sessionID   string
	}{
		{"short owner", "pending", string(JoinMethod), "binding", "x", "", ""},
		{"unsupported method", "pending", "DELETE", "binding", validOwner, "", ""},
		{"malformed fingerprint", "pending", string(JoinMethod), "sha256:bad", validOwner, "", ""},
		{"unsupported completed outcome", "completed", string(JoinMethod), "binding", validOwner, "joined", validSessionID},
		{"malformed completed session", "completed", string(JoinMethod), "binding", validOwner, string(JoinSucceeded), "not-a-uuid"},
		{"nil completed session", "completed", string(JoinMethod), "binding", validOwner, string(JoinSucceeded), uuid.Nil.String()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			t.Cleanup(func() { _ = client.Close() })
			binding := newLedgerBinding(t, time.Now().UTC())
			fingerprint := tc.fingerprint
			if fingerprint == "binding" {
				fingerprint = binding.Fingerprint()
			}
			fields := map[string]any{
				"state": tc.state, "method": tc.method, "fingerprint": fingerprint, "owner": tc.owner,
			}
			if tc.state == "completed" {
				fields["receipt_outcome"] = tc.outcome
				fields["receipt_session_id"] = tc.sessionID
			}
			require.NoError(t, client.HSet(context.Background(), redisLedgerKey(binding), fields).Err())
			if tc.state == "completed" {
				require.NoError(t, client.PExpire(context.Background(), redisLedgerKey(binding), completedReceiptRetention).Err())
			}
			_, replay, err := NewRedisLedger(client).Begin(context.Background(), binding)
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Nil(t, replay)
		})
	}
}

func TestRedisLedger_RealServerAtomicity(t *testing.T) {
	address := os.Getenv("VOICE_R18_REDIS_TEST_ADDR")
	if address == "" {
		t.Skip("VOICE_R18_REDIS_TEST_ADDR is required for isolated real Redis evidence")
	}
	binding := newLedgerBinding(t, time.Now().UTC())
	runRedisCrossClientAtomicity(t, address, binding)
	client := redis.NewClient(&redis.Options{Addr: address})
	t.Cleanup(func() { _ = client.Close() })
	ttl, err := client.PTTL(context.Background(), redisLedgerKey(binding)).Result()
	require.NoError(t, err)
	require.GreaterOrEqual(t, ttl, completedReceiptRetention-time.Second)
}

func newRedisTestLedger(t *testing.T) ledgerUnderTest {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisLedger(client)
}

type redisCommandRecorder struct {
	mu    sync.Mutex
	names []string
}

func (r *redisCommandRecorder) DialHook(next redis.DialHook) redis.DialHook { return next }
func (r *redisCommandRecorder) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		r.mu.Lock()
		r.names = append(r.names, strings.ToLower(cmd.Name()))
		r.mu.Unlock()
		return next(ctx, cmd)
	}
}
func (r *redisCommandRecorder) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, commands []redis.Cmder) error {
		r.mu.Lock()
		for _, command := range commands {
			r.names = append(r.names, strings.ToLower(command.Name()))
		}
		r.mu.Unlock()
		return next(ctx, commands)
	}
}

func (r *redisCommandRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.names...)
}

func runRedisCrossClientAtomicity(t *testing.T, address string, binding Binding) {
	t.Helper()
	const callers = 32
	newClient := func(recorder *redisCommandRecorder) *redis.Client {
		client := redis.NewClient(&redis.Options{Addr: address})
		client.AddHook(recorder)
		return client
	}

	start := make(chan struct{})
	beginResults := make(chan struct {
		reservation Reservation
		err         error
		commands    []string
	}, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			recorder := &redisCommandRecorder{}
			client := newClient(recorder)
			defer client.Close()
			reservation, _, err := NewRedisLedger(client).Begin(context.Background(), binding)
			beginResults <- struct {
				reservation Reservation
				err         error
				commands    []string
			}{reservation, err, recorder.snapshot()}
		}()
	}
	close(start)
	wg.Wait()
	close(beginResults)

	var reservation Reservation
	var beginSuccesses int
	for result := range beginResults {
		requireOnlyLuaLedgerCall(t, result.commands)
		if result.err == nil {
			beginSuccesses++
			reservation = result.reservation
			continue
		}
		require.Equal(t, codes.FailedPrecondition, status.Code(result.err))
	}
	require.Equal(t, 1, beginSuccesses)
	requireOwnerToken(t, reservation.OwnerToken())

	completeStart := make(chan struct{})
	completeResults := make(chan struct {
		receipt  Receipt
		err      error
		commands []string
	}, callers)
	for i := range callers {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-completeStart
			recorder := &redisCommandRecorder{}
			client := newClient(recorder)
			defer client.Close()
			receipt := Receipt{Outcome: JoinSucceeded, VoiceSessionID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(binding.OperationID()+string(rune(index)))).String()}
			err := NewRedisLedger(client).Complete(context.Background(), reservation, receipt)
			completeResults <- struct {
				receipt  Receipt
				err      error
				commands []string
			}{receipt, err, recorder.snapshot()}
		}(i)
	}
	close(completeStart)
	wg.Wait()
	close(completeResults)

	var durableReceipt Receipt
	var completeSuccesses int
	for result := range completeResults {
		requireOnlyLuaLedgerCall(t, result.commands)
		if result.err == nil {
			completeSuccesses++
			durableReceipt = result.receipt
			continue
		}
		require.Equal(t, codes.AlreadyExists, status.Code(result.err))
	}
	require.Equal(t, 1, completeSuccesses, "conflicting completions through independent clients need one durable winner")

	replayRecorder := &redisCommandRecorder{}
	client := newClient(replayRecorder)
	defer client.Close()
	_, replay, err := NewRedisLedger(client).Begin(context.Background(), binding)
	require.NoError(t, err)
	require.Equal(t, &durableReceipt, replay)
	requireOnlyLuaLedgerCall(t, replayRecorder.snapshot())
}

func requireOnlyLuaLedgerCall(t *testing.T, commands []string) {
	t.Helper()
	var scriptCalls int
	for _, command := range commands {
		switch command {
		case "eval", "evalsha":
			scriptCalls++
		case "hello", "client", "auth", "select":
			// Connection setup only; these commands cannot read or mutate ledger records.
		default:
			t.Fatalf("non-Lua Redis ledger command %q observed; every data transition must be one server-side script", command)
		}
	}
	require.GreaterOrEqual(t, scriptCalls, 1, "each Begin or Complete attempt must use server-side Lua")
}
