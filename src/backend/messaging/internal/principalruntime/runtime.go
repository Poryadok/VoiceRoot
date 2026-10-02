package principalruntime

import (
	"context"
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
	JWKSURL     string
	TLSCertFile string
	TLSKeyFile  string
	CAFile      string
	RedisURL    string
}

type Runtime struct {
	issuer   string
	resolver *principal.JWKSResolver
	redis    *redis.Client
	http     *http.Client
}

func New(ctx context.Context, config Config) (*Runtime, error) {
	return NewForIssuer(ctx, config, "moderation")
}

func NewForIssuer(ctx context.Context, config Config, expectedIssuer string) (*Runtime, error) {
	if expectedIssuer != "moderation" && expectedIssuer != "gateway" && expectedIssuer != "bot" && expectedIssuer != "gameintegration" && expectedIssuer != "chat" && expectedIssuer != "space" {
		return nil, errors.New("unsupported service principal issuer")
	}
	if !validPrincipalJWKSURL(config.JWKSURL, expectedIssuer) || config.TLSCertFile == "" || config.TLSKeyFile == "" || config.CAFile == "" || config.RedisURL == "" {
		return nil, fmt.Errorf("%s principal JWKS mTLS and replay configuration are required", expectedIssuer)
	}
	certificate, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("%s principal JWKS client certificate: %w", expectedIssuer, err)
	}
	caPEM, err := os.ReadFile(config.CAFile)
	if err != nil {
		return nil, fmt.Errorf("%s principal JWKS CA: %w", expectedIssuer, err)
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("%s principal JWKS CA contains no certificates", expectedIssuer)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{certificate}}
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("service principal JWKS redirects are forbidden")
	}}
	fetch := func(ctx context.Context, issuer string) ([]byte, error) {
		if issuer != expectedIssuer {
			return nil, errors.New("untrusted service principal issuer")
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, config.JWKSURL, nil)
		if err != nil {
			return nil, err
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, err
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("service JWKS returned status %d", response.StatusCode)
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, 128*1024+1))
		if err != nil || len(body) > 128*1024 {
			return nil, errors.New("service JWKS response is invalid")
		}
		return body, nil
	}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{Fetch: fetch, RefreshAfter: 20 * time.Second, HardExpiry: 60 * time.Second, UnknownKIDCooldown: 5 * time.Second})
	if err != nil {
		return nil, err
	}
	options, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		return nil, errors.New("service principal replay Redis URL is invalid")
	}
	redisClient := redis.NewClient(options)
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		_ = redisClient.Close()
		return nil, errors.New("service principal replay Redis unavailable")
	}
	return &Runtime{issuer: expectedIssuer, resolver: resolver, redis: redisClient, http: client}, nil
}

func validPrincipalJWKSURL(raw, issuer string) bool {
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	wantPath := "/.well-known/principal-jwks.json"
	if issuer == "gameintegration" {
		wantPath = "/internal/v1/principal/jwks.json"
	} else if issuer == "space" {
		wantPath = "/.well-known/jwks.json"
	}
	return u.Path == wantPath && u.RawPath == ""
}

func (r *Runtime) Verify(ctx context.Context, token, method, requestID, hash string) (principal.Principal, error) {
	if r == nil || r.resolver == nil || r.redis == nil {
		return principal.Principal{}, errors.New("service principal runtime unavailable")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return principal.Principal{}, errors.New("invalid service principal")
	}
	var hint struct {
		Issuer string `json:"iss"`
	}
	payload, err := decodeBase64URL(parts[1])
	if err != nil || json.Unmarshal(payload, &hint) != nil || hint.Issuer != r.issuer {
		return principal.Principal{}, errors.New("untrusted service principal issuer")
	}
	return principal.VerifyService(ctx, token, principal.VerifyConfig{
		ExpectedIssuer: r.issuer, ExpectedAudience: "messaging", ExpectedRPC: method,
		ExpectedRequestID: requestID, ExpectedRequestHash: hash, KeyResolver: r.resolver.Resolve,
		ReplayGuard: r.recordReplay,
	})
}

func decodeBase64URL(value string) ([]byte, error) {
	if strings.Contains(value, "=") {
		return nil, errors.New("invalid base64url")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != value {
		return nil, errors.New("invalid base64url")
	}
	return decoded, nil
}

func (r *Runtime) recordReplay(ctx context.Context, issuer, jwtID string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid service principal replay lifetime")
	}
	digest := sha256.Sum256([]byte(issuer + "\x00" + jwtID))
	result, err := r.redis.SetArgs(ctx, fmt.Sprintf("messaging:%s-principal:%x", r.issuer, digest), "1", redis.SetArgs{Mode: "NX", TTL: ttl}).Result()
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
	if r.redis != nil {
		return r.redis.Close()
	}
	return nil
}
