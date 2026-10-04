package matchsquadprincipal

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

const envPrefix = "CHAT_MATCH_SQUAD_PRINCIPAL_"

type Config struct {
	ListenerAddr        string
	TLSCertFile         string
	TLSKeyFile          string
	ClientCAFile        string
	JWKSURL             string
	JWKSCAFile          string
	ReplayRedisAddr     string
	ReplayRedisPassword string
}

func LoadFromEnv(lookupEnv func(string) (string, bool)) (Config, bool, error) {
	if lookupEnv == nil {
		return Config{}, false, errors.New("MatchSquad principal environment reader is required")
	}
	suffixes := []string{"GRPC_LISTEN", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE", "JWKS_URL", "JWKS_CA_FILE", "REPLAY_REDIS_ADDR", "REPLAY_REDIS_PASSWORD"}
	enabled := false
	values := make([]string, len(suffixes))
	present := make([]bool, len(suffixes))
	for i, suffix := range suffixes {
		value, ok := lookupEnv(envPrefix + suffix)
		values[i] = value
		present[i] = ok
		if ok {
			enabled = true
		}
	}
	if !enabled {
		return Config{}, false, nil
	}
	for _, ok := range present {
		if !ok {
			return Config{}, true, errors.New("MatchSquad principal listener, TLS, JWKS and replay Redis configuration must be complete")
		}
	}
	cfg := Config{
		ListenerAddr: strings.TrimSpace(values[0]), TLSCertFile: strings.TrimSpace(values[1]),
		TLSKeyFile: strings.TrimSpace(values[2]), ClientCAFile: strings.TrimSpace(values[3]),
		JWKSURL: strings.TrimSpace(values[4]), JWKSCAFile: strings.TrimSpace(values[5]),
		ReplayRedisAddr: strings.TrimSpace(values[6]), ReplayRedisPassword: values[7],
	}
	if err := cfg.validate(); err != nil {
		return Config{}, true, err
	}
	return cfg, true, nil
}

func (c Config) validate() error {
	if c.ListenerAddr == "" || c.TLSCertFile == "" || c.TLSKeyFile == "" || c.ClientCAFile == "" || c.JWKSURL == "" || c.JWKSCAFile == "" || c.ReplayRedisAddr == "" {
		return errors.New("MatchSquad principal listener, TLS, JWKS and replay Redis configuration must be complete")
	}
	if _, _, err := net.SplitHostPort(c.ListenerAddr); err != nil {
		return errors.New("invalid MatchSquad principal listener address")
	}
	parsed, err := url.ParseRequestURI(c.JWKSURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("MatchSquad principal JWKS URL must be a trusted HTTPS URL")
	}
	if parsed.Path == "" || parsed.RawPath != "" {
		return fmt.Errorf("MatchSquad principal JWKS URL must have a canonical path")
	}
	return nil
}
