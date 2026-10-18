package intakeprincipal

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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"voice/backend/pkg/principal"
)

type Config struct{ Listen, CertFile, KeyFile, ClientCAFile, JWKSURL, JWKSCAFile, ReplayAddr, ReplayPassword, EpochAddr, EpochPassword string }

func FromEnv() (Config, bool, error) {
	names := []string{"GRPC_LISTEN", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE", "GATEWAY_JWKS_URL", "GATEWAY_JWKS_CA_FILE", "REPLAY_REDIS_ADDR", "REPLAY_REDIS_PASSWORD", "EPOCH_REDIS_ADDR", "EPOCH_REDIS_PASSWORD"}
	values := make([]string, len(names))
	enabled := false
	for i, name := range names {
		values[i] = os.Getenv("MODERATION_INTAKE_" + name)
		if _, ok := os.LookupEnv("MODERATION_INTAKE_" + name); ok {
			enabled = true
		}
	}
	c := Config{values[0], values[1], values[2], values[3], values[4], values[5], values[6], values[7], values[8], values[9]}
	if !enabled {
		return c, false, nil
	}
	return c, true, c.validate()
}
func (c Config) validate() error {
	for _, value := range []string{c.Listen, c.CertFile, c.KeyFile, c.ClientCAFile, c.JWKSURL, c.JWKSCAFile, c.ReplayAddr, c.EpochAddr} {
		if value == "" || value != strings.TrimSpace(value) {
			return errors.New("report intake configuration incomplete")
		}
	}
	if _, _, err := net.SplitHostPort(c.Listen); err != nil {
		return errors.New("invalid intake listener")
	}
	u, err := url.Parse(c.JWKSURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
		return errors.New("exact HTTPS Gateway JWKS required")
	}
	return nil
}

type Runtime struct {
	verifier      Verifier
	replay, epoch *redis.Client
	transport     *http.Transport
	credentials   credentials.TransportCredentials
	once          sync.Once
	closeErr      error
}

func roots(path string) (*x509.CertPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, errors.New("configured CA contains no certificates")
	}
	return pool, nil
}
func New(ctx context.Context, c Config) (*Runtime, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	certificate, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, err
	}
	clients, err := roots(c.ClientCAFile)
	if err != nil {
		return nil, err
	}
	jwksRoots, err := roots(c.JWKSCAFile)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: jwksRoots}
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("Gateway JWKS redirects forbidden") }}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{Fetch: func(ctx context.Context, issuer string) ([]byte, error) {
		if issuer != "gateway" {
			return nil, errors.New("untrusted intake issuer")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.JWKSURL, nil)
		if err != nil {
			return nil, err
		}
		response, err := client.Do(req)
		if err != nil {
			return nil, ErrDependencyUnavailable
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, ErrDependencyUnavailable
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, 65537))
		if err != nil || len(body) > 65536 {
			return nil, ErrDependencyUnavailable
		}
		keys, err := principal.ParseJWKS(body)
		if err != nil || len(keys) != 2 {
			return nil, ErrDependencyUnavailable
		}
		var first *rsa.PublicKey
		for _, key := range keys {
			if key.N.BitLen() < 2048 || first != nil && first.N.Cmp(key.N) == 0 {
				return nil, ErrDependencyUnavailable
			}
			first = key
		}
		return body, nil
	}, RefreshAfter: 20 * time.Second, HardExpiry: 60 * time.Second, UnknownKIDCooldown: 5 * time.Second})
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	redisClient := func(addr, password string) *redis.Client {
		return redis.NewClient(&redis.Options{Addr: addr, Password: password, DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second, MaxRetries: -1, ContextTimeoutEnabled: true})
	}
	r := &Runtime{replay: redisClient(c.ReplayAddr, c.ReplayPassword), epoch: redisClient(c.EpochAddr, c.EpochPassword), transport: transport, credentials: credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}, ClientCAs: clients, ClientAuth: tls.RequireAndVerifyClientCert})}
	startup, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if r.replay.Ping(startup).Err() != nil || r.epoch.Ping(startup).Err() != nil {
		_ = r.Close()
		return nil, ErrDependencyUnavailable
	}
	r.verifier = Verifier{Keys: resolver.Resolve, Replay: r.recordReplay, Epoch: r.checkEpoch}
	// Lazy JWKS fetch avoids the Gateway/Moderation startup cycle. Every request
	// still requires fresh-enough owner keys and both Redis authorities.
	return r, nil
}
func (r *Runtime) recordReplay(ctx context.Context, issuer, jti string, expires time.Time) error {
	ttl := time.Until(expires)
	if issuer != "gateway" || jti == "" || ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid report replay binding")
	}
	call, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	digest := sha256.Sum256([]byte(issuer + "\x00" + jti))
	accepted, err := r.replay.SetNX(call, fmt.Sprintf("moderation:intake:replay:%x", digest), "1", ttl+5*time.Second).Result()
	if err != nil {
		return ErrDependencyUnavailable
	}
	if !accepted {
		return errors.New("replayed report credential")
	}
	return nil
}
func (r *Runtime) checkEpoch(ctx context.Context, account string, epoch int64) error {
	if !canonicalID(account) || epoch <= 0 {
		return errors.New("invalid session epoch")
	}
	call, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	raw, err := r.epoch.Get(call, "auth:session:min_epoch:"+account).Result()
	if err != nil {
		return ErrDependencyUnavailable
	}
	minimum, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || minimum <= 0 || strconv.FormatInt(minimum, 10) != raw {
		return ErrDependencyUnavailable
	}
	if epoch < minimum {
		return errors.New("revoked report session")
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
	r.once.Do(func() {
		r.transport.CloseIdleConnections()
		r.closeErr = errors.Join(r.replay.Close(), r.epoch.Close())
	})
	return r.closeErr
}
