package spaceprincipalruntime

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
	"os"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/principalgrpc"
)

const dependencyTimeout = 2 * time.Second

type Runtime struct {
	resolver    *principal.JWKSResolver
	replay      *redis.Client
	transport   *http.Transport
	credentials credentials.TransportCredentials
	cancel      context.CancelFunc
	done        chan struct{}
	closeOnce   sync.Once
	closeErr    error
}

func New(ctx context.Context, config Config) (*Runtime, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	serverCert, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("Space principal listener TLS: %w", err)
	}
	clientCAPEM, err := os.ReadFile(config.ClientCAFile)
	if err != nil {
		return nil, fmt.Errorf("Space principal client CA: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(clientCAPEM) {
		return nil, errors.New("Space principal client CA contains no certificates")
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if config.JWKSCAFile != "" {
		ca, readErr := os.ReadFile(config.JWKSCAFile)
		if readErr != nil {
			return nil, fmt.Errorf("Space principal JWKS CA: %w", readErr)
		}
		if !roots.AppendCertsFromPEM(ca) {
			return nil, errors.New("Space principal JWKS CA contains no certificates")
		}
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	client := &http.Client{Transport: transport, Timeout: dependencyTimeout, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("Space principal JWKS redirects are forbidden")
	}}
	endpoint := config.JWKSURLs["space"]
	fetch := func(ctx context.Context, issuer string) ([]byte, error) {
		if issuer != "space" {
			return nil, errors.New("untrusted principal issuer")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, principalJWKSUnavailable(err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, principalgrpc.Unavailable(errors.New("Space principal JWKS unavailable"))
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
		if err != nil || len(body) > 65536 {
			return nil, errors.New("invalid Space principal JWKS response")
		}
		keys, err := principal.ParseJWKS(body)
		if err != nil || len(keys) != 2 {
			return nil, errors.New("Space principal JWKS requires current and next keys")
		}
		var first *rsa.PublicKey
		for kid, key := range keys {
			if !issuerPattern.MatchString(kid) || key.N.BitLen() < 2048 || (first != nil && first.N.Cmp(key.N) == 0) {
				return nil, errors.New("invalid Space principal rotation keys")
			}
			first = key
		}
		return body, nil
	}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{Fetch: fetch, RefreshAfter: config.RefreshAfter, HardExpiry: config.HardExpiry, UnknownKIDCooldown: config.UnknownKIDCooldown})
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	replay := redis.NewClient(&redis.Options{Addr: config.ReplayAddr, Password: config.ReplayPassword, DialTimeout: dependencyTimeout, ReadTimeout: dependencyTimeout, WriteTimeout: dependencyTimeout, MaxRetries: -1, ContextTimeoutEnabled: true})
	runtime := &Runtime{
		resolver: resolver, replay: replay, transport: transport,
		credentials: credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs}),
	}
	startup, cancelStartup := context.WithTimeout(ctx, dependencyTimeout)
	defer cancelStartup()
	if err := replay.Ping(startup).Err(); err != nil {
		_ = runtime.Close()
		return nil, principalgrpc.Unavailable(errors.New("Space principal replay Redis unavailable"))
	}
	if err := resolver.Refresh(startup, "space"); err != nil {
		_ = runtime.Close()
		return nil, fmt.Errorf("load initial Space principal JWKS: %w", err)
	}
	refreshCtx, cancelRefresh := context.WithCancel(ctx)
	runtime.cancel = cancelRefresh
	runtime.done = make(chan struct{})
	go runtime.refreshLoop(refreshCtx, config.RefreshAfter)
	return runtime, nil
}

func principalJWKSUnavailable(err error) error {
	var certificateError *tls.CertificateVerificationError
	var networkError *net.OpError
	if errors.As(err, &certificateError) {
		return err
	}
	if errors.As(err, &networkError) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return principalgrpc.Unavailable(errors.New("Space principal JWKS unavailable"))
	}
	return err
}

func (r *Runtime) Verify(ctx context.Context, token, method, requestID, requestHash string) (principal.Principal, error) {
	if r == nil || r.resolver == nil || r.replay == nil {
		return principal.Principal{}, principalgrpc.Unavailable(errors.New("Space principal runtime unavailable"))
	}
	if !isLifecycleMethod(method) {
		return principal.Principal{}, status.Error(codes.PermissionDenied, "Space principal method is not allowed")
	}
	verified, err := principal.VerifyService(ctx, token, principal.VerifyConfig{
		ExpectedIssuer: "space", ExpectedAudience: "voice", ExpectedRPC: method,
		ExpectedRequestID: requestID, ExpectedRequestHash: requestHash,
		KeyResolver: r.resolver.Resolve, ReplayGuard: r.recordReplay,
	})
	if err != nil {
		return principal.Principal{}, principalgrpc.VerificationStatus(err)
	}
	if verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return principal.Principal{}, errors.New("service principal contains user authority")
	}
	return verified, nil
}

func isLifecycleMethod(method string) bool {
	return method == "/voice.calls.v1.VoiceService/ApplySpaceLifecycleFence" || method == "/voice.calls.v1.VoiceService/PurgeSpace"
}

func (r *Runtime) recordReplay(ctx context.Context, issuer, jwtID string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if issuer != "space" || jwtID == "" || ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid principal replay lifetime")
	}
	ttl = ((ttl + time.Millisecond - 1) / time.Millisecond) * time.Millisecond
	digest := sha256.Sum256([]byte(issuer + "\x00" + jwtID))
	result, err := r.replay.SetArgs(ctx, fmt.Sprintf("voice:space-principal:replay:%x", digest), "1", redis.SetArgs{Mode: "NX", TTL: ttl}).Result()
	if errors.Is(err, redis.Nil) {
		return errors.New("principal replay detected")
	}
	if err != nil {
		return principalgrpc.Unavailable(errors.New("Space principal replay Redis unavailable"))
	}
	if result != "OK" {
		return errors.New("principal replay was not recorded")
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
			_ = r.resolver.Refresh(ctx, "space")
		}
	}
}

func (r *Runtime) Credentials() credentials.TransportCredentials {
	if r == nil {
		return nil
	}
	return r.credentials
}

func (r *Runtime) ServerOptions() []grpc.ServerOption {
	if r == nil || r.credentials == nil {
		return []grpc.ServerOption{grpc.ChainUnaryInterceptor(principalgrpc.StrictUnaryInterceptor(nil))}
	}
	return []grpc.ServerOption{
		grpc.Creds(r.credentials),
		grpc.ChainUnaryInterceptor(principalgrpc.StrictUnaryInterceptor(r)),
	}
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
