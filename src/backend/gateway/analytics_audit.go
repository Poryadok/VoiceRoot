package main

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const analyticsAuditRedisKey = "gateway:analytics:audit"
const analyticsAuditRedisMax = 200

type analyticsAuditEntry struct {
	At        time.Time `json:"at"`
	Route     string    `json:"route"`
	Method    string    `json:"method"`
	ProfileID string    `json:"profile_id"`
	UserID    string    `json:"user_id"`
}

type analyticsAuditStore interface {
	Append(ctx context.Context, entry analyticsAuditEntry) error
	Recent(ctx context.Context, limit int) ([]analyticsAuditEntry, error)
}

type memoryAnalyticsAuditStore struct {
	mu      sync.Mutex
	entries []analyticsAuditEntry
	max     int
}

func newMemoryAnalyticsAuditStore(max int) *memoryAnalyticsAuditStore {
	if max <= 0 {
		max = 100
	}
	return &memoryAnalyticsAuditStore{max: max}
}

func (s *memoryAnalyticsAuditStore) Append(_ context.Context, entry analyticsAuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entry)
	if len(s.entries) > s.max {
		s.entries = s.entries[len(s.entries)-s.max:]
	}
	return nil
}

func (s *memoryAnalyticsAuditStore) Recent(_ context.Context, limit int) ([]analyticsAuditEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || limit > len(s.entries) {
		limit = len(s.entries)
	}
	out := make([]analyticsAuditEntry, limit)
	copy(out, s.entries[len(s.entries)-limit:])
	return out, nil
}

type redisAnalyticsAuditStore struct {
	client *redis.Client
	key    string
	max    int64
}

func newRedisAnalyticsAuditStore(addr, password string) *redisAnalyticsAuditStore {
	return &redisAnalyticsAuditStore{
		client: redis.NewClient(&redis.Options{Addr: addr, Password: password}),
		key:    analyticsAuditRedisKey,
		max:    analyticsAuditRedisMax,
	}
}

func (s *redisAnalyticsAuditStore) Append(ctx context.Context, entry analyticsAuditEntry) error {
	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	pipe.LPush(ctx, s.key, raw)
	pipe.LTrim(ctx, s.key, 0, s.max-1)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *redisAnalyticsAuditStore) Recent(ctx context.Context, limit int) ([]analyticsAuditEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	raws, err := s.client.LRange(ctx, s.key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]analyticsAuditEntry, 0, len(raws))
	for _, raw := range raws {
		var e analyticsAuditEntry
		if err := json.Unmarshal([]byte(raw), &e); err != nil {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}
