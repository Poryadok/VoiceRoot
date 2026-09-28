package gameprincipal

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"voice/backend/pkg/principal"
)

const dependencyTimeout = 2 * time.Second

type Runtime struct {
	verifier    *Verifier
	resolver    *principal.JWKSResolver
	replay      *redis.Client
	transport   *http.Transport
	credentials credentials.TransportCredentials
	cancel      context.CancelFunc
	done        chan struct{}
	closeOnce   sync.Once
	closeErr    error
}

func New(ctx context.Context, cfg Config) (*Runtime, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	serverTLS, err := loadServerTLS(cfg)
	if err != nil {
		return nil, err
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		return nil, errors.New("system CA roots unavailable for GIS JWKS")
	}
	if cfg.JWKSCAFile != "" {
		pem, readErr := os.ReadFile(cfg.JWKSCAFile)
		if readErr != nil {
			return nil, fmt.Errorf("GIS JWKS CA: %w", readErr)
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("GIS JWKS CA contains no certificates")
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	client := &http.Client{Transport: transport, Timeout: dependencyTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("GIS JWKS redirects are forbidden") }}
	fetch := func(ctx context.Context, issuer string) ([]byte, error) {
		if issuer != trustedIssuer {
			return nil, errors.New("untrusted GIS principal issuer")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.JWKSURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, errors.New("GIS JWKS unavailable")
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, errors.New("invalid GIS JWKS response")
		}
		document, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
		if err != nil || len(document) > 65536 {
			return nil, errors.New("invalid GIS JWKS response")
		}
		if err := validateJWKS(document); err != nil {
			return nil, err
		}
		return document, nil
	}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{
		Fetch: fetch, RefreshAfter: cfg.RefreshAfter, HardExpiry: cfg.HardExpiry, UnknownKIDCooldown: cfg.UnknownKIDCooldown,
	})
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	replay := redis.NewClient(&redis.Options{Addr: cfg.ReplayRedisAddr, Password: cfg.ReplayRedisPassword,
		DialTimeout: dependencyTimeout, ReadTimeout: dependencyTimeout, WriteTimeout: dependencyTimeout,
		MaxRetries: -1, ContextTimeoutEnabled: true})
	runtime := &Runtime{resolver: resolver, replay: replay, transport: transport, credentials: credentials.NewTLS(serverTLS)}
	runtime.verifier = &Verifier{Resolve: resolver.Resolve, Replay: runtime.recordReplay}
	startup, cancel := context.WithTimeout(ctx, dependencyTimeout)
	defer cancel()
	if err := replay.Ping(startup).Err(); err != nil {
		_ = runtime.Close()
		return nil, errors.New("GIS principal replay Redis unavailable")
	}
	if err := resolver.Refresh(startup, trustedIssuer); err != nil {
		_ = runtime.Close()
		return nil, errors.New("GIS principal JWKS unavailable")
	}
	refreshCtx, stop := context.WithCancel(ctx)
	runtime.cancel, runtime.done = stop, make(chan struct{})
	go runtime.refreshLoop(refreshCtx, cfg.RefreshAfter)
	return runtime, nil
}

func loadServerTLS(cfg Config) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, errors.New("GIS principal TLS certificate/key unavailable")
	}
	pem, err := os.ReadFile(cfg.ClientCAFile)
	if err != nil {
		return nil, errors.New("GIS principal client CA unavailable")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return nil, errors.New("GIS principal client CA is invalid")
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}, nil
}

func validateJWKS(document []byte) error {
	keys, err := principal.ParseJWKS(document)
	if err != nil || len(keys) < 1 {
		return errors.New("GIS JWKS has no usable current key")
	}
	for _, key := range keys {
		if key.N.BitLen() < 2048 {
			return errors.New("GIS JWKS signing key is too small")
		}
	}
	return nil
}

func (r *Runtime) recordReplay(ctx context.Context, issuer, jti string, expires time.Time) error {
	if r == nil || r.replay == nil || issuer != trustedIssuer || strings.TrimSpace(jti) == "" {
		return errors.New("GIS replay store unavailable")
	}
	ttl := time.Until(expires)
	if ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid GIS principal replay lifetime")
	}
	ttl = ((ttl + time.Millisecond - 1) / time.Millisecond) * time.Millisecond
	digest := sha256.Sum256([]byte(issuer + "\x00" + jti))
	key := "voice:principal:replay:" + hex.EncodeToString(digest[:])
	result, err := r.replay.SetArgs(ctx, key, "1", redis.SetArgs{Mode: "NX", TTL: ttl}).Result()
	if errors.Is(err, redis.Nil) {
		return errors.New("GIS principal replay detected")
	}
	if err != nil {
		return errors.New("GIS principal replay store unavailable")
	}
	if result != "OK" {
		return errors.New("GIS principal replay not recorded")
	}
	return nil
}

func (r *Runtime) refreshLoop(ctx context.Context, interval time.Duration) {
	defer close(r.done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = r.resolver.Refresh(ctx, trustedIssuer)
		}
	}
}

func (r *Runtime) ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.Creds(r.credentials), grpc.UnaryInterceptor(UnaryServerInterceptor(r.verifier))}
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
