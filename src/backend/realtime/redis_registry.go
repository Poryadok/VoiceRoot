package main

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// redisProfileRegistry stores ephemeral WS routing hints: profile -> set of "instance_id:conn_id".
// See docs/microservices/realtime-service.md (Redis registry).
type redisProfileRegistry struct {
	rdb    *redis.Client
	prefix string
}

func newRedisProfileRegistry(rdb *redis.Client, keyPrefix string) *redisProfileRegistry {
	return &redisProfileRegistry{rdb: rdb, prefix: keyPrefix}
}

func (r *redisProfileRegistry) profileKey(profileID string) string {
	return r.prefix + profileID
}

func memberKey(instanceID, connID string) string {
	return instanceID + ":" + connID
}

func (r *redisProfileRegistry) Register(ctx context.Context, profileID, instanceID, connID string) error {
	if r == nil || r.rdb == nil {
		return nil
	}
	return r.rdb.SAdd(ctx, r.profileKey(profileID), memberKey(instanceID, connID)).Err()
}

func (r *redisProfileRegistry) Unregister(ctx context.Context, profileID, instanceID, connID string) error {
	if r == nil || r.rdb == nil {
		return nil
	}
	_, err := r.rdb.SRem(ctx, r.profileKey(profileID), memberKey(instanceID, connID)).Result()
	return err
}
