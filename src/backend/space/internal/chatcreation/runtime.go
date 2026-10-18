package chatcreation

import (
	"context"
	"crypto/rsa"
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
	"google.golang.org/grpc/credentials"
	"voice/backend/pkg/principal"
)

var ErrDependencyUnavailable = errors.New("composite authentication dependency unavailable")

type Config struct {
	Listen, CertFile, KeyFile, ClientCAFile, JWKSURL, JWKSCAFile, RedisAddr, RedisPassword string
	JWKSClientCertFile, JWKSClientKeyFile                                                  string
}

func FromEnv() (Config, bool, error) {
	c := Config{Listen: os.Getenv("SPACE_CHAT_COMPOSITE_GRPC_LISTEN"), CertFile: os.Getenv("SPACE_CHAT_COMPOSITE_TLS_CERT_FILE"), KeyFile: os.Getenv("SPACE_CHAT_COMPOSITE_TLS_KEY_FILE"), ClientCAFile: os.Getenv("SPACE_CHAT_COMPOSITE_CLIENT_CA_FILE"), JWKSURL: os.Getenv("SPACE_CHAT_COMPOSITE_PRINCIPAL_JWKS_URL"), JWKSCAFile: os.Getenv("SPACE_CHAT_COMPOSITE_PRINCIPAL_JWKS_CA_FILE"), RedisAddr: os.Getenv("SPACE_CHAT_COMPOSITE_PRINCIPAL_REDIS_ADDR"), RedisPassword: os.Getenv("SPACE_CHAT_COMPOSITE_PRINCIPAL_REDIS_PASSWORD")}
	c.JWKSClientCertFile = os.Getenv("SPACE_CHAT_COMPOSITE_PRINCIPAL_JWKS_CLIENT_CERT_FILE")
	c.JWKSClientKeyFile = os.Getenv("SPACE_CHAT_COMPOSITE_PRINCIPAL_JWKS_CLIENT_KEY_FILE")
	configured := false
	for _, value := range []string{c.Listen, c.CertFile, c.KeyFile, c.ClientCAFile, c.JWKSURL, c.JWKSCAFile, c.RedisAddr, c.RedisPassword, c.JWKSClientCertFile, c.JWKSClientKeyFile} {
		if value != "" {
			configured = true
		}
	}
	if !configured {
		return c, false, nil
	}
	return c, true, c.validate()
}

func (c Config) validate() error {
	for _, v := range []string{c.Listen, c.CertFile, c.KeyFile, c.ClientCAFile, c.JWKSURL, c.JWKSCAFile, c.RedisAddr, c.JWKSClientCertFile, c.JWKSClientKeyFile} {
		if strings.TrimSpace(v) == "" || v != strings.TrimSpace(v) {
			return errors.New("space Chat composite configuration is incomplete")
		}
	}
	u, err := url.Parse(c.JWKSURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
		return errors.New("chat JWKS must be an exact HTTPS endpoint")
	}
	return nil
}

type Runtime struct {
	verifier    ServiceVerifier
	client      *redis.Client
	transport   *http.Transport
	credentials credentials.TransportCredentials
	once        sync.Once
	closeErr    error
}

func rootsFromFile(path string) (*x509.CertPool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(b) {
		return nil, errors.New("configured CA contains no certificates")
	}
	return roots, nil
}

func New(ctx context.Context, c Config) (*Runtime, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("space composite server identity: %w", err)
	}
	clients, err := rootsFromFile(c.ClientCAFile)
	if err != nil {
		return nil, fmt.Errorf("space composite client CA: %w", err)
	}
	roots, err := rootsFromFile(c.JWKSCAFile)
	if err != nil {
		return nil, fmt.Errorf("chat JWKS CA: %w", err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	jwksIdentity, err := tls.LoadX509KeyPair(c.JWKSClientCertFile, c.JWKSClientKeyFile)
	if err != nil {
		return nil, fmt.Errorf("chat JWKS client identity: %w", err)
	}
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{jwksIdentity}}
	httpClient := &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("chat JWKS redirects forbidden") }}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{Fetch: func(ctx context.Context, issuer string) ([]byte, error) {
		if issuer != "chat" {
			return nil, errors.New("untrusted composite issuer")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.JWKSURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%w: JWKS transport", ErrDependencyUnavailable)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%w: JWKS response", ErrDependencyUnavailable)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
		if err != nil || len(body) > 65536 {
			return nil, ErrDependencyUnavailable
		}
		keys, err := principal.ParseJWKS(body)
		if err != nil || len(keys) != 2 {
			return nil, ErrDependencyUnavailable
		}
		var first *rsa.PublicKey
		for _, k := range keys {
			if k.N.BitLen() < 2048 || first != nil && first.N.Cmp(k.N) == 0 {
				return nil, ErrDependencyUnavailable
			}
			first = k
		}
		return body, nil
	}, RefreshAfter: 20 * time.Second, HardExpiry: 60 * time.Second, UnknownKIDCooldown: 5 * time.Second})
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	client := redis.NewClient(&redis.Options{Addr: c.RedisAddr, Password: c.RedisPassword, DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second, MaxRetries: -1})
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		transport.CloseIdleConnections()
		return nil, ErrDependencyUnavailable
	}
	r := &Runtime{client: client, transport: transport, credentials: credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientCAs: clients, ClientAuth: tls.RequireAndVerifyClientCert})}
	r.verifier = ServiceVerifier{Keys: resolver.Resolve, Replay: r.recordReplay}
	// Gateway starts after Space. Until its HTTPS JWKS becomes available,
	// requests fail closed; no signing keys are copied into Space to bypass the cycle.
	return r, nil
}

func (r *Runtime) recordReplay(ctx context.Context, issuer, jti string, expiry time.Time) error {
	if issuer != "chat" || jti == "" {
		return errors.New("invalid replay binding")
	}
	ttl := time.Until(expiry)
	if ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid replay expiry")
	}
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	accepted, err := r.client.SetNX(callCtx, "principal:space:chat-composite:"+issuer+":"+jti, "1", ttl+5*time.Second).Result()
	if err != nil {
		return ErrDependencyUnavailable
	}
	if !accepted {
		return errors.New("replayed composite credential")
	}
	return nil
}

func (r *Runtime) ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.Creds(r.credentials), grpc.ChainUnaryInterceptor(r.verifier.Unary())}
}
func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	r.once.Do(func() { r.transport.CloseIdleConnections(); r.closeErr = r.client.Close() })
	return r.closeErr
}
