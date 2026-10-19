// Package chatquota admits ordinary logical chat pushes, never provider attempts.
package chatquota

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

const DecisionTTL = 192 * time.Hour

type Config struct {
	Limit  int
	Window time.Duration
}

func ConfigFromEnv(lookup func(string) (string, bool)) (Config, error) {
	cfg := Config{Limit: 60, Window: time.Minute}
	if raw, ok := lookup("NOTIFICATION_CHAT_PUSH_LIMIT"); ok {
		v, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return cfg, fmt.Errorf("invalid chat push limit")
		}
		cfg.Limit = v
	}
	if raw, ok := lookup("NOTIFICATION_CHAT_PUSH_WINDOW"); ok {
		v, err := time.ParseDuration(strings.TrimSpace(raw))
		if err != nil {
			return cfg, fmt.Errorf("invalid chat push window")
		}
		cfg.Window = v
	}
	return cfg, validate(cfg)
}
func validate(c Config) error {
	if c.Limit <= 0 || c.Window < time.Millisecond || c.Window > DecisionTTL {
		return fmt.Errorf("chat quota requires positive limit and window between 1ms and192h")
	}
	return nil
}

// Keys share a Redis cluster slot; values contain only non-content ID digests.
func keysFor(profile, chat, message uuid.UUID) []string {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte("chat-message:v1:"+chat.String()+":"+message.String())))
	prefix := "notification:chat-quota:v1:{" + profile.String() + "}:"
	return []string{prefix + "window", prefix + "decision:" + digest, digest}
}

// All validations precede writes: Redis Lua errors do not roll back mutations.
const script = `
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])
if not limit or limit < 1 or limit ~= math.floor(limit) or not window or window < 1 or not ttl or ttl < window then return redis.error_reply('invalid quota configuration') end
local wt = redis.call('TYPE', KEYS[1]).ok
local dt = redis.call('TYPE', KEYS[2]).ok
if wt ~= 'none' and wt ~= 'zset' then return redis.error_reply('invalid quota window') end
if dt ~= 'none' and dt ~= 'string' then return redis.error_reply('invalid quota decision type') end
local decision = redis.call('GET', KEYS[2])
if decision and decision ~= 'ADMITTED' and decision ~= 'SUPPRESSED' then return redis.error_reply('invalid quota decision') end
if decision and redis.call('PTTL', KEYS[2]) <= 0 then return redis.error_reply('invalid quota decision ttl') end
local clock = redis.call('TIME')
local now = tonumber(clock[1])*1000 + math.floor(tonumber(clock[2])/1000)
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now-window)
if decision == 'ADMITTED' then return 2 end
if decision == 'SUPPRESSED' then return 0 end
if redis.call('ZCARD', KEYS[1]) >= limit then
 redis.call('SET', KEYS[2], 'SUPPRESSED', 'PX', ttl)
 return 0
end
redis.call('ZADD', KEYS[1], now, ARGV[4])
redis.call('PEXPIRE', KEYS[1], window)
redis.call('SET', KEYS[2], 'ADMITTED', 'PX', ttl)
return 1
`

type Redis struct {
	client   *redis.Client
	cfg      Config
	outcomes *prometheus.CounterVec
}

func (q *Redis) Close() error {
	if q == nil {
		return nil
	}
	return q.client.Close()
}
func New(client *redis.Client, cfg Config) (*Redis, error) {
	if client == nil {
		return nil, fmt.Errorf("chat quota Redis unavailable")
	}
	if err := validate(cfg); err != nil {
		return nil, err
	}
	return &Redis{client: client, cfg: cfg}, nil
}

// RegisterMetrics uses only a fixed outcome label, never profile/message/token IDs.
func (q *Redis) RegisterMetrics(reg prometheus.Registerer) error {
	q.outcomes = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "notification_chat_push_admission_total", Help: "Ordinary chat push quota outcomes."}, []string{"outcome"})
	return reg.Register(q.outcomes)
}
func (q *Redis) observe(outcome string) {
	if q != nil && q.outcomes != nil {
		q.outcomes.WithLabelValues(outcome).Inc()
	}
}
func (q *Redis) Admit(ctx context.Context, profile, chat, message uuid.UUID) (bool, error) {
	if q == nil || profile == uuid.Nil || chat == uuid.Nil || message == uuid.Nil {
		return false, fmt.Errorf("invalid chat quota identity or wiring")
	}
	keys := keysFor(profile, chat, message)
	result, err := q.client.Eval(ctx, script, keys[:2], q.cfg.Limit, q.cfg.Window.Milliseconds(), DecisionTTL.Milliseconds(), keys[2]).Int64()
	if err != nil {
		q.observe("error")
		return false, fmt.Errorf("chat push admission unavailable: %w", err)
	}
	switch result {
	case 0:
		q.observe("suppressed")
		return false, nil
	case 1:
		q.observe("admitted")
		return true, nil
	case 2:
		q.observe("replay")
		return true, nil
	default:
		q.observe("error")
		return false, fmt.Errorf("unknown chat push admission result")
	}
}
