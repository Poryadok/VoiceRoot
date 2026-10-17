package matchsquadprincipal

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/principal"
)

const (
	dependencyTimeout  = 2 * time.Second
	refreshAfter       = 30 * time.Second
	hardExpiry         = 2 * time.Minute
	unknownKIDCooldown = 5 * time.Second
)

var (
	errVerifierUnavailable = errors.New("voice MatchSquad principal verifier unavailable")
	errReplayDetected      = errors.New("voice MatchSquad principal replay detected")
)

type Runtime struct {
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
		return nil, errors.New("system CA roots unavailable for Voice MatchSquad JWKS")
	}
	if cfg.JWKSCAFile != "" {
		pem, readErr := os.ReadFile(cfg.JWKSCAFile)
		if readErr != nil {
			return nil, errors.New("voice MatchSquad JWKS CA unavailable")
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("voice MatchSquad JWKS CA contains no certificates")
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	client := &http.Client{
		Transport: transport, Timeout: dependencyTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("voice MatchSquad JWKS redirects are forbidden")
		},
	}
	fetch := func(ctx context.Context, issuer string) ([]byte, error) {
		if issuer != trustedIssuer {
			return nil, errors.New("untrusted Voice MatchSquad principal issuer")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.JWKSURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmtVerifierUnavailable()
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, fmtVerifierUnavailable()
		}
		document, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
		if err != nil || len(document) > 65536 {
			return nil, fmtVerifierUnavailable()
		}
		keys, err := principal.ParseJWKS(document)
		if err != nil || len(keys) == 0 {
			return nil, errors.New("voice MatchSquad JWKS has no usable key")
		}
		for _, key := range keys {
			if key.N.BitLen() < 2048 {
				return nil, errors.New("voice MatchSquad JWKS key is too small")
			}
		}
		return document, nil
	}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{
		Fetch: fetch, RefreshAfter: refreshAfter, HardExpiry: hardExpiry, UnknownKIDCooldown: unknownKIDCooldown,
	})
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	replay := redis.NewClient(&redis.Options{
		Addr: cfg.ReplayRedisAddr, Password: cfg.ReplayRedisPassword,
		DialTimeout: dependencyTimeout, ReadTimeout: dependencyTimeout, WriteTimeout: dependencyTimeout,
		MaxRetries: -1, ContextTimeoutEnabled: true,
	})
	runtime := &Runtime{resolver: resolver, replay: replay, transport: transport, credentials: credentials.NewTLS(serverTLS), done: make(chan struct{})}
	startup, cancel := context.WithTimeout(ctx, dependencyTimeout)
	defer cancel()
	if err := replay.Ping(startup).Err(); err != nil {
		_ = runtime.Close()
		return nil, errors.New("voice MatchSquad replay Redis unavailable")
	}
	if err := resolver.Refresh(startup, trustedIssuer); err != nil {
		_ = runtime.Close()
		return nil, errors.New("voice MatchSquad JWKS unavailable")
	}
	refreshCtx, stop := context.WithCancel(ctx)
	runtime.cancel = stop
	go runtime.refreshLoop(refreshCtx)
	return runtime, nil
}

func loadServerTLS(cfg Config) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, errors.New("voice MatchSquad TLS certificate/key unavailable")
	}
	pem, err := os.ReadFile(cfg.ClientCAFile)
	if err != nil {
		return nil, errors.New("voice MatchSquad client CA unavailable")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return nil, errors.New("voice MatchSquad client CA is invalid")
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}, nil
}

func (r *Runtime) Verify(ctx context.Context, token, method, requestID, requestHash string) (principal.Principal, error) {
	if r == nil || r.resolver == nil || r.replay == nil {
		return principal.Principal{}, status.Error(codes.Unavailable, "Voice MatchSquad principal verifier unavailable")
	}
	verified, err := principal.VerifyService(ctx, token, principal.VerifyConfig{
		ExpectedIssuer: trustedIssuer, ExpectedAudience: trustedAudience, ExpectedRPC: method,
		ExpectedRequestID: requestID, ExpectedRequestHash: requestHash,
		KeyResolver: r.resolveKey, ReplayGuard: r.recordReplay,
	})
	if err != nil {
		var unavailable verifierUnavailable
		if errors.As(err, &unavailable) {
			return principal.Principal{}, status.Error(codes.Unavailable, "Voice MatchSquad principal verifier unavailable")
		}
		return principal.Principal{}, status.Error(codes.Unauthenticated, "invalid Voice MatchSquad principal")
	}
	if verified.Issuer != trustedIssuer || verified.Subject != trustedSubject || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return principal.Principal{}, status.Error(codes.Unauthenticated, "invalid Voice MatchSquad principal")
	}
	return verified, nil
}

func (r *Runtime) resolveKey(ctx context.Context, issuer, kid string) (*rsa.PublicKey, error) {
	key, err := r.resolver.Resolve(ctx, issuer, kid)
	if err != nil {
		return nil, verifierUnavailable{err: err}
	}
	return key, nil
}

func (r *Runtime) recordReplay(ctx context.Context, issuer, jti string, expires time.Time) error {
	if issuer != trustedIssuer || strings.TrimSpace(jti) == "" {
		return errors.New("invalid Voice MatchSquad replay key")
	}
	ttl := time.Until(expires)
	if ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid Voice MatchSquad replay lifetime")
	}
	ttl = ((ttl + time.Millisecond - 1) / time.Millisecond) * time.Millisecond
	digest := sha256.Sum256([]byte(issuer + "\x00" + jti))
	key := "voice:match_squad:principal:replay:" + hex.EncodeToString(digest[:])
	result, err := r.replay.SetArgs(ctx, key, "1", redis.SetArgs{Mode: "NX", TTL: ttl}).Result()
	if errors.Is(err, redis.Nil) {
		return errReplayDetected
	}
	if err != nil || result != "OK" {
		return verifierUnavailable{err: err}
	}
	return nil
}

func (r *Runtime) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(refreshAfter)
	defer ticker.Stop()
	defer close(r.done)
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
	return []grpc.ServerOption{grpc.Creds(r.credentials), grpc.ChainUnaryInterceptor(StrictUnaryInterceptor(r))}
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

type verifierUnavailable struct{ err error }

func (e verifierUnavailable) Error() string {
	if e.err == nil {
		return errVerifierUnavailable.Error()
	}
	return errVerifierUnavailable.Error()
}
func (e verifierUnavailable) Unwrap() error { return e.err }

func fmtVerifierUnavailable() error { return verifierUnavailable{err: errVerifierUnavailable} }
