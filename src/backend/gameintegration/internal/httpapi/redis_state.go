package httpapi

import (
	"context"
	"strconv"

	"github.com/redis/go-redis/v9"
)

const (
	sessionEpochFloorPrefix = "auth:session:min_epoch:"
	jwtBlacklistPrefix      = "jwt:blacklist:"
)

type RedisAuthorityState struct {
	Client *redis.Client
}

func (s RedisAuthorityState) Minimum(ctx context.Context, accountID string) (int64, error) {
	raw, err := s.Client.Get(ctx, sessionEpochFloorPrefix+accountID).Result()
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(raw, 10, 64)
}

func (s RedisAuthorityState) IsRevoked(ctx context.Context, jti string) (bool, error) {
	count, err := s.Client.Exists(ctx, jwtBlacklistPrefix+jti).Result()
	return count > 0, err
}
