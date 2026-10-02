package principalruntime

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
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"voice/backend/pkg/principal"
)

type Config struct {
	GISJWKSURL, GISTLSCert, GISTLSKey, GISCAFile, ReplayRedisURL                                    string
	BotPrivateKeyFile, BotKeyID, BotJWKSListen, BotTLSCert, BotTLSKey                               string
	BotJWKSClientCA, BotGameEventListen, BotGameEventCert, BotGameEventKey, BotGameEventClientCA    string
	BotSpaceLifecycleListen, BotSpaceLifecycleCert, BotSpaceLifecycleKey, BotSpaceLifecycleClientCA string
	SpaceJWKSURL, SpaceJWKSCAFile                                                                   string
}

type Runtime struct {
	issuer           *principal.Issuer
	resolver         *principal.JWKSResolver
	redis            *redis.Client
	http             *http.Client
	spaceHTTP        *http.Client
	config           Config
	privateKey       *rsa.PrivateKey
	jwksCertificate  tls.Certificate
	spaceResolver    *principal.JWKSResolver
	spaceCertificate tls.Certificate
	replayGuard      principal.ReplayGuard
}

func ConfigFromEnv(getenv func(string) string) (Config, bool, error) {
	if getenv == nil {
		return Config{}, false, errors.New("environment reader is required")
	}
	cfg := Config{GISJWKSURL: strings.TrimSpace(getenv("GAME_INTEGRATION_PRINCIPAL_JWKS_URL")),
		GISTLSCert: strings.TrimSpace(getenv("GAME_INTEGRATION_PRINCIPAL_TLS_CERT_FILE")), GISTLSKey: strings.TrimSpace(getenv("GAME_INTEGRATION_PRINCIPAL_TLS_KEY_FILE")),
		GISCAFile: strings.TrimSpace(getenv("GAME_INTEGRATION_PRINCIPAL_TLS_CA_FILE")), ReplayRedisURL: strings.TrimSpace(getenv("BOT_PRINCIPAL_REPLAY_REDIS_URL")),
		BotPrivateKeyFile: strings.TrimSpace(getenv("BOT_PRINCIPAL_PRIVATE_KEY_FILE")), BotKeyID: strings.TrimSpace(getenv("BOT_PRINCIPAL_KEY_ID")),
		BotJWKSListen: strings.TrimSpace(getenv("BOT_PRINCIPAL_JWKS_LISTEN")), BotTLSCert: strings.TrimSpace(getenv("BOT_PRINCIPAL_TLS_CERT_FILE")), BotTLSKey: strings.TrimSpace(getenv("BOT_PRINCIPAL_TLS_KEY_FILE")),
		BotJWKSClientCA: strings.TrimSpace(getenv("BOT_PRINCIPAL_TLS_CLIENT_CA_FILE")), BotGameEventListen: strings.TrimSpace(getenv("BOT_GAME_EVENT_GRPC_LISTEN")),
		BotGameEventCert: strings.TrimSpace(getenv("BOT_GAME_EVENT_TLS_CERT_FILE")), BotGameEventKey: strings.TrimSpace(getenv("BOT_GAME_EVENT_TLS_KEY_FILE")),
		BotGameEventClientCA:      strings.TrimSpace(getenv("BOT_GAME_EVENT_CLIENT_CA_FILE")),
		BotSpaceLifecycleListen:   strings.TrimSpace(getenv("BOT_SPACE_LIFECYCLE_GRPC_LISTEN")),
		BotSpaceLifecycleCert:     strings.TrimSpace(getenv("BOT_SPACE_LIFECYCLE_TLS_CERT_FILE")),
		BotSpaceLifecycleKey:      strings.TrimSpace(getenv("BOT_SPACE_LIFECYCLE_TLS_KEY_FILE")),
		BotSpaceLifecycleClientCA: strings.TrimSpace(getenv("BOT_SPACE_LIFECYCLE_CLIENT_CA_FILE")),
		SpaceJWKSURL:              strings.TrimSpace(getenv("SPACE_PRINCIPAL_JWKS_URL")),
		SpaceJWKSCAFile:           strings.TrimSpace(getenv("SPACE_PRINCIPAL_JWKS_CA_FILE"))}
	values := []string{cfg.GISJWKSURL, cfg.GISTLSCert, cfg.GISTLSKey, cfg.GISCAFile, cfg.ReplayRedisURL, cfg.BotPrivateKeyFile, cfg.BotKeyID, cfg.BotJWKSListen, cfg.BotTLSCert, cfg.BotTLSKey,
		cfg.BotJWKSClientCA, cfg.BotGameEventListen, cfg.BotGameEventCert, cfg.BotGameEventKey, cfg.BotGameEventClientCA}
	count := 0
	for _, value := range values {
		if value != "" {
			count++
		}
	}
	if count == 0 {
		return Config{}, false, nil
	}
	if count != len(values) {
		return Config{}, true, errors.New("Bot and GIS service-principal TLS, replay, signer, and JWKS settings must be configured together")
	}
	if !strings.HasPrefix(cfg.GISJWKSURL, "https://") || !strings.HasSuffix(cfg.GISJWKSURL, "/internal/v1/principal/jwks.json") {
		return Config{}, true, errors.New("Game Integration principal JWKS URL must use the internal HTTPS endpoint")
	}
	spaceValues := []string{cfg.BotSpaceLifecycleListen, cfg.BotSpaceLifecycleCert, cfg.BotSpaceLifecycleKey, cfg.BotSpaceLifecycleClientCA, cfg.SpaceJWKSURL, cfg.SpaceJWKSCAFile}
	spaceCount := 0
	for _, value := range spaceValues {
		if value != "" {
			spaceCount++
		}
	}
	if spaceCount != 0 && spaceCount != len(spaceValues) {
		return Config{}, true, errors.New("Bot Space lifecycle mTLS and Space principal JWKS settings must be configured together")
	}
	if spaceCount > 0 {
		parsed, err := url.Parse(cfg.SpaceJWKSURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "/space/jwks.json" {
			return Config{}, true, errors.New("Space principal JWKS URL must use the HTTPS Space JWKS endpoint")
		}
	}
	return cfg, true, nil
}

func (c Config) SpaceLifecycleEnabled() bool {
	return strings.TrimSpace(c.BotSpaceLifecycleListen) != ""
}

func New(ctx context.Context, cfg Config) (*Runtime, error) {
	if strings.TrimSpace(cfg.BotPrivateKeyFile) == "" || strings.TrimSpace(cfg.BotKeyID) == "" {
		return nil, errors.New("Bot principal signer is incomplete")
	}
	key, err := principal.LoadRSAPrivateKeyFile(cfg.BotPrivateKeyFile)
	if err != nil {
		return nil, err
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: cfg.BotKeyID, PrivateKey: key})
	if err != nil {
		return nil, err
	}
	cert, err := tls.LoadX509KeyPair(cfg.GISTLSCert, cfg.GISTLSKey)
	if err != nil {
		return nil, fmt.Errorf("GIS principal JWKS client certificate: %w", err)
	}
	caPEM, err := os.ReadFile(cfg.GISCAFile)
	if err != nil {
		return nil, err
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("GIS principal JWKS CA contains no certificates")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{cert}}
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("principal JWKS redirects are forbidden")
	}}
	fetch := func(ctx context.Context, issuer string) ([]byte, error) {
		if issuer != "gameintegration" {
			return nil, errors.New("untrusted service principal issuer")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.GISJWKSURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GIS principal JWKS returned status %d", resp.StatusCode)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 128*1024+1))
		if err != nil || len(body) > 128*1024 {
			return nil, errors.New("GIS principal JWKS response is invalid")
		}
		return body, nil
	}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{Fetch: fetch, RefreshAfter: 20 * time.Second, HardExpiry: 60 * time.Second, UnknownKIDCooldown: 5 * time.Second})
	if err != nil {
		return nil, err
	}
	redisOptions, err := redis.ParseURL(cfg.ReplayRedisURL)
	if err != nil {
		return nil, errors.New("Bot principal replay Redis URL is invalid")
	}
	redisClient := redis.NewClient(redisOptions)
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		_ = redisClient.Close()
		return nil, errors.New("Bot principal replay Redis unavailable")
	}
	botCertificate, err := tls.LoadX509KeyPair(cfg.BotTLSCert, cfg.BotTLSKey)
	if err != nil {
		_ = redisClient.Close()
		return nil, fmt.Errorf("Bot principal JWKS TLS certificate: %w", err)
	}
	runtime := &Runtime{issuer: issuer, resolver: resolver, redis: redisClient, http: client, config: cfg, privateKey: key, jwksCertificate: botCertificate}
	runtime.replayGuard = runtime.recordReplay
	if cfg.SpaceLifecycleEnabled() {
		spaceCAPEM, err := os.ReadFile(cfg.SpaceJWKSCAFile)
		if err != nil {
			_ = redisClient.Close()
			return nil, fmt.Errorf("Space principal JWKS CA: %w", err)
		}
		spaceRoots := x509.NewCertPool()
		if !spaceRoots.AppendCertsFromPEM(spaceCAPEM) {
			_ = redisClient.Close()
			return nil, errors.New("Space principal JWKS CA contains no certificates")
		}
		spaceTransport := http.DefaultTransport.(*http.Transport).Clone()
		spaceTransport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: spaceRoots}
		spaceClient := &http.Client{Transport: spaceTransport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("principal JWKS redirects are forbidden")
		}}
		spaceResolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{Fetch: func(ctx context.Context, issuer string) ([]byte, error) {
			if issuer != "space" {
				return nil, errors.New("untrusted service principal issuer")
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.SpaceJWKSURL, nil)
			if err != nil {
				return nil, err
			}
			resp, err := spaceClient.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("Space principal JWKS returned status %d", resp.StatusCode)
			}
			body, err := io.ReadAll(io.LimitReader(resp.Body, 128*1024+1))
			if err != nil || len(body) > 128*1024 {
				return nil, errors.New("Space principal JWKS response is invalid")
			}
			return body, nil
		}, RefreshAfter: 20 * time.Second, HardExpiry: 60 * time.Second, UnknownKIDCooldown: 5 * time.Second})
		if err != nil {
			_ = redisClient.Close()
			spaceTransport.CloseIdleConnections()
			return nil, err
		}
		warmCtx, warmCancel := context.WithTimeout(ctx, 2*time.Second)
		err = spaceResolver.Refresh(warmCtx, "space")
		warmCancel()
		if err != nil {
			_ = redisClient.Close()
			spaceTransport.CloseIdleConnections()
			return nil, fmt.Errorf("initial Space principal JWKS refresh: %w", err)
		}
		spaceCert, err := tls.LoadX509KeyPair(cfg.BotSpaceLifecycleCert, cfg.BotSpaceLifecycleKey)
		if err != nil {
			_ = redisClient.Close()
			spaceTransport.CloseIdleConnections()
			return nil, fmt.Errorf("Bot Space lifecycle TLS certificate: %w", err)
		}
		runtime.spaceResolver, runtime.spaceCertificate, runtime.spaceHTTP = spaceResolver, spaceCert, spaceClient
	}
	return runtime, nil
}

func (r *Runtime) Issuer() *principal.Issuer {
	if r == nil {
		return nil
	}
	return r.issuer
}

func (r *Runtime) JWKSHTTPServer() (*http.Server, error) {
	if r == nil {
		return nil, errors.New("Bot principal runtime unavailable")
	}
	caPEM, err := os.ReadFile(r.config.BotJWKSClientCA)
	if err != nil {
		return nil, err
	}
	clients := x509.NewCertPool()
	if !clients.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("Bot principal client CA contains no certificates")
	}
	mux := http.NewServeMux()
	mux.Handle("/.well-known/principal-jwks.json", principal.JWKSHandler(r.config.BotKeyID, &r.privateKey.PublicKey))
	return &http.Server{Addr: r.config.BotJWKSListen, Handler: mux, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{r.jwksCertificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clients}}, nil
}

func (r *Runtime) GameEventServerTLSConfig() (*tls.Config, error) {
	if r == nil {
		return nil, errors.New("Bot principal runtime unavailable")
	}
	certificate, err := tls.LoadX509KeyPair(r.config.BotGameEventCert, r.config.BotGameEventKey)
	if err != nil {
		return nil, err
	}
	caPEM, err := os.ReadFile(r.config.BotGameEventClientCA)
	if err != nil {
		return nil, err
	}
	clients := x509.NewCertPool()
	if !clients.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("Bot game event client CA contains no certificates")
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clients}, nil
}

func (r *Runtime) SpaceLifecycleListenAddr() string {
	if r == nil {
		return ""
	}
	return r.config.BotSpaceLifecycleListen
}

func (r *Runtime) SpaceLifecycleServerTLSConfig() (*tls.Config, error) {
	if r == nil || r.spaceResolver == nil {
		return nil, errors.New("Bot Space lifecycle runtime unavailable")
	}
	caPEM, err := os.ReadFile(r.config.BotSpaceLifecycleClientCA)
	if err != nil {
		return nil, err
	}
	clients := x509.NewCertPool()
	if !clients.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("Space lifecycle client CA contains no certificates")
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{r.spaceCertificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clients}, nil
}

func (r *Runtime) VerifySpace(ctx context.Context, token, method, requestID, hash string) (principal.Principal, error) {
	if r == nil || r.spaceResolver == nil || r.replayGuard == nil {
		return principal.Principal{}, errors.New("Bot Space lifecycle runtime unavailable")
	}
	if method != "" && method != "/voice.bot.v1.BotService/ApplySpaceLifecycleFence" && method != "/voice.bot.v1.BotService/PurgeSpace" {
		return principal.Principal{}, errors.New("Space principal method is not allowed")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return principal.Principal{}, errors.New("invalid service principal")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || base64.RawURLEncoding.EncodeToString(payload) != parts[1] {
		return principal.Principal{}, errors.New("invalid service principal")
	}
	var hint struct {
		Issuer string `json:"iss"`
	}
	if json.Unmarshal(payload, &hint) != nil || hint.Issuer != "space" {
		return principal.Principal{}, errors.New("untrusted service principal issuer")
	}
	p, err := principal.VerifyService(ctx, token, principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "bot", ExpectedRPC: method, ExpectedRequestID: requestID, ExpectedRequestHash: hash, KeyResolver: r.spaceResolver.Resolve, ReplayGuard: r.replayGuard})
	if err != nil {
		return principal.Principal{}, err
	}
	if p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 {
		return principal.Principal{}, errors.New("service principal contains user authority")
	}
	return p, nil
}

func (r *Runtime) Verify(ctx context.Context, token, method, requestID, hash string) (principal.Principal, error) {
	if r == nil || r.resolver == nil || r.redis == nil {
		return principal.Principal{}, errors.New("Bot principal runtime unavailable")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return principal.Principal{}, errors.New("invalid service principal")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || base64.RawURLEncoding.EncodeToString(payload) != parts[1] {
		return principal.Principal{}, errors.New("invalid service principal")
	}
	var hint struct {
		Issuer string `json:"iss"`
	}
	if json.Unmarshal(payload, &hint) != nil || hint.Issuer != "gameintegration" {
		return principal.Principal{}, errors.New("untrusted service principal issuer")
	}
	return principal.VerifyService(ctx, token, principal.VerifyConfig{ExpectedIssuer: "gameintegration", ExpectedAudience: "bot", ExpectedRPC: method, ExpectedRequestID: requestID, ExpectedRequestHash: hash, KeyResolver: r.resolver.Resolve, ReplayGuard: r.recordReplay})
}

func (r *Runtime) recordReplay(ctx context.Context, issuer, jwtID string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid principal replay lifetime")
	}
	digest := sha256.Sum256([]byte(issuer + "\x00" + jwtID))
	result, err := r.redis.SetArgs(ctx, fmt.Sprintf("bot:%x", digest), "1", redis.SetArgs{Mode: "NX", TTL: ttl}).Result()
	if errors.Is(err, redis.Nil) {
		return errors.New("service principal replay detected")
	}
	if err != nil || result != "OK" {
		return errors.New("service principal replay storage unavailable")
	}
	return nil
}
func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	if r.http != nil {
		r.http.CloseIdleConnections()
	}
	if r.spaceHTTP != nil {
		r.spaceHTTP.CloseIdleConnections()
	}
	if r.redis != nil {
		return r.redis.Close()
	}
	return nil
}
