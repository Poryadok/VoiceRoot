package principal

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
)

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}
type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// ParseJWKS accepts only an unambiguous usable RS256 signature key set. It is
// all-or-nothing, so an invalid refresh cannot partially replace a cache.
func ParseJWKS(document []byte) (map[string]*rsa.PublicKey, error) {
	var parsed jwksDocument
	if err := json.Unmarshal(document, &parsed); err != nil {
		return nil, fmt.Errorf("decode jwks: %w", err)
	}
	if len(parsed.Keys) == 0 {
		return nil, errors.New("jwks contains no signing keys")
	}
	keys := make(map[string]*rsa.PublicKey, len(parsed.Keys))
	for _, candidate := range parsed.Keys {
		kid := strings.TrimSpace(candidate.Kid)
		if candidate.Kty != "RSA" || candidate.Use != "sig" || candidate.Alg != "RS256" || kid == "" || candidate.N == "" || candidate.E == "" {
			return nil, errors.New("jwks contains invalid signing key")
		}
		if _, exists := keys[kid]; exists {
			return nil, errors.New("jwks contains duplicate kid")
		}
		key, err := rsaPublicKey(candidate)
		if err != nil {
			return nil, fmt.Errorf("jwks key %q: %w", kid, err)
		}
		keys[kid] = key
	}
	return keys, nil
}

func rsaPublicKey(candidate jwk) (*rsa.PublicKey, error) {
	n, err := base64.RawURLEncoding.DecodeString(candidate.N)
	if err != nil || len(n) == 0 {
		return nil, errors.New("invalid rsa modulus")
	}
	e, err := base64.RawURLEncoding.DecodeString(candidate.E)
	if err != nil || len(e) == 0 {
		return nil, errors.New("invalid rsa exponent")
	}
	if len(e) > 4 {
		return nil, errors.New("invalid rsa exponent")
	}
	exponent := 0
	for _, b := range e {
		exponent = exponent<<8 + int(b)
	}
	if exponent < 3 || exponent%2 == 0 {
		return nil, errors.New("invalid rsa exponent")
	}
	key := &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}
	if key.N.Sign() <= 0 {
		return nil, errors.New("invalid rsa modulus")
	}
	return key, nil
}

// JWKSFetcher remains transport-agnostic; HTTP/TLS configuration belongs to service wiring.
type JWKSFetcher func(ctx context.Context, issuer string) ([]byte, error)

// JWKSResolver caches the last complete valid set per issuer. Current+next key
// rotation is safe because successful refreshes replace a whole valid set; bad
// or empty refreshes leave the previous set untouched.
type JWKSResolver struct {
	fetch   JWKSFetcher
	mu      sync.RWMutex
	refresh sync.Mutex
	sets    map[string]map[string]*rsa.PublicKey
}

func NewJWKSResolver(fetch JWKSFetcher) *JWKSResolver {
	return &JWKSResolver{fetch: fetch, sets: make(map[string]map[string]*rsa.PublicKey)}
}

func (r *JWKSResolver) Resolve(ctx context.Context, issuer, keyID string) (*rsa.PublicKey, error) {
	issuer, keyID = strings.TrimSpace(issuer), strings.TrimSpace(keyID)
	if r == nil || r.fetch == nil || issuer == "" || keyID == "" {
		return nil, errors.New("jwks resolver configuration is invalid")
	}
	if key := r.cached(issuer, keyID); key != nil {
		return key, nil
	}
	r.refresh.Lock()
	defer r.refresh.Unlock()
	if key := r.cached(issuer, keyID); key != nil {
		return key, nil
	}
	if err := r.refreshLocked(ctx, issuer); err != nil {
		return nil, err
	}
	if key := r.cached(issuer, keyID); key != nil {
		return key, nil
	}
	return nil, errors.New("jwks kid is unknown")
}

func (r *JWKSResolver) Refresh(ctx context.Context, issuer string) error {
	issuer = strings.TrimSpace(issuer)
	if r == nil || r.fetch == nil || issuer == "" {
		return errors.New("jwks resolver configuration is invalid")
	}
	r.refresh.Lock()
	defer r.refresh.Unlock()
	return r.refreshLocked(ctx, issuer)
}

func (r *JWKSResolver) refreshLocked(ctx context.Context, issuer string) error {
	document, err := r.fetch(ctx, issuer)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	keys, err := ParseJWKS(document)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.sets[issuer] = keys
	r.mu.Unlock()
	return nil
}

func (r *JWKSResolver) cached(issuer, keyID string) *rsa.PublicKey {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sets[issuer][keyID]
}
