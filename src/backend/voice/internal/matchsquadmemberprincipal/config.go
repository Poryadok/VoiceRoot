package matchsquadmemberprincipal

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

const envPrefix = "VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_"

var envSuffixes = []string{"GRPC_LISTEN", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE", "JWKS_URL", "JWKS_CA_FILE", "REDIS_ADDR", "REDIS_PASSWORD"}

type Config struct {
	ListenerAddr, TLSCertFile, TLSKeyFile, ClientCAFile string
	JWKSURL, JWKSCAFile, RedisAddr, RedisPassword       string
}

func LoadFromEnv(lookup func(string) (string, bool)) (Config, bool, error) {
	if lookup == nil {
		return Config{}, false, errors.New("environment lookup is required")
	}
	values := make(map[string]string, len(envSuffixes))
	configured := false
	for _, key := range envSuffixes {
		value, ok := lookup(envPrefix + key)
		configured = configured || ok
		values[key] = strings.TrimSpace(value)
	}
	if !configured {
		return Config{}, false, nil
	}
	c := Config{
		ListenerAddr: values["GRPC_LISTEN"], TLSCertFile: values["TLS_CERT_FILE"], TLSKeyFile: values["TLS_KEY_FILE"],
		ClientCAFile: values["CLIENT_CA_FILE"], JWKSURL: values["JWKS_URL"], JWKSCAFile: values["JWKS_CA_FILE"],
		RedisAddr: values["REDIS_ADDR"], RedisPassword: values["REDIS_PASSWORD"],
	}
	if c.ListenerAddr == "" {
		c.ListenerAddr = ":9093"
	}
	return c, true, c.Validate()
}

func (c Config) Validate() error {
	if c.TLSCertFile == "" || c.TLSKeyFile == "" || c.ClientCAFile == "" || c.JWKSCAFile == "" || c.RedisAddr == "" {
		return errors.New("Voice MatchSquad member principal listener requires TLS, JWKS CA, and Redis configuration")
	}
	if _, _, err := net.SplitHostPort(c.ListenerAddr); err != nil {
		return errors.New("invalid Voice MatchSquad member listener address")
	}
	u, err := url.Parse(c.JWKSURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || strings.ContainsAny(c.JWKSURL, "\r\n") {
		return errors.New("Voice MatchSquad member principal requires an exact HTTPS JWKS URL")
	}
	return nil
}
