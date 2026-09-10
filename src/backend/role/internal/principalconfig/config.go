package principalconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	jwksURLsEnv               = "S2S_JWKS_URLS_JSON"
	refreshAfterEnv           = "S2S_JWKS_REFRESH_AFTER"
	hardExpiryEnv             = "S2S_JWKS_HARD_EXPIRY"
	unknownKIDCooldownEnv     = "S2S_UNKNOWN_KID_COOLDOWN"
	replayRedisAddrEnv        = "ROLE_PRINCIPAL_REPLAY_REDIS_ADDR"
	sessionEpochRedisAddrEnv  = "ROLE_PRINCIPAL_SESSION_EPOCH_REDIS_ADDR"
	defaultRefreshAfter       = 30 * time.Second
	defaultHardExpiry         = 2 * time.Minute
	defaultUnknownKIDCooldown = 5 * time.Second
)

var issuerName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type RedisDependency struct{ Addr string }
type Config struct {
	JWKSURLs           map[string]string
	RefreshAfter       time.Duration
	HardExpiry         time.Duration
	UnknownKIDCooldown time.Duration
	ReplayRedis        RedisDependency
	SessionEpochRedis  RedisDependency
}
type ReplayGuard interface {
	RecordReplay(context.Context, string, string, time.Time) error
}
type SessionEpochChecker interface {
	CheckSessionEpoch(context.Context, string, int64) error
}
type Dependencies struct {
	ReplayGuard         ReplayGuard
	SessionEpochChecker SessionEpochChecker
}

// LoadFromEnv reads the shared Phase-0 verifier contract. It is intentionally
// not invoked by Role startup until deployment supplies TLS and real Redis; when
// invoked, incomplete configuration fails closed.
func LoadFromEnv() (Config, error) {
	rawURLs, present := os.LookupEnv(jwksURLsEnv)
	if !present || strings.TrimSpace(rawURLs) == "" {
		return Config{}, fmt.Errorf("%s is required", jwksURLsEnv)
	}
	var urls map[string]string
	if err := json.Unmarshal([]byte(rawURLs), &urls); err != nil || len(urls) == 0 {
		return Config{}, fmt.Errorf("%s must be a non-empty issuer URL object", jwksURLsEnv)
	}
	normalized := make(map[string]string, len(urls))
	for issuer, endpoint := range urls {
		issuer = strings.TrimSpace(issuer)
		endpoint = strings.TrimSpace(endpoint)
		parsed, err := url.Parse(endpoint)
		if !issuerName.MatchString(issuer) || err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
			return Config{}, errors.New("principal JWKS issuer endpoint is invalid")
		}
		normalized[issuer] = endpoint
	}
	refresh, err := durationFromEnv(refreshAfterEnv, defaultRefreshAfter)
	if err != nil {
		return Config{}, err
	}
	hard, err := durationFromEnv(hardExpiryEnv, defaultHardExpiry)
	if err != nil {
		return Config{}, err
	}
	cooldown, err := durationFromEnv(unknownKIDCooldownEnv, defaultUnknownKIDCooldown)
	if err != nil {
		return Config{}, err
	}
	if refresh <= 0 || hard < refresh || cooldown <= 0 {
		return Config{}, errors.New("principal JWKS cache policy is invalid")
	}
	replay := strings.TrimSpace(os.Getenv(replayRedisAddrEnv))
	epoch := strings.TrimSpace(os.Getenv(sessionEpochRedisAddrEnv))
	if replay == "" || epoch == "" {
		return Config{}, errors.New("principal replay and session epoch Redis addresses are required")
	}
	return Config{JWKSURLs: normalized, RefreshAfter: refresh, HardExpiry: hard, UnknownKIDCooldown: cooldown, ReplayRedis: RedisDependency{Addr: replay}, SessionEpochRedis: RedisDependency{Addr: epoch}}, nil
}
func durationFromEnv(name string, defaultValue time.Duration) (time.Duration, error) {
	value, present := os.LookupEnv(name)
	if !present {
		return defaultValue, nil
	}
	if strings.TrimSpace(value) == "" {
		return 0, fmt.Errorf("%s must not be empty", name)
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s is invalid", name)
	}
	return duration, nil
}
func (c Config) Validate() error {
	if len(c.JWKSURLs) == 0 || c.RefreshAfter <= 0 || c.HardExpiry < c.RefreshAfter || c.UnknownKIDCooldown <= 0 || strings.TrimSpace(c.ReplayRedis.Addr) == "" || strings.TrimSpace(c.SessionEpochRedis.Addr) == "" {
		return errors.New("principal verifier configuration is invalid")
	}
	return nil
}

func (c Config) ValidateDependencies(deps Dependencies) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if deps.ReplayGuard == nil || deps.SessionEpochChecker == nil {
		return errors.New("principal verifier dependencies are required")
	}
	return nil
}
