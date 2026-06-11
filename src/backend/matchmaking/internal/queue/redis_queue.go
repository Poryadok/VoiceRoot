package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrQueueUnavailable = errors.New("queue unavailable")
	ErrLockHeld         = errors.New("active search lock held")
	ErrNotEnqueued      = errors.New("session not in queue")
)

const defaultLockTTL = 31 * time.Minute

// RedisQueue manages FIFO MM queues and per-profile active-search locks.
type RedisQueue struct {
	Client *redis.Client
	Prefix string
}

func (q *RedisQueue) queueKey(gameID uuid.UUID, mode, region string) string {
	p := q.Prefix
	if p == "" {
		p = "mm"
	}
	return fmt.Sprintf("%s:queue:%s:%s:%s", p, gameID.String(), mode, region)
}

func (q *RedisQueue) lockKey(profileID uuid.UUID) string {
	p := q.Prefix
	if p == "" {
		p = "mm"
	}
	return fmt.Sprintf("%s:lock:profile:%s", p, profileID.String())
}

// AcquireLock sets the active-search lock for profileID to sessionID.
func (q *RedisQueue) AcquireLock(ctx context.Context, profileID, sessionID uuid.UUID) error {
	if q == nil || q.Client == nil {
		return ErrQueueUnavailable
	}
	ok, err := q.Client.SetNX(ctx, q.lockKey(profileID), sessionID.String(), defaultLockTTL).Result()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
	}
	if !ok {
		return ErrLockHeld
	}
	return nil
}

// ReleaseLock removes the active-search lock when it points to sessionID.
func (q *RedisQueue) ReleaseLock(ctx context.Context, profileID, sessionID uuid.UUID) error {
	if q == nil || q.Client == nil {
		return ErrQueueUnavailable
	}
	key := q.lockKey(profileID)
	current, err := q.Client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
	}
	if current != sessionID.String() {
		return nil
	}
	return q.Client.Del(ctx, key).Err()
}

// Enqueue adds sessionID to the FIFO queue for game/mode/region.
func (q *RedisQueue) Enqueue(ctx context.Context, gameID uuid.UUID, mode, region string, sessionID uuid.UUID, createdAt time.Time) error {
	if q == nil || q.Client == nil {
		return ErrQueueUnavailable
	}
	score := float64(createdAt.UTC().UnixNano())
	return q.Client.ZAdd(ctx, q.queueKey(gameID, mode, region), redis.Z{
		Score:  score,
		Member: sessionID.String(),
	}).Err()
}

// Dequeue removes sessionID from the queue.
func (q *RedisQueue) Dequeue(ctx context.Context, gameID uuid.UUID, mode, region string, sessionID uuid.UUID) error {
	if q == nil || q.Client == nil {
		return ErrQueueUnavailable
	}
	removed, err := q.Client.ZRem(ctx, q.queueKey(gameID, mode, region), sessionID.String()).Result()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
	}
	if removed == 0 {
		return ErrNotEnqueued
	}
	return nil
}

// ListSessionIDs returns session IDs in FIFO order up to limit (0 = all).
func (q *RedisQueue) ListSessionIDs(ctx context.Context, gameID uuid.UUID, mode, region string, limit int64) ([]uuid.UUID, error) {
	if q == nil || q.Client == nil {
		return nil, ErrQueueUnavailable
	}
	var stop int64 = -1
	if limit > 0 {
		stop = limit - 1
	}
	members, err := q.Client.ZRange(ctx, q.queueKey(gameID, mode, region), 0, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
	}
	out := make([]uuid.UUID, 0, len(members))
	for _, m := range members {
		id, err := uuid.Parse(m)
		if err != nil {
			continue
		}
		out = append(out, id)
	}
	return out, nil
}

// QueueDepth returns the number of sessions waiting in the queue.
func (q *RedisQueue) QueueDepth(ctx context.Context, gameID uuid.UUID, mode, region string) (int64, error) {
	if q == nil || q.Client == nil {
		return 0, ErrQueueUnavailable
	}
	return q.Client.ZCard(ctx, q.queueKey(gameID, mode, region)).Result()
}

// Ping checks Redis connectivity.
func (q *RedisQueue) Ping(ctx context.Context) error {
	if q == nil || q.Client == nil {
		return ErrQueueUnavailable
	}
	return q.Client.Ping(ctx).Err()
}
