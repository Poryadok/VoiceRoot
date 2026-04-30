package main

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"
)

type redisSlidingWindowLimiter struct {
	addr     string
	password string
	timeout  time.Duration
	now      func() time.Time
	rules    map[string]rateLimitRule
}

func newRedisSlidingWindowLimiter(addr, password string, rules map[string]rateLimitRule) *redisSlidingWindowLimiter {
	return &redisSlidingWindowLimiter{
		addr:     addr,
		password: password,
		timeout:  2 * time.Second,
		now:      time.Now,
		rules:    rules,
	}
}

func (l *redisSlidingWindowLimiter) Allow(ctx context.Context, key, group string) (bool, error) {
	rule, ok := l.rules[group]
	if !ok || rule.Limit <= 0 || rule.Window <= 0 {
		return true, nil
	}
	nowMillis := l.now().UnixMilli()
	cutoffMillis := nowMillis - rule.Window.Milliseconds()
	ttlSeconds := strconv.FormatInt(int64((rule.Window+time.Second-1)/time.Second), 10)
	member := strconv.FormatInt(nowMillis, 10) + "-" + generateRequestID()
	redisKey := "ratelimit:" + group + ":" + key

	result, err := l.eval(ctx, redisKey,
		strconv.FormatInt(cutoffMillis, 10),
		strconv.Itoa(rule.Limit),
		strconv.FormatInt(nowMillis, 10),
		ttlSeconds,
		member,
	)
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (l *redisSlidingWindowLimiter) eval(ctx context.Context, key string, args ...string) (int64, error) {
	dialer := net.Dialer{Timeout: l.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", l.addr)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(l.timeout)); err != nil {
		return 0, err
	}

	reader := bufio.NewReader(conn)
	if l.password != "" {
		if _, err := redisCommand(conn, reader, "AUTH", l.password); err != nil {
			return 0, err
		}
	}

	scriptArgs := []string{"EVAL", redisSlidingWindowScript, "1", key}
	scriptArgs = append(scriptArgs, args...)
	return redisCommand(conn, reader, scriptArgs...)
}

const redisSlidingWindowScript = `redis.call("ZREMRANGEBYSCORE", KEYS[1], "-inf", ARGV[1])
local count = redis.call("ZCARD", KEYS[1])
if count >= tonumber(ARGV[2]) then
  redis.call("EXPIRE", KEYS[1], ARGV[4])
  return 0
end
redis.call("ZADD", KEYS[1], ARGV[3], ARGV[5])
redis.call("EXPIRE", KEYS[1], ARGV[4])
return 1`

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
