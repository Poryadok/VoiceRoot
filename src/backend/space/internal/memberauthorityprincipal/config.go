package memberauthorityprincipal

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

var memberauthorityEnvNames = []string{"SPACE_MEMBER_AUTHORITY_GRPC_LISTEN", "SPACE_MEMBER_AUTHORITY_TLS_CERT_FILE", "SPACE_MEMBER_AUTHORITY_TLS_KEY_FILE", "SPACE_MEMBER_AUTHORITY_CLIENT_CA_FILE", "SPACE_MEMBER_AUTHORITY_REPLAY_REDIS_ADDR", "SPACE_MEMBER_AUTHORITY_REPLAY_REDIS_PASSWORD"}

type Config struct {
	ListenAddr, CertFile, KeyFile, ClientCAFile, ReplayAddr, ReplayPassword, JWKSCAFile, JWKSClientCertFile, JWKSClientKeyFile string
	JWKSURLs                                                                                                                   map[string]string
	RefreshAfter, HardExpiry, UnknownKIDCooldown                                                                               time.Duration
}

// LoadFromEnv enables this listener only for memberauthority-owned settings. Shared JWKS configuration used
// by another Space listener must not implicitly activate an unrelated transport.
func LoadFromEnv() (Config, bool, error) {
	enabled := false
	for _, name := range memberauthorityEnvNames {
		if _, present := os.LookupEnv(name); present {
			enabled = true
		}
	}
	if !enabled {
		return Config{}, false, nil
	}
	c := Config{ListenAddr: ":9096", CertFile: os.Getenv("SPACE_MEMBER_AUTHORITY_TLS_CERT_FILE"), KeyFile: os.Getenv("SPACE_MEMBER_AUTHORITY_TLS_KEY_FILE"),
		ClientCAFile: os.Getenv("SPACE_MEMBER_AUTHORITY_CLIENT_CA_FILE"), ReplayAddr: os.Getenv("SPACE_MEMBER_AUTHORITY_REPLAY_REDIS_ADDR"),
		ReplayPassword: os.Getenv("SPACE_MEMBER_AUTHORITY_REPLAY_REDIS_PASSWORD"), JWKSCAFile: os.Getenv("S2S_JWKS_CA_FILE"),
		JWKSClientCertFile: os.Getenv("S2S_JWKS_CLIENT_CERT_FILE"), JWKSClientKeyFile: os.Getenv("S2S_JWKS_CLIENT_KEY_FILE")}
	if value, present := os.LookupEnv("SPACE_MEMBER_AUTHORITY_GRPC_LISTEN"); present {
		c.ListenAddr = value
	}
	var endpoints map[string]string
	if err := json.Unmarshal([]byte(os.Getenv("S2S_JWKS_URLS_JSON")), &endpoints); err != nil {
		return c, true, errors.New("memberauthority listener requires role HTTPS JWKS endpoints")
	}
	c.JWKSURLs = map[string]string{"role": endpoints["role"]}
	var err error
	if c.RefreshAfter, err = memberauthorityDuration("S2S_JWKS_REFRESH_AFTER", 30*time.Second); err != nil {
		return c, true, err
	}
	if c.HardExpiry, err = memberauthorityDuration("S2S_JWKS_HARD_EXPIRY", 2*time.Minute); err != nil {
		return c, true, err
	}
	if c.UnknownKIDCooldown, err = memberauthorityDuration("S2S_UNKNOWN_KID_COOLDOWN", 5*time.Second); err != nil {
		return c, true, err
	}
	return c, true, c.validate()
}

func memberauthorityDuration(name string, fallback time.Duration) (time.Duration, error) {
	value, present := os.LookupEnv(name)
	if !present {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid memberauthority principal duration %s", name)
	}
	return duration, nil
}

func (c Config) validate() error {
	for _, value := range []string{c.ListenAddr, c.CertFile, c.KeyFile, c.ClientCAFile, c.ReplayAddr} {
		if value == "" || value != strings.TrimSpace(value) {
			return errors.New("memberauthority principal configuration is incomplete")
		}
	}
	if _, _, err := net.SplitHostPort(c.ListenAddr); err != nil {
		return errors.New("invalid memberauthority listener address")
	}
	if len(c.JWKSURLs) != 1 || c.RefreshAfter <= 0 || c.HardExpiry < c.RefreshAfter || c.UnknownKIDCooldown <= 0 {
		return errors.New("invalid memberauthority JWKS cache configuration")
	}
	for _, issuer := range []string{"role"} {
		endpoint := c.JWKSURLs[issuer]
		u, err := url.Parse(endpoint)
		if err != nil || endpoint != strings.TrimSpace(endpoint) || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
			return errors.New("memberauthority role JWKS must be exact HTTPS endpoints")
		}
	}
	if (c.JWKSClientCertFile == "") != (c.JWKSClientKeyFile == "") {
		return errors.New("memberauthority JWKS client certificate/key must be paired")
	}
	return nil
}
