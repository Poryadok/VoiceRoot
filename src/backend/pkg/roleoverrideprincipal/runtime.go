package roleoverrideprincipal

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
	"voice/backend/pkg/principal"
)

const dependencyTimeout = 2 * time.Second

type Config struct {
	JWKSClientCertFile, JWKSClientKeyFile                                                             string
	Target, ListenAddr, TLSCertFile, TLSKeyFile, ClientCAFile, JWKSCAFile, ReplayAddr, ReplayPassword string
	JWKSURLs                                                                                          map[string]string
	RefreshAfter, HardExpiry, UnknownKIDCooldown                                                      time.Duration
}

func LoadFromEnv(target string) (Config, bool, error) {
	prefix := strings.ToUpper(target) + "_ROLE_OVERRIDE_PRINCIPAL_"
	names := []string{"GRPC_LISTEN", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE", "REPLAY_REDIS_ADDR", "REPLAY_REDIS_PASSWORD"}
	enabled := false
	for _, name := range names {
		if _, ok := os.LookupEnv(prefix + name); ok {
			enabled = true
		}
	}
	if !enabled {
		return Config{}, false, nil
	}
	cfg := Config{Target: target, ListenAddr: strings.TrimSpace(os.Getenv(prefix + "GRPC_LISTEN")), TLSCertFile: strings.TrimSpace(os.Getenv(prefix + "TLS_CERT_FILE")), TLSKeyFile: strings.TrimSpace(os.Getenv(prefix + "TLS_KEY_FILE")), ClientCAFile: strings.TrimSpace(os.Getenv(prefix + "CLIENT_CA_FILE")), JWKSCAFile: strings.TrimSpace(os.Getenv("S2S_JWKS_CA_FILE")), ReplayAddr: strings.TrimSpace(os.Getenv(prefix + "REPLAY_REDIS_ADDR")), ReplayPassword: os.Getenv(prefix + "REPLAY_REDIS_PASSWORD")}
	cfg.JWKSClientCertFile = strings.TrimSpace(os.Getenv("S2S_JWKS_CLIENT_CERT_FILE"))
	cfg.JWKSClientKeyFile = strings.TrimSpace(os.Getenv("S2S_JWKS_CLIENT_KEY_FILE"))
	if err := json.Unmarshal([]byte(os.Getenv("S2S_JWKS_URLS_JSON")), &cfg.JWKSURLs); err != nil {
		return Config{}, true, errors.New("invalid target principal JWKS configuration")
	}
	var err error
	cfg.RefreshAfter, err = envDuration("S2S_JWKS_REFRESH_AFTER", 30*time.Second)
	if err != nil {
		return Config{}, true, err
	}
	cfg.HardExpiry, err = envDuration("S2S_JWKS_HARD_EXPIRY", 2*time.Minute)
	if err != nil {
		return Config{}, true, err
	}
	cfg.UnknownKIDCooldown, err = envDuration("S2S_UNKNOWN_KID_COOLDOWN", 5*time.Second)
	if err != nil {
		return Config{}, true, err
	}
	return cfg, true, cfg.validate()
}

func envDuration(name string, fallback time.Duration) (time.Duration, error) {
	raw, ok := os.LookupEnv(name)
	if !ok {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return value, nil
}

func (c Config) validate() error {
	if (c.JWKSClientCertFile == "") != (c.JWKSClientKeyFile == "") {
		return errors.New("target JWKS client certificate and key must be paired")
	}
	if Method(c.Target) == "" || c.ListenAddr == "" || c.TLSCertFile == "" || c.TLSKeyFile == "" || c.ClientCAFile == "" || c.ReplayAddr == "" {
		return errors.New("complete target principal listener, mTLS and replay configuration required")
	}
	if c.RefreshAfter <= 0 || c.HardExpiry < c.RefreshAfter || c.UnknownKIDCooldown <= 0 {
		return errors.New("invalid target principal cache policy")
	}
	if c.JWKSURLs["role"] == "" {
		return errors.New("role JWKS endpoint required")
	}
	for issuer, endpoint := range c.JWKSURLs {
		parsed, err := url.Parse(endpoint)
		if strings.TrimSpace(issuer) == "" || err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
			return errors.New("invalid target principal HTTPS JWKS endpoint")
		}
	}
	return nil
}

type Runtime struct {
	target        string
	endpoints     map[string]string
	resolver      *principal.JWKSResolver
	replay        *redis.Client
	httpTransport *http.Transport
	credentials   credentials.TransportCredentials
	cancel        context.CancelFunc
	done          chan struct{}
	closeOnce     sync.Once
	closeErr      error
}

func New(ctx context.Context, cfg Config) (*Runtime, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, errors.New("target listener identity unavailable")
	}
	clientPEM, err := os.ReadFile(cfg.ClientCAFile)
	if err != nil {
		return nil, errors.New("target client CA unavailable")
	}
	clientRoots := x509.NewCertPool()
	if !clientRoots.AppendCertsFromPEM(clientPEM) {
		return nil, errors.New("invalid target client CA")
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if cfg.JWKSCAFile != "" {
		data, err := os.ReadFile(cfg.JWKSCAFile)
		if err != nil || !roots.AppendCertsFromPEM(data) {
			return nil, errors.New("invalid target JWKS CA")
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	if cfg.JWKSClientCertFile != "" {
		clientCert, err := tls.LoadX509KeyPair(cfg.JWKSClientCertFile, cfg.JWKSClientKeyFile)
		if err != nil {
			return nil, errors.New("target JWKS client identity unavailable")
		}
		transport.TLSClientConfig.Certificates = []tls.Certificate{clientCert}
	}
	client := &http.Client{Transport: transport, Timeout: dependencyTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("target JWKS redirects forbidden") }}
	endpoints := make(map[string]string, len(cfg.JWKSURLs))
	for issuer, endpoint := range cfg.JWKSURLs {
		if issuer == "role" || (cfg.Target == "space" && issuer == "messaging") {
			endpoints[issuer] = endpoint
		}
	}
	fetch := func(ctx context.Context, issuer string) ([]byte, error) {
		endpoint, ok := endpoints[issuer]
		if !ok {
			return nil, errors.New("untrusted target principal issuer")
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("%w: JWKS transport", ErrUnavailable)
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%w: JWKS response", ErrUnavailable)
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, 65537))
		if err != nil {
			return nil, fmt.Errorf("%w: JWKS read", ErrUnavailable)
		}
		if len(body) > 65536 {
			return nil, errors.New("oversized target JWKS")
		}
		keys, err := principal.ParseJWKS(body)
		if err != nil || len(keys) != 2 {
			return nil, errors.New("target JWKS requires current and next keys")
		}
		var first *rsa.PublicKey
		for _, key := range keys {
			if key.N.BitLen() < 2048 || first != nil && first.N.Cmp(key.N) == 0 {
				return nil, errors.New("invalid target rotation keys")
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
	startup, cancel := context.WithTimeout(ctx, dependencyTimeout)
	defer cancel()
	if err := replay.Ping(startup).Err(); err != nil {
		transport.CloseIdleConnections()
		_ = replay.Close()
		return nil, errors.New("target replay Redis unavailable")
	}
	serverTLS := &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientRoots}
	refreshCtx, stop := context.WithCancel(ctx)
	runtime := &Runtime{target: cfg.Target, endpoints: endpoints, resolver: resolver, replay: replay, httpTransport: transport, credentials: credentials.NewTLS(serverTLS), cancel: stop, done: make(chan struct{})}
	go func() {
		defer close(runtime.done)
		ticker := time.NewTicker(cfg.RefreshAfter)
		defer ticker.Stop()
		for {
			select {
			case <-refreshCtx.Done():
				return
			case <-ticker.C:
				for issuer := range endpoints {
					_ = resolver.Refresh(refreshCtx, issuer)
				}
			}
		}
	}()
	return runtime, nil
}

func (r *Runtime) Verify(ctx context.Context, token, method, requestID, hash string) (principal.Principal, error) {
	if r == nil || r.resolver == nil || r.replay == nil {
		return principal.Principal{}, ErrUnavailable
	}
	if !allowsMethod(r.target, method) {
		return principal.Principal{}, errors.New("invalid target method")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(token) > 16384 {
		return principal.Principal{}, errors.New("invalid target credential")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return principal.Principal{}, err
	}
	var claims struct {
		Issuer string `json:"iss"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return principal.Principal{}, err
	}
	trusted := claims.Issuer == "role" || (r.target == "space" && method == MemberAdmissionAuthorityMethod && claims.Issuer == "messaging")
	if r.endpoints[claims.Issuer] == "" || !trusted {
		return principal.Principal{}, errors.New("untrusted target principal issuer")
	}
	return principal.VerifyService(ctx, token, principal.VerifyConfig{ExpectedIssuer: claims.Issuer, ExpectedAudience: r.target, ExpectedRPC: method, ExpectedRequestID: requestID, ExpectedRequestHash: hash, KeyResolver: r.resolver.Resolve, ReplayGuard: r.recordReplay})
}

func (r *Runtime) recordReplay(ctx context.Context, issuer, jti string, expiry time.Time) error {
	ttl := time.Until(expiry)
	if ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid target replay lifetime")
	}
	ttl = ((ttl + time.Millisecond - 1) / time.Millisecond) * time.Millisecond
	digest := sha256.Sum256([]byte(issuer + "\x00" + jti))
	result, err := r.replay.SetArgs(ctx, fmt.Sprintf("%s:role-override:principal:replay:%x", r.target, digest), "1", redis.SetArgs{Mode: "NX", TTL: ttl}).Result()
	if errors.Is(err, redis.Nil) {
		return errors.New("target principal replay")
	}
	if err != nil {
		return ErrUnavailable
	}
	if result != "OK" {
		return errors.New("target replay not recorded")
	}
	return nil
}

func (r *Runtime) ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.Creds(r.credentials), grpc.ChainUnaryInterceptor(StrictUnaryInterceptor(r.target, r))}
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
		if r.httpTransport != nil {
			r.httpTransport.CloseIdleConnections()
		}
		if r.replay != nil {
			r.closeErr = r.replay.Close()
		}
	})
	return r.closeErr
}
