package principalruntime

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
	"google.golang.org/grpc/credentials"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/principalgrpc"
)

const dependencyTimeout = 2 * time.Second

// Runtime is the fail-closed verifier for the dedicated Space ownership listener.
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
	cert, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("principal listener TLS: %w", err)
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("principal JWKS trust: %w", err)
	}
	if config.JWKSCAFile != "" {
		pem, err := os.ReadFile(config.JWKSCAFile)
		if err != nil {
			return nil, fmt.Errorf("principal JWKS CA: %w", err)
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("principal JWKS CA has no certificates")
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	client := &http.Client{Transport: transport, Timeout: dependencyTimeout, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("principal JWKS redirects are forbidden")
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
			return nil, jwksTransportError(err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
				return nil, principalgrpc.Unavailable(errors.New("principal JWKS unavailable"))
			}
			return nil, errors.New("principal JWKS unavailable")
		}
		document, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
		if err != nil {
			return nil, jwksTransportError(err)
		}
		if len(document) > 65536 {
			return nil, errors.New("invalid principal JWKS response")
		}
		keys, err := principal.ParseJWKS(document)
		if err != nil || len(keys) != 2 {
			return nil, errors.New("principal JWKS requires complete current and next keys")
		}
		var first *rsa.PublicKey
		for kid, key := range keys {
			if !issuerPattern.MatchString(kid) || key.N.BitLen() < 2048 {
				return nil, errors.New("invalid principal signing key")
			}
			if first != nil && first.N.Cmp(key.N) == 0 {
				return nil, errors.New("principal rotation keys must be distinct")
			}
			first = key
		}
		return document, nil
	}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{Fetch: fetch, RefreshAfter: config.RefreshAfter, HardExpiry: config.HardExpiry, UnknownKIDCooldown: config.UnknownKIDCooldown})
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	replay := redis.NewClient(&redis.Options{Addr: config.ReplayAddr, Password: config.ReplayPassword, DialTimeout: dependencyTimeout, ReadTimeout: dependencyTimeout, WriteTimeout: dependencyTimeout, MaxRetries: -1, ContextTimeoutEnabled: true})
	runtime := &Runtime{resolver: resolver, replay: replay, transport: transport, credentials: credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}})}
	startup, cancel := context.WithTimeout(ctx, dependencyTimeout)
	defer cancel()
	if err := replay.Ping(startup).Err(); err != nil {
		_ = runtime.Close()
		return nil, errors.New("principal replay Redis unavailable")
	}
	refreshContext, stopRefresh := context.WithCancel(ctx)
	runtime.cancel = stopRefresh
	runtime.done = make(chan struct{})
	go runtime.refreshLoop(refreshContext, config.RefreshAfter)
	return runtime, nil
}

func jwksTransportError(err error) error {
	var certificateError *tls.CertificateVerificationError
	var protocolError tls.RecordHeaderError
	if errors.As(err, &certificateError) || errors.As(err, &protocolError) {
		return err
	}
	var networkError *net.OpError
	if errors.As(err, &networkError) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return principalgrpc.Unavailable(errors.New("principal JWKS unavailable"))
	}
	return err
}

func isOwnershipMethod(method string) bool {
	return method == rolev1.RoleService_ApplyOwnershipTransfer_FullMethodName || method == rolev1.RoleService_CompensateOwnershipTransfer_FullMethodName
}

func (r *Runtime) Verify(ctx context.Context, token, method, requestID, hash string) (principal.Principal, error) {
	if r == nil || r.resolver == nil || r.replay == nil {
		return principal.Principal{}, principalgrpc.Unavailable(errors.New("principal runtime unavailable"))
	}
	if !isOwnershipMethod(method) {
		return principal.Principal{}, errors.New("principal method is not allowed")
	}
	verified, err := principal.VerifyService(ctx, token, principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "role", ExpectedRPC: method, ExpectedRequestID: requestID, ExpectedRequestHash: hash, KeyResolver: r.resolver.Resolve, ReplayGuard: r.recordReplay})
	if err != nil {
		return principal.Principal{}, err
	}
	if verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return principal.Principal{}, errors.New("service principal contains user authority")
	}
	return verified, nil
}

func (r *Runtime) recordReplay(ctx context.Context, issuer, jwtID string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	// A valid 30-second credential may have iat up to five seconds ahead.
	// Bound before rounding, and use relative TTL so Redis clock skew cannot
	// expire replay protection before this verifier stops accepting the JWT.
	if ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid principal replay lifetime")
	}
	ttl = ((ttl + time.Millisecond - 1) / time.Millisecond) * time.Millisecond
	digest := sha256.Sum256([]byte(issuer + "\x00" + jwtID))
	key := fmt.Sprintf("role:principal:replay:%x", digest)
	result, err := r.replay.SetArgs(ctx, key, "1", redis.SetArgs{Mode: "NX", TTL: ttl}).Result()
	if errors.Is(err, redis.Nil) {
		return errors.New("principal replay detected")
	}
	if err != nil {
		return principalgrpc.Unavailable(errors.New("principal replay Redis unavailable"))
	}
	if result != "OK" {
		return errors.New("principal replay was not recorded")
	}
	return nil
}

// Remote JWKS is intentionally lazy: Space startup depends on Role process
// health. Requests without a complete trusted set still fail closed.
func (r *Runtime) refreshLoop(ctx context.Context, interval time.Duration) {
	defer close(r.done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// A failed refresh retains last-good only until the resolver's hard expiry.
			_ = r.resolver.Refresh(ctx, "space")
		}
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
