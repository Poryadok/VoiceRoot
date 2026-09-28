package gameprincipal

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultListenAddr   = ":9091"
	defaultRefreshAfter = 30 * time.Second
	defaultHardExpiry   = 2 * time.Minute
	defaultKIDCooldown  = 5 * time.Second
)

var ownedEnvNames = []string{
	"VOICE_GAME_PRINCIPAL_GRPC_LISTEN", "VOICE_GAME_PRINCIPAL_TLS_CERT_FILE",
	"VOICE_GAME_PRINCIPAL_TLS_KEY_FILE", "VOICE_GAME_PRINCIPAL_CLIENT_CA_FILE",
	"VOICE_GAME_PRINCIPAL_JWKS_URL", "VOICE_GAME_PRINCIPAL_JWKS_CA_FILE",
	"VOICE_GAME_PRINCIPAL_REPLAY_REDIS_ADDR", "VOICE_GAME_PRINCIPAL_REPLAY_REDIS_PASSWORD",
	"VOICE_GAME_PRINCIPAL_JWKS_REFRESH_AFTER", "VOICE_GAME_PRINCIPAL_JWKS_HARD_EXPIRY",
	"VOICE_GAME_PRINCIPAL_UNKNOWN_KID_COOLDOWN",
}

type Config struct {
	ListenAddr, TLSCertFile, TLSKeyFile, ClientCAFile string
	JWKSURL, JWKSCAFile                               string
	ReplayRedisAddr, ReplayRedisPassword              string
	RefreshAfter, HardExpiry, UnknownKIDCooldown      time.Duration
}

func LoadFromEnv() (Config, bool, error) {
	configured := false
	for _, name := range ownedEnvNames {
		if strings.TrimSpace(os.Getenv(name)) != "" {
			configured = true
		}
	}
	if !configured {
		return Config{}, false, nil
	}
	cfg := Config{
		ListenAddr:          strings.TrimSpace(os.Getenv("VOICE_GAME_PRINCIPAL_GRPC_LISTEN")),
		TLSCertFile:         strings.TrimSpace(os.Getenv("VOICE_GAME_PRINCIPAL_TLS_CERT_FILE")),
		TLSKeyFile:          strings.TrimSpace(os.Getenv("VOICE_GAME_PRINCIPAL_TLS_KEY_FILE")),
		ClientCAFile:        strings.TrimSpace(os.Getenv("VOICE_GAME_PRINCIPAL_CLIENT_CA_FILE")),
		JWKSURL:             strings.TrimSpace(os.Getenv("VOICE_GAME_PRINCIPAL_JWKS_URL")),
		JWKSCAFile:          strings.TrimSpace(os.Getenv("VOICE_GAME_PRINCIPAL_JWKS_CA_FILE")),
		ReplayRedisAddr:     strings.TrimSpace(os.Getenv("VOICE_GAME_PRINCIPAL_REPLAY_REDIS_ADDR")),
		ReplayRedisPassword: os.Getenv("VOICE_GAME_PRINCIPAL_REPLAY_REDIS_PASSWORD"),
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaultListenAddr
	}
	var err error
	if cfg.RefreshAfter, err = envDuration("VOICE_GAME_PRINCIPAL_JWKS_REFRESH_AFTER", defaultRefreshAfter); err != nil {
		return Config{}, true, err
	}
	if cfg.HardExpiry, err = envDuration("VOICE_GAME_PRINCIPAL_JWKS_HARD_EXPIRY", defaultHardExpiry); err != nil {
		return Config{}, true, err
	}
	if cfg.UnknownKIDCooldown, err = envDuration("VOICE_GAME_PRINCIPAL_UNKNOWN_KID_COOLDOWN", defaultKIDCooldown); err != nil {
		return Config{}, true, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, true, err
	}
	return cfg, true, nil
}

func envDuration(name string, fallback time.Duration) (time.Duration, error) {
	raw, present := os.LookupEnv(name)
	if !present {
		return fallback, nil
	}
	if strings.TrimSpace(raw) == "" {
		return 0, fmt.Errorf("%s must not be empty", name)
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return value, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.TLSCertFile) == "" || strings.TrimSpace(c.TLSKeyFile) == "" || strings.TrimSpace(c.ClientCAFile) == "" ||
		strings.TrimSpace(c.ReplayRedisAddr) == "" || c.RefreshAfter <= 0 || c.HardExpiry < c.RefreshAfter || c.UnknownKIDCooldown <= 0 {
		return errors.New("GIS principal listener requires TLS identity, client CA, replay Redis and valid JWKS cache settings")
	}
	if _, _, err := net.SplitHostPort(strings.TrimSpace(c.ListenAddr)); err != nil {
		return errors.New("invalid GIS principal listener address")
	}
	parsed, err := url.Parse(strings.TrimSpace(c.JWKSURL))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return errors.New("GIS principal requires an exact HTTPS JWKS URL")
	}
	if strings.ContainsAny(c.JWKSURL, "\r\n") {
		return errors.New("invalid GIS principal JWKS URL")
	}
	return nil
}

func singleIssuerJWKS(raw string) (string, error) {
	var endpoints map[string]string
	if err := json.Unmarshal([]byte(raw), &endpoints); err != nil || len(endpoints) != 1 || strings.TrimSpace(endpoints["gameintegration"]) == "" {
		return "", errors.New("GIS principal JWKS configuration must contain only gameintegration")
	}
	return strings.TrimSpace(endpoints["gameintegration"]), nil
}
