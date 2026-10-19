package principalruntime

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"voice/backend/bot/internal/principalgrpc"
	"voice/backend/pkg/principal"
)

type GatewayConfig struct {
	Listen, CertFile, KeyFile, ClientCAFile, JWKSURL, JWKSCAFile string
}

func GatewayConfigFromEnv(getenv func(string) string) (GatewayConfig, bool, error) {
	if getenv == nil {
		return GatewayConfig{}, false, errors.New("environment reader required")
	}
	c := GatewayConfig{getenv("BOT_GATEWAY_LIFECYCLE_GRPC_LISTEN"), getenv("BOT_GATEWAY_LIFECYCLE_TLS_CERT_FILE"), getenv("BOT_GATEWAY_LIFECYCLE_TLS_KEY_FILE"), getenv("BOT_GATEWAY_LIFECYCLE_CLIENT_CA_FILE"), getenv("BOT_GATEWAY_PRINCIPAL_JWKS_URL"), getenv("BOT_GATEWAY_PRINCIPAL_JWKS_CA_FILE")}
	values := []string{c.Listen, c.CertFile, c.KeyFile, c.ClientCAFile, c.JWKSURL, c.JWKSCAFile}
	count := 0
	for _, v := range values {
		if v != "" {
			count++
		}
	}
	if count == 0 {
		return c, false, nil
	}
	if count != len(values) {
		return c, true, errors.New("Bot Gateway lifecycle mTLS and JWKS configuration incomplete")
	}
	for _, v := range values {
		if strings.TrimSpace(v) != v {
			return c, true, errors.New("Bot Gateway lifecycle configuration malformed")
		}
	}
	u, err := url.Parse(c.JWKSURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
		return c, true, errors.New("Gateway principal JWKS requires exact HTTPS endpoint")
	}
	return c, true, nil
}

type GatewayRuntime struct {
	Verifier  principalgrpc.GatewayUserVerifier
	TLS       *tls.Config
	transport *http.Transport
}

func (r *Runtime) NewGatewayRuntime(c GatewayConfig, epoch principal.SessionEpochChecker) (*GatewayRuntime, error) {
	if r == nil || r.redis == nil || epoch == nil {
		return nil, errors.New("Bot Gateway principal authority incomplete")
	}
	certificate, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, err
	}
	readRoots := func(path string) (*x509.CertPool, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(data) {
			return nil, errors.New("principal CA contains no certificates")
		}
		return roots, nil
	}
	clients, err := readRoots(c.ClientCAFile)
	if err != nil {
		return nil, err
	}
	roots, err := readRoots(c.JWKSCAFile)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("principal JWKS redirects forbidden") }}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{Fetch: func(ctx context.Context, issuer string) ([]byte, error) {
		if issuer != "gateway" {
			return nil, errors.New("untrusted initiating issuer")
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.JWKSURL, nil)
		if err != nil {
			return nil, err
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, status.Error(codes.Unavailable, "Gateway JWKS unavailable")
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK {
			return nil, status.Error(codes.Unavailable, "Gateway JWKS unavailable")
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, 128*1024+1))
		if err != nil || len(body) > 128*1024 {
			return nil, status.Error(codes.Unavailable, "Gateway JWKS unavailable")
		}
		return body, nil
	}, RefreshAfter: 20 * time.Second, HardExpiry: 60 * time.Second, UnknownKIDCooldown: 5 * time.Second})
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	replay := func(ctx context.Context, issuer, jti string, expiry time.Time) error {
		if issuer != "gateway" || jti == "" {
			return errors.New("invalid initiating replay binding")
		}
		ttl := time.Until(expiry)
		if ttl <= 0 || ttl > 35*time.Second {
			return errors.New("invalid initiating lifetime")
		}
		callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		accepted, err := r.redis.SetNX(callCtx, "bot:gateway-lifecycle:"+issuer+":"+jti, "1", ttl+5*time.Second).Result()
		if err != nil {
			return status.Error(codes.Unavailable, "Bot initiating replay authority unavailable")
		}
		if !accepted {
			return errors.New("replayed initiating credential")
		}
		return nil
	}
	return &GatewayRuntime{Verifier: principalgrpc.GatewayUserVerifier{Keys: resolver.Resolve, Replay: replay, Epoch: epoch}, TLS: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clients}, transport: transport}, nil
}

func (r *GatewayRuntime) Close() {
	if r != nil && r.transport != nil {
		r.transport.CloseIdleConnections()
	}
}
