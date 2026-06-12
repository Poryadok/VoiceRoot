package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const versionCacheTTL = 5 * time.Minute

type versionPolicyCache interface {
	Get(ctx context.Context, platform string) (clientVersionRecord, bool)
	Set(ctx context.Context, platform string, record clientVersionRecord)
	Invalidate(ctx context.Context, platform string)
}

type redisVersionPolicyCache struct {
	client *redis.Client
}

func newRedisVersionPolicyCache(addr, password string) versionPolicyCache {
	return redisVersionPolicyCache{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
		}),
	}
}

func versionCacheKey(platform string) string {
	return "gateway:version:" + platform
}

func (c redisVersionPolicyCache) Get(ctx context.Context, platform string) (clientVersionRecord, bool) {
	raw, err := c.client.Get(ctx, versionCacheKey(platform)).Bytes()
	if err != nil {
		return clientVersionRecord{}, false
	}
	var record clientVersionRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return clientVersionRecord{}, false
	}
	return record, true
}

func (c redisVersionPolicyCache) Set(ctx context.Context, platform string, record clientVersionRecord) {
	raw, err := json.Marshal(record)
	if err != nil {
		return
	}
	_ = c.client.Set(ctx, versionCacheKey(platform), raw, versionCacheTTL).Err()
}

func (c redisVersionPolicyCache) Invalidate(ctx context.Context, platform string) {
	_ = c.client.Del(ctx, versionCacheKey(platform)).Err()
}

func versionPolicyCacheFromConfig(config gatewayConfig) versionPolicyCache {
	if strings.TrimSpace(config.versionCacheRedis) == "" {
		return nil
	}
	return newRedisVersionPolicyCache(config.versionCacheRedis, os.Getenv("GATEWAY_REDIS_PASSWORD"))
}
