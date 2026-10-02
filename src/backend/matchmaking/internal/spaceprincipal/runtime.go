// Package spaceprincipal serves Matchmaking's Space lifecycle RPCs over a
// dedicated mTLS listener with request-bound service:space principals.
package spaceprincipal

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/pkg/principal"
)

const dependencyTimeout = 2 * time.Second

var kidPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Config struct {
	ListenAddr, TLSCertFile, TLSKeyFile, ClientCAFile string
	JWKSCAFile, JWKSURL, ReplayAddr, ReplayPassword   string
	RefreshAfter, HardExpiry, UnknownKIDCooldown      time.Duration
}

func LoadFromEnv() (Config, bool, error) {
	names := []string{
		"MATCHMAKING_SPACE_PRINCIPAL_GRPC_LISTEN", "MATCHMAKING_SPACE_PRINCIPAL_TLS_CERT_FILE",
		"MATCHMAKING_SPACE_PRINCIPAL_TLS_KEY_FILE", "MATCHMAKING_SPACE_PRINCIPAL_CLIENT_CA_FILE",
		"MATCHMAKING_SPACE_PRINCIPAL_REPLAY_REDIS_ADDR", "MATCHMAKING_SPACE_PRINCIPAL_REPLAY_REDIS_PASSWORD",
		"S2S_JWKS_URLS_JSON", "S2S_JWKS_CA_FILE", "S2S_JWKS_REFRESH_AFTER", "S2S_JWKS_HARD_EXPIRY", "S2S_UNKNOWN_KID_COOLDOWN",
	}
	enabled := false
	for _, name := range names {
		if _, ok := os.LookupEnv(name); ok {
			enabled = true
		}
	}
	if !enabled {
		return Config{}, false, nil
	}
	var jwks map[string]string
	if err := json.Unmarshal([]byte(os.Getenv("S2S_JWKS_URLS_JSON")), &jwks); err != nil {
		return Config{}, true, errors.New("invalid principal JWKS configuration")
	}
	cfg := Config{
		ListenAddr:     strings.TrimSpace(env("MATCHMAKING_SPACE_PRINCIPAL_GRPC_LISTEN", ":9092")),
		TLSCertFile:    strings.TrimSpace(os.Getenv("MATCHMAKING_SPACE_PRINCIPAL_TLS_CERT_FILE")),
		TLSKeyFile:     strings.TrimSpace(os.Getenv("MATCHMAKING_SPACE_PRINCIPAL_TLS_KEY_FILE")),
		ClientCAFile:   strings.TrimSpace(os.Getenv("MATCHMAKING_SPACE_PRINCIPAL_CLIENT_CA_FILE")),
		JWKSCAFile:     strings.TrimSpace(os.Getenv("S2S_JWKS_CA_FILE")),
		JWKSURL:        strings.TrimSpace(jwks["space"]),
		ReplayAddr:     strings.TrimSpace(os.Getenv("MATCHMAKING_SPACE_PRINCIPAL_REPLAY_REDIS_ADDR")),
		ReplayPassword: os.Getenv("MATCHMAKING_SPACE_PRINCIPAL_REPLAY_REDIS_PASSWORD"),
	}
	var err error
	if cfg.RefreshAfter, err = envDuration("S2S_JWKS_REFRESH_AFTER", 30*time.Second); err != nil {
		return Config{}, true, err
	}
	if cfg.HardExpiry, err = envDuration("S2S_JWKS_HARD_EXPIRY", 2*time.Minute); err != nil {
		return Config{}, true, err
	}
	if cfg.UnknownKIDCooldown, err = envDuration("S2S_UNKNOWN_KID_COOLDOWN", 5*time.Second); err != nil {
		return Config{}, true, err
	}
	if err := cfg.validate(); err != nil {
		return Config{}, true, err
	}
	return cfg, true, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return duration, nil
}

func (c Config) validate() error {
	if c.ListenAddr == "" || c.TLSCertFile == "" || c.TLSKeyFile == "" || c.ClientCAFile == "" || c.ReplayAddr == "" || c.JWKSURL == "" {
		return errors.New("Matchmaking Space principal listener, trust, TLS identity, and replay Redis are required")
	}
	if _, _, err := net.SplitHostPort(c.ListenAddr); err != nil {
		return errors.New("invalid Matchmaking Space principal listener")
	}
	if c.RefreshAfter <= 0 || c.HardExpiry < c.RefreshAfter || c.UnknownKIDCooldown <= 0 {
		return errors.New("invalid principal JWKS cache policy")
	}
	endpoint, err := http.NewRequest(http.MethodGet, c.JWKSURL, nil)
	if err != nil || endpoint.URL.Scheme != "https" || endpoint.URL.Hostname() == "" || endpoint.URL.User != nil || endpoint.URL.Fragment != "" {
		return errors.New("Space principal JWKS URL must use HTTPS")
	}
	return nil
}

type Runtime struct {
	config      Config
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
	serverCert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("Matchmaking Space principal TLS identity: %w", err)
	}
	clientCAPEM, err := os.ReadFile(cfg.ClientCAFile)
	if err != nil {
		return nil, fmt.Errorf("Matchmaking Space principal client CA: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(clientCAPEM) {
		return nil, errors.New("Matchmaking Space principal client CA is empty")
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if cfg.JWKSCAFile != "" {
		pem, err := os.ReadFile(cfg.JWKSCAFile)
		if err != nil {
			return nil, fmt.Errorf("Space principal JWKS CA: %w", err)
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("Space principal JWKS CA is empty")
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	client := &http.Client{Transport: transport, Timeout: dependencyTimeout, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("principal JWKS redirects are forbidden")
	}}
	fetch := func(ctx context.Context, issuer string) ([]byte, error) {
		if issuer != "space" {
			return nil, errors.New("untrusted lifecycle principal issuer")
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.JWKSURL, nil)
		if err != nil {
			return nil, err
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, errors.New("Space principal JWKS unavailable")
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, 65537))
		if err != nil || len(body) > 65536 {
			return nil, errors.New("invalid Space principal JWKS response")
		}
		keys, err := principal.ParseJWKS(body)
		if err != nil || len(keys) != 2 {
			return nil, errors.New("Space principal JWKS requires current and next keys")
		}
		var first *rsa.PublicKey
		for kid, key := range keys {
			if !kidPattern.MatchString(kid) || key.N.BitLen() < 2048 || first != nil && first.N.Cmp(key.N) == 0 {
				return nil, errors.New("invalid Space principal rotation keys")
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
	runtime := &Runtime{config: cfg, resolver: resolver, replay: replay, transport: transport,
		credentials: credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs})}
	startup, cancel := context.WithTimeout(ctx, dependencyTimeout)
	defer cancel()
	if err := replay.Ping(startup).Err(); err != nil {
		_ = runtime.Close()
		return nil, errors.New("Matchmaking principal replay Redis unavailable")
	}
	if err := resolver.Refresh(startup, "space"); err != nil {
		_ = runtime.Close()
		return nil, fmt.Errorf("load Space principal JWKS: %w", err)
	}
	refreshCtx, stop := context.WithCancel(ctx)
	runtime.cancel, runtime.done = stop, make(chan struct{})
	go runtime.refreshLoop(refreshCtx)
	return runtime, nil
}

func (r *Runtime) Verify(ctx context.Context, token, method, requestID, requestHash string) (principal.Principal, error) {
	if r == nil || r.resolver == nil || r.replay == nil || !isLifecycleMethod(method) {
		return principal.Principal{}, errors.New("Space principal runtime unavailable")
	}
	verified, err := principal.VerifyService(ctx, token, principal.VerifyConfig{
		ExpectedIssuer: "space", ExpectedAudience: "matchmaking", ExpectedRPC: method,
		ExpectedRequestID: requestID, ExpectedRequestHash: requestHash,
		KeyResolver: r.resolver.Resolve, ReplayGuard: r.recordReplay,
	})
	if err != nil {
		return principal.Principal{}, err
	}
	if verified.Subject != "service:space" || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return principal.Principal{}, errors.New("Space principal contains user authority")
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
	result, err := r.replay.SetArgs(ctx, fmt.Sprintf("matchmaking:space-principal:replay:%x", digest), "1", redis.SetArgs{Mode: "NX", TTL: ttl}).Result()
	if errors.Is(err, redis.Nil) {
		return errors.New("principal replay detected")
	}
	if err != nil {
		return errors.New("Matchmaking principal replay Redis unavailable")
	}
	if result != "OK" {
		return errors.New("principal replay was not recorded")
	}
	return nil
}

func (r *Runtime) refreshLoop(ctx context.Context) {
	defer close(r.done)
	ticker := time.NewTicker(r.config.RefreshAfter)
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

func (r *Runtime) ServerOptions() []grpc.ServerOption {
	if r == nil {
		return []grpc.ServerOption{grpc.ChainUnaryInterceptor(StrictUnaryInterceptor(nil))}
	}
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

type verifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

func isLifecycleMethod(method string) bool {
	return method == matchmakingv1.MatchmakingService_ApplySpaceLifecycleFence_FullMethodName || method == matchmakingv1.MatchmakingService_PurgeSpace_FullMethodName
}

func OrdinaryUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if isLifecycleMethod(info.FullMethod) {
			return nil, status.Error(codes.Unavailable, "protected lifecycle RPC is unavailable on this listener")
		}
		return handler(ctx, request)
	}
}

func StrictUnaryInterceptor(v verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !isLifecycleMethod(info.FullMethod) {
			return nil, status.Error(codes.PermissionDenied, "ordinary RPC is unavailable on lifecycle listener")
		}
		if v == nil {
			return nil, status.Error(codes.Unavailable, "principal verifier unavailable")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		message, ok := request.(proto.Message)
		if !ok || message == nil || !message.ProtoReflect().IsValid() || len(message.ProtoReflect().GetUnknown()) != 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		requestHash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		verified, err := v.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, requestHash)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		if verified.Kind != "service" || verified.Issuer != "space" || verified.Subject != "service:space" || verified.Audience != "matchmaking" || verified.RPC != info.FullMethod || verified.RequestID != transport.RequestID || verified.RequestHash != requestHash || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
			return nil, status.Error(codes.PermissionDenied, "Space lifecycle principal required")
		}
		return handler(principal.WithVerified(ctx, verified), request)
	}
}
