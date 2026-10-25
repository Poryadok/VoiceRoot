package installprincipal

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
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
	"voice/backend/pkg/principal"
)

type Config struct{ Listen, CertFile, KeyFile, ClientCAFile, JWKSURL, JWKSCAFile, ReplayURL string }

func LoadConfig(lookup func(string) (string, bool)) (Config, bool, error) {
	names := []string{"BOT_INSTALL_GRPC_LISTEN", "BOT_INSTALL_TLS_CERT_FILE", "BOT_INSTALL_TLS_KEY_FILE", "BOT_INSTALL_TLS_CLIENT_CA_FILE", "BOT_INSTALL_GATEWAY_JWKS_URL", "BOT_INSTALL_GATEWAY_JWKS_CA_FILE"}
	values := make([]string, len(names))
	enabled := false
	for i, n := range names {
		v, p := lookup(n)
		values[i] = v
		enabled = enabled || p
	}
	if !enabled {
		return Config{}, false, nil
	}
	for _, v := range values {
		if v == "" || strings.TrimSpace(v) != v {
			return Config{}, true, ErrAdministrator
		}
	}
	replay, _ := lookup("BOT_PRINCIPAL_REPLAY_REDIS_URL")
	c := Config{values[0], values[1], values[2], values[3], values[4], values[5], replay}
	u, err := url.Parse(c.JWKSURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || replay == "" {
		return Config{}, true, ErrAdministrator
	}
	return c, true, nil
}

type Runtime struct {
	Verifier    *Verifier
	credentials credentials.TransportCredentials
	redis       *redis.Client
	http        *http.Client
	once        sync.Once
	closeErr    error
}

func rootCA(path string) (*x509.CertPool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	p := x509.NewCertPool()
	if !p.AppendCertsFromPEM(raw) {
		return nil, ErrAdministrator
	}
	return p, nil
}
func New(ctx context.Context, c Config, epoch principal.SessionEpochChecker) (result *Runtime, err error) {
	if epoch == nil {
		return nil, ErrAdministrator
	}
	cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, err
	}
	clients, err := rootCA(c.ClientCAFile)
	if err != nil {
		return nil, err
	}
	roots, err := rootCA(c.JWKSCAFile)
	if err != nil {
		return nil, err
	}
	options, err := redis.ParseURL(c.ReplayURL)
	if err != nil {
		return nil, err
	}
	options.DialTimeout = 2 * time.Second
	options.ReadTimeout = 2 * time.Second
	options.WriteTimeout = 2 * time.Second
	options.MaxRetries = -1
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{cert}}
	r := &Runtime{redis: redis.NewClient(options), http: &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return ErrAdministrator }}, credentials: credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientCAs: clients, ClientAuth: tls.RequireAndVerifyClientCert})}
	defer func() {
		if err != nil {
			_ = r.Close()
		}
	}()
	if err = r.redis.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{Fetch: func(ctx context.Context, issuer string) ([]byte, error) {
		if issuer != "gateway" {
			return nil, ErrAdministrator
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.JWKSURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := r.http.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, ErrAdministrator
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
		if err != nil || len(body) > 65536 {
			return nil, ErrAdministrator
		}
		keys, err := principal.ParseJWKS(body)
		if err != nil || len(keys) != 2 {
			return nil, ErrAdministrator
		}
		seen := map[string]bool{}
		for _, key := range keys {
			n := key.N.String()
			if key.N.BitLen() < 2048 || seen[n] {
				return nil, ErrAdministrator
			}
			seen[n] = true
		}
		return body, nil
	}, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second})
	if err != nil {
		return nil, err
	}
	// Gateway can publish after Bot starts. No request is admitted without a
	// successful fetch; the resolver keeps its bounded last-good set on failures.
	r.Verifier = &Verifier{Keys: resolver.Resolve, Replay: r.recordReplay, Epoch: epoch}
	return r, nil
}
func (r *Runtime) recordReplay(ctx context.Context, issuer, jti string, expiry time.Time) error {
	ttl := time.Until(expiry)
	if issuer != "gateway" || jti == "" || ttl <= 0 || ttl > 35*time.Second {
		return ErrAdministrator
	}
	hash := sha256.Sum256([]byte(issuer + "\x00" + jti))
	value, err := r.redis.SetArgs(ctx, fmt.Sprintf("bot:install:replay:%x", hash), "1", redis.SetArgs{Mode: "NX", TTL: ttl}).Result()
	if err != nil || value != "OK" {
		return ErrAdministrator
	}
	return nil
}
func (r *Runtime) ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.Creds(r.credentials), grpc.ChainUnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		if !Protected(info.FullMethod) {
			return nil, status.Error(codes.PermissionDenied, "method is not available on installation listener")
		}
		return r.Verifier.Unary()(ctx, req, info, next)
	}), grpc.ChainStreamInterceptor(func(_ any, _ grpc.ServerStream, _ *grpc.StreamServerInfo, _ grpc.StreamHandler) error {
		return status.Error(codes.PermissionDenied, "streaming is not available on installation listener")
	})}
}
func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	r.once.Do(func() {
		if r.http != nil {
			r.http.CloseIdleConnections()
		}
		if r.redis != nil {
			r.closeErr = r.redis.Close()
		}
	})
	return r.closeErr
}
