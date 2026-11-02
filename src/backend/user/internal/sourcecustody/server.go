package sourcecustody

import (
	"encoding/hex"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ServerConfig struct {
	ControlDatabaseURL, ListenAddress        string
	TLSCertFile, TLSKeyFile, TLSClientCAFile string
	UserJWKSURL, UserJWKSCAFile              string
	RedisAddress, RedisPassword              string
	RedisTLSCAFile, RedisTLSServerName       string
	Binding                                  Binding
	Heartbeat                                time.Duration
	Operator                                 *ActivationOperatorConfig
	// Test-only listener control; not a source authority.
	testListen             func(string, string) (net.Listener, error)
	testAllowSyntheticOpen bool
}

func LoadServerConfig(lookup func(string) (string, bool)) (ServerConfig, error) {
	if lookup == nil {
		return ServerConfig{}, ErrUnavailable
	}
	names := []string{"CUSTODY_CONTROL_DATABASE_URL", "CUSTODY_LISTEN_ADDR", "CUSTODY_TLS_CERT_FILE", "CUSTODY_TLS_KEY_FILE", "CUSTODY_TLS_CLIENT_CA_FILE", "CUSTODY_USER_JWKS_URL", "CUSTODY_USER_JWKS_CA_FILE", "CUSTODY_REDIS_ADDR", "CUSTODY_EPOCH", "CUSTODY_SOURCE_ID", "CUSTODY_HISTORY_GENERATION", "CUSTODY_USER_SOURCE_GENERATION", "CUSTODY_KEYSET_DIGEST", "CUSTODY_HEARTBEAT", "CUSTODY_REDIS_TLS_CA_FILE", "CUSTODY_REDIS_TLS_SERVER_NAME"}
	values := make([]string, len(names))
	for i, name := range names {
		v, ok := lookup(name)
		if !ok || v == "" || strings.TrimSpace(v) != v {
			return ServerConfig{}, ErrUnavailable
		}
		values[i] = v
	}
	epoch, err := strconv.ParseInt(values[8], 10, 64)
	if err != nil || strconv.FormatInt(epoch, 10) != values[8] {
		return ServerConfig{}, ErrUnavailable
	}
	h, err := uuid.Parse(values[10])
	if err != nil || h.String() != values[10] {
		return ServerConfig{}, ErrUnavailable
	}
	u, err := uuid.Parse(values[11])
	if err != nil || u.String() != values[11] {
		return ServerConfig{}, ErrUnavailable
	}
	digest, err := hex.DecodeString(values[12])
	if err != nil || len(digest) != 32 || hex.EncodeToString(digest) != values[12] {
		return ServerConfig{}, ErrUnavailable
	}
	var d [32]byte
	copy(d[:], digest)
	heartbeat, err := time.ParseDuration(values[13])
	if err != nil {
		return ServerConfig{}, ErrUnavailable
	}
	password, _ := lookup("CUSTODY_REDIS_PASSWORD")
	cfg := ServerConfig{ControlDatabaseURL: values[0], ListenAddress: values[1], TLSCertFile: values[2], TLSKeyFile: values[3], TLSClientCAFile: values[4], UserJWKSURL: values[5], UserJWKSCAFile: values[6], RedisAddress: values[7], RedisPassword: password, RedisTLSCAFile: values[14], RedisTLSServerName: values[15], Binding: Binding{Epoch: epoch, SourceID: values[9], HistoryGeneration: h, UserSourceGeneration: u, KeysetDigest: d}, Heartbeat: heartbeat}
	cfg.Operator, err = loadActivationOperatorConfig(lookup)
	if err != nil {
		return ServerConfig{}, ErrUnavailable
	}
	if !cfg.valid() {
		return ServerConfig{}, ErrUnavailable
	}
	return cfg, nil
}

func (c ServerConfig) valid() bool {
	for _, value := range []string{c.ControlDatabaseURL, c.ListenAddress, c.TLSCertFile, c.TLSKeyFile, c.TLSClientCAFile, c.UserJWKSURL, c.UserJWKSCAFile, c.RedisAddress, c.RedisTLSCAFile, c.RedisTLSServerName} {
		if value == "" || strings.TrimSpace(value) != value {
			return false
		}
	}
	if !c.Binding.valid() || c.Heartbeat <= 0 || c.Heartbeat > 5*time.Second {
		return false
	}
	if !validServerAddress(c.ListenAddress, true) {
		return false
	}
	if !validServerAddress(c.RedisAddress, false) || strings.ContainsAny(c.RedisTLSServerName, "/@ \\?#:") {
		return false
	}
	j, err := url.Parse(c.UserJWKSURL)
	if err != nil || j.Scheme != "https" || j.Hostname() == "" || j.User != nil || j.Fragment != "" || j.RawQuery != "" {
		return false
	}
	return validControlURL(c.ControlDatabaseURL)
}

func validServerAddress(addr string, allowZero bool) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || host == "" || strings.ContainsAny(host, "/@ \\?#") {
		return false
	}
	n, err := strconv.Atoi(port)
	return err == nil && n >= 0 && n <= 65535 && (allowZero || n > 0)
}

func validControlURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" || u.Port() == "" || u.User == nil || u.User.Username() == "" || u.Path == "" || u.Path == "/" || u.Fragment != "" || strings.ContainsAny(u.Hostname(), ",/ ") {
		return false
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 {
		return false
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return false
	}
	for k, v := range query {
		if len(v) != 1 || (k != "sslmode" && k != "sslrootcert" && k != "sslcert" && k != "sslkey") || v[0] == "" {
			return false
		}
	}
	return query.Get("sslmode") == "verify-full" && query.Get("sslrootcert") != ""
}
