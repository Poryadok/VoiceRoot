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

type Config struct {
	ClientCAFile                                 string
	JWKSURLs                                     map[string]string
	RefreshAfter, HardExpiry, UnknownKIDCooldown time.Duration
	ReplayAddr, ReplayPassword, JWKSCAFile       string
	JWKSClientCertFile, JWKSClientKeyFile        string
	TLSCertFile, TLSKeyFile, ListenAddr          string
}

var issuerPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func LoadFromEnv() (Config, bool, error) {
	names := []string{"FILE_PRINCIPAL_GRPC_LISTEN", "FILE_PRINCIPAL_TLS_CERT_FILE", "FILE_PRINCIPAL_TLS_KEY_FILE", "FILE_PRINCIPAL_CLIENT_CA_FILE", "FILE_PRINCIPAL_REPLAY_REDIS_ADDR", "FILE_PRINCIPAL_REPLAY_REDIS_PASSWORD", "S2S_JWKS_URLS_JSON", "S2S_JWKS_REFRESH_AFTER", "S2S_JWKS_HARD_EXPIRY", "S2S_UNKNOWN_KID_COOLDOWN", "S2S_JWKS_CA_FILE", "S2S_JWKS_CLIENT_CERT_FILE", "S2S_JWKS_CLIENT_KEY_FILE"}
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
		ClientCAFile: strings.TrimSpace(os.Getenv("FILE_PRINCIPAL_CLIENT_CA_FILE")),
		ReplayAddr:   strings.TrimSpace(os.Getenv("FILE_PRINCIPAL_REPLAY_REDIS_ADDR")), ReplayPassword: os.Getenv("FILE_PRINCIPAL_REPLAY_REDIS_PASSWORD"),
		JWKSCAFile: strings.TrimSpace(os.Getenv("S2S_JWKS_CA_FILE")), JWKSClientCertFile: strings.TrimSpace(os.Getenv("S2S_JWKS_CLIENT_CERT_FILE")), JWKSClientKeyFile: strings.TrimSpace(os.Getenv("S2S_JWKS_CLIENT_KEY_FILE")), TLSCertFile: strings.TrimSpace(os.Getenv("FILE_PRINCIPAL_TLS_CERT_FILE")), TLSKeyFile: strings.TrimSpace(os.Getenv("FILE_PRINCIPAL_TLS_KEY_FILE")),
		ListenAddr: strings.TrimSpace(os.Getenv("FILE_PRINCIPAL_GRPC_LISTEN")),
	}
	if err := json.Unmarshal([]byte(os.Getenv("S2S_JWKS_URLS_JSON")), &cfg.JWKSURLs); err != nil {
		return Config{}, true, errors.New("invalid principal JWKS configuration")
	}
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
	raw, ok := os.LookupEnv(name)
	if !ok {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return d, nil
}

func (c Config) validate() error {
	if strings.TrimSpace(c.ClientCAFile) == "" {
		return errors.New("principal mTLS client CA is required")
	}
	if c.RefreshAfter <= 0 || c.HardExpiry < c.RefreshAfter || c.UnknownKIDCooldown <= 0 {
		return errors.New("invalid principal cache policy")
	}
	if c.ListenAddr == "" || c.ReplayAddr == "" || c.TLSCertFile == "" || c.TLSKeyFile == "" {
		return errors.New("principal listener, Redis and TLS certificate/key are required")
	}
	if _, _, err := net.SplitHostPort(c.ListenAddr); err != nil {
		return errors.New("invalid principal listener address")
	}
	if c.JWKSURLs["story"] == "" {
		return errors.New("exact trusted Story JWKS endpoint required")
	}
	if c.JWKSURLs["messaging"] == "" {
		return errors.New("exact trusted Messaging JWKS endpoint required")
	}
	if (c.JWKSClientCertFile == "") != (c.JWKSClientKeyFile == "") || (c.JWKSClientCertFile == "") {
		return errors.New("JWKS client certificate and key are required")
	}
	for issuer, endpoint := range c.JWKSURLs {
		u, err := url.Parse(endpoint)
		if !issuerPattern.MatchString(issuer) || err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
			return errors.New("invalid principal HTTPS JWKS endpoint")
		}
	}
	return nil
}
