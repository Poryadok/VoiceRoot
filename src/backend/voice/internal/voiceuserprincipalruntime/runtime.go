package voiceuserprincipalruntime

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
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/principalgrpc"
)

const dependencyTimeout = 2 * time.Second

type SessionEpochChecker interface {
	RequireCurrent(context.Context, string, int64) error
}

type Runtime struct {
	resolver    *principal.JWKSResolver
	replay      *redis.Client
	transport   *http.Transport
	credentials credentials.TransportCredentials
	epochs      SessionEpochChecker
	cancel      context.CancelFunc
	done        chan struct{}
	closeOnce   sync.Once
	closeErr    error
}

func New(ctx context.Context, config Config, epochs SessionEpochChecker) (*Runtime, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	if epochs == nil {
		return nil, errors.New("Voice user principal requires Auth session epoch lookup")
	}
	serverCert, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
	if err != nil {
		return nil, errors.New("Voice user principal listener TLS unavailable")
	}
	clientCAPEM, err := os.ReadFile(config.ClientCAFile)
	if err != nil {
		return nil, errors.New("Voice user principal client CA unavailable")
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(clientCAPEM) {
		return nil, errors.New("Voice user principal client CA invalid")
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	jwksCA, err := os.ReadFile(config.JWKSCAFile)
	if err != nil || !roots.AppendCertsFromPEM(jwksCA) {
		return nil, errors.New("Gateway principal JWKS CA invalid")
	}
	clientCert, err := tls.LoadX509KeyPair(config.JWKSClientCertFile, config.JWKSClientKeyFile)
	if err != nil {
		return nil, errors.New("Gateway principal JWKS client certificate unavailable")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{clientCert}}
	httpClient := &http.Client{Transport: transport, Timeout: dependencyTimeout, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("Gateway principal JWKS redirects are forbidden")
	}}
	fetch := func(ctx context.Context, issuer string) ([]byte, error) {
		if issuer != "gateway" {
			return nil, errors.New("untrusted principal issuer")
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, config.JWKSURL, nil)
		if err != nil {
			return nil, err
		}
		response, err := httpClient.Do(request)
		if err != nil {
			return nil, principalgrpc.Unavailable(errors.New("Gateway principal JWKS unavailable"))
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, principalgrpc.Unavailable(errors.New("Gateway principal JWKS unavailable"))
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, 65537))
		if err != nil || len(body) > 65536 {
			return nil, errors.New("invalid Gateway principal JWKS response")
		}
		keys, err := principal.ParseJWKS(body)
		if err != nil || len(keys) != 2 {
			return nil, errors.New("Gateway principal JWKS requires current and next keys")
		}
		var first *rsa.PublicKey
		for _, key := range keys {
			if key.N.BitLen() < 2048 || first != nil && first.N.Cmp(key.N) == 0 {
				return nil, errors.New("invalid Gateway principal rotation keys")
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
		resolver: resolver, replay: replay, transport: transport, epochs: epochs,
		credentials: credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs}),
	}
	startup, cancelStartup := context.WithTimeout(ctx, dependencyTimeout)
	defer cancelStartup()
	if err := replay.Ping(startup).Err(); err != nil {
		_ = runtime.Close()
		return nil, principalgrpc.Unavailable(errors.New("Voice user principal replay Redis unavailable"))
	}
	if err := resolver.Refresh(startup, "gateway"); err != nil {
		_ = runtime.Close()
		return nil, errors.New("initial Gateway principal JWKS unavailable")
	}
	refreshCtx, cancelRefresh := context.WithCancel(ctx)
	runtime.cancel = cancelRefresh
	runtime.done = make(chan struct{})
	go runtime.refreshLoop(refreshCtx, config.RefreshAfter)
	return runtime, nil
}

func (r *Runtime) Verify(ctx context.Context, token, method, requestID, requestHash string) (principal.Principal, error) {
	if r == nil || r.resolver == nil || r.replay == nil || r.epochs == nil {
		return principal.Principal{}, principalgrpc.Unavailable(errors.New("Voice user principal runtime unavailable"))
	}
	if !AllowsMethod(method) {
		return principal.Principal{}, status.Error(codes.PermissionDenied, "method unavailable on Voice user listener")
	}
	verified, err := principal.VerifyDelegatedUser(ctx, token, principal.VerifyConfig{
		ExpectedIssuer: "gateway", ExpectedAudience: "voice", ExpectedRPC: method,
		ExpectedRequestID: requestID, ExpectedRequestHash: requestHash,
		KeyResolver: r.resolver.Resolve, ReplayGuard: r.recordReplay,
		SessionEpochChecker: func(ctx context.Context, accountID string, sessionEpoch int64) error {
			err := r.epochs.RequireCurrent(ctx, accountID, sessionEpoch)
			if status.Code(err) == codes.Unavailable {
				return principalgrpc.Unavailable(err)
			}
			return err
		},
	})
	if err != nil {
		return principal.Principal{}, principalgrpc.VerificationStatus(err)
	}
	return verified, nil
}

func AllowsMethod(method string) bool {
	return method == callsv1.VoiceService_JoinVoiceRoom_FullMethodName ||
		method == callsv1.VoiceService_GetJoinToken_FullMethodName ||
		method == callsv1.VoiceService_LeaveVoiceRoom_FullMethodName
}

func (r *Runtime) recordReplay(ctx context.Context, issuer, jwtID string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if issuer != "gateway" || jwtID == "" || ttl <= 0 || ttl > 35*time.Second {
		return errors.New("invalid delegated principal replay lifetime")
	}
	ttl = ((ttl + time.Millisecond - 1) / time.Millisecond) * time.Millisecond
	digest := sha256.Sum256([]byte(issuer + "\x00" + jwtID))
	result, err := r.replay.SetArgs(ctx, fmt.Sprintf("voice:user-principal:replay:%x", digest), "1", redis.SetArgs{Mode: "NX", TTL: ttl}).Result()
	if errors.Is(err, redis.Nil) {
		return errors.New("delegated principal replay detected")
	}
	if err != nil {
		return principalgrpc.Unavailable(errors.New("Voice user principal replay Redis unavailable"))
	}
	if result != "OK" {
		return errors.New("delegated principal replay was not recorded")
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
			_ = r.resolver.Refresh(ctx, "gateway")
		}
	}
}

func (r *Runtime) Credentials() credentials.TransportCredentials {
	if r == nil {
		return nil
	}
	return r.credentials
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
