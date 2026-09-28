package s2s

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"voice/backend/messaging/internal/gameprotocol"
	"voice/backend/pkg/principal"
)

type GameAuthKeysConfig struct{ JWKSURL, TLSCertFile, TLSKeyFile, CAFile string }

type GameAuthKeys struct {
	url       string
	client    *http.Client
	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	refreshed time.Time
}

func NewGameAuthKeys(config GameAuthKeysConfig) (*GameAuthKeys, error) {
	if !strings.HasPrefix(config.JWKSURL, "https://") || !strings.HasSuffix(config.JWKSURL, "/.well-known/principal-jwks.json") || config.TLSCertFile == "" || config.TLSKeyFile == "" || config.CAFile == "" {
		return nil, errors.New("Auth principal JWKS requires HTTPS and mTLS configuration")
	}
	certificate, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
	if err != nil {
		return nil, errors.New("Auth principal JWKS client certificate is invalid")
	}
	caPEM, err := os.ReadFile(config.CAFile)
	if err != nil {
		return nil, errors.New("Auth principal JWKS CA is unavailable")
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("Auth principal JWKS CA has no certificates")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{certificate}}
	return &GameAuthKeys{url: config.JWKSURL, client: &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("Auth principal JWKS redirects are forbidden")
	}}}, nil
}

func (s *GameAuthKeys) Snapshot(ctx context.Context) (gameprotocol.AuthKeySnapshot, error) {
	if s == nil || s.client == nil || s.url == "" {
		return gameprotocol.AuthKeySnapshot{}, errors.New("Auth principal JWKS is unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.refreshed.IsZero() && time.Since(s.refreshed) <= 4*time.Second {
		return gameprotocol.AuthKeySnapshot{Keys: cloneRSAKeys(s.keys), RefreshedAt: s.refreshed}, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return gameprotocol.AuthKeySnapshot{}, errors.New("Auth principal JWKS request could not be created")
	}
	response, err := s.client.Do(request)
	if err != nil {
		return gameprotocol.AuthKeySnapshot{}, errors.New("Auth principal JWKS request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return gameprotocol.AuthKeySnapshot{}, fmt.Errorf("Auth principal JWKS returned status %d", response.StatusCode)
	}
	document, err := io.ReadAll(io.LimitReader(response.Body, 128*1024+1))
	if err != nil || len(document) > 128*1024 {
		return gameprotocol.AuthKeySnapshot{}, errors.New("Auth principal JWKS response is invalid")
	}
	keys, err := principal.ParseJWKS(document)
	if err != nil {
		return gameprotocol.AuthKeySnapshot{}, err
	}
	s.keys, s.refreshed = keys, time.Now().UTC()
	return gameprotocol.AuthKeySnapshot{Keys: cloneRSAKeys(keys), RefreshedAt: s.refreshed}, nil
}

func cloneRSAKeys(source map[string]*rsa.PublicKey) map[string]*rsa.PublicKey {
	copy := make(map[string]*rsa.PublicKey, len(source))
	for id, key := range source {
		if key != nil {
			clone := *key
			clone.N = new(big.Int).Set(key.N)
			copy[id] = &clone
		}
	}
	return copy
}

func (s *GameAuthKeys) Close() {
	if s != nil && s.client != nil {
		s.client.CloseIdleConnections()
	}
}
