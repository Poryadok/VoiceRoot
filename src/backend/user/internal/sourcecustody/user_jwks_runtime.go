package sourcecustody

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type UserJWKSRuntime struct {
	server  *http.Server
	address string
	done    chan struct{}
	err     error
}

type UserJWKSConfig struct {
	Address, CertFile, KeyFile string
}

func LoadUserJWKSConfig(lookup func(string) (string, bool)) (UserJWKSConfig, bool, error) {
	var empty UserJWKSConfig
	if lookup == nil {
		return empty, false, ErrUnavailable
	}
	names := []string{"USER_PRINCIPAL_JWKS_LISTEN", "USER_PRINCIPAL_JWKS_TLS_CERT_FILE", "USER_PRINCIPAL_JWKS_TLS_KEY_FILE"}
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
	for _, value := range values {
		if value == "" || strings.TrimSpace(value) != value {
			return empty, false, ErrUnavailable
		}
	}
	host, port, err := net.SplitHostPort(values[0])
	if err != nil || host == "" || strings.ContainsAny(host, "/@ \\?#") {
		return empty, false, ErrUnavailable
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 || strconv.Itoa(number) != port {
		return empty, false, ErrUnavailable
	}
	return UserJWKSConfig{Address: values[0], CertFile: values[1], KeyFile: values[2]}, true, nil
}

// StartUserJWKS serves public keys on a dedicated TLS listener. The caller
// must supervise Done and owns key rotation and deployment trust.
func StartUserJWKS(ctx context.Context, address, certFile, keyFile string, public []byte) (*UserJWKSRuntime, error) {
	return startUserJWKS(ctx, address, certFile, keyFile, public, false)
}

func startUserJWKS(ctx context.Context, address, certFile, keyFile string, public []byte, allowZero bool) (*UserJWKSRuntime, error) {
	if ctx == nil || ctx.Err() != nil || address == "" || address != strings.TrimSpace(address) || certFile == "" || certFile != strings.TrimSpace(certFile) || keyFile == "" || keyFile != strings.TrimSpace(keyFile) {
		return nil, ErrUnavailable
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" || strings.ContainsAny(host, "/@ \\?#") {
		return nil, ErrUnavailable
	}
	parsed, err := strconv.Atoi(port)
	if err != nil || parsed < 0 || parsed > 65535 || (!allowZero && parsed == 0) || strconv.Itoa(parsed) != port {
		return nil, ErrUnavailable
	}
	handler, err := NewUserJWKSHandler(public)
	if err != nil {
		return nil, ErrUnavailable
	}
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, ErrUnavailable
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, ErrUnavailable
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 3 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 8192}
	runtime := &UserJWKSRuntime{server: server, address: listener.Addr().String(), done: make(chan struct{})}
	go func() {
		err := server.Serve(tls.NewListener(listener, &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}}))
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			runtime.err = ErrUnavailable
		}
		close(runtime.done)
	}()
	go func() {
		select {
		case <-ctx.Done():
			cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if server.Shutdown(cleanup) != nil {
				_ = server.Close()
			}
		case <-runtime.done:
		}
	}()
	if ctx.Err() != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = runtime.Close(cleanup)
		return nil, ErrUnavailable
	}
	return runtime, nil
}

func (r *UserJWKSRuntime) Address() string {
	if r == nil {
		return ""
	}
	return r.address
}

func (r *UserJWKSRuntime) Done() <-chan struct{} {
	if r == nil {
		return nil
	}
	return r.done
}

func (r *UserJWKSRuntime) Close(ctx context.Context) error {
	if r == nil || r.server == nil || ctx == nil {
		return ErrUnavailable
	}
	if r.server.Shutdown(ctx) != nil {
		_ = r.server.Close()
		return ErrUnavailable
	}
	select {
	case <-r.done:
		return r.err
	case <-ctx.Done():
		_ = r.server.Close()
		return ErrUnavailable
	}
}
