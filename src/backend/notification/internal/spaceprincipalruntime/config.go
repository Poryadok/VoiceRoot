package spaceprincipalruntime

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

type Config struct {
	JWKSURL                                      string
	RefreshAfter, HardExpiry, UnknownKIDCooldown time.Duration
	ReplayAddr, ReplayPassword, JWKSCAFile       string
	TLSCertFile, TLSKeyFile, ClientCAFile        string
	ListenAddr                                   string
}

func LoadFromEnv() (Config, bool, error) {
	names := []string{"NOTIFICATION_SPACE_LIFECYCLE_GRPC_LISTEN", "NOTIFICATION_SPACE_LIFECYCLE_TLS_CERT_FILE", "NOTIFICATION_SPACE_LIFECYCLE_TLS_KEY_FILE", "NOTIFICATION_SPACE_LIFECYCLE_CLIENT_CA_FILE", "NOTIFICATION_SPACE_LIFECYCLE_REPLAY_REDIS_ADDR", "NOTIFICATION_SPACE_LIFECYCLE_REPLAY_REDIS_PASSWORD", "S2S_JWKS_URLS_JSON", "S2S_JWKS_REFRESH_AFTER", "S2S_JWKS_HARD_EXPIRY", "S2S_UNKNOWN_KID_COOLDOWN", "S2S_JWKS_CA_FILE"}
	enabled := false
	for _, name := range names {
		if _, ok := os.LookupEnv(name); ok {
			enabled = true
		}
	}
	if !enabled {
		return Config{}, false, nil
	}
	cfg := Config{
		ListenAddr:  strings.TrimSpace(os.Getenv("NOTIFICATION_SPACE_LIFECYCLE_GRPC_LISTEN")),
		TLSCertFile: strings.TrimSpace(os.Getenv("NOTIFICATION_SPACE_LIFECYCLE_TLS_CERT_FILE")), TLSKeyFile: strings.TrimSpace(os.Getenv("NOTIFICATION_SPACE_LIFECYCLE_TLS_KEY_FILE")),
		ClientCAFile: strings.TrimSpace(os.Getenv("NOTIFICATION_SPACE_LIFECYCLE_CLIENT_CA_FILE")), ReplayAddr: strings.TrimSpace(os.Getenv("NOTIFICATION_SPACE_LIFECYCLE_REPLAY_REDIS_ADDR")),
		ReplayPassword: os.Getenv("NOTIFICATION_SPACE_LIFECYCLE_REPLAY_REDIS_PASSWORD"), JWKSCAFile: strings.TrimSpace(os.Getenv("S2S_JWKS_CA_FILE")),
	}
	var endpoints map[string]string
	if err := json.Unmarshal([]byte(os.Getenv("S2S_JWKS_URLS_JSON")), &endpoints); err != nil {
		return Config{}, true, errors.New("invalid principal JWKS configuration")
	}
	cfg.JWKSURL = strings.TrimSpace(endpoints["space"])
	var err error
	if cfg.RefreshAfter, err = envDuration("S2S_JWKS_REFRESH_AFTER", 30*time.Second); err != nil {
		return Config{}, true, err
	}
	if cfg.HardExpiry, err = envDuration("S2S_JWKS_HARD_EXPIRY", 2*time.Minute); err != nil {
		return Config{}, true, err
	}
	if cfg.UnknownKIDCooldown, err = envDuration("S2S_UNKNOWN_KID_COOLDOWN", 5*time.Second); err != nil {
		return Config{}, true, err
	}
	if err := cfg.validate(); err != nil {
		return Config{}, true, err
	}
	return cfg, true, nil
}

func envDuration(name string, fallback time.Duration) (time.Duration, error) {
	raw, present := os.LookupEnv(name)
	if !present {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return value, nil
}

func (c Config) validate() error {
	if c.RefreshAfter <= 0 || c.HardExpiry < c.RefreshAfter || c.UnknownKIDCooldown <= 0 {
		return errors.New("invalid principal cache policy")
	}
	if c.ReplayAddr == "" || c.TLSCertFile == "" || c.TLSKeyFile == "" || c.ClientCAFile == "" {
		return errors.New("space lifecycle replay Redis and mutual TLS files are required")
	}
	if _, _, err := net.SplitHostPort(c.ListenAddr); err != nil {
		return errors.New("invalid Space lifecycle listener address")
	}
	parsed, err := url.Parse(c.JWKSURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return errors.New("trusted Space HTTPS JWKS endpoint required")
	}
	return nil
}
