package identityassociation

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
)

type AuthParentRuntimeConfig struct {
	Address, ServerName, CAFile, CertFile, KeyFile string
}

type AuthParentRuntime struct {
	Client *AuthParentClient
	conn   *grpc.ClientConn
}

func LoadAuthParentRuntimeConfig(lookup func(string) (string, bool)) (AuthParentRuntimeConfig, bool, error) {
	var empty AuthParentRuntimeConfig
	if lookup == nil {
		return empty, false, ErrIncomplete
	}
	names := []string{"USER_IDENTITY_AUTH_GRPC_ADDR", "USER_IDENTITY_AUTH_TLS_SERVER_NAME", "USER_IDENTITY_AUTH_TLS_CA_FILE", "USER_IDENTITY_AUTH_TLS_CERT_FILE", "USER_IDENTITY_AUTH_TLS_KEY_FILE"}
	values := make([]string, len(names))
	enabled := false
	for i, name := range names {
		value, present := lookup(name)
		enabled = enabled || present
		values[i] = value
	}
	if !enabled {
		return empty, false, nil
	}
	config := AuthParentRuntimeConfig{Address: values[0], ServerName: values[1], CAFile: values[2], CertFile: values[3], KeyFile: values[4]}
	if !config.valid() {
		return empty, false, ErrIncomplete
	}
	return config, true, nil
}

func (c AuthParentRuntimeConfig) valid() bool {
	for _, value := range []string{c.Address, c.ServerName, c.CAFile, c.CertFile, c.KeyFile} {
		if value == "" || strings.TrimSpace(value) != value {
			return false
		}
	}
	host, port, err := net.SplitHostPort(c.Address)
	if err != nil || host == "" || strings.ContainsAny(host, "/@ \\?#") || strings.ContainsAny(c.ServerName, "/@ \\?#:") {
		return false
	}
	parsed, err := strconv.Atoi(port)
	return err == nil && parsed > 0 && parsed <= 65535 && strconv.Itoa(parsed) == port
}

// OpenAuthParentRuntime proves a TLS connection can be established. This is
// not proof of Auth source continuity, peer domain readiness, or User custody.
func OpenAuthParentRuntime(ctx context.Context, cfg AuthParentRuntimeConfig, issuer *principal.Issuer) (*AuthParentRuntime, error) {
	if ctx == nil || ctx.Err() != nil || !cfg.valid() || issuer == nil {
		return nil, ErrIncomplete
	}
	ca, err := readBoundedKeyFile(cfg.CAFile, 1<<20)
	if err != nil || len(ca) == 0 {
		return nil, ErrIncomplete
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, ErrIncomplete
	}
	identity, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, ErrIncomplete
	}
	client, conn, err := NewAuthParentClient(AuthParentTransport{Address: cfg.Address, ServerName: cfg.ServerName, Roots: roots, Certificate: identity}, issuer)
	if err != nil {
		return nil, ErrIncomplete
	}
	conn.Connect()
	ready, cancel := context.WithTimeout(ctx, 3*time.Second)
	err = grpcclient.WaitForReady(ready, conn)
	cancel()
	if err != nil || ctx.Err() != nil {
		_ = conn.Close()
		return nil, ErrIncomplete
	}
	return &AuthParentRuntime{Client: client, conn: conn}, nil
}

func (r *AuthParentRuntime) Close() error {
	if r == nil || r.conn == nil {
		return nil
	}
	return r.conn.Close()
}
