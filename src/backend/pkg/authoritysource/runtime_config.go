package authoritysource

import (
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
)

type RuntimeConfig struct {
	Owner                                        authorityv1.AuthorityOwner
	JWKSURL, JWKSCAFile                          string
	TLSCertFile, TLSKeyFile, ClientCAFile        string
	ReplayAddr, ReplayPassword, ListenAddr       string
	RefreshAfter, HardExpiry, UnknownKIDCooldown time.Duration
}

func LoadRuntimeConfig(owner authorityv1.AuthorityOwner, defaultListen string) (RuntimeConfig, bool, error) {
	audience := Audience(owner)
	if audience == "" {
		return RuntimeConfig{}, false, errors.New("unknown source owner")
	}
	prefix := strings.ToUpper(audience) + "_AUTHORITY_SOURCE_"
	flag, present := os.LookupEnv(prefix + "ENABLED")
	names := []string{"JWKS_CA_FILE", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE", "REPLAY_REDIS_ADDR", "REPLAY_REDIS_PASSWORD", "GRPC_LISTEN"}
	if !present {
		for _, name := range names {
			if _, configured := os.LookupEnv(prefix + name); configured {
				return RuntimeConfig{}, false, errors.New("explicit source activation flag required")
			}
		}
		return RuntimeConfig{}, false, nil
	}
	if flag == "false" {
		return RuntimeConfig{}, false, nil
	}
	if flag != "true" {
		return RuntimeConfig{}, true, errors.New("source activation flag must be true or false")
	}
	var endpoints map[string]string
	if json.Unmarshal([]byte(os.Getenv("S2S_JWKS_URLS_JSON")), &endpoints) != nil {
		return RuntimeConfig{}, true, errors.New("invalid source JWKS configuration")
	}
	cfg := RuntimeConfig{Owner: owner, JWKSURL: endpoints["federation"], JWKSCAFile: strings.TrimSpace(os.Getenv(prefix + "JWKS_CA_FILE")), TLSCertFile: strings.TrimSpace(os.Getenv(prefix + "TLS_CERT_FILE")), TLSKeyFile: strings.TrimSpace(os.Getenv(prefix + "TLS_KEY_FILE")), ClientCAFile: strings.TrimSpace(os.Getenv(prefix + "CLIENT_CA_FILE")), ReplayAddr: strings.TrimSpace(os.Getenv(prefix + "REPLAY_REDIS_ADDR")), ReplayPassword: os.Getenv(prefix + "REPLAY_REDIS_PASSWORD"), ListenAddr: defaultListen, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second}
	if value, configured := os.LookupEnv(prefix + "GRPC_LISTEN"); configured {
		cfg.ListenAddr = strings.TrimSpace(value)
	}
	if err := cfg.validate(); err != nil {
		return RuntimeConfig{}, true, err
	}
	return cfg, true, nil
}

func (c RuntimeConfig) validate() error {
	endpoint, err := url.Parse(c.JWKSURL)
	if Audience(c.Owner) == "" || err != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.Fragment != "" {
		return errors.New("trusted Federation HTTPS JWKS endpoint required")
	}
	if c.TLSCertFile == "" || c.TLSKeyFile == "" || c.ClientCAFile == "" || c.ReplayAddr == "" {
		return errors.New("source server TLS, client CA and replay Redis required")
	}
	if _, _, err := net.SplitHostPort(c.ListenAddr); err != nil {
		return errors.New("invalid source listener address")
	}
	if c.RefreshAfter <= 0 || c.HardExpiry < c.RefreshAfter || c.UnknownKIDCooldown <= 0 {
		return errors.New("invalid source JWKS cache policy")
	}
	return nil
}
