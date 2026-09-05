package grouping_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/grouping"
	"voice/backend/notification/internal/push"
)

func TestApplyToPayload_NilStoreStillSetsCounter(t *testing.T) {
	profileID := uuid.New()
	payload := push.Payload{Body: "Only"}
	require.NoError(t, grouping.ApplyToPayload(context.Background(), nil, profileID, "chat-x", "Only", &payload))
	require.Equal(t, 1, payload.Counter)
}

func TestApplyToPayload_IncrementsCounter(t *testing.T) {
	store := grouping.NewMemoryStore()
	profileID := uuid.New()
	chatID := uuid.NewString()
	payload := push.Payload{Body: "First"}
	require.NoError(t, grouping.ApplyToPayload(context.Background(), store, profileID, chatID, "First", &payload))
	require.Equal(t, 1, payload.Counter)
	require.NotEmpty(t, payload.CollapseTag)

	payload2 := push.Payload{Body: "Second"}
	require.NoError(t, grouping.ApplyToPayload(context.Background(), store, profileID, chatID, "Second", &payload2))
	require.Equal(t, 2, payload2.Counter)
	require.Equal(t, "Second", payload2.Body)
}

func TestApplyToPayload_ConcurrentCountersAreAtomicAndKeysAreIsolated(t *testing.T) {
	const notificationsPerChat = 16
	const chatCount = 2

	profileID := uuid.New()
	chats := []string{"chat-a", "chat-b"}
	store := newControllableStore(notificationsPerChat * chatCount)
	start := make(chan struct{})
	type result struct {
		chatID      string
		counter     int
		collapseTag string
		err         error
	}
	results := make(chan result, notificationsPerChat*chatCount)

	var wg sync.WaitGroup
	for _, chatID := range chats {
		for i := 0; i < notificationsPerChat; i++ {
			wg.Add(1)
			go func(chatID string, sequence int) {
				defer wg.Done()
				<-start
				payload := push.Payload{Body: fmt.Sprintf("%s-%d", chatID, sequence)}
				err := grouping.ApplyToPayload(context.Background(), store, profileID, chatID, payload.Body, &payload)
				results <- result{chatID: chatID, counter: payload.Counter, collapseTag: payload.CollapseTag, err: err}
			}(chatID, i)
		}
	}
	close(start)
	wg.Wait()
	close(results)

	countersByChat := make(map[string][]int, chatCount)
	tagsByChat := make(map[string]map[string]struct{}, chatCount)
	for got := range results {
		require.NoError(t, got.err)
		countersByChat[got.chatID] = append(countersByChat[got.chatID], got.counter)
		if tagsByChat[got.chatID] == nil {
			tagsByChat[got.chatID] = make(map[string]struct{})
		}
		tagsByChat[got.chatID][got.collapseTag] = struct{}{}
	}

	wantCounters := make([]int, notificationsPerChat)
	for i := range wantCounters {
		wantCounters[i] = i + 1
	}
	for _, chatID := range chats {
		gotCounters := countersByChat[chatID]
		sort.Ints(gotCounters)
		require.Equal(t, wantCounters, gotCounters, "each grouping key must assign every counter exactly once: %s", chatID)
		require.Equal(t, map[string]struct{}{delivery.GroupingKey(profileID, chatID): {}}, tagsByChat[chatID], "collapse identity must remain stable per chat")

		finalState, ok := store.state(chatID, profileID)
		require.True(t, ok)
		require.Equal(t, notificationsPerChat, finalState.Counter, "stored counters must not lose updates: %s", chatID)
	}
}

func TestApplyToPayload_PropagatesGetErrors(t *testing.T) {
	wantErr := errors.New("redis unavailable")
	store := &errorStore{getErr: wantErr}
	payload := push.Payload{Body: "before"}

	err := grouping.ApplyToPayload(context.Background(), store, uuid.New(), "chat", "after", &payload)

	require.ErrorIs(t, err, wantErr)
	require.Equal(t, "before", payload.Body, "payload must not be changed when grouping state cannot be read")
	require.Empty(t, store.setCalls)
}

func TestApplyToPayload_PropagatesSetErrors(t *testing.T) {
	wantErr := errors.New("redis write failed")
	store := &errorStore{setErr: wantErr}
	payload := push.Payload{Body: "before"}

	err := grouping.ApplyToPayload(context.Background(), store, uuid.New(), "chat", "after", &payload)

	require.ErrorIs(t, err, wantErr)
	require.Equal(t, "before", payload.Body, "payload must not be changed when grouping state cannot be stored")
}

func TestApplyToPayload_PropagatesAtomicUpdateErrors(t *testing.T) {
	wantErr := errors.New("redis atomic update failed")
	store := &errorStore{updateErr: wantErr}
	payload := push.Payload{Body: "before"}

	err := grouping.ApplyToPayload(context.Background(), store, uuid.New(), "chat", "after", &payload)

	require.ErrorIs(t, err, wantErr)
	require.Equal(t, "before", payload.Body, "payload must not be changed when the atomic update fails")
	require.Empty(t, store.setCalls)
}

func TestRedisStoreUpdateResetsGroupingTTLWithAtomicScript(t *testing.T) {
	intercept := &commandIntercept{err: errors.New("stop after command capture")}
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1})
	t.Cleanup(func() { require.NoError(t, rdb.Close()) })
	rdb.AddHook(intercept)

	profileID := uuid.New()
	key := delivery.GroupingKey(profileID, "chat-x")
	_, err := grouping.NewRedisStore(rdb).Update(context.Background(), key, "latest body")

	require.ErrorIs(t, err, intercept.err)
	require.Len(t, intercept.commands, 1)
	args := intercept.commands[0]
	require.Equal(t, "eval", args[0])
	require.Equal(t, "1", fmt.Sprint(args[2]))
	require.Equal(t, key, fmt.Sprint(args[3]))
	require.Equal(t, strconv.FormatInt(int64((24*time.Hour)/time.Second), 10), fmt.Sprint(args[len(args)-1]))
	require.Contains(t, fmt.Sprint(args[1]), "SET")
	require.True(t, strings.Contains(fmt.Sprint(args[1]), "EX") || strings.Contains(fmt.Sprint(args[1]), "'EX'"))
}

// controllableStore deliberately snapshots every read before allowing any write.
// This is a deterministic, race-free model of the current Redis Get+Set lost update.
// Update is the atomic seam expected by the production fix; the current code does
// not call it, so this RED test exposes the non-atomic fallback.
type controllableStore struct {
	mu          sync.Mutex
	data        map[string]delivery.GroupingState
	getCount    int
	totalGets   int
	readsReady  chan struct{}
	readsClosed bool
}

func newControllableStore(totalGets int) *controllableStore {
	return &controllableStore{
		data:       make(map[string]delivery.GroupingState),
		totalGets:  totalGets,
		readsReady: make(chan struct{}),
	}
}

func (s *controllableStore) Get(_ context.Context, key string) (*delivery.GroupingState, error) {
	s.mu.Lock()
	var snapshot *delivery.GroupingState
	if state, ok := s.data[key]; ok {
		copy := state
		snapshot = &copy
	}
	s.getCount++
	if s.getCount == s.totalGets && !s.readsClosed {
		close(s.readsReady)
		s.readsClosed = true
	}
	s.mu.Unlock()

	<-s.readsReady
	return snapshot, nil
}

func (s *controllableStore) Set(_ context.Context, key string, state delivery.GroupingState) error {
	s.mu.Lock()
	s.data[key] = state
	s.mu.Unlock()
	return nil
}

// Update is intentionally not part of the current Store interface. It gives the
// eventual implementation a test-only-compatible atomic transition seam without
// requiring Redis or a third-party test server.
func (s *controllableStore) Update(_ context.Context, key, body string) (delivery.GroupingState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var previous *delivery.GroupingState
	if state, ok := s.data[key]; ok {
		copy := state
		previous = &copy
	}
	next := delivery.NextGroupingState(key, previous, body)
	s.data[key] = next
	return next, nil
}

func (s *controllableStore) state(chatID string, profileID uuid.UUID) (delivery.GroupingState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.data[delivery.GroupingKey(profileID, chatID)]
	return state, ok
}

type errorStore struct {
	getErr    error
	setErr    error
	updateErr error
	setCalls  int
}

func (s *errorStore) Get(context.Context, string) (*delivery.GroupingState, error) {
	return nil, s.getErr
}

func (s *errorStore) Set(context.Context, string, delivery.GroupingState) error {
	s.setCalls++
	return s.setErr
}

func (s *errorStore) Update(context.Context, string, string) (delivery.GroupingState, error) {
	return delivery.GroupingState{}, s.updateErr
}

type commandIntercept struct {
	err      error
	commands [][]interface{}
}

func (h *commandIntercept) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h *commandIntercept) ProcessHook(_ redis.ProcessHook) redis.ProcessHook {
	return func(_ context.Context, cmd redis.Cmder) error {
		h.commands = append(h.commands, cmd.Args())
		return h.err
	}
}

func (h *commandIntercept) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
