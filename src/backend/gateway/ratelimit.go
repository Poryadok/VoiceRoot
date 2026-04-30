package main

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateLimiter interface {
	Allow(ctx context.Context, key, group string) (bool, error)
}

type staticGroupLimiter map[string]bool

func (l staticGroupLimiter) Allow(_ context.Context, _, group string) (bool, error) {
	return !l[group], nil
}

type rateLimitRule struct {
	Limit  int
	Window time.Duration
}

type slidingWindowLimiter struct {
	now     func() time.Time
	rules   map[string]rateLimitRule
	mu      sync.Mutex
	entries map[string][]time.Time
}

func newSlidingWindowLimiter(rules map[string]rateLimitRule) *slidingWindowLimiter {
	return &slidingWindowLimiter{
		now:     time.Now,
		rules:   rules,
		entries: map[string][]time.Time{},
	}
}

func (l *slidingWindowLimiter) Allow(_ context.Context, key, group string) (bool, error) {
	rule, ok := l.rules[group]
	if !ok || rule.Limit <= 0 || rule.Window <= 0 {
		return true, nil
	}
	now := l.now()
	cutoff := now.Add(-rule.Window)
	bucketKey := group + ":" + key

	l.mu.Lock()
	defer l.mu.Unlock()

	kept := l.entries[bucketKey][:0]
	for _, ts := range l.entries[bucketKey] {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= rule.Limit {
		l.entries[bucketKey] = kept
		return false, nil
	}
	l.entries[bucketKey] = append(kept, now)
	return true, nil
}

func defaultRateLimitRules() map[string]rateLimitRule {
	return map[string]rateLimitRule{
		"Auth":          {Limit: 5, Window: 15 * time.Minute},
		"OTP":           {Limit: 3, Window: 10 * time.Minute},
		"MessagesSend":  {Limit: 5, Window: 5 * time.Second},
		"FileUpload":    {Limit: 10, Window: time.Hour},
		"SpaceCreation": {Limit: 5, Window: 24 * time.Hour},
		"BotAPI":        {Limit: 5000, Window: time.Minute},
	}
}

func rateLimitGroup(method, path string) string {
	switch {
	case method == http.MethodPost && (path == "/api/v1/auth/login" || path == "/api/v1/auth/register"):
		return "Auth"
	case method == http.MethodPost && strings.HasPrefix(path, "/api/v1/auth/otp/"):
		return "OTP"
	case method == http.MethodPost && path == "/api/v1/messages/send":
		return "MessagesSend"
	case method == http.MethodPost && path == "/api/v1/files/upload":
		return "FileUpload"
	case method == http.MethodPost && path == "/api/v1/spaces":
		return "SpaceCreation"
	case method == http.MethodPost && strings.HasPrefix(path, "/api/v1/bots/"):
		return "BotAPI"
	default:
		return ""
	}
}

func (g *gateway) rateLimitKey(r *http.Request, claims tokenClaims, publicRoute bool) string {
	if publicRoute || claims.UserID == "" {
		return "ip:" + g.clientIP(r)
	}
	return "user:" + claims.UserID
}
