package principalruntime

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/peer"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
	"voice/backend/pkg/principal"
)

const messagingSendAuthorityMethod = "/voice.bot.v1.BotService/CheckBotMessageSendAuthority"

func (c Config) validateMessagingAuthority() error {
	count := 0
	for _, v := range []string{c.MessagingJWKSURL, c.MessagingJWKSCAFile, c.MessagingJWKSClientCert, c.MessagingJWKSClientKey, c.MessagingURISAN} {
		if v != "" {
			count++
		}
	}
	if count == 0 {
		return nil
	}
	uri, uriErr := url.Parse(c.MessagingURISAN)
	if uriErr != nil || uri.Scheme != "spiffe" || uri.Host == "" || uri.User != nil || uri.RawQuery != "" || uri.Fragment != "" {
		return errors.New("exact Messaging URI SAN required")
	}
	u, err := url.Parse(c.MessagingJWKSURL)
	if count != 5 || !c.SpaceLifecycleEnabled() || err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || u.Path != "/.well-known/principal-jwks.json" {
		return errors.New("complete exact Messaging authority HTTPS and mTLS settings required")
	}
	return nil
}

func (r *Runtime) initMessagingAuthority() error {
	c := r.config
	if err := c.validateMessagingAuthority(); err != nil {
		return err
	}
	if c.MessagingJWKSURL == "" {
		return nil
	}
	cert, err := tls.LoadX509KeyPair(c.MessagingJWKSClientCert, c.MessagingJWKSClientKey)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(c.MessagingJWKSCAFile)
	if err != nil {
		return err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(raw) {
		return errors.New("messaging JWKS CA is empty")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{cert}}
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("messaging JWKS redirect denied") }}
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{Fetch: func(ctx context.Context, issuer string) ([]byte, error) {
		if issuer != "messaging" {
			return nil, errors.New("messaging issuer required")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.MessagingJWKSURL, nil)
		if err != nil {
			return nil, err
		}
		response, err := client.Do(req)
		if err != nil {
			return nil, status.Error(codes.Unavailable, "Messaging JWKS unavailable")
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK {
			return nil, status.Error(codes.Unavailable, fmt.Sprintf("Messaging JWKS status %d", response.StatusCode))
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, 128*1024+1))
		if err != nil || len(body) > 128*1024 {
			return nil, errors.New("invalid Messaging JWKS response")
		}
		return body, nil
	}, RefreshAfter: 20 * time.Second, HardExpiry: 60 * time.Second, UnknownKIDCooldown: 5 * time.Second})
	if err != nil {
		transport.CloseIdleConnections()
		return err
	}
	// This callback dependency can start after Bot. Resolve remains fail closed.
	r.messagingResolver, r.messagingHTTP = resolver, client
	return nil
}

func (r *Runtime) VerifyMessaging(ctx context.Context, token, method, id, hash string) (principal.Principal, error) {
	if r == nil || r.messagingResolver == nil || r.replayGuard == nil {
		return principal.Principal{}, status.Error(codes.Unavailable, "Messaging authority unavailable")
	}
	if method != messagingSendAuthorityMethod {
		return principal.Principal{}, errors.New("messaging method denied")
	}
	remote, ok := peer.FromContext(ctx)
	if !ok {
		return principal.Principal{}, errors.New("verified Messaging TLS peer required")
	}
	tlsInfo, ok := remote.AuthInfo.(credentials.TLSInfo)
	if !ok || len(tlsInfo.State.VerifiedChains) == 0 || len(tlsInfo.State.PeerCertificates) == 0 {
		return principal.Principal{}, errors.New("verified Messaging TLS chain required")
	}
	matched := false
	for _, uri := range tlsInfo.State.PeerCertificates[0].URIs {
		if uri.String() == r.config.MessagingURISAN {
			matched = true
		}
	}
	if !matched {
		return principal.Principal{}, errors.New("messaging TLS identity mismatch")
	}
	return principal.VerifyService(ctx, token, principal.VerifyConfig{ExpectedIssuer: "messaging", ExpectedAudience: "bot", ExpectedRPC: method, ExpectedRequestID: id, ExpectedRequestHash: hash, KeyResolver: r.messagingResolver.Resolve, ReplayGuard: r.replayGuard})
}
