package voiceuserprincipalruntime

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
	JWKSURL, JWKSCAFile, JWKSClientCertFile, JWKSClientKeyFile string
	RefreshAfter, HardExpiry, UnknownKIDCooldown               time.Duration
	ReplayAddr, ReplayPassword                                 string
	TLSCertFile, TLSKeyFile, ClientCAFile, ListenAddr          string
}

func LoadFromEnv() (Config, bool, error) {
	const prefix = "VOICE_USER_PRINCIPAL_"
	names := []string{prefix + "REPLAY_REDIS_ADDR", prefix + "REPLAY_REDIS_PASSWORD", prefix + "TLS_CERT_FILE",
		prefix + "TLS_KEY_FILE", prefix + "CLIENT_CA_FILE", prefix + "GRPC_LISTEN"}
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
		JWKSCAFile:         strings.TrimSpace(os.Getenv("S2S_JWKS_CA_FILE")),
		JWKSClientCertFile: strings.TrimSpace(os.Getenv("S2S_JWKS_CLIENT_CERT_FILE")),
		JWKSClientKeyFile:  strings.TrimSpace(os.Getenv("S2S_JWKS_CLIENT_KEY_FILE")),
		TLSCertFile:        strings.TrimSpace(os.Getenv(prefix + "TLS_CERT_FILE")), TLSKeyFile: strings.TrimSpace(os.Getenv(prefix + "TLS_KEY_FILE")),
		ClientCAFile: strings.TrimSpace(os.Getenv(prefix + "CLIENT_CA_FILE")), ListenAddr: ":9092",
	}
	if value, ok := os.LookupEnv(prefix + "GRPC_LISTEN"); ok {
		cfg.ListenAddr = strings.TrimSpace(value)
	}
	var endpoints map[string]string
	if err := json.Unmarshal([]byte(os.Getenv("S2S_JWKS_URLS_JSON")), &endpoints); err != nil {
		return Config{}, true, errors.New("invalid Voice user principal JWKS configuration")
	}
	cfg.JWKSURL = endpoints["gateway"]
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
	value, ok := os.LookupEnv(name)
	if !ok {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return duration, nil
}

func (c Config) validate() error {
	if c.ReplayAddr == "" || c.TLSCertFile == "" || c.TLSKeyFile == "" || c.ClientCAFile == "" ||
		c.JWKSClientCertFile == "" || c.JWKSClientKeyFile == "" {
		return errors.New("Voice user principal Redis, TLS and JWKS client credentials are required")
	}
	if c.RefreshAfter <= 0 || c.HardExpiry < c.RefreshAfter || c.UnknownKIDCooldown <= 0 {
		return errors.New("invalid Voice user principal cache policy")
	}
	if _, _, err := net.SplitHostPort(c.ListenAddr); err != nil {
		return errors.New("invalid Voice user principal listener")
	}
	if strings.TrimSpace(c.JWKSCAFile) == "" {
		return errors.New("Voice user principal JWKS CA is required")
	}
	endpoint, err := url.Parse(c.JWKSURL)
	if err != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.Fragment != "" {
		return errors.New("trusted Gateway HTTPS JWKS endpoint required")
	}
	return nil
}
