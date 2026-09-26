package httpapi

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisNonceStore struct {
	Client *redis.Client
}

func (s RedisNonceStore) Use(ctx context.Context, nonce string, ttl time.Duration) (bool, error) {
	if s.Client == nil {
		return false, ErrWorkloadUnavailable
	}
	return s.Client.SetNX(ctx, "gameintegration:auth-workload-nonce:"+nonce, "1", ttl).Result()
}
