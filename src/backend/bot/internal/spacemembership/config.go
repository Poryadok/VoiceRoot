package spacemembership

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"os"
	"strings"

	"github.com/google/uuid"
)

var ErrUnavailable = errors.New("protected Bot installation dependency unavailable")

type Endpoint struct{ Address, CAFile, CertFile, KeyFile, ServerName string }
type Config struct {
	Space, Role, Chat, Messaging Endpoint
	SourceGeneration             uuid.UUID
	SourceKind                   string
}

// Presence is observed separately from value: a present blank field is invalid,
// never an instruction to fall back to ordinary SPACE_GRPC_ADDR/ROLE_GRPC_ADDR.
func LoadConfig(lookup func(string) (string, bool)) (Config, bool, error) {
	if lookup == nil {
		return Config{}, false, ErrUnavailable
	}
	var cfg Config
	enabled := false
	for _, prefix := range []string{"BOT_SPACE_MEMBERSHIP_", "BOT_INSTALL_ROLE_", "BOT_INSTALL_CHAT_", "BOT_INSTALL_MESSAGING_"} {
		for _, suffix := range []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_CERT_FILE", "TLS_KEY_FILE", "TLS_SERVER_NAME"} {
			if _, ok := lookup(prefix + suffix); ok {
				enabled = true
			}
		}
	}
	for _, name := range []string{"BOT_SPACE_MEMBERSHIP_SOURCE_GENERATION", "BOT_SPACE_MEMBERSHIP_SOURCE_KIND"} {
		if _, ok := lookup(name); ok {
			enabled = true
		}
	}
	if !enabled {
		return cfg, false, nil
	}
	var err error
	cfg.Space, err = loadEndpoint(lookup, "BOT_SPACE_MEMBERSHIP_")
	if err != nil {
		return Config{}, true, err
	}
	cfg.Role, err = loadEndpoint(lookup, "BOT_INSTALL_ROLE_")
	if err != nil {
		return Config{}, true, err
	}
	cfg.Chat, err = loadEndpoint(lookup, "BOT_INSTALL_CHAT_")
	if err != nil {
		return Config{}, true, err
	}
	cfg.Messaging, err = loadEndpoint(lookup, "BOT_INSTALL_MESSAGING_")
	if err != nil {
		return Config{}, true, err
	}
	raw, _ := lookup("BOT_SPACE_MEMBERSHIP_SOURCE_GENERATION")
	cfg.SourceGeneration, err = uuid.Parse(raw)
	if err != nil || cfg.SourceGeneration == uuid.Nil || cfg.SourceGeneration.String() != raw {
		return Config{}, true, ErrUnavailable
	}
	cfg.SourceKind, _ = lookup("BOT_SPACE_MEMBERSHIP_SOURCE_KIND")
	if cfg.SourceKind != "OWNED_FRESH" && cfg.SourceKind != "KNOWN_RETAINED" {
		return Config{}, true, ErrUnavailable
	}
	return cfg, true, nil
}
func loadEndpoint(lookup func(string) (string, bool), prefix string) (Endpoint, error) {
	var values []string
	for _, suffix := range []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_CERT_FILE", "TLS_KEY_FILE"} {
		v, ok := lookup(prefix + suffix)
		if !ok || v == "" || strings.TrimSpace(v) != v {
			return Endpoint{}, ErrUnavailable
		}
		values = append(values, v)
	}
	name, present := lookup(prefix + "TLS_SERVER_NAME")
	if present && (name == "" || strings.TrimSpace(name) != name) {
		return Endpoint{}, ErrUnavailable
	}
	if !present {
		host, _, err := net.SplitHostPort(strings.TrimPrefix(values[0], "dns:///"))
		if err != nil || host == "" {
			return Endpoint{}, ErrUnavailable
		}
		name = host
	}
	return Endpoint{values[0], values[1], values[2], values[3], name}, nil
}

func LoadEndpoint(lookup func(string) (string, bool), prefix string) (Endpoint, error) {
	return loadEndpoint(lookup, prefix)
}
func (e Endpoint) TLS() (*tls.Config, error) {
	if e.Address == "" || e.ServerName == "" {
		return nil, ErrUnavailable
	}
	cert, err := tls.LoadX509KeyPair(e.CertFile, e.KeyFile)
	if err != nil {
		return nil, ErrUnavailable
	}
	ca, err := os.ReadFile(e.CAFile)
	if err != nil {
		return nil, ErrUnavailable
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, ErrUnavailable
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{cert}, ServerName: e.ServerName}, nil
}
