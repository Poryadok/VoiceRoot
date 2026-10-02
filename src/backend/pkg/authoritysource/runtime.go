package authoritysource

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
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
	"voice/backend/pkg/principal"
)

type SchemaReader interface {
	Reader
	CheckAuthoritySourceSchema(context.Context) error
}

type Runtime struct {
	server      *Server
	interceptor grpc.UnaryServerInterceptor
	credentials credentials.TransportCredentials
	resolver    *principal.JWKSResolver
	replay      *redis.Client
	transport   *http.Transport
	cancel      context.CancelFunc
	done        chan struct{}
	closeOnce   sync.Once
	closeErr    error
	audience    string
}

const sourceDependencyTimeout = 2 * time.Second

var sourceKeyID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// NewRuntime constructs a dedicated source-only mTLS serving set, after the
// actual owning reader passes preflight. Remote JWKS is lazy to avoid service
// boot cycles, but no request is accepted until a complete trust set resolves.
func NewRuntime(ctx context.Context, cfg RuntimeConfig, reader SchemaReader) (*Runtime, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if reader == nil {
		return nil, errors.New("owning source reader required")
	}
	preflight, stop := context.WithTimeout(ctx, sourceDependencyTimeout)
	defer stop()
	if err := reader.CheckAuthoritySourceSchema(preflight); err != nil {
		return nil, errors.New("source schema preflight failed")
	}
	server, err := NewServer(cfg.Owner, reader)
	if err != nil {
		return nil, err
	}
	certificate, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, errors.New("source TLS identity unavailable")
	}
	clientCA, err := os.ReadFile(cfg.ClientCAFile)
	if err != nil {
		return nil, errors.New("source client CA unavailable")
	}
	clientRoots := x509.NewCertPool()
	if !clientRoots.AppendCertsFromPEM(clientCA) {
		return nil, errors.New("source client CA invalid")
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, errors.New("source JWKS system trust unavailable")
	}
	if cfg.JWKSCAFile != "" {
		pem, err := os.ReadFile(cfg.JWKSCAFile)
		if err != nil || !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("source JWKS CA invalid")
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	client := &http.Client{Transport: transport, Timeout: sourceDependencyTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("source JWKS redirect forbidden") }}
	endpoint := cfg.JWKSURL
	fetch := func(ctx context.Context, issuer string) ([]byte, error) {
		if issuer != "federation" {
			return nil, errors.New("untrusted source issuer")
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, errors.New("invalid source JWKS request")
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, errors.New("source JWKS unavailable")
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK {
			return nil, errors.New("source JWKS unavailable")
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, 65537))
		if err != nil || len(body) > 65536 {
			return nil, errors.New("source JWKS invalid")
		}
		keys, err := principal.ParseJWKS(body)
		if err != nil || len(keys) != 2 {
			return nil, errors.New("source JWKS requires current and next keys")
		}
		var first *rsa.PublicKey
		for id, key := range keys {
			if !sourceKeyID.MatchString(id) || key.N.BitLen() < 2048 || first != nil && first.N.Cmp(key.N) == 0 {
				return nil, errors.New("source rotation keys invalid")
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
	runtime := &Runtime{server: server, resolver: resolver, transport: transport, audience: Audience(cfg.Owner), credentials: credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientRoots})}
	runtime.replay = redis.NewClient(&redis.Options{Addr: cfg.ReplayAddr, Password: cfg.ReplayPassword, DialTimeout: sourceDependencyTimeout, ReadTimeout: sourceDependencyTimeout, WriteTimeout: sourceDependencyTimeout, MaxRetries: -1, ContextTimeoutEnabled: true})
	startup, cancel := context.WithTimeout(ctx, sourceDependencyTimeout)
	defer cancel()
	if runtime.replay.Ping(startup).Err() != nil {
		_ = runtime.Close()
		return nil, errors.New("source replay Redis unavailable")
	}
	runtime.interceptor, err = UnaryInterceptor(cfg.Owner, resolver.Resolve, runtime.recordReplay)
	if err != nil {
		_ = runtime.Close()
		return nil, err
	}
	refresh, stopRefresh := context.WithCancel(ctx)
	runtime.cancel, runtime.done = stopRefresh, make(chan struct{})
	go runtime.refreshLoop(refresh, cfg.RefreshAfter)
	return runtime, nil
}

func (r *Runtime) recordReplay(ctx context.Context, issuer, id string, expires time.Time) error {
	ttl := time.Until(expires)
	if ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid source replay lifetime")
	}
	ttl = ((ttl + time.Millisecond - 1) / time.Millisecond) * time.Millisecond
	digest := sha256.Sum256([]byte(issuer + "\x00" + id))
	key := fmt.Sprintf("%s:authority-source:replay:%x", r.audience, digest)
	result, err := r.replay.SetArgs(ctx, key, "1", redis.SetArgs{Mode: "NX", TTL: ttl}).Result()
	if err != nil || result != "OK" {
		return errors.New("source replay rejected")
	}
	return nil
}

func (r *Runtime) refreshLoop(ctx context.Context, interval time.Duration) {
	defer close(r.done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = r.resolver.Refresh(ctx, "federation")
		}
	}
}

func (r *Runtime) ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.Creds(r.credentials), grpc.MaxRecvMsgSize(1 << 20), grpc.MaxSendMsgSize(MaxStateBytes + (1 << 20)), grpc.ChainUnaryInterceptor(r.interceptor)}
}

func (r *Runtime) Register(server *grpc.Server) {
	authorityv1.RegisterAuthoritySourceServiceServer(server, r.server)
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
		if r.transport != nil {
			r.transport.CloseIdleConnections()
		}
		if r.replay != nil {
			r.closeErr = r.replay.Close()
		}
	})
	return r.closeErr
}
