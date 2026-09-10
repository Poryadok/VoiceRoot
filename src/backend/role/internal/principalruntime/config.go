package principalruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// Config configures only the dedicated Space ownership surface.
type Config struct {
	JWKSURLs                                     map[string]string
	RefreshAfter, HardExpiry, UnknownKIDCooldown time.Duration
	ReplayAddr, ReplayPassword, JWKSCAFile       string
	TLSCertFile, TLSKeyFile, ListenAddr          string
}

var issuerPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// LoadFromEnv disables the listener only when all of its configuration is absent.
func LoadFromEnv() (Config, bool, error) {
	names := []string{"S2S_JWKS_URLS_JSON", "S2S_JWKS_REFRESH_AFTER", "S2S_JWKS_HARD_EXPIRY", "S2S_UNKNOWN_KID_COOLDOWN", "S2S_JWKS_CA_FILE", "ROLE_PRINCIPAL_REPLAY_REDIS_ADDR", "ROLE_PRINCIPAL_REPLAY_REDIS_PASSWORD", "ROLE_PRINCIPAL_TLS_CERT_FILE", "ROLE_PRINCIPAL_TLS_KEY_FILE", "ROLE_PRINCIPAL_GRPC_LISTEN"}
	enabled := false
	for _, name := range names {
		if _, ok := os.LookupEnv(name); ok {
			enabled = true
		}
	}
	if !enabled {
		return Config{}, false, nil
	}
	cfg := Config{ReplayAddr: strings.TrimSpace(os.Getenv("ROLE_PRINCIPAL_REPLAY_REDIS_ADDR")), ReplayPassword: os.Getenv("ROLE_PRINCIPAL_REPLAY_REDIS_PASSWORD"), JWKSCAFile: strings.TrimSpace(os.Getenv("S2S_JWKS_CA_FILE")), TLSCertFile: strings.TrimSpace(os.Getenv("ROLE_PRINCIPAL_TLS_CERT_FILE")), TLSKeyFile: strings.TrimSpace(os.Getenv("ROLE_PRINCIPAL_TLS_KEY_FILE")), ListenAddr: ":9091"}
	if value, ok := os.LookupEnv("ROLE_PRINCIPAL_GRPC_LISTEN"); ok {
		cfg.ListenAddr = strings.TrimSpace(value)
	}
	if err := json.Unmarshal([]byte(os.Getenv("S2S_JWKS_URLS_JSON")), &cfg.JWKSURLs); err != nil {
		return Config{}, true, errors.New("invalid principal JWKS configuration")
	}
	var err error
	cfg.RefreshAfter, err = envDuration("S2S_JWKS_REFRESH_AFTER", 30*time.Second)
	if err != nil {
		return Config{}, true, err
	}
	cfg.HardExpiry, err = envDuration("S2S_JWKS_HARD_EXPIRY", 2*time.Minute)
	if err != nil {
		return Config{}, true, err
	}
	cfg.UnknownKIDCooldown, err = envDuration("S2S_UNKNOWN_KID_COOLDOWN", 5*time.Second)
	if err != nil {
		return Config{}, true, err
	}
	if err := cfg.validate(); err != nil {
		return Config{}, true, err
	}
	return cfg, true, nil
}
func envDuration(name string, fallback time.Duration) (time.Duration, error) {
	raw, ok := os.LookupEnv(name)
	if !ok {
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
	if strings.TrimSpace(c.ReplayAddr) == "" || strings.TrimSpace(c.TLSCertFile) == "" || strings.TrimSpace(c.TLSKeyFile) == "" {
		return errors.New("principal Redis and TLS certificate/key are required")
	}
	if _, _, err := net.SplitHostPort(c.ListenAddr); err != nil {
		return errors.New("invalid principal listener address")
	}
	if c.JWKSURLs["space"] == "" {
		return errors.New("trusted Space JWKS endpoint required")
	}
	for issuer, endpoint := range c.JWKSURLs {
		parsed, err := url.Parse(endpoint)
		if !issuerPattern.MatchString(issuer) || err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
			return errors.New("invalid principal HTTPS JWKS endpoint")
		}
	}
	return nil
}
