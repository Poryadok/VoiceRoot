package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/evidence"
	"voice/backend/pkg/automodclient"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
)

type autoModAuthorityConfig struct {
	Principal                                                       socialprincipal.Config
	AuthAddr, AuthServerName, AuthCAFile, AuthCertFile, AuthKeyFile string
}

func loadAutoModAuthorityConfig() (*autoModAuthorityConfig, error) {
	principalConfig, enabled, err := socialprincipal.LoadFromEnvWithAudience("moderation", "automod-evidence", "MODERATION_AUTOMOD_PRINCIPAL_", ":9467")
	if err != nil {
		return nil, err
	}
	fields := []string{"GRPC_ADDR", "TLS_SERVER_NAME", "CA_FILE", "TLS_CERT_FILE", "TLS_KEY_FILE"}
	values := make([]string, len(fields))
	anyAuth := false
	for index, field := range fields {
		raw, set := os.LookupEnv("MODERATION_AUTOMOD_AUTH_" + field)
		anyAuth = anyAuth || set
		values[index] = strings.TrimSpace(raw)
		if set && values[index] != raw {
			return nil, errors.New("invalid Moderation Auth authority whitespace")
		}
	}
	if !enabled && !anyAuth {
		return nil, nil
	}
	if !enabled {
		return nil, errors.New("Moderation private evidence listener configuration required")
	}
	for _, value := range values {
		if value == "" {
			return nil, errors.New("complete Moderation Auth authority mTLS configuration required")
		}
	}
	return &autoModAuthorityConfig{Principal: principalConfig, AuthAddr: values[0], AuthServerName: values[1], AuthCAFile: values[2], AuthCertFile: values[3], AuthKeyFile: values[4]}, nil
}

type autoModAuthorityRuntime struct {
	Store          *automod.Store
	Auth           *automodclient.AuthClient
	Gate           *automod.BootGate
	Issuer         *principal.Issuer
	server         *grpc.Server
	listener       net.Listener
	verifier       *socialprincipal.Runtime
	authConnection *grpc.ClientConn
	done           chan struct{}
	errors         chan error
	once           sync.Once
}

// Local TLS, replay storage and key configuration are checked before binding.
// Auth dialing and JWKS resolution are lazy; no public readiness or reciprocal
// peer RPC is a prerequisite for discovering this private P0 listener.
func newAutoModAuthorityRuntime(ctx context.Context, cfg autoModAuthorityConfig, subjects *automod.Store, issuer *principal.Issuer, gate *automod.BootGate) (*autoModAuthorityRuntime, error) {
	if ctx == nil || ctx.Err() != nil || subjects == nil || issuer == nil || gate == nil || cfg.Principal.Target != "moderation" || cfg.Principal.Capability != "automod-evidence" {
		return nil, errors.New("shared Moderation authority dependencies required")
	}
	connection, err := dialAutoModAuth(cfg)
	if err != nil {
		return nil, err
	}
	verifier, err := socialprincipal.New(ctx, cfg.Principal)
	if err != nil {
		_ = connection.Close()
		return nil, err
	}
	listener, err := net.Listen("tcp", cfg.Principal.ListenAddr)
	if err != nil {
		_ = verifier.Close()
		_ = connection.Close()
		return nil, err
	}
	runtime := &autoModAuthorityRuntime{Store: subjects, Auth: automodclient.NewAuthClient(issuer, connection), Gate: gate, Issuer: issuer, verifier: verifier, authConnection: connection, listener: listener, done: make(chan struct{}), errors: make(chan error, 1)}
	runtime.server = grpc.NewServer(verifier.ServerOptions()...)
	automodv1.RegisterModerationEvidenceServiceServer(runtime.server, &evidence.Service{Store: subjects, Auth: runtime.Auth, Gate: gate})
	go func() {
		defer close(runtime.done)
		defer runtime.Gate.Close()
		if err := runtime.server.Serve(listener); err != nil {
			runtime.errors <- err
		}
	}()
	go func() {
		select {
		case <-ctx.Done():
			runtime.Close()
		case <-runtime.done:
		}
	}()
	return runtime, nil
}

func dialAutoModAuth(cfg autoModAuthorityConfig) (*grpc.ClientConn, error) {
	host, port, err := net.SplitHostPort(cfg.AuthAddr)
	number, parseErr := strconv.Atoi(port)
	if err != nil || parseErr != nil || host == "" || strings.ContainsAny(host, "/@ ") || number <= 0 || number > 65535 || cfg.AuthServerName == "" || strings.TrimSpace(cfg.AuthServerName) != cfg.AuthServerName {
		return nil, errors.New("exact private Auth authority address and TLS server name required")
	}
	if cfg.AuthCertFile == "" || cfg.AuthKeyFile == "" || cfg.AuthCAFile == "" {
		return nil, errors.New("private Auth workload TLS files required")
	}
	pair, err := tls.LoadX509KeyPair(cfg.AuthCertFile, cfg.AuthKeyFile)
	if err != nil {
		return nil, err
	}
	pem, err := os.ReadFile(cfg.AuthCAFile)
	if err != nil || len(pem) > 1<<20 {
		return nil, errors.New("private Auth CA unavailable")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return nil, errors.New("private Auth CA contains no certificates")
	}
	return grpc.NewClient(cfg.AuthAddr, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{pair}, ServerName: cfg.AuthServerName})))
}

func (r *autoModAuthorityRuntime) Close() {
	if r == nil {
		return
	}
	r.once.Do(func() {
		r.Gate.Close()
		r.server.Stop()
		_ = r.listener.Close()
		_ = r.verifier.Close()
		_ = r.authConnection.Close()
	})
}

func autoModPublicGate(gate *automod.BootGate) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, _ *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		check, err := gate.AcquireEffects(ctx)
		if err != nil {
			return nil, status.Error(codes.Unavailable, "current-boot Moderation reconciliation incomplete")
		}
		response, err := next(ctx, request)
		if err != nil {
			return nil, err
		}
		if err = check(ctx); err != nil {
			return nil, status.Error(codes.Unavailable, "Moderation generation changed")
		}
		return response, nil
	}
}
