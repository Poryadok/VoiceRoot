package principalconfig

import (
	"context"
	"testing"
	"time"
)

func TestLoadFromEnv_UsesDocumentedCacheDefaultsAndTrustedHTTPSIssuerURLs(t *testing.T) {
	t.Setenv("S2S_JWKS_URLS_JSON", `{"gateway":"https://gateway.internal/.well-known/voice-principal-jwks.json","space":"https://space.internal/.well-known/voice-principal-jwks.json"}`)
	t.Setenv("ROLE_PRINCIPAL_REPLAY_REDIS_ADDR", "redis:6379")
	t.Setenv("ROLE_PRINCIPAL_SESSION_EPOCH_REDIS_ADDR", "redis:6379")
	config, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if got := config.JWKSURLs["gateway"]; got != "https://gateway.internal/.well-known/voice-principal-jwks.json" {
		t.Fatalf("gateway URL = %q", got)
	}
	if config.RefreshAfter != 30*time.Second || config.HardExpiry != 2*time.Minute || config.UnknownKIDCooldown != 5*time.Second {
		t.Fatalf("cache policy = %#v", config)
	}
	if config.ReplayRedis.Addr != "redis:6379" || config.SessionEpochRedis.Addr != "redis:6379" {
		t.Fatalf("redis config = %#v", config)
	}
}
func TestLoadFromEnv_RejectsPartialMalformedOrUnsafeVerifierConfiguration(t *testing.T) {
	valid := `{"gateway":"https://gateway.internal/jwks"}`
	cases := []struct{ name, urls, refresh, hard, cooldown, replay, epoch string }{
		{name: "missing urls", replay: "redis:6379", epoch: "redis:6379"}, {name: "bad json", urls: "{", replay: "redis:6379", epoch: "redis:6379"}, {name: "non https", urls: `{"gateway":"http://gateway.internal/jwks"}`, replay: "redis:6379", epoch: "redis:6379"}, {name: "empty issuer", urls: `{" ":"https://gateway.internal/jwks"}`, replay: "redis:6379", epoch: "redis:6379"}, {name: "empty URL", urls: `{"gateway":" "}`, replay: "redis:6379", epoch: "redis:6379"}, {name: "invalid refresh", urls: valid, refresh: "nope", replay: "redis:6379", epoch: "redis:6379"}, {name: "hard before refresh", urls: valid, refresh: "30s", hard: "29s", replay: "redis:6379", epoch: "redis:6379"}, {name: "missing replay", urls: valid, epoch: "redis:6379"}, {name: "missing epoch", urls: valid, replay: "redis:6379"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("S2S_JWKS_URLS_JSON", tc.urls)
			t.Setenv("S2S_JWKS_REFRESH_AFTER", tc.refresh)
			t.Setenv("S2S_JWKS_HARD_EXPIRY", tc.hard)
			t.Setenv("S2S_UNKNOWN_KID_COOLDOWN", tc.cooldown)
			t.Setenv("ROLE_PRINCIPAL_REPLAY_REDIS_ADDR", tc.replay)
			t.Setenv("ROLE_PRINCIPAL_SESSION_EPOCH_REDIS_ADDR", tc.epoch)
			if _, err := LoadFromEnv(); err == nil {
				t.Fatal("LoadFromEnv() unexpectedly succeeded")
			}
		})
	}
}
func TestDependenciesValidate_FailsClosedWithoutBothVerifierDependencies(t *testing.T) {
	config := Config{JWKSURLs: map[string]string{"gateway": "https://gateway.internal/jwks"}, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second, ReplayRedis: RedisDependency{Addr: "redis:6379"}, SessionEpochRedis: RedisDependency{Addr: "redis:6379"}}
	if err := config.ValidateDependencies(Dependencies{}); err == nil {
		t.Fatal("nil dependencies accepted")
	}
	if err := config.ValidateDependencies(Dependencies{ReplayGuard: replayGuardFunc(func() {})}); err == nil {
		t.Fatal("missing epoch dependency accepted")
	}
	if err := config.ValidateDependencies(Dependencies{ReplayGuard: replayGuardFunc(func() {}), SessionEpochChecker: sessionEpochCheckerFunc(func() {})}); err != nil {
		t.Fatalf("valid dependencies rejected: %v", err)
	}
}

type replayGuardFunc func()

func (replayGuardFunc) RecordReplay(context.Context, string, string, time.Time) error { return nil }

type sessionEpochCheckerFunc func()

func (sessionEpochCheckerFunc) CheckSessionEpoch(context.Context, string, int64) error { return nil }
