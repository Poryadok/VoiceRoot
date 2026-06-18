package ratelimit

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/bot/internal/authctx"
)

const redisSlidingWindowScript = `redis.call("ZREMRANGEBYSCORE", KEYS[1], "-inf", ARGV[1])
local count = redis.call("ZCARD", KEYS[1])
if count >= tonumber(ARGV[2]) then
  redis.call("EXPIRE", KEYS[1], ARGV[4])
  return 0
end
redis.call("ZADD", KEYS[1], ARGV[3], ARGV[5])
redis.call("EXPIRE", KEYS[1], ARGV[4])
return 1`

// RedisGRPCLimiter enforces quotas via Redis ZSET sliding window (multi-replica safe).
type RedisGRPCLimiter struct {
	addr     string
	password string
	timeout  time.Duration
	rules    map[string]Limit
	now      func() time.Time
}

func newRedisGRPCLimiter(addr, password string, cfg Config) *RedisGRPCLimiter {
	rules := map[string]Limit{
		"BotAPI":        cfg.BotAPI,
		"BotRoleOps":    cfg.BotRoleOps,
		"TouchPresence": cfg.TouchPresence,
	}
	return &RedisGRPCLimiter{
		addr:     addr,
		password: password,
		timeout:  2 * time.Second,
		rules:    rules,
		now:      time.Now,
	}
}

// NewRedisGRPCLimiterForTest exposes Redis limiter for tests (miniredis).
func NewRedisGRPCLimiterForTest(addr, password string, cfg Config) *RedisGRPCLimiter {
	return newRedisGRPCLimiter(addr, password, cfg)
}

func (l *RedisGRPCLimiter) allow(ctx context.Context, group, key string) (bool, error) {
	rule, ok := l.rules[group]
	if !ok || rule.Max <= 0 || rule.Window <= 0 {
		return true, nil
	}
	nowMillis := l.now().UnixMilli()
	cutoffMillis := nowMillis - rule.Window.Milliseconds()
	ttlSeconds := strconv.FormatInt(int64((rule.Window+time.Second-1)/time.Second), 10)
	member := strconv.FormatInt(nowMillis, 10) + "-" + randomRequestID()
	redisKey := "bot-ratelimit:" + group + ":" + key

	result, err := l.eval(ctx, redisKey,
		strconv.FormatInt(cutoffMillis, 10),
		strconv.Itoa(rule.Max),
		strconv.FormatInt(nowMillis, 10),
		ttlSeconds,
		member,
	)
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (l *RedisGRPCLimiter) eval(ctx context.Context, key string, args ...string) (int64, error) {
	scriptArgs := []string{"EVAL", redisSlidingWindowScript, "1", key}
	scriptArgs = append(scriptArgs, args...)
	return redisEval(ctx, l.addr, l.password, l.timeout, scriptArgs...)
}

func (l *RedisGRPCLimiter) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !isBotRuntimeMethod(info.FullMethod) {
			return handler(ctx, req)
		}
		token, ok := authctx.BotToken(ctx)
		if !ok || token == "" {
			return handler(ctx, req)
		}
		group := grpcGroup(info.FullMethod)
		allowed, err := l.allow(ctx, group, token)
		if err != nil {
			// Fail open when Redis is unavailable (degraded mode).
			return handler(ctx, req)
		}
		if !allowed {
			retry := int(defaultConfig().BotAPI.Window.Seconds())
			if rule, ok := l.rules[group]; ok {
				retry = int(rule.Window.Seconds())
			}
			_ = grpc.SetHeader(ctx, metadata.Pairs("retry-after", strconv.Itoa(retry)))
			return nil, status.Error(codes.ResourceExhausted, "bot rate limit exceeded")
		}
		return handler(ctx, req)
	}
}

func redisEval(ctx context.Context, addr, password string, timeout time.Duration, args ...string) (int64, error) {
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return 0, err
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return 0, err
	}
	reader := bufio.NewReader(conn)
	if password != "" {
		if _, err := redisCommand(conn, reader, "AUTH", password); err != nil {
			return 0, err
		}
	}
	return redisCommand(conn, reader, args...)
}

func redisCommand(conn net.Conn, reader *bufio.Reader, args ...string) (int64, error) {
	var b strings.Builder
	b.WriteString("*")
	b.WriteString(strconv.Itoa(len(args)))
	b.WriteString("\r\n")
	for _, arg := range args {
		b.WriteString("$")
		b.WriteString(strconv.Itoa(len(arg)))
		b.WriteString("\r\n")
		b.WriteString(arg)
		b.WriteString("\r\n")
	}
	if _, err := conn.Write([]byte(b.String())); err != nil {
		return 0, err
	}
	return readRedisInteger(reader)
}

func readRedisInteger(reader *bufio.Reader) (int64, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return 0, err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return 0, err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	switch prefix {
	case ':':
		return strconv.ParseInt(line, 10, 64)
	case '+':
		return 1, nil
	case '-':
		return 0, errors.New(line)
	default:
		return 0, errors.New("unexpected redis response")
	}
}

func randomRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// ServerLimiterFromEnv returns Redis limiter when BOT_REDIS_ADDR is set, else in-memory.
func ServerLimiterFromEnv() ServerLimiter {
	if strings.TrimSpace(os.Getenv("BOT_RATE_LIMIT_DISABLED")) == "true" {
		return nilLimiter{}
	}
	cfg := defaultConfig()
	if addr := strings.TrimSpace(os.Getenv("BOT_REDIS_ADDR")); addr != "" {
		password := strings.TrimSpace(os.Getenv("BOT_REDIS_PASSWORD"))
		return newRedisGRPCLimiter(addr, password, cfg)
	}
	return NewGRPCLimiter(cfg)
}

type nilLimiter struct{}

func (nilLimiter) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(ctx, req)
	}
}
