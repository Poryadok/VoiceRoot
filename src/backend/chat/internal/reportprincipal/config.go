package reportprincipal

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

var reportreadEnvNames = []string{"CHAT_REPORT_POLICY_GRPC_LISTEN", "CHAT_REPORT_POLICY_TLS_CERT_FILE", "CHAT_REPORT_POLICY_TLS_KEY_FILE", "CHAT_REPORT_POLICY_CLIENT_CA_FILE", "CHAT_REPORT_POLICY_REPLAY_REDIS_ADDR", "CHAT_REPORT_POLICY_REPLAY_REDIS_PASSWORD"}

type Config struct {
	ListenAddr, CertFile, KeyFile, ClientCAFile, ReplayAddr, ReplayPassword, JWKSCAFile, JWKSClientCertFile, JWKSClientKeyFile string
	JWKSURLs                                                                                                                   map[string]string
	RefreshAfter, HardExpiry, UnknownKIDCooldown                                                                               time.Duration
}

// LoadFromEnv enables this listener only for reportread-owned settings. Shared JWKS configuration used
// by another Chat listener must not implicitly activate an unrelated transport.
func LoadFromEnv() (Config, bool, error) {
	enabled := false
	for _, name := range reportreadEnvNames {
		if _, present := os.LookupEnv(name); present {
			enabled = true
		}
	}
	if !enabled {
		return Config{}, false, nil
	}
	c := Config{ListenAddr: ":9099", CertFile: os.Getenv("CHAT_REPORT_POLICY_TLS_CERT_FILE"), KeyFile: os.Getenv("CHAT_REPORT_POLICY_TLS_KEY_FILE"),
		ClientCAFile: os.Getenv("CHAT_REPORT_POLICY_CLIENT_CA_FILE"), ReplayAddr: os.Getenv("CHAT_REPORT_POLICY_REPLAY_REDIS_ADDR"),
		ReplayPassword: os.Getenv("CHAT_REPORT_POLICY_REPLAY_REDIS_PASSWORD"), JWKSCAFile: os.Getenv("S2S_JWKS_CA_FILE"),
		JWKSClientCertFile: os.Getenv("S2S_JWKS_CLIENT_CERT_FILE"), JWKSClientKeyFile: os.Getenv("S2S_JWKS_CLIENT_KEY_FILE")}
	if value, present := os.LookupEnv("CHAT_REPORT_POLICY_GRPC_LISTEN"); present {
		c.ListenAddr = value
	}
	var endpoints map[string]string
	if err := json.Unmarshal([]byte(os.Getenv("S2S_JWKS_URLS_JSON")), &endpoints); err != nil {
		return c, true, errors.New("reportread listener requires messaging HTTPS JWKS endpoints")
	}
	c.JWKSURLs = map[string]string{"messaging": endpoints["messaging"]}
	var err error
	if c.RefreshAfter, err = reportreadDuration("S2S_JWKS_REFRESH_AFTER", 30*time.Second); err != nil {
		return c, true, err
	}
	if c.HardExpiry, err = reportreadDuration("S2S_JWKS_HARD_EXPIRY", 2*time.Minute); err != nil {
		return c, true, err
	}
	if c.UnknownKIDCooldown, err = reportreadDuration("S2S_UNKNOWN_KID_COOLDOWN", 5*time.Second); err != nil {
		return c, true, err
	}
	return c, true, c.validate()
}

func reportreadDuration(name string, fallback time.Duration) (time.Duration, error) {
	value, present := os.LookupEnv(name)
	if !present {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid reportread principal duration %s", name)
	}
	return duration, nil
}

func (c Config) validate() error {
	for _, value := range []string{c.ListenAddr, c.CertFile, c.KeyFile, c.ClientCAFile, c.ReplayAddr} {
		if value == "" || value != strings.TrimSpace(value) {
			return errors.New("reportread principal configuration is incomplete")
		}
	}
	if _, _, err := net.SplitHostPort(c.ListenAddr); err != nil {
		return errors.New("invalid reportread listener address")
	}
	if len(c.JWKSURLs) != 1 || c.RefreshAfter <= 0 || c.HardExpiry < c.RefreshAfter || c.UnknownKIDCooldown <= 0 {
		return errors.New("invalid reportread JWKS cache configuration")
	}
	for _, issuer := range []string{"messaging"} {
		endpoint := c.JWKSURLs[issuer]
		u, err := url.Parse(endpoint)
		if err != nil || endpoint != strings.TrimSpace(endpoint) || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
			return errors.New("reportread messaging JWKS must be exact HTTPS endpoints")
		}
	}
	if (c.JWKSClientCertFile == "") != (c.JWKSClientKeyFile == "") {
		return errors.New("reportread JWKS client certificate/key must be paired")
	}
	return nil
}
