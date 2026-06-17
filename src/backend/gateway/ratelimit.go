package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
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
		"AuthLogin":     {Limit: 5, Window: 15 * time.Minute},
		"AuthRegister":  {Limit: 5, Window: 15 * time.Minute},
		"AuthOAuth":     {Limit: 30, Window: 15 * time.Minute},
		"OTP":           {Limit: 3, Window: 10 * time.Minute},
		"MessagesSend":  {Limit: 5, Window: 5 * time.Second},
		"FileUpload":    {Limit: 10, Window: time.Hour},
		"SpaceCreation": {Limit: 5, Window: 24 * time.Hour},
		"BotAPI":        {Limit: 5000, Window: time.Minute},
		"BotRoleOps":    {Limit: 100, Window: time.Minute},
		"E2EKeyBackupPut": {Limit: 5, Window: time.Minute},
		"E2EKeyBackupGet": {Limit: 30, Window: time.Minute},
		"PreKeyUpload":    {Limit: 10, Window: time.Minute},
		"PreKeyGet":       {Limit: 60, Window: time.Minute},
	}
}

// rateLimitRuleSpec is the JSON shape for GATEWAY_RATE_LIMIT_RULES_JSON.
// Window uses Go duration strings (e.g. "15m", "5s"). Limit <= 0 disables the group.
type rateLimitRuleSpec struct {
	Limit  int    `json:"limit"`
	Window string `json:"window"`
}

func rateLimitRulesFromEnv() map[string]rateLimitRule {
	rules := copyRateLimitRules(defaultRateLimitRules())
	var overrides map[string]rateLimitRuleSpec
	raw := strings.TrimSpace(os.Getenv("GATEWAY_RATE_LIMIT_RULES_JSON"))
	if raw == "" {
		return rules
	}
	if err := json.Unmarshal([]byte(raw), &overrides); err != nil {
		log.Printf("invalid GATEWAY_RATE_LIMIT_RULES_JSON: %v", err)
		return rules
	}
	applyRateLimitRuleOverrides(rules, overrides)
	return rules
}

func copyRateLimitRules(src map[string]rateLimitRule) map[string]rateLimitRule {
	dst := make(map[string]rateLimitRule, len(src))
	for group, rule := range src {
		dst[group] = rule
	}
	return dst
}

func applyRateLimitRuleOverrides(rules map[string]rateLimitRule, overrides map[string]rateLimitRuleSpec) {
	for group, spec := range overrides {
		rule := rateLimitRuleFromSpec(spec)
		if group == "Auth" {
			rules["AuthLogin"] = rule
			rules["AuthRegister"] = rule
			continue
		}
		rules[group] = rule
	}
}

func rateLimitRuleFromSpec(spec rateLimitRuleSpec) rateLimitRule {
	window, err := time.ParseDuration(strings.TrimSpace(spec.Window))
	if err != nil {
		window = 0
	}
	return rateLimitRule{Limit: spec.Limit, Window: window}
}

func rateLimitGroup(method, path string) string {
	switch {
	case method == http.MethodPost && path == "/api/v1/auth/login":
		return "AuthLogin"
	case method == http.MethodPost && path == "/api/v1/auth/register":
		return "AuthRegister"
	case path == "/api/v1/auth/oauth2/authorize" &&
		(method == http.MethodGet || method == http.MethodPost):
		return "AuthOAuth"
	case method == http.MethodPost && path == "/api/v1/auth/oauth2/token":
		return "AuthOAuth"
	case method == http.MethodPost && strings.HasPrefix(path, "/api/v1/auth/otp/"):
		return "OTP"
	case method == http.MethodPost && path == "/api/v1/messages/send":
		return "MessagesSend"
	case method == http.MethodPost && path == "/api/v1/files/upload":
		return "FileUpload"
	case method == http.MethodPost && path == "/api/v1/users/me/avatar/presigned-upload":
		// Phase 1 avatar: mint presigned PUT (same abuse class as file upload); see api-gateway.md rate limits.
		return "FileUpload"
	case method == http.MethodPost && path == "/api/v1/spaces":
		return "SpaceCreation"
	case method == http.MethodPost && strings.HasPrefix(path, "/api/v1/bots/me/spaces/") && strings.HasSuffix(path, "/roles/assign"):
		return "BotRoleOps"
	case method == http.MethodPost && strings.HasPrefix(path, "/api/v1/bots/me/spaces/") && strings.HasSuffix(path, "/roles/revoke"):
		return "BotRoleOps"
	case method == http.MethodPost && path == "/api/v1/bots/me/roles":
		return "BotRoleOps"
	case (method == http.MethodPost || method == http.MethodGet) && strings.HasPrefix(path, "/api/v1/bots/me/"):
		return "BotAPI"
	case method == http.MethodPost && strings.HasPrefix(path, "/api/v1/bots/"):
		return "BotAPI"
	case method == http.MethodPut && path == "/api/v1/auth/e2e-key-backup":
		return "E2EKeyBackupPut"
	case method == http.MethodGet && path == "/api/v1/auth/e2e-key-backup":
		return "E2EKeyBackupGet"
	case method == http.MethodPost && path == "/api/v1/messages/prekeys":
		return "PreKeyUpload"
	case method == http.MethodGet && path == "/api/v1/messages/prekeys":
		return "PreKeyGet"
	default:
		return ""
	}
}

func rateLimitRetryAfterSeconds(group string) int {
	rules := defaultRateLimitRules()
	rule, ok := rules[group]
	if !ok || rule.Window <= 0 {
		return 0
	}
	sec := int(rule.Window.Seconds())
	if sec < 1 {
		return 1
	}
	return sec
}

func (g *gateway) rateLimitKey(r *http.Request, claims tokenClaims, publicRoute bool) string {
	if token := botBearerToken(r); token != "" {
		return "bot:" + token
	}
	if publicRoute || claims.UserID == "" {
		return "ip:" + g.clientIP(r)
	}
	return "user:" + claims.UserID
}
