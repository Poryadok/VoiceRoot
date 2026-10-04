package matchsquadmemberprincipal

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
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

const (
	issuer          = "gateway"
	audience        = "voice"
	dependencyTTL   = 2 * time.Second
	replayKeyPrefix = "voice:match_squad:member:replay:"
	epochKeyPrefix  = "auth:session:min_epoch:"
)

type Runtime struct {
	resolver    *principal.JWKSResolver
	redis       *redis.Client
	transport   *http.Transport
	credentials credentials.TransportCredentials
	cancel      context.CancelFunc
	done        chan struct{}
	once        sync.Once
	closeErr    error
}

func New(ctx context.Context, cfg Config) (*Runtime, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, errors.New("Voice MatchSquad member TLS identity unavailable")
	}
	clientPEM, err := os.ReadFile(cfg.ClientCAFile)
	if err != nil {
		return nil, errors.New("Voice MatchSquad member client CA unavailable")
	}
	clientRoots := x509.NewCertPool()
	if !clientRoots.AppendCertsFromPEM(clientPEM) {
		return nil, errors.New("Voice MatchSquad member client CA is invalid")
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		return nil, errors.New("system CA roots unavailable for delegated user JWKS")
	}
	caPEM, err := os.ReadFile(cfg.JWKSCAFile)
	if err != nil || !roots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("Voice MatchSquad member JWKS CA is invalid")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	client := &http.Client{Transport: transport, Timeout: dependencyTTL, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("delegated user JWKS redirects are forbidden")
	}}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{
		Fetch: func(ctx context.Context, requestedIssuer string) ([]byte, error) {
			if requestedIssuer != issuer {
				return nil, errors.New("untrusted delegated user issuer")
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.JWKSURL, nil)
			if err != nil {
				return nil, err
			}
			resp, err := client.Do(req)
			if err != nil {
				return nil, errors.New("delegated user JWKS unavailable")
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusOK {
				return nil, errors.New("delegated user JWKS unavailable")
			}
			body, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
			if err != nil || len(body) > 65536 {
				return nil, errors.New("delegated user JWKS unavailable")
			}
			keys, err := principal.ParseJWKS(body)
			if err != nil || len(keys) == 0 {
				return nil, errors.New("delegated user JWKS invalid")
			}
			for _, key := range keys {
				if key.N.BitLen() < 2048 {
					return nil, errors.New("delegated user JWKS key is too small")
				}
			}
			return body, nil
		}, RefreshAfter: 20 * time.Second, HardExpiry: time.Minute, UnknownKIDCooldown: 5 * time.Second,
	})
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DialTimeout: dependencyTTL, ReadTimeout: dependencyTTL, WriteTimeout: dependencyTTL, MaxRetries: -1, ContextTimeoutEnabled: true})
	runtime := &Runtime{resolver: resolver, redis: rdb, transport: transport, credentials: credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientCAs: clientRoots, ClientAuth: tls.RequireAndVerifyClientCert}), done: make(chan struct{})}
	startup, cancel := context.WithTimeout(ctx, dependencyTTL)
	defer cancel()
	if err := rdb.Ping(startup).Err(); err != nil {
		_ = runtime.Close()
		return nil, errors.New("Voice MatchSquad member Redis unavailable")
	}
	if err := resolver.Refresh(startup, issuer); err != nil {
		_ = runtime.Close()
		return nil, errors.New("Voice delegated user JWKS unavailable")
	}
	refreshCtx, stop := context.WithCancel(ctx)
	runtime.cancel = stop
	go runtime.refreshLoop(refreshCtx)
	return runtime, nil
}

func (r *Runtime) Verify(ctx context.Context, token, method, requestID, requestHash string) (principal.Principal, error) {
	if r == nil || r.resolver == nil || r.redis == nil {
		return principal.Principal{}, status.Error(codes.Unavailable, "Voice MatchSquad member verifier unavailable")
	}
	verified, err := principal.VerifyDelegatedUser(ctx, token, principal.VerifyConfig{
		ExpectedIssuer: issuer, ExpectedAudience: audience, ExpectedRPC: method, ExpectedRequestID: requestID, ExpectedRequestHash: requestHash,
		KeyResolver: r.resolveKey, ReplayGuard: r.recordReplay, SessionEpochChecker: r.checkEpoch,
	})
	if err != nil {
		if errors.Is(err, errDependencyUnavailable) {
			return principal.Principal{}, status.Error(codes.Unavailable, "Voice MatchSquad member verifier unavailable")
		}
		return principal.Principal{}, status.Error(codes.Unauthenticated, "invalid delegated user principal")
	}
	return verified, nil
}

func (r *Runtime) resolveKey(ctx context.Context, requestedIssuer, kid string) (*rsa.PublicKey, error) {
	if r == nil || r.resolver == nil || requestedIssuer != issuer {
		return nil, errDependencyUnavailable
	}
	key, err := r.resolver.Resolve(ctx, requestedIssuer, kid)
	if err != nil {
		return nil, errDependencyUnavailable
	}
	return key, nil
}

func (r *Runtime) recordReplay(ctx context.Context, requestedIssuer, jti string, expires time.Time) error {
	if r == nil || r.redis == nil || requestedIssuer != issuer || strings.TrimSpace(jti) == "" {
		return errDependencyUnavailable
	}
	ttl := time.Until(expires)
	if ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid delegated user replay expiry")
	}
	digest := sha256.Sum256([]byte(requestedIssuer + "\x00" + jti))
	callCtx, cancel := context.WithTimeout(ctx, dependencyTTL)
	defer cancel()
	accepted, err := r.redis.SetNX(callCtx, replayKeyPrefix+hex.EncodeToString(digest[:]), "1", ttl+5*time.Second).Result()
	if err != nil {
		return errDependencyUnavailable
	}
	if !accepted {
		return errors.New("delegated user principal replay detected")
	}
	return nil
}

func (r *Runtime) checkEpoch(ctx context.Context, account string, claimed int64) error {
	if r == nil || r.redis == nil || !canonicalUUID(account) || claimed <= 0 {
		return errors.New("invalid delegated user session epoch")
	}
	callCtx, cancel := context.WithTimeout(ctx, dependencyTTL)
	defer cancel()
	value, err := r.redis.Get(callCtx, epochKeyPrefix+account).Result()
	if err != nil {
		return errDependencyUnavailable
	}
	minimum, err := strconv.ParseInt(value, 10, 64)
	if err != nil || minimum <= 0 || strconv.FormatInt(minimum, 10) != value {
		return errDependencyUnavailable
	}
	if claimed < minimum {
		return errors.New("delegated user session epoch revoked")
	}
	return nil
}

// CheckCurrent repeats the time and Auth minimum-epoch checks after a handler
// has waited on database or media dependencies, immediately before it commits
// an admission decision or returns a short-lived grant.
func (r *Runtime) CheckCurrent(ctx context.Context, actor principal.Principal, now time.Time) error {
	if actor.Kind != "delegated_user" || actor.Issuer != issuer || actor.Audience != audience || actor.ExpiresAt.IsZero() || !now.Before(actor.ExpiresAt) {
		return status.Error(codes.Unauthenticated, "delegated user credential expired")
	}
	if err := r.checkEpoch(ctx, actor.AccountID, actor.SessionEpoch); err != nil {
		if errors.Is(err, errDependencyUnavailable) {
			return status.Error(codes.Unavailable, "Voice MatchSquad member verifier unavailable")
		}
		return status.Error(codes.Unauthenticated, "delegated user session is no longer current")
	}
	return nil
}

func (r *Runtime) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	defer close(r.done)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = r.resolver.Refresh(ctx, issuer)
		}
	}
}

func (r *Runtime) ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.Creds(r.credentials), grpc.ChainUnaryInterceptor(StrictUnaryInterceptor(r))}
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	r.once.Do(func() {
		if r.cancel != nil {
			r.cancel()
			<-r.done
		}
		if r.transport != nil {
			r.transport.CloseIdleConnections()
		}
		if r.redis != nil {
			r.closeErr = r.redis.Close()
		}
	})
	return r.closeErr
}

var errDependencyUnavailable = errors.New("MatchSquad member authentication dependency unavailable")
