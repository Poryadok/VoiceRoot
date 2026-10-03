package spaceprincipalruntime

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	notificationv1 "voice.app/voice/notification/v1"
	"voice/backend/notification/internal/principalgrpc"
	"voice/backend/pkg/principal"
)

const dependencyTimeout = 2 * time.Second

type Runtime struct {
	resolver     *principal.JWKSResolver
	replay       *redis.Client
	transport    *http.Transport
	credentials  credentials.TransportCredentials
	refreshAfter time.Duration
	cancel       context.CancelFunc
	done         chan struct{}
	closeOnce    sync.Once
	closeErr     error
}

func New(ctx context.Context, cfg Config) (*Runtime, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	serverCert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("notification lifecycle server TLS: %w", err)
	}
	clientCAPEM, err := os.ReadFile(cfg.ClientCAFile)
	if err != nil {
		return nil, fmt.Errorf("notification lifecycle client CA: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(clientCAPEM) {
		return nil, errors.New("notification lifecycle client CA has no certificates")
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if cfg.JWKSCAFile != "" {
		ca, err := os.ReadFile(cfg.JWKSCAFile)
		if err != nil {
			return nil, fmt.Errorf("space principal JWKS CA: %w", err)
		}
		if !roots.AppendCertsFromPEM(ca) {
			return nil, errors.New("space principal JWKS CA has no certificates")
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	client := &http.Client{Transport: transport, Timeout: dependencyTimeout, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("space principal JWKS redirects are forbidden")
	}}
	fetch := func(ctx context.Context, issuer string) ([]byte, error) {
		if issuer != "space" {
			return nil, errors.New("untrusted principal issuer")
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.JWKSURL, nil)
		if err != nil {
			return nil, err
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, unavailableJWKS(err)
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK {
			return nil, status.Error(codes.Unavailable, "space principal JWKS unavailable")
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, 65537))
		if err != nil {
			return nil, unavailableJWKS(err)
		}
		if len(body) > 65536 {
			return nil, errors.New("invalid Space principal JWKS response")
		}
		keys, err := principal.ParseJWKS(body)
		if err != nil || len(keys) != 2 {
			return nil, errors.New("space principal JWKS requires current and next keys")
		}
		var first *rsa.PublicKey
		for _, key := range keys {
			if key.N.BitLen() < 2048 || (first != nil && first.N.Cmp(key.N) == 0) {
				return nil, errors.New("invalid Space principal signing keys")
			}
			first = key
		}
		return body, nil
	}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{Fetch: fetch, RefreshAfter: cfg.RefreshAfter, HardExpiry: cfg.HardExpiry, UnknownKIDCooldown: cfg.UnknownKIDCooldown})
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	replay := redis.NewClient(&redis.Options{Addr: cfg.ReplayAddr, Password: cfg.ReplayPassword, DialTimeout: dependencyTimeout, ReadTimeout: dependencyTimeout, WriteTimeout: dependencyTimeout, MaxRetries: -1, ContextTimeoutEnabled: true})
	r := &Runtime{resolver: resolver, replay: replay, transport: transport, refreshAfter: cfg.RefreshAfter, credentials: credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs})}
	startup, cancel := context.WithTimeout(ctx, dependencyTimeout)
	defer cancel()
	if err := replay.Ping(startup).Err(); err != nil {
		_ = r.Close()
		return nil, status.Error(codes.Unavailable, "space principal replay Redis unavailable")
	}
	if err := resolver.Refresh(startup, "space"); err != nil {
		_ = r.Close()
		return nil, fmt.Errorf("load initial Space principal JWKS: %w", err)
	}
	refreshCtx, stop := context.WithCancel(ctx)
	r.cancel = stop
	r.done = make(chan struct{})
	go r.refreshLoop(refreshCtx)
	return r, nil
}

func unavailableJWKS(err error) error {
	_ = err
	return status.Error(codes.Unavailable, "space principal JWKS unavailable")
}

func (r *Runtime) Verify(ctx context.Context, token, method, requestID, hash string) (principal.Principal, error) {
	if r == nil || r.resolver == nil || r.replay == nil {
		return principal.Principal{}, status.Error(codes.Unavailable, "space principal verifier unavailable")
	}
	if !principalgrpc.IsLifecycleMethod(method) {
		return principal.Principal{}, status.Error(codes.PermissionDenied, "method is not allowed")
	}
	verified, err := principal.VerifyService(ctx, token, principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "notification", ExpectedRPC: method, ExpectedRequestID: requestID, ExpectedRequestHash: hash, KeyResolver: r.resolver.Resolve, ReplayGuard: r.recordReplay})
	if err != nil {
		return principal.Principal{}, err
	}
	if verified.Subject != "service:space" || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return principal.Principal{}, errors.New("space principal contains user authority")
	}
	return verified, nil
}

func (r *Runtime) recordReplay(ctx context.Context, issuer, jwtID string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if issuer != "space" || jwtID == "" || ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid principal replay lifetime")
	}
	ttl = ((ttl + time.Millisecond - 1) / time.Millisecond) * time.Millisecond
	digest := sha256.Sum256([]byte(issuer + "\x00" + jwtID))
	result, err := r.replay.SetArgs(ctx, fmt.Sprintf("notification:space-principal:replay:%x", digest), "1", redis.SetArgs{Mode: "NX", TTL: ttl}).Result()
	if errors.Is(err, redis.Nil) {
		return errors.New("principal replay detected")
	}
	if err != nil {
		return status.Error(codes.Unavailable, "space principal replay Redis unavailable")
	}
	if result != "OK" {
		return errors.New("principal replay was not recorded")
	}
	return nil
}

func (r *Runtime) ServerOptions() []grpc.ServerOption {
	if r == nil || r.credentials == nil {
		return []grpc.ServerOption{grpc.ChainUnaryInterceptor(principalgrpc.StrictUnaryInterceptor(nil))}
	}
	return []grpc.ServerOption{grpc.Creds(r.credentials), grpc.ChainUnaryInterceptor(principalgrpc.StrictUnaryInterceptor(r))}
}

func (r *Runtime) refreshLoop(ctx context.Context) {
	defer close(r.done)
	ticker := time.NewTicker(r.refreshAfter)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = r.resolver.Refresh(ctx, "space")
		}
	}
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		if r.cancel != nil {
			r.cancel()
			<-r.done
		}
		if r.transport != nil {
			r.transport.CloseIdleConnections()
		}
		if r.replay != nil {
			r.closeErr = r.replay.Close()
		}
	})
	return r.closeErr
}

func LifecycleMethodAllowlist() []string {
	return []string{notificationv1.NotificationService_ApplySpaceLifecycleFence_FullMethodName, notificationv1.NotificationService_PurgeSpace_FullMethodName, notificationv1.NotificationService_ImportSpacePurgeManifestPage_FullMethodName}
}
