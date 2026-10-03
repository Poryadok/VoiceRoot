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
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/pkg/principal"
)

const (
	Issuer                = "gameintegration"
	Audience              = "chat"
	ProvisionMethod       = "/voice.chat.v1.GameIntegrationChatService/ProvisionManagedChat"
	SyncMembersMethod     = "/voice.chat.v1.GameIntegrationChatService/SyncManagedChatMembers"
	ApplyLifecycleMethod  = "/voice.chat.v1.ChatService/ApplySpaceLifecycleFence"
	PurgeSpaceMethod      = "/voice.chat.v1.ChatService/PurgeSpace"
	PrepareManifestMethod = "/voice.chat.v1.ChatService/PrepareSpaceDeletionManifest"
	GetManifestPageMethod = "/voice.chat.v1.ChatService/GetSpacePurgeManifestPage"
)

func spaceLifecycleMethods() []string {
	return []string{ApplyLifecycleMethod, PurgeSpaceMethod, PrepareManifestMethod, GetManifestPageMethod}
}

type Config struct {
	Issuer             string
	Audience           string
	AllowedMethods     []string
	TLSCertFile        string
	TLSKeyFile         string
	ClientCAFile       string
	JWKSURL            string
	JWKSCAFile         string
	JWKSClientCertFile string
	JWKSClientKeyFile  string
	ReplayAddr         string
	ReplayPassword     string
	RefreshAfter       time.Duration
	HardExpiry         time.Duration
}

func ConfigFromEnv() (Config, bool, error) {
	get := func(key string) string { return strings.TrimSpace(os.Getenv(key)) }
	cfg := Config{
		Issuer: Issuer, Audience: Audience, AllowedMethods: []string{ProvisionMethod, SyncMembersMethod},
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
		return Config{}, true, errors.New("chat GIS principal configuration is incomplete")
	}
	return cfg, true, nil
}

// SpaceLifecycleConfigFromEnv configures a separate mTLS listener for the
// request-bound Space principal. It intentionally has its own server identity,
// trust roots, JWKS source and replay Redis settings from the GIS listener.
func SpaceLifecycleConfigFromEnv() (Config, string, bool, error) {
	get := func(key string) string { return strings.TrimSpace(os.Getenv(key)) }
	listen := get("CHAT_SPACE_LIFECYCLE_GRPC_LISTEN")
	cfg := Config{
		Issuer: "space", Audience: "chat", AllowedMethods: spaceLifecycleMethods(),
		TLSCertFile: get("CHAT_SPACE_LIFECYCLE_TLS_CERT_FILE"), TLSKeyFile: get("CHAT_SPACE_LIFECYCLE_TLS_KEY_FILE"),
		ClientCAFile: get("CHAT_SPACE_LIFECYCLE_CLIENT_CA_FILE"), JWKSURL: get("CHAT_SPACE_PRINCIPAL_JWKS_URL"),
		JWKSCAFile: get("CHAT_SPACE_PRINCIPAL_JWKS_CA_FILE"), ReplayAddr: get("CHAT_SPACE_PRINCIPAL_REPLAY_REDIS_ADDR"),
		ReplayPassword: os.Getenv("CHAT_SPACE_PRINCIPAL_REPLAY_REDIS_PASSWORD"),
	}
	configured := listen != "" || cfg.TLSCertFile != "" || cfg.TLSKeyFile != "" || cfg.ClientCAFile != "" || cfg.JWKSURL != "" || cfg.JWKSCAFile != "" || cfg.ReplayAddr != "" || cfg.ReplayPassword != ""
	if !configured {
		return Config{}, "", false, nil
	}
	if listen == "" || cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" || cfg.ClientCAFile == "" || cfg.JWKSURL == "" || cfg.ReplayAddr == "" {
		return Config{}, "", true, errors.New("chat Space lifecycle principal configuration is incomplete")
	}
	return cfg, listen, true, nil
}

// SearchManifestConfigFromEnv admits only the canonical immutable page read.
// Search never shares Space's lifecycle mutation listener or client CA.
func SearchManifestConfigFromEnv() (Config, string, bool, error) {
	get := func(key string) string { return strings.TrimSpace(os.Getenv(key)) }
	listen := get("CHAT_SEARCH_MANIFEST_GRPC_LISTEN")
	cfg := Config{
		Issuer: "search", Audience: Audience, AllowedMethods: []string{GetManifestPageMethod},
		TLSCertFile: get("CHAT_SEARCH_MANIFEST_TLS_CERT_FILE"), TLSKeyFile: get("CHAT_SEARCH_MANIFEST_TLS_KEY_FILE"),
		ClientCAFile: get("CHAT_SEARCH_MANIFEST_CLIENT_CA_FILE"), JWKSURL: get("CHAT_SEARCH_PRINCIPAL_JWKS_URL"),
		JWKSCAFile: get("CHAT_SEARCH_PRINCIPAL_JWKS_CA_FILE"), ReplayAddr: get("CHAT_SEARCH_PRINCIPAL_REPLAY_REDIS_ADDR"),
		JWKSClientCertFile: get("CHAT_SEARCH_PRINCIPAL_JWKS_CLIENT_CERT_FILE"), JWKSClientKeyFile: get("CHAT_SEARCH_PRINCIPAL_JWKS_CLIENT_KEY_FILE"),
		ReplayPassword: os.Getenv("CHAT_SEARCH_PRINCIPAL_REPLAY_REDIS_PASSWORD"),
	}
	configured := listen != "" || cfg.TLSCertFile != "" || cfg.TLSKeyFile != "" || cfg.ClientCAFile != "" || cfg.JWKSURL != "" || cfg.JWKSCAFile != "" || cfg.ReplayAddr != "" || cfg.ReplayPassword != "" || cfg.JWKSClientCertFile != "" || cfg.JWKSClientKeyFile != ""
	if !configured {
		return Config{}, "", false, nil
	}
	if listen == "" || cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" || cfg.ClientCAFile == "" || cfg.JWKSURL == "" || cfg.JWKSCAFile == "" || cfg.ReplayAddr == "" || (cfg.JWKSClientCertFile == "") != (cfg.JWKSClientKeyFile == "") {
		return Config{}, "", true, errors.New("chat Search manifest principal configuration is incomplete")
	}
	return cfg, listen, true, nil
}

type Runtime struct {
	issuer      string
	audience    string
	methods     map[string]struct{}
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
	if cfg.Issuer == "" {
		cfg.Issuer = Issuer
	}
	if cfg.Audience == "" {
		cfg.Audience = Audience
	}
	if len(cfg.AllowedMethods) == 0 {
		cfg.AllowedMethods = []string{ProvisionMethod, SyncMembersMethod}
	}
	if strings.TrimSpace(cfg.Issuer) == "" || strings.TrimSpace(cfg.Audience) == "" {
		return nil, errors.New("chat principal issuer and audience are required")
	}
	if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" || cfg.ClientCAFile == "" || cfg.JWKSURL == "" || cfg.ReplayAddr == "" {
		return nil, errors.New("chat GIS principal configuration is incomplete")
	}
	jwksURL, err := url.Parse(cfg.JWKSURL)
	if err != nil || jwksURL.Scheme != "https" || jwksURL.Host == "" || jwksURL.User != nil || jwksURL.Fragment != "" {
		return nil, errors.New("chat GIS principal JWKS URL must be an HTTPS endpoint")
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
			return nil, fmt.Errorf("chat GIS JWKS CA: %w", err)
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("chat GIS JWKS CA contains no certificates")
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	if (cfg.JWKSClientCertFile == "") != (cfg.JWKSClientKeyFile == "") {
		return nil, errors.New("chat principal JWKS client certificate and key are required together")
	}
	if cfg.JWKSClientCertFile != "" {
		certificate, err := tls.LoadX509KeyPair(cfg.JWKSClientCertFile, cfg.JWKSClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("chat principal JWKS client identity: %w", err)
		}
		transport.TLSClientConfig.Certificates = []tls.Certificate{certificate}
	}
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("jwks redirects forbidden") }}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{
		Fetch: func(ctx context.Context, issuer string) ([]byte, error) {
			if issuer != cfg.Issuer {
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
			defer func() { _ = resp.Body.Close() }()
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
	methods, methodErr := configuredMethods(cfg.Issuer, cfg.AllowedMethods)
	if methodErr != nil {
		_ = replay.Close()
		transport.CloseIdleConnections()
		return nil, methodErr
	}
	runtime := &Runtime{issuer: cfg.Issuer, audience: cfg.Audience, methods: methods, resolver: resolver, replay: replay, transport: transport, credentials: credentials.NewTLS(serverTLS), done: make(chan struct{})}
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
	if err := resolver.Refresh(startup, cfg.Issuer); err != nil {
		_ = replay.Close()
		transport.CloseIdleConnections()
		return nil, fmt.Errorf("principal JWKS unavailable: %w", err)
	}
	refreshCtx, stop := context.WithCancel(ctx)
	runtime.stop = stop
	go runtime.refresh(refreshCtx, cfg.RefreshAfter)
	return runtime, nil
}

func configuredMethods(issuer string, allowed []string) (map[string]struct{}, error) {
	approved := make(map[string]struct{})
	switch issuer {
	case Issuer:
		approved[ProvisionMethod] = struct{}{}
		approved[SyncMembersMethod] = struct{}{}
	case "space":
		for _, method := range spaceLifecycleMethods() {
			approved[method] = struct{}{}
		}
	case "search":
		approved[GetManifestPageMethod] = struct{}{}
	default:
		return nil, errors.New("untrusted Chat principal issuer")
	}
	methods := make(map[string]struct{}, len(allowed))
	for _, method := range allowed {
		if _, ok := approved[method]; !ok || strings.TrimSpace(method) != method {
			return nil, errors.New("invalid Chat principal method allowlist")
		}
		if _, exists := methods[method]; exists {
			return nil, errors.New("duplicate Chat principal method")
		}
		methods[method] = struct{}{}
	}
	if len(methods) == 0 {
		return nil, errors.New("chat principal method allowlist is empty")
	}
	return methods, nil
}

func loadServerTLS(certFile, keyFile, clientCAFile string) (*tls.Config, error) {
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("chat GIS TLS certificate: %w", err)
	}
	clientCAPEM, err := os.ReadFile(clientCAFile)
	if err != nil {
		return nil, fmt.Errorf("chat GIS client CA: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(clientCAPEM) {
		return nil, errors.New("chat GIS client CA contains no certificates")
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
			_ = r.resolver.Refresh(ctx, r.issuer)
		}
	}
}

func (r *Runtime) ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.Creds(r.credentials), grpc.ChainUnaryInterceptor(r.allowlist, strictInterceptor{runtime: r}.intercept)}
}

func (r *Runtime) allowlist(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if _, ok := r.methods[info.FullMethod]; !ok {
		return nil, status.Error(codes.PermissionDenied, "method unavailable on protected Chat listener")
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
	issuer, audience := i.runtime.issuer, i.runtime.audience
	if issuer == "" {
		issuer = Issuer
	}
	if audience == "" {
		audience = Audience
	}
	verified, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{
		ExpectedIssuer: issuer, ExpectedAudience: audience, ExpectedRPC: info.FullMethod,
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
	if verified.Subject != "service:"+issuer || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return nil, status.Error(codes.PermissionDenied, "Chat service principal required")
	}
	return handler(principal.WithVerified(ctx, verified), req)
}

func requestOperationID(message proto.Message) (string, bool) {
	switch request := message.(type) {
	case *chatv1.ApplySpaceLifecycleFenceRequest:
		if request.GetFence() == nil {
			return "", false
		}
		return request.GetFence().GetDeletionOperationId(), true
	case *chatv1.PurgeSpaceRequest:
		if request.GetPurge() == nil {
			return "", false
		}
		return request.GetPurge().GetDeletionOperationId(), true
	case *chatv1.GetSpacePurgeManifestPageRequest:
		return request.GetDeletionOperationId(), true
	case *chatv1.PrepareSpaceDeletionManifestRequest:
		return request.GetDeletionOperationId(), true
	case interface {
		GetFence() *commonv1.SpaceLifecycleFenceRequest
	}:
		if request.GetFence() == nil {
			return "", false
		}
		return request.GetFence().GetDeletionOperationId(), true
	case interface {
		GetPurge() *commonv1.SpacePurgeRequest
	}:
		if request.GetPurge() == nil {
			return "", false
		}
		return request.GetPurge().GetDeletionOperationId(), true
	case interface{ GetOperationId() string }:
		return request.GetOperationId(), true
	case interface{ GetDeletionOperationId() string }:
		return request.GetDeletionOperationId(), true
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
