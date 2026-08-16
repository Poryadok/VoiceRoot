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

func (q *RedisQueue) prefix() string {
	if q == nil || q.Prefix == "" {
		return "mm"
	}
	return q.Prefix
}

func (q *RedisQueue) queueKey(gameID uuid.UUID, mode, region string) string {
	return fmt.Sprintf("%s:queue:%s:%s:%s", q.prefix(), gameID.String(), mode, region)
}

// spaceQueueKey is Redis mm:space:{space_id}:queue:{game}:{mode}:{region} (roadmap П.1).
func (q *RedisQueue) spaceQueueKey(spaceID, gameID uuid.UUID, mode, region string) string {
	return fmt.Sprintf("%s:space:%s:queue:%s:%s:%s", q.prefix(), spaceID.String(), gameID.String(), mode, region)
}

func (q *RedisQueue) scopedQueueKey(spaceID *uuid.UUID, gameID uuid.UUID, mode, region string) string {
	if spaceID != nil {
		return q.spaceQueueKey(*spaceID, gameID, mode, region)
	}
	return q.queueKey(gameID, mode, region)
}

func (q *RedisQueue) lockKey(profileID uuid.UUID) string {
	return fmt.Sprintf("%s:lock:profile:%s", q.prefix(), profileID.String())
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

// Enqueue adds sessionID to the global FIFO queue for game/mode/region.
func (q *RedisQueue) Enqueue(ctx context.Context, gameID uuid.UUID, mode, region string, sessionID uuid.UUID, createdAt time.Time) error {
	return q.EnqueueScoped(ctx, nil, gameID, mode, region, sessionID, createdAt)
}

// EnqueueScoped enqueues into the global queue or mm:space:{id}:… when spaceID is set.
func (q *RedisQueue) EnqueueScoped(ctx context.Context, spaceID *uuid.UUID, gameID uuid.UUID, mode, region string, sessionID uuid.UUID, createdAt time.Time) error {
	if q == nil || q.Client == nil {
		return ErrQueueUnavailable
	}
	score := float64(createdAt.UTC().UnixNano())
	return q.Client.ZAdd(ctx, q.scopedQueueKey(spaceID, gameID, mode, region), redis.Z{
		Score:  score,
		Member: sessionID.String(),
	}).Err()
}

// Dequeue removes sessionID from the global queue.
func (q *RedisQueue) Dequeue(ctx context.Context, gameID uuid.UUID, mode, region string, sessionID uuid.UUID) error {
	return q.DequeueScoped(ctx, nil, gameID, mode, region, sessionID)
}

// DequeueScoped removes sessionID from the global or space-scoped queue.
func (q *RedisQueue) DequeueScoped(ctx context.Context, spaceID *uuid.UUID, gameID uuid.UUID, mode, region string, sessionID uuid.UUID) error {
	if q == nil || q.Client == nil {
		return ErrQueueUnavailable
	}
	removed, err := q.Client.ZRem(ctx, q.scopedQueueKey(spaceID, gameID, mode, region), sessionID.String()).Result()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
	}
	if removed == 0 {
		return ErrNotEnqueued
	}
	return nil
}

// ListSessionIDs returns session IDs in FIFO order up to limit (0 = all) from the global queue.
func (q *RedisQueue) ListSessionIDs(ctx context.Context, gameID uuid.UUID, mode, region string, limit int64) ([]uuid.UUID, error) {
	return q.ListSessionIDsScoped(ctx, nil, gameID, mode, region, limit)
}

// ListSessionIDsScoped lists session IDs from the global or space-scoped queue.
func (q *RedisQueue) ListSessionIDsScoped(ctx context.Context, spaceID *uuid.UUID, gameID uuid.UUID, mode, region string, limit int64) ([]uuid.UUID, error) {
	if q == nil || q.Client == nil {
		return nil, ErrQueueUnavailable
	}
	var stop int64 = -1
	if limit > 0 {
		stop = limit - 1
	}
	members, err := q.Client.ZRange(ctx, q.scopedQueueKey(spaceID, gameID, mode, region), 0, stop).Result()
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

// QueueDepth returns the number of sessions waiting in the global queue.
func (q *RedisQueue) QueueDepth(ctx context.Context, gameID uuid.UUID, mode, region string) (int64, error) {
	return q.QueueDepthScoped(ctx, nil, gameID, mode, region)
}

// QueueDepthScoped returns depth for the global or space-scoped queue.
func (q *RedisQueue) QueueDepthScoped(ctx context.Context, spaceID *uuid.UUID, gameID uuid.UUID, mode, region string) (int64, error) {
	if q == nil || q.Client == nil {
		return 0, ErrQueueUnavailable
	}
	return q.Client.ZCard(ctx, q.scopedQueueKey(spaceID, gameID, mode, region)).Result()
}

// Ping checks Redis connectivity.
func (q *RedisQueue) Ping(ctx context.Context) error {
	if q == nil || q.Client == nil {
		return ErrQueueUnavailable
	}
	return q.Client.Ping(ctx).Err()
}
