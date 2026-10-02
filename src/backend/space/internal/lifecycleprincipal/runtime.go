package lifecycleprincipal

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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"voice/backend/pkg/principal"
)

var ErrDependencyUnavailable = errors.New("lifecycle authentication dependency unavailable")

type Config struct {
	Listen, CertFile, KeyFile, ClientCAFile, JWKSURL, JWKSCAFile, RedisAddr, RedisPassword string
}

func FromEnv() (Config, bool, error) {
	c := Config{Listen: os.Getenv("SPACE_GATEWAY_LIFECYCLE_GRPC_LISTEN"), CertFile: os.Getenv("SPACE_GATEWAY_LIFECYCLE_TLS_CERT_FILE"), KeyFile: os.Getenv("SPACE_GATEWAY_LIFECYCLE_TLS_KEY_FILE"), ClientCAFile: os.Getenv("SPACE_GATEWAY_LIFECYCLE_CLIENT_CA_FILE"), JWKSURL: os.Getenv("SPACE_GATEWAY_PRINCIPAL_JWKS_URL"), JWKSCAFile: os.Getenv("SPACE_GATEWAY_PRINCIPAL_JWKS_CA_FILE"), RedisAddr: os.Getenv("SPACE_GATEWAY_PRINCIPAL_REDIS_ADDR"), RedisPassword: os.Getenv("SPACE_GATEWAY_PRINCIPAL_REDIS_PASSWORD")}
	configured := false
	for _, value := range []string{c.Listen, c.CertFile, c.KeyFile, c.ClientCAFile, c.JWKSURL, c.JWKSCAFile, c.RedisAddr, c.RedisPassword} {
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
	for _, v := range []string{c.Listen, c.CertFile, c.KeyFile, c.ClientCAFile, c.JWKSURL, c.JWKSCAFile, c.RedisAddr} {
		if strings.TrimSpace(v) == "" || v != strings.TrimSpace(v) {
			return errors.New("Space Gateway lifecycle configuration is incomplete")
		}
	}
	u, err := url.Parse(c.JWKSURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
		return errors.New("Gateway JWKS must be an exact HTTPS endpoint")
	}
	return nil
}

type Runtime struct {
	verifier    Verifier
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
		return nil, fmt.Errorf("Space lifecycle server identity: %w", err)
	}
	clients, err := rootsFromFile(c.ClientCAFile)
	if err != nil {
		return nil, fmt.Errorf("Space lifecycle client CA: %w", err)
	}
	roots, err := rootsFromFile(c.JWKSCAFile)
	if err != nil {
		return nil, fmt.Errorf("Gateway JWKS CA: %w", err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	httpClient := &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("Gateway JWKS redirects forbidden") }}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{Fetch: func(ctx context.Context, issuer string) ([]byte, error) {
		if issuer != "gateway" {
			return nil, errors.New("untrusted lifecycle issuer")
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
	r.verifier = Verifier{Keys: resolver.Resolve, Replay: r.recordReplay, Epoch: r.checkEpoch}
	// Gateway starts after Space. Until its HTTPS JWKS becomes available,
	// requests fail closed; no signing keys are copied into Space to bypass the cycle.
	return r, nil
}

func (r *Runtime) recordReplay(ctx context.Context, issuer, jti string, expiry time.Time) error {
	if issuer != "gateway" || jti == "" {
		return errors.New("invalid replay binding")
	}
	ttl := time.Until(expiry)
	if ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid replay expiry")
	}
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	accepted, err := r.client.SetNX(callCtx, "principal:space:gateway-lifecycle:"+issuer+":"+jti, "1", ttl+5*time.Second).Result()
	if err != nil {
		return ErrDependencyUnavailable
	}
	if !accepted {
		return errors.New("replayed lifecycle credential")
	}
	return nil
}

func (r *Runtime) checkEpoch(ctx context.Context, account string, epoch int64) error {
	if !canonicalID(account) || epoch <= 0 {
		return errors.New("invalid session epoch")
	}
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	value, err := r.client.Get(callCtx, "auth:session:min_epoch:"+account).Result()
	if err != nil {
		return ErrDependencyUnavailable
	}
	minimum, err := strconv.ParseInt(value, 10, 64)
	if err != nil || minimum <= 0 || strconv.FormatInt(minimum, 10) != value {
		return ErrDependencyUnavailable
	}
	if epoch < minimum {
		return errors.New("revoked lifecycle session")
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
