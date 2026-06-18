package ratelimit

import (
	"context"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/bot/internal/authctx"
)

// Limit defines a sliding-window quota.
type Limit struct {
	Max    int
	Window time.Duration
}

// Config holds per-group limits for bot runtime gRPC.
type Config struct {
	BotAPI        Limit
	BotRoleOps    Limit
	TouchPresence Limit
}

// GRPCLimiter enforces bot-token rate limits on direct gRPC (mirrors Gateway BotAPI / BotRoleOps).
type GRPCLimiter struct {
	mu      sync.Mutex
	entries map[string][]time.Time
	rules   map[string]Limit
	now     func() time.Time
}

func defaultConfig() Config {
	return Config{
		BotAPI:        Limit{Max: 5000, Window: time.Minute},
		BotRoleOps:    Limit{Max: 100, Window: time.Minute},
		TouchPresence: Limit{Max: 5000, Window: time.Minute},
	}
}

// NewGRPCLimiter builds an in-memory sliding-window limiter.
func NewGRPCLimiter(cfg Config) *GRPCLimiter {
	rules := map[string]Limit{
		"BotAPI":        cfg.BotAPI,
		"BotRoleOps":    cfg.BotRoleOps,
		"TouchPresence": cfg.TouchPresence,
	}
	return &GRPCLimiter{
		entries: map[string][]time.Time{},
		rules:   rules,
		now:     time.Now,
	}
}

// FromEnv returns a limiter with production defaults; nil when rate limiting is disabled.
func FromEnv() ServerLimiter {
	return ServerLimiterFromEnv()
}

func (l *GRPCLimiter) allow(group, key string) bool {
	if l == nil {
		return true
	}
	rule, ok := l.rules[group]
	if !ok || rule.Max <= 0 || rule.Window <= 0 {
		return true
	}
	now := l.now()
	cutoff := now.Add(-rule.Window)
	bucket := group + ":" + key

	l.mu.Lock()
	defer l.mu.Unlock()

	kept := l.entries[bucket][:0]
	for _, ts := range l.entries[bucket] {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= rule.Max {
		l.entries[bucket] = kept
		return false
	}
	l.entries[bucket] = append(kept, now)
	return true
}

// AllowTouchPresence reports whether TouchPresence is within quota for botID.
func (l *GRPCLimiter) AllowTouchPresence(botID string) bool {
	return l.allow("TouchPresence", botID)
}

func grpcGroup(fullMethod string) string {
	switch fullMethod {
	case "/voice.bot.v1.BotService/AssignBotRole",
		"/voice.bot.v1.BotService/RevokeBotRole",
		"/voice.bot.v1.BotService/CreateBotRole":
		return "BotRoleOps"
	case "/voice.bot.v1.BotService/TouchPresence":
		return "TouchPresence"
	default:
		return "BotAPI"
	}
}

func isBotRuntimeMethod(fullMethod string) bool {
	switch fullMethod {
	case "/voice.bot.v1.BotService/RegisterBot",
		"/voice.bot.v1.BotService/UpdateBot",
		"/voice.bot.v1.BotService/DeleteBot",
		"/voice.bot.v1.BotService/GetBot",
		"/voice.bot.v1.BotService/GetBotBySlug",
		"/voice.bot.v1.BotService/ListBots",
		"/voice.bot.v1.BotService/RegenerateToken",
		"/voice.bot.v1.BotService/RegenerateWebhookSecret",
		"/voice.bot.v1.BotService/RegisterCommands",
		"/voice.bot.v1.BotService/GetCommands",
		"/voice.bot.v1.BotService/SetWebhookURL",
		"/voice.bot.v1.BotService/GetWebhookURL",
		"/voice.bot.v1.BotService/SetChatWhitelist",
		"/voice.bot.v1.BotService/GetChatWhitelist",
		"/voice.bot.v1.BotService/ValidateManifest",
		"/voice.bot.v1.BotService/ApplyManifest",
		"/voice.bot.v1.BotService/InstallBotInSpace",
		"/voice.bot.v1.BotService/UninstallBotFromSpace",
		"/voice.bot.v1.BotService/ListInstalledBots",
		"/voice.bot.v1.BotService/ListBotsInChat",
		"/voice.bot.v1.BotService/SetBotChatEnabled",
		"/voice.bot.v1.BotService/ExecuteSlashInteraction",
		"/voice.bot.v1.BotService/ListSlashCommandsForChat",
		"/voice.bot.v1.BotService/AutocompleteSlashOption":
		return false
	default:
		return strings.HasPrefix(fullMethod, "/voice.bot.v1.BotService/")
	}
}

// UnaryServerInterceptor returns ResourceExhausted when bot-token RPC exceeds quota.
func (l *GRPCLimiter) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	if l == nil {
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !isBotRuntimeMethod(info.FullMethod) {
			return handler(ctx, req)
		}
		token, ok := authctx.BotToken(ctx)
		if !ok || token == "" {
			return handler(ctx, req)
		}
		group := grpcGroup(info.FullMethod)
		if !l.allow(group, token) {
			return nil, status.Error(codes.ResourceExhausted, "bot rate limit exceeded")
		}
		return handler(ctx, req)
	}
}
