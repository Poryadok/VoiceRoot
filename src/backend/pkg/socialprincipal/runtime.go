package socialprincipal

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"voice/backend/pkg/principal"
)

const dependencyTimeout = 2 * time.Second

type Runtime struct {
	target      string
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
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("principal TLS: %w", err)
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	if cfg.JWKSCAFile != "" {
		pem, err := os.ReadFile(cfg.JWKSCAFile)
		if err != nil {
			return nil, err
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("principal JWKS CA has no certificates")
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	client := &http.Client{Transport: transport, Timeout: dependencyTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("principal JWKS redirects forbidden") }}
	endpoints := make(map[string]string, len(cfg.JWKSURLs))
	issuers := make(map[string]bool, len(cfg.JWKSURLs))
	for issuer, url := range cfg.JWKSURLs {
		endpoints[issuer] = url
		issuers[issuer] = true
	}
	fetch := func(ctx context.Context, issuer string) ([]byte, error) {
		endpoint, ok := endpoints[issuer]
		if !ok {
			return nil, errors.New("untrusted issuer")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, jwksTransportError(err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
				return nil, Unavailable(errors.New("principal JWKS unavailable"))
			}
			return nil, errors.New("invalid principal JWKS response")
		}
		document, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
		if err != nil {
			return nil, jwksTransportError(err)
		}
		if len(document) > 65536 {
			return nil, errors.New("oversized principal JWKS")
		}
		if err := validateJWKS(document); err != nil {
			return nil, err
		}
		return document, nil
	}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{Fetch: fetch, RefreshAfter: cfg.RefreshAfter, HardExpiry: cfg.HardExpiry, UnknownKIDCooldown: cfg.UnknownKIDCooldown})
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	replay := redis.NewClient(&redis.Options{Addr: cfg.ReplayAddr, Password: cfg.ReplayPassword, DialTimeout: dependencyTimeout, ReadTimeout: dependencyTimeout, WriteTimeout: dependencyTimeout, MaxRetries: -1, ContextTimeoutEnabled: true})
	r := &Runtime{target: cfg.Target, resolver: resolver, replay: replay, transport: transport, credentials: credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}})}
	r.verifier = &Verifier{Target: cfg.Target, Issuers: issuers, Resolve: resolver.Resolve, Replay: r.recordReplay}
	startup, cancel := context.WithTimeout(ctx, dependencyTimeout)
	defer cancel()
	if err := replay.Ping(startup).Err(); err != nil {
		_ = r.Close()
		return nil, errors.New("principal replay Redis unavailable")
	}
	refreshContext, stop := context.WithCancel(ctx)
	r.cancel = stop
	r.done = make(chan struct{})
	go r.refreshLoop(refreshContext, cfg.RefreshAfter, issuers)
	return r, nil
}

func validateJWKS(document []byte) error {
	keys, err := principal.ParseJWKS(document)
	if err != nil || len(keys) != 2 {
		return errors.New("principal JWKS requires current and next keys")
	}
	var first *rsa.PublicKey
	for kid, key := range keys {
		if !issuerPattern.MatchString(kid) || key.N.BitLen() < 2048 {
			return errors.New("invalid principal signing key")
		}
		if first != nil && first.N.Cmp(key.N) == 0 {
			return errors.New("principal rotation keys must be distinct")
		}
		first = key
	}
	return nil
}
func jwksTransportError(err error) error {
	var certificateError *tls.CertificateVerificationError
	var protocolError tls.RecordHeaderError
	if errors.As(err, &certificateError) || errors.As(err, &protocolError) {
		return err
	}
	var networkError *net.OpError
	if errors.As(err, &networkError) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return Unavailable(errors.New("principal JWKS unavailable"))
	}
	return err
}
func (r *Runtime) recordReplay(ctx context.Context, issuer, jti string, expires time.Time) error {
	ttl := time.Until(expires)
	if ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid principal replay lifetime")
	}
	ttl = ((ttl + time.Millisecond - 1) / time.Millisecond) * time.Millisecond
	digest := sha256.Sum256([]byte(issuer + "\x00" + jti))
	key := fmt.Sprintf("%s:principal:replay:%x", r.target, digest)
	result, err := r.replay.SetArgs(ctx, key, "1", redis.SetArgs{Mode: "NX", TTL: ttl}).Result()
	if errors.Is(err, redis.Nil) {
		return errors.New("principal replay detected")
	}
	if err != nil {
		return Unavailable(errors.New("principal replay Redis unavailable"))
	}
	if result != "OK" {
		return errors.New("principal replay not recorded")
	}
	return nil
}
func (r *Runtime) refreshLoop(ctx context.Context, interval time.Duration, issuers map[string]bool) {
	defer close(r.done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for issuer := range issuers {
				_ = r.resolver.Refresh(ctx, issuer)
			}
		}
	}
}

func (r *Runtime) ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.Creds(r.credentials), grpc.ChainUnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod != Method(r.target) {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on privacy listener")
		}
		return handler(ctx, req)
	}, StrictUnaryInterceptor(r.verifier))}
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
