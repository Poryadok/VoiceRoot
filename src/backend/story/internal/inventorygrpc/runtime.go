package inventorygrpc

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
	"google.golang.org/grpc/credentials"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/referenceinventory"
)

const dependencyTimeout = 2 * time.Second
const refreshAfter = 30 * time.Second

type Runtime struct {
	resolver    *principal.JWKSResolver
	replay      *redis.Client
	transport   *http.Transport
	credentials credentials.TransportCredentials
	cancel      context.CancelFunc
	done        chan struct{}
	once        sync.Once
	closeErr    error
}

func NewRuntime(ctx context.Context, cfg Config) (*Runtime, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, errors.New("inventory TLS identity unavailable")
	}
	clientPEM, err := os.ReadFile(cfg.ClientCAFile)
	if err != nil {
		return nil, errors.New("inventory client CA unavailable")
	}
	clientRoots := x509.NewCertPool()
	if !clientRoots.AppendCertsFromPEM(clientPEM) {
		return nil, errors.New("invalid inventory client CA")
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if cfg.JWKSCAFile != "" {
		raw, err := os.ReadFile(cfg.JWKSCAFile)
		if err != nil || !roots.AppendCertsFromPEM(raw) {
			return nil, errors.New("invalid inventory JWKS CA")
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	if cfg.JWKSClientCertFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.JWKSClientCertFile, cfg.JWKSClientKeyFile)
		if err != nil {
			return nil, errors.New("inventory JWKS client identity unavailable")
		}
		transport.TLSClientConfig.Certificates = []tls.Certificate{cert}
	}
	client := &http.Client{Transport: transport, Timeout: dependencyTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("inventory JWKS redirects forbidden") }}
	fetch := func(ctx context.Context, issuer string) ([]byte, error) {
		if issuer != "file" {
			return nil, errors.New("untrusted inventory issuer")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.FileJWKSURL, nil)
		if err != nil {
			return nil, err
		}
		response, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%w: File JWKS transport", referenceinventory.ErrUnavailable)
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%w: File JWKS response", referenceinventory.ErrUnavailable)
		}
		raw, err := io.ReadAll(io.LimitReader(response.Body, 65537))
		if err != nil {
			return nil, fmt.Errorf("%w: File JWKS read", referenceinventory.ErrUnavailable)
		}
		if len(raw) > 65536 {
			return nil, errors.New("oversized inventory JWKS")
		}
		keys, err := principal.ParseJWKS(raw)
		if err != nil || len(keys) != 2 {
			return nil, errors.New("inventory File JWKS requires current and next keys")
		}
		var first *rsa.PublicKey
		for _, key := range keys {
			if key.N.BitLen() < 2048 || first != nil && first.N.Cmp(key.N) == 0 {
				return nil, errors.New("invalid inventory rotation keys")
			}
			first = key
		}
		return raw, nil
	}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{Fetch: fetch, RefreshAfter: refreshAfter, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second})
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	replay := redis.NewClient(&redis.Options{Addr: cfg.ReplayAddr, Password: cfg.ReplayPassword, DialTimeout: dependencyTimeout, ReadTimeout: dependencyTimeout, WriteTimeout: dependencyTimeout, MaxRetries: -1, ContextTimeoutEnabled: true})
	startup, cancel := context.WithTimeout(ctx, dependencyTimeout)
	defer cancel()
	if err := replay.Ping(startup).Err(); err != nil {
		transport.CloseIdleConnections()
		_ = replay.Close()
		return nil, errors.New("inventory replay Redis unavailable")
	}
	if err := resolver.Refresh(startup, "file"); err != nil {
		transport.CloseIdleConnections()
		_ = replay.Close()
		return nil, errors.New("inventory File JWKS unavailable")
	}
	refreshCtx, stop := context.WithCancel(ctx)
	r := &Runtime{resolver: resolver, replay: replay, transport: transport, credentials: credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientRoots}), cancel: stop, done: make(chan struct{})}
	go func() {
		defer close(r.done)
		ticker := time.NewTicker(refreshAfter)
		defer ticker.Stop()
		for {
			select {
			case <-refreshCtx.Done():
				return
			case <-ticker.C:
				_ = resolver.Refresh(refreshCtx, "file")
			}
		}
	}()
	return r, nil
}

func (r *Runtime) Verify(ctx context.Context, token, method, requestID, hash string) (principal.Principal, error) {
	if r == nil || r.resolver == nil || r.replay == nil {
		return principal.Principal{}, referenceinventory.ErrUnavailable
	}
	if !allowedMethod(method) || len(token) > 16384 {
		return principal.Principal{}, errors.New("invalid inventory credential")
	}
	return principal.VerifyService(ctx, token, principal.VerifyConfig{ExpectedIssuer: "file", ExpectedAudience: "story", ExpectedRPC: method, ExpectedRequestID: requestID, ExpectedRequestHash: hash, KeyResolver: r.resolver.Resolve, ReplayGuard: r.recordReplay})
}

func (r *Runtime) recordReplay(ctx context.Context, issuer, jti string, expiry time.Time) error {
	ttl := time.Until(expiry)
	if issuer != "file" || ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid inventory replay lifetime")
	}
	ttl = ((ttl + time.Millisecond - 1) / time.Millisecond) * time.Millisecond
	digest := sha256.Sum256([]byte(issuer + "\x00" + jti))
	result, err := r.replay.SetArgs(ctx, fmt.Sprintf("story:reference-inventory:principal:replay:%x", digest), "1", redis.SetArgs{Mode: "NX", TTL: ttl}).Result()
	if errors.Is(err, redis.Nil) {
		return errors.New("inventory credential replay")
	}
	if err != nil {
		return referenceinventory.ErrUnavailable
	}
	if result != "OK" {
		return errors.New("inventory replay not recorded")
	}
	return nil
}

func (r *Runtime) ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.Creds(r.credentials), grpc.ChainUnaryInterceptor(StrictUnaryInterceptor(r))}
}
func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	r.once.Do(func() {
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
