package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"os"
	"strings"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/usercapture"
)

type userCaptureRuntimeConfig struct{ address, caFile, serverName, certFile, keyFile string }
type userCaptureRuntime struct {
	client *usercapture.Client
	conn   *grpc.ClientConn
}

func loadUserCaptureRuntimeConfig(getenv func(string) string, publicEnabled bool) (userCaptureRuntimeConfig, bool, error) {
	names := []string{"SPACE_USER_CAPTURE_GRPC_ADDR", "SPACE_USER_CAPTURE_TLS_CA_FILE", "SPACE_USER_CAPTURE_TLS_SERVER_NAME", "SPACE_USER_CAPTURE_CLIENT_CERT_FILE", "SPACE_USER_CAPTURE_CLIENT_KEY_FILE"}
	values := make([]string, len(names))
	enabled := false
	for i, name := range names {
		values[i] = getenv(name)
		enabled = enabled || values[i] != ""
	}
	if !enabled {
		return userCaptureRuntimeConfig{}, false, nil
	}
	for i, value := range values {
		if value == "" || value != strings.TrimSpace(value) {
			return userCaptureRuntimeConfig{}, false, fmt.Errorf("%s is required without surrounding whitespace", names[i])
		}
	}
	if strings.TrimSpace(getenv("DATABASE_URL")) == "" || !publicEnabled {
		return userCaptureRuntimeConfig{}, false, errors.New("Space User capture requires DATABASE_URL and protected Gateway listener")
	}
	return userCaptureRuntimeConfig{address: values[0], caFile: values[1], serverName: values[2], certFile: values[3], keyFile: values[4]}, true, nil
}

// newUserCaptureRuntime uses only caller-owned workload material. It installs
// no identity provider, signing-key fallback or ordinary User transport.
func newUserCaptureRuntime(ctx context.Context, c userCaptureRuntimeConfig, issuer *principal.Issuer) (*userCaptureRuntime, error) {
	if ctx == nil || issuer == nil {
		return nil, errors.New("Space User capture requires context and Space principal issuer")
	}
	ca, err := os.ReadFile(c.caFile)
	if err != nil {
		return nil, fmt.Errorf("Space User capture CA: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, errors.New("Space User capture CA contains no certificates")
	}
	identity, err := tls.LoadX509KeyPair(c.certFile, c.keyFile)
	if err != nil {
		return nil, fmt.Errorf("Space User capture identity: %w", err)
	}
	client, conn, err := usercapture.New(usercapture.TransportConfig{Address: c.address, ServerName: c.serverName, Roots: roots, Certificate: identity}, issuer)
	if err != nil {
		return nil, fmt.Errorf("Space User capture transport: %w", err)
	}
	conn.Connect()
	readyCtx, cancel := context.WithTimeout(ctx, grpcclient.DialTimeoutFromEnv())
	err = grpcclient.WaitForReady(readyCtx, conn)
	cancel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("Space User capture readiness: %w", err)
	}
	return &userCaptureRuntime{client: client, conn: conn}, nil
}

func (r *userCaptureRuntime) Close() error {
	if r == nil || r.conn == nil {
		return nil
	}
	return r.conn.Close()
}
