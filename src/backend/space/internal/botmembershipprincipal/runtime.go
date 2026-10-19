package botmembershipprincipal

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
	"voice/backend/pkg/principal"
)

type Runtime struct {
	verifier    Verifier
	redis       *redis.Client
	transport   *http.Transport
	credentials credentials.TransportCredentials
	once        sync.Once
	closeErr    error
}

func New(ctx context.Context, c Config) (*Runtime, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	identity, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("botmembership listener identity: %w", err)
	}
	clients, err := botmembershipCertPool(c.ClientCAFile)
	if err != nil {
		return nil, fmt.Errorf("botmembership client CA: %w", err)
	}
	var roots *x509.CertPool
	if c.JWKSCAFile != "" {
		roots, err = botmembershipCertPool(c.JWKSCAFile)
		if err != nil {
			return nil, fmt.Errorf("botmembership JWKS CA: %w", err)
		}
	}
	clientTLS := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	if c.JWKSClientCertFile != "" {
		pair, err := tls.LoadX509KeyPair(c.JWKSClientCertFile, c.JWKSClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("botmembership JWKS identity: %w", err)
		}
		clientTLS.Certificates = []tls.Certificate{pair}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = clientTLS
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("botmembership JWKS redirects forbidden")
	}}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{
		RefreshAfter: c.RefreshAfter, HardExpiry: c.HardExpiry, UnknownKIDCooldown: c.UnknownKIDCooldown,
		Fetch: func(ctx context.Context, issuer string) ([]byte, error) {
			endpoint, ok := c.JWKSURLs[issuer]
			if !ok || issuer != "bot" {
				return nil, errors.New("untrusted botmembership issuer")
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
			if err != nil {
				return nil, err
			}
			response, err := client.Do(req)
			if err != nil {
				return nil, &dependencyUnavailable{cause: err}
			}
			defer func() { _ = response.Body.Close() }()
			if response.StatusCode != http.StatusOK {
				return nil, &dependencyUnavailable{cause: errors.New("botmembership JWKS unavailable")}
			}
			document, err := io.ReadAll(io.LimitReader(response.Body, 65537))
			if err != nil || len(document) > 65536 {
				return nil, &dependencyUnavailable{cause: errors.New("invalid botmembership JWKS document")}
			}
			keys, err := principal.ParseJWKS(document)
			if err != nil || len(keys) != 2 {
				return nil, &dependencyUnavailable{cause: errors.New("botmembership JWKS requires current and next keys")}
			}
			var modulus string
			for _, key := range keys {
				if key.N.BitLen() < 2048 || key.N.String() == modulus {
					return nil, &dependencyUnavailable{cause: errors.New("botmembership JWKS keys must be distinct RSA-2048 or stronger")}
				}
				modulus = key.N.String()
			}
			return document, nil
		},
	})
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	r := &Runtime{transport: transport, redis: redis.NewClient(&redis.Options{Addr: c.ReplayAddr, Password: c.ReplayPassword, DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second, MaxRetries: -1}),
		credentials: credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{identity}, ClientCAs: clients, ClientAuth: tls.RequireAndVerifyClientCert})}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := r.redis.Ping(pingCtx).Err(); err != nil {
		_ = r.Close()
		return nil, fmt.Errorf("botmembership replay dependency: %w", err)
	}
	r.verifier = Verifier{Keys: resolver.Resolve, Replay: r.recordReplay}
	return r, nil
}

func botmembershipCertPool(path string) (*x509.CertPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, errors.New("CA file contains no certificates")
	}
	return pool, nil
}

func (r *Runtime) recordReplay(ctx context.Context, issuer, jti string, expires time.Time) error {
	ttl := time.Until(expires)
	if issuer != "bot" || jti == "" || ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid botmembership replay claim")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	accepted, err := r.redis.SetNX(ctx, "principal:space:bot-membership:"+issuer+":"+jti, "1", ttl+5*time.Second).Result()
	if err != nil {
		return &dependencyUnavailable{cause: errors.New("botmembership replay dependency unavailable")}
	}
	if !accepted {
		return errors.New("botmembership credential replay")
	}
	return nil
}

func (r *Runtime) ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.Creds(r.credentials), grpc.ChainUnaryInterceptor(r.verifier.Unary())}
}

func (r *Runtime) Close() error {
	r.once.Do(func() { r.closeErr = r.redis.Close(); r.transport.CloseIdleConnections() })
	return r.closeErr
}
