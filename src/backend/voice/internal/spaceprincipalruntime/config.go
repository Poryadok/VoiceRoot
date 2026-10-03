package spaceprincipalruntime

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

type Config struct {
	JWKSURLs                                     map[string]string
	RefreshAfter, HardExpiry, UnknownKIDCooldown time.Duration
	ReplayAddr, ReplayPassword, JWKSCAFile       string
	TLSCertFile, TLSKeyFile, ClientCAFile        string
	ListenAddr                                   string
}

var issuerPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func LoadFromEnv() (Config, bool, error) {
	prefix := "VOICE_SPACE_LIFECYCLE_PRINCIPAL_"
	names := []string{"S2S_JWKS_URLS_JSON", "S2S_JWKS_REFRESH_AFTER", "S2S_JWKS_HARD_EXPIRY", "S2S_UNKNOWN_KID_COOLDOWN", "S2S_JWKS_CA_FILE",
		prefix + "REPLAY_REDIS_ADDR", prefix + "REPLAY_REDIS_PASSWORD", prefix + "TLS_CERT_FILE", prefix + "TLS_KEY_FILE", prefix + "CLIENT_CA_FILE", prefix + "GRPC_LISTEN"}
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
		ReplayAddr: strings.TrimSpace(os.Getenv(prefix + "REPLAY_REDIS_ADDR")), ReplayPassword: os.Getenv(prefix + "REPLAY_REDIS_PASSWORD"),
		JWKSCAFile: strings.TrimSpace(os.Getenv("S2S_JWKS_CA_FILE")), TLSCertFile: strings.TrimSpace(os.Getenv(prefix + "TLS_CERT_FILE")),
		TLSKeyFile: strings.TrimSpace(os.Getenv(prefix + "TLS_KEY_FILE")), ClientCAFile: strings.TrimSpace(os.Getenv(prefix + "CLIENT_CA_FILE")), ListenAddr: ":9091",
	}
	if value, ok := os.LookupEnv(prefix + "GRPC_LISTEN"); ok {
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
	if strings.TrimSpace(c.ReplayAddr) == "" || strings.TrimSpace(c.TLSCertFile) == "" || strings.TrimSpace(c.TLSKeyFile) == "" || strings.TrimSpace(c.ClientCAFile) == "" {
		return errors.New("principal Redis and mutual TLS files are required")
	}
	if _, _, err := net.SplitHostPort(c.ListenAddr); err != nil {
		return errors.New("invalid principal listener address")
	}
	endpoint := c.JWKSURLs["space"]
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return errors.New("trusted Space HTTPS JWKS endpoint required")
	}
	for issuer, endpoint := range c.JWKSURLs {
		parsed, err := url.Parse(endpoint)
		if !issuerPattern.MatchString(issuer) || err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
			return errors.New("invalid principal HTTPS JWKS endpoint")
		}
	}
	return nil
}
