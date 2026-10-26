package catalogassets

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
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

type ListenerConfig struct {
	Listen, CertFile, KeyFile, ClientCAFile           string
	GatewayJWKS, MessagingJWKS, FileJWKS              string
	JWKSCAFile, JWKSClientCertFile, JWKSClientKeyFile string
	RedisAddr, RedisPassword                          string
}

func (c ListenerConfig) Validate() error {
	for _, value := range []string{c.Listen, c.CertFile, c.KeyFile, c.ClientCAFile, c.GatewayJWKS, c.MessagingJWKS, c.FileJWKS, c.JWKSCAFile, c.JWKSClientCertFile, c.JWKSClientKeyFile, c.RedisAddr} {
		if value == "" || value != strings.TrimSpace(value) {
			return errors.New("complete catalog listener mTLS, issuer JWKS and replay configuration required")
		}
	}
	if _, port, err := net.SplitHostPort(c.Listen); err != nil || port == "" {
		return errors.New("invalid catalog listener address")
	}
	for _, endpoint := range []string{c.GatewayJWKS, c.MessagingJWKS, c.FileJWKS} {
		u, err := url.ParseRequestURI(endpoint)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.Path == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
			return errors.New("catalog issuer JWKS must be canonical HTTPS endpoints")
		}
	}
	return nil
}

type ListenerRuntime struct {
	verifier    Authenticator
	credentials credentials.TransportCredentials
	redis       *redis.Client
	transport   *http.Transport
	once        sync.Once
	closeErr    error
}

func catalogRoots(path string) (*x509.CertPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("catalog CA unavailable")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(data) {
		return nil, errors.New("catalog CA invalid")
	}
	return roots, nil
}
func NewListenerRuntime(ctx context.Context, c ListenerConfig, epoch principal.SessionEpochChecker) (*ListenerRuntime, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if epoch == nil {
		return nil, errors.New("catalog authoritative Auth checker required")
	}
	cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, errors.New("catalog server identity unavailable")
	}
	clients, err := catalogRoots(c.ClientCAFile)
	if err != nil {
		return nil, err
	}
	roots, err := catalogRoots(c.JWKSCAFile)
	if err != nil {
		return nil, err
	}
	identity, err := tls.LoadX509KeyPair(c.JWKSClientCertFile, c.JWKSClientKeyFile)
	if err != nil {
		return nil, errors.New("catalog JWKS identity unavailable")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{identity}}
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("catalog JWKS redirect forbidden") }}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{Fetch: func(ctx context.Context, issuer string) ([]byte, error) {
		endpoints := map[string]string{"gateway": c.GatewayJWKS, "messaging": c.MessagingJWKS, "file": c.FileJWKS}
		endpoint, ok := endpoints[issuer]
		if !ok {
			return nil, errors.New("untrusted catalog issuer")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, catalogDependencyError()
		}
		response, err := client.Do(req)
		if err != nil {
			return nil, catalogDependencyError()
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK {
			return nil, catalogDependencyError()
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, 65537))
		if err != nil || len(body) > 65536 {
			return nil, catalogDependencyError()
		}
		keys, err := principal.ParseJWKS(body)
		if err != nil || len(keys) != 2 {
			return nil, catalogDependencyError()
		}
		var first *rsa.PublicKey
		for _, key := range keys {
			if key.N.BitLen() < 2048 || first != nil && first.N.Cmp(key.N) == 0 {
				return nil, catalogDependencyError()
			}
			first = key
		}
		return body, nil
	}, RefreshAfter: 20 * time.Second, HardExpiry: 60 * time.Second, UnknownKIDCooldown: 5 * time.Second})
	if err != nil {
		transport.CloseIdleConnections()
		return nil, errors.New("catalog key resolver unavailable")
	}
	replay := redis.NewClient(&redis.Options{Addr: c.RedisAddr, Password: c.RedisPassword, DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second, MaxRetries: -1})
	ping, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := replay.Ping(ping).Err(); err != nil {
		_ = replay.Close()
		transport.CloseIdleConnections()
		return nil, catalogDependencyError()
	}
	r := &ListenerRuntime{redis: replay, transport: transport, credentials: credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clients})}
	r.verifier = Authenticator{Keys: resolver.Resolve, Replay: r.recordReplay, SessionEpoch: epoch}
	return r, nil
}
func catalogDependencyError() error {
	return status.Error(codes.Unavailable, "catalog principal dependency unavailable")
}
func (r *ListenerRuntime) recordReplay(ctx context.Context, issuer, jti string, expiry time.Time) error {
	if (issuer != "gateway" && issuer != "messaging" && issuer != "file") || jti == "" {
		return errors.New("invalid catalog replay binding")
	}
	ttl := time.Until(expiry)
	if ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid catalog replay expiry")
	}
	digest := sha256.Sum256([]byte(issuer + "\x00" + jti))
	call, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ok, err := r.redis.SetNX(call, fmt.Sprintf("principal:chat:catalog-assets:%x", digest), "1", ttl+5*time.Second).Result()
	if err != nil {
		return catalogDependencyError()
	}
	if !ok {
		return errors.New("catalog credential replay")
	}
	return nil
}
func (r *ListenerRuntime) ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.Creds(r.credentials), grpc.ChainUnaryInterceptor(r.verifier.UnaryInterceptor()), grpc.MaxRecvMsgSize(64 * 1024)}
}
func (r *ListenerRuntime) Close() error {
	if r == nil {
		return nil
	}
	r.once.Do(func() { r.transport.CloseIdleConnections(); r.closeErr = r.redis.Close() })
	return r.closeErr
}
