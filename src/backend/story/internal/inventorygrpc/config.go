package inventorygrpc

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

const envPrefix = "STORY_REFERENCE_INVENTORY_"

var configNames = []string{"GRPC_LISTEN", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE", "REPLAY_REDIS_ADDR", "REPLAY_REDIS_PASSWORD", "FILE_JWKS_URL", "JWKS_CA_FILE", "JWKS_CLIENT_CERT_FILE", "JWKS_CLIENT_KEY_FILE"}

type Config struct {
	ListenAddr, TLSCertFile, TLSKeyFile, ClientCAFile              string
	FileJWKSURL, JWKSCAFile, JWKSClientCertFile, JWKSClientKeyFile string
	ReplayAddr, ReplayPassword                                     string
}

// LoadFromEnv keeps this optional listener independent of ordinary Story/S2S settings.
// Presence of even an empty setting enables complete-configuration validation.
func LoadFromEnv(lookup func(string) (string, bool)) (Config, bool, error) {
	if lookup == nil {
		return Config{}, false, errors.New("inventory environment reader required")
	}
	values := make(map[string]string, len(configNames))
	enabled := false
	for _, name := range configNames {
		v, ok := lookup(envPrefix + name)
		enabled = enabled || ok
		values[name] = strings.TrimSpace(v)
	}
	if !enabled {
		return Config{}, false, nil
	}
	password, _ := lookup(envPrefix + "REPLAY_REDIS_PASSWORD")
	c := Config{ListenAddr: values["GRPC_LISTEN"], TLSCertFile: values["TLS_CERT_FILE"], TLSKeyFile: values["TLS_KEY_FILE"], ClientCAFile: values["CLIENT_CA_FILE"], FileJWKSURL: values["FILE_JWKS_URL"], JWKSCAFile: values["JWKS_CA_FILE"], JWKSClientCertFile: values["JWKS_CLIENT_CERT_FILE"], JWKSClientKeyFile: values["JWKS_CLIENT_KEY_FILE"], ReplayAddr: values["REPLAY_REDIS_ADDR"], ReplayPassword: password}
	return c, true, c.validate()
}

func (c Config) validate() error {
	if c.ListenAddr == "" || c.TLSCertFile == "" || c.TLSKeyFile == "" || c.ClientCAFile == "" || c.ReplayAddr == "" || c.FileJWKSURL == "" {
		return errors.New("complete inventory listener, mTLS, File JWKS and replay Redis configuration required")
	}
	if _, _, err := net.SplitHostPort(c.ListenAddr); err != nil {
		return errors.New("invalid inventory listener address")
	}
	if (c.JWKSClientCertFile == "") != (c.JWKSClientKeyFile == "") {
		return errors.New("inventory JWKS client certificate and key must be paired")
	}
	u, err := url.Parse(c.FileJWKSURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || u.ForceQuery {
		return errors.New("inventory File JWKS requires a configured HTTPS endpoint")
	}
	return nil
}
