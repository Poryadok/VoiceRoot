// Package gisprincipal provides the narrowly scoped GIS-to-Chat transport.
package gisprincipal

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
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"voice/backend/pkg/principal"
)

const (
	Issuer            = "gameintegration"
	Audience          = "chat"
	ProvisionMethod   = "/voice.chat.v1.GameIntegrationChatService/ProvisionManagedChat"
	SyncMembersMethod = "/voice.chat.v1.GameIntegrationChatService/SyncManagedChatMembers"
)

type Config struct {
	TLSCertFile    string
	TLSKeyFile     string
	ClientCAFile   string
	JWKSURL        string
	JWKSCAFile     string
	ReplayAddr     string
	ReplayPassword string
	RefreshAfter   time.Duration
	HardExpiry     time.Duration
}

func ConfigFromEnv() (Config, bool, error) {
	get := func(key string) string { return strings.TrimSpace(os.Getenv(key)) }
	cfg := Config{
		TLSCertFile: get("CHAT_GIS_TLS_CERT_FILE"), TLSKeyFile: get("CHAT_GIS_TLS_KEY_FILE"),
		ClientCAFile: get("CHAT_GIS_CLIENT_CA_FILE"), JWKSURL: get("GAME_INTEGRATION_PRINCIPAL_JWKS_URL"),
		JWKSCAFile: get("GAME_INTEGRATION_PRINCIPAL_JWKS_CA_FILE"), ReplayAddr: get("GAME_INTEGRATION_PRINCIPAL_REPLAY_REDIS_ADDR"),
		ReplayPassword: os.Getenv("GAME_INTEGRATION_PRINCIPAL_REPLAY_REDIS_PASSWORD"),
	}
	configured := cfg.TLSCertFile != "" || cfg.TLSKeyFile != "" || cfg.ClientCAFile != "" || cfg.JWKSURL != "" || cfg.JWKSCAFile != "" || cfg.ReplayAddr != "" || cfg.ReplayPassword != ""
	if !configured {
		return Config{}, false, nil
	}
	if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" || cfg.ClientCAFile == "" || cfg.JWKSURL == "" || cfg.ReplayAddr == "" {
		return Config{}, true, errors.New("Chat GIS principal configuration is incomplete")
	}
	return cfg, true, nil
}

type Runtime struct {
	resolver    *principal.JWKSResolver
	keyResolver principal.KeyResolver
	replayGuard principal.ReplayGuard
	replay      *redis.Client
	transport   *http.Transport
	credentials credentials.TransportCredentials
	stop        context.CancelFunc
	done        chan struct{}
	closeOnce   sync.Once
	closeErr    error
}

type unavailableError struct{ err error }

func (e *unavailableError) Error() string { return e.err.Error() }
func (e *unavailableError) Unwrap() error { return e.err }
func unavailable(err error) error         { return &unavailableError{err: err} }

func New(ctx context.Context, cfg Config) (*Runtime, error) {
	if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" || cfg.ClientCAFile == "" || cfg.JWKSURL == "" || cfg.ReplayAddr == "" {
		return nil, errors.New("Chat GIS principal configuration is incomplete")
	}
	jwksURL, err := url.Parse(cfg.JWKSURL)
	if err != nil || jwksURL.Scheme != "https" || jwksURL.Host == "" || jwksURL.User != nil || jwksURL.Fragment != "" {
		return nil, errors.New("Chat GIS principal JWKS URL must be an HTTPS endpoint")
	}
	serverTLS, err := loadServerTLS(cfg.TLSCertFile, cfg.TLSKeyFile, cfg.ClientCAFile)
	if err != nil {
		return nil, err
	}

	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if cfg.JWKSCAFile != "" {
		pem, err := os.ReadFile(cfg.JWKSCAFile)
		if err != nil {
			return nil, fmt.Errorf("Chat GIS JWKS CA: %w", err)
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("Chat GIS JWKS CA contains no certificates")
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("JWKS redirects forbidden") }}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{
		Fetch: func(ctx context.Context, issuer string) ([]byte, error) {
			if issuer != Issuer {
				return nil, errors.New("untrusted principal issuer")
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.JWKSURL, nil)
			if err != nil {
				return nil, err
			}
			resp, err := client.Do(req)
			if err != nil {
				return nil, unavailable(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
					return nil, unavailable(fmt.Errorf("principal JWKS returned HTTP %d", resp.StatusCode))
				}
				return nil, fmt.Errorf("principal JWKS returned HTTP %d", resp.StatusCode)
			}
			body, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
			if err != nil || len(body) > 65536 {
				return nil, errors.New("invalid principal JWKS document")
			}
			keys, err := principal.ParseJWKS(body)
			if err != nil || len(keys) != 2 {
				return nil, errors.New("principal JWKS requires current and next keys")
			}
			var first *rsa.PublicKey
			for _, key := range keys {
				if key.N.BitLen() < 2048 || first != nil && first.N.Cmp(key.N) == 0 {
					return nil, errors.New("principal JWKS rotation keys are invalid")
				}
				first = key
			}
			return body, nil
		}, RefreshAfter: cfg.RefreshAfter, HardExpiry: cfg.HardExpiry,
	})
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	replay := redis.NewClient(&redis.Options{Addr: cfg.ReplayAddr, Password: cfg.ReplayPassword, DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second, MaxRetries: -1, ContextTimeoutEnabled: true})
	runtime := &Runtime{resolver: resolver, replay: replay, transport: transport, credentials: credentials.NewTLS(serverTLS), done: make(chan struct{})}
	runtime.keyResolver = func(ctx context.Context, issuer, keyID string) (*rsa.PublicKey, error) {
		key, resolveErr := resolver.Resolve(ctx, issuer, keyID)
		if resolveErr != nil && !strings.Contains(resolveErr.Error(), "kid is unknown") && !strings.Contains(resolveErr.Error(), "cooling down") {
			return nil, unavailable(resolveErr)
		}
		return key, resolveErr
	}
	runtime.replayGuard = runtime.recordReplay
	startup, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := replay.Ping(startup).Err(); err != nil {
		_ = replay.Close()
		transport.CloseIdleConnections()
		return nil, errors.New("principal replay Redis unavailable")
	}
	if err := resolver.Refresh(startup, Issuer); err != nil {
		_ = replay.Close()
		transport.CloseIdleConnections()
		return nil, fmt.Errorf("principal JWKS unavailable: %w", err)
	}
	refreshCtx, stop := context.WithCancel(ctx)
	runtime.stop = stop
	go runtime.refresh(refreshCtx, cfg.RefreshAfter)
	return runtime, nil
}

func loadServerTLS(certFile, keyFile, clientCAFile string) (*tls.Config, error) {
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("Chat GIS TLS certificate: %w", err)
	}
	clientCAPEM, err := os.ReadFile(clientCAFile)
	if err != nil {
		return nil, fmt.Errorf("Chat GIS client CA: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(clientCAPEM) {
		return nil, errors.New("Chat GIS client CA contains no certificates")
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs}, nil
}

func (r *Runtime) refresh(ctx context.Context, interval time.Duration) {
	defer close(r.done)
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = r.resolver.Refresh(ctx, Issuer)
		}
	}
}

func (r *Runtime) ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.Creds(r.credentials), grpc.ChainUnaryInterceptor(allowlistInterceptor, strictInterceptor{runtime: r}.intercept)}
}

func allowlistInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if !AllowsMethod(info.FullMethod) {
		return nil, status.Error(codes.PermissionDenied, "method unavailable on GIS listener")
	}
	return handler(ctx, req)
}
func AllowsMethod(method string) bool {
	return method == ProvisionMethod || method == SyncMembersMethod
}

type strictInterceptor struct{ runtime *Runtime }

func (i strictInterceptor) intercept(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid principal")
	}
	message, ok := req.(proto.Message)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	requestHash, err := principal.RequestHash(message)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	requestID, ok := requestOperationID(message)
	if !ok || requestID != transport.RequestID {
		return nil, status.Error(codes.Unauthenticated, "invalid principal binding")
	}
	verified, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{
		ExpectedIssuer: Issuer, ExpectedAudience: Audience, ExpectedRPC: info.FullMethod,
		ExpectedRequestID: transport.RequestID, ExpectedRequestHash: requestHash, KeyResolver: i.runtime.keyResolver,
		ReplayGuard: i.runtime.replayGuard,
	})
	if err != nil {
		var unavailableCause *unavailableError
		if errors.As(err, &unavailableCause) {
			return nil, status.Error(codes.Unavailable, "principal verifier unavailable")
		}
		return nil, status.Error(codes.Unauthenticated, "invalid principal")
	}
	if verified.Subject != "service:"+Issuer || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return nil, status.Error(codes.PermissionDenied, "GIS service principal required")
	}
	return handler(principal.WithVerified(ctx, verified), req)
}

func requestOperationID(message proto.Message) (string, bool) {
	switch request := message.(type) {
	case interface{ GetOperationId() string }:
		return request.GetOperationId(), true
	default:
		return "", false
	}
}

func (r *Runtime) recordReplay(ctx context.Context, issuer, jti string, expires time.Time) error {
	ttl := time.Until(expires)
	if ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid replay lifetime")
	}
	ttl = ((ttl + time.Millisecond - 1) / time.Millisecond) * time.Millisecond
	digest := sha256.Sum256([]byte(issuer + "\x00" + jti))
	result, err := r.replay.SetArgs(ctx, "chat:principal:replay:"+fmt.Sprintf("%x", digest), "1", redis.SetArgs{Mode: "NX", TTL: ttl}).Result()
	if errors.Is(err, redis.Nil) {
		return errors.New("principal replay detected")
	}
	if err != nil {
		return unavailable(fmt.Errorf("principal replay unavailable: %w", err))
	}
	if result != "OK" {
		return errors.New("principal replay not recorded")
	}
	return nil
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		if r.stop != nil {
			r.stop()
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
