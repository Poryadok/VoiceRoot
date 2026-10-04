package queue

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrQueueUnavailable = errors.New("queue unavailable")
	ErrLockHeld         = errors.New("active search lock held")
	ErrNotEnqueued      = errors.New("session not in queue")
	ErrQueueGeneration  = errors.New("stale search queue generation")
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

func (q *RedisQueue) generationKey() string {
	return fmt.Sprintf("%s:queue:session-generations", q.prefix())
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
	const compareAndDelete = `
		if redis.call('GET', KEYS[1]) == ARGV[1] then
			return redis.call('DEL', KEYS[1])
		end
		return 0
	`
	if err := q.Client.Eval(ctx, compareAndDelete, []string{key}, sessionID.String()).Err(); err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
	}
	return nil
}

// Enqueue adds sessionID to the global FIFO queue for game/mode/region.
func (q *RedisQueue) Enqueue(ctx context.Context, gameID uuid.UUID, mode, region string, sessionID uuid.UUID, createdAt time.Time) error {
	return q.EnqueueScoped(ctx, nil, gameID, mode, region, sessionID, createdAt)
}

// EnqueueScoped enqueues into the global queue or mm:space:{id}:… when spaceID is set.
func (q *RedisQueue) EnqueueScoped(ctx context.Context, spaceID *uuid.UUID, gameID uuid.UUID, mode, region string, sessionID uuid.UUID, createdAt time.Time) error {
	return q.EnqueueScopedGeneration(ctx, spaceID, gameID, mode, region, sessionID, createdAt, 0)
}

// RecoverSearch atomically reacquires the same session's profile lock, advances
// its queue generation, and restores the FIFO member. A different lock owner or
// a newer generation fails closed.
func (q *RedisQueue) RecoverSearch(ctx context.Context, sess SearchRecovery) error {
	if q == nil || q.Client == nil {
		return ErrQueueUnavailable
	}
	const script = `
	local current = tonumber(redis.call('HGET', KEYS[2], ARGV[1]) or '-1')
	local wanted = tonumber(ARGV[2])
	if current > wanted then return 0 end
	local owner = redis.call('GET', KEYS[3])
	if owner and owner ~= ARGV[1] then return -1 end
	redis.call('SET', KEYS[3], ARGV[1], 'EX', ARGV[5])
	redis.call('HSET', KEYS[2], ARGV[1], ARGV[2])
	redis.call('ZADD', KEYS[1], ARGV[3], ARGV[1])
	return 1
	`
	n, err := q.Client.Eval(ctx, script, []string{q.scopedQueueKey(sess.SpaceID, sess.GameID, sess.Mode, sess.Region), q.generationKey(), q.lockKey(sess.ProfileID)},
		sess.SessionID.String(), strconv.FormatInt(sess.Generation, 10),
		strconv.FormatFloat(float64(sess.CreatedAt.UTC().UnixNano()), 'f', -1, 64),
		strconv.FormatInt(int64(defaultLockTTL/time.Second), 10)).Int()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
	}
	if n == -1 {
		return ErrLockHeld
	}
	if n == 0 {
		return ErrQueueGeneration
	}
	return nil
}

type SearchRecovery struct {
	SpaceID              *uuid.UUID
	GameID               uuid.UUID
	Mode, Region         string
	SessionID, ProfileID uuid.UUID
	CreatedAt            time.Time
	Generation           int64
}

// EnqueueScopedGeneration adds a queue member only if it does not regress the
// session's Redis generation. Normal search creation uses generation zero.
func (q *RedisQueue) EnqueueScopedGeneration(ctx context.Context, spaceID *uuid.UUID, gameID uuid.UUID, mode, region string, sessionID uuid.UUID, createdAt time.Time, generation int64) error {
	if q == nil || q.Client == nil {
		return ErrQueueUnavailable
	}
	const script = `
	local current = tonumber(redis.call('HGET', KEYS[2], ARGV[1]) or '-1')
	local wanted = tonumber(ARGV[2])
	if current > wanted then return 0 end
	redis.call('HSET', KEYS[2], ARGV[1], ARGV[2])
	redis.call('ZADD', KEYS[1], ARGV[3], ARGV[1])
	return 1
	`
	n, err := q.Client.Eval(ctx, script, []string{q.scopedQueueKey(spaceID, gameID, mode, region), q.generationKey()},
		sessionID.String(), strconv.FormatInt(generation, 10),
		strconv.FormatFloat(float64(createdAt.UTC().UnixNano()), 'f', -1, 64)).Int()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
	}
	if n == 0 {
		return ErrQueueGeneration
	}
	return nil
}

// RecoverRelease records the newer terminal generation, removes any stale FIFO
// member, and releases only the lock still owned by this session in one script.
func (q *RedisQueue) RecoverRelease(ctx context.Context, sess SearchRecovery) error {
	if q == nil || q.Client == nil {
		return ErrQueueUnavailable
	}
	const script = `
	local current = tonumber(redis.call('HGET', KEYS[2], ARGV[1]) or '-1')
	local wanted = tonumber(ARGV[2])
	if current > wanted then return 0 end
	redis.call('HSET', KEYS[2], ARGV[1], ARGV[2])
	redis.call('ZREM', KEYS[1], ARGV[1])
	if redis.call('GET', KEYS[3]) == ARGV[1] then redis.call('DEL', KEYS[3]) end
	return 1
	`
	n, err := q.Client.Eval(ctx, script, []string{q.scopedQueueKey(sess.SpaceID, sess.GameID, sess.Mode, sess.Region), q.generationKey(), q.lockKey(sess.ProfileID)},
		sess.SessionID.String(), strconv.FormatInt(sess.Generation, 10)).Int()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
	}
	if n == 0 {
		return ErrQueueGeneration
	}
	return nil
}

// Dequeue removes sessionID from the global queue.
func (q *RedisQueue) Dequeue(ctx context.Context, gameID uuid.UUID, mode, region string, sessionID uuid.UUID) error {
	return q.DequeueScoped(ctx, nil, gameID, mode, region, sessionID)
}

// DequeueScoped removes sessionID from the global or space-scoped queue.
func (q *RedisQueue) DequeueScoped(ctx context.Context, spaceID *uuid.UUID, gameID uuid.UUID, mode, region string, sessionID uuid.UUID) error {
	return q.DequeueScopedGeneration(ctx, spaceID, gameID, mode, region, sessionID, 0)
}

// DequeueScopedGeneration leaves a newer queue projection untouched when a
// delayed reservation/cancel tries to remove an older session generation.
func (q *RedisQueue) DequeueScopedGeneration(ctx context.Context, spaceID *uuid.UUID, gameID uuid.UUID, mode, region string, sessionID uuid.UUID, generation int64) error {
	if q == nil || q.Client == nil {
		return ErrQueueUnavailable
	}
	const script = `
	local current = tonumber(redis.call('HGET', KEYS[2], ARGV[1]) or '-1')
	local wanted = tonumber(ARGV[2])
	if current > wanted then return 0 end
	redis.call('HSET', KEYS[2], ARGV[1], ARGV[2])
	return redis.call('ZREM', KEYS[1], ARGV[1])
	`
	removed, err := q.Client.Eval(ctx, script, []string{q.scopedQueueKey(spaceID, gameID, mode, region), q.generationKey()},
		sessionID.String(), strconv.FormatInt(generation, 10)).Int()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
	}
	if removed == 0 && generation == 0 {
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

// ClearSpaceQueues removes every scoped matchmaking queue for one Space. It
// leaves global queues, other Spaces, and per-profile active-search locks
// untouched; callers release locks from the authoritative session rows.
func (q *RedisQueue) ClearSpaceQueues(ctx context.Context, spaceID uuid.UUID) error {
	if q == nil || q.Client == nil || spaceID == uuid.Nil {
		return ErrQueueUnavailable
	}
	pattern := fmt.Sprintf("%s:space:%s:queue:*", q.prefix(), spaceID.String())
	var cursor uint64
	for {
		keys, next, err := q.Client.Scan(ctx, cursor, pattern, 256).Result()
		if err != nil {
			return fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
		}
		if len(keys) > 0 {
			if err := q.Client.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

// Ping checks Redis connectivity.
func (q *RedisQueue) Ping(ctx context.Context) error {
	if q == nil || q.Client == nil {
		return ErrQueueUnavailable
	}
	return q.Client.Ping(ctx).Err()
}
