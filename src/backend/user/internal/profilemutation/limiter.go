// Package profilemutation bounds authenticated profile churn across User replicas.
package profilemutation

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	Create      = "create"
	Delete      = "delete"
	MaxAttempts = 10
	Window      = time.Hour
)

// Limiter uses Redis server time and one atomic admission operation. Both keys
// share the account hash tag so the script can also run on a clustered Redis.
type Limiter struct {
	client *redis.Client
}

// New deliberately retains a nil client as an unavailable dependency; it never
// substitutes an instance-local budget when shared Redis is absent.
func New(client *redis.Client) *Limiter { return &Limiter{client: client} }

var admissionScript = redis.NewScript(`
local clock = redis.call('TIME')
local now = tonumber(clock[1]) * 1000 + math.floor(tonumber(clock[2]) / 1000)
local window = tonumber(ARGV[2])
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now - window)
if redis.call('ZCARD', KEYS[1]) >= tonumber(ARGV[1]) then
  return 0
end
local sequence = redis.call('INCR', KEYS[2])
redis.call('ZADD', KEYS[1], now, tostring(now) .. ':' .. tostring(sequence))
redis.call('PEXPIRE', KEYS[1], window)
redis.call('PEXPIRE', KEYS[2], window)
return 1
`)

// Allow consumes one authenticated attempt, including a later rejected business
// operation. Failed/denied requests are never refunded with a racy second write.
func (l *Limiter) Allow(ctx context.Context, accountID uuid.UUID, operation string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if l == nil || l.client == nil {
		return false, errors.New("profile mutation limiter unavailable")
	}
	if accountID == uuid.Nil || (operation != Create && operation != Delete) {
		return false, errors.New("invalid profile mutation scope")
	}
	key := "user:profile-mutations:v1:{" + accountID.String() + "}:" + operation
	result, err := admissionScript.Run(ctx, l.client, []string{key, key + ":sequence"}, MaxAttempts, Window.Milliseconds()).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}
