package store

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

func (s *RedisCallStore) spaceLifecycleKey(space string) string {
	return s.prefix + "space_lifecycle:" + space
}

// SetSpaceLifecycle never lets a stale LIVE writer lower a restrictive watermark.
// No TTL: projection loss is unavailable until PostgreSQL-backed restoration.
func (s *RedisCallStore) SetSpaceLifecycle(ctx context.Context, space string, generation int64, live bool) error {
	if space == "" || generation < 0 {
		return ErrInvalidState
	}
	key := s.spaceLifecycleKey(space)
	state := "DENY"
	if live {
		state = "LIVE"
	}
	want := strconv.FormatInt(generation, 10) + ":" + state
	for range redisTransitionAttempts {
		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			old, err := tx.Get(ctx, key).Result()
			if err != nil && !errors.Is(err, redis.Nil) {
				return err
			}
			if err == nil {
				parts := strings.Split(old, ":")
				if len(parts) != 2 {
					return ErrInvalidState
				}
				saved, err := strconv.ParseInt(parts[0], 10, 64)
				if err != nil || saved < 0 {
					return ErrInvalidState
				}
				if saved > generation || (saved == generation && parts[1] == "DENY" && live) {
					return ErrSpaceMediaStaleGrant
				}
				if old == want {
					return nil
				}
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error { pipe.Set(ctx, key, want, 0); return nil })
			return err
		}, key)
		if errors.Is(err, redis.TxFailedErr) {
			continue
		}
		return err
	}
	return ErrMoveContention
}

func (s *RedisCallStore) requireSpaceLive(ctx context.Context, tx *redis.Tx, space string, generation int64) error {
	value, err := tx.Get(ctx, s.spaceLifecycleKey(space)).Result()
	if err != nil {
		return ErrSpaceMediaStaleGrant
	}
	if value != strconv.FormatInt(generation, 10)+":LIVE" {
		return ErrSpaceMediaStaleGrant
	}
	return nil
}
