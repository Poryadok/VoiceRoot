package grouping

import (
	"context"
	"encoding/json"
	"fmt"
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
}

// MemoryStore is an in-memory grouping store for tests.
type MemoryStore struct {
	data map[string]delivery.GroupingState
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{data: make(map[string]delivery.GroupingState)}
}

func (s *MemoryStore) Get(_ context.Context, key string) (*delivery.GroupingState, error) {
	if s == nil {
		return nil, nil
	}
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
	s.data[key] = state
	return nil
}

// RedisStore stores grouping state in Redis.
type RedisStore struct {
	rdb *redis.Client
}

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
	var prev *delivery.GroupingState
	if store != nil {
		got, err := store.Get(ctx, key)
		if err != nil {
			return err
		}
		prev = got
	}
	next := delivery.NextGroupingState(key, prev, previewBody)
	if store != nil {
		if err := store.Set(ctx, key, next); err != nil {
			return err
		}
	}
	payload.CollapseTag = next.CollapseTag
	payload.Counter = next.Counter
	if previewBody != "" {
		payload.Body = next.LastBody
	}
	return nil
}
