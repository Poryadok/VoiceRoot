package matchsquadprincipal

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

const envPrefix = "VOICE_MATCH_SQUAD_PRINCIPAL_"

var envSuffixes = []string{
	"GRPC_LISTEN", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE",
	"JWKS_URL", "JWKS_CA_FILE", "REPLAY_REDIS_ADDR", "REPLAY_REDIS_PASSWORD",
}

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

// LoadFromEnv disables the capability only when none of its owned variables
// exist. A partially declared capability is an error, including empty values.
func LoadFromEnv(lookupEnv func(string) (string, bool)) (Config, bool, error) {
	if lookupEnv == nil {
		return Config{}, false, errors.New("environment lookup is required")
	}
	values := make(map[string]string, len(envSuffixes))
	configured := false
	for _, suffix := range envSuffixes {
		value, present := lookupEnv(envPrefix + suffix)
		if present {
			configured = true
		}
		values[suffix] = strings.TrimSpace(value)
	}
	if !configured {
		return Config{}, false, nil
	}
	cfg := Config{
		ListenerAddr: values["GRPC_LISTEN"], TLSCertFile: values["TLS_CERT_FILE"],
		TLSKeyFile: values["TLS_KEY_FILE"], ClientCAFile: values["CLIENT_CA_FILE"],
		JWKSURL: values["JWKS_URL"], JWKSCAFile: values["JWKS_CA_FILE"],
		ReplayRedisAddr: values["REPLAY_REDIS_ADDR"], ReplayRedisPassword: values["REPLAY_REDIS_PASSWORD"],
	}
	if cfg.ListenerAddr == "" {
		cfg.ListenerAddr = ":9092"
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, true, err
	}
	return cfg, true, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.TLSCertFile) == "" || strings.TrimSpace(c.TLSKeyFile) == "" ||
		strings.TrimSpace(c.ClientCAFile) == "" || strings.TrimSpace(c.ReplayRedisAddr) == "" {
		return errors.New("Voice MatchSquad principal listener requires TLS identity, client CA, and replay Redis")
	}
	if _, _, err := net.SplitHostPort(strings.TrimSpace(c.ListenerAddr)); err != nil {
		return errors.New("invalid Voice MatchSquad principal listener address")
	}
	parsed, err := url.Parse(strings.TrimSpace(c.JWKSURL))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("Voice MatchSquad principal requires an exact HTTPS JWKS URL without query credentials")
	}
	if strings.ContainsAny(c.JWKSURL, "\r\n") {
		return fmt.Errorf("invalid Voice MatchSquad principal JWKS URL")
	}
	return nil
}
