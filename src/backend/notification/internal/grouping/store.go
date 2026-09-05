package grouping

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/push"
)

const groupingTTL = 24 * time.Hour

// Store persists collapsed push metadata per chat.
type Store interface {
	Get(ctx context.Context, key string) (*delivery.GroupingState, error)
	Set(ctx context.Context, key string, state delivery.GroupingState) error
	Update(ctx context.Context, key, body string) (delivery.GroupingState, error)
}

// MemoryStore is an in-memory grouping store for tests.
type MemoryStore struct {
	mu   sync.Mutex
	data map[string]delivery.GroupingState
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{data: make(map[string]delivery.GroupingState)}
}

func (s *MemoryStore) Get(_ context.Context, key string) (*delivery.GroupingState, error) {
	if s == nil {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.data[key]
	if !ok {
		return nil, nil
	}
	copy := st
	return &copy, nil
}

func (s *MemoryStore) Set(_ context.Context, key string, state delivery.GroupingState) error {
	if s == nil {
		return fmt.Errorf("grouping store unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = state
	return nil
}

// Update atomically advances one in-memory grouping state for tests.
func (s *MemoryStore) Update(_ context.Context, key, body string) (delivery.GroupingState, error) {
	if s == nil {
		return delivery.GroupingState{}, fmt.Errorf("grouping store unavailable")
	}
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

// RedisStore stores grouping state in Redis.
type RedisStore struct {
	rdb *redis.Client
}

const redisGroupingUpdateScript = `
local previous = redis.call('GET', KEYS[1])
local next = {CollapseTag = KEYS[1], Counter = 1, LastBody = ARGV[1]}
if previous then
  local state = cjson.decode(previous)
  if state.CollapseTag and state.CollapseTag ~= '' then
    next.CollapseTag = state.CollapseTag
  end
  if state.Counter then
    next.Counter = tonumber(state.Counter) + 1
  end
end
redis.call('SET', KEYS[1], cjson.encode(next), 'EX', ARGV[2])
return cjson.encode(next)
`

func NewRedisStore(rdb *redis.Client) *RedisStore {
	return &RedisStore{rdb: rdb}
}

func (s *RedisStore) Get(ctx context.Context, key string) (*delivery.GroupingState, error) {
	if s == nil || s.rdb == nil {
		return nil, nil
	}
	raw, err := s.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var st delivery.GroupingState
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *RedisStore) Set(ctx context.Context, key string, state delivery.GroupingState) error {
	if s == nil || s.rdb == nil {
		return fmt.Errorf("grouping redis unavailable")
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, key, raw, groupingTTL).Err()
}

// Update advances a grouping state in one Redis script invocation. SET EX keeps
// the established 24-hour sliding grouping window on every successful update.
func (s *RedisStore) Update(ctx context.Context, key, body string) (delivery.GroupingState, error) {
	if s == nil || s.rdb == nil {
		return delivery.GroupingState{}, fmt.Errorf("grouping redis unavailable")
	}
	raw, err := s.rdb.Eval(
		ctx,
		redisGroupingUpdateScript,
		[]string{key},
		body,
		strconv.FormatInt(int64(groupingTTL/time.Second), 10),
	).Text()
	if err != nil {
		return delivery.GroupingState{}, err
	}
	var state delivery.GroupingState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return delivery.GroupingState{}, err
	}
	return state, nil
}

// ApplyToPayload updates collapse tag, counter, and body for chat-grouped pushes.
func ApplyToPayload(
	ctx context.Context,
	store Store,
	profileID uuid.UUID,
	chatID, previewBody string,
	payload *push.Payload,
) error {
	if payload == nil {
		return fmt.Errorf("grouping: nil payload")
	}
	key := delivery.GroupingKey(profileID, chatID)
	var next delivery.GroupingState
	if store != nil {
		got, err := store.Update(ctx, key, previewBody)
		if err != nil {
			return err
		}
		next = got
	} else {
		next = delivery.NextGroupingState(key, nil, previewBody)
	}
	payload.CollapseTag = next.CollapseTag
	payload.Counter = next.Counter
	if previewBody != "" {
		payload.Body = next.LastBody
	}
	return nil
}
