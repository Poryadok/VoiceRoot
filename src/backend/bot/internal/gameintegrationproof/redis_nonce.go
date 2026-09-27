package gameintegrationproof

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"voice/backend/pkg/workloadproof"
)

const noncePrefix = "bot:game-integration-proof:nonce:"

// RedisNonceStore shares the replay boundary across Bot replicas. Redis errors
// are returned to the verifier, which fails the protected endpoint closed.
type RedisNonceStore struct{ Client redis.Cmdable }

func (s RedisNonceStore) Use(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if s.Client == nil || ttl <= 0 {
		return false, workloadproof.ErrUnavailable
	}
	return s.Client.SetNX(ctx, noncePrefix+key, "1", ttl).Result()
}
