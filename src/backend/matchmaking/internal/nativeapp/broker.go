package nativeapp

import (
	"crypto/tls"
	"crypto/x509"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

func nativeBrokerOptions(cfg nativeConfig) ([]nats.Option, error) {
	endpoint, err := url.Parse(cfg.values["NATS_URL"])
	if err != nil || endpoint.Scheme != "tls" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, errUnavailable
	}
	for _, key := range []string{"NATS_CREDS_FILE", "NATS_CA_FILE", "NATS_CLIENT_CERT_FILE", "NATS_CLIENT_KEY_FILE", "NATS_SERVER_NAME"} {
		if strings.TrimSpace(cfg.values[key]) == "" {
			return nil, errUnavailable
		}
	}
	certificate, err := tls.LoadX509KeyPair(cfg.values["NATS_CLIENT_CERT_FILE"], cfg.values["NATS_CLIENT_KEY_FILE"])
	if err != nil {
		return nil, errUnavailable
	}
	ca, err := os.ReadFile(cfg.values["NATS_CA_FILE"])
	if err != nil {
		return nil, errUnavailable
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, errUnavailable
	}
	// Credentials and trust are mandatory native inputs, never anonymous fallback.
	// The immutable launch/mount manifest separately establishes their custody.
	credentials, err := os.ReadFile(cfg.values["NATS_CREDS_FILE"])
	if err != nil || len(credentials) == 0 {
		return nil, errUnavailable
	}
	clear(credentials)
	return []nats.Option{nats.UserCredentials(cfg.values["NATS_CREDS_FILE"]), nats.Secure(&tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, Certificates: []tls.Certificate{certificate}, ServerName: cfg.values["NATS_SERVER_NAME"]}), nats.RetryOnFailedConnect(false), nats.Timeout(3 * time.Second)}, nil
}
