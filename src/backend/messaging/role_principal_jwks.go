package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type rolePrincipalJWKSConfig struct{ listen, certFile, keyFile, clientCAFile string }

func loadRolePrincipalJWKSConfig(lookup func(string) (string, bool)) (rolePrincipalJWKSConfig, bool, error) {
	names := []string{"MESSAGING_ROLE_JWKS_LISTEN", "MESSAGING_ROLE_JWKS_TLS_CERT_FILE", "MESSAGING_ROLE_JWKS_TLS_KEY_FILE", "MESSAGING_ROLE_JWKS_CLIENT_CA_FILE"}
	values := make([]string, len(names))
	enabled := false
	for i, name := range names {
		value, set := lookup(name)
		enabled = enabled || set
		values[i] = strings.TrimSpace(value)
	}
	if !enabled {
		return rolePrincipalJWKSConfig{}, false, nil
	}
	for i, value := range values {
		if value == "" {
			return rolePrincipalJWKSConfig{}, true, fmt.Errorf("Messaging Role JWKS requires %s", names[i])
		}
	}
	if _, _, err := net.SplitHostPort(values[0]); err != nil {
		return rolePrincipalJWKSConfig{}, true, errors.New("Messaging Role JWKS listen address is invalid")
	}
	return rolePrincipalJWKSConfig{values[0], values[1], values[2], values[3]}, true, nil
}

// start publishes the existing runtime's immutable public-key handler under a
// separate transport trust domain; it never creates or reloads signing keys.
func (c rolePrincipalJWKSConfig) start(handler http.Handler) (*http.Server, error) {
	if handler == nil {
		return nil, errors.New("Messaging Role JWKS requires the existing signing runtime")
	}
	cert, err := tls.LoadX509KeyPair(c.certFile, c.keyFile)
	if err != nil {
		return nil, fmt.Errorf("Messaging Role JWKS certificate: %w", err)
	}
	caPEM, err := os.ReadFile(c.clientCAFile)
	if err != nil {
		return nil, fmt.Errorf("Messaging Role JWKS client CA: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("Messaging Role JWKS client CA contains no certificates")
	}
	listener, err := net.Listen("tcp", c.listen)
	if err != nil {
		return nil, fmt.Errorf("Messaging Role JWKS listen: %w", err)
	}
	server := &http.Server{Addr: listener.Addr().String(), Handler: handler, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}}
	go func() {
		if err := server.ServeTLS(listener, "", ""); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Messaging Role JWKS server exited: %v\n", err)
		}
	}()
	return server, nil
}
